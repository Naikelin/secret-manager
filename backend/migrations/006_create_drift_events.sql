-- 006_create_drift_events.sql
-- Create drift_events table for tracking Git vs K8s mismatches

CREATE TABLE IF NOT EXISTS drift_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    secret_name VARCHAR(255) NOT NULL,
    namespace_id UUID REFERENCES namespaces(id) ON DELETE CASCADE,
    detected_at TIMESTAMP DEFAULT NOW(),
    git_version JSONB NOT NULL,
    k8s_version JSONB NOT NULL,
    diff JSONB NOT NULL,
    resolved_at TIMESTAMP,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    resolution_action VARCHAR(50) CHECK (resolution_action IN ('sync_from_git', 'import_to_git', 'ignore')),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_drift_events_secret_namespace ON drift_events(secret_name, namespace_id);
CREATE INDEX IF NOT EXISTS idx_drift_events_detected_at ON drift_events(detected_at);
CREATE INDEX IF NOT EXISTS idx_drift_events_unresolved ON drift_events(detected_at) WHERE resolved_at IS NULL;
