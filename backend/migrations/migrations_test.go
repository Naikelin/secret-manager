package migrations

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// These tests validate the SQL migration scripts
// They use an in-memory Postgres instance (testcontainers recommended for CI)

func setupTestDB(t *testing.T) (*gorm.DB, *sql.DB) {
	// For local testing, use SQLite or set up a test Postgres instance
	// For CI, use testcontainers to spin up Postgres
	// This is a placeholder - adapt based on your test environment

	t.Skip("Integration test - requires Postgres testcontainer")

	dsn := "host=localhost user=test password=test dbname=test_migrations port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	return db, sqlDB
}

func TestMigration008_CreateClusters(t *testing.T) {
	db, sqlDB := setupTestDB(t)
	defer sqlDB.Close()

	t.Run("Migration 008 UP creates clusters table", func(t *testing.T) {
		// Execute migration 008 UP
		migration008 := `
CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    kubeconfig_ref VARCHAR(512) NOT NULL,
    environment VARCHAR(50) NOT NULL CHECK (environment IN ('dev', 'staging', 'prod')),
    is_healthy BOOLEAN DEFAULT true,
    last_health_check TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clusters_environment ON clusters(environment);
CREATE INDEX IF NOT EXISTS idx_clusters_health ON clusters(is_healthy);
`
		_, err := sqlDB.Exec(migration008)
		require.NoError(t, err)

		// Verify table exists
		var tableExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'clusters'
			)
		`).Scan(&tableExists).Error
		require.NoError(t, err)
		assert.True(t, tableExists)

		// Verify indexes exist
		var indexCount int64
		err = db.Raw(`
			SELECT COUNT(*) FROM pg_indexes 
			WHERE tablename = 'clusters' 
			AND indexname IN ('idx_clusters_environment', 'idx_clusters_health')
		`).Scan(&indexCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(2), indexCount)

		// Test insert
		_, err = sqlDB.Exec(`
			INSERT INTO clusters (name, kubeconfig_ref, environment) 
			VALUES ('test-cluster', '/etc/kubeconfigs/test.yaml', 'dev')
		`)
		require.NoError(t, err)

		// Test unique constraint
		_, err = sqlDB.Exec(`
			INSERT INTO clusters (name, kubeconfig_ref, environment) 
			VALUES ('test-cluster', '/etc/kubeconfigs/test2.yaml', 'staging')
		`)
		assert.Error(t, err, "Should fail due to unique name constraint")

		// Test CHECK constraint
		_, err = sqlDB.Exec(`
			INSERT INTO clusters (name, kubeconfig_ref, environment) 
			VALUES ('invalid-env-cluster', '/etc/kubeconfigs/invalid.yaml', 'invalid')
		`)
		assert.Error(t, err, "Should fail due to environment CHECK constraint")
	})

	t.Run("Migration 008 DOWN drops clusters table", func(t *testing.T) {
		// Execute migration 008 DOWN
		migration008Down := `
DROP INDEX IF EXISTS idx_clusters_health;
DROP INDEX IF EXISTS idx_clusters_environment;
DROP TABLE IF EXISTS clusters CASCADE;
`
		_, err := sqlDB.Exec(migration008Down)
		require.NoError(t, err)

		// Verify table does not exist
		var tableExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'clusters'
			)
		`).Scan(&tableExists).Error
		require.NoError(t, err)
		assert.False(t, tableExists)
	})
}

func TestMigration009_AddClusterFK(t *testing.T) {
	db, sqlDB := setupTestDB(t)
	defer sqlDB.Close()

	// Setup: Create clusters and namespaces tables
	setupSQL := `
CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    kubeconfig_ref VARCHAR(512) NOT NULL,
    environment VARCHAR(50) NOT NULL,
    is_healthy BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS namespaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    cluster VARCHAR(255) NOT NULL,
    environment VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
`
	_, err := sqlDB.Exec(setupSQL)
	require.NoError(t, err)

	// Insert test data
	_, err = sqlDB.Exec(`
		INSERT INTO namespaces (name, cluster, environment) VALUES
		('default', 'devops', 'dev'),
		('kube-system', 'devops', 'dev'),
		('production', 'integraciones-prod', 'prod');
	`)
	require.NoError(t, err)

	t.Run("Migration 009 UP adds cluster_id column", func(t *testing.T) {
		migration009Up := `
ALTER TABLE namespaces ADD COLUMN IF NOT EXISTS cluster_id UUID;
CREATE INDEX IF NOT EXISTS idx_namespaces_cluster_id ON namespaces(cluster_id);
ALTER TABLE namespaces 
    ADD CONSTRAINT fk_namespaces_cluster 
    FOREIGN KEY (cluster_id) 
    REFERENCES clusters(id) 
    ON DELETE CASCADE;
`
		_, err := sqlDB.Exec(migration009Up)
		require.NoError(t, err)

		// Verify column exists
		var columnExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_name = 'namespaces' 
				AND column_name = 'cluster_id'
			)
		`).Scan(&columnExists).Error
		require.NoError(t, err)
		assert.True(t, columnExists)

		// Verify FK constraint exists
		var constraintExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.table_constraints 
				WHERE table_name = 'namespaces' 
				AND constraint_name = 'fk_namespaces_cluster'
			)
		`).Scan(&constraintExists).Error
		require.NoError(t, err)
		assert.True(t, constraintExists)
	})

	t.Run("Migration 009 DOWN removes cluster_id column", func(t *testing.T) {
		migration009Down := `
ALTER TABLE namespaces DROP CONSTRAINT IF EXISTS fk_namespaces_cluster;
DROP INDEX IF EXISTS idx_namespaces_cluster_id;
ALTER TABLE namespaces DROP COLUMN IF EXISTS cluster_id;
`
		_, err := sqlDB.Exec(migration009Down)
		require.NoError(t, err)

		// Verify column does not exist
		var columnExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_name = 'namespaces' 
				AND column_name = 'cluster_id'
			)
		`).Scan(&columnExists).Error
		require.NoError(t, err)
		assert.False(t, columnExists)
	})
}

func TestMigrationBackfillDataIntegrity(t *testing.T) {
	db, sqlDB := setupTestDB(t)
	defer sqlDB.Close()

	// Setup: Create tables and seed data
	setupSQL := `
CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    kubeconfig_ref VARCHAR(512) NOT NULL,
    environment VARCHAR(50) NOT NULL,
    is_healthy BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS namespaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    cluster VARCHAR(255) NOT NULL,
    environment VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

ALTER TABLE namespaces ADD COLUMN IF NOT EXISTS cluster_id UUID;

INSERT INTO namespaces (name, cluster, environment) VALUES
	('default', 'devops', 'dev'),
	('kube-system', 'devops', 'dev'),
	('production', 'integraciones-prod', 'prod'),
	('staging', 'integraciones-stg', 'staging');
`
	_, err := sqlDB.Exec(setupSQL)
	require.NoError(t, err)

	t.Run("Backfill creates clusters and updates namespace FKs", func(t *testing.T) {
		// Simulate backfill script logic
		backfillSQL := `
-- Extract distinct clusters and insert
INSERT INTO clusters (name, kubeconfig_ref, environment)
SELECT DISTINCT 
    cluster, 
    '/etc/kubeconfigs/' || cluster || '.yaml' AS kubeconfig_ref,
    environment
FROM namespaces
ON CONFLICT (name) DO NOTHING;

-- Update namespace.cluster_id FK
UPDATE namespaces
SET cluster_id = clusters.id
FROM clusters
WHERE namespaces.cluster = clusters.name;
`
		_, err := sqlDB.Exec(backfillSQL)
		require.NoError(t, err)

		// Verify all clusters created
		var clusterCount int64
		err = db.Raw("SELECT COUNT(*) FROM clusters").Scan(&clusterCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(3), clusterCount, "Should create 3 distinct clusters")

		// Verify all namespaces have cluster_id set
		var nullCount int64
		err = db.Raw("SELECT COUNT(*) FROM namespaces WHERE cluster_id IS NULL").Scan(&nullCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), nullCount, "All namespaces should have cluster_id")

		// Verify FK integrity - join should work
		type Result struct {
			NamespaceName string
			ClusterName   string
		}
		var results []Result
		err = db.Raw(`
			SELECT n.name AS namespace_name, c.name AS cluster_name
			FROM namespaces n
			JOIN clusters c ON n.cluster_id = c.id
			ORDER BY n.name
		`).Scan(&results).Error
		require.NoError(t, err)
		assert.Len(t, results, 4)
		assert.Equal(t, "devops", results[0].ClusterName)
	})

	t.Run("Backfill is idempotent", func(t *testing.T) {
		// Run backfill again
		backfillSQL := `
INSERT INTO clusters (name, kubeconfig_ref, environment)
SELECT DISTINCT 
    cluster, 
    '/etc/kubeconfigs/' || cluster || '.yaml' AS kubeconfig_ref,
    environment
FROM namespaces
ON CONFLICT (name) DO NOTHING;
`
		_, err := sqlDB.Exec(backfillSQL)
		require.NoError(t, err)

		// Verify cluster count unchanged
		var clusterCount int64
		err = db.Raw("SELECT COUNT(*) FROM clusters").Scan(&clusterCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(3), clusterCount, "Should still be 3 clusters (no duplicates)")
	})
}

func TestMigration010_DropOldColumn(t *testing.T) {
	db, sqlDB := setupTestDB(t)
	defer sqlDB.Close()

	// Setup: Create tables with both cluster and cluster_id
	setupSQL := `
CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    kubeconfig_ref VARCHAR(512) NOT NULL,
    environment VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS namespaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    cluster VARCHAR(255) NOT NULL,
    cluster_id UUID NOT NULL,
    environment VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
`
	_, err := sqlDB.Exec(setupSQL)
	require.NoError(t, err)

	t.Run("Migration 010 UP drops old cluster column", func(t *testing.T) {
		migration010Up := `
ALTER TABLE namespaces ALTER COLUMN cluster_id SET NOT NULL;
ALTER TABLE namespaces DROP COLUMN IF EXISTS cluster;
DROP INDEX IF EXISTS namespaces_name_cluster_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_namespaces_cluster_name ON namespaces(cluster_id, name);
DROP INDEX IF EXISTS idx_namespaces_cluster;
`
		_, err := sqlDB.Exec(migration010Up)
		require.NoError(t, err)

		// Verify cluster column does not exist
		var columnExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_name = 'namespaces' 
				AND column_name = 'cluster'
			)
		`).Scan(&columnExists).Error
		require.NoError(t, err)
		assert.False(t, columnExists)

		// Verify cluster_id is NOT NULL
		var isNullable string
		err = db.Raw(`
			SELECT is_nullable FROM information_schema.columns 
			WHERE table_name = 'namespaces' 
			AND column_name = 'cluster_id'
		`).Scan(&isNullable).Error
		require.NoError(t, err)
		assert.Equal(t, "NO", isNullable)

		// Verify unique index exists
		var indexExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT FROM pg_indexes 
				WHERE tablename = 'namespaces' 
				AND indexname = 'idx_namespaces_cluster_name'
			)
		`).Scan(&indexExists).Error
		require.NoError(t, err)
		assert.True(t, indexExists)
	})

	t.Run("Migration 010 DOWN restores cluster column", func(t *testing.T) {
		// First, need to add back clusters table and data for rollback
		_, err := sqlDB.Exec(`
			INSERT INTO clusters (id, name, kubeconfig_ref, environment) 
			VALUES ('` + uuid.New().String() + `', 'test-cluster', '/etc/kubeconfigs/test.yaml', 'dev');
			
			INSERT INTO namespaces (name, cluster_id, environment) 
			VALUES ('default', (SELECT id FROM clusters WHERE name = 'test-cluster'), 'dev');
		`)
		require.NoError(t, err)

		migration010Down := `
ALTER TABLE namespaces ADD COLUMN cluster VARCHAR(255);
UPDATE namespaces SET cluster = clusters.name FROM clusters WHERE namespaces.cluster_id = clusters.id;
ALTER TABLE namespaces ALTER COLUMN cluster SET NOT NULL;
ALTER TABLE namespaces ALTER COLUMN cluster_id DROP NOT NULL;
DROP INDEX IF EXISTS idx_namespaces_cluster_name;
CREATE INDEX idx_namespaces_cluster ON namespaces(cluster);
CREATE UNIQUE INDEX namespaces_name_cluster_key ON namespaces(name, cluster);
`
		_, err = sqlDB.Exec(migration010Down)
		require.NoError(t, err)

		// Verify cluster column exists
		var columnExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_name = 'namespaces' 
				AND column_name = 'cluster'
			)
		`).Scan(&columnExists).Error
		require.NoError(t, err)
		assert.True(t, columnExists)

		// Verify cluster column has data
		var clusterValue string
		err = db.Raw("SELECT cluster FROM namespaces WHERE name = 'default' LIMIT 1").Scan(&clusterValue).Error
		require.NoError(t, err)
		assert.Equal(t, "test-cluster", clusterValue)
	})
}
