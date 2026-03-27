# FluxCD Setup for Multi-Cluster Secret Manager

> **Note**: This document covers both the original single-cluster setup and the new multi-cluster architecture introduced in Phase 6 of the multi-cluster-support change.

## Architecture Overview

Secret Manager now supports **multi-cluster deployments** with a cluster-first Git repository structure:

```
secrets-repo/
└── clusters/
    ├── devops/
    │   └── namespaces/
    │       ├── namespace-a/
    │       │   └── secrets/
    │       │       └── secret-1.yaml
    │       └── namespace-b/
    │           └── secrets/
    │               └── secret-2.yaml
    ├── integraciones-dev/
    │   └── namespaces/
    ├── integraciones-stg/
    │   └── namespaces/
    ├── integraciones-pro/
    │   └── namespaces/
    ├── kyndryl-dev-stg/
    │   └── namespaces/
    └── kyndryl-pro/
        └── namespaces/
```

**Key principles:**
- Each cluster has its own isolated path in Git
- FluxCD reconciles **per-cluster** (no cross-cluster interference)
- Secrets are encrypted with SOPS (Age encryption)
- Backward compatibility maintained with legacy flat structure

## ✅ Completed Tasks

### 1. FluxCD Installation
- ✅ Flux CLI installed at `/usr/local/bin/flux`
- ✅ Flux controllers deployed to `flux-system` namespace
- ✅ All controllers healthy and running:
  - helm-controller
  - kustomize-controller
  - notification-controller
  - source-controller

Verify with:
```bash
flux check
kubectl -n flux-system get pods
```

### 2. SOPS Age Key Configuration
- ✅ Age private key configured in Kubernetes secret
- ✅ Secret `sops-age` created in `flux-system` namespace
- ✅ Contains `age.agekey` from `/home/nk/secret-manager/dev-data/age-keys/keys.txt`

Verify with:
```bash
kubectl -n flux-system get secret sops-age
```

## Multi-Cluster FluxCD Configuration

### Per-Cluster Kustomization Setup

Each cluster requires its own Kustomization resource that points to the cluster-specific path in the Git repository. FluxCD will reconcile each cluster **independently** based on changes to its designated path.

#### Example 1: DevOps Cluster

Create `flux-config/kustomization-devops.yaml`:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-devops
  namespace: flux-system
spec:
  interval: 1m
  path: ./clusters/devops/namespaces
  prune: true
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  targetNamespace: default
```

**Key settings:**
- `path: ./clusters/devops/namespaces` - Points to devops cluster secrets only
- `interval: 1m` - Checks Git repository every minute for changes
- `prune: true` - Removes secrets deleted from Git
- `decryption.provider: sops` - Decrypts SOPS-encrypted secrets before applying

#### Example 2: Integraciones Dev Cluster

Create `flux-config/kustomization-integraciones-dev.yaml`:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-integraciones-dev
  namespace: flux-system
spec:
  interval: 1m
  path: ./clusters/integraciones-dev/namespaces
  prune: true
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  targetNamespace: default
```

#### Example 3: Integraciones Staging Cluster

Create `flux-config/kustomization-integraciones-stg.yaml`:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-integraciones-stg
  namespace: flux-system
spec:
  interval: 1m
  path: ./clusters/integraciones-stg/namespaces
  prune: true
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  targetNamespace: default
```

#### Example 4: Integraciones Production Cluster

Create `flux-config/kustomization-integraciones-pro.yaml`:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-integraciones-pro
  namespace: flux-system
spec:
  interval: 1m
  path: ./clusters/integraciones-pro/namespaces
  prune: true
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  targetNamespace: default
```

#### Example 5: Kyndryl Dev/Staging Cluster

Create `flux-config/kustomization-kyndryl-dev-stg.yaml`:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-kyndryl-dev-stg
  namespace: flux-system
spec:
  interval: 1m
  path: ./clusters/kyndryl-dev-stg/namespaces
  prune: true
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  targetNamespace: default
```

#### Example 6: Kyndryl Production Cluster

Create `flux-config/kustomization-kyndryl-pro.yaml`:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-kyndryl-pro
  namespace: flux-system
spec:
  interval: 1m
  path: ./clusters/kyndryl-pro/namespaces
  prune: true
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  targetNamespace: default
```

### Applying Cluster-Specific Kustomizations

Deploy the Kustomization resources to each cluster:

```bash
# DevOps cluster
kubectl --context devops apply -f flux-config/kustomization-devops.yaml

# Integraciones clusters
kubectl --context integraciones-dev apply -f flux-config/kustomization-integraciones-dev.yaml
kubectl --context integraciones-stg apply -f flux-config/kustomization-integraciones-stg.yaml
kubectl --context integraciones-pro apply -f flux-config/kustomization-integraciones-pro.yaml

# Kyndryl clusters
kubectl --context kyndryl-dev-stg apply -f flux-config/kustomization-kyndryl-dev-stg.yaml
kubectl --context kyndryl-pro apply -f flux-config/kustomization-kyndryl-pro.yaml
```

## How FluxCD Reconciliation Works

### Per-Cluster Isolation

FluxCD reconciles **each cluster independently**:

1. **GitRepository Monitoring**: The `source-controller` watches the configured Git repository for changes
2. **Path-Based Filtering**: Each Kustomization watches only its cluster-specific path (e.g., `./clusters/devops/namespaces`)
3. **Change Detection**: When a commit modifies files in a cluster's path, FluxCD triggers reconciliation **only for that cluster**
4. **Decryption**: The `kustomize-controller` decrypts SOPS-encrypted secrets using the Age key from `sops-age` secret
5. **Application**: Decrypted secrets are applied to the target Kubernetes cluster
6. **Isolation Guarantee**: Changes to `clusters/devops/` do NOT trigger reconciliation for `clusters/integraciones-dev/`

### Reconciliation Triggers

FluxCD reconciles secrets when:

| Trigger | Description | Timing |
|---------|-------------|--------|
| **Interval** | Periodic polling of Git repository | Every 1 minute (configurable via `spec.interval`) |
| **Git Commit** | New commit pushed to monitored branch | Immediate (if webhook configured) |
| **Manual** | Operator forces reconciliation | On-demand via `flux reconcile` |
| **Secret Change** | SOPS Age key updated | Automatic retry after secret update |

### Adding a New Namespace to a Cluster

When you add a new namespace to a cluster (e.g., `clusters/devops/namespaces/new-namespace/secrets/`):

1. **Commit to Git**: Push the new directory structure with encrypted secrets
2. **FluxCD Detects Change**: Within 1 minute, FluxCD detects the commit
3. **Automatic Reconciliation**: The Kustomization for `devops` reconciles the new namespace
4. **Namespace Creation**: Kubernetes namespace is created (if it doesn't exist)
5. **Secret Application**: All secrets in `new-namespace/secrets/` are decrypted and applied

**No manual intervention required** — FluxCD handles the entire lifecycle.

### Removing a Secret from Git

When you delete a secret from Git (e.g., remove `clusters/devops/namespaces/production/secrets/old-secret.yaml`):

1. **Commit Deletion**: Push commit removing the YAML file
2. **FluxCD Detects Deletion**: Kustomization detects the file is missing
3. **Prune Behavior**: If `spec.prune: true`, FluxCD **deletes the secret from Kubernetes**
4. **Safety**: Secrets removed from Git are removed from the cluster (ensure backups if needed)

### 3. Manual SOPS Decryption Testing
- ✅ SOPS decryption tested successfully
- ✅ Secrets can be decrypted and applied to Kubernetes
- ✅ Example: `db-credentials` secret created in `development` namespace

Test manually:
```bash
export SOPS_AGE_KEY_FILE=/home/nk/secret-manager/dev-data/age-keys/keys.txt
sops -d dev-data/secrets-repo/namespaces/development/secrets/db-credentials.yaml | kubectl apply -f -
```

### 4. Automation Scripts Created
- ✅ `scripts/sync-secrets-to-k8s.sh` - Manual sync script for development
- ✅ `scripts/setup-flux-gitops.sh` - FluxCD GitOps setup script
- ✅ `flux-config/` directory with GitRepository and Kustomization manifests

## ⚠️ Pending: Git Remote Configuration

FluxCD requires a Git repository that is accessible over the network. The current secrets repository at `/home/nk/secret-manager/dev-data/secrets-repo/` is **local only** and has no remote configured.

### Why This Matters

FluxCD's source-controller needs to clone the Git repository to sync secrets. It cannot access local filesystem paths directly. This is a limitation of the GitOps model, which requires a Git server.

### Options to Complete GitOps Setup

#### Option A: Use GitHub/GitLab (Recommended for Production)

1. Create a new **private** repository on GitHub or GitLab
2. Push the secrets repo:
   ```bash
   cd /home/nk/secret-manager/dev-data/secrets-repo
   git remote add origin https://github.com/yourorg/secrets-repo.git
   git push -u origin main
   ```
3. Configure Flux:
   ```bash
   export SECRETS_REPO_URL=https://github.com/yourorg/secrets-repo.git
   ./scripts/setup-flux-gitops.sh
   ```
4. For private repos, create credentials:
   ```bash
   kubectl create secret generic git-credentials \
     --namespace=flux-system \
     --from-literal=username=your-username \
     --from-literal=password=your-token
   ```
   Then update `flux-config/gitrepository.yaml` to reference `git-credentials`.

#### Option B: Local Git Daemon (Development Only)

Start a local Git daemon to expose the repo:

1. **Start git daemon**:
   ```bash
   cd /home/nk/secret-manager/dev-data/secrets-repo
   git daemon --reuseaddr --base-path=. --export-all --verbose --enable=receive-pack &
   ```

2. **Configure Flux to use local daemon**:
   ```bash
   # Update URL in flux-config/gitrepository.yaml to:
   # url: git://host.docker.internal/secrets-repo
   kubectl apply -f flux-config/
   ```

   **Note**: `host.docker.internal` allows the Kubernetes cluster running in Docker Desktop to reach the host machine.

3. **Verify sync**:
   ```bash
   flux get sources git secrets-repo
   flux get kustomizations secrets
   ```

#### Option C: Manual Sync for Local Development

For local development without a Git remote, continue using the manual sync script:

```bash
./scripts/sync-secrets-to-k8s.sh
```

This script:
- Reads all secrets from `dev-data/secrets-repo/namespaces/*/secrets/`
- Decrypts them using SOPS
- Applies them to the cluster

**Advantages**:
- No Git server required
- Instant sync (no 1-minute interval)
- Simple to use

**Disadvantages**:
- Not automated (must run manually after publish)
- Doesn't follow true GitOps principles

## 🔄 Current Workflow

### With Manual Sync (Current State)

1. User edits secret via UI → Backend creates draft
2. User clicks "Publish" → Backend writes encrypted YAML to Git repo (`dev-data/secrets-repo/`)
3. **Manual step**: Run `./scripts/sync-secrets-to-k8s.sh` to sync to cluster
4. Secrets available in Kubernetes

### With FluxCD (After Git Remote Setup)

1. User edits secret via UI → Backend creates draft
2. User clicks "Publish" → Backend writes encrypted YAML, commits & **pushes to remote Git**
3. **Flux automatically detects change** (within 1 minute)
4. Flux decrypts and applies secret to cluster
5. Secrets available in Kubernetes

## 🧪 Testing the Complete Pipeline

### Test Backend Fix (Edit Published Secret)

```bash
# 1. Create and publish a secret via UI
# 2. Try to edit the published secret
# 3. Should succeed and revert status to "draft"
# 4. Re-publish button should appear

# Or via API:
curl -X PUT http://localhost:8080/api/v1/namespaces/{ns-id}/secrets/test-secret \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{"data": {"key": "newvalue"}}'

# Expected: 200 OK with status: "draft"
```

### Test Frontend Fix

1. Navigate to http://localhost:3000/secrets
2. Find a published secret
3. Verify:
   - ✅ Edit button is visible with ✏️ icon
   - ✅ "Re-Publish" button is visible
   - ✅ Hover tooltip shows "Editing will create a new draft version"

### Test SOPS Sync

```bash
# Manual sync
./scripts/sync-secrets-to-k8s.sh

# Verify secrets exist
kubectl get secrets -A | grep -E 'development|staging|production'

# Check secret content
kubectl -n development get secret db-credentials -o jsonpath='{.data.DB_HOST}' | base64 -d
# Expected: postgres.dev.svc.cluster.local
```

## 📊 Current Status Summary

| Task | Status | Notes |
|------|--------|-------|
| Backend: Allow editing published secrets | ✅ Complete | Auto-reverts to draft, creates audit log |
| Backend: Allow deleting published secrets | ✅ Complete | Creates audit log before deletion |
| Frontend: Show Edit button for all statuses | ✅ Complete | Shows ✏️ icon for published/drifted |
| Frontend: Conditional Publish/Re-Publish | ✅ Complete | Shows "Re-Publish" for published/drifted |
| FluxCD: Install Flux | ✅ Complete | All controllers running |
| FluxCD: Configure SOPS Age key | ✅ Complete | Secret created in flux-system |
| FluxCD: Manual SOPS decrypt test | ✅ Complete | Successfully decrypted and applied |
| FluxCD: GitRepository configuration | ⚠️ Blocked | Requires Git remote URL |
| FluxCD: Kustomization with SOPS | ⚠️ Blocked | Requires GitRepository |
| FluxCD: Automated sync | ⚠️ Blocked | Requires Git remote URL |

## 🚀 Next Steps

1. **Choose a Git remote option** (A, B, or C above)
2. **For Option A (GitHub/GitLab)**:
   - Create private repository
   - Push secrets repo
   - Configure Flux with `./scripts/setup-flux-gitops.sh`
3. **For Option B (Git Daemon)**:
   - Start git daemon
   - Apply Flux configuration
4. **For Option C (Manual Sync)**:
   - Continue using `./scripts/sync-secrets-to-k8s.sh`
   - Consider integrating into backend publish workflow

## 📚 Documentation Files

- `flux-config/README.md` - Detailed FluxCD setup instructions
- `flux-config/gitrepository.yaml` - GitRepository manifest (requires URL update)
- `flux-config/kustomization.yaml` - Kustomization with SOPS decryption
- `scripts/sync-secrets-to-k8s.sh` - Manual sync script
- `scripts/setup-flux-gitops.sh` - Automated Flux setup script

## 🔍 Verification Commands

```bash
# Check Flux health
flux check

# List Flux resources
flux get all

# Check GitRepository (after setup)
flux get sources git secrets-repo
kubectl -n flux-system describe gitrepository secrets-repo

# Check Kustomization (after setup)
flux get kustomizations secrets
kubectl -n flux-system describe kustomization secrets

# Monitor Flux logs
kubectl -n flux-system logs -l app=source-controller -f
kubectl -n flux-system logs -l app=kustomize-controller -f

# Verify secrets in cluster
kubectl get secrets -A | grep -E 'development|staging|production'
```

## 🐛 Troubleshooting

### Backend not restarting after code changes

```bash
docker-compose restart backend
docker-compose logs -f backend
```

### Frontend not showing changes

```bash
# Clear browser cache or hard refresh (Ctrl+Shift+R)
# Or restart Next.js dev server if running locally
```

### SOPS decryption fails

```bash
# Verify Age key is correct
cat /home/nk/secret-manager/dev-data/age-keys/keys.txt

# Test decryption manually
export SOPS_AGE_KEY_FILE=/home/nk/secret-manager/dev-data/age-keys/keys.txt
sops -d dev-data/secrets-repo/namespaces/development/secrets/db-credentials.yaml
```

### Flux not syncing (after Git remote setup)

```bash
# Force reconciliation
flux reconcile source git secrets-repo
flux reconcile kustomization secrets

# Check events
kubectl -n flux-system get events --sort-by='.lastTimestamp'
```

## 🔧 Multi-Cluster Troubleshooting

This section covers common issues specific to multi-cluster FluxCD deployments.

### Issue 1: Path Mismatch - Kustomization Not Finding Secrets

**Symptoms:**
- Kustomization status shows "path not found" or "no resources found"
- `flux get kustomizations` shows "Applied revision" but no secrets are created
- Logs show: `Kustomization/flux-system/secrets-devops - no Kubernetes objects found`

**Cause:**
The `spec.path` in the Kustomization YAML does not match the actual Git repository structure.

**Diagnosis:**
```bash
# Check Kustomization status
kubectl -n flux-system describe kustomization secrets-devops

# Check GitRepository to see cloned content
flux get sources git secrets-repo -o yaml

# Manually inspect Git repository structure
git clone <your-repo-url>
ls -R clusters/
```

**Solution:**
1. Verify the correct path in your Git repository:
   ```bash
   git ls-tree -r HEAD --name-only | grep clusters/devops
   ```
2. Update the Kustomization path to match:
   ```yaml
   spec:
     path: ./clusters/devops/namespaces  # Must match Git structure exactly
   ```
3. Reapply the Kustomization:
   ```bash
   kubectl apply -f flux-config/kustomization-devops.yaml
   flux reconcile kustomization secrets-devops
   ```

**Common Path Mistakes:**
| Wrong Path | Correct Path | Issue |
|------------|--------------|-------|
| `./devops/namespaces` | `./clusters/devops/namespaces` | Missing `clusters/` prefix |
| `/clusters/devops/namespaces` | `./clusters/devops/namespaces` | Leading `/` causes absolute path error |
| `clusters/devops/namespaces/` | `./clusters/devops/namespaces` | Trailing `/` may cause issues |
| `./clusters/devops` | `./clusters/devops/namespaces` | Too shallow - needs `namespaces/` subdirectory |

### Issue 2: Cluster Unreachable - Kustomization Fails to Apply

**Symptoms:**
- Kustomization status shows "Health check failed"
- Secrets exist in Git but are not applied to Kubernetes
- Error: `unable to connect to cluster: connection refused`

**Cause:**
The Kubernetes cluster is unreachable or the kubeconfig is misconfigured.

**Diagnosis:**
```bash
# Test cluster connectivity
kubectl --context <cluster-name> get nodes

# Check FluxCD logs for connection errors
kubectl -n flux-system logs deployment/kustomize-controller | grep -i "connection\|unreachable\|timeout"

# Verify kubeconfig is correct
kubectl config get-contexts
kubectl config use-context <cluster-name>
```

**Solution:**

**For clusters in the same network:**
```bash
# Ensure kubeconfig is accessible to FluxCD
kubectl -n flux-system create secret generic kubeconfig \
  --from-file=config=$HOME/.kube/config

# Update Kustomization to use kubeconfig secret (if needed)
# Note: Default behavior uses in-cluster authentication
```

**For remote clusters:**
```bash
# Verify network connectivity
ping <cluster-api-endpoint>
telnet <cluster-api-endpoint> 6443

# Check firewall rules allow FluxCD to reach cluster API
# Check VPN/VPC peering is configured correctly
```

**For permission issues:**
```bash
# Verify FluxCD service account has permissions
kubectl -n flux-system get serviceaccount flux-system -o yaml
kubectl auth can-i create secrets --as=system:serviceaccount:flux-system:flux-system -n default
```

**Workaround:**
If a cluster is temporarily unreachable, FluxCD will:
- Skip reconciliation for that cluster
- Continue reconciling other healthy clusters
- Retry automatically on the next interval (1 minute by default)

### Issue 3: Secret Format Errors - SOPS Decryption Fails

**Symptoms:**
- Kustomization status shows "decryption failed"
- Error: `MAC mismatch` or `failed to decrypt: no key could decrypt the data`
- Secrets remain encrypted in Kubernetes (base64 of encrypted YAML instead of decrypted values)

**Cause:**
- SOPS Age key is incorrect or missing
- Secret was encrypted with a different Age key
- Secret YAML is malformed

**Diagnosis:**
```bash
# Check if SOPS Age key exists in flux-system namespace
kubectl -n flux-system get secret sops-age -o jsonpath='{.data.age\.agekey}' | base64 -d

# Test manual decryption with the same key
export SOPS_AGE_KEY_FILE=/home/nk/secret-manager/dev-data/age-keys/keys.txt
sops -d dev-data/secrets-repo/clusters/devops/namespaces/production/secrets/db-credentials.yaml

# Check Kustomization controller logs for decryption errors
kubectl -n flux-system logs deployment/kustomize-controller | grep -i "sops\|decrypt\|age"
```

**Solution:**

**Key mismatch:**
```bash
# Re-create sops-age secret with correct Age private key
kubectl -n flux-system delete secret sops-age
kubectl -n flux-system create secret generic sops-age \
  --from-file=age.agekey=/path/to/correct/keys.txt
```

**Re-encrypt secrets with correct key:**
```bash
# Get Age public key from private key
age-keygen -y /path/to/keys.txt

# Re-encrypt all secrets with new key
find dev-data/secrets-repo/clusters -name "*.yaml" -exec sops updatekeys {} \;
```

**Malformed YAML:**
```bash
# Validate YAML syntax
sops -d secret.yaml | kubectl apply --dry-run=client -f -

# Check for common issues:
# - Missing 'sops' metadata block
# - Invalid base64 encoding
# - Incorrect YAML indentation
```

### Issue 4: Kubeconfig Issues - Multiple Clusters Not Recognized

**Symptoms:**
- FluxCD only reconciles one cluster despite multiple Kustomizations
- Error: `cluster not found in kubeconfig`
- Wrong cluster receives secrets intended for another cluster

**Cause:**
- FluxCD is installed in a single cluster and attempts to manage multiple clusters
- Kubeconfig contexts are not properly configured
- Misunderstanding: Each cluster needs **its own FluxCD installation**

**Important Concept:**
FluxCD operates **per-cluster**. To manage 6 clusters, you need:
- 6 separate FluxCD installations (one per cluster)
- Each FluxCD instance watches the same Git repository
- Each FluxCD instance applies only the Kustomization for its cluster

**Solution:**

```bash
# Install FluxCD on EACH cluster separately
# Cluster 1: DevOps
kubectl config use-context devops
flux bootstrap github \
  --owner=<github-org> \
  --repository=fleet-infra \
  --path=clusters/devops

# Cluster 2: Integraciones Dev
kubectl config use-context integraciones-dev
flux bootstrap github \
  --owner=<github-org> \
  --repository=fleet-infra \
  --path=clusters/integraciones-dev

# Repeat for all 6 clusters...
```

**Correct Multi-Cluster Architecture:**

```
┌─────────────────────────────────────────────────────┐
│           Git Repository (Single Source)            │
│  clusters/                                          │
│  ├── devops/namespaces/                            │
│  ├── integraciones-dev/namespaces/                 │
│  ├── integraciones-stg/namespaces/                 │
│  ├── integraciones-pro/namespaces/                 │
│  ├── kyndryl-dev-stg/namespaces/                   │
│  └── kyndryl-pro/namespaces/                       │
└─────────────────────────────────────────────────────┘
                       │
         ┌─────────────┴─────────────┬────────────┬─────────────┐
         │                           │            │             │
    ┌────▼─────┐            ┌────────▼───┐  ┌─────▼─────┐  ┌──▼──────┐
    │ DevOps   │            │ Integ-Dev  │  │ Integ-Stg │  │  ...    │
    │ Cluster  │            │ Cluster    │  │ Cluster   │  │         │
    │          │            │            │  │           │  │         │
    │ FluxCD   │            │  FluxCD    │  │  FluxCD   │  │ FluxCD  │
    │ (watches │            │ (watches   │  │ (watches  │  │(watches)│
    │  devops/)│            │  integ-dev)│  │ integ-stg)│  │         │
    └──────────┘            └────────────┘  └───────────┘  └─────────┘
```

### Issue 5: Secrets Not Updating After Git Commit

**Symptoms:**
- Commit pushed to Git but secrets unchanged in Kubernetes
- `flux get sources git` shows old revision
- Logs show no reconciliation activity

**Cause:**
- Git polling interval not reached yet (default: 1 minute)
- GitRepository resource not configured correctly
- Branch mismatch (watching `main` but pushing to `master`)

**Diagnosis:**
```bash
# Check GitRepository status and last synced revision
flux get sources git secrets-repo -o yaml

# Compare with actual Git HEAD
git ls-remote <repo-url> HEAD

# Check source-controller logs
kubectl -n flux-system logs deployment/source-controller | tail -n 50
```

**Solution:**

**Force immediate reconciliation:**
```bash
flux reconcile source git secrets-repo --with-source
flux reconcile kustomization secrets-devops
```

**Reduce polling interval (faster updates):**
```yaml
# In flux-config/gitrepository.yaml
spec:
  interval: 30s  # Check every 30 seconds instead of 1 minute
```

**Configure Git webhook (instant updates):**
```bash
# Generate webhook token
flux create receiver secrets-repo \
  --type=github \
  --event=ping \
  --event=push \
  --secret-ref=webhook-token

# Add webhook URL to GitHub repository settings
# URL: https://<flux-ingress>/hook/<receiver-id>
```

**Verify branch configuration:**
```yaml
# In flux-config/gitrepository.yaml
spec:
  ref:
    branch: main  # Ensure this matches your default branch
```

### Issue 6: Secrets Applied to Wrong Namespace

**Symptoms:**
- Secrets appear in `default` namespace instead of intended namespace (e.g., `production`)
- Error: `namespace not found` despite existing in Git structure

**Cause:**
- Kustomization's `spec.targetNamespace` overrides namespace in secret YAML
- Secret YAML missing `metadata.namespace` field

**Diagnosis:**
```bash
# Check where secrets are being created
kubectl get secrets -A | grep <secret-name>

# Inspect Kustomization configuration
kubectl -n flux-system get kustomization secrets-devops -o yaml | grep targetNamespace
```

**Solution:**

**Option 1: Remove targetNamespace (recommended):**
```yaml
# Let secrets use namespace defined in their YAML
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-devops
spec:
  # Remove or comment out targetNamespace
  # targetNamespace: default
```

**Option 2: Ensure secrets define namespace explicitly:**
```yaml
# In clusters/devops/namespaces/production/secrets/db-credentials.yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  namespace: production  # Must be present
```

**Option 3: Use per-namespace Kustomizations:**
```yaml
# Create separate Kustomization for each namespace
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-devops-production
spec:
  path: ./clusters/devops/namespaces/production
  targetNamespace: production
```

### Debugging Commands Reference

```bash
# View all FluxCD resources
flux get all

# Check GitRepository sync status
flux get sources git

# Check Kustomization status for specific cluster
flux get kustomizations secrets-devops

# View detailed Kustomization status
kubectl -n flux-system describe kustomization secrets-devops

# Force reconciliation (useful for testing)
flux reconcile source git secrets-repo
flux reconcile kustomization secrets-devops --with-source

# View FluxCD controller logs
kubectl -n flux-system logs deployment/source-controller -f
kubectl -n flux-system logs deployment/kustomize-controller -f

# Check SOPS decryption key
kubectl -n flux-system get secret sops-age -o yaml

# List all secrets across namespaces (verify deployment)
kubectl get secrets -A | grep -v "default-token\|kube-"

# Check FluxCD events for errors
kubectl -n flux-system get events --sort-by='.lastTimestamp' | tail -n 20

# Verify FluxCD health
flux check --pre

# Suspend/resume reconciliation (useful for maintenance)
flux suspend kustomization secrets-devops
flux resume kustomization secrets-devops
```
