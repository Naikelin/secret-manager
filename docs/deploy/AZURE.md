# Azure Deployment Guide

Deploy Secret Manager on Microsoft Azure using Container Apps, Azure Database for PostgreSQL, and Application Gateway.

## Architecture Overview

```
┌──────────────┐     ┌───────────────┐     ┌──────────────┐
│  Azure DNS   │────▶│ Application   │────▶│  Container   │
│              │     │   Gateway     │     │     Apps     │
└──────────────┘     └───────────────┘     └──────────────┘
                            │                      │
                            │                      ├─ Backend App
                            │                      ├─ Frontend App
                            ▼                      │
                     ┌──────────────┐              ▼
                     │ SSL/TLS      │     ┌──────────────┐
                     │ Certificate  │     │  Azure DB    │
                     └──────────────┘     │  PostgreSQL  │
                                          └──────────────┘
                     ┌──────────────┐     
                     │  Container   │     ┌──────────────┐
                     │   Registry   │     │ Azure        │
                     └──────────────┘     │ Monitor      │
                                          │ + Log        │
                     ┌──────────────┐     │ Analytics    │
                     │    VNet      │     └──────────────┘
                     │   Subnet     │
                     └──────────────┘
```

### Components

- **Compute**: Azure Container Apps (serverless containers)
- **Database**: Azure Database for PostgreSQL (Flexible Server)
- **Load Balancer**: Application Gateway
- **Container Registry**: Azure Container Registry (ACR)
- **Monitoring**: Azure Monitor and Log Analytics
- **SSL/TLS**: Azure-managed certificates
- **Networking**: Virtual Network with subnets

## Prerequisites

### Required Tools

- Azure CLI v2.50+
- Terraform v1.5+ (if using IaC)
- Docker v20+
- Git

### Azure Subscription Requirements

- Azure subscription with billing enabled
- Required providers registered:
  ```bash
  az provider register --namespace Microsoft.App
  az provider register --namespace Microsoft.DBforPostgreSQL
  az provider register --namespace Microsoft.Network
  az provider register --namespace Microsoft.ContainerRegistry
  az provider register --namespace Microsoft.OperationalInsights
  ```

### Domain Setup

- Domain name registered (Azure DNS or external)
- Access to configure DNS records

## Quick Start

### Option 1: Terraform (Recommended)

```bash
# 1. Clone repository
git clone https://github.com/Naikelin/secret-manager.git
cd secret-manager

# 2. Configure Terraform variables
cd terraform/azure
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your Azure subscription details

# 3. Initialize and apply
terraform init
terraform plan
terraform apply

# 4. Build and deploy
./scripts/deploy-azure.sh
```

### Option 2: Azure CLI

```bash
# Quick deployment script
./scripts/deploy-azure.sh
```

## Infrastructure Setup

### 1. Resource Group and Virtual Network

**Terraform** (see `terraform/azure/resource-group.tf` and `terraform/azure/vnet.tf`):

```hcl
resource "azurerm_resource_group" "main" {
  name     = "${var.project_name}-rg"
  location = var.location
  
  tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "azurerm_virtual_network" "main" {
  name                = "${var.project_name}-vnet"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  address_space       = ["10.0.0.0/16"]
  
  tags = {
    Environment = var.environment
  }
}

resource "azurerm_subnet" "container_apps" {
  name                 = "container-apps-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.0.1.0/24"]
}

resource "azurerm_subnet" "postgres" {
  name                 = "postgres-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.0.2.0/24"]
  
  delegation {
    name = "postgres-delegation"
    
    service_delegation {
      name = "Microsoft.DBforPostgreSQL/flexibleServers"
      actions = [
        "Microsoft.Network/virtualNetworks/subnets/join/action"
      ]
    }
  }
}

resource "azurerm_subnet" "app_gateway" {
  name                 = "app-gateway-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.0.3.0/24"]
}
```

**Azure CLI**:

```bash
# Create resource group
az group create \
  --name secret-manager-rg \
  --location eastus

# Create virtual network
az network vnet create \
  --resource-group secret-manager-rg \
  --name secret-manager-vnet \
  --address-prefix 10.0.0.0/16

# Create subnets
az network vnet subnet create \
  --resource-group secret-manager-rg \
  --vnet-name secret-manager-vnet \
  --name container-apps-subnet \
  --address-prefixes 10.0.1.0/24

az network vnet subnet create \
  --resource-group secret-manager-rg \
  --vnet-name secret-manager-vnet \
  --name postgres-subnet \
  --address-prefixes 10.0.2.0/24 \
  --delegations Microsoft.DBforPostgreSQL/flexibleServers

az network vnet subnet create \
  --resource-group secret-manager-rg \
  --vnet-name secret-manager-vnet \
  --name app-gateway-subnet \
  --address-prefixes 10.0.3.0/24
```

### 2. Azure Database for PostgreSQL

**Terraform** (see `terraform/azure/postgres.tf`):

```hcl
resource "azurerm_private_dns_zone" "postgres" {
  name                = "${var.project_name}-postgres.private.postgres.database.azure.com"
  resource_group_name = azurerm_resource_group.main.name
}

resource "azurerm_private_dns_zone_virtual_network_link" "postgres" {
  name                  = "${var.project_name}-postgres-link"
  private_dns_zone_name = azurerm_private_dns_zone.postgres.name
  resource_group_name   = azurerm_resource_group.main.name
  virtual_network_id    = azurerm_virtual_network.main.id
}

resource "azurerm_postgresql_flexible_server" "main" {
  name                   = "${var.project_name}-db"
  resource_group_name    = azurerm_resource_group.main.name
  location               = azurerm_resource_group.main.location
  version                = "15"
  delegated_subnet_id    = azurerm_subnet.postgres.id
  private_dns_zone_id    = azurerm_private_dns_zone.postgres.id
  administrator_login    = var.db_admin_username
  administrator_password = var.db_admin_password
  zone                   = "1"
  
  storage_mb = 32768
  sku_name   = var.db_sku_name  # B_Standard_B1ms for dev, GP_Standard_D2s_v3 for prod
  
  backup_retention_days        = 7
  geo_redundant_backup_enabled = var.environment == "production"
  
  high_availability {
    mode                      = var.environment == "production" ? "ZoneRedundant" : "Disabled"
    standby_availability_zone = var.environment == "production" ? "2" : null
  }
  
  tags = {
    Environment = var.environment
  }
  
  depends_on = [azurerm_private_dns_zone_virtual_network_link.postgres]
}

resource "azurerm_postgresql_flexible_server_database" "main" {
  name      = "secretmanager"
  server_id = azurerm_postgresql_flexible_server.main.id
  collation = "en_US.utf8"
  charset   = "UTF8"
}

resource "azurerm_postgresql_flexible_server_firewall_rule" "allow_azure" {
  name             = "allow-azure-services"
  server_id        = azurerm_postgresql_flexible_server.main.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "0.0.0.0"
}
```

**Azure CLI**:

```bash
# Create PostgreSQL flexible server
az postgres flexible-server create \
  --resource-group secret-manager-rg \
  --name secret-manager-db \
  --location eastus \
  --admin-user dbadmin \
  --admin-password 'YOUR_SECURE_PASSWORD' \
  --sku-name Standard_B1ms \
  --tier Burstable \
  --version 15 \
  --storage-size 32 \
  --vnet secret-manager-vnet \
  --subnet postgres-subnet \
  --yes

# Create database
az postgres flexible-server db create \
  --resource-group secret-manager-rg \
  --server-name secret-manager-db \
  --database-name secretmanager
```

### 3. Azure Container Registry

**Terraform** (see `terraform/azure/acr.tf`):

```hcl
resource "azurerm_container_registry" "main" {
  name                = "${var.project_name}acr"  # Must be alphanumeric
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = "Basic"
  admin_enabled       = true
  
  tags = {
    Environment = var.environment
  }
}
```

**Azure CLI**:

```bash
# Create container registry
az acr create \
  --resource-group secret-manager-rg \
  --name secretmanageracr \
  --sku Basic \
  --admin-enabled true
```

### 4. Build and Push Docker Images

```bash
# Login to ACR
az acr login --name secretmanageracr

# Build and push backend
cd backend
az acr build \
  --registry secretmanageracr \
  --image backend:latest \
  --file Dockerfile.prod \
  .

# Build and push frontend
cd ../frontend
az acr build \
  --registry secretmanageracr \
  --image frontend:latest \
  --file Dockerfile.prod \
  --build-arg NEXT_PUBLIC_API_URL=https://secrets.example.com/api \
  .
```

### 5. Container Apps Environment

**Terraform** (see `terraform/azure/container-apps.tf`):

```hcl
resource "azurerm_log_analytics_workspace" "main" {
  name                = "${var.project_name}-logs"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
}

resource "azurerm_container_app_environment" "main" {
  name                       = "${var.project_name}-env"
  location                   = azurerm_resource_group.main.location
  resource_group_name        = azurerm_resource_group.main.name
  log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id
  
  tags = {
    Environment = var.environment
  }
}

# Backend Container App
resource "azurerm_container_app" "backend" {
  name                         = "${var.project_name}-backend"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = "Single"
  
  registry {
    server               = azurerm_container_registry.main.login_server
    username             = azurerm_container_registry.main.admin_username
    password_secret_name = "acr-password"
  }
  
  secret {
    name  = "acr-password"
    value = azurerm_container_registry.main.admin_password
  }
  
  secret {
    name  = "db-password"
    value = var.db_admin_password
  }
  
  secret {
    name  = "jwt-secret"
    value = var.jwt_secret
  }
  
  template {
    container {
      name   = "backend"
      image  = "${azurerm_container_registry.main.login_server}/backend:latest"
      cpu    = 0.5
      memory = "1Gi"
      
      env {
        name  = "DB_HOST"
        value = azurerm_postgresql_flexible_server.main.fqdn
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
        value = var.db_admin_username
      }
      
      env {
        name        = "DB_PASSWORD"
        secret_name = "db-password"
      }
      
      env {
        name        = "JWT_SECRET"
        secret_name = "jwt-secret"
      }
      
      env {
        name  = "PORT"
        value = "8080"
      }
      
      liveness_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        initial_delay_seconds = 10
        period_seconds        = 10
      }
    }
    
    min_replicas = 1
    max_replicas = 10
  }
  
  ingress {
    external_enabled = true
    target_port      = 8080
    
    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }
}

# Frontend Container App
resource "azurerm_container_app" "frontend" {
  name                         = "${var.project_name}-frontend"
  container_app_environment_id = azurerm_container_app_environment.main.id
  resource_group_name          = azurerm_resource_group.main.name
  revision_mode                = "Single"
  
  registry {
    server               = azurerm_container_registry.main.login_server
    username             = azurerm_container_registry.main.admin_username
    password_secret_name = "acr-password"
  }
  
  secret {
    name  = "acr-password"
    value = azurerm_container_registry.main.admin_password
  }
  
  template {
    container {
      name   = "frontend"
      image  = "${azurerm_container_registry.main.login_server}/frontend:latest"
      cpu    = 0.25
      memory = "0.5Gi"
      
      env {
        name  = "NEXT_PUBLIC_API_URL"
        value = "https://${var.domain_name}/api"
      }
      
      env {
        name  = "NODE_ENV"
        value = "production"
      }
    }
    
    min_replicas = 1
    max_replicas = 5
  }
  
  ingress {
    external_enabled = true
    target_port      = 3000
    
    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }
}
```

**Azure CLI**:

```bash
# Create Log Analytics workspace
az monitor log-analytics workspace create \
  --resource-group secret-manager-rg \
  --workspace-name secret-manager-logs

# Get workspace ID
WORKSPACE_ID=$(az monitor log-analytics workspace show \
  --resource-group secret-manager-rg \
  --workspace-name secret-manager-logs \
  --query customerId -o tsv)

WORKSPACE_KEY=$(az monitor log-analytics workspace get-shared-keys \
  --resource-group secret-manager-rg \
  --workspace-name secret-manager-logs \
  --query primarySharedKey -o tsv)

# Create Container Apps environment
az containerapp env create \
  --name secret-manager-env \
  --resource-group secret-manager-rg \
  --location eastus \
  --logs-workspace-id $WORKSPACE_ID \
  --logs-workspace-key $WORKSPACE_KEY

# Deploy backend
az containerapp create \
  --name backend \
  --resource-group secret-manager-rg \
  --environment secret-manager-env \
  --image secretmanageracr.azurecr.io/backend:latest \
  --target-port 8080 \
  --ingress external \
  --registry-server secretmanageracr.azurecr.io \
  --registry-username $(az acr credential show -n secretmanageracr --query username -o tsv) \
  --registry-password $(az acr credential show -n secretmanageracr --query passwords[0].value -o tsv) \
  --cpu 0.5 --memory 1.0 \
  --min-replicas 1 --max-replicas 10 \
  --env-vars \
    DB_HOST=secret-manager-db.postgres.database.azure.com \
    DB_PORT=5432 \
    DB_NAME=secretmanager \
    DB_USER=dbadmin \
    PORT=8080 \
  --secrets \
    db-password=YOUR_DB_PASSWORD \
    jwt-secret=YOUR_JWT_SECRET

# Deploy frontend
az containerapp create \
  --name frontend \
  --resource-group secret-manager-rg \
  --environment secret-manager-env \
  --image secretmanageracr.azurecr.io/frontend:latest \
  --target-port 3000 \
  --ingress external \
  --registry-server secretmanageracr.azurecr.io \
  --registry-username $(az acr credential show -n secretmanageracr --query username -o tsv) \
  --registry-password $(az acr credential show -n secretmanageracr --query passwords[0].value -o tsv) \
  --cpu 0.25 --memory 0.5 \
  --min-replicas 1 --max-replicas 5 \
  --env-vars \
    NEXT_PUBLIC_API_URL=https://secrets.example.com/api \
    NODE_ENV=production
```

### 6. Application Gateway (Optional - for custom domain)

**Terraform** (see `terraform/azure/app-gateway.tf`):

```hcl
resource "azurerm_public_ip" "app_gateway" {
  name                = "${var.project_name}-gw-ip"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  allocation_method   = "Static"
  sku                 = "Standard"
}

resource "azurerm_application_gateway" "main" {
  name                = "${var.project_name}-app-gateway"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  
  sku {
    name     = "Standard_v2"
    tier     = "Standard_v2"
    capacity = 2
  }
  
  gateway_ip_configuration {
    name      = "gateway-ip-config"
    subnet_id = azurerm_subnet.app_gateway.id
  }
  
  frontend_port {
    name = "http-port"
    port = 80
  }
  
  frontend_port {
    name = "https-port"
    port = 443
  }
  
  frontend_ip_configuration {
    name                 = "frontend-ip"
    public_ip_address_id = azurerm_public_ip.app_gateway.id
  }
  
  backend_address_pool {
    name  = "backend-pool"
    fqdns = [azurerm_container_app.backend.ingress[0].fqdn]
  }
  
  backend_address_pool {
    name  = "frontend-pool"
    fqdns = [azurerm_container_app.frontend.ingress[0].fqdn]
  }
  
  backend_http_settings {
    name                  = "backend-http-settings"
    cookie_based_affinity = "Disabled"
    port                  = 443
    protocol              = "Https"
    request_timeout       = 30
  }
  
  backend_http_settings {
    name                  = "frontend-http-settings"
    cookie_based_affinity = "Disabled"
    port                  = 443
    protocol              = "Https"
    request_timeout       = 30
  }
  
  http_listener {
    name                           = "http-listener"
    frontend_ip_configuration_name = "frontend-ip"
    frontend_port_name             = "http-port"
    protocol                       = "Http"
  }
  
  request_routing_rule {
    name                       = "routing-rule"
    rule_type                  = "PathBasedRouting"
    http_listener_name         = "http-listener"
    url_path_map_name          = "path-map"
    priority                   = 100
  }
  
  url_path_map {
    name                               = "path-map"
    default_backend_address_pool_name  = "frontend-pool"
    default_backend_http_settings_name = "frontend-http-settings"
    
    path_rule {
      name                       = "api-rule"
      paths                      = ["/api/*", "/health"]
      backend_address_pool_name  = "backend-pool"
      backend_http_settings_name = "backend-http-settings"
    }
  }
}
```

### 7. Configure DNS

```bash
# Get Application Gateway public IP
export APP_GW_IP=$(az network public-ip show \
  --resource-group secret-manager-rg \
  --name secret-manager-gw-ip \
  --query ipAddress -o tsv)

# Create DNS A record (Azure DNS)
az network dns record-set a add-record \
  --resource-group secret-manager-rg \
  --zone-name example.com \
  --record-set-name secrets \
  --ipv4-address $APP_GW_IP

# Or add to external DNS provider
echo "Add A record: secrets.example.com -> $APP_GW_IP"
```

## Monitoring and Logging

### Azure Monitor Logs

```bash
# Query backend logs
az monitor log-analytics query \
  --workspace $(az monitor log-analytics workspace show \
    --resource-group secret-manager-rg \
    --workspace-name secret-manager-logs \
    --query customerId -o tsv) \
  --analytics-query "ContainerAppConsoleLogs_CL | where ContainerAppName_s == 'backend' | top 50 by TimeGenerated desc"

# Stream logs in real-time
az containerapp logs show \
  --name backend \
  --resource-group secret-manager-rg \
  --follow
```

### Azure Monitor Alerts

**Create alert rules** (Terraform in `terraform/azure/monitoring.tf`):

```hcl
resource "azurerm_monitor_metric_alert" "backend_cpu" {
  name                = "${var.project_name}-backend-cpu-alert"
  resource_group_name = azurerm_resource_group.main.name
  scopes              = [azurerm_container_app.backend.id]
  description         = "Alert when backend CPU exceeds 80%"
  severity            = 2
  
  criteria {
    metric_namespace = "Microsoft.App/containerApps"
    metric_name      = "UsageNanoCores"
    aggregation      = "Average"
    operator         = "GreaterThan"
    threshold        = 400000000  # 0.4 cores (80% of 0.5)
  }
  
  action {
    action_group_id = azurerm_monitor_action_group.main.id
  }
}
```

## Auto-Scaling

Container Apps **auto-scale by default** based on HTTP traffic and custom metrics.

**Configure scaling rules**:

```bash
# HTTP scaling
az containerapp update \
  --name backend \
  --resource-group secret-manager-rg \
  --min-replicas 1 \
  --max-replicas 10 \
  --scale-rule-name http-rule \
  --scale-rule-type http \
  --scale-rule-http-concurrency 50

# CPU-based scaling
az containerapp update \
  --name backend \
  --resource-group secret-manager-rg \
  --scale-rule-name cpu-rule \
  --scale-rule-type cpu \
  --scale-rule-metadata type=Utilization value=70
```

## Cost Estimation

### Small Deployment (Development)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Container Apps | 2 apps @ 50% utilization | ~$20 |
| PostgreSQL | Burstable B1ms (1 vCPU, 2GB) | ~$15 |
| Container Registry | Basic + 10GB storage | ~$5 |
| Log Analytics | 5GB/month | ~$3 |
| VNet | Standard | ~$2 |
| **Total** | | **~$45/month** |

### Medium Deployment (Production)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Container Apps | 4 apps @ 70% utilization | ~$80 |
| PostgreSQL | General Purpose D2s_v3 (2 vCPU, 8GB) | ~$120 |
| Container Registry | Basic + 50GB storage | ~$8 |
| Application Gateway | Standard_v2 | ~$150 |
| Log Analytics | 50GB/month | ~$15 |
| VNet | Standard | ~$5 |
| **Total** | | **~$378/month** |

### Large Deployment (Enterprise)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Container Apps | 10 apps @ 80% utilization + HA | ~$400 |
| PostgreSQL | General Purpose D8s_v3 (8 vCPU, 32GB) + HA | ~$600 |
| Container Registry | Standard + 200GB storage | ~$30 |
| Application Gateway | WAF_v2 + high throughput | ~$250 |
| Log Analytics | 200GB/month | ~$50 |
| VNet | Premium features | ~$10 |
| **Total** | | **~$1,340/month** |

> **Note**: Container Apps pricing is consumption-based (vCPU-seconds and memory-seconds), making it very cost-effective for variable workloads.

## Backup and Disaster Recovery

### Automated Backups

```bash
# Backups are automatic with Azure Database for PostgreSQL
# Configure backup retention
az postgres flexible-server update \
  --resource-group secret-manager-rg \
  --name secret-manager-db \
  --backup-retention 14

# Enable geo-redundant backup (production only)
az postgres flexible-server update \
  --resource-group secret-manager-rg \
  --name secret-manager-db \
  --geo-redundant-backup Enabled
```

### Point-in-Time Restore

```bash
# Restore to specific time
az postgres flexible-server restore \
  --resource-group secret-manager-rg \
  --name secret-manager-db-restored \
  --source-server secret-manager-db \
  --restore-time "2026-03-26T10:00:00Z"
```

### Manual Backups

```bash
# Export database
az postgres flexible-server db export \
  --resource-group secret-manager-rg \
  --server-name secret-manager-db \
  --database-name secretmanager \
  --output-file backup-$(date +%Y%m%d).sql
```

## Security Best Practices

1. **Use Azure Key Vault** for secrets management
2. **Enable Azure Defender** for Container Apps and PostgreSQL
3. **Use Managed Identities** (no passwords in code)
4. **Configure NSGs** for network segmentation
5. **Enable private endpoints** for database access
6. **Configure Azure Firewall** or WAF on Application Gateway
7. **Enable Azure Security Center** for compliance monitoring
8. **Use Azure Policy** for governance

## Troubleshooting

### Container App Not Starting

```bash
# Check revision status
az containerapp revision list \
  --name backend \
  --resource-group secret-manager-rg

# View detailed logs
az containerapp logs show \
  --name backend \
  --resource-group secret-manager-rg \
  --follow \
  --tail 100

# Check replica status
az containerapp replica list \
  --name backend \
  --resource-group secret-manager-rg
```

### Database Connection Issues

```bash
# Test database connectivity
az postgres flexible-server connect \
  --name secret-manager-db \
  --admin-user dbadmin

# Check firewall rules
az postgres flexible-server firewall-rule list \
  --resource-group secret-manager-rg \
  --name secret-manager-db

# Verify private DNS
az network private-dns record-set list \
  --resource-group secret-manager-rg \
  --zone-name secret-manager-postgres.private.postgres.database.azure.com
```

### High Latency

```bash
# Check Container App metrics
az monitor metrics list \
  --resource $(az containerapp show \
    --name backend \
    --resource-group secret-manager-rg \
    --query id -o tsv) \
  --metric "Requests"

# Check Application Gateway health
az network application-gateway show-backend-health \
  --resource-group secret-manager-rg \
  --name secret-manager-app-gateway
```

## Next Steps

- [Configure GitHub Actions for CI/CD](../../.github/workflows/deploy-azure.yml)
- [Set up monitoring dashboards](./MONITORING.md)
- [Configure backups and disaster recovery](./BACKUP.md)
- [Review security hardening guide](./SECURITY.md)

## References

- [Azure Container Apps Documentation](https://learn.microsoft.com/en-us/azure/container-apps/)
- [Azure Database for PostgreSQL](https://learn.microsoft.com/en-us/azure/postgresql/)
- [Terraform AzureRM Provider](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs)
- [Complete Terraform code](../../terraform/azure/)
