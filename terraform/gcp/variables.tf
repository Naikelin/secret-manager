# Project Configuration
variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "secret-manager"
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
}

# Database
variable "db_tier" {
  description = "Cloud SQL tier"
  type        = string
  default     = "db-f1-micro"  # db-custom-2-7680 for production
}

variable "db_user" {
  description = "Database user"
  type        = string
  default     = "secretmanager"
  sensitive   = true
}

variable "db_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}

# Application
variable "jwt_secret" {
  description = "JWT secret key"
  type        = string
  sensitive   = true
}

variable "domain_name" {
  description = "Domain name for the application"
  type        = string
}
