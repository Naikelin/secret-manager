package sops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSOPSClient(t *testing.T) {
	tests := []struct {
		name        string
		encryptType string
		ageKeyPath  string
		kmsKeyARN   string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid age client with real key",
			encryptType: "age",
			ageKeyPath:  createTestAgeKey(t),
			kmsKeyARN:   "",
			wantErr:     false,
		},
		{
			name:        "valid kms client",
			encryptType: "kms",
			ageKeyPath:  "",
			kmsKeyARN:   "arn:aws:kms:us-east-1:123456789:key/abcd",
			wantErr:     false,
		},
		{
			name:        "invalid encrypt type",
			encryptType: "invalid",
			ageKeyPath:  "",
			kmsKeyARN:   "",
			wantErr:     true,
			errContains: "invalid encrypt type",
		},
		{
			name:        "age mode without key path",
			encryptType: "age",
			ageKeyPath:  "",
			kmsKeyARN:   "",
			wantErr:     true,
			errContains: "ageKeyPath is required",
		},
		{
			name:        "age mode with non-existent key",
			encryptType: "age",
			ageKeyPath:  "/nonexistent/key.txt",
			kmsKeyARN:   "",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "kms mode without ARN",
			encryptType: "kms",
			ageKeyPath:  "",
			kmsKeyARN:   "",
			wantErr:     true,
			errContains: "kmsKeyARN is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewSOPSClient(tt.encryptType, tt.ageKeyPath, tt.kmsKeyARN, "")

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewSOPSClient() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewSOPSClient() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NewSOPSClient() unexpected error = %v", err)
				return
			}

			if client == nil {
				t.Errorf("NewSOPSClient() returned nil client")
			}
		})
	}
}

func TestEncryptDecryptYAML(t *testing.T) {
	// Skip if SOPS or Age not installed
	if _, err := VerifySOPSInstalled(); err != nil {
		t.Skip("SOPS not installed, skipping encryption tests")
	}
	if _, err := VerifyAgeInstalled(); err != nil {
		t.Skip("Age not installed, skipping encryption tests")
	}

	// Create test Age key
	ageKeyPath := createTestAgeKey(t)
	defer os.Remove(ageKeyPath)

	// Create SOPS client
	client, err := NewSOPSClient("age", ageKeyPath, "", "")
	if err != nil {
		t.Fatalf("Failed to create SOPS client: %v", err)
	}

	// Test YAML content
	originalYAML := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: default
type: Opaque
data:
  username: YWRtaW4=
  password: c2VjcmV0MTIz
`)

	t.Run("encrypt and decrypt roundtrip", func(t *testing.T) {
		// Encrypt
		encrypted, err := client.EncryptYAML(originalYAML)
		if err != nil {
			t.Fatalf("EncryptYAML() error = %v", err)
		}

		// Verify encrypted content is different
		if string(encrypted) == string(originalYAML) {
			t.Error("Encrypted content should differ from original")
		}

		// Verify encrypted content contains SOPS metadata
		if !strings.Contains(string(encrypted), "sops:") {
			t.Error("Encrypted content should contain SOPS metadata")
		}

		// Decrypt
		decrypted, err := client.DecryptYAML(encrypted)
		if err != nil {
			t.Fatalf("DecryptYAML() error = %v", err)
		}

		// Verify decrypted matches original (normalize whitespace differences)
		// SOPS may reformat YAML with different indentation
		if !yamlEqual(originalYAML, decrypted) {
			t.Errorf("Decrypted content doesn't match original.\nWant:\n%s\nGot:\n%s",
				string(originalYAML), string(decrypted))
		}
	})
}

func TestEncryptDecryptFile(t *testing.T) {
	// Skip if SOPS or Age not installed
	if _, err := VerifySOPSInstalled(); err != nil {
		t.Skip("SOPS not installed, skipping file encryption tests")
	}
	if _, err := VerifyAgeInstalled(); err != nil {
		t.Skip("Age not installed, skipping file encryption tests")
	}

	// Create test Age key
	ageKeyPath := createTestAgeKey(t)
	defer os.Remove(ageKeyPath)

	// Create SOPS client
	client, err := NewSOPSClient("age", ageKeyPath, "", "")
	if err != nil {
		t.Fatalf("Failed to create SOPS client: %v", err)
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Test YAML content
	originalContent := `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: Opaque
data:
  key: value
`

	inputFile := filepath.Join(tmpDir, "input.yaml")
	encryptedFile := filepath.Join(tmpDir, "encrypted.yaml")
	decryptedFile := filepath.Join(tmpDir, "decrypted.yaml")

	// Write input file
	if err := os.WriteFile(inputFile, []byte(originalContent), 0600); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	t.Run("encrypt file", func(t *testing.T) {
		err := client.EncryptFile(inputFile, encryptedFile)
		if err != nil {
			t.Fatalf("EncryptFile() error = %v", err)
		}

		// Verify encrypted file exists
		if _, err := os.Stat(encryptedFile); err != nil {
			t.Errorf("Encrypted file not created: %v", err)
		}

		// Verify encrypted content is different
		encContent, _ := os.ReadFile(encryptedFile)
		if string(encContent) == originalContent {
			t.Error("Encrypted file content should differ from original")
		}
	})

	t.Run("decrypt file", func(t *testing.T) {
		err := client.DecryptFile(encryptedFile, decryptedFile)
		if err != nil {
			t.Fatalf("DecryptFile() error = %v", err)
		}

		// Verify decrypted file exists
		decContent, err := os.ReadFile(decryptedFile)
		if err != nil {
			t.Errorf("Failed to read decrypted file: %v", err)
		}

		// Verify decrypted matches original (normalize whitespace)
		if !yamlEqual([]byte(originalContent), decContent) {
			t.Errorf("Decrypted content doesn't match original.\nWant:\n%s\nGot:\n%s",
				originalContent, string(decContent))
		}
	})
}

// yamlEqual compares two YAML documents for semantic equality, ignoring formatting differences
func yamlEqual(a, b []byte) bool {
	// Simple comparison: both should contain same non-whitespace content
	normalizeYAML := func(data []byte) string {
		lines := strings.Split(string(data), "\n")
		var result []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return strings.Join(result, "\n")
	}

	return normalizeYAML(a) == normalizeYAML(b)
}

// createTestAgeKey creates a temporary Age key for testing
func createTestAgeKey(t *testing.T) string {
	t.Helper()

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-age-key-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	// Generate Age key
	pubKey, privKey, err := GenerateAgeKey()
	if err != nil {
		t.Fatalf("Failed to generate Age key: %v", err)
	}

	// Write private key to file
	if err := os.WriteFile(tmpFile.Name(), []byte(privKey), 0600); err != nil {
		t.Fatalf("Failed to write Age key: %v", err)
	}

	t.Logf("Created test Age key: %s (public: %s)", tmpFile.Name(), pubKey)

	return tmpFile.Name()
}

func TestEncryptFile_NonExistentInput(t *testing.T) {
	ageKeyPath := createTestAgeKey(t)
	defer os.Remove(ageKeyPath)

	client, err := NewSOPSClient("age", ageKeyPath, "", "")
	if err != nil {
		t.Fatalf("Failed to create SOPS client: %v", err)
	}

	err = client.EncryptFile("/nonexistent/file.yaml", "/tmp/output.yaml")
	if err == nil {
		t.Error("EncryptFile() with non-existent input should return error")
	}
}

func TestDecryptFile_NonExistentInput(t *testing.T) {
	ageKeyPath := createTestAgeKey(t)
	defer os.Remove(ageKeyPath)

	client, err := NewSOPSClient("age", ageKeyPath, "", "")
	if err != nil {
		t.Fatalf("Failed to create SOPS client: %v", err)
	}

	err = client.DecryptFile("/nonexistent/file.yaml", "/tmp/output.yaml")
	if err == nil {
		t.Error("DecryptFile() with non-existent input should return error")
	}
}
