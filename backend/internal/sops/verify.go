package sops

import (
	"fmt"
	"os/exec"
	"strings"
)

// VerifySOPSInstalled checks if SOPS binary is installed and returns version
func VerifySOPSInstalled() (version string, err error) {
	// Check if sops is in PATH
	sopsPath, err := exec.LookPath("sops")
	if err != nil {
		return "", fmt.Errorf("SOPS not found in PATH: %w", err)
	}

	// Get version
	cmd := exec.Command(sopsPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get SOPS version: %w", err)
	}

	// Parse version from output
	// SOPS outputs: "sops 3.7.3 (latest)"
	versionStr := strings.TrimSpace(string(output))
	parts := strings.Fields(versionStr)
	if len(parts) >= 2 {
		version = parts[1] // Extract version number
	} else {
		version = versionStr
	}

	return version, nil
}

// VerifyAgeInstalled checks if Age binary is installed and returns version
func VerifyAgeInstalled() (version string, err error) {
	// Check if age is in PATH
	agePath, err := exec.LookPath("age")
	if err != nil {
		return "", fmt.Errorf("Age not found in PATH: %w", err)
	}

	// Get version
	cmd := exec.Command(agePath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get Age version: %w", err)
	}

	// Parse version from output
	// Age outputs: "v1.1.1"
	versionStr := strings.TrimSpace(string(output))

	return versionStr, nil
}

// VerifyAgeKeygenInstalled checks if age-keygen is available
func VerifyAgeKeygenInstalled() error {
	_, err := exec.LookPath("age-keygen")
	if err != nil {
		return fmt.Errorf("age-keygen not found in PATH: %w", err)
	}
	return nil
}

// VerifyAllDependencies checks that all required binaries are installed
func VerifyAllDependencies() error {
	// Check SOPS
	sopsVersion, err := VerifySOPSInstalled()
	if err != nil {
		return fmt.Errorf("SOPS verification failed: %w", err)
	}

	// Check Age
	ageVersion, err := VerifyAgeInstalled()
	if err != nil {
		return fmt.Errorf("Age verification failed: %w", err)
	}

	// Check age-keygen
	if err := VerifyAgeKeygenInstalled(); err != nil {
		return fmt.Errorf("age-keygen verification failed: %w", err)
	}

	// All dependencies verified
	_ = sopsVersion // Logged elsewhere if needed
	_ = ageVersion

	return nil
}
