#!/bin/bash

# Production Deployment Script for Secret Manager
# This script helps automate production deployment tasks

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="docker-compose.prod.yml"
ENV_FILE=".env.prod"
BACKUP_DIR="/var/backups/secretmanager"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if .env.prod exists
check_env_file() {
    if [ ! -f "$ENV_FILE" ]; then
        log_error "Environment file $ENV_FILE not found!"
        log_info "Copy .env.prod.example to .env.prod and configure it"
        exit 1
    fi
}

# Validate required environment variables
validate_env() {
    log_info "Validating environment configuration..."
    
    # Source the env file
    set -a
    source "$ENV_FILE"
    set +a
    
    # Check required variables
    REQUIRED_VARS=(
        "DB_PASSWORD"
        "JWT_SECRET"
        "OAUTH_CLIENT_ID"
        "OAUTH_CLIENT_SECRET"
        "NEXTAUTH_SECRET"
        "NEXTAUTH_URL"
    )
    
    MISSING_VARS=()
    for var in "${REQUIRED_VARS[@]}"; do
        if [ -z "${!var}" ]; then
            MISSING_VARS+=("$var")
        fi
    done
    
    if [ ${#MISSING_VARS[@]} -ne 0 ]; then
        log_error "Missing required environment variables:"
        for var in "${MISSING_VARS[@]}"; do
            echo "  - $var"
        done
        exit 1
    fi
    
    # Check for default/example values
    if [[ "$DB_PASSWORD" == *"CHANGE_ME"* ]]; then
        log_error "DB_PASSWORD still contains CHANGE_ME placeholder"
        exit 1
    fi
    
    if [[ "$JWT_SECRET" == *"CHANGE_ME"* ]]; then
        log_error "JWT_SECRET still contains CHANGE_ME placeholder"
        exit 1
    fi
    
    log_success "Environment validation passed"
}

# Check required directories and files
check_requirements() {
    log_info "Checking requirements..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    # Check Docker Compose
    if ! docker compose version &> /dev/null; then
        log_error "Docker Compose v2 is not installed"
        exit 1
    fi
    
    # Check prod-keys directory
    if [ ! -d "prod-keys" ]; then
        log_warning "prod-keys directory not found"
        log_info "Creating prod-keys directory structure..."
        mkdir -p prod-keys/{age-keys,ssh-keys,kubeconfig}
        chmod 700 prod-keys
        log_info "Please populate prod-keys with your secrets before deploying"
    fi
    
    # Check SSL certificates
    if [ ! -f "nginx/ssl/cert.pem" ] || [ ! -f "nginx/ssl/key.pem" ]; then
        log_warning "SSL certificates not found in nginx/ssl/"
        log_info "HTTPS will not be available. See docs/DEPLOYMENT.md for SSL setup"
    fi
    
    log_success "Requirements check passed"
}

# Backup database
backup_database() {
    log_info "Backing up database..."
    
    mkdir -p "$BACKUP_DIR"
    DATE=$(date +%Y%m%d_%H%M%S)
    BACKUP_FILE="$BACKUP_DIR/db_backup_$DATE.sql.gz"
    
    # Check if postgres container is running
    if docker compose -f "$COMPOSE_FILE" ps postgres | grep -q "Up"; then
        docker compose -f "$COMPOSE_FILE" exec -T postgres \
            pg_dump -U secretmanager secretmanager | gzip > "$BACKUP_FILE"
        
        log_success "Database backed up to $BACKUP_FILE"
        
        # Keep only last 30 backups
        ls -t "$BACKUP_DIR"/db_backup_*.sql.gz | tail -n +31 | xargs -r rm
        log_info "Old backups cleaned (keeping last 30)"
    else
        log_warning "Postgres container is not running, skipping backup"
    fi
}

# Build images
build_images() {
    log_info "Building production images..."
    
    docker compose -f "$COMPOSE_FILE" build
    
    log_success "Images built successfully"
}

# Start services
start_services() {
    log_info "Starting services..."
    
    docker compose -f "$COMPOSE_FILE" up -d
    
    log_success "Services started"
}

# Stop services
stop_services() {
    log_info "Stopping services..."
    
    docker compose -f "$COMPOSE_FILE" down
    
    log_success "Services stopped"
}

# Show service status
show_status() {
    log_info "Service status:"
    docker compose -f "$COMPOSE_FILE" ps
}

# Show logs
show_logs() {
    SERVICE="${1:-}"
    
    if [ -z "$SERVICE" ]; then
        docker compose -f "$COMPOSE_FILE" logs -f
    else
        docker compose -f "$COMPOSE_FILE" logs -f "$SERVICE"
    fi
}

# Health check
health_check() {
    log_info "Running health checks..."
    
    # Check if services are running
    if ! docker compose -f "$COMPOSE_FILE" ps | grep -q "Up"; then
        log_error "Services are not running"
        return 1
    fi
    
    # Check postgres health
    log_info "Checking postgres..."
    if docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U secretmanager &> /dev/null; then
        log_success "Postgres: healthy"
    else
        log_error "Postgres: unhealthy"
        return 1
    fi
    
    # Check backend health
    log_info "Checking backend..."
    sleep 5  # Give backend time to start
    if docker compose -f "$COMPOSE_FILE" exec -T backend curl -f http://localhost:8080/health &> /dev/null; then
        log_success "Backend: healthy"
    else
        log_error "Backend: unhealthy"
        return 1
    fi
    
    # Check nginx
    log_info "Checking nginx..."
    if curl -f http://localhost/health &> /dev/null; then
        log_success "Nginx: healthy"
    else
        log_error "Nginx: unhealthy"
        return 1
    fi
    
    log_success "All health checks passed"
}

# Full deployment
full_deploy() {
    log_info "Starting full deployment..."
    
    check_env_file
    validate_env
    check_requirements
    backup_database
    build_images
    start_services
    
    log_info "Waiting for services to start..."
    sleep 10
    
    health_check
    
    log_success "Deployment completed successfully!"
    log_info "Access your application at: ${NEXTAUTH_URL:-http://localhost}"
}

# Update deployment
update_deploy() {
    log_info "Starting update deployment..."
    
    backup_database
    
    log_info "Pulling latest changes..."
    git pull
    
    build_images
    
    log_info "Stopping services..."
    docker compose -f "$COMPOSE_FILE" down
    
    start_services
    
    log_info "Waiting for services to start..."
    sleep 10
    
    health_check
    
    log_success "Update completed successfully!"
}

# Restore database from backup
restore_database() {
    BACKUP_FILE="$1"
    
    if [ -z "$BACKUP_FILE" ]; then
        log_error "Usage: $0 restore <backup_file.sql.gz>"
        exit 1
    fi
    
    if [ ! -f "$BACKUP_FILE" ]; then
        log_error "Backup file not found: $BACKUP_FILE"
        exit 1
    fi
    
    log_warning "This will restore the database from $BACKUP_FILE"
    read -p "Are you sure? (yes/no): " -r
    if [[ ! $REPLY =~ ^yes$ ]]; then
        log_info "Restore cancelled"
        exit 0
    fi
    
    log_info "Stopping backend..."
    docker compose -f "$COMPOSE_FILE" stop backend
    
    log_info "Restoring database..."
    gunzip -c "$BACKUP_FILE" | docker compose -f "$COMPOSE_FILE" exec -T postgres \
        psql -U secretmanager secretmanager
    
    log_info "Restarting services..."
    docker compose -f "$COMPOSE_FILE" start backend
    
    log_success "Database restored successfully"
}

# Clean up
cleanup() {
    log_warning "This will remove all stopped containers, unused networks, and dangling images"
    read -p "Continue? (yes/no): " -r
    if [[ ! $REPLY =~ ^yes$ ]]; then
        log_info "Cleanup cancelled"
        exit 0
    fi
    
    log_info "Cleaning up Docker resources..."
    docker system prune -f
    
    log_success "Cleanup completed"
}

# Show usage
usage() {
    cat << EOF
Production Deployment Script for Secret Manager

Usage: $0 <command> [options]

Commands:
    validate        Validate environment configuration
    build           Build production Docker images
    start           Start all services
    stop            Stop all services
    restart         Restart all services
    status          Show service status
    logs [service]  Show logs (optionally for specific service)
    health          Run health checks
    deploy          Full deployment (build, backup, start)
    update          Update deployment (backup, pull, rebuild, restart)
    backup          Backup database
    restore <file>  Restore database from backup
    cleanup         Clean up Docker resources
    help            Show this help message

Examples:
    $0 validate              # Validate .env.prod configuration
    $0 deploy                # Full production deployment
    $0 update                # Update to latest version
    $0 logs backend          # Show backend logs
    $0 restore backup.sql.gz # Restore database from backup

EOF
}

# Main script
COMMAND="${1:-help}"

case "$COMMAND" in
    validate)
        check_env_file
        validate_env
        check_requirements
        ;;
    build)
        check_env_file
        build_images
        ;;
    start)
        check_env_file
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        stop_services
        start_services
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs "${2:-}"
        ;;
    health)
        health_check
        ;;
    deploy)
        full_deploy
        ;;
    update)
        update_deploy
        ;;
    backup)
        check_env_file
        backup_database
        ;;
    restore)
        check_env_file
        restore_database "$2"
        ;;
    cleanup)
        cleanup
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        log_error "Unknown command: $COMMAND"
        usage
        exit 1
        ;;
esac
