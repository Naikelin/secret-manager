#!/bin/bash
# sync-secrets-to-k8s.sh
# Manually sync SOPS-encrypted secrets to Kubernetes cluster
# This simulates what FluxCD would do automatically

set -e

SECRETS_REPO="/home/nk/secret-manager/dev-data/secrets-repo"
AGE_KEY_FILE="/home/nk/secret-manager/dev-data/age-keys/keys.txt"

export SOPS_AGE_KEY_FILE="$AGE_KEY_FILE"

echo "🔄 Syncing secrets from $SECRETS_REPO to Kubernetes..."

# Function to sync secrets for a namespace
sync_namespace() {
    local ns=$1
    local secrets_dir="$SECRETS_REPO/namespaces/$ns/secrets"
    
    if [ ! -d "$secrets_dir" ]; then
        echo "⚠️  No secrets directory found for namespace: $ns"
        return
    fi
    
    # Create namespace if it doesn't exist
    kubectl create namespace "$ns" --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1
    
    # Sync all YAML files in the secrets directory
    for secret_file in "$secrets_dir"/*.yaml; do
        if [ -f "$secret_file" ]; then
            local secret_name=$(basename "$secret_file" .yaml)
            echo "  📦 Syncing $ns/$secret_name"
            sops -d "$secret_file" | kubectl apply -f - > /dev/null 2>&1
        fi
    done
}

# Sync all namespaces
for ns_dir in "$SECRETS_REPO/namespaces"/*; do
    if [ -d "$ns_dir" ]; then
        ns=$(basename "$ns_dir")
        echo "📂 Namespace: $ns"
        sync_namespace "$ns"
    fi
done

echo "✅ Sync complete!"
echo ""
echo "To verify secrets:"
echo "  kubectl get secrets -A | grep -E 'development|staging|production'"
