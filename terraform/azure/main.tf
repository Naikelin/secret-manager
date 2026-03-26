# ==========================================
# Azure Secret Manager Infrastructure
# Terraform configuration for Container Apps deployment
# ==========================================

# Resource Group
resource "azurerm_resource_group" "main" {
  name     = "${var.project_name}-rg"
  location = var.location
}

# Virtual Network
resource "azurerm_virtual_network" "main" {
  name                = "${var.project_name}-vnet"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  address_space       = ["10.0.0.0/16"]
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
      actions = ["Microsoft.Network/virtualNetworks/subnets/join/action"]
    }
  }
}

# Azure Container Registry
resource "azurerm_container_registry" "main" {
  name                = "${replace(var.project_name, "-", "")}acr"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = "Basic"
  admin_enabled       = true
}

# PostgreSQL Flexible Server
resource "azurerm_private_dns_zone" "postgres" {
  name                = "${var.project_name}.postgres.database.azure.com"
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
  storage_mb             = 32768
  sku_name               = var.db_sku_name
  backup_retention_days  = 7
  
  depends_on = [azurerm_private_dns_zone_virtual_network_link.postgres]
}

resource "azurerm_postgresql_flexible_server_database" "main" {
  name      = "secretmanager"
  server_id = azurerm_postgresql_flexible_server.main.id
  collation = "en_US.utf8"
  charset   = "UTF8"
}

# Log Analytics Workspace
resource "azurerm_log_analytics_workspace" "main" {
  name                = "${var.project_name}-logs"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
}

# Container Apps Environment
resource "azurerm_container_app_environment" "main" {
  name                       = "${var.project_name}-env"
  location                   = azurerm_resource_group.main.location
  resource_group_name        = azurerm_resource_group.main.name
  log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id
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
