package sops

import (
	"testing"
)

func TestVerifySOPSInstalled(t *testing.T) {
	version, err := VerifySOPSInstalled()

	if err != nil {
		t.Skipf("SOPS not installed: %v", err)
	}

	if version == "" {
		t.Error("VerifySOPSInstalled() returned empty version")
	}

	t.Logf("SOPS version: %s", version)
}

func TestVerifyAgeInstalled(t *testing.T) {
	version, err := VerifyAgeInstalled()

	if err != nil {
		t.Skipf("Age not installed: %v", err)
	}

	if version == "" {
		t.Error("VerifyAgeInstalled() returned empty version")
	}

	t.Logf("Age version: %s", version)
}

func TestVerifyAgeKeygenInstalled(t *testing.T) {
	err := VerifyAgeKeygenInstalled()

	if err != nil {
		t.Skipf("age-keygen not installed: %v", err)
	}

	t.Log("age-keygen is installed")
}

func TestVerifyAllDependencies(t *testing.T) {
	err := VerifyAllDependencies()

	if err != nil {
		t.Skipf("Not all dependencies installed: %v", err)
	}

	t.Log("All SOPS dependencies verified")
}
