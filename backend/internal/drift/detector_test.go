package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	EnsureRepoFunc  func() error
	ReadFileFunc    func(path string) ([]byte, error)
	WriteFileFunc   func(path string, content []byte) error
	CommitFunc      func(message, authorName string, files []string) (string, error)
	PushFunc        func() error
	GetFilePathFunc func(namespace, secretName string) string
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

func (m *MockGitClient) GetFilePath(namespace, secretName string) string {
	if m.GetFilePathFunc != nil {
		return m.GetFilePathFunc(namespace, secretName)
	}
	return fmt.Sprintf("namespaces/%s/secrets/%s.yaml", namespace, secretName)
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
	GetSecretFunc   func(namespace, name string) (*corev1.Secret, error)
	ApplySecretFunc func(ctx context.Context, namespace string, secret *corev1.Secret) error
}

func (m *MockK8sClient) GetSecret(namespace, name string) (*corev1.Secret, error) {
	if m.GetSecretFunc != nil {
		return m.GetSecretFunc(namespace, name)
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
}

func (m *MockK8sClient) ApplySecret(ctx context.Context, namespace string, secret *corev1.Secret) error {
	if m.ApplySecretFunc != nil {
		return m.ApplySecretFunc(ctx, namespace, secret)
	}
	return nil
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
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient, nil)

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
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient, nil)

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
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient, nil)

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
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient, nil)

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
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient, nil)

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
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient, nil)

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
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient, nil)

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
