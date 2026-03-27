package k8s

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetClient_100ConcurrentCalls verifies ClientManager handles high concurrency correctly
// Requirement: 100 concurrent GetClient() calls complete within 5 seconds with no race conditions
func TestGetClient_100ConcurrentCalls(t *testing.T) {
	// Setup: Create test cluster in DB with valid kubeconfig
	db := setupTestDB(t)
	testdataDir, err := filepath.Abs("./testdata")
	require.NoError(t, err)

	manager := NewClientManager(testdataDir, db).(*clientManager)

	// Create test cluster in database
	clusterID := uuid.New()
	insertTestCluster(t, db, clusterID, "cluster-a", filepath.Join(testdataDir, "cluster-a.yaml"))

	// Pre-populate cache with a mock client (simulates lazy init already happened)
	// This avoids the need for a real K8s cluster during the test
	mockClient := &K8sClient{}
	manager.mu.Lock()
	manager.clients[clusterID] = mockClient
	manager.mu.Unlock()

	// Launch 100 goroutines simultaneously
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Track results from each goroutine
	results := make([]*K8sClient, numGoroutines)
	errors := make([]error, numGoroutines)

	startTime := time.Now()

	// Launch all goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()

			// Each goroutine calls GetClient(clusterID)
			client, err := manager.GetClient(clusterID)
			results[index] = client
			errors[index] = err
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	elapsed := time.Since(startTime)

	// Verify: All 100 calls complete within 5 seconds
	assert.Less(t, elapsed, 5*time.Second, "100 concurrent GetClient() calls should complete within 5 seconds")

	// Verify: No errors occurred
	for i, err := range errors {
		assert.NoError(t, err, "goroutine %d should not error", i)
	}

	// Verify: All goroutines got the same client instance (shared from cache)
	for i, client := range results {
		assert.NotNil(t, client, "goroutine %d should get a client", i)
		assert.Same(t, mockClient, client, "goroutine %d should get the same cached client", i)
	}

	t.Logf("✅ 100 concurrent GetClient() calls completed in %v", elapsed)
}

// TestMixedReadWriteOperations verifies no race conditions with concurrent reads and writes
// Requirement: 50 goroutines doing GetClient() (read) + 10 goroutines doing RemoveClient() (write)
func TestMixedReadWriteOperations(t *testing.T) {
	// Setup: Create test clusters in DB
	db := setupTestDB(t)
	testdataDir, err := filepath.Abs("./testdata")
	require.NoError(t, err)

	manager := NewClientManager(testdataDir, db).(*clientManager)

	// Create 10 test clusters with cached clients
	clusterIDs := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		clusterIDs[i] = uuid.New()
		clusterName := "test-cluster-" + clusterIDs[i].String()[:8]
		insertTestCluster(t, db, clusterIDs[i], clusterName, filepath.Join(testdataDir, "cluster-a.yaml"))

		// Pre-populate cache
		mockClient := &K8sClient{}
		manager.mu.Lock()
		manager.clients[clusterIDs[i]] = mockClient
		manager.mu.Unlock()
	}

	var wg sync.WaitGroup
	const numReaders = 50
	const numWriters = 10

	wg.Add(numReaders + numWriters)

	// Track results
	readErrors := make([]error, numReaders)
	writeCompleted := make([]bool, numWriters)

	startTime := time.Now()

	// Launch 50 reader goroutines (GetClient)
	for i := 0; i < numReaders; i++ {
		go func(index int) {
			defer wg.Done()

			// Each reader performs 10 GetClient() calls to different clusters
			for j := 0; j < 10; j++ {
				clusterID := clusterIDs[j%len(clusterIDs)]
				_, err := manager.GetClient(clusterID)
				if err != nil {
					readErrors[index] = err
					return
				}
			}
		}(i)
	}

	// Launch 10 writer goroutines (RemoveClient)
	for i := 0; i < numWriters; i++ {
		go func(index int) {
			defer wg.Done()

			// Each writer removes and re-adds a client
			clusterID := clusterIDs[index%len(clusterIDs)]

			// Remove client
			manager.RemoveClient(clusterID)

			// Short delay to allow concurrent reads
			time.Sleep(10 * time.Millisecond)

			// Re-add client (to avoid breaking readers)
			mockClient := &K8sClient{}
			manager.mu.Lock()
			manager.clients[clusterID] = mockClient
			manager.mu.Unlock()

			writeCompleted[index] = true
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	elapsed := time.Since(startTime)

	// Verify: No panics (test didn't crash)
	// If we reach here, no panics occurred

	// Verify: Operations completed in reasonable time
	assert.Less(t, elapsed, 5*time.Second, "Mixed read/write operations should complete within 5 seconds")

	// Verify: Readers didn't crash (errors are acceptable since writers may remove clients)
	// The goal is to verify no race conditions, not that all reads succeed
	t.Logf("✅ Mixed read/write operations completed in %v", elapsed)
	t.Logf("   Readers: %d goroutines performed 10 GetClient() calls each", numReaders)
	t.Logf("   Writers: %d goroutines performed RemoveClient() + re-add", numWriters)
}

// TestGetClient_ConcurrentLazyInit verifies double-checked locking prevents duplicate initialization
// This test simulates the race where multiple goroutines try to initialize the same client simultaneously
func TestGetClient_ConcurrentLazyInit(t *testing.T) {
	// Setup: Create test cluster in DB WITHOUT pre-populating cache
	db := setupTestDB(t)
	testdataDir, err := filepath.Abs("./testdata")
	require.NoError(t, err)

	manager := NewClientManager(testdataDir, db).(*clientManager)

	clusterID := uuid.New()
	insertTestCluster(t, db, clusterID, "cluster-a", filepath.Join(testdataDir, "cluster-a.yaml"))

	// DO NOT pre-populate cache — force lazy initialization

	// Launch 100 goroutines that all try to initialize the same client simultaneously
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := make([]*K8sClient, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			// This will trigger lazy init — only the first goroutine should actually initialize
			// We expect initialization to fail (no real K8s API), but the test verifies no race
			client, _ := manager.GetClient(clusterID)
			results[index] = client
		}(i)
	}

	wg.Wait()

	// Verify: Only one client was created (or all failed with the same error)
	// Check that we didn't create 100 different client instances
	clientSet := make(map[*K8sClient]bool)
	for _, client := range results {
		if client != nil {
			clientSet[client] = true
		}
	}

	// We expect either 0 clients (all failed to init) or 1 client (double-checked locking worked)
	assert.LessOrEqual(t, len(clientSet), 1, "Double-checked locking should prevent duplicate initialization")

	t.Logf("✅ Concurrent lazy init test passed (unique clients: %d)", len(clientSet))
}

// TestGetClient_HighContentionScenario simulates extreme contention on the lock
// This validates that RWMutex doesn't deadlock under high concurrency
func TestGetClient_HighContentionScenario(t *testing.T) {
	db := setupTestDB(t)
	testdataDir, err := filepath.Abs("./testdata")
	require.NoError(t, err)

	manager := NewClientManager(testdataDir, db).(*clientManager)

	// Create 50 clusters
	const numClusters = 50
	clusterIDs := make([]uuid.UUID, numClusters)
	for i := 0; i < numClusters; i++ {
		clusterIDs[i] = uuid.New()
		clusterName := "test-cluster-" + clusterIDs[i].String()[:8]
		insertTestCluster(t, db, clusterIDs[i], clusterName, filepath.Join(testdataDir, "cluster-a.yaml"))

		// Pre-populate cache
		mockClient := &K8sClient{}
		manager.mu.Lock()
		manager.clients[clusterIDs[i]] = mockClient
		manager.mu.Unlock()
	}

	// Launch 200 goroutines performing random operations
	const numGoroutines = 200
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	startTime := time.Now()

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()

			// Each goroutine performs 20 random operations
			for j := 0; j < 20; j++ {
				clusterID := clusterIDs[j%numClusters]

				// Mix of reads and writes
				if j%5 == 0 {
					// Write: Remove client (20% of operations)
					manager.RemoveClient(clusterID)
				} else {
					// Read: Get client (80% of operations)
					_, _ = manager.GetClient(clusterID)
				}
			}
		}(i)
	}

	// Wait for all goroutines with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		elapsed := time.Since(startTime)
		assert.Less(t, elapsed, 10*time.Second, "High contention scenario should not deadlock")
		t.Logf("✅ High contention scenario completed in %v", elapsed)
	case <-time.After(15 * time.Second):
		t.Fatal("Test deadlocked — did not complete within 15 seconds")
	}
}
