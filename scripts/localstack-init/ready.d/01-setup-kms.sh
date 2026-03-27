#!/bin/bash
set -e

echo "==> LocalStack Init: Setting up KMS key for SOPS"

# Create KMS key with deterministic alias
# LocalStack doesn't support deterministic key IDs, but we can use alias consistently
KEY_ID=$(awslocal kms create-key --description "SOPS encryption key" --query 'KeyMetadata.KeyId' --output text)

echo "==> KMS Key created: $KEY_ID"

# Create alias (this is what we'll reference in config)
awslocal kms create-alias --alias-name alias/sops-secrets --target-key-id "$KEY_ID"

echo "==> KMS Key alias created: alias/sops-secrets"

# Get full ARN for logging
KMS_ARN=$(awslocal kms describe-key --key-id "$KEY_ID" --query 'KeyMetadata.Arn' --output text)

echo "==> KMS Key ARN: $KMS_ARN"
echo "==> LocalStack KMS setup complete!"
