#!/bin/bash

# E2E Testing with Docker
# Runs all Playwright tests in isolated Docker containers
# All services (postgres, seed, backend, frontend, playwright) run in containers

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Secret Manager E2E Tests (Docker)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Step 1: Clean up any existing E2E containers and volumes
echo -e "${YELLOW}→ Cleaning up existing E2E containers and volumes...${NC}"
cd "$PROJECT_ROOT"
docker-compose -f docker-compose.e2e.yml down -v 2>/dev/null || true
echo -e "${GREEN}  ✓ Cleaned up${NC}"
echo ""

# Step 2: Build and start services (including seed)
echo -e "${YELLOW}→ Building and starting services...${NC}"
docker-compose -f docker-compose.e2e.yml up -d postgres
echo ""

# Step 3: Wait for all services to be healthy/complete
echo -e "${YELLOW}→ Waiting for services to be ready...${NC}"

# Wait for postgres
echo -e "  - Waiting for postgres..."
timeout=30
elapsed=0
until docker-compose -f docker-compose.e2e.yml exec -T postgres pg_isready -U dev -d secretmanager > /dev/null 2>&1; do
  sleep 1
  elapsed=$((elapsed + 1))
  if [ $elapsed -ge $timeout ]; then
    echo -e "${RED}✗ Postgres failed to start within ${timeout}s${NC}"
    docker-compose -f docker-compose.e2e.yml logs postgres
    exit 1
  fi
done
echo -e "${GREEN}  ✓ Postgres is ready${NC}"

# Run seed data population
echo -e "  - Running seed data population..."
docker-compose -f docker-compose.e2e.yml run --rm seed
if [ $? -ne 0 ]; then
  echo -e "${RED}✗ Seed data population failed${NC}"
  docker-compose -f docker-compose.e2e.yml logs seed
  exit 1
fi
echo -e "${GREEN}  ✓ Seed data populated${NC}"

# Start backend and frontend
docker-compose -f docker-compose.e2e.yml up -d backend frontend

# Wait for backend
echo -e "  - Waiting for backend..."
timeout=30
elapsed=0
until docker-compose -f docker-compose.e2e.yml exec -T backend wget --no-verbose --tries=1 --spider http://localhost:8080/health > /dev/null 2>&1; do
  sleep 1
  elapsed=$((elapsed + 1))
  if [ $elapsed -ge $timeout ]; then
    echo -e "${RED}✗ Backend failed to start within ${timeout}s${NC}"
    docker-compose -f docker-compose.e2e.yml logs backend
    exit 1
  fi
done
echo -e "${GREEN}  ✓ Backend is ready${NC}"

# Wait for frontend
echo -e "  - Waiting for frontend..."
timeout=60
elapsed=0
until docker-compose -f docker-compose.e2e.yml exec -T frontend wget --no-verbose --tries=1 --spider http://localhost:3000 > /dev/null 2>&1; do
  sleep 1
  elapsed=$((elapsed + 1))
  if [ $elapsed -ge $timeout ]; then
    echo -e "${RED}✗ Frontend failed to start within ${timeout}s${NC}"
    docker-compose -f docker-compose.e2e.yml logs frontend
    exit 1
  fi
  # Show progress every 10 seconds
  if [ $((elapsed % 10)) -eq 0 ]; then
    echo -e "    (still waiting... ${elapsed}s elapsed)"
  fi
done
echo -e "${GREEN}  ✓ Frontend is ready${NC}"
echo ""

# Step 4: Run Playwright tests
echo -e "${YELLOW}→ Running Playwright tests in container...${NC}"
echo ""

# Remove old test results
rm -rf "$PROJECT_ROOT/frontend/playwright-report" "$PROJECT_ROOT/frontend/test-results" 2>/dev/null || true

# Run tests with proper exit code handling
set +e
docker-compose -f docker-compose.e2e.yml run --rm \
  -e PLAYWRIGHT_BASE_URL=http://frontend:3000 \
  -e CI=true \
  playwright sh -c "npm install && npx playwright test --reporter=list"
TEST_EXIT_CODE=$?
set -e

echo ""

# Step 5: Show results and decide cleanup
if [ $TEST_EXIT_CODE -eq 0 ]; then
  echo -e "${GREEN}========================================${NC}"
  echo -e "${GREEN}  ✓ All E2E tests passed!${NC}"
  echo -e "${GREEN}========================================${NC}"
  echo ""
  echo -e "${YELLOW}→ Cleaning up containers...${NC}"
  docker-compose -f docker-compose.e2e.yml down -v
  echo -e "${GREEN}✓ Done!${NC}"
else
  echo -e "${RED}========================================${NC}"
  echo -e "${RED}  ✗ Some E2E tests failed${NC}"
  echo -e "${RED}========================================${NC}"
  echo ""
  echo -e "${YELLOW}Containers are still running for debugging.${NC}"
  echo -e "${YELLOW}Available commands:${NC}"
  echo -e "  - View logs:          docker-compose -f docker-compose.e2e.yml logs [service]"
  echo -e "  - Access backend:     http://localhost:8081"
  echo -e "  - Access frontend:    http://localhost:3001"
  echo -e "  - Stop containers:    docker-compose -f docker-compose.e2e.yml down -v"
  echo ""
  echo -e "${YELLOW}View detailed test report:${NC}"
  echo -e "  cd frontend && npm run test:e2e:report"
  echo ""
fi

exit $TEST_EXIT_CODE
