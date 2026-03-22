#!/bin/bash
set -e

echo "🔐 Setting up SOPS with Age for local development"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
AGE_KEYS_DIR="${PROJECT_ROOT}/dev-data/age-keys"
AGE_KEY_FILE="${AGE_KEYS_DIR}/keys.txt"

# Check if age-keygen is installed
if ! command -v age-keygen &> /dev/null; then
    echo "❌ age-keygen is not installed. Please install it first:"
    echo "   brew install age  # macOS"
    echo "   apt install age   # Debian/Ubuntu"
    exit 1
fi

# Create Age keys directory
mkdir -p "$AGE_KEYS_DIR"

# Generate Age key if doesn't exist
if [[ ! -f "$AGE_KEY_FILE" ]]; then
  echo "📝 Generating new Age key pair..."
  age-keygen -o "$AGE_KEY_FILE"
  chmod 600 "$AGE_KEY_FILE"
  echo "✅ Age key generated: $AGE_KEY_FILE"
else
  echo "✅ Age key already exists: $AGE_KEY_FILE"
fi

# Extract public key
PUB_KEY=$(grep "# public key:" "$AGE_KEY_FILE" | awk '{print $NF}')
echo "🔓 Age public key: $PUB_KEY"

# Generate .sops.yaml in git repo (if GIT_REPO_PATH exists)
if [[ -n "$GIT_REPO_PATH" ]] && [[ -d "$GIT_REPO_PATH" ]]; then
  SOPS_CONFIG="$GIT_REPO_PATH/.sops.yaml"
  cat > "$SOPS_CONFIG" <<EOF
creation_rules:
  - path_regex: \.yaml$
    encrypted_regex: '^(data|stringData)$'
    age: $PUB_KEY
EOF
  echo "✅ .sops.yaml created in repository: $SOPS_CONFIG"
else
  echo "ℹ️  GIT_REPO_PATH not set or directory doesn't exist"
  echo "   .sops.yaml will be created in: ${PROJECT_ROOT}/dev-data/secrets-repo/.sops.yaml"
  
  # Create in default dev repo path
  DEV_REPO_PATH="${PROJECT_ROOT}/dev-data/secrets-repo"
  mkdir -p "$DEV_REPO_PATH"
  
  SOPS_CONFIG="$DEV_REPO_PATH/.sops.yaml"
  cat > "$SOPS_CONFIG" <<EOF
creation_rules:
  - path_regex: \.yaml$
    encrypted_regex: '^(data|stringData)$'
    age: $PUB_KEY
EOF
  echo "✅ .sops.yaml created in: $SOPS_CONFIG"
fi

# Create example encrypted secret for testing
EXAMPLE_SECRET_DIR="${PROJECT_ROOT}/dev-data/secrets-repo/secrets"
mkdir -p "$EXAMPLE_SECRET_DIR"

EXAMPLE_FILE="${EXAMPLE_SECRET_DIR}/example-secret.yaml"
if [[ ! -f "$EXAMPLE_FILE" ]]; then
  cat > "$EXAMPLE_FILE" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: example-secret
  namespace: default
type: Opaque
data:
  username: YWRtaW4=
  password: c2VjcmV0MTIz
EOF
  echo "✅ Example secret created: $EXAMPLE_FILE"
fi

echo ""
echo "🎉 SOPS setup complete!"
echo ""
echo "📋 Summary:"
echo "   Private key: $AGE_KEY_FILE"
echo "   Public key:  $PUB_KEY"
echo "   SOPS config: $SOPS_CONFIG"
echo ""
echo "🧪 Test encryption with:"
echo "   SOPS_AGE_KEY_FILE=$AGE_KEY_FILE sops -e $EXAMPLE_FILE"
echo ""
echo "⚠️  IMPORTANT: Never commit Age private keys to Git!"
