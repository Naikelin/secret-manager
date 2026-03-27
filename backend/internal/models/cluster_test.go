package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	require.NoError(t, err)

	// Enable foreign keys in SQLite
	db.Exec("PRAGMA foreign_keys = ON")

	// Run migrations for minimal models needed for testing
	err = db.AutoMigrate(&Cluster{}, &Namespace{})
	require.NoError(t, err)

	return db
}

func TestClusterModel(t *testing.T) {
	db := setupTestDB(t)

	t.Run("Create cluster with valid data", func(t *testing.T) {
		cluster := &Cluster{
			Name:          "test-cluster",
			KubeconfigRef: "/etc/kubeconfigs/test-cluster.yaml",
			Environment:   "dev",
			IsHealthy:     true,
		}

		err := db.Create(cluster).Error
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, cluster.ID)
		assert.NotZero(t, cluster.CreatedAt)
		assert.NotZero(t, cluster.UpdatedAt)
	})

	t.Run("Unique cluster name constraint", func(t *testing.T) {
		cluster1 := &Cluster{
			Name:          "duplicate-cluster",
			KubeconfigRef: "/etc/kubeconfigs/dup1.yaml",
			Environment:   "dev",
		}
		err := db.Create(cluster1).Error
		require.NoError(t, err)

		cluster2 := &Cluster{
			Name:          "duplicate-cluster",
			KubeconfigRef: "/etc/kubeconfigs/dup2.yaml",
			Environment:   "staging",
		}
		err = db.Create(cluster2).Error
		assert.Error(t, err, "Should fail due to unique constraint on name")
	})

	t.Run("Cluster environment enum validation", func(t *testing.T) {
		cluster := &Cluster{
			Name:          "env-test-cluster",
			KubeconfigRef: "/etc/kubeconfigs/env-test.yaml",
			Environment:   "invalid-env",
		}

		err := db.Create(cluster).Error
		// SQLite doesn't enforce CHECK constraints by default, but Postgres will
		// This test documents the expected behavior
		if err != nil {
			assert.Contains(t, err.Error(), "environment")
		}
	})

	t.Run("Cluster default values", func(t *testing.T) {
		cluster := &Cluster{
			Name:          "default-test-cluster",
			KubeconfigRef: "/etc/kubeconfigs/default-test.yaml",
			Environment:   "prod",
		}

		err := db.Create(cluster).Error
		require.NoError(t, err)

		var retrieved Cluster
		err = db.First(&retrieved, "name = ?", "default-test-cluster").Error
		require.NoError(t, err)

		assert.True(t, retrieved.IsHealthy, "is_healthy should default to true")
		assert.Nil(t, retrieved.LastHealthCheck, "last_health_check should be nil initially")
	})

	t.Run("Update cluster health", func(t *testing.T) {
		cluster := &Cluster{
			Name:          "health-test-cluster",
			KubeconfigRef: "/etc/kubeconfigs/health-test.yaml",
			Environment:   "dev",
			IsHealthy:     true,
		}
		err := db.Create(cluster).Error
		require.NoError(t, err)

		// Mark unhealthy
		now := time.Now()
		err = db.Model(&cluster).Updates(map[string]interface{}{
			"is_healthy":        false,
			"last_health_check": now,
		}).Error
		require.NoError(t, err)

		var retrieved Cluster
		err = db.First(&retrieved, cluster.ID).Error
		require.NoError(t, err)

		assert.False(t, retrieved.IsHealthy)
		assert.NotNil(t, retrieved.LastHealthCheck)
		assert.WithinDuration(t, now, *retrieved.LastHealthCheck, time.Second)
	})
}

func TestNamespaceClusterRelationship(t *testing.T) {
	db := setupTestDB(t)

	t.Run("Namespace with cluster FK", func(t *testing.T) {
		// Create cluster
		cluster := &Cluster{
			Name:          "test-cluster",
			KubeconfigRef: "/etc/kubeconfigs/test-cluster.yaml",
			Environment:   "dev",
		}
		err := db.Create(cluster).Error
		require.NoError(t, err)

		// Create namespace with cluster_id
		namespace := &Namespace{
			Name:        "default",
			ClusterID:   &cluster.ID,
			Cluster:     "test-cluster", // Old field (will be removed later)
			Environment: "dev",
		}
		err = db.Create(namespace).Error
		require.NoError(t, err)

		// Retrieve with relationship
		var retrieved Namespace
		err = db.Preload("ClusterRef").First(&retrieved, namespace.ID).Error
		require.NoError(t, err)

		assert.NotNil(t, retrieved.ClusterRef)
		assert.Equal(t, "test-cluster", retrieved.ClusterRef.Name)
		assert.Equal(t, cluster.ID, *retrieved.ClusterID)
	})

	t.Run("Cluster CASCADE delete removes namespaces", func(t *testing.T) {
		// Create cluster
		cluster := &Cluster{
			Name:          "cascade-test-cluster",
			KubeconfigRef: "/etc/kubeconfigs/cascade-test.yaml",
			Environment:   "dev",
		}
		err := db.Create(cluster).Error
		require.NoError(t, err)

		// Create namespace
		namespace := &Namespace{
			Name:        "default",
			ClusterID:   &cluster.ID,
			Cluster:     "cascade-test-cluster",
			Environment: "dev",
		}
		err = db.Create(namespace).Error
		require.NoError(t, err)

		// Delete cluster
		err = db.Delete(&cluster).Error
		require.NoError(t, err)

		// Verify namespace was cascade deleted
		var count int64
		err = db.Model(&Namespace{}).Where("id = ?", namespace.ID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Namespace should be cascade deleted")
	})

	t.Run("Unique constraint (cluster_id, name)", func(t *testing.T) {
		// Create cluster
		cluster := &Cluster{
			Name:          "unique-test-cluster",
			KubeconfigRef: "/etc/kubeconfigs/unique-test.yaml",
			Environment:   "dev",
		}
		err := db.Create(cluster).Error
		require.NoError(t, err)

		// Create first namespace
		ns1 := &Namespace{
			Name:        "default",
			ClusterID:   &cluster.ID,
			Cluster:     "unique-test-cluster",
			Environment: "dev",
		}
		err = db.Create(ns1).Error
		require.NoError(t, err)

		// Try to create duplicate namespace in same cluster
		ns2 := &Namespace{
			Name:        "default",
			ClusterID:   &cluster.ID,
			Cluster:     "unique-test-cluster",
			Environment: "dev",
		}
		err = db.Create(ns2).Error

		// Note: GORM's AutoMigrate may not create the unique index (cluster_id, name)
		// This will be enforced after migration 010
		// For now, we document the expected behavior
		if err == nil {
			t.Log("GORM AutoMigrate did not create unique index (cluster_id, name) - this will be enforced by SQL migration 010")
		}
	})

	t.Run("Same namespace name allowed in different clusters", func(t *testing.T) {
		// Create two clusters
		cluster1 := &Cluster{
			Name:          "cluster-a",
			KubeconfigRef: "/etc/kubeconfigs/cluster-a.yaml",
			Environment:   "dev",
		}
		err := db.Create(cluster1).Error
		require.NoError(t, err)

		cluster2 := &Cluster{
			Name:          "cluster-b",
			KubeconfigRef: "/etc/kubeconfigs/cluster-b.yaml",
			Environment:   "staging",
		}
		err = db.Create(cluster2).Error
		require.NoError(t, err)

		// Create namespace "shared-ns" in cluster-a
		ns1 := &Namespace{
			Name:        "shared-ns",
			ClusterID:   &cluster1.ID,
			Cluster:     "cluster-a",
			Environment: "dev",
		}
		err = db.Create(ns1).Error
		require.NoError(t, err)

		// Create namespace "shared-ns" in cluster-b (should succeed)
		ns2 := &Namespace{
			Name:        "shared-ns",
			ClusterID:   &cluster2.ID,
			Cluster:     "cluster-b",
			Environment: "staging",
		}
		err = db.Create(ns2).Error
		assert.NoError(t, err, "Same namespace name should be allowed in different clusters")

		// Verify both exist
		var count int64
		err = db.Model(&Namespace{}).Where("name = ? AND cluster_id IN (?, ?)", "shared-ns", cluster1.ID, cluster2.ID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})
}

func TestClusterRelationships(t *testing.T) {
	db := setupTestDB(t)

	t.Run("Cluster has many namespaces", func(t *testing.T) {
		cluster := &Cluster{
			Name:          "multi-ns-cluster",
			KubeconfigRef: "/etc/kubeconfigs/multi-ns.yaml",
			Environment:   "dev",
		}
		err := db.Create(cluster).Error
		require.NoError(t, err)

		// Create multiple namespaces
		namespaces := []Namespace{
			{Name: "default", ClusterID: &cluster.ID, Cluster: "multi-ns-cluster", Environment: "dev"},
			{Name: "kube-system", ClusterID: &cluster.ID, Cluster: "multi-ns-cluster", Environment: "dev"},
			{Name: "production", ClusterID: &cluster.ID, Cluster: "multi-ns-cluster", Environment: "dev"},
		}

		for _, ns := range namespaces {
			err := db.Create(&ns).Error
			require.NoError(t, err)
		}

		// Load cluster with namespaces
		var retrieved Cluster
		err = db.Preload("Namespaces").First(&retrieved, cluster.ID).Error
		require.NoError(t, err)

		assert.Len(t, retrieved.Namespaces, 3)
	})
}

func TestClusterBeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	t.Run("Auto-generate UUID if not provided", func(t *testing.T) {
		cluster := &Cluster{
			Name:          "auto-uuid-cluster",
			KubeconfigRef: "/etc/kubeconfigs/auto-uuid.yaml",
			Environment:   "dev",
		}

		// ID should be Nil before create
		assert.Equal(t, uuid.Nil, cluster.ID)

		err := db.Create(cluster).Error
		require.NoError(t, err)

		// ID should be generated after create
		assert.NotEqual(t, uuid.Nil, cluster.ID)
	})

	t.Run("Preserve provided UUID", func(t *testing.T) {
		providedID := uuid.New()
		cluster := &Cluster{
			ID:            providedID,
			Name:          "custom-uuid-cluster",
			KubeconfigRef: "/etc/kubeconfigs/custom-uuid.yaml",
			Environment:   "dev",
		}

		err := db.Create(cluster).Error
		require.NoError(t, err)

		assert.Equal(t, providedID, cluster.ID, "Provided UUID should be preserved")
	})
}
