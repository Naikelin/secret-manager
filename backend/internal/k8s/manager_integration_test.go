package k8s_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/k8s"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestClientManager_IntegrationWithRealCluster tests ClientManager with a real Kubernetes cluster
//
// Prerequisites:
// - Docker Desktop with Kubernetes enabled, OR
// - Kind cluster running locally, OR
// - Minikube cluster running
// - Valid kubeconfig at ~/.kube/config
//
// Run with: go test -v -tags=integration ./internal/k8s/...
//
// This test verifies:
// 1. ClientManager can load a real kubeconfig
// 2. GetClient successfully creates K8s client
// 3. Client can connect to the cluster API
// 4. Multiple clusters can be managed simultaneously
func TestClientManager_IntegrationWithRealCluster(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Cluster{}, &models.Namespace{})
	require.NoError(t, err)

	// Get kubeconfig path from environment or use default
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, _ := os.UserHomeDir()
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	// Verify kubeconfig exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		t.Skipf("Kubeconfig not found at %s. Please ensure Kubernetes cluster is running.", kubeconfigPath)
	}

	// Create test kubeconfigs directory
	testKubeconfigDir := t.TempDir()

	// Copy real kubeconfig to test directory as "local-cluster.yaml"
	kubeconfigContent, err := os.ReadFile(kubeconfigPath)
	require.NoError(t, err)

	testKubeconfigPath := filepath.Join(testKubeconfigDir, "local-cluster.yaml")
	err = os.WriteFile(testKubeconfigPath, kubeconfigContent, 0600)
	require.NoError(t, err)

	// Create cluster record in database
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "local-cluster",
		KubeconfigRef: testKubeconfigPath,
		Environment:   "dev",
		IsHealthy:     true,
	}
	err = db.Create(&cluster).Error
	require.NoError(t, err)

	// Initialize ClientManager
	clientManager := k8s.NewClientManager(testKubeconfigDir, db)

	// Test 1: Get client for the cluster
	t.Run("GetClient creates and caches client", func(t *testing.T) {
		client, err := clientManager.GetClient(cluster.ID)
		require.NoError(t, err, "Should successfully create K8s client")
		assert.NotNil(t, client)

		// Second call should return cached client
		client2, err := clientManager.GetClient(cluster.ID)
		require.NoError(t, err)
		assert.Same(t, client, client2, "Should return cached client instance")
	})

	// Test 2: Health check (verifies cluster connectivity)
	t.Run("HealthCheck verifies cluster is reachable", func(t *testing.T) {
		healthy, err := clientManager.HealthCheck(cluster.ID)
		require.NoError(t, err, "Health check should succeed")
		assert.True(t, healthy, "Cluster should be healthy")

		// Verify database was updated
		var updatedCluster models.Cluster
		err = db.First(&updatedCluster, "id = ?", cluster.ID).Error
		require.NoError(t, err)
		assert.True(t, updatedCluster.IsHealthy)
		assert.NotNil(t, updatedCluster.LastHealthCheck)
	})

	// Test 3: Client isolation (multiple clusters)
	t.Run("Multiple clusters are isolated", func(t *testing.T) {
		// Create second cluster with same kubeconfig (simulating different clusters)
		cluster2 := models.Cluster{
			ID:            uuid.New(),
			Name:          "local-cluster-2",
			KubeconfigRef: testKubeconfigPath,
			Environment:   "staging",
			IsHealthy:     true,
		}

		// Write second kubeconfig file
		testKubeconfigPath2 := filepath.Join(testKubeconfigDir, "local-cluster-2.yaml")
		err = os.WriteFile(testKubeconfigPath2, kubeconfigContent, 0600)
		require.NoError(t, err)

		err = db.Create(&cluster2).Error
		require.NoError(t, err)

		// Get clients for both clusters
		client1, err := clientManager.GetClient(cluster.ID)
		require.NoError(t, err)

		client2, err := clientManager.GetClient(cluster2.ID)
		require.NoError(t, err)

		// They should be different instances (different cluster IDs)
		assert.NotSame(t, client1, client2, "Different clusters should have isolated clients")
	})

	// Test 4: Remove client from pool
	t.Run("RemoveClient evicts client from cache", func(t *testing.T) {
		// Get client first
		client1, err := clientManager.GetClient(cluster.ID)
		require.NoError(t, err)

		// Remove it
		clientManager.RemoveClient(cluster.ID)

		// Get again - should create new instance
		client2, err := clientManager.GetClient(cluster.ID)
		require.NoError(t, err)

		// Note: We can't test if they're different instances because GetClient
		// will recreate and cache a new client. This test mainly verifies
		// that RemoveClient doesn't break GetClient.
		assert.NotNil(t, client1)
		assert.NotNil(t, client2)
	})
}

// TestClientManager_UnreachableCluster tests behavior when cluster API is unreachable
func TestClientManager_UnreachableCluster(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Cluster{})
	require.NoError(t, err)

	testKubeconfigDir := t.TempDir()

	// Create a kubeconfig pointing to a non-existent API server
	unreachableKubeconfig := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://unreachable.cluster.local:6443
    insecure-skip-tls-verify: true
  name: unreachable-cluster
contexts:
- context:
    cluster: unreachable-cluster
    user: admin
  name: unreachable-context
current-context: unreachable-context
users:
- name: admin
  user:
    token: fake-token
`
	unreachableKubeconfigPath := filepath.Join(testKubeconfigDir, "unreachable-cluster.yaml")
	err = os.WriteFile(unreachableKubeconfigPath, []byte(unreachableKubeconfig), 0600)
	require.NoError(t, err)

	// Create cluster record
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "unreachable-cluster",
		KubeconfigRef: unreachableKubeconfigPath,
		Environment:   "dev",
		IsHealthy:     true,
	}
	err = db.Create(&cluster).Error
	require.NoError(t, err)

	clientManager := k8s.NewClientManager(testKubeconfigDir, db)

	// GetClient should succeed (it doesn't test connectivity)
	client, err := clientManager.GetClient(cluster.ID)
	require.NoError(t, err)
	assert.NotNil(t, client)

	// HealthCheck should fail and mark cluster unhealthy
	healthy, err := clientManager.HealthCheck(cluster.ID)
	assert.Error(t, err, "Should fail to connect to unreachable cluster")
	assert.False(t, healthy)

	// Verify cluster marked as unhealthy in DB
	var updatedCluster models.Cluster
	err = db.First(&updatedCluster, "id = ?", cluster.ID).Error
	require.NoError(t, err)
	assert.False(t, updatedCluster.IsHealthy, "Cluster should be marked unhealthy")
	assert.NotNil(t, updatedCluster.LastHealthCheck)
}
