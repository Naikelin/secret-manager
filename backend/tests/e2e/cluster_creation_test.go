package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/api"
	"github.com/yourorg/secret-manager/internal/k8s"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockClientManager is a mock implementation of k8s.ClientManager for testing
type MockClientManager struct {
	mock.Mock
}

func (m *MockClientManager) GetClient(clusterID uuid.UUID) (*k8s.K8sClient, error) {
	args := m.Called(clusterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*k8s.K8sClient), args.Error(1)
}

func (m *MockClientManager) AddClient(clusterID uuid.UUID, kubeconfigPath string) error {
	args := m.Called(clusterID, kubeconfigPath)
	return args.Error(0)
}

func (m *MockClientManager) RemoveClient(clusterID uuid.UUID) {
	m.Called(clusterID)
}

func (m *MockClientManager) HealthCheck(clusterID uuid.UUID) (bool, error) {
	args := m.Called(clusterID)
	return args.Bool(0), args.Error(1)
}

// setupE2ETestDB creates an in-memory SQLite database for E2E testing
func setupE2ETestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations for all required models
	err = db.AutoMigrate(
		&models.Cluster{},
		&models.Namespace{},
	)
	require.NoError(t, err)

	return db
}

// TestClusterCreationE2E tests the complete cluster creation flow through the API
func TestClusterCreationE2E(t *testing.T) {
	// Setup: Create in-memory test environment
	db := setupE2ETestDB(t)
	mockClientManager := new(MockClientManager)

	// Mock ClientManager to simulate successful kubeconfig loading
	// This simulates that the K8s client can be initialized with the provided kubeconfig
	mockClientManager.On("GetClient", mock.AnythingOfType("uuid.UUID")).Return(&k8s.K8sClient{}, nil).Once()

	// Initialize API handlers
	handlers := api.NewClusterHandlers(db, mockClientManager)

	// Step 1: Prepare cluster creation request
	createRequest := api.CreateClusterRequest{
		Name:          "test-cluster",
		KubeconfigRef: "kubeconfigs/test-cluster.yaml",
		Environment:   "development",
	}
	requestBody, err := json.Marshal(createRequest)
	require.NoError(t, err)

	// Step 2: POST /api/v1/clusters to create the cluster
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(requestBody))
	w := httptest.NewRecorder()

	handlers.CreateCluster(w, req)

	// Step 3: Verify response status is 201 Created
	assert.Equal(t, http.StatusCreated, w.Code, "Expected 201 Created status")

	// Step 4: Verify response body contains cluster UUID and correct data
	var responseCluster models.Cluster
	err = json.Unmarshal(w.Body.Bytes(), &responseCluster)
	require.NoError(t, err, "Response body should be valid JSON")

	assert.NotEqual(t, uuid.Nil, responseCluster.ID, "Cluster ID should not be nil")
	assert.Equal(t, "test-cluster", responseCluster.Name, "Cluster name should match request")
	assert.Equal(t, "kubeconfigs/test-cluster.yaml", responseCluster.KubeconfigRef, "Kubeconfig ref should match request")
	assert.Equal(t, "development", responseCluster.Environment, "Environment should match request")
	assert.True(t, responseCluster.IsHealthy, "Cluster should be healthy by default")

	// Step 5: Verify cluster exists in database
	var dbCluster models.Cluster
	err = db.First(&dbCluster, "id = ?", responseCluster.ID).Error
	require.NoError(t, err, "Cluster should exist in database")

	assert.Equal(t, responseCluster.ID, dbCluster.ID, "Database cluster ID should match response")
	assert.Equal(t, "test-cluster", dbCluster.Name, "Database cluster name should match")
	assert.Equal(t, "kubeconfigs/test-cluster.yaml", dbCluster.KubeconfigRef, "Database kubeconfig ref should match")
	assert.Equal(t, "development", dbCluster.Environment, "Database environment should match")

	// Step 6: Verify ClientManager can load kubeconfig (mock validation)
	// The GetClient call was already verified via the mock expectation
	mockClientManager.AssertExpectations(t)

	t.Log("✅ E2E cluster creation test passed: Full API → DB → ClientManager flow verified")
}

// TestClusterCreationE2E_ValidationErrors tests error handling in the E2E flow
func TestClusterCreationE2E_ValidationErrors(t *testing.T) {
	db := setupE2ETestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := api.NewClusterHandlers(db, mockClientManager)

	tests := []struct {
		name           string
		request        api.CreateClusterRequest
		expectedStatus int
		errorContains  string
	}{
		{
			name: "missing name",
			request: api.CreateClusterRequest{
				KubeconfigRef: "kubeconfigs/test.yaml",
				Environment:   "development",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "Missing required fields",
		},
		{
			name: "missing kubeconfig_ref",
			request: api.CreateClusterRequest{
				Name:        "test-cluster",
				Environment: "development",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "Missing required fields",
		},
		{
			name: "invalid environment",
			request: api.CreateClusterRequest{
				Name:          "test-cluster",
				KubeconfigRef: "kubeconfigs/test.yaml",
				Environment:   "invalid-env",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "Invalid environment",
		},
		{
			name: "invalid kubeconfig path with directory traversal",
			request: api.CreateClusterRequest{
				Name:          "test-cluster",
				KubeconfigRef: "../../../etc/passwd",
				Environment:   "development",
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "Invalid kubeconfig_ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(requestBody))
			w := httptest.NewRecorder()

			handlers.CreateCluster(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status code %d", tt.expectedStatus)
			assert.Contains(t, w.Body.String(), tt.errorContains, "Error message should contain: %s", tt.errorContains)
		})
	}
}

// TestClusterCreationE2E_DuplicateName tests that duplicate cluster names are rejected
func TestClusterCreationE2E_DuplicateName(t *testing.T) {
	db := setupE2ETestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := api.NewClusterHandlers(db, mockClientManager)

	// Mock GetClient for both cluster creations
	mockClientManager.On("GetClient", mock.AnythingOfType("uuid.UUID")).Return(&k8s.K8sClient{}, nil).Times(2)

	// Create first cluster
	createRequest := api.CreateClusterRequest{
		Name:          "duplicate-cluster",
		KubeconfigRef: "kubeconfigs/cluster1.yaml",
		Environment:   "development",
	}
	requestBody, _ := json.Marshal(createRequest)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(requestBody))
	w1 := httptest.NewRecorder()
	handlers.CreateCluster(w1, req1)

	assert.Equal(t, http.StatusCreated, w1.Code, "First cluster should be created successfully")

	// Try to create second cluster with same name
	createRequest2 := api.CreateClusterRequest{
		Name:          "duplicate-cluster", // Same name
		KubeconfigRef: "kubeconfigs/cluster2.yaml",
		Environment:   "staging",
	}
	requestBody2, _ := json.Marshal(createRequest2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(requestBody2))
	w2 := httptest.NewRecorder()
	handlers.CreateCluster(w2, req2)

	assert.Equal(t, http.StatusConflict, w2.Code, "Second cluster with duplicate name should be rejected")
	assert.Contains(t, w2.Body.String(), "already exists", "Error should mention duplicate name")
}

// TestClusterCreationE2E_Repeatability tests that the test can be run multiple times
func TestClusterCreationE2E_Repeatability(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run("iteration", func(t *testing.T) {
			// Each iteration gets a fresh database
			db := setupE2ETestDB(t)
			mockClientManager := new(MockClientManager)
			mockClientManager.On("GetClient", mock.AnythingOfType("uuid.UUID")).Return(&k8s.K8sClient{}, nil).Once()
			handlers := api.NewClusterHandlers(db, mockClientManager)

			createRequest := api.CreateClusterRequest{
				Name:          "repeatable-cluster",
				KubeconfigRef: "kubeconfigs/test.yaml",
				Environment:   "development",
			}
			requestBody, _ := json.Marshal(createRequest)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(requestBody))
			w := httptest.NewRecorder()

			handlers.CreateCluster(w, req)

			assert.Equal(t, http.StatusCreated, w.Code, "Cluster creation should succeed in iteration %d", i+1)

			var responseCluster models.Cluster
			json.Unmarshal(w.Body.Bytes(), &responseCluster)
			assert.Equal(t, "repeatable-cluster", responseCluster.Name)
		})
	}

	t.Log("✅ Test is repeatable: 3 iterations completed successfully")
}
