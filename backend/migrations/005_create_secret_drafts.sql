-- 005_create_secret_drafts.sql
-- Create secret_drafts table for staging area workflow

CREATE TABLE IF NOT EXISTS secret_drafts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    secret_name VARCHAR(255) NOT NULL,
    namespace_id UUID REFERENCES namespaces(id) ON DELETE CASCADE,
    data JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'drifted')),
    git_base_sha VARCHAR(64),
    edited_by UUID REFERENCES users(id) ON DELETE SET NULL,
    edited_at TIMESTAMP DEFAULT NOW(),
    published_by UUID REFERENCES users(id) ON DELETE SET NULL,
    published_at TIMESTAMP,
    commit_sha VARCHAR(64),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_secret_drafts_secret_namespace ON secret_drafts(secret_name, namespace_id);
CREATE INDEX IF NOT EXISTS idx_secret_drafts_status ON secret_drafts(status);
CREATE INDEX IF NOT EXISTS idx_secret_drafts_edited_by ON secret_drafts(edited_by);
CREATE INDEX IF NOT EXISTS idx_secret_drafts_auto_discard ON secret_drafts(edited_at) WHERE status = 'draft';
CREATE INDEX IF NOT EXISTS idx_secret_drafts_namespace ON secret_drafts(namespace_id);
