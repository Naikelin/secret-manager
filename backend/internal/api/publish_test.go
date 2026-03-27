package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gopkg.in/yaml.v3"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func init() {
	// Initialize logger for tests
	logger.Init("error") // Use error level to reduce noise in tests
}

// MockGitClient is a mock implementation of GitClient for testing
type MockGitClient struct {
	EnsureRepoFunc           func() error
	WriteFileFunc            func(path string, content []byte) error
	ReadFileFunc             func(path string) ([]byte, error)
	CommitFunc               func(message, authorName string, files []string) (string, error)
	PushFunc                 func() error
	FileExistsFunc           func(path string) (bool, error)
	GetFilePathFunc          func(clusterName, namespace, secretName string) string
	GetFilePathLegacyFunc    func(namespace, secretName string) string
	ReadFileWithFallbackFunc func(clusterName, namespace, secretName string) ([]byte, string, error)
	RepoPathFunc             func() string
	GetCurrentSHAFunc        func() (string, error)
}

func (m *MockGitClient) EnsureRepo() error {
	if m.EnsureRepoFunc != nil {
		return m.EnsureRepoFunc()
	}
	return nil
}

func (m *MockGitClient) WriteFile(path string, content []byte) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(path, content)
	}
	return nil
}

func (m *MockGitClient) ReadFile(path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(path)
	}
	return nil, fmt.Errorf("file not found")
}

func (m *MockGitClient) Commit(message, authorName string, files []string) (string, error) {
	if m.CommitFunc != nil {
		return m.CommitFunc(message, authorName, files)
	}
	return "abc123def456", nil
}

func (m *MockGitClient) Push() error {
	if m.PushFunc != nil {
		return m.PushFunc()
	}
	return nil
}

func (m *MockGitClient) FileExists(path string) (bool, error) {
	if m.FileExistsFunc != nil {
		return m.FileExistsFunc(path)
	}
	return true, nil
}

func (m *MockGitClient) GetFilePath(clusterName, namespace, secretName string) string {
	if m.GetFilePathFunc != nil {
		return m.GetFilePathFunc(clusterName, namespace, secretName)
	}
	return fmt.Sprintf("clusters/%s/namespaces/%s/secrets/%s.yaml", clusterName, namespace, secretName)
}

func (m *MockGitClient) GetFilePathLegacy(namespace, secretName string) string {
	if m.GetFilePathLegacyFunc != nil {
		return m.GetFilePathLegacyFunc(namespace, secretName)
	}
	return fmt.Sprintf("namespaces/%s/secrets/%s.yaml", namespace, secretName)
}

func (m *MockGitClient) ReadFileWithFallback(clusterName, namespace, secretName string) ([]byte, string, error) {
	if m.ReadFileWithFallbackFunc != nil {
		return m.ReadFileWithFallbackFunc(clusterName, namespace, secretName)
	}
	// Default: try new path first
	newPath := m.GetFilePath(clusterName, namespace, secretName)
	content, err := m.ReadFile(newPath)
	if err == nil {
		return content, newPath, nil
	}
	legacyPath := m.GetFilePathLegacy(namespace, secretName)
	content, err = m.ReadFile(legacyPath)
	if err == nil {
		return content, legacyPath, nil
	}
	return nil, "", fmt.Errorf("file not found in either path")
}

func (m *MockGitClient) RepoPath() string {
	if m.RepoPathFunc != nil {
		return m.RepoPathFunc()
	}
	return "/tmp/test-repo"
}

func (m *MockGitClient) GetCurrentSHA() (string, error) {
	if m.GetCurrentSHAFunc != nil {
		return m.GetCurrentSHAFunc()
	}
	return "abc123def456", nil // Default SHA
}

// MockSOPSClient is a mock implementation of SOPSClient for testing
type MockSOPSClient struct {
	EncryptYAMLFunc func(yamlContent []byte) ([]byte, error)
	DecryptYAMLFunc func(encryptedYAML []byte) ([]byte, error)
}

func (m *MockSOPSClient) EncryptYAML(yamlContent []byte) ([]byte, error) {
	if m.EncryptYAMLFunc != nil {
		return m.EncryptYAMLFunc(yamlContent)
	}
	// Return a mock encrypted YAML
	return []byte("# Mock encrypted YAML\n" + string(yamlContent)), nil
}

func (m *MockSOPSClient) DecryptYAML(encryptedYAML []byte) ([]byte, error) {
	if m.EncryptYAMLFunc != nil {
		return m.DecryptYAMLFunc(encryptedYAML)
	}
	return encryptedYAML, nil
}

// createPublishHandlersForTest creates PublishHandlers with mock clients for testing
func createPublishHandlersForTest(db *gorm.DB, gitClient *MockGitClient, sopsClient *MockSOPSClient) *PublishHandlers {
	return &PublishHandlers{
		db:         db,
		gitClient:  gitClient,
		sopsClient: sopsClient,
	}
}

func TestPublishSecret(t *testing.T) {
	t.Run("success - publish draft secret", func(t *testing.T) {
		db := setupTestDB(t)
		user, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)
		_ = createTestSecret(t, db, namespace.ID, "my-secret", "draft", &user.ID)

		// Mock Git and SOPS clients
		gitClient := &MockGitClient{}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/my-secret/publish", nil)
		req = withUserContext(req, userCtx)

		// Add chi URL params
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "my-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.PublishSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Equal(t, "published", resp.Status)
		assert.NotNil(t, resp.PublishedBy)
		assert.NotNil(t, resp.PublishedAt)
		assert.NotEmpty(t, resp.CommitSHA)
		assert.Equal(t, user.ID, *resp.PublishedBy)

		// Verify audit log was created
		var auditLog models.AuditLog
		err = db.Where("action_type = ? AND resource_name = ?", "publish_secret", "my-secret").First(&auditLog).Error
		assert.NoError(t, err)
	})

	t.Run("error - secret not found", func(t *testing.T) {
		db := setupTestDB(t)
		_, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)

		gitClient := &MockGitClient{}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/nonexistent/publish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.PublishSecret(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("error - secret already published", func(t *testing.T) {
		db := setupTestDB(t)
		user, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)
		secret := createTestSecret(t, db, namespace.ID, "published-secret", "published", &user.ID)

		gitClient := &MockGitClient{}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/published-secret/publish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "published-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.PublishSecret(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		// Verify status wasn't changed
		var updatedSecret models.SecretDraft
		err := db.First(&updatedSecret, secret.ID).Error
		require.NoError(t, err)
		assert.Equal(t, "published", updatedSecret.Status)
	})

	t.Run("error - invalid namespace ID", func(t *testing.T) {
		db := setupTestDB(t)
		_, userCtx := createTestUser(t, db)

		gitClient := &MockGitClient{}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/invalid-uuid/secrets/test/publish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", "invalid-uuid")
		rctx.URLParams.Add("name", "test")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.PublishSecret(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("error - Git EnsureRepo fails", func(t *testing.T) {
		db := setupTestDB(t)
		user, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)
		createTestSecret(t, db, namespace.ID, "my-secret", "draft", &user.ID)

		gitClient := &MockGitClient{
			EnsureRepoFunc: func() error {
				return fmt.Errorf("git clone failed")
			},
		}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/my-secret/publish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "my-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.PublishSecret(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("error - SOPS encryption fails", func(t *testing.T) {
		db := setupTestDB(t)
		user, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)
		createTestSecret(t, db, namespace.ID, "my-secret", "draft", &user.ID)

		gitClient := &MockGitClient{}
		sopsClient := &MockSOPSClient{
			EncryptYAMLFunc: func(yamlContent []byte) ([]byte, error) {
				return nil, fmt.Errorf("encryption failed")
			},
		}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/my-secret/publish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "my-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.PublishSecret(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestUnpublishSecret(t *testing.T) {
	t.Run("success - unpublish published secret", func(t *testing.T) {
		db := setupTestDB(t)
		user, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)

		// Create a published secret
		secret := createTestSecret(t, db, namespace.ID, "published-secret", "published", &user.ID)
		now := time.Now()
		secret.PublishedBy = &user.ID
		secret.PublishedAt = &now
		secret.CommitSHA = "abc123"
		db.Save(&secret)

		// Mock Git and SOPS clients - file doesn't exist in mock repo
		gitClient := &MockGitClient{
			FileExistsFunc: func(path string) (bool, error) {
				return false, nil // File doesn't exist, so skip deletion
			},
		}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/published-secret/unpublish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "published-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.UnpublishSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Equal(t, "draft", resp.Status)
		assert.Nil(t, resp.PublishedBy)
		assert.Nil(t, resp.PublishedAt)
		assert.Empty(t, resp.CommitSHA)

		// Verify audit log was created
		var auditLog models.AuditLog
		err = db.Where("action_type = ? AND resource_name = ?", "unpublish_secret", "published-secret").First(&auditLog).Error
		assert.NoError(t, err)
	})

	t.Run("error - secret not found", func(t *testing.T) {
		db := setupTestDB(t)
		_, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)

		gitClient := &MockGitClient{}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/nonexistent/unpublish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.UnpublishSecret(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("error - secret not published", func(t *testing.T) {
		db := setupTestDB(t)
		user, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)
		createTestSecret(t, db, namespace.ID, "draft-secret", "draft", &user.ID)

		gitClient := &MockGitClient{}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/draft-secret/unpublish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "draft-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.UnpublishSecret(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("success - unpublish when file doesn't exist in Git", func(t *testing.T) {
		db := setupTestDB(t)
		user, userCtx := createTestUser(t, db)
		namespace := createTestNamespace(t, db)

		secret := createTestSecret(t, db, namespace.ID, "published-secret", "published", &user.ID)
		now := time.Now()
		secret.PublishedBy = &user.ID
		secret.PublishedAt = &now
		secret.CommitSHA = "abc123"
		db.Save(&secret)

		gitClient := &MockGitClient{
			FileExistsFunc: func(path string) (bool, error) {
				return false, nil // File doesn't exist
			},
		}
		sopsClient := &MockSOPSClient{}
		handlers := createPublishHandlersForTest(db, gitClient, sopsClient)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/"+namespace.ID.String()+"/secrets/published-secret/unpublish", nil)
		req = withUserContext(req, userCtx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("namespace", namespace.ID.String())
		rctx.URLParams.Add("name", "published-secret")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handlers.UnpublishSecret(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.SecretDraft
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "draft", resp.Status)
	})
}

func TestConvertToK8sSecret(t *testing.T) {
	t.Run("success - convert secret with valid data", func(t *testing.T) {
		data := map[string]interface{}{
			"username": "admin",
			"password": "secret123",
		}
		dataJSON, _ := json.Marshal(data)

		secret := &models.SecretDraft{
			SecretName: "my-secret",
			Data:       datatypes.JSON(dataJSON),
		}

		yamlBytes, err := convertToK8sSecret(secret, "test-namespace")
		require.NoError(t, err)
		assert.NotEmpty(t, yamlBytes)

		// Parse YAML to verify structure
		var k8sSecret K8sSecret
		err = yaml.Unmarshal(yamlBytes, &k8sSecret)
		require.NoError(t, err)

		assert.Equal(t, "v1", k8sSecret.APIVersion)
		assert.Equal(t, "Secret", k8sSecret.Kind)
		assert.Equal(t, "my-secret", k8sSecret.Metadata.Name)
		assert.Equal(t, "test-namespace", k8sSecret.Metadata.Namespace)
		assert.Equal(t, "Opaque", k8sSecret.Type)
		assert.Contains(t, k8sSecret.Data, "username")
		assert.Contains(t, k8sSecret.Data, "password")

		// Verify values are base64 encoded
		assert.NotEqual(t, "admin", k8sSecret.Data["username"])
		assert.NotEqual(t, "secret123", k8sSecret.Data["password"])
	})

	t.Run("success - convert secret with empty data", func(t *testing.T) {
		data := map[string]interface{}{}
		dataJSON, _ := json.Marshal(data)

		secret := &models.SecretDraft{
			SecretName: "empty-secret",
			Data:       datatypes.JSON(dataJSON),
		}

		yamlBytes, err := convertToK8sSecret(secret, "test-namespace")
		require.NoError(t, err)

		var k8sSecret K8sSecret
		err = yaml.Unmarshal(yamlBytes, &k8sSecret)
		require.NoError(t, err)

		assert.Equal(t, 0, len(k8sSecret.Data))
	})

	t.Run("error - invalid JSON data", func(t *testing.T) {
		secret := &models.SecretDraft{
			SecretName: "invalid-secret",
			Data:       datatypes.JSON([]byte("invalid json")),
		}

		_, err := convertToK8sSecret(secret, "test-namespace")
		assert.Error(t, err)
	})
}

func TestCreateAuditLog(t *testing.T) {
	t.Run("success - create audit log", func(t *testing.T) {
		db := setupTestDB(t)

		userID := uuid.New()
		namespaceID := uuid.New()
		secretID := uuid.New()

		err := createAuditLog(db, userID, namespaceID, secretID, "publish_secret", map[string]interface{}{
			"commit_sha":  "abc123",
			"namespace":   "test-namespace",
			"secret_name": "my-secret",
		})
		require.NoError(t, err)

		// Verify audit log was created
		var auditLog models.AuditLog
		err = db.Where("action_type = ?", "publish_secret").First(&auditLog).Error
		require.NoError(t, err)

		assert.Equal(t, "publish_secret", auditLog.ActionType)
		assert.Equal(t, "secret", auditLog.ResourceType)
		assert.Equal(t, "my-secret", auditLog.ResourceName)
		assert.Equal(t, userID, *auditLog.UserID)
		assert.Equal(t, namespaceID, *auditLog.NamespaceID)
	})
}
