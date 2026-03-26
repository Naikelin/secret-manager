#!/bin/bash
set -e

echo "===================================="
echo "AWS ECS Fargate Deployment Script"
echo "===================================="

# Configuration
REGION=${AWS_REGION:-us-east-1}
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
PROJECT_NAME="secret-manager"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Step 1: Building Docker images...${NC}"

# Build backend
cd ../backend
echo "Building backend image..."
docker build -t ${PROJECT_NAME}-backend:latest -f Dockerfile.prod .

# Build frontend
cd ../frontend
echo "Building frontend image..."
docker build -t ${PROJECT_NAME}-frontend:latest -f Dockerfile.prod \
  --build-arg NEXT_PUBLIC_API_URL=https://${DOMAIN_NAME:-localhost}/api .

echo -e "${GREEN}✓ Images built successfully${NC}"

echo -e "${YELLOW}Step 2: Login to ECR...${NC}"
aws ecr get-login-password --region ${REGION} | \
  docker login --username AWS --password-stdin ${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com

echo -e "${GREEN}✓ Logged in to ECR${NC}"

echo -e "${YELLOW}Step 3: Tag and push images...${NC}"

# Tag and push backend
docker tag ${PROJECT_NAME}-backend:latest \
  ${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${PROJECT_NAME}/backend:latest
docker push ${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${PROJECT_NAME}/backend:latest

# Tag and push frontend
docker tag ${PROJECT_NAME}-frontend:latest \
  ${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${PROJECT_NAME}/frontend:latest
docker push ${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${PROJECT_NAME}/frontend:latest

echo -e "${GREEN}✓ Images pushed to ECR${NC}"

echo -e "${YELLOW}Step 4: Update ECS services...${NC}"

# Force new deployment of backend
aws ecs update-service \
  --cluster ${PROJECT_NAME}-cluster \
  --service ${PROJECT_NAME}-backend \
  --force-new-deployment \
  --region ${REGION}

# Force new deployment of frontend
aws ecs update-service \
  --cluster ${PROJECT_NAME}-cluster \
  --service ${PROJECT_NAME}-frontend \
  --force-new-deployment \
  --region ${REGION}

echo -e "${GREEN}✓ ECS services updated${NC}"

echo -e "${GREEN}===================================="
echo "Deployment complete!"
echo "====================================${NC}"

echo "Check deployment status:"
echo "  aws ecs describe-services --cluster ${PROJECT_NAME}-cluster --services ${PROJECT_NAME}-backend --region ${REGION}"
echo "  aws ecs describe-services --cluster ${PROJECT_NAME}-cluster --services ${PROJECT_NAME}-frontend --region ${REGION}"
