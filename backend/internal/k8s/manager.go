package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ClientManager manages a pool of Kubernetes clients for multiple clusters
// with lazy initialization and thread-safe access
type ClientManager interface {
	GetClient(clusterID uuid.UUID) (*K8sClient, error)
	AddClient(clusterID uuid.UUID, kubeconfigPath string) error
	RemoveClient(clusterID uuid.UUID)
	HealthCheck(clusterID uuid.UUID) (bool, error)
}

// clientManager implements ClientManager with lazy initialization
type clientManager struct {
	clients        map[uuid.UUID]*K8sClient
	mu             sync.RWMutex
	kubeconfigsDir string
	db             *gorm.DB
}

// NewClientManager creates a new ClientManager instance
// kubeconfigsDir: directory where kubeconfig files are stored (e.g., "/etc/kubeconfigs")
// db: database connection to lookup cluster metadata
func NewClientManager(kubeconfigsDir string, db *gorm.DB) ClientManager {
	return &clientManager{
		clients:        make(map[uuid.UUID]*K8sClient),
		kubeconfigsDir: kubeconfigsDir,
		db:             db,
	}
}

// GetClient returns a cached K8s client or initializes it lazily on first access.
// Uses double-checked locking pattern for optimal performance:
// - Fast path: RLock for read-only check (cached clients)
// - Slow path: Lock for initialization (only on first access per cluster)
func (m *clientManager) GetClient(clusterID uuid.UUID) (*K8sClient, error) {
	// Fast path: check if client already exists (read lock only)
	m.mu.RLock()
	if client, exists := m.clients[clusterID]; exists {
		m.mu.RUnlock()
		return client, nil
	}
	m.mu.RUnlock()

	// Slow path: need to initialize client (write lock required)
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	// (another goroutine might have initialized while we waited for the lock)
	if client, exists := m.clients[clusterID]; exists {
		return client, nil
	}

	// Load cluster metadata from database to get name and kubeconfig_ref
	var cluster struct {
		Name          string
		KubeconfigRef string
	}
	err := m.db.Table("clusters").
		Select("name, kubeconfig_ref").
		Where("id = ?", clusterID).
		First(&cluster).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("cluster not found: %s", clusterID)
		}
		return nil, fmt.Errorf("failed to lookup cluster: %w", err)
	}

	// Construct kubeconfig path from kubeconfigsDir and cluster name
	// Format: {kubeconfigsDir}/{cluster-name}.yaml
	kubeconfigPath := filepath.Join(m.kubeconfigsDir, fmt.Sprintf("%s.yaml", cluster.Name))

	// Verify kubeconfig file exists and has correct permissions
	if err := m.validateKubeconfigFile(kubeconfigPath); err != nil {
		return nil, fmt.Errorf("invalid kubeconfig for cluster %s: %w", cluster.Name, err)
	}

	// Initialize K8s client
	client, err := NewK8sClient(kubeconfigPath)
	if err != nil {
		// Mark cluster as unhealthy in database
		m.markClusterUnhealthy(clusterID)
		return nil, fmt.Errorf("failed to initialize client for cluster %s: %w", cluster.Name, err)
	}

	// Cache the client for future requests
	m.clients[clusterID] = client
	return client, nil
}

// AddClient explicitly adds a client to the pool with a given kubeconfig path.
// Useful for testing or pre-warming the cache.
func (m *clientManager) AddClient(clusterID uuid.UUID, kubeconfigPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	if _, exists := m.clients[clusterID]; exists {
		return fmt.Errorf("client already exists for cluster %s", clusterID)
	}

	// Validate kubeconfig file
	if err := m.validateKubeconfigFile(kubeconfigPath); err != nil {
		return fmt.Errorf("invalid kubeconfig: %w", err)
	}

	// Create client
	client, err := NewK8sClient(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	m.clients[clusterID] = client
	return nil
}

// RemoveClient evicts a client from the pool.
// Used for manual cleanup or when a cluster becomes unhealthy.
func (m *clientManager) RemoveClient(clusterID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, clusterID)
}

// HealthCheck pings the cluster API server to verify it's reachable.
// Returns true if healthy, false if unreachable or erroring.
func (m *clientManager) HealthCheck(clusterID uuid.UUID) (bool, error) {
	client, err := m.GetClient(clusterID)
	if err != nil {
		return false, fmt.Errorf("failed to get client: %w", err)
	}

	// Ping cluster by checking version endpoint (lightweight operation)
	// Try to access the API server discovery endpoint
	_, err = client.clientset.Discovery().ServerVersion()
	if err != nil {
		// Mark cluster as unhealthy
		m.markClusterUnhealthy(clusterID)
		return false, fmt.Errorf("cluster unreachable: %w", err)
	}

	// Update health check timestamp
	m.markClusterHealthy(clusterID)
	return true, nil
}

// validateKubeconfigFile checks if kubeconfig exists and has secure permissions
func (m *clientManager) validateKubeconfigFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig not found: %s", path)
		}
		return fmt.Errorf("failed to stat kubeconfig: %w", err)
	}

	// Check file permissions (should be 0600 or 0400, not world-readable)
	mode := info.Mode().Perm()
	if mode&0044 != 0 { // Check if group or others have read permission
		return fmt.Errorf("insecure kubeconfig permissions %o (should be 0600 or 0400)", mode)
	}

	return nil
}

// markClusterUnhealthy updates the cluster's is_healthy flag to false
func (m *clientManager) markClusterUnhealthy(clusterID uuid.UUID) {
	now := time.Now()
	m.db.Table("clusters").
		Where("id = ?", clusterID).
		Updates(map[string]interface{}{
			"is_healthy":        false,
			"last_health_check": now,
		})
}

// markClusterHealthy updates the cluster's is_healthy flag to true
func (m *clientManager) markClusterHealthy(clusterID uuid.UUID) {
	now := time.Now()
	m.db.Table("clusters").
		Where("id = ?", clusterID).
		Updates(map[string]interface{}{
			"is_healthy":        true,
			"last_health_check": now,
		})
}
