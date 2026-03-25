#!/bin/bash
set -e

BASE_URL="http://localhost:8080"
EMAIL="dev@example.com"

echo "🚀 Secret Manager API Testing Script"
echo "====================================="
echo ""

# Step 1: Login
echo "📝 Step 1: Login as $EMAIL"
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\"}")

echo "Response: $LOGIN_RESPONSE"
echo ""

# Extract redirect URL and get token
REDIRECT_URL=$(echo $LOGIN_RESPONSE | grep -o '"redirect_url":"[^"]*"' | cut -d'"' -f4)
echo "Redirect URL: $REDIRECT_URL"

# Get token from mock callback
TOKEN_RESPONSE=$(curl -s "$BASE_URL$REDIRECT_URL")
TOKEN=$(echo $TOKEN_RESPONSE | grep -o 'token=[^&"]*' | cut -d'=' -f2)

if [ -z "$TOKEN" ]; then
    echo "❌ Failed to get token"
    exit 1
fi

echo "✅ Token received: ${TOKEN:0:50}..."
echo ""

# Step 2: Get namespaces (we'll use the seeded ones)
echo "📝 Step 2: Check health endpoint"
HEALTH=$(curl -s "$BASE_URL/health")
echo "Health: $HEALTH"
echo ""

# For testing, we need to get namespace IDs from the database
# In a real scenario, you'd have an endpoint to list namespaces
# For now, let's use hardcoded UUIDs from seed data

echo "📋 Available endpoints to test:"
echo ""
echo "1. Create Draft Secret:"
echo "   curl -X POST \"$BASE_URL/api/v1/namespaces/{NAMESPACE_ID}/secrets\" \\"
echo "     -H \"Authorization: Bearer $TOKEN\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -d '{\"name\":\"my-secret\",\"data\":{\"username\":\"admin\",\"password\":\"secret123\"}}'"
echo ""
echo "2. List Secrets:"
echo "   curl \"$BASE_URL/api/v1/namespaces/{NAMESPACE_ID}/secrets\" \\"
echo "     -H \"Authorization: Bearer $TOKEN\""
echo ""
echo "3. Publish Secret:"
echo "   curl -X POST \"$BASE_URL/api/v1/namespaces/{NAMESPACE_ID}/secrets/my-secret/publish\" \\"
echo "     -H \"Authorization: Bearer $TOKEN\""
echo ""
echo "4. Get Sync Status:"
echo "   curl \"$BASE_URL/api/v1/namespaces/{NAMESPACE_ID}/sync-status\" \\"
echo "     -H \"Authorization: Bearer $TOKEN\""
echo ""
echo "5. Trigger Drift Check:"
echo "   curl -X POST \"$BASE_URL/api/v1/namespaces/{NAMESPACE_ID}/drift-check\" \\"
echo "     -H \"Authorization: Bearer $TOKEN\""
echo ""
echo "💾 Your token is saved in TOKEN variable:"
echo "   export TOKEN='$TOKEN'"
echo ""
echo "🎯 To get namespace IDs, check the database or seed data output"
echo ""
