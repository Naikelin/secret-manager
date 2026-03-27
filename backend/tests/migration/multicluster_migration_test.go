package migration

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestMultiClusterMigration tests the complete multi-cluster migration flow (008 → 009 → 010)
func TestMultiClusterMigration(t *testing.T) {
	// Setup: Create in-memory SQLite database for testing
	db, sqlDB := setupTestDB(t)
	defer sqlDB.Close()

	t.Run("Full migration flow: 008 → 009 → 010", func(t *testing.T) {
		// Step 1: Create OLD schema (before migration 008) with TEXT cluster field
		createOldSchema(t, sqlDB)

		// Step 2: Insert test data with OLD schema
		insertOldSchemaData(t, sqlDB)

		// Step 3: Apply migration 008 (create clusters table)
		applyMigration008(t, sqlDB)

		// Step 4: Verify migration 008 results
		verifyMigration008(t, db)

		// Step 5: Apply migration 009 (add cluster_id FK)
		applyMigration009(t, sqlDB)

		// Step 6: Verify migration 009 results
		verifyMigration009(t, db)

		// Step 7: Backfill clusters and namespace cluster_id
		backfillClusters(t, sqlDB)

		// Step 8: Verify backfill data integrity
		verifyBackfillIntegrity(t, db, sqlDB)

		// Step 9: Apply migration 010 (drop old cluster column)
		applyMigration010(t, sqlDB)

		// Step 10: Verify migration 010 results and final schema
		verifyMigration010(t, db)

		// Step 11: Verify no data loss throughout migration
		verifyFinalDataIntegrity(t, db)
	})
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) (*gorm.DB, *sql.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	return db, sqlDB
}

// createOldSchema creates the pre-migration 008 schema (namespaces with TEXT cluster)
func createOldSchema(t *testing.T, sqlDB *sql.DB) {
	schema := `
CREATE TABLE IF NOT EXISTS namespaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cluster TEXT NOT NULL,
    environment TEXT NOT NULL CHECK (environment IN ('dev', 'staging', 'prod')),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (name, cluster)
);

CREATE INDEX IF NOT EXISTS idx_namespaces_environment ON namespaces(environment);
CREATE INDEX IF NOT EXISTS idx_namespaces_cluster ON namespaces(cluster);
`
	_, err := sqlDB.Exec(schema)
	require.NoError(t, err, "Failed to create old schema")
}

// insertOldSchemaData inserts test data with OLD schema (TEXT cluster field)
func insertOldSchemaData(t *testing.T, sqlDB *sql.DB) {
	testData := `
INSERT INTO namespaces (id, name, cluster, environment) VALUES
('ns-1', 'default', 'devops', 'dev'),
('ns-2', 'kube-system', 'devops', 'dev'),
('ns-3', 'production', 'integraciones-prod', 'prod'),
('ns-4', 'staging', 'integraciones-stg', 'staging');
`
	_, err := sqlDB.Exec(testData)
	require.NoError(t, err, "Failed to insert old schema test data")
}

// applyMigration008 creates the clusters table
func applyMigration008(t *testing.T, sqlDB *sql.DB) {
	migration := `
CREATE TABLE IF NOT EXISTS clusters (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    kubeconfig_ref TEXT NOT NULL,
    environment TEXT NOT NULL CHECK (environment IN ('dev', 'staging', 'prod')),
    is_healthy INTEGER DEFAULT 1,
    last_health_check TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_clusters_environment ON clusters(environment);
CREATE INDEX IF NOT EXISTS idx_clusters_health ON clusters(is_healthy);
`
	_, err := sqlDB.Exec(migration)
	require.NoError(t, err, "Failed to apply migration 008")
}

// verifyMigration008 verifies that clusters table was created correctly
func verifyMigration008(t *testing.T, db *gorm.DB) {
	// Verify clusters table exists
	var tableExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master 
			WHERE type='table' AND name='clusters'
		)
	`).Scan(&tableExists).Error
	require.NoError(t, err)
	assert.True(t, tableExists, "Clusters table should exist after migration 008")

	// Verify indexes exist
	var indexCount int64
	err = db.Raw(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='index' AND tbl_name='clusters'
		AND name IN ('idx_clusters_environment', 'idx_clusters_health')
	`).Scan(&indexCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), indexCount, "Should have 2 indexes on clusters table")

	// Verify old namespaces data still intact
	var namespaceCount int64
	err = db.Raw("SELECT COUNT(*) FROM namespaces").Scan(&namespaceCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(4), namespaceCount, "All 4 namespaces should still exist")
}

// applyMigration009 adds cluster_id FK column to namespaces
func applyMigration009(t *testing.T, sqlDB *sql.DB) {
	migration := `
-- Add nullable cluster_id column
ALTER TABLE namespaces ADD COLUMN cluster_id TEXT;

-- Create index for FK lookups
CREATE INDEX IF NOT EXISTS idx_namespaces_cluster_id ON namespaces(cluster_id);

-- Note: SQLite doesn't support adding FK constraints to existing tables,
-- so we skip the FOREIGN KEY constraint for this test
`
	_, err := sqlDB.Exec(migration)
	require.NoError(t, err, "Failed to apply migration 009")
}

// verifyMigration009 verifies that cluster_id column was added
func verifyMigration009(t *testing.T, db *gorm.DB) {
	// Verify cluster_id column exists
	var columnExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pragma_table_info('namespaces')
			WHERE name = 'cluster_id'
		)
	`).Scan(&columnExists).Error
	require.NoError(t, err)
	assert.True(t, columnExists, "cluster_id column should exist after migration 009")

	// Verify both cluster (TEXT) and cluster_id columns exist
	var columnCount int64
	err = db.Raw(`
		SELECT COUNT(*) FROM pragma_table_info('namespaces')
		WHERE name IN ('cluster', 'cluster_id')
	`).Scan(&columnCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), columnCount, "Both cluster and cluster_id columns should exist during transition")
}

// backfillClusters simulates the backfill script logic
func backfillClusters(t *testing.T, sqlDB *sql.DB) {
	backfill := `
-- Extract distinct clusters and insert into clusters table
INSERT INTO clusters (id, name, kubeconfig_ref, environment)
SELECT 
    lower(hex(randomblob(16))) AS id,
    cluster, 
    '/etc/kubeconfigs/' || cluster || '.yaml' AS kubeconfig_ref,
    environment
FROM (
    SELECT DISTINCT cluster, environment FROM namespaces
);

-- Update namespace.cluster_id FK
UPDATE namespaces
SET cluster_id = (
    SELECT id FROM clusters WHERE clusters.name = namespaces.cluster
);
`
	_, err := sqlDB.Exec(backfill)
	require.NoError(t, err, "Failed to backfill clusters")
}

// verifyBackfillIntegrity verifies data integrity after backfill
func verifyBackfillIntegrity(t *testing.T, db *gorm.DB, sqlDB *sql.DB) {
	// Verify all distinct clusters were created
	var clusterCount int64
	err := db.Raw("SELECT COUNT(*) FROM clusters").Scan(&clusterCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(3), clusterCount, "Should have 3 distinct clusters (devops, integraciones-prod, integraciones-stg)")

	// Verify all namespaces have cluster_id set
	var nullCount int64
	err = db.Raw("SELECT COUNT(*) FROM namespaces WHERE cluster_id IS NULL").Scan(&nullCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), nullCount, "All namespaces should have cluster_id set after backfill")

	// Verify FK integrity - join should work
	type Result struct {
		NamespaceName string
		ClusterName   string
		OldCluster    string
	}
	var results []Result
	err = db.Raw(`
		SELECT 
			n.name AS namespace_name, 
			c.name AS cluster_name,
			n.cluster AS old_cluster
		FROM namespaces n
		JOIN clusters c ON n.cluster_id = c.id
		ORDER BY n.name
	`).Scan(&results).Error
	require.NoError(t, err)
	assert.Len(t, results, 4, "Should join all 4 namespaces with their clusters")

	// Verify data integrity: cluster_id points to correct cluster
	for _, r := range results {
		assert.Equal(t, r.OldCluster, r.ClusterName,
			"Namespace %s: cluster_id should point to cluster matching old TEXT cluster field", r.NamespaceName)
	}

	// Verify cluster names match expected values
	expectedClusters := map[string]string{
		"default":     "devops",
		"kube-system": "devops",
		"production":  "integraciones-prod",
		"staging":     "integraciones-stg",
	}
	for _, r := range results {
		expected, ok := expectedClusters[r.NamespaceName]
		require.True(t, ok, "Unexpected namespace: %s", r.NamespaceName)
		assert.Equal(t, expected, r.ClusterName, "Namespace %s should be in cluster %s", r.NamespaceName, expected)
	}
}

// applyMigration010 drops the old cluster TEXT column
func applyMigration010(t *testing.T, sqlDB *sql.DB) {
	// SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
	migration := `
-- Create temporary table with new schema
CREATE TABLE namespaces_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cluster_id TEXT NOT NULL,
    environment TEXT NOT NULL CHECK (environment IN ('dev', 'staging', 'prod')),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Copy data from old table
INSERT INTO namespaces_new (id, name, cluster_id, environment, created_at)
SELECT id, name, cluster_id, environment, created_at
FROM namespaces;

-- Drop old table
DROP TABLE namespaces;

-- Rename new table
ALTER TABLE namespaces_new RENAME TO namespaces;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_namespaces_environment ON namespaces(environment);
CREATE INDEX IF NOT EXISTS idx_namespaces_cluster_id ON namespaces(cluster_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_namespaces_cluster_name ON namespaces(cluster_id, name);
`
	_, err := sqlDB.Exec(migration)
	require.NoError(t, err, "Failed to apply migration 010")
}

// verifyMigration010 verifies that old cluster column was dropped
func verifyMigration010(t *testing.T, db *gorm.DB) {
	// Verify old cluster TEXT column no longer exists
	var columnExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pragma_table_info('namespaces')
			WHERE name = 'cluster'
		)
	`).Scan(&columnExists).Error
	require.NoError(t, err)
	assert.False(t, columnExists, "Old cluster TEXT column should be dropped after migration 010")

	// Verify cluster_id column still exists and is NOT NULL
	var clusterIDInfo struct {
		Name    string
		NotNull int
	}
	err = db.Raw(`
		SELECT name, "notnull" as not_null FROM pragma_table_info('namespaces')
		WHERE name = 'cluster_id'
	`).Scan(&clusterIDInfo).Error
	require.NoError(t, err)
	assert.Equal(t, "cluster_id", clusterIDInfo.Name)
	assert.Equal(t, 1, clusterIDInfo.NotNull, "cluster_id should be NOT NULL after migration 010")

	// Verify unique index on (cluster_id, name) exists
	var indexExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master 
			WHERE type='index' AND tbl_name='namespaces'
			AND name = 'idx_namespaces_cluster_name'
		)
	`).Scan(&indexExists).Error
	require.NoError(t, err)
	assert.True(t, indexExists, "Unique index on (cluster_id, name) should exist")
}

// verifyFinalDataIntegrity verifies no data loss throughout entire migration
func verifyFinalDataIntegrity(t *testing.T, db *gorm.DB) {
	// Verify all 4 namespaces still exist
	var namespaceCount int64
	err := db.Raw("SELECT COUNT(*) FROM namespaces").Scan(&namespaceCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(4), namespaceCount, "All 4 namespaces should exist after full migration")

	// Verify all 3 clusters exist
	var clusterCount int64
	err = db.Raw("SELECT COUNT(*) FROM clusters").Scan(&clusterCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(3), clusterCount, "All 3 clusters should exist after full migration")

	// Verify all namespaces can still join with clusters
	type FinalResult struct {
		NamespaceName string
		ClusterName   string
		Environment   string
	}
	var results []FinalResult
	err = db.Raw(`
		SELECT 
			n.name AS namespace_name, 
			c.name AS cluster_name,
			n.environment AS environment
		FROM namespaces n
		JOIN clusters c ON n.cluster_id = c.id
		ORDER BY n.name
	`).Scan(&results).Error
	require.NoError(t, err)
	assert.Len(t, results, 4, "Should join all 4 namespaces with their clusters after migration")

	// Verify specific namespace → cluster mappings
	expectedMappings := map[string]string{
		"default":     "devops",
		"kube-system": "devops",
		"production":  "integraciones-prod",
		"staging":     "integraciones-stg",
	}

	for _, r := range results {
		expected := expectedMappings[r.NamespaceName]
		assert.Equal(t, expected, r.ClusterName,
			"Namespace %s should be in cluster %s after full migration", r.NamespaceName, expected)
	}

	t.Log("✅ Multi-cluster migration complete: No data loss, all relationships preserved")
}

// TestMigrationRollback tests rolling back migrations (optional)
func TestMigrationRollback(t *testing.T) {
	t.Skip("Rollback testing not implemented - migrations are designed to be forward-only")

	// Future implementation could test DOWN migrations if added
	// For now, multi-cluster migration is designed to be irreversible after backfill
}
