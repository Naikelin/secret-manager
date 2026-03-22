#!/bin/bash
set -e

echo "🌱 Seeding development data..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL..."
until docker compose exec -T postgres pg_isready -U dev > /dev/null 2>&1; do
    sleep 1
done

echo "✅ PostgreSQL is ready"

# Seed data using SQL
docker compose exec -T postgres psql -U dev -d secretmanager <<'EOSQL'
-- Seed Users
INSERT INTO users (id, email, name, azure_ad_oid) VALUES
    ('11111111-1111-1111-1111-111111111111', 'admin@example.com', 'Admin User', NULL),
    ('22222222-2222-2222-2222-222222222222', 'dev@example.com', 'Dev User', NULL)
ON CONFLICT (email) DO NOTHING;

-- Seed Groups
INSERT INTO groups (id, name, azure_ad_gid) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'admins', NULL),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'developers', NULL)
ON CONFLICT (name) DO NOTHING;

-- Seed User-Group memberships
INSERT INTO user_groups (user_id, group_id) VALUES
    ('11111111-1111-1111-1111-111111111111', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'),
    ('22222222-2222-2222-2222-222222222222', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb')
ON CONFLICT DO NOTHING;

-- Seed Namespaces
INSERT INTO namespaces (id, name, cluster, environment) VALUES
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 'development', 'kind-secretmanager', 'dev'),
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'staging', 'aks-staging', 'staging'),
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', 'production', 'aks-prod', 'prod')
ON CONFLICT (name, cluster) DO NOTHING;

-- Seed Group Permissions
INSERT INTO group_permissions (group_id, namespace_id, role) VALUES
    -- Admins have admin role on all namespaces
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'admin'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'admin'),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'admin'),
    -- Developers have editor role on development namespace only
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'editor')
ON CONFLICT (group_id, namespace_id) DO NOTHING;

-- Seed Sample Draft Secret
INSERT INTO secret_drafts (secret_name, namespace_id, data, status, edited_by) VALUES
    ('db-credentials', 'dddddddd-dddd-dddd-dddd-dddddddddddd', '{"username": "postgres", "password": "changeme123"}'::jsonb, 'draft', '22222222-2222-2222-2222-222222222222')
ON CONFLICT DO NOTHING;

EOSQL

echo "✅ Seed data inserted successfully!"
echo ""
echo "Sample Users:"
echo "  - admin@example.com (admin role on all namespaces)"
echo "  - dev@example.com (editor role on development namespace)"
echo ""
echo "Sample Namespaces:"
echo "  - development (dev cluster)"
echo "  - staging (staging cluster)"
echo "  - production (prod cluster)"
