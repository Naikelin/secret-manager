#!/bin/bash
set -e

echo "===================================="
echo "Azure Container Apps Deployment Script"
echo "===================================="

# Configuration
RESOURCE_GROUP="${RESOURCE_GROUP:-secret-manager-rg}"
REGISTRY_NAME="${REGISTRY_NAME:-secretmanageracr}"
PROJECT_NAME="secret-manager"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Resource Group: ${RESOURCE_GROUP}"
echo "Registry: ${REGISTRY_NAME}"

echo -e "${YELLOW}Step 1: Login to ACR...${NC}"
az acr login --name ${REGISTRY_NAME}

echo -e "${GREEN}✓ Logged in to ACR${NC}"

echo -e "${YELLOW}Step 2: Building and pushing backend...${NC}"
cd ../backend
az acr build \
  --registry ${REGISTRY_NAME} \
  --image backend:latest \
  --file Dockerfile.prod \
  .

echo -e "${GREEN}✓ Backend image built and pushed${NC}"

echo -e "${YELLOW}Step 3: Building and pushing frontend...${NC}"
cd ../frontend
az acr build \
  --registry ${REGISTRY_NAME} \
  --image frontend:latest \
  --file Dockerfile.prod \
  --build-arg NEXT_PUBLIC_API_URL=https://${DOMAIN_NAME:-localhost}/api \
  .

echo -e "${GREEN}✓ Frontend image built and pushed${NC}"

echo -e "${YELLOW}Step 4: Updating Container Apps...${NC}"

# Update backend
az containerapp update \
  --name ${PROJECT_NAME}-backend \
  --resource-group ${RESOURCE_GROUP} \
  --image ${REGISTRY_NAME}.azurecr.io/backend:latest

# Update frontend
az containerapp update \
  --name ${PROJECT_NAME}-frontend \
  --resource-group ${RESOURCE_GROUP} \
  --image ${REGISTRY_NAME}.azurecr.io/frontend:latest

echo -e "${GREEN}✓ Container Apps updated${NC}"

echo -e "${GREEN}===================================="
echo "Deployment complete!"
echo "====================================${NC}"

echo "Service URLs:"
az containerapp show --name ${PROJECT_NAME}-backend --resource-group ${RESOURCE_GROUP} --query properties.configuration.ingress.fqdn -o tsv
az containerapp show --name ${PROJECT_NAME}-frontend --resource-group ${RESOURCE_GROUP} --query properties.configuration.ingress.fqdn -o tsv
