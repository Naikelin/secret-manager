#!/bin/bash
set -e

# Setup multiple Kind clusters for multi-cluster testing
# This script creates 3 lightweight K8s clusters to simulate production topology

echo "=== Setting up Kind clusters for multi-cluster testing ==="

# Cluster configurations
CLUSTERS=(
  "devops:8081"
  "integraciones-dev:8082"
  "integraciones-stg:8083"
)

# Create each cluster
for cluster_config in "${CLUSTERS[@]}"; do
  CLUSTER_NAME=$(echo $cluster_config | cut -d: -f1)
  API_PORT=$(echo $cluster_config | cut -d: -f2)
  
  echo ""
  echo "--- Creating cluster: $CLUSTER_NAME (API port: $API_PORT) ---"
  
  # Check if cluster already exists
  if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "⚠️  Cluster $CLUSTER_NAME already exists, skipping..."
    continue
  fi
  
  # Create Kind cluster config
  cat > /tmp/kind-${CLUSTER_NAME}.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${CLUSTER_NAME}
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: ${API_PORT}
    protocol: TCP
EOF

  # Create the cluster
  kind create cluster --config /tmp/kind-${CLUSTER_NAME}.yaml
  
  # Clean up temp config
  rm /tmp/kind-${CLUSTER_NAME}.yaml
  
  echo "✅ Cluster $CLUSTER_NAME created"
done

echo ""
echo "=== Kind clusters created successfully ==="
echo ""
echo "Available contexts:"
kubectl config get-contexts | grep kind-

echo ""
echo "=== Next steps ==="
echo "1. Install FluxCD in each cluster: ./scripts/bootstrap-flux-kind.sh"
echo "2. Update backend to support multiple kubeconfigs"
echo "3. Test multi-cluster operations"
