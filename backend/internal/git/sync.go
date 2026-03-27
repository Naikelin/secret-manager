package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/yourorg/secret-manager/pkg/logger"
)

// EnsureRepo ensures the repository exists locally
// Clones if it doesn't exist, pulls if it does
func (c *GitClient) EnsureRepo() error {
	// Check if .git directory exists inside repo path
	gitDir := filepath.Join(c.repoPath, ".git")
	_, err := os.Stat(gitDir)
	if os.IsNotExist(err) {
		// .git doesn't exist - need to clone
		logger.Info("Repository not found locally, cloning", "path", c.repoPath)
		return c.Clone()
	}
	if err != nil {
		return fmt.Errorf("failed to check .git directory: %w", err)
	}

	// Repository exists - try to open it
	logger.Info("Opening existing repository", "path", c.repoPath)
	repo, err := git.PlainOpen(c.repoPath)
	if err != nil {
		// If open fails, try to clone instead
		logger.Warn("Failed to open existing repository, will clone fresh", "path", c.repoPath, "error", err)
		// Remove corrupted directory
		if err := os.RemoveAll(c.repoPath); err != nil {
			return fmt.Errorf("failed to remove corrupted repository: %w", err)
		}
		return c.Clone()
	}

	c.repo = repo

	// Verify we're on the correct branch
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	expectedRef := plumbing.NewBranchReferenceName(c.branch)
	if head.Name() != expectedRef {
		logger.Warn("Repository on different branch", "current", head.Name().Short(), "expected", c.branch)
		// Try to checkout the correct branch
		w, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %w", err)
		}

		err = w.Checkout(&git.CheckoutOptions{
			Branch: expectedRef,
		})
		if err != nil {
			return fmt.Errorf("failed to checkout branch %s: %w", c.branch, err)
		}
		logger.Info("Checked out branch", "branch", c.branch)
	}

	// Pull latest changes
	return c.Pull()
}

// GetFilePath returns the standardized path for a secret file in the repository
// Format: clusters/{cluster}/namespaces/{namespace}/secrets/{secretName}.yaml
func (c *GitClient) GetFilePath(clusterName, namespace, secretName string) string {
	return filepath.Join("clusters", clusterName, "namespaces", namespace, "secrets", fmt.Sprintf("%s.yaml", secretName))
}

// GetFilePathLegacy returns the old flat path format for backward compatibility
// Format: namespaces/{namespace}/secrets/{secretName}.yaml
// TODO: Remove after migration complete
func (c *GitClient) GetFilePathLegacy(namespace, secretName string) string {
	return filepath.Join("namespaces", namespace, "secrets", fmt.Sprintf("%s.yaml", secretName))
}

// ReadFileWithFallback tries to read from new cluster-first path, falls back to legacy path
// Returns: content, pathUsed, error
func (c *GitClient) ReadFileWithFallback(clusterName, namespace, secretName string) ([]byte, string, error) {
	// Try new cluster-first path first
	newPath := c.GetFilePath(clusterName, namespace, secretName)
	content, err := c.ReadFile(newPath)
	if err == nil {
		logger.Debug("[DualPath] Read from new path", "path", newPath)
		return content, newPath, nil
	}

	// Fallback to legacy path
	oldPath := c.GetFilePathLegacy(namespace, secretName)
	content, err = c.ReadFile(oldPath)
	if err == nil {
		logger.Info("[DualPath] Fallback to legacy path (TODO: migrate this secret)", "path", oldPath)
		return content, oldPath, nil
	}

	// Neither path exists
	return nil, "", fmt.Errorf("secret not found in new path (%s) or legacy path (%s)", newPath, oldPath)
}
