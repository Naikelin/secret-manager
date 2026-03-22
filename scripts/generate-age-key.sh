#!/bin/bash
set -e

echo "🔐 Generating Age encryption key..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
AGE_KEY_PATH="${PROJECT_ROOT}/dev-data/age-key.txt"

# Check if age is installed
if ! command -v age-keygen &> /dev/null; then
    echo "❌ age is not installed. Please install it first:"
    echo "   brew install age  # macOS"
    echo "   apt install age   # Debian/Ubuntu"
    exit 1
fi

# Create dev-data directory if it doesn't exist
mkdir -p "${PROJECT_ROOT}/dev-data"

# Generate Age key if it doesn't exist
if [ -f "$AGE_KEY_PATH" ]; then
    echo "⚠️  Age key already exists at: $AGE_KEY_PATH"
    read -p "Do you want to regenerate it? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "ℹ️  Keeping existing key"
        exit 0
    fi
fi

# Generate new Age key
age-keygen -o "$AGE_KEY_PATH"

echo "✅ Age key generated successfully at: $AGE_KEY_PATH"
echo ""
echo "⚠️  IMPORTANT: This key is for development only!"
echo "   Never commit this file to version control."
echo ""
echo "Public key (add to .sops.yaml):"
grep "public key:" "$AGE_KEY_PATH" | awk '{print $NF}'
