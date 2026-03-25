package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/flux"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockFluxClient is a mock implementation of FluxClientInterface
type MockFluxClient struct {
	GetKustomizationStatusFunc func(name, namespace string) (*flux.KustomizationStatus, error)
	ListKustomizationsFunc     func(namespace string) ([]flux.KustomizationStatus, error)
}

func (m *MockFluxClient) GetKustomizationStatus(name, namespace string) (*flux.KustomizationStatus, error) {
	if m.GetKustomizationStatusFunc != nil {
		return m.GetKustomizationStatusFunc(name, namespace)
	}
	return nil, nil
}

func (m *MockFluxClient) ListKustomizations(namespace string) ([]flux.KustomizationStatus, error) {
	if m.ListKustomizationsFunc != nil {
		return m.ListKustomizationsFunc(namespace)
	}
	return nil, nil
}

func setupSyncTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	// Manually create tables without PostgreSQL-specific features
	err = db.Exec(`
		CREATE TABLE namespaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			cluster TEXT NOT NULL,
			environment TEXT NOT NULL CHECK(environment IN ('dev', 'staging', 'prod')),
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE secret_drafts (
			id TEXT PRIMARY KEY,
			secret_name TEXT NOT NULL,
			namespace_id TEXT NOT NULL,
			data TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('draft', 'published', 'drifted')),
			git_base_sha TEXT,
			edited_by TEXT,
			edited_at DATETIME,
			published_by TEXT,
			published_at DATETIME,
			commit_sha TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	return db
}

func TestGetSyncStatus_Success(t *testing.T) {
	db := setupSyncTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "development",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create published secrets
	secret1 := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON([]byte(`{"password": "secret123"}`)),
		Status:      "published",
		CommitSHA:   "abc123",
		EditedAt:    time.Now(),
	}
	secret2 := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "api-key",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON([]byte(`{"key": "mykey"}`)),
		Status:      "published",
		CommitSHA:   "def456",
		EditedAt:    time.Now(),
	}
	require.NoError(t, db.Create(&secret1).Error)
	require.NoError(t, db.Create(&secret2).Error)

	// Mock FluxCD client
	mockFlux := &MockFluxClient{
		GetKustomizationStatusFunc: func(name, namespace string) (*flux.KustomizationStatus, error) {
			return &flux.KustomizationStatus{
				Name:              "secrets-development",
				Namespace:         "flux-system",
				Ready:             true,
				LastAppliedCommit: "abc123",
				LastSyncTime:      time.Date(2026, 3, 22, 10, 30, 0, 0, time.UTC),
				Message:           "Applied revision: main@sha1:abc123",
			}, nil
		},
	}

	// Mock Git client
	mockGit := &MockGitClient{
		GetCurrentSHAFunc: func() (string, error) {
			return "abc123", nil
		},
	}

	// Create handler
	handler := NewSyncHandlers(db, mockFlux, mockGit)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/sync-status", nil)

	// Setup chi context with URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	handler.GetSyncStatus(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response NamespaceSyncStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "development", response.Namespace)
	assert.True(t, response.FluxReady)
	assert.Equal(t, "abc123", response.LastAppliedCommit)
	assert.NotEmpty(t, response.LastSyncTime)
	assert.Len(t, response.Secrets, 2)

	// Check new Phase 17 fields
	assert.Equal(t, "abc123", response.GitCommit)  // Git commit from mock
	assert.Equal(t, "abc123", response.FluxCommit) // Flux commit (alias of LastAppliedCommit)
	assert.True(t, response.Synced)                // Git and Flux commits match
	assert.NotEmpty(t, response.LastSync)          // Alias of LastSyncTime
	assert.Nil(t, response.Error)                  // No errors

	// Check secret sync info
	for _, secret := range response.Secrets {
		if secret.Name == "db-creds" {
			assert.Equal(t, "published", secret.Status)
			assert.Equal(t, "abc123", secret.CommitSHA)
			assert.True(t, secret.SyncedToK8s) // Commit matches
		} else if secret.Name == "api-key" {
			assert.Equal(t, "published", secret.Status)
			assert.Equal(t, "def456", secret.CommitSHA)
			assert.False(t, secret.SyncedToK8s) // Commit doesn't match
		}
	}
}

func TestGetSyncStatus_NamespaceNotFound(t *testing.T) {
	db := setupSyncTestDB(t)

	// Mock FluxCD client (won't be called)
	mockFlux := &MockFluxClient{}

	// Mock Git client
	mockGit := &MockGitClient{
		GetCurrentSHAFunc: func() (string, error) {
			return "abc123", nil
		},
	}

	// Create handler
	handler := NewSyncHandlers(db, mockFlux, mockGit)

	// Create request with non-existent namespace
	randomID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+randomID.String()+"/sync-status", nil)

	// Setup chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", randomID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	handler.GetSyncStatus(w, req)

	// Assert response
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Namespace not found")
}

func TestGetSyncStatus_InvalidNamespaceID(t *testing.T) {
	db := setupSyncTestDB(t)
	mockFlux := &MockFluxClient{}
	mockGit := &MockGitClient{
		GetCurrentSHAFunc: func() (string, error) {
			return "abc123", nil
		},
	}
	handler := NewSyncHandlers(db, mockFlux, mockGit)

	// Create request with invalid UUID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/invalid-uuid/sync-status", nil)

	// Setup chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	handler.GetSyncStatus(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid namespace ID")
}

func TestGetSyncStatus_FluxNotAvailable(t *testing.T) {
	db := setupSyncTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "staging",
		Cluster:     "test-cluster",
		Environment: "staging",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create handler with nil FluxCD client
	handler := NewSyncHandlers(db, nil, nil)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/sync-status", nil)

	// Setup chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	handler.GetSyncStatus(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response NamespaceSyncStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "staging", response.Namespace)
	assert.False(t, response.FluxReady) // FluxCD not available
	assert.Empty(t, response.LastAppliedCommit)
	assert.Empty(t, response.Secrets)
}

func TestGetSyncStatus_NoPublishedSecrets(t *testing.T) {
	db := setupSyncTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "production",
		Cluster:     "test-cluster",
		Environment: "prod",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create draft secret (not published)
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "draft-secret",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON([]byte(`{"key": "value"}`)),
		Status:      "draft",
		EditedAt:    time.Now(),
	}
	require.NoError(t, db.Create(&secret).Error)

	// Mock FluxCD client
	mockFlux := &MockFluxClient{
		GetKustomizationStatusFunc: func(name, namespace string) (*flux.KustomizationStatus, error) {
			return &flux.KustomizationStatus{
				Name:      "secrets-production",
				Namespace: "flux-system",
				Ready:     true,
			}, nil
		},
	}

	// Mock Git client
	mockGit := &MockGitClient{
		GetCurrentSHAFunc: func() (string, error) {
			return "abc123", nil
		},
	}

	// Create handler
	handler := NewSyncHandlers(db, mockFlux, mockGit)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/sync-status", nil)

	// Setup chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	handler.GetSyncStatus(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response NamespaceSyncStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "production", response.Namespace)
	assert.True(t, response.FluxReady)
	assert.Empty(t, response.Secrets) // No published secrets
}
