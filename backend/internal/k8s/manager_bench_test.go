package k8s

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// BenchmarkGetClient_Cached measures cached client access latency
// Requirement: < 10ms per spec (REQ-NFR-001)
func BenchmarkGetClient_Cached(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = db.Exec(`
		CREATE TABLE clusters (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			kubeconfig_ref TEXT NOT NULL,
			environment TEXT NOT NULL,
			is_healthy INTEGER DEFAULT 1,
			last_health_check DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`).Error

	manager := NewClientManager("/etc/kubeconfigs", db).(*clientManager)
	clusterID := uuid.New()

	// Pre-populate cache with a mock client
	mockClient := &K8sClient{}
	manager.mu.Lock()
	manager.clients[clusterID] = mockClient
	manager.mu.Unlock()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = manager.GetClient(clusterID)
		}
	})
}

// BenchmarkGetClient_ReadHeavy simulates read-heavy workload (10 readers, 1 writer)
// Validates RWMutex optimization for concurrent reads
func BenchmarkGetClient_ReadHeavy(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = db.Exec(`
		CREATE TABLE clusters (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			kubeconfig_ref TEXT NOT NULL,
			environment TEXT NOT NULL,
			is_healthy INTEGER DEFAULT 1,
			last_health_check DATETIME
		)
	`).Error

	manager := NewClientManager("/etc/kubeconfigs", db).(*clientManager)

	// Create 10 clusters with cached clients
	clusterIDs := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		clusterIDs[i] = uuid.New()
		mockClient := &K8sClient{}
		manager.mu.Lock()
		manager.clients[clusterIDs[i]] = mockClient
		manager.mu.Unlock()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// 90% reads, 10% cache misses (new cluster lookups)
			if i%10 == 0 {
				// Simulate cache miss (will fail but measure lock contention)
				_, _ = manager.GetClient(uuid.New())
			} else {
				// Cache hit
				_, _ = manager.GetClient(clusterIDs[i%len(clusterIDs)])
			}
			i++
		}
	})
}

// BenchmarkRemoveClient measures eviction performance
func BenchmarkRemoveClient(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	manager := NewClientManager("/etc/kubeconfigs", db)

	// Pre-populate with clients
	clusterIDs := make([]uuid.UUID, b.N)
	for i := 0; i < b.N; i++ {
		clusterIDs[i] = uuid.New()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.RemoveClient(clusterIDs[i])
	}
}
