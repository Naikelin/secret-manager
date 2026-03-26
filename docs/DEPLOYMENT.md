# Production Deployment Guide

This guide covers deploying Secret Manager in a production environment using Docker Compose.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Initial Setup](#initial-setup)
3. [SSL Certificate Setup](#ssl-certificate-setup)
4. [Secret Keys Setup](#secret-keys-setup)
5. [Database Initialization](#database-initialization)
6. [Starting Services](#starting-services)
7. [Health Checks](#health-checks)
8. [Monitoring](#monitoring)
9. [Backup Strategy](#backup-strategy)
10. [Scaling](#scaling)
11. [Updates](#updates)
12. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### System Requirements

- **Server**: Linux server (Ubuntu 20.04+ or similar)
- **CPU**: 2+ cores recommended
- **RAM**: 4GB minimum, 8GB recommended
- **Disk**: 20GB minimum (more for secrets repository history)
- **Docker**: Version 20.10+ with Docker Compose v2
- **Domain**: Registered domain with DNS configured
- **Ports**: 80 (HTTP) and 443 (HTTPS) open in firewall

### Install Docker

```bash
# Update system
sudo apt-get update
sudo apt-get upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Verify installation
docker --version
docker compose version
```

### Clone Repository

```bash
git clone https://github.com/yourorg/secret-manager.git
cd secret-manager
```

---

## Initial Setup

### 1. Create Production Environment File

```bash
# Copy example environment file
cp .env.prod.example .env.prod

# Edit with your values
nano .env.prod
```

### 2. Configure Required Environment Variables

Edit `.env.prod` and set the following **required** values:

```bash
# Database password (use strong password)
DB_PASSWORD=your_strong_password_here

# JWT secret (generate with: openssl rand -base64 64)
JWT_SECRET=your_jwt_secret_here

# OAuth credentials
OAUTH_CLIENT_ID=your_oauth_client_id
OAUTH_CLIENT_SECRET=your_oauth_client_secret
OAUTH_ISSUER_URL=https://your-oauth-provider.com
OAUTH_REDIRECT_URL=https://your-domain.com/api/auth/callback

# NextAuth secret (generate with: openssl rand -base64 32)
NEXTAUTH_SECRET=your_nextauth_secret_here
NEXTAUTH_URL=https://your-domain.com

# Domain configuration
API_URL=https://your-domain.com

# Git repository
GIT_REPO_URL=git@github.com:your-org/secrets-repo.git
```

### 3. Validate Environment Configuration

```bash
# Run validation script
./scripts/deploy-prod.sh validate
```

---

## SSL Certificate Setup

### Option A: Let's Encrypt (Recommended)

```bash
# Install Certbot
sudo apt-get install certbot

# Stop nginx if running
docker compose -f docker-compose.prod.yml stop nginx

# Obtain certificate
sudo certbot certonly --standalone -d your-domain.com -d www.your-domain.com

# Copy certificates
sudo cp /etc/letsencrypt/live/your-domain.com/fullchain.pem nginx/ssl/cert.pem
sudo cp /etc/letsencrypt/live/your-domain.com/privkey.pem nginx/ssl/key.pem

# Set permissions
chmod 644 nginx/ssl/cert.pem
chmod 600 nginx/ssl/key.pem

# Enable HTTPS configuration
cp nginx/conf.d/ssl.conf.example nginx/conf.d/ssl.conf
# Edit ssl.conf and replace 'your-domain.com' with your actual domain
sed -i 's/your-domain.com/yourdomain.com/g' nginx/conf.d/ssl.conf
```

### Option B: Self-Signed Certificate (Testing Only)

```bash
# Generate self-signed certificate
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx/ssl/key.pem \
  -out nginx/ssl/cert.pem \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=your-domain.com"

chmod 644 nginx/ssl/cert.pem
chmod 600 nginx/ssl/key.pem
```

### Auto-Renewal Setup (Let's Encrypt)

```bash
# Add cron job for renewal
(crontab -l 2>/dev/null; echo "0 0 1 * * certbot renew --quiet && docker compose -f $PWD/docker-compose.prod.yml restart nginx") | crontab -
```

See [nginx/ssl/README.md](../nginx/ssl/README.md) for more details.

---

## Secret Keys Setup

### 1. Create Keys Directory

```bash
mkdir -p prod-keys/{age-keys,ssh-keys,kubeconfig}
chmod 700 prod-keys
```

### 2. AGE Encryption Keys

```bash
# Generate AGE key pair
age-keygen -o prod-keys/age-keys/keys.txt

# Set permissions
chmod 600 prod-keys/age-keys/keys.txt

# Extract public key for sharing with team
grep "public key:" prod-keys/age-keys/keys.txt
```

### 3. SSH Keys (for Git Authentication)

```bash
# Generate SSH key pair
ssh-keygen -t ed25519 -C "secretmanager@your-domain.com" -f prod-keys/ssh-keys/id_rsa -N ""

# Set permissions
chmod 600 prod-keys/ssh-keys/id_rsa
chmod 644 prod-keys/ssh-keys/id_rsa.pub

# Add public key to your Git repository
cat prod-keys/ssh-keys/id_rsa.pub
# Copy this and add as deploy key in GitHub/GitLab
```

### 4. Kubernetes Config (if using K8s integration)

```bash
# Copy your kubeconfig
cp ~/.kube/config prod-keys/kubeconfig/config

# Or create service account token
# See: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/

# Set permissions
chmod 600 prod-keys/kubeconfig/config
```

### 5. Secure the Keys Directory

```bash
# Ensure proper ownership
chown -R $USER:$USER prod-keys

# Verify permissions
ls -la prod-keys/*
```

**IMPORTANT**: Never commit `prod-keys/` to version control. It's already in `.gitignore`.

---

## Database Initialization

The database will be automatically initialized on first startup using migrations in `backend/migrations/`.

### Manual Migration (Optional)

If you need to run migrations manually:

```bash
# Start only database
docker compose -f docker-compose.prod.yml up -d postgres

# Wait for postgres to be healthy
docker compose -f docker-compose.prod.yml exec postgres pg_isready -U secretmanager

# Run migrations (backend will do this automatically)
# Or connect and run SQL files manually
docker compose -f docker-compose.prod.yml exec postgres psql -U secretmanager -d secretmanager -f /docker-entrypoint-initdb.d/001_init.sql
```

---

## Starting Services

### 1. Build Images

```bash
# Build production images
docker compose -f docker-compose.prod.yml build

# This will:
# - Build backend with optimized Go binary
# - Build frontend with production Next.js build
# - Pull nginx:alpine image
```

### 2. Start All Services

```bash
# Start in detached mode
docker compose -f docker-compose.prod.yml up -d

# View logs
docker compose -f docker-compose.prod.yml logs -f
```

### 3. Verify Services

```bash
# Check service status
docker compose -f docker-compose.prod.yml ps

# Expected output:
# NAME                          STATUS
# secretmanager-postgres-prod   Up (healthy)
# secretmanager-backend-prod    Up (healthy)
# secretmanager-frontend-prod   Up
# secretmanager-nginx-prod      Up (healthy)
```

### 4. Using the Deployment Script

```bash
# Full deployment (build, migrate, start)
./scripts/deploy-prod.sh deploy

# Or individual steps
./scripts/deploy-prod.sh build
./scripts/deploy-prod.sh start
./scripts/deploy-prod.sh status
```

---

## Health Checks

### Service Health Endpoints

```bash
# Nginx health check
curl http://localhost/health
# Expected: "healthy"

# Backend health check
curl http://localhost/api/health
# Expected: {"status":"ok"}

# Frontend (via nginx)
curl -I http://localhost/
# Expected: HTTP 200
```

### Container Health Status

```bash
# Check Docker health status
docker compose -f docker-compose.prod.yml ps

# View health check logs
docker inspect secretmanager-backend-prod | jq '.[0].State.Health'
```

### Database Connection

```bash
# Test database connection
docker compose -f docker-compose.prod.yml exec postgres psql -U secretmanager -d secretmanager -c "SELECT version();"
```

---

## Monitoring

### Log Management

```bash
# View all logs
docker compose -f docker-compose.prod.yml logs -f

# View specific service logs
docker compose -f docker-compose.prod.yml logs -f backend
docker compose -f docker-compose.prod.yml logs -f frontend
docker compose -f docker-compose.prod.yml logs -f nginx

# View nginx access logs
docker compose -f docker-compose.prod.yml exec nginx tail -f /var/log/nginx/access.log

# View nginx error logs
docker compose -f docker-compose.prod.yml exec nginx tail -f /var/log/nginx/error.log
```

### Resource Usage

```bash
# View container resource usage
docker stats

# View disk usage
docker system df
docker volume ls
```

### Metrics (Optional)

For production monitoring, consider integrating:

- **Prometheus** + **Grafana** for metrics
- **Loki** for log aggregation
- **Alertmanager** for alerts
- **Uptime monitoring** (e.g., UptimeRobot, Pingdom)

Example Prometheus integration:

```yaml
# Add to docker-compose.prod.yml
  prometheus:
    image: prom/prometheus
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    ports:
      - "9090:9090"
    networks:
      - secretmanager
```

---

## Backup Strategy

### Database Backup

#### Automated Daily Backups

```bash
# Create backup script
cat > scripts/backup-db.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/var/backups/secretmanager"
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR

# Backup database
docker compose -f docker-compose.prod.yml exec -T postgres pg_dump -U secretmanager secretmanager | gzip > $BACKUP_DIR/db_backup_$DATE.sql.gz

# Keep only last 30 days
find $BACKUP_DIR -name "db_backup_*.sql.gz" -mtime +30 -delete

echo "Backup completed: db_backup_$DATE.sql.gz"
EOF

chmod +x scripts/backup-db.sh

# Add to crontab (daily at 2 AM)
(crontab -l 2>/dev/null; echo "0 2 * * * /path/to/secret-manager/scripts/backup-db.sh") | crontab -
```

#### Manual Backup

```bash
# Create backup
docker compose -f docker-compose.prod.yml exec postgres pg_dump -U secretmanager secretmanager > backup.sql

# Or with compression
docker compose -f docker-compose.prod.yml exec -T postgres pg_dump -U secretmanager secretmanager | gzip > backup_$(date +%Y%m%d).sql.gz
```

#### Restore from Backup

```bash
# Stop backend to prevent conflicts
docker compose -f docker-compose.prod.yml stop backend

# Restore database
gunzip -c backup.sql.gz | docker compose -f docker-compose.prod.yml exec -T postgres psql -U secretmanager secretmanager

# Restart services
docker compose -f docker-compose.prod.yml start backend
```

### Volume Backup

```bash
# Backup Docker volumes
docker run --rm \
  -v secretmanager_postgres_data:/source:ro \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/postgres_data_$(date +%Y%m%d).tar.gz -C /source .

docker run --rm \
  -v secretmanager_secrets_repo:/source:ro \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/secrets_repo_$(date +%Y%m%d).tar.gz -C /source .
```

### Git Repository Backup

The secrets repository is already version-controlled. Ensure you have:

1. **Remote backup**: Push to GitHub/GitLab regularly
2. **Multiple remotes**: Consider a secondary remote
3. **Local backups**: Periodic clones to backup storage

```bash
# Add backup remote
git -C /path/to/secrets-repo remote add backup https://backup-git.example.com/secrets.git
git -C /path/to/secrets-repo push backup main
```

---

## Scaling

### Vertical Scaling (More Resources)

Edit `docker-compose.prod.yml` to add resource limits:

```yaml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

### Horizontal Scaling (Multiple Instances)

```bash
# Scale backend to 3 instances
docker compose -f docker-compose.prod.yml up -d --scale backend=3

# Scale frontend to 2 instances
docker compose -f docker-compose.prod.yml up -d --scale frontend=2
```

**Note**: For true horizontal scaling with load balancing:
1. Use Docker Swarm or Kubernetes
2. Configure nginx upstream with multiple backend servers
3. Ensure session affinity or stateless sessions
4. Use external database (not containerized)

### Database Scaling

For production at scale, consider:

- **Managed PostgreSQL** (AWS RDS, Google Cloud SQL, etc.)
- **Read replicas** for read-heavy workloads
- **Connection pooling** (PgBouncer)
- **Database tuning** (shared_buffers, work_mem, etc.)

---

## Updates

### Update Process

```bash
# 1. Backup database FIRST
./scripts/backup-db.sh

# 2. Pull latest changes
git pull origin main

# 3. Rebuild images
docker compose -f docker-compose.prod.yml build

# 4. Stop services gracefully
docker compose -f docker-compose.prod.yml down

# 5. Start with new images
docker compose -f docker-compose.prod.yml up -d

# 6. Check logs
docker compose -f docker-compose.prod.yml logs -f

# 7. Verify health
./scripts/deploy-prod.sh health
```

### Using the Deployment Script

```bash
# Automated update with backup
./scripts/deploy-prod.sh update
```

### Zero-Downtime Updates (Advanced)

For zero-downtime updates, use:

1. **Docker Swarm** with rolling updates
2. **Blue-Green Deployment** with multiple compose files
3. **Kubernetes** with rolling deployments

Example blue-green deployment:

```bash
# Start new stack (green)
docker compose -f docker-compose.prod-green.yml up -d

# Switch nginx to point to green
# Update nginx upstream configuration

# Stop old stack (blue)
docker compose -f docker-compose.prod-blue.yml down
```

### Rollback

```bash
# Rollback to previous image version
docker compose -f docker-compose.prod.yml down
git checkout previous-commit
docker compose -f docker-compose.prod.yml up -d

# Or restore from backup
./scripts/restore-db.sh backup_20260325.sql.gz
```

---

## Troubleshooting

### Service Won't Start

```bash
# Check logs for errors
docker compose -f docker-compose.prod.yml logs backend
docker compose -f docker-compose.prod.yml logs frontend

# Check configuration
docker compose -f docker-compose.prod.yml config

# Verify environment variables
docker compose -f docker-compose.prod.yml exec backend env
```

### Database Connection Failed

```bash
# Check postgres is healthy
docker compose -f docker-compose.prod.yml ps postgres

# Test connection
docker compose -f docker-compose.prod.yml exec postgres psql -U secretmanager -d secretmanager

# Check DATABASE_URL format
docker compose -f docker-compose.prod.yml exec backend env | grep DATABASE_URL
```

### Nginx 502 Bad Gateway

```bash
# Check backend is running
docker compose -f docker-compose.prod.yml ps backend

# Check backend health
curl http://localhost:8080/health

# Check nginx logs
docker compose -f docker-compose.prod.yml logs nginx

# Test nginx config
docker compose -f docker-compose.prod.yml exec nginx nginx -t
```

### SSL Certificate Issues

```bash
# Check certificate validity
openssl x509 -in nginx/ssl/cert.pem -text -noout

# Check certificate expiration
openssl x509 -in nginx/ssl/cert.pem -noout -enddate

# Verify certificate matches private key
openssl x509 -noout -modulus -in nginx/ssl/cert.pem | openssl md5
openssl rsa -noout -modulus -in nginx/ssl/key.pem | openssl md5
# These should match
```

### Port Already in Use

```bash
# Find process using port 80
sudo lsof -i :80

# Or with netstat
sudo netstat -tulpn | grep :80

# Stop conflicting service
sudo systemctl stop apache2  # or whatever is using the port
```

### Container Out of Memory

```bash
# Check memory usage
docker stats

# Increase memory limit in docker-compose.prod.yml
services:
  backend:
    deploy:
      resources:
        limits:
          memory: 2G

# Restart services
docker compose -f docker-compose.prod.yml up -d
```

### Disk Space Full

```bash
# Check disk usage
df -h
docker system df

# Clean up unused resources
docker system prune -a --volumes

# Remove old images
docker image prune -a

# Check volume sizes
docker volume ls
docker volume inspect secretmanager_postgres_data
```

### Git Authentication Failed

```bash
# Check SSH key is mounted
docker compose -f docker-compose.prod.yml exec backend ls -la /app/.ssh/

# Test SSH connection to git host
docker compose -f docker-compose.prod.yml exec backend ssh -T git@github.com

# Verify key has correct permissions
ls -la prod-keys/ssh-keys/id_rsa
# Should be: -rw------- (600)
```

### SOPS Decryption Failed

```bash
# Check AGE key is mounted
docker compose -f docker-compose.prod.yml exec backend ls -la /app/.age/

# Verify AGE key format
docker compose -f docker-compose.prod.yml exec backend cat /app/.age/keys.txt
# Should start with: AGE-SECRET-KEY-

# Test SOPS decryption
docker compose -f docker-compose.prod.yml exec backend sops --version
```

### Performance Issues

```bash
# Check resource usage
docker stats

# Check backend logs for slow queries
docker compose -f docker-compose.prod.yml logs backend | grep "slow query"

# Enable query logging in postgres
docker compose -f docker-compose.prod.yml exec postgres psql -U secretmanager -d secretmanager -c "ALTER SYSTEM SET log_min_duration_statement = 1000;"

# Check nginx access logs for slow requests
docker compose -f docker-compose.prod.yml exec nginx tail -f /var/log/nginx/access.log
```

---

## Security Checklist

Before going to production:

- [ ] Strong passwords in `.env.prod`
- [ ] JWT_SECRET and NEXTAUTH_SECRET are randomly generated
- [ ] SSL certificates installed and HTTPS enabled
- [ ] Firewall configured (only ports 80, 443, 22 open)
- [ ] SSH key authentication for Git (not password)
- [ ] AGE keys securely stored and backed up
- [ ] Database backups automated
- [ ] Non-root users in containers (already configured)
- [ ] Regular security updates scheduled
- [ ] Monitoring and alerting configured
- [ ] Rate limiting enabled in nginx (already configured)
- [ ] CORS properly configured in backend
- [ ] OAuth properly configured with real provider
- [ ] `.env.prod` not committed to git

---

## Additional Resources

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Nginx Documentation](https://nginx.org/en/docs/)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [SOPS Documentation](https://github.com/getsops/sops)
- [FluxCD Documentation](https://fluxcd.io/docs/)

---

## Support

For issues and questions:

- **GitHub Issues**: https://github.com/yourorg/secret-manager/issues
- **Documentation**: https://github.com/yourorg/secret-manager/docs
- **Security Issues**: security@your-domain.com (private disclosure)

---

*Last Updated: March 26, 2026*
