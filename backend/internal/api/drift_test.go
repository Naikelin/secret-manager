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
	"github.com/yourorg/secret-manager/internal/config"
	"github.com/yourorg/secret-manager/internal/drift"
	"github.com/yourorg/secret-manager/internal/flux"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mock types for drift testing
type driftMockGitClient struct {
	readFileFunc             func(path string) ([]byte, error)
	writeFileFunc            func(path string, content []byte) error
	commitFunc               func(message, authorName string, files []string) (string, error)
	pushFunc                 func() error
	ensureRepoFunc           func() error
	getFilePathFunc          func(clusterName, namespace, secretName string) string
	getFilePathLegacyFunc    func(namespace, secretName string) string
	readFileWithFallbackFunc func(clusterName, namespace, secretName string) ([]byte, string, error)
}

func (m *driftMockGitClient) ReadFile(path string) ([]byte, error) {
	if m.readFileFunc != nil {
		return m.readFileFunc(path)
	}
	return nil, fmt.Errorf("file not found")
}

func (m *driftMockGitClient) WriteFile(path string, content []byte) error {
	if m.writeFileFunc != nil {
		return m.writeFileFunc(path, content)
	}
	return nil
}

func (m *driftMockGitClient) Commit(message, authorName string, files []string) (string, error) {
	if m.commitFunc != nil {
		return m.commitFunc(message, authorName, files)
	}
	return "mock-commit-sha", nil
}

func (m *driftMockGitClient) Push() error {
	if m.pushFunc != nil {
		return m.pushFunc()
	}
	return nil
}

func (m *driftMockGitClient) EnsureRepo() error {
	if m.ensureRepoFunc != nil {
		return m.ensureRepoFunc()
	}
	return nil
}

func (m *driftMockGitClient) GetFilePath(clusterName, namespace, secretName string) string {
	if m.getFilePathFunc != nil {
		return m.getFilePathFunc(clusterName, namespace, secretName)
	}
	return fmt.Sprintf("clusters/%s/namespaces/%s/secrets/%s.yaml", clusterName, namespace, secretName)
}

func (m *driftMockGitClient) GetFilePathLegacy(namespace, secretName string) string {
	if m.getFilePathLegacyFunc != nil {
		return m.getFilePathLegacyFunc(namespace, secretName)
	}
	return fmt.Sprintf("namespaces/%s/secrets/%s.yaml", namespace, secretName)
}

func (m *driftMockGitClient) ReadFileWithFallback(clusterName, namespace, secretName string) ([]byte, string, error) {
	if m.readFileWithFallbackFunc != nil {
		return m.readFileWithFallbackFunc(clusterName, namespace, secretName)
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

type driftMockSOPSClient struct {
	decryptYAMLFunc func(encryptedYAML []byte) ([]byte, error)
	encryptYAMLFunc func(yamlContent []byte) ([]byte, error)
}

func (m *driftMockSOPSClient) DecryptYAML(encryptedYAML []byte) ([]byte, error) {
	if m.decryptYAMLFunc != nil {
		return m.decryptYAMLFunc(encryptedYAML)
	}
	return encryptedYAML, nil
}

func (m *driftMockSOPSClient) EncryptYAML(yamlContent []byte) ([]byte, error) {
	if m.encryptYAMLFunc != nil {
		return m.encryptYAMLFunc(yamlContent)
	}
	return yamlContent, nil
}

type driftMockK8sClient struct {
	getSecretFunc   func(namespace, name string) (*corev1.Secret, error)
	applySecretFunc func(ctx context.Context, namespace string, secret *corev1.Secret) error
}

func (m *driftMockK8sClient) GetSecret(namespace, name string) (*corev1.Secret, error) {
	if m.getSecretFunc != nil {
		return m.getSecretFunc(namespace, name)
	}
	return nil, fmt.Errorf("secret not found")
}

// setupDriftTestDB creates an in-memory database for drift testing
func setupDriftTestDB(t *testing.T) *gorm.DB {
	// Reuse the existing setupTestDB from secrets_test.go which has proper SQLite table creation
	return setupTestDB(t)
}

// driftMockFluxClient for testing
type driftMockFluxClient struct {
	triggerKustomizationFunc   func(ctx context.Context, name, namespace string) error
	triggerGitRepositoryFunc   func(ctx context.Context, name, namespace string) error
	waitForReconciliationFunc  func(ctx context.Context, name, namespace string, timeout, pollInterval time.Duration) error
	getKustomizationStatusFunc func(name, namespace string) (*flux.KustomizationStatus, error)
}

func (m *driftMockFluxClient) TriggerKustomizationReconciliation(ctx context.Context, name, namespace string) error {
	if m.triggerKustomizationFunc != nil {
		return m.triggerKustomizationFunc(ctx, name, namespace)
	}
	return nil
}

func (m *driftMockFluxClient) TriggerGitRepositoryReconciliation(ctx context.Context, name, namespace string) error {
	if m.triggerGitRepositoryFunc != nil {
		return m.triggerGitRepositoryFunc(ctx, name, namespace)
	}
	return nil
}

func (m *driftMockFluxClient) WaitForKustomizationReconciliation(ctx context.Context, name, namespace string, timeout, pollInterval time.Duration) error {
	if m.waitForReconciliationFunc != nil {
		return m.waitForReconciliationFunc(ctx, name, namespace, timeout, pollInterval)
	}
	return nil
}

func (m *driftMockFluxClient) GetKustomizationStatus(name, namespace string) (*flux.KustomizationStatus, error) {
	if m.getKustomizationStatusFunc != nil {
		return m.getKustomizationStatusFunc(name, namespace)
	}
	return &flux.KustomizationStatus{Ready: true}, nil
}

// getTestConfigForAPI returns a config with sensible test defaults
func getTestConfigForAPI() *config.Config {
	return &config.Config{
		FluxKustomizationName: "secrets",
		FluxKustomizationNS:   "flux-system",
		FluxGitRepositoryName: "secrets-repo",
		FluxReconcileTimeout:  2 * time.Minute,
		FluxPollInterval:      2 * time.Second,
	}
}

// getTestFluxClientForAPI returns a mock FluxClient that succeeds by default
func getTestFluxClientForAPI() drift.FluxClientInterface {
	return &driftMockFluxClient{
		triggerKustomizationFunc: func(ctx context.Context, name, namespace string) error {
			return nil
		},
		triggerGitRepositoryFunc: func(ctx context.Context, name, namespace string) error {
			return nil
		},
		waitForReconciliationFunc: func(ctx context.Context, name, namespace string, timeout, pollInterval time.Duration) error {
			return nil
		},
		getKustomizationStatusFunc: func(name, namespace string) (*flux.KustomizationStatus, error) {
			return &flux.KustomizationStatus{
				Name:      name,
				Namespace: namespace,
				Ready:     true,
			}, nil
		},
	}
}

// TestTriggerDriftCheck_Success tests successful drift check
func TestTriggerDriftCheck_Success(t *testing.T) {
	db := setupDriftTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secrets
	secrets := []models.SecretDraft{
		{
			ID:          uuid.New(),
			SecretName:  "secret1",
			NamespaceID: namespace.ID,
			Data:        datatypes.JSON([]byte(`{"key1":"value1"}`)),
			Status:      "published",
		},
		{
			ID:          uuid.New(),
			SecretName:  "secret2",
			NamespaceID: namespace.ID,
			Data:        datatypes.JSON([]byte(`{"key2":"value2"}`)),
			Status:      "published",
		},
	}
	for _, s := range secrets {
		require.NoError(t, db.Create(&s).Error)
	}

	// Mock Git client
	mockGitClient := &driftMockGitClient{
		readFileFunc: func(path string) ([]byte, error) {
			return []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: test-ns
type: Opaque
data:
  key1: dmFsdWUx
`), nil
		},
	}

	// Mock SOPS client
	mockSOPSClient := &driftMockSOPSClient{}

	// Mock K8s client returning DIFFERENT data (drift)
	mockK8sClient := &driftMockK8sClient{
		getSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"key1": []byte("CHANGED"), // Different value
				},
			}, nil
		},
	}

	// Create drift detector
	detector := drift.NewDriftDetector(db, mockK8sClient, mockGitClient, mockSOPSClient, nil, getTestFluxClientForAPI(), getTestConfigForAPI())
	handlers := NewDriftHandlers(db, detector)

	// Create request
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/namespaces/%s/drift-check", namespace.ID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.TriggerDriftCheck(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	var response DriftCheckResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "test-ns", response.Namespace)
	assert.Equal(t, 2, response.Checked)
	assert.Equal(t, 2, response.Drifted)
	assert.Len(t, response.Events, 2)
}

// TestTriggerDriftCheck_NamespaceNotFound tests drift check with invalid namespace
func TestTriggerDriftCheck_NamespaceNotFound(t *testing.T) {
	db := setupDriftTestDB(t)

	// Create dummy drift detector
	detector := drift.NewDriftDetector(db, nil, nil, nil, nil, nil, getTestConfigForAPI())
	handlers := NewDriftHandlers(db, detector)

	// Create request with non-existent namespace
	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/namespaces/%s/drift-check", nonExistentID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", nonExistentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.TriggerDriftCheck(rr, req)

	// Assert 404 response
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestTriggerDriftCheck_InvalidNamespaceID tests drift check with invalid UUID
func TestTriggerDriftCheck_InvalidNamespaceID(t *testing.T) {
	db := setupDriftTestDB(t)

	// Create dummy drift detector
	detector := drift.NewDriftDetector(db, nil, nil, nil, nil, nil, getTestConfigForAPI())
	handlers := NewDriftHandlers(db, detector)

	// Create request with invalid UUID
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/invalid-uuid/drift-check", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.TriggerDriftCheck(rr, req)

	// Assert 400 response
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestListDriftEvents_Success tests successful drift events listing
func TestListDriftEvents_Success(t *testing.T) {
	db := setupDriftTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create drift events
	events := []models.DriftEvent{
		{
			SecretName:  "secret1",
			NamespaceID: namespace.ID,
			GitVersion:  datatypes.JSON([]byte(`{"keys":["username"]}`)),
			K8sVersion:  datatypes.JSON([]byte(`{"keys":["username","password"]}`)),
			Diff:        datatypes.JSON([]byte(`{"differences":["Key 'password' added in K8s"]}`)),
		},
		{
			SecretName:  "secret2",
			NamespaceID: namespace.ID,
			GitVersion:  datatypes.JSON([]byte(`{"keys":["api_key"]}`)),
			K8sVersion:  datatypes.JSON([]byte(`{"status":"not_found"}`)),
			Diff:        datatypes.JSON([]byte(`{"error":"Secret missing from Kubernetes cluster"}`)),
		},
	}
	for _, e := range events {
		require.NoError(t, db.Create(&e).Error)
	}

	// Create handlers
	handlers := NewDriftHandlers(db, nil)

	// Create request
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/drift-events", namespace.ID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.ListDriftEvents(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	var response DriftEventsResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "test-ns", response.Namespace)
	assert.Equal(t, int64(2), response.Total)
	assert.Len(t, response.Events, 2)
	assert.Equal(t, "secret1", response.Events[0].SecretName)
}

// TestListDriftEvents_FilterByStatus tests filtering drift events by status
func TestListDriftEvents_FilterByStatus(t *testing.T) {
	db := setupDriftTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create drift events (some resolved, some active)
	now := time.Now()
	events := []models.DriftEvent{
		{
			SecretName:  "secret1",
			NamespaceID: namespace.ID,
			GitVersion:  datatypes.JSON([]byte(`{}`)),
			K8sVersion:  datatypes.JSON([]byte(`{}`)),
			Diff:        datatypes.JSON([]byte(`{}`)),
			ResolvedAt:  &now, // RESOLVED
		},
		{
			SecretName:  "secret2",
			NamespaceID: namespace.ID,
			GitVersion:  datatypes.JSON([]byte(`{}`)),
			K8sVersion:  datatypes.JSON([]byte(`{}`)),
			Diff:        datatypes.JSON([]byte(`{}`)),
			ResolvedAt:  nil, // ACTIVE
		},
	}
	for _, e := range events {
		require.NoError(t, db.Create(&e).Error)
	}

	// Create handlers
	handlers := NewDriftHandlers(db, nil)

	// Test filter by "active" status
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/drift-events?status=active", namespace.ID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	handlers.ListDriftEvents(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response DriftEventsResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, int64(1), response.Total)
	assert.Len(t, response.Events, 1)
	assert.Equal(t, "secret2", response.Events[0].SecretName)
}

// TestListDriftEvents_FilterBySecretName tests filtering drift events by secret name
func TestListDriftEvents_FilterBySecretName(t *testing.T) {
	db := setupDriftTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create drift events
	events := []models.DriftEvent{
		{
			SecretName:  "secret1",
			NamespaceID: namespace.ID,
			GitVersion:  datatypes.JSON([]byte(`{}`)),
			K8sVersion:  datatypes.JSON([]byte(`{}`)),
			Diff:        datatypes.JSON([]byte(`{}`)),
		},
		{
			SecretName:  "secret2",
			NamespaceID: namespace.ID,
			GitVersion:  datatypes.JSON([]byte(`{}`)),
			K8sVersion:  datatypes.JSON([]byte(`{}`)),
			Diff:        datatypes.JSON([]byte(`{}`)),
		},
	}
	for _, e := range events {
		require.NoError(t, db.Create(&e).Error)
	}

	// Create handlers
	handlers := NewDriftHandlers(db, nil)

	// Test filter by secret name
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/drift-events?secret_name=secret1", namespace.ID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	handlers.ListDriftEvents(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response DriftEventsResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, int64(1), response.Total)
	assert.Len(t, response.Events, 1)
	assert.Equal(t, "secret1", response.Events[0].SecretName)
}

// TestListDriftEvents_Pagination tests pagination of drift events
func TestListDriftEvents_Pagination(t *testing.T) {
	db := setupDriftTestDB(t)

	// Create test namespace
	namespace := models.Namespace{
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create many drift events
	for i := 0; i < 10; i++ {
		event := models.DriftEvent{
			SecretName:  fmt.Sprintf("secret%d", i),
			NamespaceID: namespace.ID,
			GitVersion:  datatypes.JSON([]byte(`{}`)),
			K8sVersion:  datatypes.JSON([]byte(`{}`)),
			Diff:        datatypes.JSON([]byte(`{}`)),
		}
		require.NoError(t, db.Create(&event).Error)
	}

	// Create handlers
	handlers := NewDriftHandlers(db, nil)

	// Test pagination (limit=5, offset=0)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/drift-events?limit=5&offset=0", namespace.ID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", namespace.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	handlers.ListDriftEvents(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response DriftEventsResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, int64(10), response.Total)
	assert.Len(t, response.Events, 5) // Limited to 5
}

// TestListDriftEvents_NamespaceNotFound tests listing with invalid namespace
func TestListDriftEvents_NamespaceNotFound(t *testing.T) {
	db := setupDriftTestDB(t)

	// Create handlers
	handlers := NewDriftHandlers(db, nil)

	// Create request with non-existent namespace
	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/drift-events", nonExistentID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", nonExistentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute request
	rr := httptest.NewRecorder()
	handlers.ListDriftEvents(rr, req)

	// Assert 404 response
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
