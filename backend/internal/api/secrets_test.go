package api

import (
	"bytes"
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
	"github.com/yourorg/secret-manager/internal/middleware"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockGitSync is a mock implementation of GitSyncInterface for testing
type MockGitSync struct {
	SyncSecretFunc        func(namespaceName, secretName string) error
	ReadSecretFromGitFunc func(namespaceName, secretName string) (map[string]string, error)
}

func (m *MockGitSync) SyncSecret(namespaceName, secretName string) error {
	if m.SyncSecretFunc != nil {
		return m.SyncSecretFunc(namespaceName, secretName)
	}
	return nil
}

func (m *MockGitSync) ReadSecretFromGit(namespaceName, secretName string) (map[string]string, error) {
	if m.ReadSecretFromGitFunc != nil {
		return m.ReadSecretFromGitFunc(namespaceName, secretName)
	}
	return nil, nil
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	// Manually create tables without PostgreSQL-specific features
	err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			azure_ad_oid TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			azure_ad_gid TEXT,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

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
		CREATE TABLE group_permissions (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL,
			namespace_id TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('viewer', 'editor', 'admin')),
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

	err = db.Exec(`
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			action_type TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_name TEXT NOT NULL,
			namespace_id TEXT,
			timestamp DATETIME,
			metadata TEXT,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE drift_events (
			id TEXT PRIMARY KEY,
			secret_name TEXT NOT NULL,
			namespace_id TEXT NOT NULL,
			detected_at DATETIME NOT NULL,
			resolved_at DATETIME,
			git_version TEXT,
			k8s_version TEXT,
			diff TEXT,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	return db
}

// createTestUser creates a test user and returns the user and context
func createTestUser(t *testing.T, db *gorm.DB) (*models.User, *middleware.UserContext) {
	user := &models.User{
		Email: "test@example.com",
		Name:  "Test User",
	}
	require.NoError(t, db.Create(user).Error)

	userCtx := &middleware.UserContext{
		UserID: user.ID,
		Email:  user.Email,
		Name:   user.Name,
	}

	return user, userCtx
}

// createTestNamespace creates a test namespace
func createTestNamespace(t *testing.T, db *gorm.DB) *models.Namespace {
	namespace := &models.Namespace{
		Name:        "test-namespace",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(namespace).Error)
	return namespace
}

// createTestSecret creates a test secret
func createTestSecret(t *testing.T, db *gorm.DB, namespaceID uuid.UUID, name string, status string, editedBy *uuid.UUID) *models.SecretDraft {
	data := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	dataJSON, err := json.Marshal(data)
	require.NoError(t, err)

	secret := &models.SecretDraft{
		SecretName:  name,
		NamespaceID: namespaceID,
		Data:        datatypes.JSON(dataJSON),
		Status:      status,
		EditedBy:    editedBy,
	}
	require.NoError(t, db.Create(secret).Error)
	return secret
}

// withUserContext adds user context to request
func withUserContext(r *http.Request, userCtx *middleware.UserContext) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, userCtx)
	return r.WithContext(ctx)
}

func TestCreateSecret(t *testing.T) {
	db := setupTestDB(t)
	handlers := NewSecretHandlers(db, nil)
	user, userCtx := createTestUser(t, db)
	namespace := createTestNamespace(t, db)

	t.Run("success - create draft secret", func(t *testing.T) {
		reqBody := CreateSecretRequest{
			Name: "my-secret",
			Data: map[string]interface{}{
				"username": "admin",
				"password": "secret123",
			},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		// Add chi URL params
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.CreateSecret(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Equal(t, "my-secret", resp.SecretName)
		assert.Equal(t, namespace.ID, resp.NamespaceID)
		assert.Equal(t, "draft", resp.Status)
		assert.Equal(t, user.ID, *resp.EditedBy)
	})

	t.Run("error - invalid namespace ID", func(t *testing.T) {
		reqBody := CreateSecretRequest{
			Name: "my-secret",
			Data: map[string]interface{}{"key": "value"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/invalid-uuid/secrets", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", "invalid-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.CreateSecret(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("error - namespace not found", func(t *testing.T) {
		nonExistentID := uuid.New()
		reqBody := CreateSecretRequest{
			Name: "my-secret",
			Data: map[string]interface{}{"key": "value"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+nonExistentID.String()+"/secrets", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", nonExistentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.CreateSecret(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("error - invalid secret name format", func(t *testing.T) {
		reqBody := CreateSecretRequest{
			Name: "Invalid_Name_With_Underscores",
			Data: map[string]interface{}{"key": "value"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.CreateSecret(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "DNS-1123")
	})

	t.Run("error - empty secret data", func(t *testing.T) {
		reqBody := CreateSecretRequest{
			Name: "my-secret",
			Data: map[string]interface{}{},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.CreateSecret(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "cannot be empty")
	})

	t.Run("error - data size exceeds 1MB", func(t *testing.T) {
		// Create a large data payload > 1MB
		largeData := make(map[string]interface{})
		largeString := string(make([]byte, 1024*1024+1)) // > 1MB
		largeData["large_key"] = largeString

		reqBody := CreateSecretRequest{
			Name: "large-secret",
			Data: largeData,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.CreateSecret(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "exceeds maximum")
	})

	t.Run("error - duplicate secret name", func(t *testing.T) {
		// Create first secret
		createTestSecret(t, db, namespace.ID, "duplicate-secret", "draft", &user.ID)

		// Try to create another with the same name
		reqBody := CreateSecretRequest{
			Name: "duplicate-secret",
			Data: map[string]interface{}{"key": "value"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.CreateSecret(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "already exists")
	})
}

func TestListSecrets(t *testing.T) {
	db := setupTestDB(t)
	handlers := NewSecretHandlers(db, nil)
	user, userCtx := createTestUser(t, db)
	namespace := createTestNamespace(t, db)

	// Create test secrets with different statuses
	createTestSecret(t, db, namespace.ID, "secret1", "draft", &user.ID)
	createTestSecret(t, db, namespace.ID, "secret2", "published", &user.ID)
	createTestSecret(t, db, namespace.ID, "secret3", "draft", &user.ID)

	t.Run("success - list all secrets", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.ListSecrets(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var secrets []models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&secrets)
		require.NoError(t, err)
		assert.Len(t, secrets, 3)
	})

	t.Run("success - filter by status draft", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets?status=draft", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.ListSecrets(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var secrets []models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&secrets)
		require.NoError(t, err)
		assert.Len(t, secrets, 2)
		for _, s := range secrets {
			assert.Equal(t, "draft", s.Status)
		}
	})

	t.Run("success - empty list", func(t *testing.T) {
		emptyNamespace := createTestNamespace(t, db)
		emptyNamespace.Name = "empty-namespace"
		db.Save(emptyNamespace)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+emptyNamespace.ID.String()+"/secrets", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", emptyNamespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.ListSecrets(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var secrets []models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&secrets)
		require.NoError(t, err)
		assert.Len(t, secrets, 0)
	})

	t.Run("error - invalid status filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets?status=invalid", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.ListSecrets(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("error - namespace not found", func(t *testing.T) {
		nonExistentID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+nonExistentID.String()+"/secrets", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", nonExistentID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.ListSecrets(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGetSecret(t *testing.T) {
	db := setupTestDB(t)
	handlers := NewSecretHandlers(db, nil)
	user, userCtx := createTestUser(t, db)
	namespace := createTestNamespace(t, db)
	secret := createTestSecret(t, db, namespace.ID, "my-secret", "draft", &user.ID)

	t.Run("success - get secret", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/my-secret", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "my-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.GetSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, secret.ID, resp.ID)
		assert.Equal(t, "my-secret", resp.SecretName)
	})

	t.Run("error - secret not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/nonexistent", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.GetSecret(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("error - invalid namespace ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/invalid-uuid/secrets/my-secret", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", "invalid-uuid")
		rctx.URLParams.Add("name", "my-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.GetSecret(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestUpdateSecret(t *testing.T) {
	db := setupTestDB(t)
	handlers := NewSecretHandlers(db, nil)
	user, userCtx := createTestUser(t, db)
	namespace := createTestNamespace(t, db)

	t.Run("success - update draft secret", func(t *testing.T) {
		secret := createTestSecret(t, db, namespace.ID, "updatable-secret", "draft", &user.ID)

		reqBody := UpdateSecretRequest{
			Data: map[string]interface{}{
				"newkey": "newvalue",
			},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/updatable-secret", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "updatable-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.UpdateSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, secret.ID, resp.ID)

		// Verify data was updated
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		assert.Equal(t, "newvalue", data["newkey"])
	})

	t.Run("error - update published secret", func(t *testing.T) {
		createTestSecret(t, db, namespace.ID, "published-secret", "published", &user.ID)

		reqBody := UpdateSecretRequest{
			Data: map[string]interface{}{"key": "value"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/published-secret", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "published-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.UpdateSecret(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "Only drafts")
	})

	t.Run("error - secret not found", func(t *testing.T) {
		reqBody := UpdateSecretRequest{
			Data: map[string]interface{}{"key": "value"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/nonexistent", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.UpdateSecret(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("error - empty data", func(t *testing.T) {
		createTestSecret(t, db, namespace.ID, "empty-data-secret", "draft", &user.ID)

		reqBody := UpdateSecretRequest{
			Data: map[string]interface{}{},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/empty-data-secret", bytes.NewReader(body))
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "empty-data-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.UpdateSecret(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "cannot be empty")
	})
}

func TestDeleteSecret(t *testing.T) {
	db := setupTestDB(t)
	handlers := NewSecretHandlers(db, nil)
	user, userCtx := createTestUser(t, db)
	namespace := createTestNamespace(t, db)

	t.Run("success - delete draft secret", func(t *testing.T) {
		secret := createTestSecret(t, db, namespace.ID, "deletable-secret", "draft", &user.ID)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/deletable-secret", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "deletable-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.DeleteSecret(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify secret was deleted
		var deleted models.SecretDraft
		err := db.First(&deleted, "id = ?", secret.ID).Error
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})

	t.Run("error - delete published secret", func(t *testing.T) {
		createTestSecret(t, db, namespace.ID, "published-secret-delete", "published", &user.ID)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/published-secret-delete", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "published-secret-delete")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.DeleteSecret(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "Cannot delete published secrets from UI")
	})

	t.Run("error - delete drifted secret without gitSync", func(t *testing.T) {
		handlers := NewSecretHandlers(db, nil) // No gitSync
		createTestSecret(t, db, namespace.ID, "drifted-secret-delete", "drifted", &user.ID)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/drifted-secret-delete", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "drifted-secret-delete")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.DeleteSecret(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "Git sync service is not available")
	})

	t.Run("success - reset drifted secret to Git state", func(t *testing.T) {
		// Create a drifted secret
		secret := createTestSecret(t, db, namespace.ID, "drifted-secret-reset", "drifted", &user.ID)

		// Create mock gitSync that resets the secret to published
		mockGitSync := &MockGitSync{
			SyncSecretFunc: func(namespaceName, secretName string) error {
				// Simulate resetting the secret to published state
				assert.Equal(t, namespace.Name, namespaceName)
				assert.Equal(t, "drifted-secret-reset", secretName)

				// Update the secret to published status in DB
				db.Model(&models.SecretDraft{}).
					Where("id = ?", secret.ID).
					Updates(map[string]interface{}{
						"status":     "published",
						"commit_sha": "abc123",
					})
				return nil
			},
		}

		handlers := NewSecretHandlers(db, mockGitSync)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/drifted-secret-reset", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "drifted-secret-reset")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.DeleteSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]string
		json.NewDecoder(w.Body).Decode(&resp)
		assert.Equal(t, "Secret reset to Git state", resp["message"])
		assert.Equal(t, "published", resp["status"])

		// Verify secret is now published
		var updatedSecret models.SecretDraft
		db.First(&updatedSecret, secret.ID)
		assert.Equal(t, "published", updatedSecret.Status)
	})

	t.Run("error - reset drifted secret fails", func(t *testing.T) {
		createTestSecret(t, db, namespace.ID, "drifted-secret-fail", "drifted", &user.ID)

		// Create mock gitSync that returns an error
		mockGitSync := &MockGitSync{
			SyncSecretFunc: func(namespaceName, secretName string) error {
				return fmt.Errorf("failed to read from Git")
			},
		}

		handlers := NewSecretHandlers(db, mockGitSync)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/drifted-secret-fail", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "drifted-secret-fail")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.DeleteSecret(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "Failed to reset secret from Git")
	})

	t.Run("error - secret not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/nonexistent", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.DeleteSecret(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("error - invalid namespace ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces/invalid-uuid/secrets/my-secret", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", "invalid-uuid")
		rctx.URLParams.Add("name", "my-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.DeleteSecret(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestValidateSecretRequest(t *testing.T) {
	t.Run("valid DNS-1123 names", func(t *testing.T) {
		validNames := []string{
			"my-secret",
			"secret123",
			"s",
			"my.secret.name",
			"secret-with-many-parts",
		}

		for _, name := range validNames {
			err := validateSecretRequest(name, map[string]interface{}{"key": "value"})
			assert.NoError(t, err, "name %s should be valid", name)
		}
	})

	t.Run("invalid DNS-1123 names", func(t *testing.T) {
		invalidNames := []string{
			"",
			"Secret",      // uppercase
			"secret_name", // underscore
			"secret name", // space
			"-secret",     // starts with hyphen
			"secret-",     // ends with hyphen
			".secret",     // starts with dot
			"secret.",     // ends with dot
		}

		for _, name := range invalidNames {
			err := validateSecretRequest(name, map[string]interface{}{"key": "value"})
			assert.Error(t, err, "name '%s' should be invalid", name)
		}
	})

	t.Run("name too long", func(t *testing.T) {
		longName := string(make([]byte, 254))
		err := validateSecretRequest(longName, map[string]interface{}{"key": "value"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "253 characters")
	})

	t.Run("empty data", func(t *testing.T) {
		err := validateSecretRequest("valid-name", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("nil data", func(t *testing.T) {
		err := validateSecretRequest("valid-name", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})
}

func TestGetSecretWithGitVersion(t *testing.T) {
	db := setupTestDB(t)

	// Create test user
	user, userCtx := createTestUser(t, db)

	// Create test namespace
	namespaceID := uuid.New()
	namespace := models.Namespace{
		ID:          namespaceID,
		Name:        "test-namespace",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create a published secret (exists in Git)
	publishedData := map[string]interface{}{"db_password": "secret123"}
	dataJSON, _ := json.Marshal(publishedData)
	publishedSecret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "published-secret",
		NamespaceID: namespaceID,
		Data:        datatypes.JSON(dataJSON),
		Status:      "published",
		CommitSHA:   "abc123",
		GitBaseSHA:  "abc123",
		EditedBy:    &user.ID,
	}
	require.NoError(t, db.Create(&publishedSecret).Error)

	// Create a draft secret (doesn't exist in Git)
	draftData := map[string]interface{}{"api_key": "draft456"}
	draftJSON, _ := json.Marshal(draftData)
	draftSecret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "draft-secret",
		NamespaceID: namespaceID,
		Data:        datatypes.JSON(draftJSON),
		Status:      "draft",
		EditedBy:    &user.ID,
	}
	require.NoError(t, db.Create(&draftSecret).Error)

	// Create a drifted secret (exists in Git but modified locally)
	driftedData := map[string]interface{}{"token": "modified789"}
	driftedJSON, _ := json.Marshal(driftedData)
	driftedSecret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "drifted-secret",
		NamespaceID: namespaceID,
		Data:        datatypes.JSON(driftedJSON),
		Status:      "drifted",
		CommitSHA:   "def456",
		GitBaseSHA:  "def456",
		EditedBy:    &user.ID,
	}
	require.NoError(t, db.Create(&driftedSecret).Error)

	t.Run("without include_git_version parameter", func(t *testing.T) {
		mockGitSync := &MockGitSync{}
		handlers := NewSecretHandlers(db, mockGitSync)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespaceID.String()+"/secrets/published-secret", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespaceID.String())
		rctx.URLParams.Add("name", "published-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = withUserContext(req, userCtx)

		w := httptest.NewRecorder()
		handlers.GetSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should NOT include git_data
		_, hasGitData := response["git_data"]
		assert.False(t, hasGitData, "git_data should not be present when include_git_version is not set")
	})

	t.Run("with include_git_version=true for published secret", func(t *testing.T) {
		gitData := map[string]string{
			"db_password": "oldpassword",
			"db_username": "admin",
		}

		mockGitSync := &MockGitSync{
			ReadSecretFromGitFunc: func(namespaceName, secretName string) (map[string]string, error) {
				assert.Equal(t, "test-namespace", namespaceName)
				assert.Equal(t, "published-secret", secretName)
				return gitData, nil
			},
		}
		handlers := NewSecretHandlers(db, mockGitSync)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespaceID.String()+"/secrets/published-secret?include_git_version=true", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespaceID.String())
		rctx.URLParams.Add("name", "published-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = withUserContext(req, userCtx)

		w := httptest.NewRecorder()
		handlers.GetSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should include git_data
		actualGitData, hasGitData := response["git_data"]
		assert.True(t, hasGitData, "git_data should be present when include_git_version=true for published secret")

		if hasGitData {
			gitDataMap := actualGitData.(map[string]interface{})
			assert.Equal(t, "oldpassword", gitDataMap["db_password"])
			assert.Equal(t, "admin", gitDataMap["db_username"])
		}
	})

	t.Run("with include_git_version=true for drifted secret", func(t *testing.T) {
		gitData := map[string]string{
			"token": "original-git-token",
		}

		mockGitSync := &MockGitSync{
			ReadSecretFromGitFunc: func(namespaceName, secretName string) (map[string]string, error) {
				assert.Equal(t, "test-namespace", namespaceName)
				assert.Equal(t, "drifted-secret", secretName)
				return gitData, nil
			},
		}
		handlers := NewSecretHandlers(db, mockGitSync)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespaceID.String()+"/secrets/drifted-secret?include_git_version=true", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespaceID.String())
		rctx.URLParams.Add("name", "drifted-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = withUserContext(req, userCtx)

		w := httptest.NewRecorder()
		handlers.GetSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should include git_data for drifted secrets
		actualGitData, hasGitData := response["git_data"]
		assert.True(t, hasGitData, "git_data should be present when include_git_version=true for drifted secret")

		if hasGitData {
			gitDataMap := actualGitData.(map[string]interface{})
			assert.Equal(t, "original-git-token", gitDataMap["token"])
		}
	})

	t.Run("with include_git_version=true for draft secret", func(t *testing.T) {
		mockGitSync := &MockGitSync{
			ReadSecretFromGitFunc: func(namespaceName, secretName string) (map[string]string, error) {
				t.Fatal("ReadSecretFromGit should not be called for draft secrets")
				return nil, nil
			},
		}
		handlers := NewSecretHandlers(db, mockGitSync)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespaceID.String()+"/secrets/draft-secret?include_git_version=true", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespaceID.String())
		rctx.URLParams.Add("name", "draft-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = withUserContext(req, userCtx)

		w := httptest.NewRecorder()
		handlers.GetSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should NOT include git_data for draft secrets
		_, hasGitData := response["git_data"]
		assert.False(t, hasGitData, "git_data should not be present for draft secrets even with include_git_version=true")
	})

	t.Run("handles Git read errors gracefully", func(t *testing.T) {
		mockGitSync := &MockGitSync{
			ReadSecretFromGitFunc: func(namespaceName, secretName string) (map[string]string, error) {
				return nil, fmt.Errorf("Git repository unavailable")
			},
		}
		handlers := NewSecretHandlers(db, mockGitSync)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+namespaceID.String()+"/secrets/published-secret?include_git_version=true", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespaceID.String())
		rctx.URLParams.Add("name", "published-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = withUserContext(req, userCtx)

		w := httptest.NewRecorder()
		handlers.GetSecret(w, req)

		// Should succeed (200) even if Git read fails
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should NOT include git_data when Git read fails
		_, hasGitData := response["git_data"]
		assert.False(t, hasGitData, "git_data should not be present when Git read fails")
	})
}
