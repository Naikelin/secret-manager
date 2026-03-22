# SOPS Integration Documentation

## Overview

**SOPS** (Secrets OPerationS) is Mozilla's encryption tool for YAML, JSON, and other configuration files. This project uses SOPS to encrypt Kubernetes Secrets before committing them to Git, ensuring secrets are never stored in plaintext.

## Why SOPS?

- **GitOps-Native**: FluxCD has built-in SOPS support for automatic decryption
- **Partial Encryption**: Only encrypts secret values, keeps structure readable
- **Multiple Key Providers**: Supports Age (development), AWS KMS (production), GCP KMS, Azure Key Vault
- **Auditable**: Encrypted files can still be diffed in Git (structure changes visible)
- **Open Source**: Battle-tested by major organizations

## Age vs KMS Comparison

| Feature | Age (Dev/Self-Hosted) | AWS KMS (Production) |
|---------|----------------------|----------------------|
| **Cost** | Free | Pay per API call (~$0.03/10k requests) |
| **Setup Complexity** | Simple (generate keypair) | Requires AWS account, IAM setup |
| **Key Rotation** | Manual (regenerate + re-encrypt) | Automatic with KMS key rotation |
| **Audit Trail** | None (file-based keys) | Full CloudTrail logs |
| **Multi-Region** | Manual key distribution | Built-in replication |
| **Best For** | Local dev, small teams | Production, compliance requirements |

### When to Use Each

**Use Age for:**
- Local development
- CI/CD pipelines (with key in secrets manager)
- Self-hosted environments without cloud KMS
- Small teams without compliance requirements

**Use AWS KMS for:**
- Production environments
- Multi-region deployments
- Compliance requirements (SOC2, HIPAA, etc.)
- Automatic key rotation needs
- Integration with AWS IAM

## Setup Instructions

### Development Setup (Age)

1. **Install Age and SOPS**:
   ```bash
   # macOS
   brew install age sops
   
   # Ubuntu/Debian
   apt install age
   # SOPS: Download from https://github.com/getsops/sops/releases
   ```

2. **Generate Age Keys**:
   ```bash
   make setup-sops
   ```
   
   This creates:
   - `dev-data/age-keys/keys.txt` - Private key (NEVER commit!)
   - `dev-data/secrets-repo/.sops.yaml` - SOPS config with public key

3. **Verify Setup**:
   ```bash
   # Test SOPS installation
   sops --version
   
   # Run SOPS tests
   make test-sops
   ```

### Production Setup (AWS KMS)

1. **Create KMS Key in AWS**:
   ```bash
   aws kms create-key \
     --description "Secret Manager SOPS encryption key" \
     --region us-east-1
   
   # Save the key ARN (e.g., arn:aws:kms:us-east-1:123456789:key/abc-def-123)
   ```

2. **Grant Access to FluxCD**:
   ```bash
   # Get FluxCD service account ARN from EKS cluster
   kubectl get sa -n flux-system flux -o jsonpath='{.metadata.annotations.eks\.amazonaws\.com/role-arn}'
   
   # Add IAM policy to allow decrypt
   aws kms create-grant \
     --key-id <KMS_KEY_ID> \
     --grantee-principal <FLUX_SA_ARN> \
     --operations Decrypt
   ```

3. **Configure Backend**:
   ```bash
   # Set environment variables
   export SOPS_ENABLED=true
   export SOPS_ENCRYPT_TYPE=kms
   export SOPS_KMS_KEY_ARN=arn:aws:kms:us-east-1:123456789:key/abc-def-123
   
   # Backend will use KMS for encryption
   ```

4. **Update .sops.yaml in Git Repo**:
   ```yaml
   creation_rules:
     - path_regex: \.yaml$
       encrypted_regex: '^(data|stringData)$'
       kms: arn:aws:kms:us-east-1:123456789:key/abc-def-123
   ```

## Encryption Workflow

```
┌─────────────────┐
│  User Creates   │
│  Secret Draft   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  PostgreSQL     │  ← Draft stored in DB (plaintext, not in Git yet)
│  Staging Area   │
└────────┬────────┘
         │
         │ User clicks "Publish"
         ▼
┌─────────────────┐
│  Generate K8s   │
│  Secret YAML    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  SOPS Encrypt   │  ← Only data/stringData fields encrypted
│  (partial)      │    Metadata stays readable
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Git Commit     │  ← Encrypted file committed to Git
│  & Push         │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  FluxCD Syncs   │  ← FluxCD detects change
│  from Git       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  SOPS Decrypt   │  ← FluxCD decrypts using Age/KMS key
│  (in-cluster)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  K8s Secret     │  ← Plaintext secret created in cluster
│  Created        │
└─────────────────┘
```

## Example Encrypted Secret

**Before Encryption** (`secret.yaml`):
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: database-credentials
  namespace: production
type: Opaque
data:
  username: YWRtaW4=
  password: c3VwZXJzZWNyZXQxMjM=
```

**After SOPS Encryption**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: database-credentials
  namespace: production
type: Opaque
data:
  username: ENC[AES256_GCM,data:abc123...,iv:...,tag:...,type:str]
  password: ENC[AES256_GCM,data:def456...,iv:...,tag:...,type:str]
sops:
  kms: []
  gcp_kms: []
  azure_kv: []
  age:
    - recipient: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
      enc: |
        -----BEGIN AGE ENCRYPTED FILE-----
        ...
        -----END AGE ENCRYPTED FILE-----
  lastmodified: "2024-03-22T10:00:00Z"
  mac: ENC[AES256_GCM,data:...,iv:...,tag:...,type:str]
  pgp: []
  encrypted_regex: ^(data|stringData)$
  version: 3.7.3
```

**Note**: Metadata fields (`kind`, `metadata`, `type`) remain **plaintext** for debugging and diff visibility.

## Troubleshooting

### Issue: "SOPS not found in PATH"

**Solution**:
```bash
# macOS
brew install sops

# Linux - manual install
wget https://github.com/getsops/sops/releases/download/v3.9.2/sops-v3.9.2.linux.amd64
sudo mv sops-v3.9.2.linux.amd64 /usr/local/bin/sops
sudo chmod +x /usr/local/bin/sops
```

### Issue: "failed to get the data key required to decrypt the SOPS file"

**Cause**: SOPS can't find the Age private key or KMS permissions are missing.

**Solution**:
```bash
# Age mode: Set environment variable
export SOPS_AGE_KEY_FILE=/path/to/age-keys/keys.txt

# KMS mode: Check AWS credentials
aws sts get-caller-identity
aws kms decrypt --help  # Verify KMS access
```

### Issue: "MAC mismatch" when decrypting

**Cause**: File was modified after encryption or wrong key is being used.

**Solution**:
- Verify you're using the correct Age key or KMS key
- If file was manually edited, re-encrypt from plaintext source
- Check `.sops.yaml` config matches the key used for encryption

### Issue: FluxCD can't decrypt secrets in-cluster

**Checklist**:
1. Verify `sops-age` secret exists in `flux-system` namespace:
   ```bash
   kubectl get secret sops-age -n flux-system
   ```

2. Check FluxCD Kustomization has decryption enabled:
   ```yaml
   apiVersion: kustomize.toolkit.fluxcd.io/v1
   kind: Kustomization
   metadata:
     name: secrets
     namespace: flux-system
   spec:
     decryption:
       provider: sops
       secretRef:
         name: sops-age  # ← Must reference Age key secret
   ```

3. View FluxCD logs:
   ```bash
   kubectl logs -n flux-system deploy/kustomize-controller -f
   ```

### Issue: "encrypted_regex" not working (entire file encrypted)

**Cause**: `.sops.yaml` config not found or incorrect.

**Solution**:
```bash
# Verify .sops.yaml exists in repo root
ls -la /data/secrets-repo/.sops.yaml

# Check encrypted_regex pattern
cat /data/secrets-repo/.sops.yaml
# Should contain:
#   encrypted_regex: '^(data|stringData)$'

# Re-encrypt with explicit config
SOPS_AGE_KEY_FILE=/keys/age.txt sops -e --config .sops.yaml input.yaml > output.yaml
```

## Security Best Practices

### DO ✅

- **Store Age private keys in secure vaults** (1Password, AWS Secrets Manager, etc.)
- **Use KMS in production** for automatic key rotation and audit trails
- **Rotate keys periodically** (every 90 days recommended)
- **Use different keys per environment** (dev/staging/prod)
- **Limit access to decryption keys** (principle of least privilege)
- **Enable CloudTrail for KMS** to audit decrypt operations

### DON'T ❌

- **Never commit Age private keys to Git** (add to `.gitignore`)
- **Never share private keys via email/Slack**
- **Don't use the same key across environments**
- **Don't store plaintext secrets in PostgreSQL after publishing**
- **Don't manually edit encrypted files** (always decrypt → edit → re-encrypt)

## Key Rotation

### Age Key Rotation

1. **Generate new Age key**:
   ```bash
   age-keygen -o new-key.txt
   NEW_PUB_KEY=$(grep "# public key:" new-key.txt | awk '{print $NF}')
   ```

2. **Re-encrypt all secrets**:
   ```bash
   # For each encrypted file
   SOPS_AGE_KEY_FILE=/old/key.txt sops -d secret.yaml | \
   SOPS_AGE_KEY_FILE=/new/key.txt sops -e --age $NEW_PUB_KEY --encrypted-regex '^(data|stringData)$' /dev/stdin > secret-new.yaml
   ```

3. **Update `.sops.yaml` and FluxCD secret**:
   ```bash
   # Update .sops.yaml with new public key
   sed -i "s/age: age1.*/age: $NEW_PUB_KEY/" .sops.yaml
   
   # Update Kubernetes secret
   kubectl delete secret sops-age -n flux-system
   kubectl create secret generic sops-age \
     --from-file=age.agekey=/new/key.txt \
     --namespace=flux-system
   ```

### KMS Key Rotation

AWS KMS supports **automatic key rotation** (enabled by default):
- Keys rotate every year automatically
- Old key versions remain available for decryption
- No manual re-encryption needed

To manually rotate:
```bash
# Create new KMS key
NEW_KEY_ARN=$(aws kms create-key --description "Rotated SOPS key" --query 'KeyMetadata.Arn' --output text)

# Re-encrypt secrets with new key
sops updatekeys --kms $NEW_KEY_ARN secret.yaml
```

## Performance Considerations

- **Encryption Time**: ~50-100ms per secret (Age), ~200-300ms (KMS)
- **Decryption Time**: Similar to encryption
- **File Size**: Encrypted files are ~2-3x larger due to metadata
- **Git Repo Size**: SOPS files compress well in Git (metadata is repetitive)

## References

- [SOPS GitHub](https://github.com/getsops/sops)
- [Age Encryption](https://age-encryption.org/)
- [FluxCD SOPS Guide](https://fluxcd.io/flux/guides/mozilla-sops/)
- [AWS KMS Documentation](https://docs.aws.amazon.com/kms/)

## Support

For issues with SOPS integration:
1. Check this documentation first
2. Run `make test-sops` to verify setup
3. Check backend logs for encryption errors
4. Open an issue with error messages and config (redact keys!)
