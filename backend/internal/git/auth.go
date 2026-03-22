package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// NewSSHAuth creates SSH authentication from a private key file
func NewSSHAuth(privateKeyPath string) (transport.AuthMethod, error) {
	if privateKeyPath == "" {
		return nil, fmt.Errorf("SSH private key path is empty")
	}

	// Check if file exists
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("SSH private key file not found: %s", privateKeyPath)
	}

	// Load SSH private key
	auth, err := ssh.NewPublicKeysFromFile("git", privateKeyPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key from %s: %w", privateKeyPath, err)
	}

	return auth, nil
}

// NewTokenAuth creates token-based authentication (for GitHub/GitLab)
// username should be "git" for GitHub, "oauth2" for GitLab
func NewTokenAuth(token, username string) transport.AuthMethod {
	if username == "" {
		username = "git" // Default to GitHub convention
	}
	return &http.BasicAuth{
		Username: username,
		Password: token,
	}
}

// NewAuthFromEnv creates authentication method from environment variables
// Reads GIT_AUTH_TYPE (ssh/token), GIT_SSH_KEY_PATH, or GIT_TOKEN
func NewAuthFromEnv() (transport.AuthMethod, error) {
	authType := os.Getenv("GIT_AUTH_TYPE")
	if authType == "" {
		authType = "ssh" // Default
	}

	switch authType {
	case "ssh":
		keyPath := os.Getenv("GIT_SSH_KEY_PATH")
		if keyPath == "" {
			return nil, fmt.Errorf("GIT_SSH_KEY_PATH environment variable is required for SSH auth")
		}
		return NewSSHAuth(keyPath)

	case "token":
		token := os.Getenv("GIT_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("GIT_TOKEN environment variable is required for token auth")
		}
		// Use "git" as default username for GitHub
		return NewTokenAuth(token, "git"), nil

	default:
		return nil, fmt.Errorf("unsupported GIT_AUTH_TYPE: %s (must be 'ssh' or 'token')", authType)
	}
}
