# Deployment Guide Index

Welcome to the Secret Manager deployment documentation. Choose your deployment method based on your needs.

## Quick Navigation

- [🚀 Quick Start](#quick-start)
- [☁️ Cloud Platforms](#cloud-platforms)
- [🐳 Docker/Container Deployments](#docker-deployments)
- [☸️ Kubernetes](#kubernetes)
- [📊 Comparison Guide](#comparison-guide)

---

## Quick Start

**For local development:**
```bash
docker compose up --build
```
See [main README](../README.md) for details.

**For production (choose your cloud):**
- [AWS ECS Fargate](./deploy/AWS.md) — Enterprise-grade, mature
- [GCP Cloud Run](./deploy/GCP.md) — Simple, cost-effective ⭐ **Recommended**
- [Azure Container Apps](./deploy/AZURE.md) — Microsoft ecosystem

---

## Cloud Platforms

### AWS (Amazon Web Services)

**Services**: ECS Fargate, RDS PostgreSQL, ALB, ECR

```bash
cd terraform/aws
terraform init && terraform apply
```

- **Best for**: Enterprise production, mature workloads
- **Cost**: ~$80/month (small), ~$285/month (medium)
- **Deployment time**: 30-45 minutes
- **Complexity**: Medium

📖 [Full AWS Guide](./deploy/AWS.md)

---

### GCP (Google Cloud Platform) ⭐ Recommended

**Services**: Cloud Run, Cloud SQL, Cloud Load Balancing, Artifact Registry

```bash
cd terraform/gcp
terraform init && terraform apply
```

- **Best for**: Startups, cost-conscious, modern apps
- **Cost**: ~$39/month (small), ~$295/month (medium)
- **Deployment time**: 20-30 minutes
- **Complexity**: Low

📖 [Full GCP Guide](./deploy/GCP.md)

---

### Azure (Microsoft Azure)

**Services**: Container Apps, Azure Database for PostgreSQL, Application Gateway, ACR

```bash
cd terraform/azure
terraform init && terraform apply
```

- **Best for**: Microsoft shops, hybrid cloud
- **Cost**: ~$51/month (small), ~$413/month (medium)
- **Deployment time**: 25-35 minutes
- **Complexity**: Medium

📖 [Full Azure Guide](./deploy/AZURE.md)

---

## Docker Deployments

### Local Development (Docker Compose)

**File**: `docker-compose.yml`

```bash
# Start all services
docker compose up --build

# With production config
docker compose -f docker-compose.prod.yml up
```

**Services**:
- Backend (Go API on :8080)
- Frontend (Next.js on :3000)
- PostgreSQL (:5432)
- Mock OAuth (:9000, dev only)

📖 See [README.md](../README.md#quick-start)

---

### Production Docker Compose

**File**: `docker-compose.prod.yml`

**Prerequisites**:
- Production `.env` file with secrets
- SSL certificates (for nginx)
- Age encryption keys
- Git SSH keys

```bash
# Production deployment
docker compose -f docker-compose.prod.yml up -d
```

**Includes**:
- Production-optimized images
- Nginx reverse proxy
- Health checks
- Restart policies
- Volume persistence

📖 See [docker-compose.prod.yml](../docker-compose.prod.yml)

---

## Kubernetes

### Helm Chart (Recommended for K8s)

**Location**: `helm/secret-manager/`

```bash
# Install with Helm
helm install secret-manager ./helm/secret-manager \
  --namespace secret-manager \
  --create-namespace \
  --values ./helm/secret-manager/values.yaml
```

**Features**:
- PostgreSQL StatefulSet
- Backend/Frontend Deployments
- Ingress with TLS
- ConfigMaps and Secrets
- Resource limits
- Health checks

📖 See [helm/secret-manager/README.md](../helm/secret-manager/README.md)

---

### Raw Kubernetes Manifests

**Location**: `k8s/`

```bash
# Apply manifests
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/postgres/
kubectl apply -f k8s/backend/
kubectl apply -f k8s/frontend/
kubectl apply -f k8s/ingress/
```

📖 See [k8s/README.md](../k8s/README.md)

---

### Local Kubernetes (kind)

**For testing Kubernetes locally:**

```bash
# Create local cluster with FluxCD
make kind-up

# Deploy Secret Manager
helm install secret-manager ./helm/secret-manager

# Delete cluster
make kind-down
```

📖 See [FLUXCD_SETUP.md](../FLUXCD_SETUP.md)

---

## Comparison Guide

Not sure which platform to choose? See the detailed comparison:

📊 [Cloud Platform Comparison](./deploy/CLOUD-COMPARISON.md)

### Quick Comparison

| Platform | Cost (small) | Complexity | Time | Best For |
|----------|--------------|------------|------|----------|
| **GCP Cloud Run** ⭐ | ~$39/mo | Low | 20 min | Startups, cost-conscious |
| **AWS ECS Fargate** | ~$80/mo | Medium | 35 min | Enterprises, mature workloads |
| **Azure Container Apps** | ~$51/mo | Medium | 30 min | Microsoft shops |
| **Docker Compose** | Server cost | Low | 10 min | Small deployments, VPS |
| **Kubernetes (Helm)** | Cluster cost | High | 45 min | Large scale, multi-tenant |

---

## Decision Tree

```
START: Where do you want to deploy?

├─ Local development?
│  └─> Use Docker Compose (docker-compose.yml)
│
├─ Small VPS/single server?
│  └─> Use Production Docker Compose (docker-compose.prod.yml)
│
├─ Cloud provider?
│  ├─ Existing AWS infrastructure? ──> AWS ECS Fargate
│  ├─ Cost-conscious or startup? ──> GCP Cloud Run ⭐
│  ├─ Microsoft ecosystem? ──> Azure Container Apps
│  └─ No preference? ──> GCP Cloud Run (simplest + cheapest)
│
└─ Existing Kubernetes cluster?
   ├─ GitOps workflow? ──> FluxCD + Helm
   └─ Simple deployment? ──> Helm chart
```

---

## Infrastructure as Code

All cloud deployments include Terraform configurations:

```
terraform/
├── aws/          # AWS ECS Fargate deployment (~450 lines)
├── gcp/          # GCP Cloud Run deployment (~320 lines)
└── azure/        # Azure Container Apps deployment (~380 lines)
```

**Usage**:
```bash
cd terraform/{aws|gcp|azure}
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
terraform init
terraform plan
terraform apply
```

---

## Deployment Scripts

Automated deployment scripts for each platform:

```
scripts/
├── deploy-aws.sh      # Build + push to ECR, deploy to ECS
├── deploy-gcp.sh      # Build + push to Artifact Registry, deploy to Cloud Run
└── deploy-azure.sh    # Build + push to ACR, deploy to Container Apps
```

---

## Prerequisites by Platform

### All Platforms
- Docker 20+
- Git
- Domain name (for production HTTPS)

### AWS
- AWS CLI v2
- AWS account with billing
- Terraform 1.5+ (if using IaC)

### GCP
- gcloud CLI
- GCP project with billing
- Terraform 1.5+ (if using IaC)

### Azure
- Azure CLI 2.50+
- Azure subscription
- Terraform 1.5+ (if using IaC)

### Kubernetes
- kubectl
- Helm 3+ (for Helm chart)
- kind (for local testing)

---

## Security Considerations

Before deploying to production:

1. **Secrets Management**
   - Use cloud provider secrets managers (AWS Secrets Manager, GCP Secret Manager, Azure Key Vault)
   - Never commit secrets to Git
   - Rotate secrets regularly

2. **Network Security**
   - Use private subnets for database
   - Configure security groups/firewall rules
   - Enable DDoS protection

3. **SSL/TLS**
   - Use managed certificates (ACM, Google-managed, Azure-managed)
   - Enforce HTTPS
   - Configure HSTS headers

4. **Database**
   - Enable encryption at rest
   - Use private networking
   - Configure automated backups
   - Enable point-in-time recovery

5. **Container Security**
   - Scan images for vulnerabilities
   - Run as non-root user
   - Use minimal base images (Alpine)
   - Keep dependencies updated

📖 See platform-specific security sections in each guide.

---

## Cost Estimates

### Small Deployment (Development)
- **Docker Compose**: Server cost only (~$5-20/mo VPS)
- **GCP**: ~$39/month ⭐ Lowest
- **Azure**: ~$51/month
- **AWS**: ~$80/month
- **Kubernetes**: Cluster cost (~$70-150/mo)

### Medium Deployment (Production)
- **GCP**: ~$295/month
- **AWS**: ~$285/month
- **Azure**: ~$413/month
- **Kubernetes**: Cluster + nodes (~$300-600/mo)

### Large Deployment (Enterprise)
- **GCP**: ~$1,230/month ⭐ Most cost-effective
- **AWS**: ~$1,400/month
- **Azure**: ~$1,480/month
- **Kubernetes**: Cluster + nodes (~$1,000-2,000/mo)

📊 [Detailed cost breakdown](./deploy/CLOUD-COMPARISON.md#6-pricing-comparison-3-tiers)

---

## Monitoring & Observability

Each platform includes:
- **Logs**: Centralized logging (CloudWatch, Cloud Logging, Azure Monitor)
- **Metrics**: CPU, memory, request rate, latency
- **Alerts**: Configurable alerts for errors, high resource usage
- **Tracing**: Application performance monitoring (optional)

See platform-specific monitoring sections.

---

## Backup & Disaster Recovery

All cloud platforms provide:
- **Automated backups**: Daily database backups
- **Point-in-time recovery**: Restore to any point in time
- **Backup retention**: 7-365 days (GCP longest)
- **Multi-region**: Optional for high availability

See platform-specific DR sections.

---

## Migration Paths

### From Docker Compose
1. **Easiest**: GCP Cloud Run (minimal changes)
2. **Most control**: AWS ECS (similar syntax)
3. **Middle ground**: Azure Container Apps

### From Kubernetes
1. **Best fit**: Azure Container Apps (built on K8s)
2. **Alternative**: Stay on Kubernetes (EKS, GKE, AKS)

### From On-Premises
1. **Start with**: GCP (simplest)
2. **Use**: Database migration services
3. **Consider**: Hybrid cloud (Azure Arc)

---

## Getting Help

- **Documentation**: See platform-specific guides in `docs/deploy/`
- **Issues**: [GitHub Issues](https://github.com/Naikelin/secret-manager/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Naikelin/secret-manager/discussions)

---

## Next Steps

1. **Choose your platform** using the [decision tree](#decision-tree)
2. **Read the platform-specific guide**:
   - [AWS Deployment Guide](./deploy/AWS.md)
   - [GCP Deployment Guide](./deploy/GCP.md)
   - [Azure Deployment Guide](./deploy/AZURE.md)
3. **Set up infrastructure** with Terraform
4. **Deploy the application**
5. **Configure monitoring and backups**
6. **Test failover and disaster recovery**

---

## Contributing

Found an issue or want to improve the deployment docs?

1. Fork the repository
2. Make your changes
3. Submit a pull request

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.
