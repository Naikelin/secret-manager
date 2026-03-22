#!/bin/bash
set -e

CLUSTER_NAME="secretmanager"

echo "🚀 Creating Kind cluster: $CLUSTER_NAME"

# Check if cluster exists
if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
  echo "⚠️  Cluster $CLUSTER_NAME already exists"
  read -p "Delete and recreate? (y/N): " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    kind delete cluster --name $CLUSTER_NAME
  else
    echo "✅ Using existing cluster"
    exit 0
  fi
fi

# Create cluster with custom config
cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
EOF

echo "✅ Kind cluster created successfully"
echo "📋 Cluster info:"
kubectl cluster-info --context kind-$CLUSTER_NAME
