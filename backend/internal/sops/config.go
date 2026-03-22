package sops

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// SOPSConfig represents the structure of .sops.yaml configuration file
type SOPSConfig struct {
	CreationRules []CreationRule `yaml:"creation_rules"`
}

// CreationRule defines encryption rules for matching files
type CreationRule struct {
	PathRegex      string `yaml:"path_regex"`
	EncryptedRegex string `yaml:"encrypted_regex"`
	Age            string `yaml:"age,omitempty"`
	KMSARN         string `yaml:"kms,omitempty"`
}

// GenerateSOPSConfig generates .sops.yaml configuration content
// Uses encrypted_regex to only encrypt secret values (data/stringData fields)
// This keeps Kubernetes Secret structure visible for debugging
func GenerateSOPSConfig(agePublicKey, kmsKeyARN string) (string, error) {
	if agePublicKey == "" && kmsKeyARN == "" {
		return "", fmt.Errorf("at least one of agePublicKey or kmsKeyARN must be provided")
	}

	config := SOPSConfig{
		CreationRules: []CreationRule{
			{
				PathRegex:      `\.yaml$`,
				EncryptedRegex: `^(data|stringData)$`,
			},
		},
	}

	// Add Age key if provided
	if agePublicKey != "" {
		config.CreationRules[0].Age = agePublicKey
	}

	// Add KMS ARN if provided
	if kmsKeyARN != "" {
		config.CreationRules[0].KMSARN = kmsKeyARN
	}

	// Marshal to YAML
	yamlContent, err := yaml.Marshal(&config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal SOPS config: %w", err)
	}

	return string(yamlContent), nil
}

// WriteSOPSConfig writes .sops.yaml to repository root
// Creates if doesn't exist, updates if exists
func WriteSOPSConfig(repoPath, agePublicKey, kmsKeyARN string) error {
	// Generate config content
	content, err := GenerateSOPSConfig(agePublicKey, kmsKeyARN)
	if err != nil {
		return err
	}

	// Path to .sops.yaml
	configPath := fmt.Sprintf("%s/.sops.yaml", strings.TrimSuffix(repoPath, "/"))

	// Check if file exists
	existingContent, err := os.ReadFile(configPath)
	if err == nil {
		// File exists - check if it needs updating
		if string(existingContent) == content {
			// No changes needed
			return nil
		}
	}

	// Write config file
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write .sops.yaml: %w", err)
	}

	return nil
}

// ParseSOPSConfig reads and parses .sops.yaml file
func ParseSOPSConfig(configPath string) (*SOPSConfig, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .sops.yaml: %w", err)
	}

	var config SOPSConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("failed to parse .sops.yaml: %w", err)
	}

	return &config, nil
}

// ValidateSOPSConfig validates that .sops.yaml has required encryption rules
func ValidateSOPSConfig(configPath string) error {
	config, err := ParseSOPSConfig(configPath)
	if err != nil {
		return err
	}

	if len(config.CreationRules) == 0 {
		return fmt.Errorf(".sops.yaml has no creation rules defined")
	}

	// Check that at least one rule has encryption key
	hasKey := false
	for _, rule := range config.CreationRules {
		if rule.Age != "" || rule.KMSARN != "" {
			hasKey = true
			break
		}
	}

	if !hasKey {
		return fmt.Errorf(".sops.yaml has no encryption keys configured")
	}

	return nil
}
