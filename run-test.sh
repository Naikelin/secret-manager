#!/bin/bash
set -e

echo "========================================="
echo "Secret Manager - Ready to Test"
echo "========================================="
echo ""

# Check PostgreSQL
echo "✓ Checking PostgreSQL..."
if ! docker ps | grep -q secretmanager-postgres; then
    echo "  Starting PostgreSQL..."
    docker-compose up -d postgres
    echo "  Waiting 10 seconds for PostgreSQL to be ready..."
    sleep 10
else
    echo "  PostgreSQL is already running"
fi
echo ""

# Check dev-data
echo "✓ Checking dev-data setup..."
if [ ! -f "dev-data/age-keys/keys.txt" ]; then
    echo "  ERROR: Age keys not found. Run: make setup-sops"
    exit 1
fi
if [ ! -f "dev-data/secrets-repo/.sops.yaml" ]; then
    echo "  ERROR: SOPS config not found. Run: make setup-sops"
    exit 1
fi
if [ ! -d "dev-data/secrets-repo/.git" ]; then
    echo "  ERROR: Git repo not initialized. Run: make setup-sops"
    exit 1
fi
echo "  Dev data is ready"
echo ""

# Check backend binary
echo "✓ Checking backend binary..."
if [ ! -f "backend/server" ]; then
    echo "  Building backend..."
    cd backend
    go build -o server cmd/server/main.go
    cd ..
else
    echo "  Backend binary exists ($(ls -lh backend/server | awk '{print $5}'))"
fi
echo ""

# Check backend .env
echo "✓ Checking backend configuration..."
if [ ! -f "backend/.env" ]; then
    echo "  WARNING: backend/.env not found!"
    echo "  Copy from backend/.env.example and configure"
    exit 1
fi
echo "  Configuration file exists"
echo ""

echo "========================================="
echo "✅ READY TO START"
echo "========================================="
echo ""
echo "Start the server with:"
echo "  cd backend && ./server"
echo ""
echo "Then in another terminal, test the API:"
echo "  ./test-api.sh"
echo ""
echo "Or test manually:"
echo ""
echo "  # 1. Login and get token"
echo "  curl -X POST http://localhost:8080/api/v1/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"email\":\"dev@example.com\"}'"
echo ""
echo "  # 2. Follow the redirect URL from step 1"
echo "  # (The test-api.sh script does this automatically)"
echo ""
echo "  # 3. Use token to call APIs"
echo "  export TOKEN='your-token-here'"
echo "  curl http://localhost:8080/health \\"
echo "    -H \"Authorization: Bearer \$TOKEN\""
echo ""
echo "========================================="
