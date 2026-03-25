#!/bin/bash
# setup-flux-gitops.sh
# Setup FluxCD to automatically sync secrets from Git repository
# Prerequisites:
#   - Flux installed (flux install)
#   - SOPS Age key configured in flux-system namespace
#   - Git repository accessible by the cluster

set -e

NAMESPACE="flux-system"
SECRETS_REPO_URL="${SECRETS_REPO_URL:-}"
SECRETS_REPO_BRANCH="${SECRETS_REPO_BRANCH:-main}"

if [ -z "$SECRETS_REPO_URL" ]; then
    echo "❌ Error: SECRETS_REPO_URL environment variable not set"
    echo ""
    echo "Usage:"
    echo "  export SECRETS_REPO_URL=https://github.com/yourorg/secrets-repo.git"
    echo "  export SECRETS_REPO_BRANCH=main  # optional, defaults to main"
    echo "  ./setup-flux-gitops.sh"
    echo ""
    echo "For local development with git-daemon:"
    echo "  1. Start git daemon in secrets-repo directory:"
    echo "     cd /home/nk/secret-manager/dev-data/secrets-repo"
    echo "     git daemon --reuseaddr --base-path=. --export-all --verbose --enable=receive-pack"
    echo ""
    echo "  2. In another terminal:"
    echo "     export SECRETS_REPO_URL=git://host.docker.internal/secrets-repo"
    echo "     ./setup-flux-gitops.sh"
    exit 1
fi

echo "🚀 Setting up FluxCD GitOps for secrets"
echo "   Repository: $SECRETS_REPO_URL"
echo "   Branch: $SECRETS_REPO_BRANCH"
echo ""

# Check if Flux is installed
if ! flux check > /dev/null 2>&1; then
    echo "❌ Flux is not installed or not healthy"
    echo "   Run: flux install"
    exit 1
fi

# Check if SOPS Age key secret exists
if ! kubectl -n flux-system get secret sops-age > /dev/null 2>&1; then
    echo "❌ SOPS Age key secret not found in flux-system namespace"
    echo "   Run: kubectl create secret generic sops-age \\"
    echo "          --namespace=flux-system \\"
    echo "          --from-file=age.agekey=/path/to/keys.txt"
    exit 1
fi

echo "✅ Prerequisites check passed"
echo ""

# Create GitRepository resource
echo "📝 Creating GitRepository resource..."
cat <<EOF | kubectl apply -f -
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: secrets-repo
  namespace: $NAMESPACE
spec:
  interval: 1m
  url: $SECRETS_REPO_URL
  ref:
    branch: $SECRETS_REPO_BRANCH
  timeout: 60s
EOF

# Create Kustomization resource with SOPS decryption
echo "📝 Creating Kustomization resource with SOPS decryption..."
cat <<EOF | kubectl apply -f -
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets
  namespace: $NAMESPACE
spec:
  interval: 1m
  path: "./namespaces"
  prune: true
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  timeout: 2m
EOF

echo ""
echo "✅ FluxCD GitOps setup complete!"
echo ""
echo "To monitor the sync:"
echo "  flux get sources git"
echo "  flux get kustomizations"
echo "  kubectl -n flux-system logs -l app=kustomize-controller -f"
echo ""
echo "To verify secrets are synced:"
echo "  kubectl get secrets -A | grep -E 'development|staging|production'"
