# Cloud SQL
output "cloudsql_instance_name" {
  description = "Name of Cloud SQL instance"
  value       = google_sql_database_instance.postgres.name
}

output "cloudsql_connection_name" {
  description = "Cloud SQL connection name"
  value       = google_sql_database_instance.postgres.connection_name
}

# Cloud Run
output "backend_url" {
  description = "URL of backend Cloud Run service"
  value       = google_cloud_run_service.backend.status[0].url
}

output "frontend_url" {
  description = "URL of frontend Cloud Run service"
  value       = google_cloud_run_service.frontend.status[0].url
}

# Load Balancer
output "lb_ip_address" {
  description = "IP address of load balancer"
  value       = google_compute_global_address.main.address
}

output "application_url" {
  description = "URL of the application"
  value       = "https://${var.domain_name}"
}
