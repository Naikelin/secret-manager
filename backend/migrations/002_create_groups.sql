-- 002_create_groups.sql
-- Create groups and user_groups tables for RBAC

CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    azure_ad_gid VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
CREATE INDEX IF NOT EXISTS idx_groups_azure_ad_gid ON groups(azure_ad_gid) WHERE azure_ad_gid IS NOT NULL;

-- Many-to-many relationship between users and groups
CREATE TABLE IF NOT EXISTS user_groups (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_user_groups_user ON user_groups(user_id);
CREATE INDEX IF NOT EXISTS idx_user_groups_group ON user_groups(group_id);
