# FluxCD Setup Status for Secret Manager

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
