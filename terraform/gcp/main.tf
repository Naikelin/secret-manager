# ==========================================
# GCP Secret Manager Infrastructure
# Complete Terraform configuration for Cloud Run deployment
# ==========================================

# Enable required APIs
resource "google_project_service" "services" {
  for_each = toset([
    "run.googleapis.com",
    "sqladmin.googleapis.com",
    "compute.googleapis.com",
    "vpcaccess.googleapis.com",
    "artifactregistry.googleapis.com",
    "secretmanager.googleapis.com"
  ])
  service            = each.value
  disable_on_destroy = false
}

# VPC and Networking
resource "google_compute_network" "main" {
  name                    = "${var.project_name}-vpc"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.services]
}

resource "google_compute_subnetwork" "main" {
  name          = "${var.project_name}-subnet"
  ip_cidr_range = "10.0.0.0/24"
  region        = var.region
  network       = google_compute_network.main.id
}

# VPC Connector for Cloud Run
resource "google_vpc_access_connector" "main" {
  name          = "${var.project_name}-connector"
  region        = var.region
  network       = google_compute_network.main.name
  ip_cidr_range = "10.8.0.0/28"
  min_instances = 2
  max_instances = 3
}

# Cloud SQL
resource "google_sql_database_instance" "postgres" {
  name             = "${var.project_name}-db"
  database_version = "POSTGRES_15"
  region           = var.region
  depends_on       = [google_project_service.services]
  
  settings {
    tier              = var.db_tier
    availability_type = var.environment == "production" ? "REGIONAL" : "ZONAL"
    disk_size         = 20
    disk_type         = "PD_SSD"
    disk_autoresize   = true
    
    backup_configuration {
      enabled                        = true
      start_time                     = "03:00"
      point_in_time_recovery_enabled = var.environment == "production"
      transaction_log_retention_days = 7
    }
    
    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.main.id
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

# Secret Manager
resource "google_secret_manager_secret" "db_password" {
  secret_id = "${var.project_name}-db-password"
  replication {
    automatic = true
  }
  depends_on = [google_project_service.services]
}

resource "google_secret_manager_secret_version" "db_password" {
  secret      = google_secret_manager_secret.db_password.id
  secret_data = var.db_password
}

resource "google_secret_manager_secret" "jwt_secret" {
  secret_id = "${var.project_name}-jwt-secret"
  replication {
    automatic = true
  }
}

resource "google_secret_manager_secret_version" "jwt_secret" {
  secret      = google_secret_manager_secret.jwt_secret.id
  secret_data = var.jwt_secret
}

# Artifact Registry
resource "google_artifact_registry_repository" "main" {
  location      = var.region
  repository_id = var.project_name
  description   = "Secret Manager container images"
  format        = "DOCKER"
  depends_on    = [google_project_service.services]
}

# Service Accounts
resource "google_service_account" "backend" {
  account_id   = "${var.project_name}-backend"
  display_name = "Backend Cloud Run Service Account"
}

resource "google_project_iam_member" "backend_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_secret_manager_secret_iam_member" "backend_db_password" {
  secret_id = google_secret_manager_secret.db_password.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_secret_manager_secret_iam_member" "backend_jwt_secret" {
  secret_id = google_secret_manager_secret.jwt_secret.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend.email}"
}

# Cloud Run Backend
resource "google_cloud_run_service" "backend" {
  name     = "${var.project_name}-backend"
  location = var.region
  
  template {
    spec {
      service_account_name = google_service_account.backend.email
      
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
    }
    
    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale"        = "1"
        "autoscaling.knative.dev/maxScale"        = "10"
        "run.googleapis.com/cloudsql-instances"   = "${var.project_id}:${var.region}:${google_sql_database_instance.postgres.name}"
        "run.googleapis.com/vpc-access-connector" = google_vpc_access_connector.main.id
        "run.googleapis.com/vpc-access-egress"    = "private-ranges-only"
      }
    }
  }
  
  traffic {
    percent         = 100
    latest_revision = true
  }
  
  depends_on = [google_project_service.services]
}

# Cloud Run Frontend
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
  
  depends_on = [google_project_service.services]
}

# Allow unauthenticated access
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

# Load Balancer (Optional for custom domain)
resource "google_compute_global_address" "main" {
  name = "${var.project_name}-ip"
}

resource "google_compute_region_network_endpoint_group" "backend" {
  name                  = "${var.project_name}-backend-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  
  cloud_run {
    service = google_cloud_run_service.backend.name
  }
}

resource "google_compute_region_network_endpoint_group" "frontend" {
  name                  = "${var.project_name}-frontend-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  
  cloud_run {
    service = google_cloud_run_service.frontend.name
  }
}

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

resource "google_compute_managed_ssl_certificate" "main" {
  name = "${var.project_name}-cert"
  
  managed {
    domains = [var.domain_name]
  }
}

resource "google_compute_target_https_proxy" "main" {
  name             = "${var.project_name}-https-proxy"
  url_map          = google_compute_url_map.main.id
  ssl_certificates = [google_compute_managed_ssl_certificate.main.id]
}

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
