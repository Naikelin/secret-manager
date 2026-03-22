#!/bin/bash
set -e

CLUSTER_NAME="secretmanager"

echo "🚀 Installing FluxCD on cluster: $CLUSTER_NAME"

# Check if cluster exists
if ! kind get clusters | grep -q "^$CLUSTER_NAME$"; then
  echo "❌ Cluster $CLUSTER_NAME not found. Run setup-kind.sh first."
  exit 1
fi

# Set kubectl context
kubectl config use-context kind-$CLUSTER_NAME

# Check if flux is already installed
if kubectl get namespace flux-system &> /dev/null; then
  echo "⚠️  FluxCD already installed"
  exit 0
fi

# Install flux
echo "📦 Installing FluxCD components..."
flux install

# Wait for flux to be ready
echo "⏳ Waiting for FluxCD to be ready..."
kubectl wait --for=condition=ready pod -l app=source-controller -n flux-system --timeout=120s
kubectl wait --for=condition=ready pod -l app=kustomize-controller -n flux-system --timeout=120s

echo "✅ FluxCD installed successfully"
flux --version
