-- 003_create_namespaces.sql
-- Create namespaces table for Kubernetes namespace tracking

CREATE TABLE IF NOT EXISTS namespaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    cluster VARCHAR(255) NOT NULL,
    environment VARCHAR(50) NOT NULL CHECK (environment IN ('dev', 'staging', 'prod')),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE (name, cluster)
);

CREATE INDEX IF NOT EXISTS idx_namespaces_environment ON namespaces(environment);
CREATE INDEX IF NOT EXISTS idx_namespaces_cluster ON namespaces(cluster);
