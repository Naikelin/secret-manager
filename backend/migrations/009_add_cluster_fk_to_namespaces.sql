-- 009_add_cluster_fk_to_namespaces.sql
-- Migrate namespace.cluster from TEXT to UUID FK (Phase 1: Add cluster_id)

-- ============================================================================
-- UP Migration
-- ============================================================================

-- Phase 1: Add nullable cluster_id column
ALTER TABLE namespaces ADD COLUMN IF NOT EXISTS cluster_id UUID;

-- Create index for FK lookups
CREATE INDEX IF NOT EXISTS idx_namespaces_cluster_id ON namespaces(cluster_id);

-- Add FK constraint (will be enforced after backfill)
-- Note: Constraint is initially NOT enforced to allow NULL values during backfill
ALTER TABLE namespaces 
    ADD CONSTRAINT fk_namespaces_cluster 
    FOREIGN KEY (cluster_id) 
    REFERENCES clusters(id) 
    ON DELETE CASCADE;

COMMENT ON COLUMN namespaces.cluster_id IS 'Foreign key to clusters table - replaces cluster TEXT field';

-- ============================================================================
-- DOWN Migration (Rollback)
-- ============================================================================

-- To rollback:
-- ALTER TABLE namespaces DROP CONSTRAINT IF EXISTS fk_namespaces_cluster;
-- DROP INDEX IF EXISTS idx_namespaces_cluster_id;
-- ALTER TABLE namespaces DROP COLUMN IF EXISTS cluster_id;
