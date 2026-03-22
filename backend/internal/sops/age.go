package sops

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GenerateAgeKey generates a new Age key pair using age-keygen
// Returns public key, private key content, and error
func GenerateAgeKey() (publicKey, privateKey string, err error) {
	// Check if age-keygen is installed
	if _, err := exec.LookPath("age-keygen"); err != nil {
		return "", "", fmt.Errorf("age-keygen not found in PATH: %w", err)
	}

	// Create temporary file for key generation
	tmpFile, err := os.CreateTemp("", "age-key-*.txt")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	// Remove temp file immediately so age-keygen can create it
	os.Remove(tmpPath)
	defer os.Remove(tmpPath) // Cleanup after reading

	// Generate Age key
	cmd := exec.Command("age-keygen", "-o", tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("age-keygen failed: %w\nOutput: %s", err, string(output))
	}

	// Read generated key file
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read generated key: %w", err)
	}

	privateKey = string(content)

	// Extract public key from private key content
	publicKey, err = GetAgePublicKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract public key: %w", err)
	}

	return publicKey, privateKey, nil
}

// LoadAgePrivateKey reads an Age private key from file and validates format
func LoadAgePrivateKey(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read Age key file: %w", err)
	}

	keyContent := string(content)

	// Validate format - must contain AGE-SECRET-KEY-
	if !strings.Contains(keyContent, "AGE-SECRET-KEY-") {
		return "", fmt.Errorf("invalid Age private key format (missing AGE-SECRET-KEY-)")
	}

	return keyContent, nil
}

// GetAgePublicKey extracts the public key from Age private key content
// Age key files have the format:
//
//	# created: 2024-03-22T10:00:00Z
//	# public key: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
//	AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
func GetAgePublicKey(privateKeyContent string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(privateKeyContent))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Look for "# public key: age1..."
		if strings.HasPrefix(line, "# public key:") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				publicKey := parts[3] // "# public key: age1..."

				// Validate public key format
				if !strings.HasPrefix(publicKey, "age1") {
					return "", fmt.Errorf("invalid public key format: %s", publicKey)
				}

				return publicKey, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading key content: %w", err)
	}

	return "", fmt.Errorf("public key not found in Age key file")
}

// SaveAgeKey saves an Age private key to a file with secure permissions
func SaveAgeKey(path, privateKeyContent string) error {
	// Write file with secure permissions (owner read-only)
	if err := os.WriteFile(path, []byte(privateKeyContent), 0400); err != nil {
		return fmt.Errorf("failed to write Age key file: %w", err)
	}

	return nil
}

// ValidateAgeKey validates that a string is a valid Age private key
func ValidateAgeKey(privateKeyContent string) error {
	// Check for required prefix
	if !strings.Contains(privateKeyContent, "AGE-SECRET-KEY-") {
		return fmt.Errorf("invalid Age private key: missing AGE-SECRET-KEY- prefix")
	}

	// Try to extract public key (validates format)
	if _, err := GetAgePublicKey(privateKeyContent); err != nil {
		return fmt.Errorf("invalid Age key format: %w", err)
	}

	return nil
}
