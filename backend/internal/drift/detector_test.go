package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/config"
	"github.com/yourorg/secret-manager/internal/flux"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func init() {
	logger.Init("error")
}

// MockGitClient for testing
type MockGitClient struct {
	EnsureRepoFunc           func() error
	ReadFileFunc             func(path string) ([]byte, error)
	WriteFileFunc            func(path string, content []byte) error
	CommitFunc               func(message, authorName string, files []string) (string, error)
	PushFunc                 func() error
	GetFilePathFunc          func(clusterName, namespace, secretName string) string
	GetFilePathLegacyFunc    func(namespace, secretName string) string
	ReadFileWithFallbackFunc func(clusterName, namespace, secretName string) ([]byte, string, error)
}

func (m *MockGitClient) EnsureRepo() error {
	if m.EnsureRepoFunc != nil {
		return m.EnsureRepoFunc()
	}
	return nil
}

func (m *MockGitClient) ReadFile(path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(path)
	}
	return nil, fmt.Errorf("file not found")
}

func (m *MockGitClient) WriteFile(path string, content []byte) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(path, content)
	}
	return nil
}

func (m *MockGitClient) Commit(message, authorName string, files []string) (string, error) {
	if m.CommitFunc != nil {
		return m.CommitFunc(message, authorName, files)
	}
	return "mock-commit-sha", nil
}

func (m *MockGitClient) Push() error {
	if m.PushFunc != nil {
		return m.PushFunc()
	}
	return nil
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
	// Default implementation: try new path, fallback to legacy
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

// MockSOPSClient for testing
type MockSOPSClient struct {
	DecryptYAMLFunc func(encryptedYAML []byte) ([]byte, error)
	EncryptYAMLFunc func(yamlContent []byte) ([]byte, error)
}

func (m *MockSOPSClient) DecryptYAML(encryptedYAML []byte) ([]byte, error) {
	if m.DecryptYAMLFunc != nil {
		return m.DecryptYAMLFunc(encryptedYAML)
	}
	return encryptedYAML, nil
}

func (m *MockSOPSClient) EncryptYAML(yamlContent []byte) ([]byte, error) {
	if m.EncryptYAMLFunc != nil {
		return m.EncryptYAMLFunc(yamlContent)
	}
	return yamlContent, nil
}

// MockK8sClient for testing
type MockK8sClient struct {
	GetSecretFunc func(namespace, name string) (*corev1.Secret, error)
}

func (m *MockK8sClient) GetSecret(namespace, name string) (*corev1.Secret, error) {
	if m.GetSecretFunc != nil {
		return m.GetSecretFunc(namespace, name)
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
}

// MockFluxClient for testing
type MockFluxClient struct {
	TriggerKustomizationReconciliationFunc func(ctx context.Context, name, namespace string) error
	TriggerGitRepositoryReconciliationFunc func(ctx context.Context, name, namespace string) error
	WaitForKustomizationReconciliationFunc func(ctx context.Context, name, namespace string, timeout, pollInterval time.Duration) error
	GetKustomizationStatusFunc             func(name, namespace string) (*flux.KustomizationStatus, error)
}

func (m *MockFluxClient) TriggerKustomizationReconciliation(ctx context.Context, name, namespace string) error {
	if m.TriggerKustomizationReconciliationFunc != nil {
		return m.TriggerKustomizationReconciliationFunc(ctx, name, namespace)
	}
	return nil
}

func (m *MockFluxClient) TriggerGitRepositoryReconciliation(ctx context.Context, name, namespace string) error {
	if m.TriggerGitRepositoryReconciliationFunc != nil {
		return m.TriggerGitRepositoryReconciliationFunc(ctx, name, namespace)
	}
	return nil
}

func (m *MockFluxClient) WaitForKustomizationReconciliation(ctx context.Context, name, namespace string, timeout, pollInterval time.Duration) error {
	if m.WaitForKustomizationReconciliationFunc != nil {
		return m.WaitForKustomizationReconciliationFunc(ctx, name, namespace, timeout, pollInterval)
	}
	return nil
}

func (m *MockFluxClient) GetKustomizationStatus(name, namespace string) (*flux.KustomizationStatus, error) {
	if m.GetKustomizationStatusFunc != nil {
		return m.GetKustomizationStatusFunc(name, namespace)
	}
	return &flux.KustomizationStatus{
		Name:      name,
		Namespace: namespace,
		Ready:     true,
	}, nil
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create tables manually for SQLite compatibility
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

	err = db.Exec(`
		CREATE TABLE drift_events (
			id TEXT PRIMARY KEY,
			secret_name TEXT NOT NULL,
			namespace_id TEXT NOT NULL,
			detected_at DATETIME NOT NULL,
			git_version TEXT NOT NULL,
			k8s_version TEXT NOT NULL,
			diff TEXT NOT NULL,
			resolved_at DATETIME,
			resolved_by TEXT,
			resolution_action TEXT CHECK(resolution_action IN ('sync_from_git', 'import_to_git', 'ignore', '') OR resolution_action IS NULL),
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			azure_ad_oid TEXT,
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
			timestamp DATETIME NOT NULL,
			metadata TEXT,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	return db
}

// getTestConfig returns a config with sensible test defaults
func getTestConfig() *config.Config {
	return &config.Config{
		FluxKustomizationName: "secrets",
		FluxKustomizationNS:   "flux-system",
		FluxGitRepositoryName: "secrets-repo",
		FluxReconcileTimeout:  2 * time.Minute,
		FluxPollInterval:      2 * time.Second,
	}
}

// getTestFluxClient returns a mock FluxClient that succeeds by default
func getTestFluxClient() *MockFluxClient {
	return &MockFluxClient{
		TriggerKustomizationReconciliationFunc: func(ctx context.Context, name, namespace string) error {
			return nil
		},
		TriggerGitRepositoryReconciliationFunc: func(ctx context.Context, name, namespace string) error {
			return nil
		},
		WaitForKustomizationReconciliationFunc: func(ctx context.Context, name, namespace string, timeout, pollInterval time.Duration) error {
			return nil
		},
		GetKustomizationStatusFunc: func(name, namespace string) (*flux.KustomizationStatus, error) {
			return &flux.KustomizationStatus{
				Name:      name,
				Namespace: namespace,
				Ready:     true,
			}, nil
		},
	}
}

// TestDetectDriftForSecret_NoChange tests drift detection when there's no drift
func TestDetectDriftForSecret_NoChange(t *testing.T) {
	db := setupTestDB(t)

	// Create test namespace
	namespace := models.Namespace{Cluster: "test-cluster", Environment: "dev",
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secret
	secretData := map[string]string{"username": "admin", "password": "secret123"}
	secretDataJSON, _ := json.Marshal(secretData)
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON(secretDataJSON),
		Status:      "published",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Mock Git client returning decrypted YAML (using stringData for plain-text values)
	gitClient := &MockGitClient{
		ReadFileFunc: func(path string) ([]byte, error) {
			yamlContent := `apiVersion: v1
kind: Secret
metadata:
  name: db-creds
  namespace: test-ns
type: Opaque
stringData:
  username: admin
  password: secret123
`
			return []byte(yamlContent), nil
		},
	}

	// Mock SOPS client (no-op decryption)
	sopsClient := &MockSOPSClient{}

	// Mock K8s client returning matching secret
	k8sClient := &MockK8sClient{
		GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-creds",
					Namespace: "test-ns",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("secret123"),
				},
			}, nil
		},
	}

	// Create drift detector
	detector := NewDriftDetectorWithSingleClient(db, k8sClient, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Detect drift
	event, err := detector.DetectDriftForSecret(secret.ID)

	// Assert no drift detected
	require.NoError(t, err)
	assert.Nil(t, event, "No drift should be detected")
}

// TestDetectDriftForSecret_Modified tests drift detection when data is modified
func TestDetectDriftForSecret_Deleted(t *testing.T) {
	db := setupTestDB(t)

	// Create test namespace
	namespace := models.Namespace{Cluster: "test-cluster", Environment: "dev",
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secret
	secretData := map[string]string{"username": "admin", "password": "secret123"}
	secretDataJSON, _ := json.Marshal(secretData)
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON(secretDataJSON),
		Status:      "published",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Mock Git client returning decrypted YAML (using stringData for plain-text values)
	gitClient := &MockGitClient{
		ReadFileFunc: func(path string) ([]byte, error) {
			yamlContent := `apiVersion: v1
kind: Secret
metadata:
  name: db-creds
  namespace: test-ns
type: Opaque
stringData:
  username: admin
  password: secret123
`
			return []byte(yamlContent), nil
		},
	}

	// Mock SOPS client (no-op decryption)
	sopsClient := &MockSOPSClient{}

	// Mock K8s client returning NOT FOUND error
	k8sClient := &MockK8sClient{
		GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
			return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
		},
	}

	// Create drift detector
	detector := NewDriftDetectorWithSingleClient(db, k8sClient, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Detect drift
	event, err := detector.DetectDriftForSecret(secret.ID)

	// Assert drift detected
	require.NoError(t, err)
	require.NotNil(t, event, "Drift should be detected (secret missing from K8s)")
	assert.Equal(t, "db-creds", event.SecretName)

	// Verify K8s version indicates not found
	var k8sVersion map[string]interface{}
	err = json.Unmarshal([]byte(event.K8sVersion), &k8sVersion)
	require.NoError(t, err)
	assert.Equal(t, "not_found", k8sVersion["status"])
}

// TestDetectDriftForSecret_GitFileMissing tests drift when file is missing from Git
func TestDetectDriftForSecret_GitFileMissing(t *testing.T) {
	db := setupTestDB(t)

	// Create test namespace
	namespace := models.Namespace{Cluster: "test-cluster", Environment: "dev",
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secret
	secretData := map[string]string{"username": "admin", "password": "secret123"}
	secretDataJSON, _ := json.Marshal(secretData)
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON(secretDataJSON),
		Status:      "published",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Mock Git client returning FILE NOT FOUND
	gitClient := &MockGitClient{
		ReadFileFunc: func(path string) ([]byte, error) {
			return nil, fmt.Errorf("file not found: %s", path)
		},
	}

	// Mock SOPS client (no-op decryption)
	sopsClient := &MockSOPSClient{}

	// Mock K8s client (won't be called in this scenario)
	k8sClient := &MockK8sClient{}

	// Create drift detector
	detector := NewDriftDetectorWithSingleClient(db, k8sClient, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Detect drift
	event, err := detector.DetectDriftForSecret(secret.ID)

	// Assert drift detected
	require.NoError(t, err)
	require.NotNil(t, event, "Drift should be detected (file missing from Git)")
	assert.Equal(t, "db-creds", event.SecretName)

	// Verify diff contains error message
	var diff map[string]interface{}
	err = json.Unmarshal([]byte(event.Diff), &diff)
	require.NoError(t, err)
	assert.Contains(t, diff["error"], "Secret file missing from Git repository")
}

// TestDetectDriftForSecret_DraftSecret tests that draft secrets are skipped
func TestDetectDriftForSecret_DraftSecret(t *testing.T) {
	db := setupTestDB(t)

	// Create test namespace
	namespace := models.Namespace{Cluster: "test-cluster", Environment: "dev",
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secret with DRAFT status
	secretData := map[string]string{"username": "admin", "password": "secret123"}
	secretDataJSON, _ := json.Marshal(secretData)
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON(secretDataJSON),
		Status:      "draft", // <-- DRAFT STATUS
	}
	require.NoError(t, db.Create(&secret).Error)

	// Mock clients (won't be called)
	gitClient := &MockGitClient{}
	sopsClient := &MockSOPSClient{}
	k8sClient := &MockK8sClient{}

	// Create drift detector
	detector := NewDriftDetectorWithSingleClient(db, k8sClient, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Detect drift
	event, err := detector.DetectDriftForSecret(secret.ID)

	// Assert NO drift check performed
	require.NoError(t, err)
	assert.Nil(t, event, "Draft secrets should not be checked for drift")
}

// TestDetectDriftForNamespace tests drift detection for all secrets in a namespace
func TestDetectDriftForNamespace(t *testing.T) {
	db := setupTestDB(t)

	// Create test namespace
	namespace := models.Namespace{Cluster: "test-cluster", Environment: "dev",
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create multiple test secrets
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
		{
			ID:          uuid.New(),
			SecretName:  "secret3",
			NamespaceID: namespace.ID,
			Data:        datatypes.JSON([]byte(`{"key3":"value3"}`)),
			Status:      "draft", // This one should be skipped
		},
	}
	for _, s := range secrets {
		require.NoError(t, db.Create(&s).Error)
	}

	// Mock Git client
	gitClient := &MockGitClient{
		ReadFileFunc: func(path string) ([]byte, error) {
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
	sopsClient := &MockSOPSClient{}

	// Mock K8s client returning different data (drift)
	k8sClient := &MockK8sClient{
		GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"key1": []byte("CHANGED"), // <-- CHANGED
				},
			}, nil
		},
	}

	// Create drift detector
	detector := NewDriftDetectorWithSingleClient(db, k8sClient, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Detect drift for entire namespace
	events, err := detector.DetectDriftForNamespace(namespace.ID)

	// Assert drift detected for published secrets only
	require.NoError(t, err)
	assert.Equal(t, 2, len(events), "Should detect drift for 2 published secrets")

	// Verify secrets are marked as drifted
	var updatedSecret1 models.SecretDraft
	db.First(&updatedSecret1, "secret_name = ?", "secret1")
	assert.Equal(t, "drifted", updatedSecret1.Status)
}

// TestDetectDriftForSecret_DecryptionError tests handling of SOPS decryption errors
func TestDetectDriftForSecret_DecryptionError(t *testing.T) {
	db := setupTestDB(t)

	// Create test namespace
	namespace := models.Namespace{Cluster: "test-cluster", Environment: "dev",
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secret
	secretData := map[string]string{"username": "admin"}
	secretDataJSON, _ := json.Marshal(secretData)
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON(secretDataJSON),
		Status:      "published",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Mock Git client
	gitClient := &MockGitClient{
		ReadFileFunc: func(path string) ([]byte, error) {
			return []byte("encrypted content"), nil
		},
	}

	// Mock SOPS client returning decryption error
	sopsClient := &MockSOPSClient{
		DecryptYAMLFunc: func(encryptedYAML []byte) ([]byte, error) {
			return nil, fmt.Errorf("decryption failed: invalid key")
		},
	}

	// Mock K8s client
	k8sClient := &MockK8sClient{}

	// Create drift detector
	detector := NewDriftDetectorWithSingleClient(db, k8sClient, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Detect drift
	event, err := detector.DetectDriftForSecret(secret.ID)

	// Assert error returned
	require.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "failed to decrypt")
}

// TestDetectDriftForSecret_KeyAdded tests drift when a key is added in K8s
func TestDetectDriftForSecret_KeyAdded(t *testing.T) {
	db := setupTestDB(t)

	// Create test namespace
	namespace := models.Namespace{Cluster: "test-cluster", Environment: "dev",
		ID:   uuid.New(),
		Name: "test-ns",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secret
	secretData := map[string]string{"username": "admin"}
	secretDataJSON, _ := json.Marshal(secretData)
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON(secretDataJSON),
		Status:      "published",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Mock Git client returning YAML with only username (using stringData for plain-text values)
	gitClient := &MockGitClient{
		ReadFileFunc: func(path string) ([]byte, error) {
			return []byte(`apiVersion: v1
kind: Secret
metadata:
  name: db-creds
  namespace: test-ns
type: Opaque
stringData:
  username: admin
`), nil
		},
	}

	// Mock SOPS client
	sopsClient := &MockSOPSClient{}

	// Mock K8s client returning secret with EXTRA key
	k8sClient := &MockK8sClient{
		GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-creds",
					Namespace: "test-ns",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("extra-key"), // <-- EXTRA KEY
				},
			}, nil
		},
	}

	// Create drift detector
	detector := NewDriftDetectorWithSingleClient(db, k8sClient, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Detect drift
	event, err := detector.DetectDriftForSecret(secret.ID)

	// Assert drift detected
	require.NoError(t, err)
	require.NotNil(t, event, "Drift should be detected (extra key in K8s)")

	// Verify diff mentions the added key
	var diff map[string]interface{}
	err = json.Unmarshal([]byte(event.Diff), &diff)
	require.NoError(t, err)

	differences := diff["differences"].([]interface{})
	assert.Contains(t, differences, "Key 'password' added in K8s (not in Git)")
}

// MockClientManager for testing
type MockClientManager struct {
	GetClientFunc   func(clusterID uuid.UUID) (K8sClientInterface, error)
	HealthCheckFunc func(clusterID uuid.UUID) (bool, error)
}

func (m *MockClientManager) GetClient(clusterID uuid.UUID) (K8sClientInterface, error) {
	if m.GetClientFunc != nil {
		return m.GetClientFunc(clusterID)
	}
	return nil, fmt.Errorf("cluster not found")
}

func (m *MockClientManager) HealthCheck(clusterID uuid.UUID) (bool, error) {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(clusterID)
	}
	return true, nil
}

// TestDetectDriftForAllClusters_MultiClusterSuccess tests drift detection across multiple healthy clusters
func TestDetectDriftForAllClusters_MultiClusterSuccess(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate schema
	err = db.AutoMigrate(&models.Cluster{}, &models.Namespace{}, &models.SecretDraft{}, &models.DriftEvent{})
	require.NoError(t, err)

	// Create two clusters
	cluster1 := models.Cluster{
		ID:          uuid.New(),
		Name:        "devops",
		Environment: "prod",
		IsHealthy:   true,
	}
	cluster2 := models.Cluster{
		ID:          uuid.New(),
		Name:        "staging",
		Environment: "staging",
		IsHealthy:   true,
	}
	require.NoError(t, db.Create(&cluster1).Error)
	require.NoError(t, db.Create(&cluster2).Error)

	// Create namespaces in each cluster
	ns1 := models.Namespace{
		ID:        uuid.New(),
		Name:      "default",
		ClusterID: &cluster1.ID,
	}
	ns2 := models.Namespace{
		ID:        uuid.New(),
		Name:      "production",
		ClusterID: &cluster2.ID,
	}
	require.NoError(t, db.Create(&ns1).Error)
	require.NoError(t, db.Create(&ns2).Error)

	// Create secrets in each namespace
	secret1 := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: ns1.ID,
		Status:      "published",
		Data:        datatypes.JSON(`{"username":"admin"}`),
	}
	secret2 := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "api-key",
		NamespaceID: ns2.ID,
		Status:      "published",
		Data:        datatypes.JSON(`{"key":"secret"}`),
	}
	require.NoError(t, db.Create(&secret1).Error)
	require.NoError(t, db.Create(&secret2).Error)

	// Mock ClientManager returning different K8s clients per cluster
	clientManager := &MockClientManager{
		GetClientFunc: func(clusterID uuid.UUID) (K8sClientInterface, error) {
			// Both clusters return working K8s clients with mismatched secrets (drift)
			return &MockK8sClient{
				GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{
							"mismatch": []byte("drift-detected"), // Different from Git
						},
					}, nil
				},
			}, nil
		},
	}

	// Mock Git client
	gitClient := &MockGitClient{
		EnsureRepoFunc: func() error { return nil },
		ReadFileFunc: func(path string) ([]byte, error) {
			// Return valid secret YAML (different from K8s)
			return []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: test-ns
type: Opaque
data:
  username: YWRtaW4=
`), nil
		},
	}

	// Mock SOPS client
	sopsClient := &MockSOPSClient{}

	// Create drift detector
	detector := NewDriftDetector(db, clientManager, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Run multi-cluster drift detection
	driftEvents, err := detector.DetectDriftForAllClusters()

	// Assert success
	require.NoError(t, err)
	assert.Len(t, driftEvents, 2, "Both clusters should have drift events")

	// Verify drift events for cluster1
	assert.Contains(t, driftEvents, cluster1.ID)
	assert.Len(t, driftEvents[cluster1.ID], 1)

	// Verify drift events for cluster2
	assert.Contains(t, driftEvents, cluster2.ID)
	assert.Len(t, driftEvents[cluster2.ID], 1)

	// Verify secrets marked as drifted
	var updatedSecret1 models.SecretDraft
	require.NoError(t, db.First(&updatedSecret1, secret1.ID).Error)
	assert.Equal(t, "drifted", updatedSecret1.Status)

	var updatedSecret2 models.SecretDraft
	require.NoError(t, db.First(&updatedSecret2, secret2.ID).Error)
	assert.Equal(t, "drifted", updatedSecret2.Status)
}

// TestDetectDriftForAllClusters_ClusterUnreachable tests that drift detection continues when one cluster is unreachable
func TestDetectDriftForAllClusters_ClusterUnreachable(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate schema
	err = db.AutoMigrate(&models.Cluster{}, &models.Namespace{}, &models.SecretDraft{}, &models.DriftEvent{})
	require.NoError(t, err)

	// Create two clusters
	cluster1 := models.Cluster{
		ID:          uuid.New(),
		Name:        "healthy-cluster",
		Environment: "prod",
		IsHealthy:   true,
	}
	cluster2 := models.Cluster{
		ID:          uuid.New(),
		Name:        "unreachable-cluster",
		Environment: "staging",
		IsHealthy:   true, // Will become unhealthy
	}
	require.NoError(t, db.Create(&cluster1).Error)
	require.NoError(t, db.Create(&cluster2).Error)

	// Create namespaces
	ns1 := models.Namespace{
		ID:        uuid.New(),
		Name:      "default",
		ClusterID: &cluster1.ID,
	}
	ns2 := models.Namespace{
		ID:        uuid.New(),
		Name:      "production",
		ClusterID: &cluster2.ID,
	}
	require.NoError(t, db.Create(&ns1).Error)
	require.NoError(t, db.Create(&ns2).Error)

	// Create secrets
	secret1 := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: ns1.ID,
		Status:      "published",
		Data:        datatypes.JSON(`{"username":"admin"}`),
	}
	secret2 := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "api-key",
		NamespaceID: ns2.ID,
		Status:      "published",
		Data:        datatypes.JSON(`{"key":"secret"}`),
	}
	require.NoError(t, db.Create(&secret1).Error)
	require.NoError(t, db.Create(&secret2).Error)

	// Mock ClientManager: cluster1 works, cluster2 fails
	clientManager := &MockClientManager{
		GetClientFunc: func(clusterID uuid.UUID) (K8sClientInterface, error) {
			if clusterID == cluster1.ID {
				// Healthy cluster - return working client
				return &MockK8sClient{
					GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
						return &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      name,
								Namespace: namespace,
							},
							Type: corev1.SecretTypeOpaque,
							Data: map[string][]byte{
								"drift": []byte("detected"),
							},
						}, nil
					},
				}, nil
			}
			// Cluster2 unreachable
			return nil, fmt.Errorf("cluster API unreachable: connection timeout")
		},
	}

	// Mock Git client
	gitClient := &MockGitClient{
		EnsureRepoFunc: func() error { return nil },
		ReadFileFunc: func(path string) ([]byte, error) {
			return []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: test-ns
type: Opaque
data:
  username: YWRtaW4=
`), nil
		},
	}

	// Mock SOPS client
	sopsClient := &MockSOPSClient{}

	// Create drift detector
	detector := NewDriftDetector(db, clientManager, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Run multi-cluster drift detection
	driftEvents, err := detector.DetectDriftForAllClusters()

	// Assert success (drift detection doesn't abort on cluster failure)
	require.NoError(t, err)

	// Verify only cluster1 has drift events (cluster2 was skipped)
	assert.Len(t, driftEvents, 1, "Only healthy cluster should have drift events")
	assert.Contains(t, driftEvents, cluster1.ID)
	assert.NotContains(t, driftEvents, cluster2.ID, "Unreachable cluster should be skipped")

	// Verify secret1 marked as drifted
	var updatedSecret1 models.SecretDraft
	require.NoError(t, db.First(&updatedSecret1, secret1.ID).Error)
	assert.Equal(t, "drifted", updatedSecret1.Status)

	// Verify secret2 NOT marked as drifted (cluster unreachable)
	var updatedSecret2 models.SecretDraft
	require.NoError(t, db.First(&updatedSecret2, secret2.ID).Error)
	assert.Equal(t, "published", updatedSecret2.Status, "Secret in unreachable cluster should remain published")
}

// TestDetectDriftForAllClusters_NoDrift tests drift detection when all secrets match
func TestDetectDriftForAllClusters_NoDrift(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate schema
	err = db.AutoMigrate(&models.Cluster{}, &models.Namespace{}, &models.SecretDraft{}, &models.DriftEvent{})
	require.NoError(t, err)

	// Create cluster
	cluster := models.Cluster{
		ID:          uuid.New(),
		Name:        "devops",
		Environment: "prod",
		IsHealthy:   true,
	}
	require.NoError(t, db.Create(&cluster).Error)

	// Create namespace
	ns := models.Namespace{
		ID:        uuid.New(),
		Name:      "default",
		ClusterID: &cluster.ID,
	}
	require.NoError(t, db.Create(&ns).Error)

	// Create secret
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: ns.ID,
		Status:      "published",
		Data:        datatypes.JSON(`{"username":"admin"}`),
	}
	require.NoError(t, db.Create(&secret).Error)

	// Mock ClientManager with matching K8s data
	clientManager := &MockClientManager{
		GetClientFunc: func(clusterID uuid.UUID) (K8sClientInterface, error) {
			return &MockK8sClient{
				GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{
							"username": []byte("admin"), // Matches Git
						},
					}, nil
				},
			}, nil
		},
	}

	// Mock Git client returning matching data
	gitClient := &MockGitClient{
		EnsureRepoFunc: func() error { return nil },
		ReadFileFunc: func(path string) ([]byte, error) {
			return []byte(`apiVersion: v1
kind: Secret
metadata:
  name: db-creds
  namespace: default
type: Opaque
data:
  username: YWRtaW4=
`), nil
		},
	}

	// Mock SOPS client
	sopsClient := &MockSOPSClient{}

	// Create drift detector
	detector := NewDriftDetector(db, clientManager, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Run multi-cluster drift detection
	driftEvents, err := detector.DetectDriftForAllClusters()

	// Assert no drift detected
	require.NoError(t, err)
	assert.Empty(t, driftEvents, "No drift should be detected when secrets match")

	// Verify secret status unchanged
	var updatedSecret models.SecretDraft
	require.NoError(t, db.First(&updatedSecret, secret.ID).Error)
	assert.Equal(t, "published", updatedSecret.Status)
}

// TestDetectDriftForAllClusters_ClusterIsolation tests that secrets from cluster A don't affect cluster B
func TestDetectDriftForAllClusters_ClusterIsolation(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate schema
	err = db.AutoMigrate(&models.Cluster{}, &models.Namespace{}, &models.SecretDraft{}, &models.DriftEvent{})
	require.NoError(t, err)

	// Create two clusters
	cluster1 := models.Cluster{
		ID:          uuid.New(),
		Name:        "cluster-a",
		Environment: "prod",
		IsHealthy:   true,
	}
	cluster2 := models.Cluster{
		ID:          uuid.New(),
		Name:        "cluster-b",
		Environment: "staging",
		IsHealthy:   true,
	}
	require.NoError(t, db.Create(&cluster1).Error)
	require.NoError(t, db.Create(&cluster2).Error)

	// Create namespaces with SAME NAME in both clusters
	ns1 := models.Namespace{
		ID:        uuid.New(),
		Name:      "default", // Same namespace name
		ClusterID: &cluster1.ID,
	}
	ns2 := models.Namespace{
		ID:        uuid.New(),
		Name:      "default", // Same namespace name
		ClusterID: &cluster2.ID,
	}
	require.NoError(t, db.Create(&ns1).Error)
	require.NoError(t, db.Create(&ns2).Error)

	// Create secrets with SAME NAME in both namespaces
	secret1 := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "shared-secret", // Same secret name
		NamespaceID: ns1.ID,
		Status:      "published",
		Data:        datatypes.JSON(`{"key":"cluster-a-value"}`),
	}
	secret2 := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "shared-secret", // Same secret name
		NamespaceID: ns2.ID,
		Status:      "published",
		Data:        datatypes.JSON(`{"key":"cluster-b-value"}`),
	}
	require.NoError(t, db.Create(&secret1).Error)
	require.NoError(t, db.Create(&secret2).Error)

	// Mock ClientManager returning DIFFERENT K8s data per cluster
	clientManager := &MockClientManager{
		GetClientFunc: func(clusterID uuid.UUID) (K8sClientInterface, error) {
			if clusterID == cluster1.ID {
				// Cluster A returns secret with "cluster-a-k8s"
				return &MockK8sClient{
					GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
						return &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      name,
								Namespace: namespace,
							},
							Type: corev1.SecretTypeOpaque,
							Data: map[string][]byte{
								"key": []byte("cluster-a-k8s"),
							},
						}, nil
					},
				}, nil
			}
			// Cluster B returns secret with "cluster-b-k8s"
			return &MockK8sClient{
				GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{
							"key": []byte("cluster-b-k8s"),
						},
					}, nil
				},
			}, nil
		},
	}

	// Mock Git client returning DIFFERENT data per cluster
	gitClient := &MockGitClient{
		EnsureRepoFunc: func() error { return nil },
		ReadFileFunc: func(path string) ([]byte, error) {
			// Return different Git data based on path
			if path == "clusters/cluster-a/namespaces/default/secrets/shared-secret.yaml" {
				return []byte(`apiVersion: v1
kind: Secret
metadata:
  name: shared-secret
  namespace: default
type: Opaque
data:
  key: Y2x1c3Rlci1hLWdpdA==
`), nil
			}
			if path == "clusters/cluster-b/namespaces/default/secrets/shared-secret.yaml" {
				return []byte(`apiVersion: v1
kind: Secret
metadata:
  name: shared-secret
  namespace: default
type: Opaque
data:
  key: Y2x1c3Rlci1iLWdpdA==
`), nil
			}
			return nil, fmt.Errorf("file not found: %s", path)
		},
	}

	// Mock SOPS client
	sopsClient := &MockSOPSClient{}

	// Create drift detector
	detector := NewDriftDetector(db, clientManager, gitClient, sopsClient, nil, getTestFluxClient(), getTestConfig())

	// Run multi-cluster drift detection
	driftEvents, err := detector.DetectDriftForAllClusters()

	// Assert both clusters have drift (Git vs K8s mismatch)
	require.NoError(t, err)
	assert.Len(t, driftEvents, 2, "Both clusters should have drift")
	assert.Len(t, driftEvents[cluster1.ID], 1, "Cluster A should have 1 drift event")
	assert.Len(t, driftEvents[cluster2.ID], 1, "Cluster B should have 1 drift event")

	// Verify drift events reference correct cluster namespaces
	cluster1Events := driftEvents[cluster1.ID]
	assert.Equal(t, ns1.ID, cluster1Events[0].NamespaceID, "Cluster A drift should reference namespace in cluster A")

	cluster2Events := driftEvents[cluster2.ID]
	assert.Equal(t, ns2.ID, cluster2Events[0].NamespaceID, "Cluster B drift should reference namespace in cluster B")
}
