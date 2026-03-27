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
		name        string
		clusterName string
		namespace   string
		secretName  string
		want        string
	}{
		{
			name:        "simple secret in devops cluster",
			clusterName: "devops",
			namespace:   "default",
			secretName:  "my-secret",
			want:        "clusters/devops/namespaces/default/secrets/my-secret.yaml",
		},
		{
			name:        "production namespace in prod cluster",
			clusterName: "production",
			namespace:   "production",
			secretName:  "db-credentials",
			want:        "clusters/production/namespaces/production/secrets/db-credentials.yaml",
		},
		{
			name:        "secret with dashes in dev cluster",
			clusterName: "dev-team",
			namespace:   "development",
			secretName:  "api-key-v2",
			want:        "clusters/dev-team/namespaces/development/secrets/api-key-v2.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.GetFilePath(tt.clusterName, tt.namespace, tt.secretName)
			if got != tt.want {
				t.Errorf("GetFilePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFilePathLegacy(t *testing.T) {
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
			name:       "legacy flat structure",
			namespace:  "default",
			secretName: "my-secret",
			want:       "namespaces/default/secrets/my-secret.yaml",
		},
		{
			name:       "development namespace",
			namespace:  "development",
			secretName: "db-credentials",
			want:       "namespaces/development/secrets/db-credentials.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.GetFilePathLegacy(tt.namespace, tt.secretName)
			if got != tt.want {
				t.Errorf("GetFilePathLegacy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadFileWithFallback(t *testing.T) {
	// Create test repository
	tmpDir, err := os.MkdirTemp("", "git-fallback-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client := &GitClient{
		repoPath: tmpDir,
	}

	t.Run("reads from new cluster-first path when it exists", func(t *testing.T) {
		// Create new path structure
		newPath := tmpDir + "/clusters/docker-desktop/namespaces/default/secrets"
		if err := os.MkdirAll(newPath, 0755); err != nil {
			t.Fatalf("Failed to create new path: %v", err)
		}

		secretContent := []byte("secret: value")
		if err := os.WriteFile(newPath+"/test-secret.yaml", secretContent, 0644); err != nil {
			t.Fatalf("Failed to write secret: %v", err)
		}

		content, pathUsed, err := client.ReadFileWithFallback("docker-desktop", "default", "test-secret")
		if err != nil {
			t.Errorf("ReadFileWithFallback() error = %v", err)
		}

		expectedPath := "clusters/docker-desktop/namespaces/default/secrets/test-secret.yaml"
		if pathUsed != expectedPath {
			t.Errorf("ReadFileWithFallback() pathUsed = %v, want %v", pathUsed, expectedPath)
		}

		if string(content) != string(secretContent) {
			t.Errorf("ReadFileWithFallback() content = %v, want %v", content, secretContent)
		}
	})

	t.Run("falls back to legacy path when new path doesn't exist", func(t *testing.T) {
		// Create only legacy path
		legacyPath := tmpDir + "/namespaces/production/secrets"
		if err := os.MkdirAll(legacyPath, 0755); err != nil {
			t.Fatalf("Failed to create legacy path: %v", err)
		}

		secretContent := []byte("legacy: secret")
		if err := os.WriteFile(legacyPath+"/legacy-secret.yaml", secretContent, 0644); err != nil {
			t.Fatalf("Failed to write legacy secret: %v", err)
		}

		content, pathUsed, err := client.ReadFileWithFallback("production-cluster", "production", "legacy-secret")
		if err != nil {
			t.Errorf("ReadFileWithFallback() error = %v", err)
		}

		expectedPath := "namespaces/production/secrets/legacy-secret.yaml"
		if pathUsed != expectedPath {
			t.Errorf("ReadFileWithFallback() pathUsed = %v, want %v", pathUsed, expectedPath)
		}

		if string(content) != string(secretContent) {
			t.Errorf("ReadFileWithFallback() content = %v, want %v", content, secretContent)
		}
	})

	t.Run("returns error when secret doesn't exist in either path", func(t *testing.T) {
		_, _, err := client.ReadFileWithFallback("missing-cluster", "missing-ns", "missing-secret")
		if err == nil {
			t.Error("ReadFileWithFallback() expected error for missing secret")
		}
	})
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
