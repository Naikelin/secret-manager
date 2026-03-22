#!/bin/bash
set -e

# Bootstrap FluxCD with GitRepository, Age Secret, and Kustomizations
# This script creates the necessary FluxCD resources to watch the secrets Git repository

echo "🚀 Bootstrapping FluxCD for Secret Manager"

# Check required environment variables
if [ -z "$GIT_REPO_URL" ]; then
  echo "❌ Error: GIT_REPO_URL environment variable is required"
  echo "   Example: export GIT_REPO_URL='file:///path/to/dev-data/secrets-repo'"
  exit 1
fi

if [ -z "$AGE_KEY_FILE" ]; then
  echo "❌ Error: AGE_KEY_FILE environment variable is required"
  echo "   Example: export AGE_KEY_FILE='dev-data/age-keys/keys.txt'"
  exit 1
fi

if [ ! -f "$AGE_KEY_FILE" ]; then
  echo "❌ Error: Age key file not found at $AGE_KEY_FILE"
  exit 1
fi

# Check if FluxCD is installed
if ! kubectl get namespace flux-system &> /dev/null; then
  echo "❌ Error: FluxCD is not installed. Run scripts/setup-flux.sh first."
  exit 1
fi

echo "📦 Creating GitRepository resource..."

# Create GitRepository resource
kubectl apply -f - <<EOF
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: secrets-repo
  namespace: flux-system
spec:
  interval: 1m
  url: ${GIT_REPO_URL}
  ref:
    branch: main
EOF

echo "🔑 Creating SOPS Age Secret..."

# Read Age private key and base64 encode it
AGE_KEY_BASE64=$(cat "$AGE_KEY_FILE" | base64 -w 0)

# Create Age secret for SOPS decryption
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: sops-age
  namespace: flux-system
type: Opaque
data:
  age.agekey: ${AGE_KEY_BASE64}
EOF

echo "📝 Creating Kustomizations for namespaces..."

# Define namespaces to create Kustomizations for
NAMESPACES=("development" "staging" "production")

for NS in "${NAMESPACES[@]}"; do
  echo "   - Creating Kustomization for namespace: $NS"
  
  kubectl apply -f - <<EOF
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: secrets-${NS}
  namespace: flux-system
spec:
  interval: 1m
  path: ./namespaces/${NS}/secrets
  prune: true
  sourceRef:
    kind: GitRepository
    name: secrets-repo
  decryption:
    provider: sops
    secretRef:
      name: sops-age
EOF
done

echo "⏳ Waiting for GitRepository to be ready..."
kubectl wait --for=condition=ready gitrepository/secrets-repo -n flux-system --timeout=60s || true

echo "✅ FluxCD bootstrap complete!"
echo ""
echo "📊 Check status with:"
echo "   kubectl get gitrepositories -n flux-system"
echo "   kubectl get kustomizations -n flux-system"
echo "   kubectl describe kustomization secrets-development -n flux-system"
