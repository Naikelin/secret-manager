#!/bin/bash
# Deploy Secret Manager to Kubernetes using kubectl
set -e

NAMESPACE=${NAMESPACE:-secret-manager}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "🚀 Deploying Secret Manager to Kubernetes..."
echo "   Namespace: $NAMESPACE"
echo "   Manifests: $PROJECT_ROOT/k8s"

# Create namespace
echo ""
echo "📦 Creating namespace..."
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Apply manifests
echo ""
echo "📋 Applying Kubernetes manifests..."
kubectl apply -f "$PROJECT_ROOT/k8s/" -n $NAMESPACE

# Wait for PostgreSQL to be ready
echo ""
echo "⏳ Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres -n $NAMESPACE --timeout=300s

# Wait for backend rollout
echo ""
echo "⏳ Waiting for backend deployment..."
kubectl rollout status deployment/backend -n $NAMESPACE --timeout=300s

# Wait for frontend rollout
echo ""
echo "⏳ Waiting for frontend deployment..."
kubectl rollout status deployment/frontend -n $NAMESPACE --timeout=300s

# Show deployment status
echo ""
echo "✅ Deployment complete!"
echo ""
echo "📊 Current status:"
kubectl get pods -n $NAMESPACE

echo ""
echo "🌐 Access the application:"
echo "   Port forward: kubectl port-forward -n $NAMESPACE svc/frontend 3000:3000"
echo "   Then open: http://localhost:3000"
echo ""
echo "📝 View logs:"
echo "   Backend:  kubectl logs -n $NAMESPACE -l app=backend -f"
echo "   Frontend: kubectl logs -n $NAMESPACE -l app=frontend -f"
