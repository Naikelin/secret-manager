-- 004_create_group_permissions.sql
-- Create group_permissions table for RBAC permissions

CREATE TABLE IF NOT EXISTS group_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    namespace_id UUID REFERENCES namespaces(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL CHECK (role IN ('viewer', 'editor', 'admin')),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE (group_id, namespace_id)
);

CREATE INDEX IF NOT EXISTS idx_group_permissions_group ON group_permissions(group_id);
CREATE INDEX IF NOT EXISTS idx_group_permissions_namespace ON group_permissions(namespace_id);
CREATE INDEX IF NOT EXISTS idx_group_permissions_role ON group_permissions(role);
