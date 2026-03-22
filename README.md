# Secret Manager

GitOps-based Kubernetes Secret Management with SOPS encryption and FluxCD auto-sync.

## Features

- **Staging Area Workflow**: Draft secrets in PostgreSQL before committing to Git
- **SOPS Encryption**: Age encryption (dev) with KMS migration path (prod)
- **FluxCD Integration**: Auto-sync secrets from Git to Kubernetes
- **Drift Detection**: Monitor and reconcile Git vs K8s mismatches
- **RBAC**: Namespace-scoped permissions (viewer/editor/admin roles)
- **Audit Trail**: Complete audit log of all secret operations

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.26+
- Node.js 25+
- kubectl
- kind (optional, for local Kubernetes cluster)
- age (for SOPS encryption)

### Development Setup

1. **Clone the repository**
   ```bash
   git clone <repo-url>
   cd secret-manager
   ```

2. **Generate Age encryption key**
   ```bash
   make age-key
   ```

3. **Initialize Git repository for secrets**
   ```bash
   cd dev-data/secrets-repo
   git init
   git config user.email "dev@example.com"
   git config user.name "Dev User"
   cd ../..
   ```

4. **Start services with Docker Compose**
   ```bash
   make dev
   # or
   docker compose up --build
   ```

5. **Seed development data**
   ```bash
   make seed
   ```

6. **Access the application**
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8080
   - Mock OAuth: http://localhost:9000

### Optional: Local Kubernetes Cluster

```bash
# Create Kind cluster with FluxCD
make kind-up

# Verify FluxCD is running
kubectl get pods -n flux-system

# Delete cluster when done
make kind-down
```

## Architecture

```
┌─────────────┐      ┌──────────────┐      ┌──────────┐
│   Next.js   │─────▶│   Go API     │─────▶│PostgreSQL│
│  Frontend   │      │  (Chi/GORM)  │      │  (Drafts)│
└─────────────┘      └──────────────┘      └──────────┘
                            │
                            ├─────▶ Git Repo (SOPS encrypted)
                            │              │
                            │              ▼
                            │         ┌─────────┐
                            │         │ FluxCD  │
                            │         └─────────┘
                            │              │
                            │              ▼
                            └─────▶ Kubernetes Secrets
```

## Development Workflow

### 1. Create a Draft Secret

```typescript
// Frontend: Create draft
POST /api/v1/secrets/db-credentials/draft
{
  "data": {
    "username": "postgres",
    "password": "secret123"
  },
  "namespace_id": "<uuid>"
}
```

### 2. Publish to Git

```typescript
// Frontend: Publish draft
POST /api/v1/secrets/db-credentials/publish
{
  "commit_message": "Add database credentials"
}

// Backend will:
// 1. Encrypt with SOPS
// 2. Commit to Git
// 3. Push to remote
// 4. Update draft status to "published"
```

### 3. FluxCD Auto-Sync

FluxCD polls the Git repository every 30 seconds and:
1. Detects new commits
2. Decrypts SOPS files using Age key
3. Applies secrets to Kubernetes cluster

### 4. Drift Detection

Background cron (every 5 minutes) compares Git vs K8s:
- Fetches secret from Git (decrypted)
- Fetches secret from K8s
- Normalizes K8s metadata
- Creates drift event if mismatch detected

## Environment Variables

### Backend (`backend/.env`)

```bash
# Database
DATABASE_URL=postgres://dev:devpass@localhost:5432/secretmanager?sslmode=disable

# Server
PORT=8080
LOG_LEVEL=debug

# Authentication
AUTH_PROVIDER=mock  # mock | azure
JWT_SECRET=dev-secret-change-in-production

# Git
GIT_REPO_PATH=/data/secrets-repo

# Kubernetes
K8S_KUBECONFIG=/root/.kube/config

# SOPS
SOPS_AGE_KEY_FILE=/keys/age.txt
```

### Frontend (`frontend/.env`)

```bash
# API URL
NEXT_PUBLIC_API_URL=http://localhost:8080

# NextAuth (future)
NEXTAUTH_URL=http://localhost:3000
NEXTAUTH_SECRET=dev-secret-change-in-production
```

## Project Structure

```
secret-manager/
├── backend/              # Go API server
│   ├── cmd/server/       # Main entry point
│   ├── internal/
│   │   ├── api/          # HTTP handlers
│   │   ├── auth/         # OAuth2 providers
│   │   ├── config/       # Configuration
│   │   ├── middleware/   # HTTP middleware
│   │   ├── models/       # GORM models
│   │   └── repository/   # Data access
│   ├── pkg/logger/       # Structured logging
│   └── migrations/       # SQL migrations
├── frontend/             # Next.js frontend
│   ├── app/              # App Router pages
│   ├── components/       # React components
│   ├── lib/              # Utilities
│   └── types/            # TypeScript types
├── scripts/              # Helper scripts
│   ├── setup-kind.sh     # Kind cluster setup
│   ├── generate-age-key.sh
│   └── seed-dev.sh       # Seed dev data
├── dev-data/             # Local development data
│   ├── secrets-repo/     # Git repository
│   └── age-key.txt       # Age encryption key
└── docker-compose.yml    # Development environment
```

## Authentication

### Development Mode (Mock OAuth)

The mock OAuth provider returns hardcoded users:

- **dev@example.com**: Developer with editor role on development namespace
- **admin@example.com**: Admin with admin role on all namespaces

Login flow:
1. Click "Login" → Redirects to mock OAuth (localhost:9000)
2. Mock auto-approves → Redirects to callback
3. Backend exchanges code for user info
4. Backend generates JWT token
5. Frontend stores token in localStorage

### Production Mode (Azure AD)

```bash
# Set environment variables
AUTH_PROVIDER=azure
AZURE_TENANT_ID=<your-tenant-id>
AZURE_CLIENT_ID=<your-client-id>
AZURE_CLIENT_SECRET=<your-client-secret>
```

Azure AD integration stub is in `backend/internal/auth/azure.go` (to be implemented).

## Testing

```bash
# Run backend tests
cd backend
go test -v ./...

# Run frontend tests
cd frontend
npm test

# End-to-end smoke test
make dev
# Verify:
# - ✅ PostgreSQL starts and migrations run
# - ✅ Backend responds at /health
# - ✅ Frontend loads at localhost:3000
# - ✅ Mock login works
```

## Troubleshooting

### PostgreSQL connection fails

```bash
# Check if PostgreSQL is running
docker compose ps postgres

# View logs
docker compose logs postgres

# Connect manually
docker compose exec postgres psql -U dev -d secretmanager
```

### Backend won't start

```bash
# Check logs
docker compose logs backend

# Rebuild
docker compose build backend
docker compose up backend
```

### Frontend hot reload not working

```bash
# Rebuild frontend container
docker compose build frontend
docker compose restart frontend
```

### Age key not found

```bash
# Generate Age key
make age-key

# Verify key exists
ls -la dev-data/age-key.txt
```

## Roadmap

### Phase 1: Foundation ✅
- [x] Go backend with Chi router
- [x] PostgreSQL database with GORM
- [x] Next.js frontend
- [x] Docker Compose setup
- [x] Mock OAuth authentication
- [x] JWT middleware

### Phase 2: Core Features (In Progress)
- [ ] Secret draft CRUD endpoints
- [ ] SOPS encryption/decryption
- [ ] Git operations (commit, push)
- [ ] RBAC enforcement
- [ ] FluxCD integration

### Phase 3: Advanced Features
- [ ] Drift detection
- [ ] Drift resolution
- [ ] Audit trail
- [ ] Frontend secret management UI

### Phase 4: Production Ready
- [ ] Azure AD authentication
- [ ] KMS encryption (Azure Key Vault)
- [ ] Multi-cluster support
- [ ] Performance optimization

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.
