variable "project_name" {
  description = "Project name"
  type        = string
  default     = "secret-manager"
}

variable "location" {
  description = "Azure region"
  type        = string
  default     = "eastus"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
}

variable "db_admin_username" {
  description = "Database admin username"
  type        = string
  default     = "dbadmin"
  sensitive   = true
}

variable "db_admin_password" {
  description = "Database admin password"
  type        = string
  sensitive   = true
}

variable "db_sku_name" {
  description = "PostgreSQL SKU"
  type        = string
  default     = "B_Standard_B1ms"  # GP_Standard_D2s_v3 for production
}

variable "jwt_secret" {
  description = "JWT secret"
  type        = string
  sensitive   = true
}

variable "domain_name" {
  description = "Domain name"
  type        = string
}
