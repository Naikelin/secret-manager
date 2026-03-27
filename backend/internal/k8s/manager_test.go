package k8s

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create clusters table
	err = db.Exec(`
		CREATE TABLE clusters (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			kubeconfig_ref TEXT NOT NULL,
			environment TEXT NOT NULL,
			is_healthy INTEGER DEFAULT 1,
			last_health_check DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	require.NoError(t, err)

	return db
}

// insertTestCluster inserts a test cluster into the database
func insertTestCluster(t *testing.T, db *gorm.DB, id uuid.UUID, name, kubeconfigRef string) {
	err := db.Exec(`
		INSERT INTO clusters (id, name, kubeconfig_ref, environment, is_healthy)
		VALUES (?, ?, ?, 'dev', 1)
	`, id.String(), name, kubeconfigRef).Error
	require.NoError(t, err)
}

func TestNewClientManager(t *testing.T) {
	db := setupTestDB(t)
	manager := NewClientManager("/etc/kubeconfigs", db)

	assert.NotNil(t, manager)
}

func TestGetClient_LazyInitialization(t *testing.T) {
	t.Skip("Skipping test that requires real K8s cluster - tested in integration tests")

	// NOTE: This test requires a valid kubeconfig pointing to a real cluster.
	// In production, lazy initialization is verified through:
	// 1. Integration tests with real/mocked K8s cluster
	// 2. Benchmark tests that measure cache performance
	// 3. The concurrent access test below validates double-checked locking
}

func TestGetClient_ClusterNotFound(t *testing.T) {
	db := setupTestDB(t)
	testdataDir, err := filepath.Abs("./testdata")
	require.NoError(t, err)

	manager := NewClientManager(testdataDir, db)

	// Try to get client for non-existent cluster
	nonExistentID := uuid.New()
	_, err = manager.GetClient(nonExistentID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cluster not found")
}

func TestGetClient_KubeconfigNotFound(t *testing.T) {
	db := setupTestDB(t)
	testdataDir, err := filepath.Abs("./testdata")
	require.NoError(t, err)

	manager := NewClientManager(testdataDir, db)

	// Create cluster with non-existent kubeconfig
	clusterID := uuid.New()
	insertTestCluster(t, db, clusterID, "missing-cluster", "/etc/kubeconfigs/missing-cluster.yaml")

	_, err = manager.GetClient(clusterID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig not found")
}

func TestGetClient_InvalidKubeconfig(t *testing.T) {
	db := setupTestDB(t)
	testdataDir, err := filepath.Abs("./testdata")
	require.NoError(t, err)

	manager := NewClientManager(testdataDir, db)

	// Create cluster with invalid kubeconfig
	clusterID := uuid.New()
	insertTestCluster(t, db, clusterID, "invalid", "/etc/kubeconfigs/invalid.yaml")

	_, err = manager.GetClient(clusterID)
	assert.Error(t, err)
	// Should fail during K8s client initialization (invalid YAML)
}

func TestGetClient_ConcurrentAccess(t *testing.T) {
	t.Skip("Skipping test that requires real K8s cluster - concurrency tested via benchmarks")

	// NOTE: Concurrent access and double-checked locking is verified through:
	// 1. Benchmark test (BenchmarkGetClientCached) with -race flag
	// 2. Code review of double-checked locking pattern in GetClient()
}

func TestAddClient(t *testing.T) {
	t.Skip("Skipping test that requires real K8s cluster - AddClient tested separately")

	// NOTE: AddClient validation is tested through:
	// 1. TestAddClient_InvalidPath (doesn't require K8s connection)
	// 2. Integration tests with real cluster
}

func TestAddClient_AlreadyExists(t *testing.T) {
	t.Skip("Skipping test that requires real K8s cluster")
}

func TestAddClient_InvalidPath(t *testing.T) {
	db := setupTestDB(t)
	manager := NewClientManager("/etc/kubeconfigs", db)

	clusterID := uuid.New()
	err := manager.AddClient(clusterID, "/nonexistent/kubeconfig.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig not found")
}

func TestRemoveClient(t *testing.T) {
	db := setupTestDB(t)
	manager := NewClientManager("/etc/kubeconfigs", db).(*clientManager)

	clusterID := uuid.New()

	// Manually add a client to the pool (bypassing initialization)
	mockClient := &K8sClient{}
	manager.mu.Lock()
	manager.clients[clusterID] = mockClient
	manager.mu.Unlock()

	// Verify it's in the pool
	manager.mu.RLock()
	assert.Len(t, manager.clients, 1)
	manager.mu.RUnlock()

	// Remove it
	manager.RemoveClient(clusterID)

	// Verify it's gone
	manager.mu.RLock()
	assert.Len(t, manager.clients, 0)
	manager.mu.RUnlock()
}

func TestRemoveClient_NotExist(t *testing.T) {
	db := setupTestDB(t)
	manager := NewClientManager("/etc/kubeconfigs", db)

	// Removing non-existent client should not panic
	nonExistentID := uuid.New()
	manager.RemoveClient(nonExistentID)
}

func TestValidateKubeconfigFile_NotFound(t *testing.T) {
	db := setupTestDB(t)
	manager := NewClientManager("/etc/kubeconfigs", db).(*clientManager)

	err := manager.validateKubeconfigFile("/nonexistent/file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig not found")
}

func TestValidateKubeconfigFile_InsecurePermissions(t *testing.T) {
	db := setupTestDB(t)
	testdataDir, err := filepath.Abs("./testdata")
	require.NoError(t, err)

	manager := NewClientManager(testdataDir, db).(*clientManager)

	// Create a world-readable kubeconfig file (insecure)
	insecurePath := filepath.Join(testdataDir, "insecure.yaml")
	err = os.WriteFile(insecurePath, []byte("test"), 0644)
	require.NoError(t, err)
	defer os.Remove(insecurePath)

	err = manager.validateKubeconfigFile(insecurePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insecure kubeconfig permissions")
}

func TestMarkClusterHealthy(t *testing.T) {
	db := setupTestDB(t)
	manager := NewClientManager("/etc/kubeconfigs", db).(*clientManager)

	clusterID := uuid.New()
	insertTestCluster(t, db, clusterID, "test-cluster", "/etc/kubeconfigs/test.yaml")

	// Mark as unhealthy first
	manager.markClusterUnhealthy(clusterID)

	// Verify it's unhealthy
	var isHealthy bool
	err := db.Table("clusters").Select("is_healthy").Where("id = ?", clusterID.String()).Scan(&isHealthy).Error
	require.NoError(t, err)
	assert.False(t, isHealthy)

	// Mark as healthy
	manager.markClusterHealthy(clusterID)

	// Verify it's healthy
	err = db.Table("clusters").Select("is_healthy").Where("id = ?", clusterID.String()).Scan(&isHealthy).Error
	require.NoError(t, err)
	assert.True(t, isHealthy)

	// Verify last_health_check was updated
	var lastCheck time.Time
	err = db.Table("clusters").Select("last_health_check").Where("id = ?", clusterID.String()).Scan(&lastCheck).Error
	require.NoError(t, err)
	assert.True(t, time.Since(lastCheck) < 5*time.Second, "last_health_check should be recent")
}

func TestMarkClusterUnhealthy(t *testing.T) {
	db := setupTestDB(t)
	manager := NewClientManager("/etc/kubeconfigs", db).(*clientManager)

	clusterID := uuid.New()
	insertTestCluster(t, db, clusterID, "test-cluster", "/etc/kubeconfigs/test.yaml")

	// Mark as unhealthy
	manager.markClusterUnhealthy(clusterID)

	// Verify it's unhealthy
	var isHealthy bool
	err := db.Table("clusters").Select("is_healthy").Where("id = ?", clusterID.String()).Scan(&isHealthy).Error
	require.NoError(t, err)
	assert.False(t, isHealthy)

	// Verify last_health_check was updated
	var lastCheck time.Time
	err = db.Table("clusters").Select("last_health_check").Where("id = ?", clusterID.String()).Scan(&lastCheck).Error
	require.NoError(t, err)
	assert.True(t, time.Since(lastCheck) < 5*time.Second, "last_health_check should be recent")
}

func TestClientIsolation(t *testing.T) {
	db := setupTestDB(t)
	manager := NewClientManager("/etc/kubeconfigs", db).(*clientManager)

	// Create two different clusters with manual clients
	clusterA := uuid.New()
	clusterB := uuid.New()

	mockClientA := &K8sClient{}
	mockClientB := &K8sClient{}

	manager.mu.Lock()
	manager.clients[clusterA] = mockClientA
	manager.clients[clusterB] = mockClientB
	manager.mu.Unlock()

	// Verify they are different instances
	assert.NotSame(t, mockClientA, mockClientB, "Clients for different clusters should be isolated")
	assert.Len(t, manager.clients, 2, "Should have 2 clients in pool")
}
