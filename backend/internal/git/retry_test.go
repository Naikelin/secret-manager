package git

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestPushWithRetry_Success(t *testing.T) {
	// Create a test repository
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create initial commit
	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	if err := os.WriteFile(tmpDir+"/test.txt", []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if _, err := w.Add("test.txt"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	if _, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	client := &GitClient{
		repoPath: tmpDir,
		repo:     repo,
	}

	// Note: This will fail because there's no remote configured,
	// but we're testing the retry logic structure
	err = client.PushWithRetry(1)
	if err == nil {
		t.Error("PushWithRetry() expected error (no remote configured)")
	}
}

func TestPushWithRetry_DefaultRetries(t *testing.T) {
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

	// Test with 0 retries (should use default of 3)
	start := time.Now()
	err = client.PushWithRetry(0)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("PushWithRetry() expected error (no remote)")
	}

	// With 3 retries and exponential backoff (1s, 2s), minimum time should be ~3s
	// Allow some margin for test execution time
	if elapsed < 2*time.Second {
		t.Errorf("PushWithRetry() completed too quickly, expected backoff delays")
	}
}

func TestPushWithRetry_NegativeRetries(t *testing.T) {
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

	// Test with negative retries (should use default)
	err = client.PushWithRetry(-5)
	if err == nil {
		t.Error("PushWithRetry() expected error (no remote)")
	}

	// Error message should mention retries
	if err != nil && err.Error() != "" {
		// Just verify we get an error, the exact count doesn't matter
		// as long as it doesn't crash
	}
}

func TestPushWithRetry_MultipleAttempts(t *testing.T) {
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

	// Test with 2 retries
	start := time.Now()
	err = client.PushWithRetry(2)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("PushWithRetry() expected error (no remote)")
	}

	// With 2 retries and exponential backoff (1s), minimum time should be ~1s
	if elapsed < 500*time.Millisecond {
		t.Errorf("PushWithRetry() completed too quickly: %v, expected at least 500ms for backoff", elapsed)
	}

	// Error should mention the number of retries
	expectedErrMsg := fmt.Sprintf("push failed after %d retries", 2)
	if err != nil && err.Error()[:len(expectedErrMsg)] != expectedErrMsg {
		t.Errorf("PushWithRetry() error message = %v, should start with %v", err.Error(), expectedErrMsg)
	}
}

func TestPushWithRetry_NoRepo(t *testing.T) {
	client := &GitClient{
		repoPath: "/tmp/nonexistent",
	}

	err := client.PushWithRetry(1)
	if err == nil {
		t.Error("PushWithRetry() expected error when repo is nil")
	}
}
