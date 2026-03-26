#!/bin/bash
# Deploy Secret Manager using Helm
set -e

NAMESPACE=${NAMESPACE:-secret-manager}
RELEASE=${RELEASE:-secret-manager}
VALUES_FILE=${VALUES_FILE:-}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHART_PATH="$PROJECT_ROOT/helm/secret-manager"

echo "🚀 Deploying Secret Manager with Helm..."
echo "   Release:   $RELEASE"
echo "   Namespace: $NAMESPACE"
echo "   Chart:     $CHART_PATH"

# Determine values file
if [ -z "$VALUES_FILE" ]; then
  if [ -f "$CHART_PATH/values-prod.yaml" ] && [ "$ENVIRONMENT" = "production" ]; then
    VALUES_FILE="$CHART_PATH/values-prod.yaml"
    echo "   Values:    values-prod.yaml (production)"
  else
    VALUES_FILE="$CHART_PATH/values.yaml"
    echo "   Values:    values.yaml (default)"
  fi
else
  echo "   Values:    $VALUES_FILE (custom)"
fi

# Validate chart
echo ""
echo "🔍 Validating Helm chart..."
helm lint "$CHART_PATH"

# Install or upgrade
echo ""
echo "📦 Installing/upgrading release..."
helm upgrade --install $RELEASE "$CHART_PATH" \
  --namespace $NAMESPACE \
  --create-namespace \
  --values "$VALUES_FILE" \
  --wait \
  --timeout 10m

# Show deployment status
echo ""
echo "✅ Deployment complete!"
echo ""
echo "📊 Release information:"
helm status $RELEASE -n $NAMESPACE

echo ""
echo "📝 To view resources:"
echo "   helm get all $RELEASE -n $NAMESPACE"
echo ""
echo "📝 To view logs:"
echo "   kubectl logs -n $NAMESPACE -l app=backend -f"
echo "   kubectl logs -n $NAMESPACE -l app=frontend -f"
echo ""
echo "🌐 Access the application:"
INGRESS_HOST=$(helm get values $RELEASE -n $NAMESPACE -o json | grep -o '"host":"[^"]*' | cut -d'"' -f4 || echo "")
if [ -n "$INGRESS_HOST" ]; then
  echo "   Ingress: https://$INGRESS_HOST"
else
  echo "   Port forward: kubectl port-forward -n $NAMESPACE svc/frontend 3000:3000"
  echo "   Then open: http://localhost:3000"
fi
