# Secret Manager Backend

Go backend for the GitOps-based Secret Management platform.

## Project Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go          # Entry point
├── internal/
│   ├── api/
│   │   ├── router.go        # Chi router setup
│   │   └── auth.go          # Auth handlers
│   ├── auth/
│   │   ├── provider.go      # Auth provider interface
│   │   ├── mock.go          # Mock OAuth2 provider
│   │   └── azure.go         # Azure AD provider (stub)
│   ├── config/
│   │   └── config.go        # Configuration loading
│   ├── middleware/
│   │   ├── cors.go          # CORS middleware
│   │   ├── logger.go        # Request logging middleware
│   │   └── auth.go          # JWT authentication middleware
│   ├── models/
│   │   ├── user.go          # User model
│   │   └── secret.go        # Secret draft model
│   └── repository/
│       └── (future data access layer)
├── pkg/
│   └── logger/
│       └── logger.go        # Structured logging setup
├── migrations/
│   └── (future SQL migrations)
├── bin/
│   └── server               # Compiled binary
├── go.mod
├── go.sum
├── .env.example
└── README.md
```

## Dependencies

- **github.com/go-chi/chi/v5** - HTTP router
- **github.com/go-chi/cors** - CORS middleware
- **gorm.io/gorm** - ORM
- **gorm.io/driver/postgres** - PostgreSQL driver
- **github.com/joho/godotenv** - .env file loading
- **github.com/golang-jwt/jwt/v5** - JWT handling
- **github.com/google/uuid** - UUID generation

## Configuration

Create a `.env` file based on `.env.example`:

```bash
DATABASE_URL=postgres://dev:devpass@localhost:5432/secretmanager?sslmode=disable
PORT=8080
LOG_LEVEL=debug
JWT_SECRET=dev-secret-change-in-production
AUTH_PROVIDER=mock
GIT_REPO_PATH=/data/secrets-repo
SOPS_AGE_KEY_FILE=/keys/age.txt
```

## Build & Run

```bash
# Install dependencies
go mod tidy

# Build
go build -o bin/server ./cmd/server

# Run
./bin/server

# Or run directly
go run cmd/server/main.go
```

## Development

```bash
# Start PostgreSQL via docker-compose (from project root)
cd ..
docker compose up -d postgres

# Run with hot reload (if Air is installed)
air

# Run tests
go test ./...

# Format code
go fmt ./...

# Lint
golangci-lint run
```

## API Endpoints

### Health Check

```bash
GET /health
```

Response:
```json
{
  "status": "ok"
}
```

### Authentication (Phase 3)

```bash
POST   /api/v1/auth/login      # OAuth2 login redirect
GET    /api/v1/auth/callback   # OAuth2 callback handler
POST   /api/v1/auth/logout     # Logout (clear session)
```

## Logging

The application uses structured logging via `log/slog`:

- **Format**: JSON
- **Levels**: debug, info, warn, error
- **Request Logging**: Method, path, status, duration, remote addr, user agent

Example log entry:
```json
{
  "time": "2026-03-22T12:42:44.297494733-03:00",
  "level": "INFO",
  "msg": "http_request",
  "method": "GET",
  "path": "/health",
  "status": 200,
  "duration_ms": 0,
  "remote_addr": "[::1]:37970",
  "user_agent": "curl/8.19.0"
}
```

## CORS Configuration

Configured to allow requests from frontend:

- **Allowed Origins**: http://localhost:3000
- **Allowed Methods**: GET, POST, PUT, DELETE, OPTIONS
- **Allowed Headers**: Accept, Authorization, Content-Type
- **Allow Credentials**: true
- **Max Age**: 300s

## TODO

- [ ] Implement database migrations (Phase 2)
- [ ] Implement RBAC middleware (Phase 4)
- [ ] Add Git operations client (Phase 5)
- [ ] Add SOPS encryption/decryption (Phase 6)
- [ ] Implement draft workflow endpoints (Phase 7)
- [ ] Implement publish workflow (Phase 8)
- [ ] Add drift detection service (Phase 11)
- [ ] Add comprehensive test coverage (Phase 18)
