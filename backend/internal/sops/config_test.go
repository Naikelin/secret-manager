package sops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSOPSConfig(t *testing.T) {
	tests := []struct {
		name         string
		agePublicKey string
		kmsKeyARN    string
		wantErr      bool
		wantContains []string
	}{
		{
			name:         "age only",
			agePublicKey: "age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			kmsKeyARN:    "",
			wantErr:      false,
			wantContains: []string{"age:", "encrypted_regex:", "path_regex:"},
		},
		{
			name:         "kms only",
			agePublicKey: "",
			kmsKeyARN:    "arn:aws:kms:us-east-1:123456789:key/abcd",
			wantErr:      false,
			wantContains: []string{"kms:", "encrypted_regex:", "path_regex:"},
		},
		{
			name:         "both age and kms",
			agePublicKey: "age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			kmsKeyARN:    "arn:aws:kms:us-east-1:123456789:key/abcd",
			wantErr:      false,
			wantContains: []string{"age:", "kms:", "encrypted_regex:", "path_regex:"},
		},
		{
			name:         "neither age nor kms",
			agePublicKey: "",
			kmsKeyARN:    "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GenerateSOPSConfig(tt.agePublicKey, tt.kmsKeyARN)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GenerateSOPSConfig() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GenerateSOPSConfig() unexpected error = %v", err)
				return
			}

			// Verify content contains expected fields
			for _, want := range tt.wantContains {
				if !strings.Contains(config, want) {
					t.Errorf("GenerateSOPSConfig() content missing '%s'\nGot:\n%s", want, config)
				}
			}

			// Verify encrypted_regex is present
			if !strings.Contains(config, "^(data|stringData)$") {
				t.Error("Config should contain encrypted_regex for data and stringData fields")
			}
		})
	}
}

func TestWriteSOPSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	agePublicKey := "age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

	t.Run("write new config", func(t *testing.T) {
		err := WriteSOPSConfig(tmpDir, agePublicKey, "")
		if err != nil {
			t.Fatalf("WriteSOPSConfig() error = %v", err)
		}

		// Verify file exists
		configPath := filepath.Join(tmpDir, ".sops.yaml")
		if _, err := os.Stat(configPath); err != nil {
			t.Errorf("Config file not created: %v", err)
		}

		// Verify content
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		if !strings.Contains(string(content), agePublicKey) {
			t.Error("Config file should contain Age public key")
		}
	})

	t.Run("update existing config with same content", func(t *testing.T) {
		// Write same config again
		err := WriteSOPSConfig(tmpDir, agePublicKey, "")
		if err != nil {
			t.Errorf("WriteSOPSConfig() second write error = %v", err)
		}
	})
}

func TestParseSOPSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".sops.yaml")

	// Create test config
	configContent := `creation_rules:
  - path_regex: \.yaml$
    encrypted_regex: ^(data|stringData)$
    age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := ParseSOPSConfig(configPath)
	if err != nil {
		t.Fatalf("ParseSOPSConfig() error = %v", err)
	}

	if len(config.CreationRules) != 1 {
		t.Errorf("Expected 1 creation rule, got %d", len(config.CreationRules))
	}

	rule := config.CreationRules[0]
	if rule.Age != "age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" {
		t.Errorf("Expected Age key, got %s", rule.Age)
	}

	if rule.EncryptedRegex != "^(data|stringData)$" {
		t.Errorf("Expected encrypted_regex pattern, got %s", rule.EncryptedRegex)
	}
}

func TestValidateSOPSConfig(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid age config",
			configYAML: `creation_rules:
  - path_regex: \.yaml$
    age: age1xxxxxx
`,
			wantErr: false,
		},
		{
			name: "valid kms config",
			configYAML: `creation_rules:
  - path_regex: \.yaml$
    kms: arn:aws:kms:us-east-1:123:key/abc
`,
			wantErr: false,
		},
		{
			name:        "no creation rules",
			configYAML:  `creation_rules: []`,
			wantErr:     true,
			errContains: "no creation rules",
		},
		{
			name: "no encryption keys",
			configYAML: `creation_rules:
  - path_regex: \.yaml$
`,
			wantErr:     true,
			errContains: "no encryption keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".sops.yaml")

			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			err := ValidateSOPSConfig(configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSOPSConfig() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateSOPSConfig() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateSOPSConfig() unexpected error = %v", err)
			}
		})
	}
}

func TestParseSOPSConfig_InvalidFile(t *testing.T) {
	_, err := ParseSOPSConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("ParseSOPSConfig() with nonexistent file should return error")
	}
}

func TestParseSOPSConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".sops.yaml")

	// Write invalid YAML
	invalidYAML := `this is not: valid: yaml: content`
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := ParseSOPSConfig(configPath)
	if err == nil {
		t.Error("ParseSOPSConfig() with invalid YAML should return error")
	}
}
