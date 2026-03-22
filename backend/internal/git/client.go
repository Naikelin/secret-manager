package git

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/yourorg/secret-manager/pkg/logger"
)

// GitClient manages Git repository operations
type GitClient struct {
	repoPath string               // Local path to cloned repo
	repoURL  string               // Remote Git URL
	branch   string               // Default branch (main/master)
	auth     transport.AuthMethod // SSH key or token auth
	repo     *git.Repository      // go-git repository handle
	author   *object.Signature    // Commit author information
}

// NewGitClient creates a new Git client
func NewGitClient(repoPath, repoURL, branch string, auth transport.AuthMethod) (*GitClient, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}
	if repoURL == "" {
		return nil, fmt.Errorf("repository URL cannot be empty")
	}
	if branch == "" {
		branch = "main" // Default branch
	}

	return &GitClient{
		repoPath: repoPath,
		repoURL:  repoURL,
		branch:   branch,
		auth:     auth,
		author: &object.Signature{
			Name:  "Secret Manager",
			Email: "secret-manager@example.com",
			When:  time.Now(),
		},
	}, nil
}

// SetAuthor sets the commit author information
func (c *GitClient) SetAuthor(name, email string) {
	c.author = &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}
}

// RepoPath returns the local repository path
func (c *GitClient) RepoPath() string {
	return c.repoPath
}

// Clone clones the repository to repoPath
func (c *GitClient) Clone() error {
	logger.Info("Cloning repository", "url", c.repoURL, "branch", c.branch, "path", c.repoPath)

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(c.repoPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Clone repository
	repo, err := git.PlainClone(c.repoPath, false, &git.CloneOptions{
		URL:           c.repoURL,
		Auth:          c.auth,
		ReferenceName: plumbing.NewBranchReferenceName(c.branch),
		SingleBranch:  true,
		Progress:      nil, // Set to os.Stdout for debug
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	c.repo = repo
	logger.Info("Repository cloned successfully", "path", c.repoPath)
	return nil
}

// Pull fetches and merges latest changes from remote
func (c *GitClient) Pull() error {
	if c.repo == nil {
		return fmt.Errorf("repository not initialized, call Clone or EnsureRepo first")
	}

	logger.Info("Pulling latest changes", "branch", c.branch)

	// Get worktree
	w, err := c.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Pull changes
	err = w.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(c.branch),
		Auth:          c.auth,
		SingleBranch:  true,
		Progress:      nil,
	})

	// git.NoErrAlreadyUpToDate is not an error
	if err == git.NoErrAlreadyUpToDate {
		logger.Info("Repository already up-to-date")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to pull changes: %w", err)
	}

	logger.Info("Pulled latest changes successfully")
	return nil
}

// Commit stages specified files and creates a commit
func (c *GitClient) Commit(message, authorName string, files []string) (string, error) {
	if c.repo == nil {
		return "", fmt.Errorf("repository not initialized, call Clone or EnsureRepo first")
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no files to commit")
	}

	logger.Info("Creating commit", "message", message, "files", len(files))

	// Get worktree
	w, err := c.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Stage files
	for _, file := range files {
		_, err := w.Add(file)
		if err != nil {
			return "", fmt.Errorf("failed to stage file %s: %w", file, err)
		}
		logger.Debug("Staged file", "file", file)
	}

	// Update author timestamp
	author := c.author
	if authorName != "" {
		author = &object.Signature{
			Name:  authorName,
			Email: c.author.Email,
			When:  time.Now(),
		}
	} else {
		author.When = time.Now()
	}

	// Create commit
	commitHash, err := w.Commit(message, &git.CommitOptions{
		Author: author,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	commitSHA := commitHash.String()
	logger.Info("Committed changes", "sha", commitSHA, "files", len(files))
	return commitSHA, nil
}

// Push pushes commits to the remote repository
func (c *GitClient) Push() error {
	if c.repo == nil {
		return fmt.Errorf("repository not initialized, call Clone or EnsureRepo first")
	}

	logger.Info("Pushing to remote", "branch", c.branch)

	err := c.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       c.auth,
		Progress:   nil,
	})
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	logger.Info("Push successful")
	return nil
}

// GetCurrentSHA returns the current HEAD commit SHA
func (c *GitClient) GetCurrentSHA() (string, error) {
	if c.repo == nil {
		return "", fmt.Errorf("repository not initialized, call Clone or EnsureRepo first")
	}

	ref, err := c.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String(), nil
}

// FileExists checks if a file exists in the repository working tree
func (c *GitClient) FileExists(path string) (bool, error) {
	fullPath := filepath.Join(c.repoPath, path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check file existence: %w", err)
}

// WriteFile writes content to a file in the repository working tree
func (c *GitClient) WriteFile(path string, content []byte) error {
	fullPath := filepath.Join(c.repoPath, path)

	// Create directory structure if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	logger.Debug("Wrote file to repository", "path", path, "size", len(content))
	return nil
}

// ReadFile reads content from a file in the repository working tree
func (c *GitClient) ReadFile(path string) ([]byte, error) {
	fullPath := filepath.Join(c.repoPath, path)

	// Read file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	logger.Debug("Read file from repository", "path", path, "size", len(content))
	return content, nil
}
