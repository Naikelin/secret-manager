#!/bin/bash

# Quick E2E test run with limited tests for debugging
# Runs a single test file to debug issues faster

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
echo -e "${BLUE}  Secret Manager E2E Tests (Debug)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Step 1: Clean up
echo -e "${YELLOW}→ Cleaning up...${NC}"
cd "$PROJECT_ROOT"
docker-compose -f docker-compose.e2e.yml down -v 2>/dev/null || true
echo ""

# Step 2: Start services
echo -e "${YELLOW}→ Starting services...${NC}"
docker-compose -f docker-compose.e2e.yml up -d postgres backend frontend
echo ""

# Step 3: Wait for health
echo -e "${YELLOW}→ Waiting for services...${NC}"

# Postgres
timeout=30
elapsed=0
until docker-compose -f docker-compose.e2e.yml exec -T postgres pg_isready -U dev -d secretmanager > /dev/null 2>&1; do
  sleep 1
  elapsed=$((elapsed + 1))
  if [ $elapsed -ge $timeout ]; then
    echo -e "${RED}✗ Postgres failed${NC}"
    docker-compose -f docker-compose.e2e.yml down -v
    exit 1
  fi
done
echo -e "${GREEN}  ✓ Postgres ready${NC}"

# Backend
timeout=30
elapsed=0
until docker-compose -f docker-compose.e2e.yml exec -T backend wget --no-verbose --tries=1 --spider http://localhost:8080/health > /dev/null 2>&1; do
  sleep 1
  elapsed=$((elapsed + 1))
  if [ $elapsed -ge $timeout ]; then
    echo -e "${RED}✗ Backend failed${NC}"
    docker-compose -f docker-compose.e2e.yml down -v
    exit 1
  fi
done
echo -e "${GREEN}  ✓ Backend ready${NC}"

# Frontend
timeout=60
elapsed=0
until docker-compose -f docker-compose.e2e.yml exec -T frontend wget --no-verbose --tries=1 --spider http://localhost:3000 > /dev/null 2>&1; do
  sleep 1
  elapsed=$((elapsed + 1))
  if [ $elapsed -ge $timeout ]; then
    echo -e "${RED}✗ Frontend failed${NC}"
    docker-compose -f docker-compose.e2e.yml down -v
    exit 1
  fi
  if [ $((elapsed % 10)) -eq 0 ]; then
    echo -e "    (waiting... ${elapsed}s)"
  fi
done
echo -e "${GREEN}  ✓ Frontend ready${NC}"
echo ""

# Step 4: Run single test
echo -e "${YELLOW}→ Running Playwright tests (limited)...${NC}"
echo ""

set +e
docker-compose -f docker-compose.e2e.yml run --rm \
  -e PLAYWRIGHT_BASE_URL=http://frontend:3000 \
  -e CI=true \
  playwright sh -c "npm install && npx playwright test e2e/auth/login.spec.ts --reporter=list"
TEST_EXIT_CODE=$?
set -e

echo ""

# Step 5: Cleanup
echo -e "${YELLOW}→ Cleaning up...${NC}"
docker-compose -f docker-compose.e2e.yml down -v
echo ""

if [ $TEST_EXIT_CODE -eq 0 ]; then
  echo -e "${GREEN}✓ Test passed!${NC}"
else
  echo -e "${RED}✗ Test failed${NC}"
fi

exit $TEST_EXIT_CODE
