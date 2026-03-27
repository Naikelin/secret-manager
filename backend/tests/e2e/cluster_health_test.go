package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/api"
	"github.com/yourorg/secret-manager/internal/models"
)

// TestClusterHealthCheckE2E tests the cluster health check endpoint and unhealthy cluster handling
func TestClusterHealthCheckE2E(t *testing.T) {
	// Setup: Create in-memory test environment
	db := setupE2ETestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := api.NewClusterHandlers(db, mockClientManager)

	// Step 1: Create a cluster (initially healthy)
	cluster := models.Cluster{
		Name:          "health-test-cluster",
		KubeconfigRef: "kubeconfigs/health-test.yaml",
		Environment:   "development",
		IsHealthy:     true,
	}
	err := db.Create(&cluster).Error
	require.NoError(t, err, "Cluster should be created in DB")

	// Step 2: Test Health Check Endpoint - Healthy Cluster
	// Mock HealthCheck to return success
	mockClientManager.On("HealthCheck", cluster.ID).Return(true, nil).Once()

	// Create request with cluster ID in URL
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/clusters/%s/health", cluster.ID), nil)

	// Setup chi context with URL parameter
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cluster.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handlers.GetClusterHealth(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "Health check should return 200 OK")

	var healthResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &healthResponse)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, cluster.ID.String(), healthResponse["cluster_id"], "Cluster ID should match")
	assert.Equal(t, true, healthResponse["healthy"], "Cluster should be healthy")
	assert.Nil(t, healthResponse["error"], "No error should be present for healthy cluster")

	t.Log("✅ Healthy cluster health check passed")

	// Step 3: Test Unhealthy Cluster Behavior
	// Mock HealthCheck to return connection error
	connectionErr := fmt.Errorf("connection refused: unable to reach API server")
	mockClientManager.On("HealthCheck", cluster.ID).Return(false, connectionErr).Once()

	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/clusters/%s/health", cluster.ID), nil)
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", cluster.ID.String())
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	w2 := httptest.NewRecorder()
	handlers.GetClusterHealth(w2, req2)

	// Verify unhealthy response is handled gracefully (200 OK with error details)
	assert.Equal(t, http.StatusOK, w2.Code, "Health check should return 200 OK even for unhealthy cluster")

	var unhealthyResponse map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &unhealthyResponse)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, cluster.ID.String(), unhealthyResponse["cluster_id"], "Cluster ID should match")
	assert.Equal(t, false, unhealthyResponse["healthy"], "Cluster should be marked unhealthy")
	assert.NotNil(t, unhealthyResponse["error"], "Error should be present for unhealthy cluster")
	assert.Contains(t, unhealthyResponse["error"].(string), "connection refused", "Error message should describe connection failure")

	t.Log("✅ Unhealthy cluster health check passed - error handled gracefully")

	// Step 4: Test Recovery - Cluster becomes healthy again
	// Mock HealthCheck to succeed after recovery
	mockClientManager.On("HealthCheck", cluster.ID).Return(true, nil).Once()

	req3 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/clusters/%s/health", cluster.ID), nil)
	rctx3 := chi.NewRouteContext()
	rctx3.URLParams.Add("id", cluster.ID.String())
	req3 = req3.WithContext(context.WithValue(req3.Context(), chi.RouteCtxKey, rctx3))

	w3 := httptest.NewRecorder()
	handlers.GetClusterHealth(w3, req3)

	// Verify cluster recovered
	assert.Equal(t, http.StatusOK, w3.Code, "Health check should return 200 OK")

	var recoveredResponse map[string]interface{}
	err = json.Unmarshal(w3.Body.Bytes(), &recoveredResponse)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, cluster.ID.String(), recoveredResponse["cluster_id"], "Cluster ID should match")
	assert.Equal(t, true, recoveredResponse["healthy"], "Cluster should be healthy after recovery")
	assert.Nil(t, recoveredResponse["error"], "No error should be present after recovery")

	t.Log("✅ Cluster recovery verified - cluster became healthy again")

	// Verify all mock expectations were met
	mockClientManager.AssertExpectations(t)

	t.Log("✅ E2E cluster health check and recovery test passed")
}

// TestClusterHealthCheckE2E_ClusterNotFound tests health check for non-existent cluster
func TestClusterHealthCheckE2E_ClusterNotFound(t *testing.T) {
	db := setupE2ETestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := api.NewClusterHandlers(db, mockClientManager)

	// Use a random UUID that doesn't exist in DB
	nonExistentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/clusters/%s/health", nonExistentID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", nonExistentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handlers.GetClusterHealth(w, req)

	// Verify 404 Not Found response
	assert.Equal(t, http.StatusNotFound, w.Code, "Non-existent cluster should return 404")
	assert.Contains(t, w.Body.String(), "Cluster not found", "Error message should indicate cluster not found")

	t.Log("✅ Health check for non-existent cluster handled correctly")
}

// TestClusterHealthCheckE2E_InvalidClusterID tests health check with invalid UUID format
func TestClusterHealthCheckE2E_InvalidClusterID(t *testing.T) {
	db := setupE2ETestDB(t)
	mockClientManager := new(MockClientManager)
	handlers := api.NewClusterHandlers(db, mockClientManager)

	// Test with invalid UUID format
	invalidID := "not-a-valid-uuid"

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/clusters/%s/health", invalidID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", invalidID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handlers.GetClusterHealth(w, req)

	// Verify 400 Bad Request response
	assert.Equal(t, http.StatusBadRequest, w.Code, "Invalid UUID should return 400")
	assert.Contains(t, w.Body.String(), "Invalid cluster ID format", "Error message should indicate invalid UUID")

	t.Log("✅ Health check with invalid UUID handled correctly")
}
