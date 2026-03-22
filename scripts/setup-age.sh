#!/bin/bash
set -e

CLUSTER_NAME="secretmanager"
AGE_KEY_FILE="./dev-data/age-key.txt"

echo "🔑 Setting up Age encryption keys"

# Create dev-data directory
mkdir -p ./dev-data

# Generate Age key if it doesn't exist
if [ ! -f "$AGE_KEY_FILE" ]; then
  echo "📝 Generating new Age key pair..."
  age-keygen -o "$AGE_KEY_FILE"
  echo "✅ Age key generated: $AGE_KEY_FILE"
else
  echo "✅ Age key already exists: $AGE_KEY_FILE"
fi

# Extract public key
PUBLIC_KEY=$(grep "# public key:" "$AGE_KEY_FILE" | cut -d' ' -f4)
echo "🔓 Public key: $PUBLIC_KEY"

# Create .sops.yaml config
cat > .sops.yaml <<EOF
creation_rules:
  - path_regex: .*\.yaml$
    encrypted_regex: ^(data|stringData)$
    age: $PUBLIC_KEY
EOF

echo "✅ Created .sops.yaml with Age public key"

# Set kubectl context
kubectl config use-context kind-$CLUSTER_NAME 2>/dev/null || true

# Create Age secret in Kubernetes (if cluster exists)
if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
  kubectl create namespace flux-system --dry-run=client -o yaml | kubectl apply -f -
  
  kubectl create secret generic sops-age \
    --from-file=age.agekey=$AGE_KEY_FILE \
    --namespace=flux-system \
    --dry-run=client -o yaml | kubectl apply -f -
  
  echo "✅ Age key stored in Kubernetes secret: flux-system/sops-age"
else
  echo "⚠️  Kind cluster not found. Age key NOT uploaded to Kubernetes."
  echo "   Run setup-kind.sh first, then re-run this script."
fi

echo ""
echo "🎉 Age setup complete!"
echo "   Private key: $AGE_KEY_FILE"
echo "   Public key: $PUBLIC_KEY"
echo "   SOPS config: .sops.yaml"
