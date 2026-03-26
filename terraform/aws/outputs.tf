# VPC
output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "private_subnet_ids" {
  description = "IDs of private subnets"
  value       = aws_subnet.private[*].id
}

output "public_subnet_ids" {
  description = "IDs of public subnets"
  value       = aws_subnet.public[*].id
}

# Database
output "db_endpoint" {
  description = "RDS instance endpoint"
  value       = aws_db_instance.postgres.endpoint
}

output "db_address" {
  description = "RDS instance address"
  value       = aws_db_instance.postgres.address
}

# ECR
output "backend_ecr_repository_url" {
  description = "URL of backend ECR repository"
  value       = aws_ecr_repository.backend.repository_url
}

output "frontend_ecr_repository_url" {
  description = "URL of frontend ECR repository"
  value       = aws_ecr_repository.frontend.repository_url
}

# ECS
output "ecs_cluster_name" {
  description = "Name of ECS cluster"
  value       = aws_ecs_cluster.main.name
}

output "backend_service_name" {
  description = "Name of backend ECS service"
  value       = aws_ecs_service.backend.name
}

output "frontend_service_name" {
  description = "Name of frontend ECS service"
  value       = aws_ecs_service.frontend.name
}

# Load Balancer
output "alb_dns_name" {
  description = "DNS name of the ALB"
  value       = aws_lb.main.dns_name
}

output "alb_zone_id" {
  description = "Zone ID of the ALB for Route53 alias"
  value       = aws_lb.main.zone_id
}

output "alb_url" {
  description = "URL of the application"
  value       = "https://${var.domain_name}"
}

# CloudWatch
output "backend_log_group" {
  description = "CloudWatch log group for backend"
  value       = aws_cloudwatch_log_group.backend.name
}

output "frontend_log_group" {
  description = "CloudWatch log group for frontend"
  value       = aws_cloudwatch_log_group.frontend.name
}
