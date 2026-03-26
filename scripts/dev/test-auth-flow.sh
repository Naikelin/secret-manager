#!/bin/bash
set -e

echo "🧪 Testing Auth Flow (Self-Contained Mock)"
echo "==========================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Health check
echo -e "${YELLOW}1. Testing backend health...${NC}"
HEALTH=$(curl -s http://localhost:8080/health)
if [[ $HEALTH == *"ok"* ]]; then
  echo -e "${GREEN}✓ Backend healthy${NC}"
else
  echo -e "${RED}✗ Backend not responding${NC}"
  exit 1
fi
echo ""

# Step 2: Login endpoint
echo -e "${YELLOW}2. Testing login endpoint (should return redirect_url)...${NC}"
LOGIN_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"dev@example.com"}')

REDIRECT_URL=$(echo $LOGIN_RESPONSE | grep -o '"redirect_url":"[^"]*"' | cut -d'"' -f4)

if [[ -z "$REDIRECT_URL" ]]; then
  echo -e "${RED}✗ No redirect_url in response${NC}"
  echo "Response: $LOGIN_RESPONSE"
  exit 1
fi

echo -e "${GREEN}✓ Got redirect URL: $REDIRECT_URL${NC}"
echo ""

# Step 3: Follow redirect (mock callback)
echo -e "${YELLOW}3. Following redirect to mock callback...${NC}"
CALLBACK_RESPONSE=$(curl -s -L "$REDIRECT_URL")

# Check if we got redirected to frontend with token
if [[ $CALLBACK_RESPONSE == *"token="* ]]; then
  echo -e "${GREEN}✓ Redirected to frontend with token${NC}"
  
  # Extract token from HTML redirect (if any)
  TOKEN=$(echo "$CALLBACK_RESPONSE" | grep -o 'token=[^"&]*' | head -1 | cut -d'=' -f2)
  
  if [[ -n "$TOKEN" ]]; then
    echo -e "${GREEN}✓ Token extracted: ${TOKEN:0:50}...${NC}"
  fi
else
  echo -e "${RED}✗ No token in callback response${NC}"
  echo "Response: $CALLBACK_RESPONSE"
  exit 1
fi
echo ""

# Step 4: Test JWT validation (future phase)
# echo -e "${YELLOW}4. Testing protected endpoint with JWT...${NC}"
# TODO: Add when we have protected endpoints

echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}🎉 Auth flow test PASSED${NC}"
echo -e "${GREEN}=========================================${NC}"
