package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKubeconfigPermissions_WorldReadable verifies that world-readable kubeconfigs are rejected
func TestKubeconfigPermissions_WorldReadable(t *testing.T) {
	db := setupTestDB(t)
	tempDir := t.TempDir()

	manager := NewClientManager(tempDir, db).(*clientManager)

	// Create a world-readable kubeconfig (0644 = -rw-r--r--)
	worldReadablePath := filepath.Join(tempDir, "world-readable.yaml")
	// Write minimal valid kubeconfig YAML (won't be parsed due to permission check)
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: fake-token
`
	err := os.WriteFile(worldReadablePath, []byte(kubeconfigContent), 0644)
	require.NoError(t, err)

	// Verify file has world-readable permissions
	info, err := os.Stat(worldReadablePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm(), "File should have 0644 permissions")

	// Attempt to validate the kubeconfig
	err = manager.validateKubeconfigFile(worldReadablePath)
	assert.Error(t, err, "World-readable kubeconfig should be rejected")
	assert.Contains(t, err.Error(), "insecure kubeconfig permissions", "Error should mention insecure permissions")
	assert.Contains(t, err.Error(), "644", "Error should show the actual permissions")
}

// TestKubeconfigPermissions_GroupReadable verifies that group-readable kubeconfigs are rejected
func TestKubeconfigPermissions_GroupReadable(t *testing.T) {
	db := setupTestDB(t)
	tempDir := t.TempDir()

	manager := NewClientManager(tempDir, db).(*clientManager)

	// Create a group-readable kubeconfig (0640 = -rw-r-----)
	groupReadablePath := filepath.Join(tempDir, "group-readable.yaml")
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: fake-token
`
	err := os.WriteFile(groupReadablePath, []byte(kubeconfigContent), 0640)
	require.NoError(t, err)

	// Verify file has group-readable permissions
	info, err := os.Stat(groupReadablePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm(), "File should have 0640 permissions")

	// Attempt to validate the kubeconfig
	err = manager.validateKubeconfigFile(groupReadablePath)
	assert.Error(t, err, "Group-readable kubeconfig should be rejected")
	assert.Contains(t, err.Error(), "insecure kubeconfig permissions", "Error should mention insecure permissions")
	assert.Contains(t, err.Error(), "640", "Error should show the actual permissions")
}

// TestKubeconfigPermissions_Secure0600 verifies that 0600 permissions are accepted
func TestKubeconfigPermissions_Secure0600(t *testing.T) {
	db := setupTestDB(t)
	tempDir := t.TempDir()

	manager := NewClientManager(tempDir, db).(*clientManager)

	// Create a secure kubeconfig (0600 = -rw-------)
	securePath := filepath.Join(tempDir, "secure-0600.yaml")
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: fake-token
`
	err := os.WriteFile(securePath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err)

	// Verify file has secure permissions
	info, err := os.Stat(securePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "File should have 0600 permissions")

	// Attempt to validate the kubeconfig - should pass permission check
	err = manager.validateKubeconfigFile(securePath)
	assert.NoError(t, err, "Secure 0600 permissions should be accepted")
}

// TestKubeconfigPermissions_Secure0400 verifies that 0400 permissions are accepted
func TestKubeconfigPermissions_Secure0400(t *testing.T) {
	db := setupTestDB(t)
	tempDir := t.TempDir()

	manager := NewClientManager(tempDir, db).(*clientManager)

	// Create a read-only secure kubeconfig (0400 = -r--------)
	securePath := filepath.Join(tempDir, "secure-0400.yaml")
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: fake-token
`
	err := os.WriteFile(securePath, []byte(kubeconfigContent), 0400)
	require.NoError(t, err)

	// Verify file has secure permissions
	info, err := os.Stat(securePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0400), info.Mode().Perm(), "File should have 0400 permissions")

	// Attempt to validate the kubeconfig - should pass permission check
	err = manager.validateKubeconfigFile(securePath)
	assert.NoError(t, err, "Secure 0400 permissions should be accepted")
}

// TestGetClient_RejectsInsecureKubeconfig verifies the full flow from GetClient
func TestGetClient_RejectsInsecureKubeconfig(t *testing.T) {
	db := setupTestDB(t)
	tempDir := t.TempDir()

	manager := NewClientManager(tempDir, db)

	// Create cluster in database
	clusterID := uuid.New()
	clusterName := "insecure-cluster"
	insertTestCluster(t, db, clusterID, clusterName, "unused-ref")

	// Create world-readable kubeconfig matching the cluster name
	insecurePath := filepath.Join(tempDir, clusterName+".yaml")
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: fake-token
`
	err := os.WriteFile(insecurePath, []byte(kubeconfigContent), 0644)
	require.NoError(t, err)

	// Attempt to get client - should fail at permission validation step
	_, err = manager.GetClient(clusterID)
	assert.Error(t, err, "GetClient should reject insecure kubeconfig")
	assert.Contains(t, err.Error(), "invalid kubeconfig", "Error should mention invalid kubeconfig")
	assert.Contains(t, err.Error(), "insecure", "Error should mention insecurity")
}

// TestAddClient_RejectsInsecureKubeconfig verifies AddClient also validates permissions
func TestAddClient_RejectsInsecureKubeconfig(t *testing.T) {
	db := setupTestDB(t)
	tempDir := t.TempDir()

	manager := NewClientManager(tempDir, db)

	// Create world-readable kubeconfig
	insecurePath := filepath.Join(tempDir, "insecure-add.yaml")
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: fake-token
`
	err := os.WriteFile(insecurePath, []byte(kubeconfigContent), 0644)
	require.NoError(t, err)

	// Attempt to add client - should fail at permission validation step
	clusterID := uuid.New()
	err = manager.AddClient(clusterID, insecurePath)
	assert.Error(t, err, "AddClient should reject insecure kubeconfig")
	assert.Contains(t, err.Error(), "invalid kubeconfig", "Error should mention invalid kubeconfig")
	assert.Contains(t, err.Error(), "insecure", "Error should mention insecurity")
}
