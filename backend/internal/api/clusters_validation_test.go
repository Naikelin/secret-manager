package api

import (
	"testing"
)

func TestValidateKubeconfigPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			path:    "configs/cluster1.yaml",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  "path cannot be empty",
		},
		{
			name:    "whitespace only",
			path:    "   ",
			wantErr: true,
			errMsg:  "path cannot be empty",
		},
		{
			name:    "absolute path with slash",
			path:    "/etc/kubeconfig",
			wantErr: true,
			errMsg:  "path must be relative, not absolute",
		},
		{
			name:    "directory traversal with ..",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "path contains directory traversal sequence",
		},
		{
			name:    "directory traversal in middle",
			path:    "configs/../../../etc/passwd",
			wantErr: true,
			errMsg:  "path contains directory traversal sequence",
		},
		{
			name:    "null byte injection",
			path:    "config\x00.yaml",
			wantErr: true,
			errMsg:  "path contains null byte",
		},
		{
			name:    "valid nested path",
			path:    "clusters/prod/us-east-1.yaml",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKubeconfigPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateKubeconfigPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateKubeconfigPath() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateEnvironment(t *testing.T) {
	validEnvironments := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  true,
	}

	tests := []struct {
		name        string
		environment string
		wantValid   bool
	}{
		{"development is valid", "development", true},
		{"staging is valid", "staging", true},
		{"production is valid", "production", true},
		{"test is invalid", "test", false},
		{"prod is invalid", "prod", false},
		{"empty is invalid", "", false},
		{"PRODUCTION is invalid (case sensitive)", "PRODUCTION", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validEnvironments[tt.environment]
			if isValid != tt.wantValid {
				t.Errorf("environment %q validity = %v, want %v", tt.environment, isValid, tt.wantValid)
			}
		})
	}
}
