# AWS Deployment Guide

Deploy Secret Manager on AWS using ECS Fargate, RDS PostgreSQL, and Application Load Balancer.

## Architecture Overview

```
┌──────────────┐     ┌───────────────┐     ┌──────────────┐
│   Route 53   │────▶│ Application   │────▶│ ECS Fargate  │
│     DNS      │     │ Load Balancer │     │   Cluster    │
└──────────────┘     └───────────────┘     └──────────────┘
                            │                      │
                            │                      ├─ Backend Task
                            │                      ├─ Frontend Task
                            ▼                      │
                     ┌──────────────┐              ▼
                     │  ACM (SSL)   │     ┌──────────────┐
                     └──────────────┘     │ RDS Postgres │
                                          │   Instance   │
                     ┌──────────────┐     └──────────────┘
                     │ ECR Registry │
                     └──────────────┘     ┌──────────────┐
                                          │  CloudWatch  │
                     ┌──────────────┐     │ Logs/Metrics │
                     │     VPC      │     └──────────────┘
                     │ Public/Priv  │
                     │   Subnets    │
                     └──────────────┘
```

### Components

- **Compute**: ECS Fargate (serverless containers)
- **Database**: RDS PostgreSQL 15
- **Load Balancer**: Application Load Balancer (ALB)
- **Container Registry**: Elastic Container Registry (ECR)
- **Monitoring**: CloudWatch Logs and Metrics
- **SSL/TLS**: AWS Certificate Manager (ACM)
- **Networking**: VPC with public/private subnets

## Prerequisites

### Required Tools

- AWS CLI v2 (configured with credentials)
- Terraform v1.5+ (if using IaC)
- Docker v20+
- Git

### AWS Account Requirements

- AWS account with billing enabled
- IAM user with sufficient permissions:
  - EC2, ECS, ECR
  - RDS
  - VPC, ALB, Route53
  - ACM, CloudWatch
  - IAM (for service roles)

### Domain Setup

- Domain name registered (Route53 or external)
- ACM certificate for your domain (in the same region)

## Quick Start

### Option 1: Terraform (Recommended)

```bash
# 1. Clone repository
git clone https://github.com/Naikelin/secret-manager.git
cd secret-manager

# 2. Configure Terraform variables
cd terraform/aws
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values

# 3. Initialize and apply
terraform init
terraform plan
terraform apply

# 4. Build and push Docker images
./scripts/deploy-aws.sh
```

### Option 2: CloudFormation

```bash
# Deploy using CloudFormation
aws cloudformation create-stack \
  --stack-name secret-manager \
  --template-body file://cloudformation/aws-stack.yaml \
  --parameters \
    ParameterKey=Environment,ParameterValue=production \
    ParameterKey=DomainName,ParameterValue=secrets.example.com \
  --capabilities CAPABILITY_IAM
```

## Infrastructure Setup

### 1. VPC and Networking

Create a VPC with public and private subnets across multiple availability zones.

**Terraform** (see `terraform/aws/vpc.tf`):

```hcl
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "${var.project_name}-vpc"
  }
}

# Public subnets for ALB
resource "aws_subnet" "public" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index + 1}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]
  
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.project_name}-public-${count.index + 1}"
  }
}

# Private subnets for ECS tasks and RDS
resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index + 10}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "${var.project_name}-private-${count.index + 1}"
  }
}
```

**AWS CLI**:

```bash
# Create VPC
VPC_ID=$(aws ec2 create-vpc \
  --cidr-block 10.0.0.0/16 \
  --query 'Vpc.VpcId' \
  --output text)

# Enable DNS
aws ec2 modify-vpc-attribute \
  --vpc-id $VPC_ID \
  --enable-dns-hostnames

# Create Internet Gateway
IGW_ID=$(aws ec2 create-internet-gateway \
  --query 'InternetGateway.InternetGatewayId' \
  --output text)

aws ec2 attach-internet-gateway \
  --vpc-id $VPC_ID \
  --internet-gateway-id $IGW_ID

# Create subnets (repeat for each AZ)
aws ec2 create-subnet \
  --vpc-id $VPC_ID \
  --cidr-block 10.0.1.0/24 \
  --availability-zone us-east-1a
```

### 2. RDS PostgreSQL Instance

**Terraform** (see `terraform/aws/rds.tf`):

```hcl
resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-db-subnet"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "${var.project_name}-db-subnet-group"
  }
}

resource "aws_db_instance" "postgres" {
  identifier     = "${var.project_name}-db"
  engine         = "postgres"
  engine_version = "15.4"
  
  instance_class    = var.db_instance_class  # db.t3.micro for dev
  allocated_storage = 20
  storage_type      = "gp3"
  storage_encrypted = true
  
  db_name  = "secretmanager"
  username = var.db_username
  password = var.db_password
  
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  
  backup_retention_period = 7
  backup_window          = "03:00-04:00"
  maintenance_window     = "mon:04:00-mon:05:00"
  
  skip_final_snapshot = var.environment != "production"
  
  tags = {
    Name = "${var.project_name}-postgres"
  }
}
```

**AWS CLI**:

```bash
# Create DB subnet group
aws rds create-db-subnet-group \
  --db-subnet-group-name secret-manager-db-subnet \
  --db-subnet-group-description "Secret Manager DB Subnets" \
  --subnet-ids subnet-xxx subnet-yyy

# Create RDS instance
aws rds create-db-instance \
  --db-instance-identifier secret-manager-db \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --engine-version 15.4 \
  --master-username admin \
  --master-user-password YOUR_SECURE_PASSWORD \
  --allocated-storage 20 \
  --vpc-security-group-ids sg-xxx \
  --db-subnet-group-name secret-manager-db-subnet \
  --backup-retention-period 7 \
  --storage-encrypted \
  --publicly-accessible false
```

### 3. ECR Repositories

**Terraform** (see `terraform/aws/ecr.tf`):

```hcl
resource "aws_ecr_repository" "backend" {
  name                 = "${var.project_name}/backend"
  image_tag_mutability = "MUTABLE"
  
  image_scanning_configuration {
    scan_on_push = true
  }
  
  tags = {
    Name = "${var.project_name}-backend"
  }
}

resource "aws_ecr_repository" "frontend" {
  name                 = "${var.project_name}/frontend"
  image_tag_mutability = "MUTABLE"
  
  image_scanning_configuration {
    scan_on_push = true
  }
  
  tags = {
    Name = "${var.project_name}-frontend"
  }
}
```

**AWS CLI**:

```bash
# Create repositories
aws ecr create-repository --repository-name secret-manager/backend
aws ecr create-repository --repository-name secret-manager/frontend
```

### 4. ECS Cluster and Task Definitions

**Terraform** (see `terraform/aws/ecs.tf`):

```hcl
resource "aws_ecs_cluster" "main" {
  name = "${var.project_name}-cluster"
  
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
  
  tags = {
    Name = var.project_name
  }
}

# Backend Task Definition
resource "aws_ecs_task_definition" "backend" {
  family                   = "${var.project_name}-backend"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "512"
  memory                   = "1024"
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn
  
  container_definitions = jsonencode([{
    name  = "backend"
    image = "${aws_ecr_repository.backend.repository_url}:latest"
    
    portMappings = [{
      containerPort = 8080
      protocol      = "tcp"
    }]
    
    environment = [
      {
        name  = "DB_HOST"
        value = aws_db_instance.postgres.address
      },
      {
        name  = "DB_PORT"
        value = "5432"
      },
      {
        name  = "DB_NAME"
        value = "secretmanager"
      },
      {
        name  = "DB_USER"
        value = var.db_username
      },
      {
        name  = "PORT"
        value = "8080"
      },
      {
        name  = "LOG_LEVEL"
        value = "info"
      }
    ]
    
    secrets = [
      {
        name      = "DB_PASSWORD"
        valueFrom = aws_secretsmanager_secret.db_password.arn
      },
      {
        name      = "JWT_SECRET"
        valueFrom = aws_secretsmanager_secret.jwt_secret.arn
      }
    ]
    
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.backend.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "backend"
      }
    }
    
    healthCheck = {
      command     = ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"]
      interval    = 30
      timeout     = 5
      retries     = 3
      startPeriod = 60
    }
  }])
}

# Frontend Task Definition
resource "aws_ecs_task_definition" "frontend" {
  family                   = "${var.project_name}-frontend"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  
  container_definitions = jsonencode([{
    name  = "frontend"
    image = "${aws_ecr_repository.frontend.repository_url}:latest"
    
    portMappings = [{
      containerPort = 3000
      protocol      = "tcp"
    }]
    
    environment = [
      {
        name  = "NEXT_PUBLIC_API_URL"
        value = "https://${var.domain_name}/api"
      },
      {
        name  = "NODE_ENV"
        value = "production"
      }
    ]
    
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.frontend.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "frontend"
      }
    }
  }])
}

# ECS Services
resource "aws_ecs_service" "backend" {
  name            = "${var.project_name}-backend"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.backend.arn
  desired_count   = var.backend_desired_count
  launch_type     = "FARGATE"
  
  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = false
  }
  
  load_balancer {
    target_group_arn = aws_lb_target_group.backend.arn
    container_name   = "backend"
    container_port   = 8080
  }
  
  depends_on = [aws_lb_listener.https]
}

resource "aws_ecs_service" "frontend" {
  name            = "${var.project_name}-frontend"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.frontend.arn
  desired_count   = var.frontend_desired_count
  launch_type     = "FARGATE"
  
  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = false
  }
  
  load_balancer {
    target_group_arn = aws_lb_target_group.frontend.arn
    container_name   = "frontend"
    container_port   = 3000
  }
  
  depends_on = [aws_lb_listener.https]
}
```

### 5. Application Load Balancer

**Terraform** (see `terraform/aws/alb.tf`):

```hcl
resource "aws_lb" "main" {
  name               = "${var.project_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id
  
  enable_deletion_protection = var.environment == "production"
  
  tags = {
    Name = "${var.project_name}-alb"
  }
}

# Target Groups
resource "aws_lb_target_group" "backend" {
  name        = "${var.project_name}-backend-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"
  
  health_check {
    enabled             = true
    path                = "/health"
    port                = "traffic-port"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    matcher             = "200"
  }
  
  deregistration_delay = 30
}

resource "aws_lb_target_group" "frontend" {
  name        = "${var.project_name}-frontend-tg"
  port        = 3000
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"
  
  health_check {
    enabled             = true
    path                = "/api/health"
    port                = "traffic-port"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    matcher             = "200"
  }
  
  deregistration_delay = 30
}

# Listeners
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = "80"
  protocol          = "HTTP"
  
  default_action {
    type = "redirect"
    
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.main.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS-1-2-2017-01"
  certificate_arn   = var.acm_certificate_arn
  
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.frontend.arn
  }
}

# Path-based routing
resource "aws_lb_listener_rule" "api" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 100
  
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.backend.arn
  }
  
  condition {
    path_pattern {
      values = ["/api/*", "/health"]
    }
  }
}
```

## Deployment Steps

### 1. Build Docker Images

```bash
# Backend
cd backend
docker build -t secret-manager-backend:latest -f Dockerfile.prod .

# Frontend
cd ../frontend
docker build -t secret-manager-frontend:latest -f Dockerfile.prod \
  --build-arg NEXT_PUBLIC_API_URL=https://secrets.example.com/api .
```

### 2. Push to ECR

```bash
# Get ECR login
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com

# Tag and push backend
docker tag secret-manager-backend:latest \
  ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/secret-manager/backend:latest
docker push ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/secret-manager/backend:latest

# Tag and push frontend
docker tag secret-manager-frontend:latest \
  ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/secret-manager/frontend:latest
docker push ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/secret-manager/frontend:latest
```

### 3. Deploy to ECS

```bash
# Update ECS services (forces new deployment)
aws ecs update-service \
  --cluster secret-manager-cluster \
  --service secret-manager-backend \
  --force-new-deployment

aws ecs update-service \
  --cluster secret-manager-cluster \
  --service secret-manager-frontend \
  --force-new-deployment
```

### 4. Verify Deployment

```bash
# Check service status
aws ecs describe-services \
  --cluster secret-manager-cluster \
  --services secret-manager-backend secret-manager-frontend

# Check task health
aws ecs list-tasks --cluster secret-manager-cluster
aws ecs describe-tasks --cluster secret-manager-cluster --tasks TASK_ARN

# Check ALB health
aws elbv2 describe-target-health \
  --target-group-arn TARGET_GROUP_ARN
```

## SSL/TLS with ACM

### Request Certificate

```bash
# Request certificate for your domain
aws acm request-certificate \
  --domain-name secrets.example.com \
  --validation-method DNS \
  --subject-alternative-names "*.secrets.example.com"

# Validate via DNS (add CNAME record to your DNS)
aws acm describe-certificate --certificate-arn CERT_ARN
```

### Configure DNS

```bash
# Create Route53 A record pointing to ALB
aws route53 change-resource-record-sets \
  --hosted-zone-id ZONE_ID \
  --change-batch '{
    "Changes": [{
      "Action": "CREATE",
      "ResourceRecordSet": {
        "Name": "secrets.example.com",
        "Type": "A",
        "AliasTarget": {
          "HostedZoneId": "ALB_HOSTED_ZONE_ID",
          "DNSName": "ALB_DNS_NAME",
          "EvaluateTargetHealth": true
        }
      }
    }]
  }'
```

## Monitoring and Logging

### CloudWatch Logs

```bash
# View backend logs
aws logs tail /ecs/secret-manager/backend --follow

# View frontend logs
aws logs tail /ecs/secret-manager/frontend --follow
```

### CloudWatch Metrics

Key metrics to monitor:

- **ECS**: CPU/Memory utilization, task count
- **ALB**: Request count, target response time, HTTP errors
- **RDS**: CPU, connections, storage, IOPS

**Create alarms** (Terraform in `terraform/aws/monitoring.tf`):

```hcl
resource "aws_cloudwatch_metric_alarm" "backend_cpu" {
  alarm_name          = "${var.project_name}-backend-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ECS"
  period              = "300"
  statistic           = "Average"
  threshold           = "80"
  alarm_description   = "Backend CPU utilization too high"
  
  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.backend.name
  }
}
```

## Auto-Scaling

### ECS Service Auto-Scaling

**Terraform** (see `terraform/aws/autoscaling.tf`):

```hcl
resource "aws_appautoscaling_target" "backend" {
  max_capacity       = 10
  min_capacity       = 2
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.backend.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "backend_cpu" {
  name               = "${var.project_name}-backend-cpu-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.backend.resource_id
  scalable_dimension = aws_appautoscaling_target.backend.scalable_dimension
  service_namespace  = aws_appautoscaling_target.backend.service_namespace
  
  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value = 70.0
  }
}
```

## Cost Estimation

### Small Deployment (Development)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| ECS Fargate | 2 tasks (0.5 vCPU, 1GB) | ~$30 |
| RDS PostgreSQL | db.t3.micro (1 vCPU, 1GB) | ~$15 |
| ALB | 1 ALB | ~$20 |
| Data Transfer | 100GB/month | ~$9 |
| ECR | 10GB storage | ~$1 |
| CloudWatch | Logs & Metrics | ~$5 |
| **Total** | | **~$80/month** |

### Medium Deployment (Production)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| ECS Fargate | 4 tasks (1 vCPU, 2GB) | ~$120 |
| RDS PostgreSQL | db.t3.medium (2 vCPU, 4GB) | ~$70 |
| ALB | 1 ALB + data processing | ~$30 |
| Data Transfer | 500GB/month | ~$45 |
| ECR | 50GB storage | ~$5 |
| CloudWatch | Enhanced monitoring | ~$15 |
| **Total** | | **~$285/month** |

### Large Deployment (Enterprise)

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| ECS Fargate | 10 tasks (2 vCPU, 4GB) | ~$600 |
| RDS PostgreSQL | db.r6g.xlarge (4 vCPU, 32GB) + Multi-AZ | ~$500 |
| ALB | 1 ALB + high data processing | ~$50 |
| Data Transfer | 2TB/month | ~$180 |
| ECR | 200GB storage | ~$20 |
| CloudWatch | Full observability | ~$50 |
| **Total** | | **~$1,400/month** |

> **Note**: Prices are estimates for US East (N. Virginia) region and may vary.

## Backup and Disaster Recovery

### RDS Automated Backups

```hcl
# In terraform/aws/rds.tf
resource "aws_db_instance" "postgres" {
  # ... other config ...
  
  backup_retention_period = 7
  backup_window          = "03:00-04:00"
  maintenance_window     = "mon:04:00-mon:05:00"
  
  # Enable Multi-AZ for high availability
  multi_az = var.environment == "production"
}
```

### Manual Snapshots

```bash
# Create manual snapshot
aws rds create-db-snapshot \
  --db-instance-identifier secret-manager-db \
  --db-snapshot-identifier secret-manager-snapshot-$(date +%Y%m%d)

# Restore from snapshot
aws rds restore-db-instance-from-db-snapshot \
  --db-instance-identifier secret-manager-db-restored \
  --db-snapshot-identifier secret-manager-snapshot-20260326
```

## Security Best Practices

1. **Use AWS Secrets Manager** for sensitive values (DB password, JWT secret)
2. **Enable VPC Flow Logs** for network monitoring
3. **Restrict security groups** to minimum required access
4. **Enable RDS encryption at rest**
5. **Use IAM roles** for ECS task permissions (not access keys)
6. **Enable AWS Shield** for DDoS protection
7. **Configure AWS WAF** on ALB for web application firewall
8. **Enable CloudTrail** for audit logging

## Troubleshooting

### Tasks Not Starting

```bash
# Check ECS events
aws ecs describe-services \
  --cluster secret-manager-cluster \
  --services secret-manager-backend \
  --query 'services[0].events[:5]'

# Check task stopped reason
aws ecs describe-tasks \
  --cluster secret-manager-cluster \
  --tasks TASK_ARN \
  --query 'tasks[0].stoppedReason'
```

### Database Connection Issues

```bash
# Test connectivity from ECS task
aws ecs execute-command \
  --cluster secret-manager-cluster \
  --task TASK_ARN \
  --container backend \
  --command "nc -zv RDS_ENDPOINT 5432" \
  --interactive
```

### High Costs

1. Review CloudWatch metrics for over-provisioned resources
2. Enable ECS Service Auto Scaling to reduce idle capacity
3. Consider RDS Reserved Instances for long-term savings
4. Use Fargate Spot for non-critical workloads (up to 70% savings)

## Next Steps

- [Configure GitHub Actions for CI/CD](../../.github/workflows/deploy-aws.yml)
- [Set up monitoring dashboards](./MONITORING.md)
- [Configure backups and disaster recovery](./BACKUP.md)
- [Review security hardening guide](./SECURITY.md)

## References

- [AWS ECS Best Practices](https://docs.aws.amazon.com/AmazonECS/latest/bestpracticesguide/)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
- [Complete Terraform code](../../terraform/aws/)
