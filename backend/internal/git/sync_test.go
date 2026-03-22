package git

import (
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func testAuthor() *object.Signature {
	return &object.Signature{
		Name:  "Test User",
		Email: "test@example.com",
		When:  time.Now(),
	}
}

func TestGetFilePath(t *testing.T) {
	client := &GitClient{
		repoPath: "/tmp/repo",
	}

	tests := []struct {
		name       string
		namespace  string
		secretName string
		want       string
	}{
		{
			name:       "simple secret",
			namespace:  "default",
			secretName: "my-secret",
			want:       "namespaces/default/secrets/my-secret.yaml",
		},
		{
			name:       "production namespace",
			namespace:  "production",
			secretName: "db-credentials",
			want:       "namespaces/production/secrets/db-credentials.yaml",
		},
		{
			name:       "secret with dashes",
			namespace:  "dev-team",
			secretName: "api-key-v2",
			want:       "namespaces/dev-team/secrets/api-key-v2.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.GetFilePath(tt.namespace, tt.secretName)
			if got != tt.want {
				t.Errorf("GetFilePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureRepo_Clone(t *testing.T) {
	// Create a "remote" repo to clone from
	remoteDir, err := os.MkdirTemp("", "git-remote-*")
	if err != nil {
		t.Fatalf("Failed to create remote dir: %v", err)
	}
	defer os.RemoveAll(remoteDir)

	// Initialize remote repo with initial commit
	remoteRepo, err := git.PlainInit(remoteDir, false)
	if err != nil {
		t.Fatalf("Failed to init remote repo: %v", err)
	}

	// Create initial commit in remote
	w, err := remoteRepo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	if err := os.WriteFile(remoteDir+"/README.md", []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write README: %v", err)
	}

	if _, err := w.Add("README.md"); err != nil {
		t.Fatalf("Failed to add README: %v", err)
	}

	if _, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: testAuthor(),
	}); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Create a local directory for clone
	localDir, err := os.MkdirTemp("", "git-local-*")
	if err != nil {
		t.Fatalf("Failed to create local dir: %v", err)
	}
	defer os.RemoveAll(localDir)

	// Test EnsureRepo (should clone)
	client := &GitClient{
		repoPath: localDir + "/repo",
		repoURL:  remoteDir, // Use local path as URL for test
		branch:   "master",  // git.PlainInit creates master by default
	}

	err = client.EnsureRepo()
	if err != nil {
		t.Errorf("EnsureRepo() error = %v", err)
	}

	// Verify repo was cloned
	if client.repo == nil {
		t.Error("EnsureRepo() did not set repo")
	}

	// Verify README exists
	if _, err := os.Stat(client.repoPath + "/README.md"); os.IsNotExist(err) {
		t.Error("EnsureRepo() did not clone files")
	}
}

func TestEnsureRepo_ExistingRepo(t *testing.T) {
	// Create a local repo
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize repo with initial commit
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	if err := os.WriteFile(tmpDir+"/test.txt", []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if _, err := w.Add("test.txt"); err != nil {
		t.Fatalf("Failed to add test file: %v", err)
	}

	if _, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: testAuthor(),
	}); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Test EnsureRepo on existing repo
	client := &GitClient{
		repoPath: tmpDir,
		repoURL:  "https://github.com/test/repo.git", // Dummy URL
		branch:   "master",
	}

	err = client.EnsureRepo()
	// Note: Pull will fail because there's no remote, but repo should be opened
	// We only care that the repo was opened, not that pull succeeded
	if client.repo == nil {
		t.Error("EnsureRepo() did not open existing repo")
	}
}

func TestEnsureRepo_InvalidPath(t *testing.T) {
	client := &GitClient{
		repoPath: "/proc/invalid-for-git", // Path that can't be used as a git repo
		repoURL:  "https://github.com/test/repo.git",
		branch:   "main",
	}

	err := client.EnsureRepo()
	if err == nil {
		t.Error("EnsureRepo() expected error for invalid path")
	}
}
