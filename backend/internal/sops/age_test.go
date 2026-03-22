package sops

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateAgeKey(t *testing.T) {
	// Skip if age-keygen not installed
	if err := VerifyAgeKeygenInstalled(); err != nil {
		t.Skip("age-keygen not installed, skipping Age key generation tests")
	}

	publicKey, privateKey, err := GenerateAgeKey()
	if err != nil {
		t.Fatalf("GenerateAgeKey() error = %v", err)
	}

	// Verify public key format
	if !strings.HasPrefix(publicKey, "age1") {
		t.Errorf("Public key should start with 'age1', got: %s", publicKey)
	}

	// Verify private key format
	if !strings.Contains(privateKey, "AGE-SECRET-KEY-") {
		t.Error("Private key should contain 'AGE-SECRET-KEY-'")
	}

	// Verify private key contains public key comment
	if !strings.Contains(privateKey, "# public key:") {
		t.Error("Private key should contain public key comment")
	}

	// Verify public key in comment matches returned public key
	if !strings.Contains(privateKey, publicKey) {
		t.Errorf("Private key should contain public key %s", publicKey)
	}
}

func TestLoadAgePrivateKey(t *testing.T) {
	// Create test key file
	tmpFile, err := os.CreateTemp("", "test-age-key-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	validKey := `# created: 2024-01-01T00:00:00Z
# public key: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
`
	if _, err := tmpFile.WriteString(validKey); err != nil {
		t.Fatalf("Failed to write test key: %v", err)
	}
	tmpFile.Close()

	t.Run("valid key", func(t *testing.T) {
		key, err := LoadAgePrivateKey(tmpFile.Name())
		if err != nil {
			t.Errorf("LoadAgePrivateKey() error = %v", err)
		}

		if !strings.Contains(key, "AGE-SECRET-KEY-") {
			t.Error("Loaded key should contain 'AGE-SECRET-KEY-'")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := LoadAgePrivateKey("/nonexistent/key.txt")
		if err == nil {
			t.Error("LoadAgePrivateKey() with nonexistent file should return error")
		}
	})

	// Create invalid key file
	invalidFile, err := os.CreateTemp("", "invalid-age-key-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(invalidFile.Name())

	invalidKey := `this is not a valid Age key`
	if _, err := invalidFile.WriteString(invalidKey); err != nil {
		t.Fatalf("Failed to write invalid key: %v", err)
	}
	invalidFile.Close()

	t.Run("invalid key format", func(t *testing.T) {
		_, err := LoadAgePrivateKey(invalidFile.Name())
		if err == nil {
			t.Error("LoadAgePrivateKey() with invalid key should return error")
		}
		if !strings.Contains(err.Error(), "invalid Age private key format") {
			t.Errorf("Expected error about invalid format, got: %v", err)
		}
	})
}

func TestGetAgePublicKey(t *testing.T) {
	tests := []struct {
		name          string
		privateKey    string
		wantPublicKey string
		wantErr       bool
	}{
		{
			name: "valid key",
			privateKey: `# created: 2024-01-01T00:00:00Z
# public key: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`,
			wantPublicKey: "age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			wantErr:       false,
		},
		{
			name:       "missing public key comment",
			privateKey: `AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`,
			wantErr:    true,
		},
		{
			name: "invalid public key format",
			privateKey: `# created: 2024-01-01T00:00:00Z
# public key: invalid-key
AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicKey, err := GetAgePublicKey(tt.privateKey)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetAgePublicKey() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetAgePublicKey() unexpected error = %v", err)
				return
			}

			if publicKey != tt.wantPublicKey {
				t.Errorf("GetAgePublicKey() = %v, want %v", publicKey, tt.wantPublicKey)
			}
		})
	}
}

func TestSaveAgeKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := tmpDir + "/test-key.txt"

	privateKey := `# created: 2024-01-01T00:00:00Z
# public key: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
`

	err := SaveAgeKey(keyPath, privateKey)
	if err != nil {
		t.Fatalf("SaveAgeKey() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("Key file not created: %v", err)
	}

	// Verify file permissions (should be 0400 - read-only)
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Failed to stat key file: %v", err)
	}

	// Check permissions are restrictive
	mode := info.Mode().Perm()
	if mode != 0400 {
		t.Errorf("Key file permissions = %o, want 0400", mode)
	}

	// Verify content
	content, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read saved key: %v", err)
	}

	if string(content) != privateKey {
		t.Error("Saved key content doesn't match original")
	}
}

func TestValidateAgeKey(t *testing.T) {
	tests := []struct {
		name       string
		privateKey string
		wantErr    bool
	}{
		{
			name: "valid key",
			privateKey: `# created: 2024-01-01T00:00:00Z
# public key: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`,
			wantErr: false,
		},
		{
			name:       "missing AGE-SECRET-KEY prefix",
			privateKey: `# public key: age1xxx\nNOT-A-SECRET-KEY`,
			wantErr:    true,
		},
		{
			name:       "missing public key comment",
			privateKey: `AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`,
			wantErr:    true,
		},
		{
			name:       "empty key",
			privateKey: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgeKey(tt.privateKey)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateAgeKey() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateAgeKey() unexpected error = %v", err)
			}
		})
	}
}

func TestGetAgePublicKey_RealKey(t *testing.T) {
	// Skip if age-keygen not installed
	if err := VerifyAgeKeygenInstalled(); err != nil {
		t.Skip("age-keygen not installed, skipping real key test")
	}

	// Generate a real key
	pubKey, privKey, err := GenerateAgeKey()
	if err != nil {
		t.Fatalf("Failed to generate Age key: %v", err)
	}

	// Extract public key from private key
	extractedPubKey, err := GetAgePublicKey(privKey)
	if err != nil {
		t.Fatalf("GetAgePublicKey() error = %v", err)
	}

	// Verify extracted public key matches generated public key
	if extractedPubKey != pubKey {
		t.Errorf("Extracted public key %s doesn't match generated %s", extractedPubKey, pubKey)
	}
}
