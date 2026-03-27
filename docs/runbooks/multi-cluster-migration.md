# Multi-Cluster Support Migration Runbook

**Version:** 1.0  
**Target Environment:** Production  
**Estimated Duration:** 3-4 hours (with monitoring windows)  
**Risk Level:** Medium (reversible at each phase)

## Overview

This runbook guides the production migration from single-cluster to multi-cluster support. The migration follows a **blue-green strategy** with three coordinated changes:

1. **Database**: Normalize cluster references (TEXT → FK)
2. **Git Structure**: Reorganize from flat to cluster-first hierarchy
3. **Backend**: Deploy ClientManager with lazy client pool

Each phase is reversible independently. Total downtime: **0 minutes** (rolling updates).

---

## Pre-Migration Checklist

### 1. Prerequisites

- [ ] **Backup Database**
  ```bash
  # Production database backup
  pg_dump "$DATABASE_URL" > /backup/secretmanager_$(date +%Y%m%d_%H%M%S).sql
  
  # Verify backup file size
  ls -lh /backup/secretmanager_*.sql | tail -1
  
  # Test restore (optional, on staging)
  psql "$STAGING_DATABASE_URL" < /backup/secretmanager_YYYYMMDD_HHMMSS.sql
  ```
  **Expected**: Backup file > 0 bytes, no errors in `pg_dump` output.

- [ ] **Test Migration in Staging**
  ```bash
  # Apply all steps 1-7 in staging environment first
  # Document any issues encountered
  ```
  **Expected**: All steps complete without errors, drift detection works, secrets publish successfully.

- [ ] **Prepare Kubeconfig Files**
  ```bash
  # Verify kubeconfigs exist for all 6 clusters
  ls -l /etc/kubeconfigs/
  # Expected files:
  # - devops.yaml
  # - integraciones-dev.yaml
  # - integraciones-stg.yaml
  # - integraciones-pro.yaml
  # - kyndryl-dev-stg.yaml
  # - kyndryl-pro.yaml
  
  # Verify file permissions (must be 0400 or 0600)
  chmod 0600 /etc/kubeconfigs/*.yaml
  
  # Test each kubeconfig
  for kubeconfig in /etc/kubeconfigs/*.yaml; do
      echo "Testing $kubeconfig..."
      kubectl --kubeconfig="$kubeconfig" cluster-info
  done
  ```
  **Expected**: All 6 kubeconfigs valid, cluster-info returns successfully.

- [ ] **Verify FluxCD Ready**
  ```bash
  # Check FluxCD is running in each cluster
  for kubeconfig in /etc/kubeconfigs/*.yaml; do
      echo "Checking FluxCD in $(basename $kubeconfig)..."
      kubectl --kubeconfig="$kubeconfig" get kustomizations -n flux-system
  done
  ```
  **Expected**: All clusters have FluxCD Kustomizations in `Ready` state.

- [ ] **Communication**
  - [ ] Notify engineering team of migration window
  - [ ] Schedule 4-hour maintenance window (low-traffic period)
  - [ ] Prepare rollback communication template

---

## Migration Steps

### Step 1: Apply Database Migrations

**Objective**: Create `clusters` table and add `cluster_id` FK to `namespaces` (without dropping old `cluster` column yet).

**Duration**: 5-10 minutes  
**Rollback**: Quick (ALTER TABLE DROP)

```bash
# 1.1 Connect to production database
psql "$DATABASE_URL"

# 1.2 Verify current schema
\d namespaces
# Expected: 'cluster' column exists (VARCHAR), 'cluster_id' column does NOT exist

# 1.3 Apply migration 008 (create clusters table)
\i /path/to/backend/migrations/008_create_clusters.sql

# 1.4 Verify clusters table created
\d clusters
# Expected: Table with columns: id, name, kubeconfig_ref, environment, is_healthy, last_health_check, created_at, updated_at

# 1.5 Apply migration 009 (add cluster_id FK)
\i /path/to/backend/migrations/009_add_cluster_fk_to_namespaces.sql

# 1.6 Verify namespaces table updated
\d namespaces
# Expected: Both 'cluster' (VARCHAR) and 'cluster_id' (UUID) columns exist
#           FK constraint 'fk_namespaces_cluster' points to clusters(id)

\q
```

**Verification Queries**:
```sql
-- Check clusters table is empty (expected before backfill)
SELECT COUNT(*) FROM clusters;
-- Expected: 0

-- Check namespaces.cluster_id is NULL (expected before backfill)
SELECT COUNT(*) FROM namespaces WHERE cluster_id IS NULL;
-- Expected: Equal to total namespace count
```

**Rollback Step 1**:
```sql
ALTER TABLE namespaces DROP CONSTRAINT IF EXISTS fk_namespaces_cluster;
DROP INDEX IF EXISTS idx_namespaces_cluster_id;
ALTER TABLE namespaces DROP COLUMN IF EXISTS cluster_id;
DROP TABLE IF EXISTS clusters;
```

---

### Step 2: Run Backfill Command

**Objective**: Populate `clusters` table from distinct `namespaces.cluster` values and update `cluster_id` FK.

**Duration**: 2-5 minutes  
**Rollback**: TRUNCATE clusters, SET cluster_id = NULL

```bash
# 2.1 Build backfill command
cd /path/to/secret-manager/backend
go build -o /tmp/backfill-clusters ./cmd/backfill-clusters

# 2.2 Dry-run verification (inspect source data)
psql "$DATABASE_URL" -c "SELECT DISTINCT cluster, environment, COUNT(*) AS namespace_count 
                          FROM namespaces 
                          WHERE cluster != '' 
                          GROUP BY cluster, environment 
                          ORDER BY cluster;"
# Expected: List of 6 clusters (devops, integraciones-dev, integraciones-stg, etc.)

# 2.3 Run backfill command
DATABASE_URL="$DATABASE_URL" /tmp/backfill-clusters

# Expected output:
# === Cluster Data Backfill Script ===
# Found 6 distinct cluster(s) in namespaces table
#   [INSERT] Created cluster 'devops' (ID: <uuid>, Env: prod)
#   [INSERT] Created cluster 'integraciones-dev' (ID: <uuid>, Env: dev)
#   ... (6 total)
# Cluster table update: 6 inserted, 0 skipped
# Updating N namespace(s) with cluster_id...
#   Updated N/N namespaces
# ✅ Verification passed: All namespaces have cluster_id set
# === Backfill Complete ===

# 2.4 Verify clusters table populated
psql "$DATABASE_URL" -c "SELECT id, name, kubeconfig_ref, environment, is_healthy FROM clusters ORDER BY name;"
# Expected: 6 rows, kubeconfig_ref = '/etc/kubeconfigs/{cluster-name}.yaml'

# 2.5 Verify namespaces.cluster_id populated
psql "$DATABASE_URL" -c "SELECT COUNT(*) FROM namespaces WHERE cluster_id IS NULL;"
# Expected: 0

# 2.6 Verify referential integrity
psql "$DATABASE_URL" -c "SELECT c.name AS cluster, COUNT(n.id) AS namespace_count 
                          FROM clusters c 
                          LEFT JOIN namespaces n ON c.id = n.cluster_id 
                          GROUP BY c.name 
                          ORDER BY c.name;"
# Expected: Each cluster shows its namespace count (matches old data)
```

**Rollback Step 2**:
```sql
-- Reset cluster_id to NULL
UPDATE namespaces SET cluster_id = NULL;

-- Truncate clusters table
TRUNCATE TABLE clusters CASCADE;
```

---

### Step 3: Migrate Git Repository Structure

**Objective**: Reorganize Git from `namespaces/{ns}/secrets/` to `clusters/{cluster}/namespaces/{ns}/secrets/` while preserving history.

**Duration**: 10-20 minutes (depends on repo size)  
**Rollback**: Git revert commit

```bash
# 3.1 Clone production secrets repository (if not already local)
git clone "$GIT_REPO_URL" /tmp/secrets-repo-migration
cd /tmp/secrets-repo-migration

# 3.2 Verify current structure
ls -la namespaces/
# Expected: Flat list of namespace directories

# 3.3 Run migration script
/path/to/secret-manager/scripts/migrate-git-structure.sh \
    /tmp/secrets-repo-migration \
    "$DATABASE_URL" \
    devops  # Default cluster for any unmapped namespaces

# Expected output:
# [INFO] Git Migration Script - Multi-cluster support
# [INFO] Repository: /tmp/secrets-repo-migration
# [INFO] Found N namespace(s) to migrate
# Migration Plan:
#   default -> clusters/devops/namespaces/default
#   production -> clusters/integraciones-pro/namespaces/production
#   ...
# Proceed with migration? (y/N) y
# [INFO] Moving: namespaces/default -> clusters/devops/namespaces/default
# [INFO] Migration committed successfully
# [WARN] IMPORTANT: Do NOT push to remote yet!

# 3.4 Verify Git structure
ls -la clusters/
# Expected: Subdirectories for each cluster (devops, integraciones-dev, etc.)

ls -la clusters/devops/namespaces/
# Expected: Namespace directories that were previously in namespaces/

# 3.5 Verify Git history preserved
git log --follow clusters/devops/namespaces/default/secrets/ | head -20
# Expected: Commit history includes old commits when path was namespaces/default/secrets/

# 3.6 Verify commit created
git log -1 --stat
# Expected: Single commit with message "chore(git): migrate to cluster-first structure"
#           Shows moved files with 'namespaces/{ns}' => 'clusters/{cluster}/namespaces/{ns}'

# 3.7 DO NOT PUSH YET - wait for Step 4 backend verification
```

**Rollback Step 3**:
```bash
# If not pushed to remote yet:
git reset --hard HEAD~1

# If already pushed (requires force push - DANGER):
# 1. Revert the migration commit
git revert HEAD
git push origin main

# 2. Or restore from backup
git reset --hard <commit-before-migration>
git push --force-with-lease origin main  # USE WITH CAUTION
```

---

### Step 4: Update Backend Deployment

**Objective**: Deploy new backend version with ClientManager, multi-cluster support, and dual-path Git reading.

**Duration**: 10-15 minutes  
**Rollback**: Redeploy previous image tag

```bash
# 4.1 Build new backend image with multi-cluster support
cd /path/to/secret-manager/backend
docker build -t secret-manager-backend:multi-cluster-v1.0 .

# 4.2 Tag and push to registry
docker tag secret-manager-backend:multi-cluster-v1.0 your-registry.io/secret-manager-backend:multi-cluster-v1.0
docker push your-registry.io/secret-manager-backend:multi-cluster-v1.0

# 4.3 Update Kubernetes deployment YAML with new environment variables
cat <<EOF >> /tmp/backend-deployment-patch.yaml
spec:
  template:
    spec:
      containers:
      - name: backend
        image: your-registry.io/secret-manager-backend:multi-cluster-v1.0
        env:
        # NEW: Multi-cluster kubeconfigs directory
        - name: K8S_KUBECONFIGS_DIR
          value: "/etc/kubeconfigs"
        
        # NEW: Enable dual-path mode (reads both old and new Git structure)
        - name: ENABLE_DUAL_PATH_MODE
          value: "true"
        
        volumeMounts:
        # NEW: Mount all kubeconfigs as a volume
        - name: kubeconfigs
          mountPath: /etc/kubeconfigs
          readOnly: true
      
      volumes:
      # NEW: Volume containing all 6 cluster kubeconfigs
      - name: kubeconfigs
        secret:
          secretName: k8s-kubeconfigs
          defaultMode: 0400  # Read-only for owner
EOF

# 4.4 Apply Kubernetes Secret with kubeconfigs (if not already exists)
kubectl create secret generic k8s-kubeconfigs \
    --from-file=devops.yaml=/etc/kubeconfigs/devops.yaml \
    --from-file=integraciones-dev.yaml=/etc/kubeconfigs/integraciones-dev.yaml \
    --from-file=integraciones-stg.yaml=/etc/kubeconfigs/integraciones-stg.yaml \
    --from-file=integraciones-pro.yaml=/etc/kubeconfigs/integraciones-pro.yaml \
    --from-file=kyndryl-dev-stg.yaml=/etc/kubeconfigs/kyndryl-dev-stg.yaml \
    --from-file=kyndryl-pro.yaml=/etc/kubeconfigs/kyndryl-pro.yaml \
    -n secret-manager \
    --dry-run=client -o yaml | kubectl apply -f -

# 4.5 Verify Secret created
kubectl get secret k8s-kubeconfigs -n secret-manager -o yaml | grep -A 10 data:
# Expected: 6 base64-encoded kubeconfig files

# 4.6 Apply deployment patch
kubectl patch deployment secret-manager-backend -n secret-manager --patch-file /tmp/backend-deployment-patch.yaml

# Or if using Helm/Kustomize, update values and apply
# helm upgrade secret-manager ./charts/secret-manager -f values-prod.yaml
```

**Important**: The new backend is **backward compatible**:
- Reads `cluster_id` FK (new) AND `cluster` TEXT (old, for safety)
- **Dual-path mode**: Reads Git from both `clusters/{cluster}/namespaces/` (new) AND `namespaces/` (old)
- If a secret isn't found in new path, falls back to old path automatically

---

### Step 5: Restart Backend Pods

**Objective**: Rolling restart to load new ClientManager with kubeconfig volume mounts.

**Duration**: 5 minutes  
**Rollback**: Redeploy previous version (see Step 4 rollback)

```bash
# 5.1 Trigger rolling restart
kubectl rollout restart deployment/secret-manager-backend -n secret-manager

# 5.2 Watch rollout status
kubectl rollout status deployment/secret-manager-backend -n secret-manager --timeout=5m
# Expected: "deployment "secret-manager-backend" successfully rolled out"

# 5.3 Verify new pods are running
kubectl get pods -n secret-manager -l app=secret-manager-backend
# Expected: All pods in 'Running' state with recent start time (< 5 minutes ago)

# 5.4 Check pod logs for successful startup
kubectl logs -n secret-manager deployment/secret-manager-backend --tail=50 | grep -i "cluster"
# Expected logs:
# [INFO] Initializing ClientManager with kubeconfigs directory: /etc/kubeconfigs
# [INFO] Loaded 6 clusters from database
# [INFO] Server started on :8080
```

**Rollback Step 5**:
```bash
# Rollback to previous deployment revision
kubectl rollout undo deployment/secret-manager-backend -n secret-manager

# Or redeploy previous image tag
kubectl set image deployment/secret-manager-backend backend=your-registry.io/secret-manager-backend:previous-tag -n secret-manager
```

---

### Step 6: Verify Cluster Health Checks

**Objective**: Confirm all 6 clusters are marked `is_healthy=true` and backend can reach them.

**Duration**: 5-10 minutes  
**Rollback**: N/A (read-only verification)

```bash
# 6.1 Check clusters table health status
psql "$DATABASE_URL" -c "SELECT name, environment, is_healthy, last_health_check FROM clusters ORDER BY name;"
# Expected: All 6 clusters with is_healthy = true

# If any cluster shows is_healthy = false:
#   - Check kubeconfig file exists and is readable
#   - Test connectivity: kubectl --kubeconfig=/etc/kubeconfigs/{cluster}.yaml cluster-info
#   - Check pod logs for ClientManager errors

# 6.2 Trigger drift detection (forces health checks on all clusters)
curl -X POST http://secret-manager-backend.secret-manager.svc.cluster.local:8080/api/drift/detect \
    -H "Authorization: Bearer $ADMIN_JWT_TOKEN"
# Expected: HTTP 200 OK, response body shows drift detection started

# 6.3 Check drift detection logs
kubectl logs -n secret-manager deployment/secret-manager-backend --tail=100 | grep -A 5 "DetectDriftForAllClusters"
# Expected logs:
# [INFO] Starting drift detection for all clusters
# [INFO] Checking cluster: devops
# [INFO] Loaded K8s client for cluster devops (cached)
# [INFO] Checking cluster: integraciones-dev
# ... (repeat for all 6 clusters)
# [INFO] Drift detection completed for 6 clusters

# 6.4 Verify no errors in logs
kubectl logs -n secret-manager deployment/secret-manager-backend --tail=200 | grep -i "error\|fail"
# Expected: No errors related to "cluster" or "kubeconfig"

# 6.5 Test lazy client initialization (GET /api/clusters)
curl http://secret-manager-backend.secret-manager.svc.cluster.local:8080/api/clusters \
    -H "Authorization: Bearer $ADMIN_JWT_TOKEN"
# Expected: JSON array with 6 cluster objects, all with "is_healthy": true
```

**Troubleshooting**:
- **Cluster marked unhealthy**: Check kubeconfig permissions (must be 0400/0600), verify kubectl can reach cluster from pod
- **Client initialization timeout**: Increase timeout in ClientManager (default 10s), check network connectivity
- **"Kubeconfig not found" error**: Verify volume mount path matches K8S_KUBECONFIGS_DIR, check Secret exists

---

### Step 7: Push Git Changes and Update FluxCD Kustomizations

**Objective**: Push cluster-first Git structure to remote and update FluxCD to point to new paths.

**Duration**: 30-60 minutes (staggered per cluster with monitoring)  
**Rollback**: Revert Kustomization path changes

#### 7.1 Push Git Changes

```bash
# From Step 3 working directory
cd /tmp/secrets-repo-migration

# Final verification before push
git log -1 --stat
git diff HEAD~1

# Push to remote
git push origin main

# Expected: Push successful, no conflicts
# Verify on GitLab/GitHub UI that clusters/ directory structure is visible
```

#### 7.2 Update FluxCD Kustomizations (PER CLUSTER, STAGGERED)

**Strategy**: Update one cluster at a time, monitor for 10 minutes before proceeding to next.

**Kustomization Changes**:
```yaml
# OLD path:
spec:
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  path: ./namespaces  # <-- OLD

# NEW path:
spec:
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  path: ./clusters/devops/namespaces  # <-- NEW (cluster-specific)
```

**For each cluster** (repeat 6 times):

```bash
# Define cluster name
CLUSTER_NAME="devops"  # Change for each cluster: devops, integraciones-dev, etc.
KUBECONFIG="/etc/kubeconfigs/${CLUSTER_NAME}.yaml"

# 7.2.1 Backup current Kustomization
kubectl --kubeconfig="$KUBECONFIG" get kustomization secrets -n flux-system -o yaml > "/tmp/kustomization-${CLUSTER_NAME}-backup.yaml"

# 7.2.2 Update Kustomization path
kubectl --kubeconfig="$KUBECONFIG" patch kustomization secrets -n flux-system --type=merge -p "{\"spec\":{\"path\":\"./clusters/${CLUSTER_NAME}/namespaces\"}}"

# Expected: kustomization.kustomize.toolkit.fluxcd.io/secrets patched

# 7.2.3 Force FluxCD reconciliation
kubectl --kubeconfig="$KUBECONFIG" annotate kustomization secrets -n flux-system reconcile.fluxcd.io/requestedAt="$(date +%s)" --overwrite

# 7.2.4 Watch reconciliation status
kubectl --kubeconfig="$KUBECONFIG" get kustomization secrets -n flux-system -w
# Expected: Status changes from "Unknown" -> "Progressing" -> "Ready"
# Wait for "Ready" status before proceeding (typically 1-3 minutes)

# 7.2.5 Verify secrets applied
kubectl --kubeconfig="$KUBECONFIG" get secrets -A | grep -i "secret-manager"
# Expected: Secrets from new Git path are applied

# 7.2.6 Check for reconciliation errors
kubectl --kubeconfig="$KUBECONFIG" describe kustomization secrets -n flux-system | grep -A 20 "Events:"
# Expected: No error events

# 7.2.7 Monitor for 10 minutes (check application logs, alerts)
# If any issues detected, rollback this cluster before proceeding to next

# 7.2.8 Repeat for next cluster
```

**Rollback Kustomization**:
```bash
# Restore from backup
kubectl --kubeconfig="$KUBECONFIG" apply -f "/tmp/kustomization-${CLUSTER_NAME}-backup.yaml"

# Or patch back to old path
kubectl --kubeconfig="$KUBECONFIG" patch kustomization secrets -n flux-system --type=merge -p '{"spec":{"path":"./namespaces"}}'
```

---

## Rollback Plan

### Full Rollback (Undo All Changes)

**Execute in reverse order**:

#### Phase 3 Rollback: Revert Backend Deployment
```bash
# Rollback to previous Kubernetes deployment
kubectl rollout undo deployment/secret-manager-backend -n secret-manager

# Verify old backend is running
kubectl get pods -n secret-manager -l app=secret-manager-backend
kubectl logs -n secret-manager deployment/secret-manager-backend --tail=20
```

#### Phase 2 Rollback: Revert Git Structure
```bash
cd /tmp/secrets-repo-migration

# If not pushed to remote:
git reset --hard HEAD~1

# If already pushed:
git revert HEAD  # Creates a new commit that undoes migration
git push origin main

# Update FluxCD Kustomizations back to old path (all clusters)
for kubeconfig in /etc/kubeconfigs/*.yaml; do
    kubectl --kubeconfig="$kubeconfig" patch kustomization secrets -n flux-system --type=merge -p '{"spec":{"path":"./namespaces"}}'
done
```

#### Phase 1 Rollback: Revert Database Migrations
```bash
# Connect to database
psql "$DATABASE_URL"

# Drop migration 010 changes (if applied)
ALTER TABLE namespaces ADD COLUMN IF NOT EXISTS cluster VARCHAR(255);
UPDATE namespaces SET cluster = clusters.name FROM clusters WHERE namespaces.cluster_id = clusters.id;
ALTER TABLE namespaces ALTER COLUMN cluster SET NOT NULL;
ALTER TABLE namespaces ALTER COLUMN cluster_id DROP NOT NULL;
DROP INDEX IF EXISTS idx_namespaces_cluster_name;
CREATE INDEX IF NOT EXISTS idx_namespaces_cluster ON namespaces(cluster);
CREATE UNIQUE INDEX IF NOT EXISTS namespaces_name_cluster_key ON namespaces(name, cluster);

# Drop migration 009 changes
ALTER TABLE namespaces DROP CONSTRAINT IF EXISTS fk_namespaces_cluster;
DROP INDEX IF EXISTS idx_namespaces_cluster_id;
ALTER TABLE namespaces DROP COLUMN IF EXISTS cluster_id;

# Drop migration 008 changes
DROP TABLE IF EXISTS clusters CASCADE;

# Verify rollback
\d namespaces
-- Expected: Only 'cluster' VARCHAR column exists, no 'cluster_id'

\q
```

---

## Verification Steps

### Post-Migration Health Checks

#### 1. Database Integrity
```sql
-- All namespaces have valid cluster_id FK
SELECT COUNT(*) FROM namespaces WHERE cluster_id IS NULL;
-- Expected: 0

-- All clusters are represented
SELECT c.name, COUNT(n.id) AS namespace_count 
FROM clusters c 
LEFT JOIN namespaces n ON c.id = n.cluster_id 
GROUP BY c.name;
-- Expected: 6 clusters with namespace counts matching pre-migration data

-- No orphaned namespaces
SELECT n.id, n.name FROM namespaces n 
LEFT JOIN clusters c ON n.cluster_id = c.id 
WHERE c.id IS NULL;
-- Expected: 0 rows
```

#### 2. Git Structure
```bash
# Verify new structure exists
ls -la /tmp/secrets-repo-migration/clusters/
# Expected: 6 cluster directories

# Verify old structure removed
ls -la /tmp/secrets-repo-migration/namespaces/
# Expected: Directory does not exist OR is empty

# Verify secret files exist in new paths
find /tmp/secrets-repo-migration/clusters/ -name "*.yaml" -type f | wc -l
# Expected: Count matches old structure (all secrets migrated)
```

#### 3. Backend Functionality
```bash
# Test cluster health endpoint (NEW)
curl http://secret-manager-backend.secret-manager.svc.cluster.local:8080/api/clusters \
    -H "Authorization: Bearer $ADMIN_JWT_TOKEN" | jq '.[] | {name, is_healthy}'
# Expected: All 6 clusters with is_healthy: true

# Test drift detection for all clusters
curl -X POST http://secret-manager-backend.secret-manager.svc.cluster.local:8080/api/drift/detect \
    -H "Authorization: Bearer $ADMIN_JWT_TOKEN"
# Expected: HTTP 200, logs show drift checked for all 6 clusters

# Test secret publish to specific cluster
curl -X POST http://secret-manager-backend.secret-manager.svc.cluster.local:8080/api/namespaces/{namespace_id}/secrets/test-secret/publish \
    -H "Authorization: Bearer $ADMIN_JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"data":{"key":"value"}}'
# Expected: HTTP 200, secret appears in Git at clusters/{cluster}/namespaces/{ns}/secrets/test-secret.yaml
```

#### 4. FluxCD Reconciliation
```bash
# Check all cluster Kustomizations are Ready
for kubeconfig in /etc/kubeconfigs/*.yaml; do
    echo "Cluster: $(basename $kubeconfig)"
    kubectl --kubeconfig="$kubeconfig" get kustomization secrets -n flux-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
    echo ""
done
# Expected: All print "True"

# Verify secrets reconciled from new paths
for kubeconfig in /etc/kubeconfigs/*.yaml; do
    echo "Cluster: $(basename $kubeconfig)"
    kubectl --kubeconfig="$kubeconfig" get secrets -A | grep "secret-manager" | wc -l
    echo ""
done
# Expected: Secret counts match pre-migration counts
```

---

## Troubleshooting

### Issue: Backend can't load kubeconfigs

**Symptoms**:
- Logs show "failed to initialize client for cluster X: stat /etc/kubeconfigs/X.yaml: no such file or directory"
- Clusters marked `is_healthy=false`

**Resolution**:
1. Check volume mount exists:
   ```bash
   kubectl exec -n secret-manager deployment/secret-manager-backend -- ls -la /etc/kubeconfigs/
   ```
2. Verify Secret exists:
   ```bash
   kubectl get secret k8s-kubeconfigs -n secret-manager
   ```
3. Check file permissions:
   ```bash
   kubectl exec -n secret-manager deployment/secret-manager-backend -- stat /etc/kubeconfigs/devops.yaml
   # Expected: Mode: 0400 or 0600
   ```

### Issue: Drift detection fails for one cluster

**Symptoms**:
- Logs show "cluster X: context deadline exceeded" or "connection refused"
- Other clusters work fine

**Resolution**:
1. Test kubeconfig from pod:
   ```bash
   kubectl exec -n secret-manager deployment/secret-manager-backend -- kubectl --kubeconfig=/etc/kubeconfigs/X.yaml cluster-info
   ```
2. Check cluster API server reachability from pod network
3. Temporarily mark cluster as unhealthy:
   ```sql
   UPDATE clusters SET is_healthy = false WHERE name = 'X';
   ```
4. Drift detection will skip unhealthy clusters

### Issue: FluxCD Kustomization stuck in "Progressing"

**Symptoms**:
- `kubectl get kustomization secrets -n flux-system` shows "Progressing" for > 5 minutes

**Resolution**:
1. Check Kustomization events:
   ```bash
   kubectl describe kustomization secrets -n flux-system
   ```
2. Verify Git path exists:
   ```bash
   # From Git repository root
   ls -la clusters/{cluster-name}/namespaces/
   ```
3. Check FluxCD logs:
   ```bash
   kubectl logs -n flux-system deployment/kustomize-controller --tail=100
   ```
4. Force reconciliation:
   ```bash
   flux reconcile kustomization secrets --with-source
   ```

### Issue: Secret publish writes to wrong Git path

**Symptoms**:
- Secret appears in old path `namespaces/{ns}/secrets/` instead of new path `clusters/{cluster}/namespaces/{ns}/secrets/`

**Resolution**:
1. Verify `ENABLE_DUAL_PATH_MODE=false` (should be false after Step 7 completion)
2. Check namespace has correct `cluster_id`:
   ```sql
   SELECT n.name, n.cluster_id, c.name AS cluster_name 
   FROM namespaces n 
   JOIN clusters c ON n.cluster_id = c.id 
   WHERE n.id = '{namespace_id}';
   ```
3. Restart backend pods:
   ```bash
   kubectl rollout restart deployment/secret-manager-backend -n secret-manager
   ```

---

## Post-Migration Tasks

### 1. Disable Dual-Path Mode (After 48h Monitoring)

Once confident the new structure is stable:

```bash
# Update backend deployment to remove dual-path flag
kubectl set env deployment/secret-manager-backend -n secret-manager ENABLE_DUAL_PATH_MODE=false

# Restart pods
kubectl rollout restart deployment/secret-manager-backend -n secret-manager
```

This forces the backend to **only** read from the new cluster-first Git structure.

### 2. Apply Migration 010 (Drop Old Cluster Column)

After 1 week of stable operation:

```bash
psql "$DATABASE_URL"

# Final verification: All namespaces have cluster_id
SELECT COUNT(*) FROM namespaces WHERE cluster_id IS NULL;
-- Expected: 0

# Apply migration 010
\i /path/to/backend/migrations/010_drop_old_cluster_column.sql

# Verify old column dropped
\d namespaces
-- Expected: Only 'cluster_id' column exists, 'cluster' column removed

\q
```

### 3. Remove Old Git Paths (After 2 Weeks)

After confirming all FluxCD Kustomizations use new paths:

```bash
cd /tmp/secrets-repo-migration

# Verify no remaining references to old path
grep -r "path.*namespaces" .flux/ kustomizations/ manifests/ || echo "No old paths found"

# If safe, delete old path from history (optional - requires force push)
# NOT RECOMMENDED unless disk space critical - Git history is cheap

# Alternative: Keep old path for audit trail (recommended)
```

---

## Success Criteria

- [ ] All 6 clusters visible in `GET /api/clusters` API endpoint
- [ ] Database integrity checks pass (no NULL `cluster_id`, all FKs valid)
- [ ] Drift detection completes successfully for all 6 clusters
- [ ] Secret publish creates files in cluster-specific Git paths
- [ ] FluxCD Kustomizations reconcile from new paths across all clusters
- [ ] No errors in backend logs related to cluster/kubeconfig loading
- [ ] All health checks return 200 OK
- [ ] Rollback tested and documented (in staging)

---

## Timeline Summary

| Step | Duration | Cumulative | Downtime |
|------|----------|------------|----------|
| 1. Database migrations | 10 min | 10 min | 0 min |
| 2. Backfill command | 5 min | 15 min | 0 min |
| 3. Git structure migration | 20 min | 35 min | 0 min |
| 4. Update deployment | 15 min | 50 min | 0 min |
| 5. Restart pods | 5 min | 55 min | 0 min (rolling) |
| 6. Health check verification | 10 min | 65 min | 0 min |
| 7. FluxCD updates (6 clusters × 15 min) | 90 min | 155 min | 0 min |
| **Buffer for issues** | 65 min | **220 min (3h 40m)** | **0 min** |

**Total Estimated Time**: 3-4 hours  
**Total Downtime**: 0 minutes (rolling updates throughout)

---

## Contacts

- **On-Call Engineer**: [Your Contact]
- **Database Admin**: [DBA Contact]
- **FluxCD Expert**: [FluxCD Contact]
- **Escalation**: [Manager Contact]

---

## Appendix: Quick Reference Commands

```bash
# Check backend health
curl http://secret-manager-backend.secret-manager.svc.cluster.local:8080/health

# Check all cluster statuses
psql "$DATABASE_URL" -c "SELECT name, is_healthy FROM clusters;"

# Check FluxCD across all clusters
for kc in /etc/kubeconfigs/*.yaml; do kubectl --kubeconfig="$kc" get kustomization -n flux-system; done

# Check backend logs for errors
kubectl logs -n secret-manager deployment/secret-manager-backend --tail=100 | grep -i error

# Trigger drift detection
curl -X POST http://secret-manager-backend.secret-manager.svc.cluster.local:8080/api/drift/detect \
    -H "Authorization: Bearer $ADMIN_JWT_TOKEN"
```
