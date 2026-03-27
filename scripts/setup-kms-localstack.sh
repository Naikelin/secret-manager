#!/bin/bash
set -e

# Set dummy AWS credentials for LocalStack
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

echo "==> Waiting for LocalStack KMS to be ready..."
until curl -s http://localhost:4566/_localstack/health | grep -q '"kms": "available"'; do
  echo "Waiting for KMS service..."
  sleep 2
done

echo "==> Creating KMS key..."
# Use awslocal if available on host, otherwise use docker exec or aws with endpoint
if command -v awslocal &> /dev/null; then
  KEY_ID=$(awslocal kms create-key --description "SOPS encryption key" --query 'KeyMetadata.KeyId' --output text)
  awslocal kms create-alias --alias-name alias/sops-secrets --target-key-id "$KEY_ID"
  KMS_ARN=$(awslocal kms describe-key --key-id "$KEY_ID" --query 'KeyMetadata.Arn' --output text)
elif command -v docker &> /dev/null && docker ps --filter "name=secretmanager-localstack" --format "{{.Names}}" | grep -q secretmanager-localstack; then
  # Use AWS CLI inside LocalStack container
  KEY_ID=$(docker exec -e AWS_ACCESS_KEY_ID=test -e AWS_SECRET_ACCESS_KEY=test secretmanager-localstack aws --endpoint-url=http://localhost:4566 kms create-key --region us-east-1 --description "SOPS encryption key" --query 'KeyMetadata.KeyId' --output text)
  docker exec -e AWS_ACCESS_KEY_ID=test -e AWS_SECRET_ACCESS_KEY=test secretmanager-localstack aws --endpoint-url=http://localhost:4566 kms create-alias --region us-east-1 --alias-name alias/sops-secrets --target-key-id "$KEY_ID"
  KMS_ARN=$(docker exec -e AWS_ACCESS_KEY_ID=test -e AWS_SECRET_ACCESS_KEY=test secretmanager-localstack aws --endpoint-url=http://localhost:4566 kms describe-key --region us-east-1 --key-id "$KEY_ID" --query 'KeyMetadata.Arn' --output text)
elif command -v aws &> /dev/null; then
  KEY_ID=$(aws --endpoint-url=http://localhost:4566 kms create-key --region us-east-1 --description "SOPS encryption key" --query 'KeyMetadata.KeyId' --output text)
  aws --endpoint-url=http://localhost:4566 kms create-alias --region us-east-1 --alias-name alias/sops-secrets --target-key-id "$KEY_ID"
  KMS_ARN=$(aws --endpoint-url=http://localhost:4566 kms describe-key --region us-east-1 --key-id "$KEY_ID" --query 'KeyMetadata.Arn' --output text)
else
  echo "ERROR: No AWS CLI found. Please install awscli-local, aws-cli, or ensure LocalStack container is running."
  exit 1
fi

echo "==> KMS Key created successfully!"
echo "Key ID: $KEY_ID"
echo "ARN: $KMS_ARN"

# Save ARN for reference
mkdir -p dev-data
echo "$KMS_ARN" > dev-data/kms-arn.txt
echo "==> ARN saved to dev-data/kms-arn.txt"
