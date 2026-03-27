package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

// setupTestDB creates an in-memory SQLite database for testing
func setupClusterTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Cluster{}, &models.Namespace{})
	assert.NoError(t, err)

	return db
}

func TestListClusters(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Seed test data
	cluster1 := models.Cluster{
		ID:            uuid.New(),
		Name:          "devops",
		KubeconfigRef: "/etc/kubeconfigs/devops.yaml",
		Environment:   "prod",
		IsHealthy:     true,
	}
	cluster2 := models.Cluster{
		ID:            uuid.New(),
		Name:          "staging",
		KubeconfigRef: "/etc/kubeconfigs/staging.yaml",
		Environment:   "staging",
		IsHealthy:     false,
	}
	db.Create(&cluster1)
	db.Create(&cluster2)

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/clusters", nil)
	w := httptest.NewRecorder()

	// Execute handler
	handlers.ListClusters(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var clusters []models.Cluster
	err := json.Unmarshal(w.Body.Bytes(), &clusters)
	assert.NoError(t, err)
	assert.Len(t, clusters, 2)
	assert.Equal(t, "devops", clusters[0].Name)
	assert.Equal(t, "staging", clusters[1].Name)
}

func TestGetCluster(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Seed test data
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "devops",
		KubeconfigRef: "/etc/kubeconfigs/devops.yaml",
		Environment:   "prod",
		IsHealthy:     true,
	}
	db.Create(&cluster)

	// Create request with cluster ID in URL
	req := httptest.NewRequest("GET", "/api/v1/clusters/"+cluster.ID.String(), nil)
	w := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cluster.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute handler
	handlers.GetCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseCluster models.Cluster
	err := json.Unmarshal(w.Body.Bytes(), &responseCluster)
	assert.NoError(t, err)
	assert.Equal(t, cluster.ID, responseCluster.ID)
	assert.Equal(t, "devops", responseCluster.Name)
}

func TestGetCluster_NotFound(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Create request with non-existent cluster ID
	nonExistentID := uuid.New()
	req := httptest.NewRequest("GET", "/api/v1/clusters/"+nonExistentID.String(), nil)
	w := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", nonExistentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute handler
	handlers.GetCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Cluster not found")
}

func TestGetCluster_InvalidID(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Create request with invalid cluster ID
	req := httptest.NewRequest("GET", "/api/v1/clusters/invalid-uuid", nil)
	w := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute handler
	handlers.GetCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid cluster ID format")
}

func TestCreateCluster(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Mock GetClient to succeed (validates kubeconfig)
	mockClientManager.On("GetClient", mock.AnythingOfType("uuid.UUID")).Return(&k8s.K8sClient{}, nil).Once()

	// Create request body
	reqBody := CreateClusterRequest{
		Name:          "test-cluster",
		KubeconfigRef: "/etc/kubeconfigs/test-cluster.yaml",
		Environment:   "dev",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/clusters", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	// Execute handler
	handlers.CreateCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseCluster models.Cluster
	err := json.Unmarshal(w.Body.Bytes(), &responseCluster)
	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", responseCluster.Name)
	assert.Equal(t, "dev", responseCluster.Environment)
	assert.NotEqual(t, uuid.Nil, responseCluster.ID)

	// Verify cluster was saved to database
	var dbCluster models.Cluster
	err = db.First(&dbCluster, "name = ?", "test-cluster").Error
	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", dbCluster.Name)

	mockClientManager.AssertExpectations(t)
}

func TestCreateCluster_DuplicateName(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Seed existing cluster
	existingCluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "devops",
		KubeconfigRef: "/etc/kubeconfigs/devops.yaml",
		Environment:   "prod",
	}
	db.Create(&existingCluster)

	// Try to create duplicate
	reqBody := CreateClusterRequest{
		Name:          "devops",
		KubeconfigRef: "/etc/kubeconfigs/devops-new.yaml",
		Environment:   "dev",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/clusters", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	// Execute handler
	handlers.CreateCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "Cluster name already exists")
}

func TestCreateCluster_InvalidEnvironment(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Create request with invalid environment
	reqBody := CreateClusterRequest{
		Name:          "test-cluster",
		KubeconfigRef: "/etc/kubeconfigs/test-cluster.yaml",
		Environment:   "invalid-env",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/clusters", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	// Execute handler
	handlers.CreateCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid environment")
}

func TestCreateCluster_MissingFields(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Create request with missing fields
	reqBody := CreateClusterRequest{
		Name: "test-cluster",
		// Missing KubeconfigRef and Environment
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/clusters", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	// Execute handler
	handlers.CreateCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing required fields")
}

func TestDeleteCluster(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Seed test data
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "test-cluster",
		KubeconfigRef: "/etc/kubeconfigs/test-cluster.yaml",
		Environment:   "dev",
	}
	db.Create(&cluster)

	// Mock RemoveClient
	mockClientManager.On("RemoveClient", cluster.ID).Return().Once()

	// Create request
	req := httptest.NewRequest("DELETE", "/api/v1/clusters/"+cluster.ID.String(), nil)
	w := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cluster.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute handler
	handlers.DeleteCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify cluster was deleted from database
	var dbCluster models.Cluster
	err := db.First(&dbCluster, "id = ?", cluster.ID).Error
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)

	mockClientManager.AssertExpectations(t)
}

func TestDeleteCluster_WithNamespaces(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Seed test data with cluster and namespace
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "test-cluster",
		KubeconfigRef: "/etc/kubeconfigs/test-cluster.yaml",
		Environment:   "dev",
	}
	db.Create(&cluster)

	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "default",
		ClusterID:   &cluster.ID,
		Cluster:     "test-cluster", // Legacy field
		Environment: "dev",
	}
	db.Create(&namespace)

	// Create request
	req := httptest.NewRequest("DELETE", "/api/v1/clusters/"+cluster.ID.String(), nil)
	w := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cluster.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute handler
	handlers.DeleteCluster(w, req)

	// Assert response
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "Cannot delete cluster")
	assert.Contains(t, w.Body.String(), "namespace(s) are associated")

	// Verify cluster still exists
	var dbCluster models.Cluster
	err := db.First(&dbCluster, "id = ?", cluster.ID).Error
	assert.NoError(t, err)
}

func TestListClusterNamespaces(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Seed test data
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "devops",
		KubeconfigRef: "/etc/kubeconfigs/devops.yaml",
		Environment:   "prod",
	}
	db.Create(&cluster)

	ns1 := models.Namespace{
		ID:          uuid.New(),
		Name:        "default",
		ClusterID:   &cluster.ID,
		Cluster:     "devops",
		Environment: "prod",
	}
	ns2 := models.Namespace{
		ID:          uuid.New(),
		Name:        "production",
		ClusterID:   &cluster.ID,
		Cluster:     "devops",
		Environment: "prod",
	}
	db.Create(&ns1)
	db.Create(&ns2)

	// Create namespace in different cluster (should not appear in results)
	otherCluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "staging",
		KubeconfigRef: "/etc/kubeconfigs/staging.yaml",
		Environment:   "staging",
	}
	db.Create(&otherCluster)
	otherNs := models.Namespace{
		ID:          uuid.New(),
		Name:        "staging-ns",
		ClusterID:   &otherCluster.ID,
		Cluster:     "staging",
		Environment: "staging",
	}
	db.Create(&otherNs)

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/clusters/"+cluster.ID.String()+"/namespaces", nil)
	w := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cluster.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute handler
	handlers.ListClusterNamespaces(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var namespaces []models.Namespace
	err := json.Unmarshal(w.Body.Bytes(), &namespaces)
	assert.NoError(t, err)
	assert.Len(t, namespaces, 2)
	assert.Equal(t, "default", namespaces[0].Name)
	assert.Equal(t, "production", namespaces[1].Name)
}

func TestGetClusterHealth(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Seed test data
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "devops",
		KubeconfigRef: "/etc/kubeconfigs/devops.yaml",
		Environment:   "prod",
		IsHealthy:     true,
	}
	db.Create(&cluster)

	// Mock HealthCheck to return healthy
	mockClientManager.On("HealthCheck", cluster.ID).Return(true, nil).Once()

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/clusters/"+cluster.ID.String()+"/health", nil)
	w := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cluster.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute handler
	handlers.GetClusterHealth(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["healthy"])
	assert.Equal(t, cluster.ID.String(), response["cluster_id"])

	mockClientManager.AssertExpectations(t)
}

func TestGetClusterHealth_Unhealthy(t *testing.T) {
	db := setupClusterTestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := NewClusterHandlers(db, mockClientManager)

	// Seed test data
	cluster := models.Cluster{
		ID:            uuid.New(),
		Name:          "devops",
		KubeconfigRef: "/etc/kubeconfigs/devops.yaml",
		Environment:   "prod",
		IsHealthy:     false,
	}
	db.Create(&cluster)

	// Mock HealthCheck to return error
	mockClientManager.On("HealthCheck", cluster.ID).Return(false, assert.AnError).Once()

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/clusters/"+cluster.ID.String()+"/health", nil)
	w := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cluster.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute handler
	handlers.GetClusterHealth(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, false, response["healthy"])
	assert.NotNil(t, response["error"])

	mockClientManager.AssertExpectations(t)
}
