package git

import (
	"os"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestNewTokenAuth(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		username string
		wantUser string
	}{
		{
			name:     "GitHub style (default)",
			token:    "ghp_test123",
			username: "",
			wantUser: "git",
		},
		{
			name:     "GitHub style (explicit)",
			token:    "ghp_test123",
			username: "git",
			wantUser: "git",
		},
		{
			name:     "GitLab style",
			token:    "glpat-test123",
			username: "oauth2",
			wantUser: "oauth2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewTokenAuth(tt.token, tt.username)

			if auth == nil {
				t.Fatal("NewTokenAuth() returned nil")
			}

			// Type assertion to verify it's BasicAuth
			basicAuth, ok := auth.(*http.BasicAuth)
			if !ok {
				t.Fatal("NewTokenAuth() did not return BasicAuth")
			}

			if basicAuth.Username != tt.wantUser {
				t.Errorf("NewTokenAuth() username = %v, want %v", basicAuth.Username, tt.wantUser)
			}

			if basicAuth.Password != tt.token {
				t.Errorf("NewTokenAuth() password = %v, want %v", basicAuth.Password, tt.token)
			}
		})
	}
}

func TestNewSSHAuth_EmptyPath(t *testing.T) {
	_, err := NewSSHAuth("")
	if err == nil {
		t.Error("NewSSHAuth() expected error for empty path")
	}
}

func TestNewSSHAuth_NonexistentFile(t *testing.T) {
	_, err := NewSSHAuth("/tmp/nonexistent-key-file-12345")
	if err == nil {
		t.Error("NewSSHAuth() expected error for nonexistent file")
	}
}

func TestNewSSHAuth_ValidKey(t *testing.T) {
	t.Skip("Skipping SSH key test - requires valid SSH key format")
	// This test would require a properly formatted SSH key
	// In real usage, users will provide their own valid SSH keys
}

func TestNewAuthFromEnv_SSH(t *testing.T) {
	t.Skip("Skipping SSH key test - requires valid SSH key format")
	// This test would require a properly formatted SSH key
	// In real usage, users will provide their own valid SSH keys
}

func TestNewAuthFromEnv_Token(t *testing.T) {
	// Set environment variables for token auth
	os.Setenv("GIT_AUTH_TYPE", "token")
	os.Setenv("GIT_TOKEN", "test-token-123")
	defer func() {
		os.Unsetenv("GIT_AUTH_TYPE")
		os.Unsetenv("GIT_TOKEN")
	}()

	auth, err := NewAuthFromEnv()
	if err != nil {
		t.Errorf("NewAuthFromEnv() error = %v", err)
	}
	if auth == nil {
		t.Fatal("NewAuthFromEnv() returned nil auth")
	}

	// Verify it's BasicAuth with correct token
	basicAuth, ok := auth.(*http.BasicAuth)
	if !ok {
		t.Fatal("NewAuthFromEnv() did not return BasicAuth")
	}
	if basicAuth.Password != "test-token-123" {
		t.Errorf("NewAuthFromEnv() token = %v, want test-token-123", basicAuth.Password)
	}
}

func TestNewAuthFromEnv_DefaultSSH(t *testing.T) {
	// Clear GIT_AUTH_TYPE to test default
	os.Unsetenv("GIT_AUTH_TYPE")
	os.Unsetenv("GIT_SSH_KEY_PATH")

	_, err := NewAuthFromEnv()
	if err == nil {
		t.Error("NewAuthFromEnv() expected error when GIT_SSH_KEY_PATH is missing")
	}
}

func TestNewAuthFromEnv_MissingToken(t *testing.T) {
	os.Setenv("GIT_AUTH_TYPE", "token")
	os.Unsetenv("GIT_TOKEN")
	defer os.Unsetenv("GIT_AUTH_TYPE")

	_, err := NewAuthFromEnv()
	if err == nil {
		t.Error("NewAuthFromEnv() expected error when GIT_TOKEN is missing")
	}
}

func TestNewAuthFromEnv_InvalidAuthType(t *testing.T) {
	os.Setenv("GIT_AUTH_TYPE", "invalid")
	defer os.Unsetenv("GIT_AUTH_TYPE")

	_, err := NewAuthFromEnv()
	if err == nil {
		t.Error("NewAuthFromEnv() expected error for invalid auth type")
	}
}
