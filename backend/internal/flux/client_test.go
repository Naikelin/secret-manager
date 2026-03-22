package flux

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKustomization(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid namespace",
			namespace: "development",
			wantErr:   false,
		},
		{
			name:      "empty namespace should error",
			namespace: "",
			wantErr:   true,
			errMsg:    "namespace cannot be empty",
		},
		{
			name:      "production namespace",
			namespace: "production",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml, err := GenerateKustomization(tt.namespace)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, yaml)

			// Verify YAML contains expected values
			yamlStr := string(yaml)
			assert.Contains(t, yamlStr, "apiVersion: kustomize.toolkit.fluxcd.io/v1")
			assert.Contains(t, yamlStr, "kind: Kustomization")
			assert.Contains(t, yamlStr, "name: secrets-"+tt.namespace)
			assert.Contains(t, yamlStr, "namespace: flux-system")
			assert.Contains(t, yamlStr, "path: ./namespaces/"+tt.namespace+"/secrets")
			assert.Contains(t, yamlStr, "interval: 1m")
			assert.Contains(t, yamlStr, "prune: true")
			assert.Contains(t, yamlStr, "kind: GitRepository")
			assert.Contains(t, yamlStr, "name: secrets-repo")
			assert.Contains(t, yamlStr, "provider: sops")
			assert.Contains(t, yamlStr, "name: sops-age")
		})
	}
}

func TestExtractCommitSHA(t *testing.T) {
	tests := []struct {
		name     string
		revision string
		want     string
	}{
		{
			name:     "revision with @sha1 format",
			revision: "main@sha1:abc123def456",
			want:     "abc123def456",
		},
		{
			name:     "revision with / format",
			revision: "main/abc123def456",
			want:     "abc123def456",
		},
		{
			name:     "plain commit SHA",
			revision: "abc123def456",
			want:     "abc123def456",
		},
		{
			name:     "empty revision",
			revision: "",
			want:     "",
		},
		{
			name:     "branch with long path",
			revision: "feature/test/abc123",
			want:     "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCommitSHA(tt.revision)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewFluxClient_InvalidKubeconfig(t *testing.T) {
	// Test with invalid kubeconfig path
	_, err := NewFluxClient("/nonexistent/kubeconfig")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build kubernetes config")
}

func TestKustomizationManifestStructure(t *testing.T) {
	// Test that the manifest structure is correct
	yaml, err := GenerateKustomization("test-namespace")
	require.NoError(t, err)

	yamlStr := string(yaml)

	// Verify indentation and structure
	assert.Contains(t, yamlStr, "metadata:")
	assert.Contains(t, yamlStr, "spec:")
	assert.Contains(t, yamlStr, "sourceRef:")
	assert.Contains(t, yamlStr, "decryption:")
	assert.Contains(t, yamlStr, "secretRef:")
}
