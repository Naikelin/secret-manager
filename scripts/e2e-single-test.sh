#!/bin/bash
set -e

# Colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TEST_FILE="${1:-e2e/drift/comparison.spec.ts}"
TEST_NAME="${2:-}"

echo -e "${BLUE}========================================"
echo -e "  Running Single E2E Test"
echo -e "========================================${NC}"

# Start services if not running
if ! docker ps | grep -q secretmanager-e2e-backend; then
  echo -e "${YELLOW}→ Starting E2E services...${NC}"
  docker compose -f docker-compose.e2e.yml up -d --wait
  echo -e "${GREEN}  ✓ Services ready${NC}"
fi

# Run specific test
echo -e "${YELLOW}→ Running test: ${TEST_FILE}${NC}"

TEST_COMMAND="npx playwright test ${TEST_FILE}"
if [ -n "$TEST_NAME" ]; then
  TEST_COMMAND="$TEST_COMMAND -g \"$TEST_NAME\""
fi

docker compose -f docker-compose.e2e.yml run --rm \
  -e PLAYWRIGHT_BASE_URL=http://frontend:3000 \
  -e NEXT_PUBLIC_API_URL=http://backend:8080 \
  playwright \
  bash -c "npm install --silent && $TEST_COMMAND --reporter=list"

echo ""
echo -e "${GREEN}✓ Test complete${NC}"
echo -e "${YELLOW}→ Screenshots/traces in frontend/test-results/${NC}"
