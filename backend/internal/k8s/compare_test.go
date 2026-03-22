package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompareSecretData_Identical(t *testing.T) {
	k8sSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
		},
	}

	gitData := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	result := CompareSecretData(k8sSecret, gitData)
	assert.True(t, result)
}

func TestCompareSecretData_DifferentValues(t *testing.T) {
	k8sSecret := &corev1.Secret{
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("different"),
		},
	}

	gitData := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	result := CompareSecretData(k8sSecret, gitData)
	assert.False(t, result)
}

func TestCompareSecretData_MissingKeyInK8s(t *testing.T) {
	k8sSecret := &corev1.Secret{
		Data: map[string][]byte{
			"username": []byte("admin"),
		},
	}

	gitData := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	result := CompareSecretData(k8sSecret, gitData)
	assert.False(t, result)
}

func TestCompareSecretData_ExtraKeyInK8s(t *testing.T) {
	k8sSecret := &corev1.Secret{
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
			"token":    []byte("extra"),
		},
	}

	gitData := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	result := CompareSecretData(k8sSecret, gitData)
	assert.False(t, result)
}

func TestCompareSecretData_NilSecret(t *testing.T) {
	gitData := map[string]string{
		"username": "admin",
	}

	result := CompareSecretData(nil, gitData)
	assert.False(t, result)
}

func TestNormalizeSecretData_Success(t *testing.T) {
	k8sSecret := &corev1.Secret{
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
		},
	}

	normalized := NormalizeSecretData(k8sSecret)

	assert.Equal(t, 2, len(normalized))
	assert.Equal(t, "admin", normalized["username"])
	assert.Equal(t, "secret123", normalized["password"])
}

func TestNormalizeSecretData_FiltersServiceAccountKeys(t *testing.T) {
	k8sSecret := &corev1.Secret{
		Data: map[string][]byte{
			"username":              []byte("admin"),
			"password":              []byte("secret123"),
			"ca.crt":                []byte("cert-data"),
			"namespace":             []byte("default"),
			"service-account-token": []byte("sa-token"),
		},
	}

	normalized := NormalizeSecretData(k8sSecret)

	// Should only contain username and password
	assert.Equal(t, 2, len(normalized))
	assert.Equal(t, "admin", normalized["username"])
	assert.Equal(t, "secret123", normalized["password"])
	assert.NotContains(t, normalized, "ca.crt")
	assert.NotContains(t, normalized, "namespace")
	assert.NotContains(t, normalized, "service-account-token")
}

func TestNormalizeSecretData_FiltersCertManagerKeys(t *testing.T) {
	k8sSecret := &corev1.Secret{
		Data: map[string][]byte{
			"username": []byte("admin"),
			"tls.crt":  []byte("cert-data"),
			"tls.key":  []byte("key-data"),
		},
	}

	normalized := NormalizeSecretData(k8sSecret)

	// Should only contain username
	assert.Equal(t, 1, len(normalized))
	assert.Equal(t, "admin", normalized["username"])
	assert.NotContains(t, normalized, "tls.crt")
	assert.NotContains(t, normalized, "tls.key")
}

func TestNormalizeSecretData_EmptySecret(t *testing.T) {
	k8sSecret := &corev1.Secret{
		Data: map[string][]byte{},
	}

	normalized := NormalizeSecretData(k8sSecret)
	assert.Empty(t, normalized)
}

func TestNormalizeSecretData_NilSecret(t *testing.T) {
	normalized := NormalizeSecretData(nil)
	assert.NotNil(t, normalized)
	assert.Empty(t, normalized)
}

func TestNormalizeSecretData_StringData(t *testing.T) {
	k8sSecret := &corev1.Secret{
		StringData: map[string]string{
			"username": "admin",
			"password": "secret123",
		},
	}

	normalized := NormalizeSecretData(k8sSecret)

	assert.Equal(t, 2, len(normalized))
	assert.Equal(t, "admin", normalized["username"])
	assert.Equal(t, "secret123", normalized["password"])
}

func TestCalculateSecretHash_Deterministic(t *testing.T) {
	data := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	hash1 := CalculateSecretHash(data)
	hash2 := CalculateSecretHash(data)

	assert.Equal(t, hash1, hash2)
	assert.NotEmpty(t, hash1)
}

func TestCalculateSecretHash_OrderIndependent(t *testing.T) {
	// Create two maps with same data but different insertion order
	data1 := map[string]string{
		"username": "admin",
		"password": "secret123",
		"token":    "abc123",
	}

	data2 := map[string]string{
		"token":    "abc123",
		"password": "secret123",
		"username": "admin",
	}

	hash1 := CalculateSecretHash(data1)
	hash2 := CalculateSecretHash(data2)

	assert.Equal(t, hash1, hash2)
}

func TestCalculateSecretHash_DifferentDataDifferentHash(t *testing.T) {
	data1 := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	data2 := map[string]string{
		"username": "admin",
		"password": "different",
	}

	hash1 := CalculateSecretHash(data1)
	hash2 := CalculateSecretHash(data2)

	assert.NotEqual(t, hash1, hash2)
}

func TestCalculateSecretHash_EmptyData(t *testing.T) {
	data := map[string]string{}

	hash := CalculateSecretHash(data)
	assert.Empty(t, hash)
}

func TestComputeDiff_NoChanges(t *testing.T) {
	gitData := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	k8sData := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	diff := ComputeDiff(gitData, k8sData)
	assert.Empty(t, diff)
}

func TestComputeDiff_MissingKeyInK8s(t *testing.T) {
	gitData := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	k8sData := map[string]string{
		"username": "admin",
	}

	diff := ComputeDiff(gitData, k8sData)
	assert.Len(t, diff, 1)
	assert.Contains(t, diff[0], "Key 'password' missing in K8s")
}

func TestComputeDiff_ExtraKeyInK8s(t *testing.T) {
	gitData := map[string]string{
		"username": "admin",
	}

	k8sData := map[string]string{
		"username": "admin",
		"token":    "extra",
	}

	diff := ComputeDiff(gitData, k8sData)
	assert.Len(t, diff, 1)
	assert.Contains(t, diff[0], "Key 'token' added in K8s (not in Git)")
}

func TestComputeDiff_ValueDiffers(t *testing.T) {
	gitData := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	k8sData := map[string]string{
		"username": "admin",
		"password": "different",
	}

	diff := ComputeDiff(gitData, k8sData)
	assert.Len(t, diff, 1)
	assert.Contains(t, diff[0], "Key 'password' value differs")
}

func TestComputeDiff_MultipleDifferences(t *testing.T) {
	gitData := map[string]string{
		"username": "admin",
		"password": "secret123",
		"apikey":   "gitkey",
	}

	k8sData := map[string]string{
		"username": "admin",
		"password": "different",
		"token":    "extra",
	}

	diff := ComputeDiff(gitData, k8sData)
	assert.Len(t, diff, 3)

	diffStr := ""
	for _, d := range diff {
		diffStr += d + " "
	}

	assert.Contains(t, diffStr, "Key 'apikey' missing in K8s")
	assert.Contains(t, diffStr, "Key 'token' added in K8s")
	assert.Contains(t, diffStr, "Key 'password' value differs")
}

func TestShouldSkipKey(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		shouldSkip bool
	}{
		{"service account ca.crt", "ca.crt", true},
		{"service account namespace", "namespace", true},
		{"service account prefix", "service-account-token", true},
		{"cert manager tls cert", "tls.crt", true},
		{"cert manager tls key", "tls.key", true},
		{"regular username", "username", false},
		{"regular password", "password", false},
		{"regular token named differently", "api-token", false},
		{"user-defined token key", "token", false}, // User secret, not K8s-injected
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipKey(tt.key)
			assert.Equal(t, tt.shouldSkip, result)
		})
	}
}
