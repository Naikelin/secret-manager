# GCP Deployment Guide

Deploy Secret Manager on Google Cloud Platform using Cloud Run, Cloud SQL PostgreSQL, and Cloud Load Balancing.

## Architecture Overview

```
┌──────────────┐     ┌───────────────┐     ┌──────────────┐
│  Cloud DNS   │────▶│ Cloud Load    │────▶│  Cloud Run   │
│              │     │   Balancer    │     │   Services   │
└──────────────┘     └───────────────┘     └──────────────┘
                            │                      │
                            │                      ├─ Backend (Go)
                            │                      ├─ Frontend (Next.js)
                            ▼                      │
                     ┌──────────────┐              ▼
                     │ Managed SSL  │     ┌──────────────┐
                     │ Certificate  │     │  Cloud SQL   │
                     └──────────────┘     │  PostgreSQL  │
                                          └──────────────┘
                     ┌──────────────┐     
                     │ Artifact     │     ┌──────────────┐
                     │ Registry     │     │ Cloud        │
                     └──────────────┘     │ Logging/     │
                                          │ Monitoring   │
                     ┌──────────────┐     └──────────────┘
                     │     VPC      │
                     │  Connector   │
                     └──────────────┘
```

### Components

- **Compute**: Cloud Run (serverless containers, auto-scaling)
- **Database**: Cloud SQL PostgreSQL 15
- **Load Balancer**: Cloud Load Balancing (HTTPS)
- **Container Registry**: Artifact Registry
- **Monitoring**: Cloud Logging and Cloud Monitoring
- **SSL/TLS**: Google-managed certificates
- **Networking**: VPC with Serverless VPC Access

## Prerequisites

### Required Tools

- Google Cloud SDK (gcloud CLI)
- Terraform v1.5+ (if using IaC)
- Docker v20+
- Git

### GCP Project Requirements

- GCP project with billing enabled
- Required APIs enabled:
  ```bash
  gcloud services enable \
    run.googleapis.com \
    sqladmin.googleapis.com \
    compute.googleapis.com \
    vpcaccess.googleapis.com \
    artifactregistry.googleapis.com \
    cloudresourcemanager.googleapis.com
  ```

### Domain Setup

- Domain name registered (Cloud DNS or external)
- Access to configure DNS records

## Quick Start

### Option 1: Terraform (Recommended)

```bash
# 1. Clone repository
git clone https://github.com/Naikelin/secret-manager.git
cd secret-manager

# 2. Configure Terraform variables
cd terraform/gcp
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your GCP project details

# 3. Initialize and apply
terraform init
terraform plan
terraform apply

# 4. Build and deploy
./scripts/deploy-gcp.sh
```

### Option 2: gcloud CLI

```bash
# Quick deployment script
./scripts/deploy-gcp.sh
```

## Infrastructure Setup

### 1. Create VPC and Serverless VPC Access

Cloud Run services need VPC access to connect to Cloud SQL.

**Terraform** (see `terraform/gcp/vpc.tf`):

```hcl
resource "google_compute_network" "main" {
  name                    = "${var.project_name}-vpc"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "main" {
  name          = "${var.project_name}-subnet"
  ip_cidr_range = "10.0.0.0/24"
  region        = var.region
  network       = google_compute_network.main.id
}

# VPC Connector for Cloud Run to access Cloud SQL
resource "google_vpc_access_connector" "main" {
  name          = "${var.project_name}-connector"
  region        = var.region
  network       = google_compute_network.main.name
  ip_cidr_range = "10.8.0.0/28"
  
  min_instances = 2
  max_instances = 3
}
```

**gcloud CLI**:

```bash
# Create VPC network
gcloud compute networks create secret-manager-vpc \
  --subnet-mode=custom

# Create subnet
gcloud compute networks subnets create secret-manager-subnet \
  --network=secret-manager-vpc \
  --region=us-central1 \
  --range=10.0.0.0/24

# Create VPC connector
gcloud compute networks vpc-access connectors create secret-manager-connector \
  --region=us-central1 \
  --subnet=secret-manager-subnet \
  --min-instances=2 \
  --max-instances=3
```

### 2. Cloud SQL PostgreSQL Instance

**Terraform** (see `terraform/gcp/cloudsql.tf`):

```hcl
resource "google_sql_database_instance" "postgres" {
  name             = "${var.project_name}-db"
  database_version = "POSTGRES_15"
  region           = var.region
  
  settings {
    tier              = var.db_tier  # db-f1-micro for dev, db-custom-2-7680 for prod
    availability_type = var.environment == "production" ? "REGIONAL" : "ZONAL"
    disk_size         = 20
    disk_type         = "PD_SSD"
    disk_autoresize   = true
    
    backup_configuration {
      enabled            = true
      start_time         = "03:00"
      point_in_time_recovery_enabled = var.environment == "production"
      transaction_log_retention_days = 7
    }
    
    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.main.id
    }
    
    database_flags {
      name  = "max_connections"
      value = "100"
    }
  }
  
  deletion_protection = var.environment == "production"
}

resource "google_sql_database" "main" {
  name     = "secretmanager"
  instance = google_sql_database_instance.postgres.name
}

resource "google_sql_user" "main" {
  name     = var.db_user
  instance = google_sql_database_instance.postgres.name
  password = var.db_password
}
```

**gcloud CLI**:

```bash
# Create Cloud SQL instance
gcloud sql instances create secret-manager-db \
  --database-version=POSTGRES_15 \
  --tier=db-f1-micro \
  --region=us-central1 \
  --network=projects/PROJECT_ID/global/networks/secret-manager-vpc \
  --no-assign-ip \
  --backup \
  --backup-start-time=03:00

# Create database
gcloud sql databases create secretmanager \
  --instance=secret-manager-db

# Create user
gcloud sql users create admin \
  --instance=secret-manager-db \
  --password=YOUR_SECURE_PASSWORD
```

### 3. Artifact Registry

**Terraform** (see `terraform/gcp/artifact-registry.tf`):

```hcl
resource "google_artifact_registry_repository" "main" {
  location      = var.region
  repository_id = var.project_name
  description   = "Secret Manager container images"
  format        = "DOCKER"
}
```

**gcloud CLI**:

```bash
# Create repository
gcloud artifacts repositories create secret-manager \
  --repository-format=docker \
  --location=us-central1 \
  --description="Secret Manager container images"

# Configure Docker authentication
gcloud auth configure-docker us-central1-docker.pkg.dev
```

### 4. Build and Push Docker Images

```bash
# Set variables
export PROJECT_ID=$(gcloud config get-value project)
export REGION=us-central1
export REPO=secret-manager

# Build backend
cd backend
docker build -t $REGION-docker.pkg.dev/$PROJECT_ID/$REPO/backend:latest -f Dockerfile.prod .
docker push $REGION-docker.pkg.dev/$PROJECT_ID/$REPO/backend:latest

# Build frontend
cd ../frontend
docker build -t $REGION-docker.pkg.dev/$PROJECT_ID/$REPO/frontend:latest -f Dockerfile.prod \
  --build-arg NEXT_PUBLIC_API_URL=https://secrets.example.com/api .
docker push $REGION-docker.pkg.dev/$PROJECT_ID/$REPO/frontend:latest
```

### 5. Deploy to Cloud Run

**Terraform** (see `terraform/gcp/cloudrun.tf`):

```hcl
# Backend Service
resource "google_cloud_run_service" "backend" {
  name     = "${var.project_name}-backend"
  location = var.region
  
  template {
    spec {
      containers {
        image = "${var.region}-docker.pkg.dev/${var.project_id}/${var.project_name}/backend:latest"
        
        ports {
          container_port = 8080
        }
        
        env {
          name  = "DB_HOST"
          value = "/cloudsql/${var.project_id}:${var.region}:${google_sql_database_instance.postgres.name}"
        }
        
        env {
          name  = "DB_PORT"
          value = "5432"
        }
        
        env {
          name  = "DB_NAME"
          value = "secretmanager"
        }
        
        env {
          name  = "DB_USER"
          value = var.db_user
        }
        
        env {
          name = "DB_PASSWORD"
          value_from {
            secret_key_ref {
              name = google_secret_manager_secret.db_password.secret_id
              key  = "latest"
            }
          }
        }
        
        env {
          name  = "PORT"
          value = "8080"
        }
        
        resources {
          limits = {
            cpu    = "1000m"
            memory = "512Mi"
          }
        }
      }
      
      service_account_name = google_service_account.backend.email
    }
    
    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale"      = "1"
        "autoscaling.knative.dev/maxScale"      = "10"
        "run.googleapis.com/cloudsql-instances" = "${var.project_id}:${var.region}:${google_sql_database_instance.postgres.name}"
        "run.googleapis.com/vpc-access-connector" = google_vpc_access_connector.main.id
        "run.googleapis.com/vpc-access-egress"    = "private-ranges-only"
      }
    }
  }
  
  traffic {
    percent         = 100
    latest_revision = true
  }
}

# Frontend Service
resource "google_cloud_run_service" "frontend" {
  name     = "${var.project_name}-frontend"
  location = var.region
  
  template {
    spec {
      containers {
        image = "${var.region}-docker.pkg.dev/${var.project_id}/${var.project_name}/frontend:latest"
        
        ports {
          container_port = 3000
        }
        
        env {
          name  = "NEXT_PUBLIC_API_URL"
          value = "https://${var.domain_name}/api"
        }
        
        env {
          name  = "NODE_ENV"
          value = "production"
        }
        
        resources {
          limits = {
            cpu    = "1000m"
            memory = "256Mi"
          }
        }
      }
    }
    
    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale" = "1"
        "autoscaling.knative.dev/maxScale" = "5"
      }
    }
  }
  
  traffic {
    percent         = 100
    latest_revision = true
  }
}

# Allow unauthenticated access (public)
resource "google_cloud_run_service_iam_member" "backend_public" {
  service  = google_cloud_run_service.backend.name
  location = google_cloud_run_service.backend.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_service_iam_member" "frontend_public" {
  service  = google_cloud_run_service.frontend.name
  location = google_cloud_run_service.frontend.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}
```

**gcloud CLI**:

```bash
# Deploy backend
gcloud run deploy backend \
  --image us-central1-docker.pkg.dev/PROJECT_ID/secret-manager/backend:latest \
  --platform managed \
  --region us-central1 \
  --add-cloudsql-instances PROJECT_ID:us-central1:secret-manager-db \
  --vpc-connector secret-manager-connector \
  --vpc-egress private-ranges-only \
  --set-env-vars "DB_HOST=/cloudsql/PROJECT_ID:us-central1:secret-manager-db,DB_PORT=5432,DB_NAME=secretmanager,DB_USER=admin" \
  --set-secrets "DB_PASSWORD=db-password:latest,JWT_SECRET=jwt-secret:latest" \
  --allow-unauthenticated \
  --min-instances 1 \
  --max-instances 10 \
  --cpu 1 \
  --memory 512Mi

# Deploy frontend
gcloud run deploy frontend \
  --image us-central1-docker.pkg.dev/PROJECT_ID/secret-manager/frontend:latest \
  --platform managed \
  --region us-central1 \
  --set-env-vars "NEXT_PUBLIC_API_URL=https://secrets.example.com/api,NODE_ENV=production" \
  --allow-unauthenticated \
  --min-instances 1 \
  --max-instances 5 \
  --cpu 1 \
  --memory 256Mi
```

### 6. Cloud Load Balancer with Custom Domain

**Terraform** (see `terraform/gcp/loadbalancer.tf`):

```hcl
# Reserve static IP
resource "google_compute_global_address" "main" {
  name = "${var.project_name}-ip"
}

# Backend NEG for Cloud Run backend
resource "google_compute_region_network_endpoint_group" "backend" {
  name                  = "${var.project_name}-backend-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  
  cloud_run {
    service = google_cloud_run_service.backend.name
  }
}

# Backend NEG for Cloud Run frontend
resource "google_compute_region_network_endpoint_group" "frontend" {
  name                  = "${var.project_name}-frontend-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  
  cloud_run {
    service = google_cloud_run_service.frontend.name
  }
}

# Backend services
resource "google_compute_backend_service" "backend" {
  name                  = "${var.project_name}-backend"
  protocol              = "HTTP"
  port_name             = "http"
  timeout_sec           = 30
  enable_cdn            = false
  
  backend {
    group = google_compute_region_network_endpoint_group.backend.id
  }
}

resource "google_compute_backend_service" "frontend" {
  name                  = "${var.project_name}-frontend"
  protocol              = "HTTP"
  port_name             = "http"
  timeout_sec           = 30
  enable_cdn            = true
  
  backend {
    group = google_compute_region_network_endpoint_group.frontend.id
  }
}

# URL map with path-based routing
resource "google_compute_url_map" "main" {
  name            = "${var.project_name}-lb"
  default_service = google_compute_backend_service.frontend.id
  
  host_rule {
    hosts        = [var.domain_name]
    path_matcher = "main"
  }
  
  path_matcher {
    name            = "main"
    default_service = google_compute_backend_service.frontend.id
    
    path_rule {
      paths   = ["/api/*", "/health"]
      service = google_compute_backend_service.backend.id
    }
  }
}

# Managed SSL certificate
resource "google_compute_managed_ssl_certificate" "main" {
  name = "${var.project_name}-cert"
  
  managed {
    domains = [var.domain_name]
  }
}

# HTTPS proxy
resource "google_compute_target_https_proxy" "main" {
  name             = "${var.project_name}-https-proxy"
  url_map          = google_compute_url_map.main.id
  ssl_certificates = [google_compute_managed_ssl_certificate.main.id]
}

# Forwarding rule (HTTPS)
resource "google_compute_global_forwarding_rule" "https" {
  name                  = "${var.project_name}-https"
  target                = google_compute_target_https_proxy.main.id
  port_range            = "443"
  ip_address            = google_compute_global_address.main.address
  load_balancing_scheme = "EXTERNAL"
}

# HTTP to HTTPS redirect
resource "google_compute_url_map" "https_redirect" {
  name = "${var.project_name}-https-redirect"
  
  default_url_redirect {
    https_redirect         = true
    redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
    strip_query            = false
  }
}

resource "google_compute_target_http_proxy" "https_redirect" {
  name    = "${var.project_name}-http-proxy"
  url_map = google_compute_url_map.https_redirect.id
}

resource "google_compute_global_forwarding_rule" "http" {
  name                  = "${var.project_name}-http"
  target                = google_compute_target_http_proxy.https_redirect.id
  port_range            = "80"
  ip_address            = google_compute_global_address.main.address
  load_balancing_scheme = "EXTERNAL"
}
```

### 7. Configure DNS

```bash
# Get the static IP
export LB_IP=$(gcloud compute addresses describe secret-manager-ip --global --format="get(address)")

# Create DNS A record (Cloud DNS)
gcloud dns record-sets create secrets.example.com. \
  --rrdatas=$LB_IP \
  --type=A \
  --ttl=300 \
  --zone=YOUR_DNS_ZONE

# Or add to external DNS provider
echo "Add A record: secrets.example.com -> $LB_IP"
```

## Monitoring and Logging

### Cloud Logging

```bash
# View backend logs
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=backend" \
  --limit 50 \
  --format json

# View frontend logs
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=frontend" \
  --limit 50 \
  --format json

# Real-time streaming
gcloud logging tail "resource.type=cloud_run_revision"
```

### Cloud Monitoring

**Create alerting policies** (Terraform in `terraform/gcp/monitoring.tf`):

```hcl
resource "google_monitoring_alert_policy" "backend_error_rate" {
  display_name = "Backend Error Rate"
  combiner     = "OR"
  
  conditions {
    display_name = "Error rate > 5%"
    
    condition_threshold {
      filter          = "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"backend\" AND metric.type=\"run.googleapis.com/request_count\" AND metric.labels.response_code_class=\"5xx\""
      duration        = "60s"
      comparison      = "COMPARISON_GT"
      threshold_value = 0.05
      
      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_RATE"
      }
    }
  }
  
  notification_channels = [google_monitoring_notification_channel.email.name]
}
```

## Auto-Scaling

Cloud Run **auto-scales by default** based on:
- Request concurrency (default: 80 concurrent requests per instance)
- CPU utilization
- Memory usage

**Configure scaling limits**:

```bash
# Set min/max instances
gcloud run services update backend \
  --min-instances 1 \
  --max-instances 10 \
  --region us-central1

# Set concurrency
gcloud run services update backend \
  --concurrency 100 \
  --region us-central1
```

## Cost Estimation

### Small Deployment (Development)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Cloud Run | 2 instances (1 vCPU, 512MB) @ 50% utilization | ~$15 |
| Cloud SQL | db-f1-micro (0.6GB RAM) | ~$10 |
| VPC Connector | 2 instances | ~$15 |
| Load Balancer | Forwarding rules + data | ~$20 |
| Artifact Registry | 10GB storage | ~$1 |
| Cloud Logging | 10GB/month | ~$5 |
| **Total** | | **~$66/month** |

### Medium Deployment (Production)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Cloud Run | 4 instances (1 vCPU, 1GB) @ 70% utilization | ~$50 |
| Cloud SQL | db-custom-2-7680 (2 vCPU, 7.5GB) | ~$140 |
| VPC Connector | 3 instances | ~$22 |
| Load Balancer | High traffic | ~$40 |
| Artifact Registry | 50GB storage | ~$5 |
| Cloud Logging | 50GB/month | ~$15 |
| **Total** | | **~$272/month** |

### Large Deployment (Enterprise)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Cloud Run | 10 instances (2 vCPU, 2GB) @ 80% utilization | ~$280 |
| Cloud SQL | db-custom-8-30720 (8 vCPU, 30GB) + HA | ~$600 |
| VPC Connector | 3 max instances | ~$22 |
| Load Balancer | Very high traffic + CDN | ~$80 |
| Artifact Registry | 200GB storage | ~$20 |
| Cloud Logging | 200GB/month | ~$50 |
| **Total** | | **~$1,052/month** |

> **Note**: Cloud Run pricing is based on actual resource usage (CPU, memory, requests), making it very cost-effective for variable workloads.

## Backup and Disaster Recovery

### Automated Backups

```bash
# Backups are automatic with Cloud SQL
# Configure backup window
gcloud sql instances patch secret-manager-db \
  --backup-start-time=03:00

# Enable point-in-time recovery (write-ahead logs)
gcloud sql instances patch secret-manager-db \
  --enable-point-in-time-recovery
```

### Manual Backups

```bash
# Create on-demand backup
gcloud sql backups create \
  --instance=secret-manager-db \
  --description="Pre-deployment backup"

# List backups
gcloud sql backups list --instance=secret-manager-db

# Restore from backup
gcloud sql backups restore BACKUP_ID \
  --backup-instance=secret-manager-db \
  --backup-instance-region=us-central1
```

### Export Database

```bash
# Export to Cloud Storage
gcloud sql export sql secret-manager-db \
  gs://YOUR_BUCKET/backup-$(date +%Y%m%d).sql \
  --database=secretmanager
```

## Security Best Practices

1. **Use Secret Manager** for sensitive values (DB password, JWT secret)
2. **Enable VPC Service Controls** for data exfiltration prevention
3. **Use private IP for Cloud SQL** (no public IP)
4. **Configure IAM roles** with least privilege
5. **Enable Cloud Armor** for DDoS protection on Load Balancer
6. **Use Cloud KMS** for encryption keys
7. **Enable Cloud Audit Logs** for compliance
8. **Configure VPC Flow Logs** for network monitoring

## Troubleshooting

### Cloud Run Service Not Starting

```bash
# Check service status
gcloud run services describe backend --region us-central1

# View logs
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=backend" \
  --limit 20 \
  --format json

# Check latest revision
gcloud run revisions list --service backend --region us-central1
```

### Database Connection Issues

```bash
# Test Cloud SQL connectivity
gcloud sql connect secret-manager-db --user=admin

# Check VPC connector status
gcloud compute networks vpc-access connectors describe secret-manager-connector \
  --region us-central1

# Verify service account has Cloud SQL Client role
gcloud projects get-iam-policy PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:backend@PROJECT_ID.iam.gserviceaccount.com"
```

### SSL Certificate Not Provisioning

```bash
# Check certificate status (takes 15-60 minutes)
gcloud compute ssl-certificates describe secret-manager-cert

# Verify DNS points to load balancer IP
dig secrets.example.com

# Check domain validation
gcloud compute ssl-certificates list --filter="name=secret-manager-cert"
```

## Next Steps

- [Configure Cloud Build for CI/CD](../../.github/workflows/deploy-gcp.yml)
- [Set up monitoring dashboards](./MONITORING.md)
- [Configure backups and disaster recovery](./BACKUP.md)
- [Review security hardening guide](./SECURITY.md)

## References

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Cloud SQL Best Practices](https://cloud.google.com/sql/docs/postgres/best-practices)
- [Terraform Google Provider](https://registry.terraform.io/providers/hashicorp/google/latest/docs)
- [Complete Terraform code](../../terraform/gcp/)
