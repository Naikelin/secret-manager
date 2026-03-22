-- 007_create_audit_logs.sql
-- Create audit_logs table for tracking all user actions

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action_type VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_name VARCHAR(255) NOT NULL,
    namespace_id UUID REFERENCES namespaces(id) ON DELETE SET NULL,
    timestamp TIMESTAMP DEFAULT NOW(),
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_timestamp ON audit_logs(user_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_timestamp ON audit_logs(resource_name, timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_logs_namespace_timestamp ON audit_logs(namespace_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_type ON audit_logs(action_type);
