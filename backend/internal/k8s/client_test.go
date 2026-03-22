package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewK8sClient_InvalidKubeconfig(t *testing.T) {
	// Test with invalid kubeconfig path
	_, err := NewK8sClient("/nonexistent/kubeconfig")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build kubernetes config")
}

func TestGetSecret_Success(t *testing.T) {
	// Create fake clientset with a secret
	fakeClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret123"),
			},
		},
	)

	client := &K8sClient{clientset: fakeClient}

	secret, err := client.GetSecret("default", "test-secret")
	require.NoError(t, err)
	require.NotNil(t, secret)
	assert.Equal(t, "test-secret", secret.Name)
	assert.Equal(t, "default", secret.Namespace)
	assert.Equal(t, []byte("admin"), secret.Data["username"])
}

func TestGetSecret_NotFound(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	_, err := client.GetSecret("default", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetSecret_EmptyNamespace(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	_, err := client.GetSecret("", "test-secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace cannot be empty")
}

func TestGetSecret_EmptyName(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	_, err := client.GetSecret("default", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret name cannot be empty")
}

func TestListSecrets_Success(t *testing.T) {
	// Create fake clientset with multiple secrets
	fakeClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: "default",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: "default",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret3",
				Namespace: "other-namespace",
			},
		},
	)

	client := &K8sClient{clientset: fakeClient}

	secrets, err := client.ListSecrets("default")
	require.NoError(t, err)
	require.Len(t, secrets, 2)

	names := []string{secrets[0].Name, secrets[1].Name}
	assert.Contains(t, names, "secret1")
	assert.Contains(t, names, "secret2")
}

func TestListSecrets_EmptyNamespace(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	_, err := client.ListSecrets("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace cannot be empty")
}

func TestListSecrets_EmptyResult(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	secrets, err := client.ListSecrets("empty-namespace")
	require.NoError(t, err)
	assert.Empty(t, secrets)
}

func TestSecretExists_True(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-secret",
				Namespace: "default",
			},
		},
	)

	client := &K8sClient{clientset: fakeClient}

	exists, err := client.SecretExists("default", "existing-secret")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestSecretExists_False(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	exists, err := client.SecretExists("default", "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestSecretExists_EmptyParameters(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	tests := []struct {
		name       string
		namespace  string
		secretName string
		errMsg     string
	}{
		{
			name:       "empty namespace",
			namespace:  "",
			secretName: "test",
			errMsg:     "namespace cannot be empty",
		},
		{
			name:       "empty secret name",
			namespace:  "default",
			secretName: "",
			errMsg:     "secret name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.SecretExists(tt.namespace, tt.secretName)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestDeleteSecret_Success(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "delete-me",
				Namespace: "default",
			},
		},
	)

	client := &K8sClient{clientset: fakeClient}

	err := client.DeleteSecret("default", "delete-me")
	require.NoError(t, err)

	// Verify secret was deleted
	exists, err := client.SecretExists("default", "delete-me")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestDeleteSecret_NotFound(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	// Deleting non-existent secret should not error
	err := client.DeleteSecret("default", "nonexistent")
	require.NoError(t, err)
}

func TestDeleteSecret_EmptyParameters(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	client := &K8sClient{clientset: fakeClient}

	tests := []struct {
		name       string
		namespace  string
		secretName string
		errMsg     string
	}{
		{
			name:       "empty namespace",
			namespace:  "",
			secretName: "test",
			errMsg:     "namespace cannot be empty",
		},
		{
			name:       "empty secret name",
			namespace:  "default",
			secretName: "",
			errMsg:     "secret name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.DeleteSecret(tt.namespace, tt.secretName)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}
