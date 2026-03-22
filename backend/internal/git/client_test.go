package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/yourorg/secret-manager/pkg/logger"
)

func init() {
	// Initialize logger for tests
	logger.Init("error") // Use error level to reduce test noise
}

func TestNewGitClient(t *testing.T) {
	tests := []struct {
		name       string
		repoPath   string
		repoURL    string
		branch     string
		wantErr    bool
		wantBranch string
	}{
		{
			name:       "valid client with all params",
			repoPath:   "/tmp/repo",
			repoURL:    "https://github.com/test/repo.git",
			branch:     "main",
			wantErr:    false,
			wantBranch: "main",
		},
		{
			name:       "valid client with default branch",
			repoPath:   "/tmp/repo",
			repoURL:    "https://github.com/test/repo.git",
			branch:     "",
			wantErr:    false,
			wantBranch: "main",
		},
		{
			name:     "empty repo path",
			repoPath: "",
			repoURL:  "https://github.com/test/repo.git",
			branch:   "main",
			wantErr:  true,
		},
		{
			name:     "empty repo URL",
			repoPath: "/tmp/repo",
			repoURL:  "",
			branch:   "main",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewGitClient(tt.repoPath, tt.repoURL, tt.branch, nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewGitClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if client == nil {
					t.Fatal("NewGitClient() returned nil client")
				}
				if client.branch != tt.wantBranch {
					t.Errorf("NewGitClient() branch = %v, want %v", client.branch, tt.wantBranch)
				}
				if client.repoPath != tt.repoPath {
					t.Errorf("NewGitClient() repoPath = %v, want %v", client.repoPath, tt.repoPath)
				}
				if client.repoURL != tt.repoURL {
					t.Errorf("NewGitClient() repoURL = %v, want %v", client.repoURL, tt.repoURL)
				}
			}
		})
	}
}

func TestSetAuthor(t *testing.T) {
	client, err := NewGitClient("/tmp/repo", "https://github.com/test/repo.git", "main", nil)
	if err != nil {
		t.Fatalf("NewGitClient() failed: %v", err)
	}

	name := "Test User"
	email := "test@example.com"
	client.SetAuthor(name, email)

	if client.author.Name != name {
		t.Errorf("SetAuthor() name = %v, want %v", client.author.Name, name)
	}
	if client.author.Email != email {
		t.Errorf("SetAuthor() email = %v, want %v", client.author.Email, email)
	}
}

func TestGetCurrentSHA(t *testing.T) {
	// Create a temporary directory for the test repo
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repo
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create a file and commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	_, err = w.Add("test.txt")
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	commitHash, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Test GetCurrentSHA
	client := &GitClient{
		repoPath: tmpDir,
		repo:     repo,
	}

	sha, err := client.GetCurrentSHA()
	if err != nil {
		t.Errorf("GetCurrentSHA() error = %v", err)
	}
	if sha != commitHash.String() {
		t.Errorf("GetCurrentSHA() = %v, want %v", sha, commitHash.String())
	}
}

func TestGetCurrentSHA_NoRepo(t *testing.T) {
	client := &GitClient{
		repoPath: "/tmp/nonexistent",
	}

	_, err := client.GetCurrentSHA()
	if err == nil {
		t.Error("GetCurrentSHA() expected error when repo is nil")
	}
}

func TestFileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	client := &GitClient{
		repoPath: tmpDir,
	}

	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name:    "file exists",
			path:    "test.txt",
			want:    true,
			wantErr: false,
		},
		{
			name:    "file does not exist",
			path:    "nonexistent.txt",
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := client.FileExists(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if exists != tt.want {
				t.Errorf("FileExists() = %v, want %v", exists, tt.want)
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client := &GitClient{
		repoPath: tmpDir,
	}

	tests := []struct {
		name    string
		path    string
		content []byte
		wantErr bool
	}{
		{
			name:    "write simple file",
			path:    "test.txt",
			content: []byte("test content"),
			wantErr: false,
		},
		{
			name:    "write file in subdirectory",
			path:    "subdir/test.txt",
			content: []byte("test content"),
			wantErr: false,
		},
		{
			name:    "write file with nested subdirectories",
			path:    "a/b/c/test.txt",
			content: []byte("nested content"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.WriteFile(tt.path, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify file was written
				fullPath := filepath.Join(tmpDir, tt.path)
				content, err := os.ReadFile(fullPath)
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
					return
				}
				if string(content) != string(tt.content) {
					t.Errorf("WriteFile() wrote %v, want %v", string(content), string(tt.content))
				}
			}
		})
	}
}

func TestCommit(t *testing.T) {
	// Create a temporary directory for the test repo
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repo
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	client := &GitClient{
		repoPath: tmpDir,
		repo:     repo,
		author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}

	// Write a test file
	if err := client.WriteFile("test.txt", []byte("test content")); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Test commit
	sha, err := client.Commit("Test commit", "", []string{"test.txt"})
	if err != nil {
		t.Errorf("Commit() error = %v", err)
	}
	if sha == "" {
		t.Error("Commit() returned empty SHA")
	}

	// Verify commit exists
	currentSHA, err := client.GetCurrentSHA()
	if err != nil {
		t.Errorf("GetCurrentSHA() error = %v", err)
	}
	if currentSHA != sha {
		t.Errorf("Commit SHA mismatch: got %v, want %v", currentSHA, sha)
	}
}

func TestCommit_NoFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	client := &GitClient{
		repoPath: tmpDir,
		repo:     repo,
	}

	_, err = client.Commit("Test commit", "", []string{})
	if err == nil {
		t.Error("Commit() expected error when no files provided")
	}
}

func TestCommit_NoRepo(t *testing.T) {
	client := &GitClient{
		repoPath: "/tmp/nonexistent",
	}

	_, err := client.Commit("Test commit", "", []string{"test.txt"})
	if err == nil {
		t.Error("Commit() expected error when repo is nil")
	}
}
