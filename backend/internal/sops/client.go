package sops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SOPSClient wraps SOPS binary operations for encrypting/decrypting secrets
type SOPSClient struct {
	ageKeyPath  string // Path to Age private key (dev mode)
	kmsKeyARN   string // AWS KMS key ARN (prod mode)
	encryptType string // "age" or "kms"
	configPath  string // Path to .sops.yaml config file
}

// NewSOPSClient creates a new SOPS client with the specified configuration
func NewSOPSClient(encryptType, ageKeyPath, kmsKeyARN, configPath string) (*SOPSClient, error) {
	// Validate encrypt type
	if encryptType != "age" && encryptType != "kms" {
		return nil, fmt.Errorf("invalid encrypt type: %s (must be 'age' or 'kms')", encryptType)
	}

	// Validate that required keys are present
	if encryptType == "age" {
		if ageKeyPath == "" {
			return nil, fmt.Errorf("ageKeyPath is required when encryptType is 'age'")
		}
		// Check if Age key file exists
		if _, err := os.Stat(ageKeyPath); err != nil {
			return nil, fmt.Errorf("Age key file not found at %s: %w", ageKeyPath, err)
		}
	} else if encryptType == "kms" && kmsKeyARN == "" {
		return nil, fmt.Errorf("kmsKeyARN is required when encryptType is 'kms'")
	}

	return &SOPSClient{
		ageKeyPath:  ageKeyPath,
		kmsKeyARN:   kmsKeyARN,
		encryptType: encryptType,
		configPath:  configPath,
	}, nil
}

// EncryptFile encrypts a file using SOPS
// Uses .sops.yaml config if exists, otherwise falls back to explicit key specification
func (c *SOPSClient) EncryptFile(inputPath, outputPath string) error {
	// Check if input file exists
	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("input file not found at %s: %w", inputPath, err)
	}

	// Build SOPS command
	args := []string{
		"-e", // encrypt
		"--input-type", "yaml",
		"--output-type", "yaml",
	}

	// Add encryption key based on type
	configFound := false
	if c.configPath != "" {
		// Check if .sops.yaml exists
		if _, err := os.Stat(c.configPath); err == nil {
			args = append(args, "--config", c.configPath)
			configFound = true
		}
	}

	// If no config, specify key explicitly
	if !configFound {
		if c.encryptType == "age" {
			// Extract public key from Age private key
			privKeyContent, err := LoadAgePrivateKey(c.ageKeyPath)
			if err != nil {
				return fmt.Errorf("failed to load Age key: %w", err)
			}
			pubKey, err := GetAgePublicKey(privKeyContent)
			if err != nil {
				return fmt.Errorf("failed to extract Age public key: %w", err)
			}
			args = append(args, "--age", pubKey)
		} else if c.encryptType == "kms" {
			args = append(args, "--kms", c.kmsKeyARN)
		}
	}

	args = append(args, "--output", outputPath, inputPath)

	cmd := exec.Command("sops", args...)

	// Set Age key file environment variable for encryption
	if c.encryptType == "age" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("SOPS_AGE_KEY_FILE=%s", c.ageKeyPath))
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sops encryption failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// DecryptFile decrypts a SOPS-encrypted file
// Used for testing/validation and drift detection
func (c *SOPSClient) DecryptFile(inputPath, outputPath string) error {
	// Check if input file exists
	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("input file not found at %s: %w", inputPath, err)
	}

	// Build SOPS command
	args := []string{
		"-d", // decrypt
		"--input-type", "yaml",
		"--output-type", "yaml",
		"--output", outputPath,
		inputPath,
	}

	cmd := exec.Command("sops", args...)

	// Set Age key file environment variable for decryption
	if c.encryptType == "age" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("SOPS_AGE_KEY_FILE=%s", c.ageKeyPath))
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sops decryption failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// EncryptYAML encrypts YAML content in-memory without temp files
// Uses temporary files internally but cleans them up
func (c *SOPSClient) EncryptYAML(yamlContent []byte) ([]byte, error) {
	// Create temporary directory for operation
	tmpDir, err := os.MkdirTemp("", "sops-encrypt-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Cleanup

	// Write input to temp file
	inputPath := filepath.Join(tmpDir, "input.yaml")
	if err := os.WriteFile(inputPath, yamlContent, 0600); err != nil {
		return nil, fmt.Errorf("failed to write temp input file: %w", err)
	}

	// Encrypt to temp output file
	outputPath := filepath.Join(tmpDir, "output.yaml")
	if err := c.EncryptFile(inputPath, outputPath); err != nil {
		return nil, err
	}

	// Read encrypted result
	encryptedContent, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted output: %w", err)
	}

	return encryptedContent, nil
}

// DecryptYAML decrypts YAML content in-memory
// Uses temporary files internally but cleans them up
func (c *SOPSClient) DecryptYAML(encryptedYAML []byte) ([]byte, error) {
	// Create temporary directory for operation
	tmpDir, err := os.MkdirTemp("", "sops-decrypt-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Cleanup

	// Write encrypted input to temp file
	inputPath := filepath.Join(tmpDir, "input.yaml")
	if err := os.WriteFile(inputPath, encryptedYAML, 0600); err != nil {
		return nil, fmt.Errorf("failed to write temp input file: %w", err)
	}

	// Decrypt to temp output file
	outputPath := filepath.Join(tmpDir, "output.yaml")
	if err := c.DecryptFile(inputPath, outputPath); err != nil {
		return nil, err
	}

	// Read decrypted result
	decryptedContent, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted output: %w", err)
	}

	return decryptedContent, nil
}
