-- 010_drop_old_cluster_column.sql
-- Migrate namespace.cluster from TEXT to UUID FK (Phase 2: Drop old column)
-- ONLY run this after backfill script completes successfully!

-- ============================================================================
-- UP Migration
-- ============================================================================

-- Verify all namespaces have cluster_id set before proceeding
-- This should return 0 rows:
-- SELECT id, name, cluster FROM namespaces WHERE cluster_id IS NULL;

-- Make cluster_id NOT NULL
ALTER TABLE namespaces ALTER COLUMN cluster_id SET NOT NULL;

-- Drop old cluster TEXT column
ALTER TABLE namespaces DROP COLUMN IF EXISTS cluster;

-- Update unique constraint from (name, cluster) to (name, cluster_id)
DROP INDEX IF EXISTS namespaces_name_cluster_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_namespaces_cluster_name ON namespaces(cluster_id, name);

-- Drop old cluster index (no longer needed)
DROP INDEX IF EXISTS idx_namespaces_cluster;

COMMENT ON TABLE namespaces IS 'Kubernetes namespaces - now normalized with cluster FK';

-- ============================================================================
-- DOWN Migration (Rollback)
-- ============================================================================

-- To rollback:
-- ALTER TABLE namespaces ADD COLUMN cluster VARCHAR(255);
-- UPDATE namespaces SET cluster = clusters.name FROM clusters WHERE namespaces.cluster_id = clusters.id;
-- ALTER TABLE namespaces ALTER COLUMN cluster SET NOT NULL;
-- ALTER TABLE namespaces ALTER COLUMN cluster_id DROP NOT NULL;
-- DROP INDEX IF EXISTS idx_namespaces_cluster_name;
-- CREATE INDEX idx_namespaces_cluster ON namespaces(cluster);
-- CREATE UNIQUE INDEX namespaces_name_cluster_key ON namespaces(name, cluster);
