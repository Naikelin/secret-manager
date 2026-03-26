#!/bin/bash
set -e

echo "===================================="
echo "GCP Cloud Run Deployment Script"
echo "===================================="

# Configuration
PROJECT_ID=$(gcloud config get-value project)
REGION=${GCP_REGION:-us-central1}
PROJECT_NAME="secret-manager"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Project ID: ${PROJECT_ID}"
echo "Region: ${REGION}"

echo -e "${YELLOW}Step 1: Building and pushing backend...${NC}"
cd ../backend
gcloud builds submit \
  --tag ${REGION}-docker.pkg.dev/${PROJECT_ID}/${PROJECT_NAME}/backend:latest \
  --project ${PROJECT_ID}

echo -e "${GREEN}✓ Backend image built and pushed${NC}"

echo -e "${YELLOW}Step 2: Building and pushing frontend...${NC}"
cd ../frontend
gcloud builds submit \
  --tag ${REGION}-docker.pkg.dev/${PROJECT_ID}/${PROJECT_NAME}/frontend:latest \
  --build-arg NEXT_PUBLIC_API_URL=https://${DOMAIN_NAME:-localhost}/api \
  --project ${PROJECT_ID}

echo -e "${GREEN}✓ Frontend image built and pushed${NC}"

echo -e "${YELLOW}Step 3: Deploying to Cloud Run...${NC}"

# Deploy backend
gcloud run deploy ${PROJECT_NAME}-backend \
  --image ${REGION}-docker.pkg.dev/${PROJECT_ID}/${PROJECT_NAME}/backend:latest \
  --region ${REGION} \
  --project ${PROJECT_ID}

# Deploy frontend
gcloud run deploy ${PROJECT_NAME}-frontend \
  --image ${REGION}-docker.pkg.dev/${PROJECT_ID}/${PROJECT_NAME}/frontend:latest \
  --region ${REGION} \
  --project ${PROJECT_ID}

echo -e "${GREEN}✓ Services deployed to Cloud Run${NC}"

echo -e "${GREEN}===================================="
echo "Deployment complete!"
echo "====================================${NC}"

echo "Service URLs:"
gcloud run services describe ${PROJECT_NAME}-backend --region ${REGION} --format='value(status.url)' --project ${PROJECT_ID}
gcloud run services describe ${PROJECT_NAME}-frontend --region ${REGION} --format='value(status.url)' --project ${PROJECT_ID}
