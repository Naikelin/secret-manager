# FluxCD Configuration for Secret Manager

This directory contains FluxCD configuration for automatically syncing SOPS-encrypted secrets to Kubernetes.

## Prerequisites

1. **Flux installed**: `flux install`
2. **SOPS Age key configured**:
   ```bash
   kubectl create secret generic sops-age \
     --namespace=flux-system \
     --from-file=age.agekey=/home/nk/secret-manager/dev-data/age-keys/keys.txt
   ```
3. **Git repository accessible by the cluster**

## Directory Structure

```
flux-config/
├── README.md              # This file
├── gitrepository.yaml     # GitRepository source configuration
└── kustomization.yaml     # Kustomization with SOPS decryption
```

## Setup

### Option 1: Using the Setup Script (Recommended)

```bash
# For remote Git repository
export SECRETS_REPO_URL=https://github.com/yourorg/secrets-repo.git
export SECRETS_REPO_BRANCH=main
./scripts/setup-flux-gitops.sh
```

### Option 2: Manual Application

```bash
kubectl apply -f flux-config/
```

### Option 3: Local Development (Git Daemon)

For local development without a remote Git server:

1. Start git daemon in secrets-repo:
   ```bash
   cd /home/nk/secret-manager/dev-data/secrets-repo
   git daemon --reuseaddr --base-path=. --export-all --verbose --enable=receive-pack &
   ```

2. Update `flux-config/gitrepository.yaml` URL to:
   ```yaml
   url: git://host.docker.internal/secrets-repo
   ```

3. Apply configuration:
   ```bash
   kubectl apply -f flux-config/
   ```

## Monitoring

Check GitRepository sync status:
```bash
flux get sources git
kubectl -n flux-system get gitrepository secrets-repo
```

Check Kustomization status:
```bash
flux get kustomizations
kubectl -n flux-system get kustomization secrets
```

View controller logs:
```bash
kubectl -n flux-system logs -l app=source-controller -f
kubectl -n flux-system logs -l app=kustomize-controller -f
```

## Verify Secrets

Check that secrets are decrypted and applied:
```bash
kubectl get secrets -A | grep -E 'development|staging|production'
kubectl -n development get secret db-credentials -o yaml
```

## Manual Sync Alternative

If FluxCD is not configured with a Git remote, you can manually sync secrets:

```bash
./scripts/sync-secrets-to-k8s.sh
```

This script:
- Reads all YAML files from `dev-data/secrets-repo/namespaces/*/secrets/`
- Decrypts them using SOPS with the Age key
- Applies them to the Kubernetes cluster

## Troubleshooting

### Secrets not syncing

1. Check GitRepository is healthy:
   ```bash
   flux get sources git secrets-repo
   ```

2. Check Kustomization events:
   ```bash
   kubectl -n flux-system describe kustomization secrets
   ```

3. Check SOPS decryption is working:
   ```bash
   # Manual test
   export SOPS_AGE_KEY_FILE=/home/nk/secret-manager/dev-data/age-keys/keys.txt
   sops -d dev-data/secrets-repo/namespaces/development/secrets/db-credentials.yaml
   ```

### Authentication errors

If using a private Git repository:
```bash
# Create Git credentials secret
kubectl create secret generic git-credentials \
  --namespace=flux-system \
  --from-literal=username=your-username \
  --from-literal=password=your-token

# Update gitrepository.yaml to reference the secret
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Git Repository                           │
│  (SOPS-encrypted secrets, one per namespace)                 │
└────────────────────────────┬────────────────────────────────┘
                             │
                             │ Flux watches (1m interval)
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                 Flux Source Controller                       │
│  (Clones repo, detects changes)                              │
└────────────────────────────┬────────────────────────────────┘
                             │
                             │ Provides source to
                             ▼
┌─────────────────────────────────────────────────────────────┐
│              Flux Kustomize Controller                       │
│  (Decrypts with SOPS + Age key, applies to cluster)         │
└────────────────────────────┬────────────────────────────────┘
                             │
                             │ Creates/Updates
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                  Kubernetes Secrets                          │
│  (Decrypted, ready for pods to consume)                     │
└─────────────────────────────────────────────────────────────┘
```

## Integration with Secret Manager API

1. User edits secret via UI → Backend creates draft
2. User clicks "Publish" → Backend writes encrypted YAML to Git, commits & pushes
3. Flux detects Git change (within 1 minute)
4. Flux decrypts secret using SOPS + Age key
5. Flux applies secret to Kubernetes cluster
6. Pods can consume the secret via environment variables or volume mounts

## Next Steps

- [ ] Configure Git repository remote (GitHub, GitLab, etc.)
- [ ] Set up Git credentials for private repos (if needed)
- [ ] Configure webhook receiver for instant sync (optional)
- [ ] Set up Flux alerts for failed syncs (optional)
