package drift

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/datatypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestSyncFromGit_Success tests successful sync from Git to K8s
func TestSyncFromGit_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "test-ns",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secret
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON([]byte(`{"username":"user","password":"pass"}`)),
		Status:      "drifted",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Create drift event
	driftEvent := models.DriftEvent{
		ID:          uuid.New(),
		SecretName:  secret.SecretName,
		NamespaceID: namespace.ID,
		DetectedAt:  time.Now(),
		GitVersion:  datatypes.JSON([]byte(`{"keys":["username","password"]}`)),
		K8sVersion:  datatypes.JSON([]byte(`{"keys":["username"]}`)),
		Diff:        datatypes.JSON([]byte(`{"differences":["password missing"]}`)),
	}
	require.NoError(t, db.Create(&driftEvent).Error)

	// Mock Git client - return secret YAML (with plain text in data - will be handled by SOPS in real scenario)
	gitClient := &MockGitClient{
		ReadFileFunc: func(path string) ([]byte, error) {
			// Return a valid K8s Secret YAML structure
			// In reality, this would be SOPS-encrypted, but for testing we simplify
			return []byte(`apiVersion: v1
kind: Secret
metadata:
  name: db-creds
  namespace: test-ns
type: Opaque
data:
  username: dXNlcg==
  password: cGFzcw==
`), nil
		},
	}

	// Mock SOPS client - decrypt YAML (return as-is for test since we're using plain YAML)
	sopsClient := &MockSOPSClient{
		DecryptYAMLFunc: func(encryptedYAML []byte) ([]byte, error) {
			return encryptedYAML, nil
		},
	}

	// Mock K8s client - verify apply was called with correct namespace
	applyCalled := false
	k8sClient := &MockK8sClient{
		ApplySecretFunc: func(ctx context.Context, namespace string, secret *corev1.Secret) error {
			assert.Equal(t, "test-ns", namespace)
			// Just verify it was called - YAML parsing details are tested elsewhere
			applyCalled = true
			return nil
		},
	}

	// Create detector and sync
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient)
	err := detector.SyncFromGit(ctx, driftEvent.ID)
	require.NoError(t, err)

	// Verify secret was applied to K8s
	assert.True(t, applyCalled, "ApplySecret should have been called")

	// Verify drift event was marked as resolved
	var updatedDrift models.DriftEvent
	require.NoError(t, db.First(&updatedDrift, driftEvent.ID).Error)
	assert.NotNil(t, updatedDrift.ResolvedAt)
	assert.Equal(t, "sync_from_git", updatedDrift.ResolutionAction)

	// Verify secret status updated to published
	var updatedSecret models.SecretDraft
	require.NoError(t, db.First(&updatedSecret, secret.ID).Error)
	assert.Equal(t, "published", updatedSecret.Status)

	// Verify audit log created
	var auditLog models.AuditLog
	err = db.Where("action_type = ?", "drift_sync_from_git").First(&auditLog).Error
	require.NoError(t, err)
	assert.Equal(t, "secret", auditLog.ResourceType)
	assert.Equal(t, "db-creds", auditLog.ResourceName)
}

// TestSyncFromGit_GitFileNotFound tests sync failure when Git file is missing
func TestSyncFromGit_GitFileNotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "test-ns",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create drift event
	driftEvent := models.DriftEvent{
		ID:          uuid.New(),
		SecretName:  "missing-secret",
		NamespaceID: namespace.ID,
		DetectedAt:  time.Now(),
		GitVersion:  datatypes.JSON([]byte(`{}`)),
		K8sVersion:  datatypes.JSON([]byte(`{}`)),
		Diff:        datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(&driftEvent).Error)

	// Mock Git client - return error
	gitClient := &MockGitClient{
		ReadFileFunc: func(path string) ([]byte, error) {
			return nil, fmt.Errorf("file not found")
		},
	}

	sopsClient := &MockSOPSClient{}
	k8sClient := &MockK8sClient{}

	// Create detector and attempt sync
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient)
	err := detector.SyncFromGit(ctx, driftEvent.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read secret from Git")
}

// TestSyncFromGit_AlreadyResolved tests sync failure when drift already resolved
func TestSyncFromGit_AlreadyResolved(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "test-ns",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create drift event that's already resolved
	now := time.Now()
	driftEvent := models.DriftEvent{
		ID:               uuid.New(),
		SecretName:       "db-creds",
		NamespaceID:      namespace.ID,
		DetectedAt:       time.Now(),
		ResolvedAt:       &now,
		ResolutionAction: "ignore",
		GitVersion:       datatypes.JSON([]byte(`{}`)),
		K8sVersion:       datatypes.JSON([]byte(`{}`)),
		Diff:             datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(&driftEvent).Error)

	gitClient := &MockGitClient{}
	sopsClient := &MockSOPSClient{}
	k8sClient := &MockK8sClient{}

	// Create detector and attempt sync
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient)
	err := detector.SyncFromGit(ctx, driftEvent.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already resolved")
}

// TestImportToGit_Success tests successful import from K8s to Git
func TestImportToGit_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "test-ns",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test secret
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON([]byte(`{"username":"user"}`)),
		Status:      "drifted",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Create drift event
	driftEvent := models.DriftEvent{
		ID:          uuid.New(),
		SecretName:  secret.SecretName,
		NamespaceID: namespace.ID,
		DetectedAt:  time.Now(),
		GitVersion:  datatypes.JSON([]byte(`{"keys":["username"]}`)),
		K8sVersion:  datatypes.JSON([]byte(`{"keys":["username","password"]}`)),
		Diff:        datatypes.JSON([]byte(`{"differences":["password added"]}`)),
	}
	require.NoError(t, db.Create(&driftEvent).Error)

	// Mock K8s client - return secret
	k8sClient := &MockK8sClient{
		GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-creds",
					Namespace: "test-ns",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"username": []byte("user"),
					"password": []byte("pass"),
				},
			}, nil
		},
	}

	// Mock SOPS client - encrypt YAML
	sopsClient := &MockSOPSClient{
		EncryptYAMLFunc: func(yamlContent []byte) ([]byte, error) {
			return append([]byte("# encrypted\n"), yamlContent...), nil
		},
	}

	// Mock Git client - write and commit
	writtenFiles := make(map[string][]byte)
	committed := false
	pushed := false
	gitClient := &MockGitClient{
		WriteFileFunc: func(path string, content []byte) error {
			writtenFiles[path] = content
			return nil
		},
		CommitFunc: func(message, authorName string, files []string) (string, error) {
			assert.Contains(t, message, "Import secret")
			assert.Contains(t, message, "db-creds")
			committed = true
			return "commit-sha", nil
		},
		PushFunc: func() error {
			pushed = true
			return nil
		},
	}

	// Create detector and import
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient)
	err := detector.ImportToGit(ctx, driftEvent.ID)
	require.NoError(t, err)

	// Verify file was written, committed, and pushed
	assert.NotEmpty(t, writtenFiles)
	assert.True(t, committed)
	assert.True(t, pushed)

	// Verify drift event was marked as resolved
	var updatedDrift models.DriftEvent
	require.NoError(t, db.First(&updatedDrift, driftEvent.ID).Error)
	assert.NotNil(t, updatedDrift.ResolvedAt)
	assert.Equal(t, "import_to_git", updatedDrift.ResolutionAction)

	// Verify secret status updated to published
	var updatedSecret models.SecretDraft
	require.NoError(t, db.First(&updatedSecret, secret.ID).Error)
	assert.Equal(t, "published", updatedSecret.Status)

	// Verify audit log created
	var auditLog models.AuditLog
	err = db.Where("action_type = ?", "drift_import_to_git").First(&auditLog).Error
	require.NoError(t, err)
	assert.Equal(t, "secret", auditLog.ResourceType)
	assert.Equal(t, "db-creds", auditLog.ResourceName)
}

// TestImportToGit_K8sSecretNotFound tests import failure when K8s secret missing
func TestImportToGit_K8sSecretNotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "test-ns",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create drift event
	driftEvent := models.DriftEvent{
		ID:          uuid.New(),
		SecretName:  "missing-secret",
		NamespaceID: namespace.ID,
		DetectedAt:  time.Now(),
		GitVersion:  datatypes.JSON([]byte(`{}`)),
		K8sVersion:  datatypes.JSON([]byte(`{}`)),
		Diff:        datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(&driftEvent).Error)

	// Mock K8s client - return error
	k8sClient := &MockK8sClient{
		GetSecretFunc: func(namespace, name string) (*corev1.Secret, error) {
			return nil, fmt.Errorf("secret not found")
		},
	}

	gitClient := &MockGitClient{}
	sopsClient := &MockSOPSClient{}

	// Create detector and attempt import
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient)
	err := detector.ImportToGit(ctx, driftEvent.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get secret from Kubernetes")
}

// TestMarkResolved_Success tests successful manual mark as resolved
func TestMarkResolved_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create test namespace
	namespace := models.Namespace{
		ID:          uuid.New(),
		Name:        "test-ns",
		Cluster:     "test-cluster",
		Environment: "dev",
	}
	require.NoError(t, db.Create(&namespace).Error)

	// Create test user
	user := models.User{
		ID:    uuid.New(),
		Email: "admin@example.com",
		Name:  "Admin User",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create test secret
	secret := models.SecretDraft{
		ID:          uuid.New(),
		SecretName:  "db-creds",
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON([]byte(`{"username":"user"}`)),
		Status:      "drifted",
	}
	require.NoError(t, db.Create(&secret).Error)

	// Create drift event
	driftEvent := models.DriftEvent{
		ID:          uuid.New(),
		SecretName:  secret.SecretName,
		NamespaceID: namespace.ID,
		DetectedAt:  time.Now(),
		GitVersion:  datatypes.JSON([]byte(`{}`)),
		K8sVersion:  datatypes.JSON([]byte(`{}`)),
		Diff:        datatypes.JSON([]byte(`{}`)),
	}
	require.NoError(t, db.Create(&driftEvent).Error)

	gitClient := &MockGitClient{}
	sopsClient := &MockSOPSClient{}
	k8sClient := &MockK8sClient{}

	// Create detector and mark resolved
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient)
	err := detector.MarkResolved(ctx, driftEvent.ID, user.ID)
	require.NoError(t, err)

	// Verify drift event was marked as resolved
	var updatedDrift models.DriftEvent
	require.NoError(t, db.First(&updatedDrift, driftEvent.ID).Error)
	assert.NotNil(t, updatedDrift.ResolvedAt)
	assert.NotNil(t, updatedDrift.ResolvedBy)
	assert.Equal(t, user.ID, *updatedDrift.ResolvedBy)
	assert.Equal(t, "ignore", updatedDrift.ResolutionAction)

	// Verify secret status updated to published
	var updatedSecret models.SecretDraft
	require.NoError(t, db.First(&updatedSecret, secret.ID).Error)
	assert.Equal(t, "published", updatedSecret.Status)

	// Verify audit log created
	var auditLog models.AuditLog
	err = db.Where("action_type = ?", "drift_mark_resolved").First(&auditLog).Error
	require.NoError(t, err)
	assert.Equal(t, "secret", auditLog.ResourceType)
	assert.Equal(t, "db-creds", auditLog.ResourceName)
	assert.Equal(t, user.ID, *auditLog.UserID)
}

// TestMarkResolved_DriftNotFound tests mark resolved with invalid drift ID
func TestMarkResolved_DriftNotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	gitClient := &MockGitClient{}
	sopsClient := &MockSOPSClient{}
	k8sClient := &MockK8sClient{}

	// Create detector and attempt to mark non-existent drift
	detector := NewDriftDetector(db, k8sClient, gitClient, sopsClient)
	err := detector.MarkResolved(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load drift event")
}
