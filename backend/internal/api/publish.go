package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/middleware"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// GitClientInterface defines the Git operations needed for publishing
type GitClientInterface interface {
	EnsureRepo() error
	WriteFile(path string, content []byte) error
	ReadFile(path string) ([]byte, error)
	Commit(message, authorName string, files []string) (string, error)
	Push() error
	FileExists(path string) (bool, error)
	GetFilePath(namespace, secretName string) string
	RepoPath() string
	GetCurrentSHA() (string, error)
}

// SOPSClientInterface defines the SOPS operations needed for publishing
type SOPSClientInterface interface {
	EncryptYAML(yamlContent []byte) ([]byte, error)
	DecryptYAML(encryptedYAML []byte) ([]byte, error)
}

// PublishHandlers contains handlers for publish/unpublish operations
type PublishHandlers struct {
	db         *gorm.DB
	gitClient  GitClientInterface
	sopsClient SOPSClientInterface
}

// NewPublishHandlers creates a new PublishHandlers instance
func NewPublishHandlers(db *gorm.DB, gitClient GitClientInterface, sopsClient SOPSClientInterface) *PublishHandlers {
	return &PublishHandlers{
		db:         db,
		gitClient:  gitClient,
		sopsClient: sopsClient,
	}
}

// K8sSecret represents a Kubernetes Secret structure
type K8sSecret struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   K8sMetadata       `yaml:"metadata"`
	Type       string            `yaml:"type"`
	Data       map[string]string `yaml:"data"`
}

// K8sMetadata represents Kubernetes Secret metadata
type K8sMetadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// PublishSecret handles POST /api/v1/namespaces/{namespace}/secrets/{name}/publish
func (h *PublishHandlers) PublishSecret(w http.ResponseWriter, r *http.Request) {
	// Check if Git and SOPS clients are initialized
	if h.gitClient == nil {
		logger.Error("Git client not initialized - check GIT_REPO_PATH configuration")
		respondError(w, http.StatusServiceUnavailable, "Git repository not configured")
		return
	}
	if h.sopsClient == nil {
		logger.Error("SOPS client not initialized - check SOPS configuration")
		respondError(w, http.StatusServiceUnavailable, "SOPS encryption not configured")
		return
	}

	ctx := r.Context()
	namespaceIDStr := chi.URLParam(r, "namespace")
	secretName := chi.URLParam(r, "name")

	// Parse namespace ID
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid namespace ID")
		return
	}

	// Get user from context
	userCtx, err := middleware.GetUserFromContext(ctx)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Fetch namespace to get the name
	var namespace models.Namespace
	if err := h.db.First(&namespace, "id = ?", namespaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Namespace not found")
		} else {
			logger.Error("Failed to fetch namespace", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to fetch namespace")
		}
		return
	}

	// Fetch secret
	var secret models.SecretDraft
	if err := h.db.Where("secret_name = ? AND namespace_id = ?", secretName, namespaceID).First(&secret).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Secret not found")
		} else {
			logger.Error("Failed to fetch secret", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to fetch secret")
		}
		return
	}

	// Allow publishing drafts, re-publishing published secrets, and resolving drifted secrets
	// No status validation needed - all statuses can be published/re-published

	// Convert secret to Kubernetes Secret YAML
	k8sSecretYAML, err := convertToK8sSecret(&secret, namespace.Name)
	if err != nil {
		logger.Error("Failed to convert secret to K8s format", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to convert secret to Kubernetes format")
		return
	}

	// Encrypt YAML with SOPS
	encryptedYAML, err := h.sopsClient.EncryptYAML(k8sSecretYAML)
	if err != nil {
		logger.Error("Failed to encrypt secret with SOPS", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to encrypt secret")
		return
	}

	// Ensure Git repository is ready
	if err := h.gitClient.EnsureRepo(); err != nil {
		logger.Error("Failed to ensure Git repository", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to prepare Git repository")
		return
	}

	// Write encrypted YAML to Git repo
	filePath := h.gitClient.GetFilePath(namespace.Name, secretName)
	if err := h.gitClient.WriteFile(filePath, encryptedYAML); err != nil {
		logger.Error("Failed to write secret file to Git", "error", err, "path", filePath)
		respondError(w, http.StatusInternalServerError, "Failed to write secret to Git repository")
		return
	}

	// Commit changes
	commitMessage := fmt.Sprintf("feat: publish secret %s/%s", namespace.Name, secretName)
	commitSHA, err := h.gitClient.Commit(commitMessage, "", []string{filePath})
	if err != nil {
		logger.Error("Failed to commit secret", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to commit secret to Git")
		return
	}

	// Push to remote with retry
	if err := h.gitClient.Push(); err != nil {
		logger.Error("Failed to push secret to remote", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to push secret to remote repository")
		return
	}

	// Update secret status
	now := time.Now()
	secret.Status = "published"
	secret.CommitSHA = commitSHA
	secret.PublishedBy = &userCtx.UserID
	secret.PublishedAt = &now

	if err := h.db.Save(&secret).Error; err != nil {
		logger.Error("Failed to update secret status", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to update secret status")
		return
	}

	// Create audit log entry
	if err := createAuditLog(h.db, userCtx.UserID, namespaceID, secret.ID, "publish_secret", map[string]interface{}{
		"commit_sha":  commitSHA,
		"namespace":   namespace.Name,
		"secret_name": secretName,
	}); err != nil {
		logger.Error("Failed to create audit log", "error", err)
		// Don't fail the request if audit log creation fails
	}

	logger.Info("Secret published successfully",
		"namespace", namespace.Name,
		"secret", secretName,
		"commit_sha", commitSHA,
		"user", userCtx.Email,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(secret)
}

// UnpublishSecret handles POST /api/v1/namespaces/{namespace}/secrets/{name}/unpublish
func (h *PublishHandlers) UnpublishSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespaceIDStr := chi.URLParam(r, "namespace")
	secretName := chi.URLParam(r, "name")

	// Parse namespace ID
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid namespace ID")
		return
	}

	// Get user from context
	userCtx, err := middleware.GetUserFromContext(ctx)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Fetch namespace to get the name
	var namespace models.Namespace
	if err := h.db.First(&namespace, "id = ?", namespaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Namespace not found")
		} else {
			logger.Error("Failed to fetch namespace", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to fetch namespace")
		}
		return
	}

	// Fetch secret
	var secret models.SecretDraft
	if err := h.db.Where("secret_name = ? AND namespace_id = ?", secretName, namespaceID).First(&secret).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Secret not found")
		} else {
			logger.Error("Failed to fetch secret", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to fetch secret")
		}
		return
	}

	// Validate secret status
	if secret.Status != "published" {
		respondError(w, http.StatusConflict, fmt.Sprintf("Cannot unpublish secret with status '%s'. Only published secrets can be unpublished.", secret.Status))
		return
	}

	// Ensure Git repository is ready
	if err := h.gitClient.EnsureRepo(); err != nil {
		logger.Error("Failed to ensure Git repository", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to prepare Git repository")
		return
	}

	// Delete file from Git repo
	filePath := h.gitClient.GetFilePath(namespace.Name, secretName)

	// Check if file exists before attempting to delete
	exists, err := h.gitClient.FileExists(filePath)
	if err != nil {
		logger.Error("Failed to check if file exists", "error", err, "path", filePath)
		respondError(w, http.StatusInternalServerError, "Failed to check file existence")
		return
	}

	if exists {
		// Delete the file from filesystem - only if repo path is available
		// For testing with mocks, this might not do anything
		repoPath := h.gitClient.RepoPath()
		if repoPath != "" {
			fullPath := filepath.Join(repoPath, filePath)
			// Only delete if the file actually exists on disk
			if _, statErr := os.Stat(fullPath); statErr == nil {
				if err := os.Remove(fullPath); err != nil {
					logger.Error("Failed to delete secret file", "error", err, "path", filePath)
					respondError(w, http.StatusInternalServerError, "Failed to delete secret file")
					return
				}
			}
		}

		// Commit deletion
		commitMessage := fmt.Sprintf("feat: unpublish secret %s/%s", namespace.Name, secretName)
		commitSHA, err := h.gitClient.Commit(commitMessage, "", []string{filePath})
		if err != nil {
			logger.Error("Failed to commit secret deletion", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to commit deletion to Git")
			return
		}

		// Push to remote
		if err := h.gitClient.Push(); err != nil {
			logger.Error("Failed to push deletion to remote", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to push deletion to remote repository")
			return
		}

		logger.Info("Secret file deleted from Git",
			"namespace", namespace.Name,
			"secret", secretName,
			"commit_sha", commitSHA,
		)

		// Create audit log entry with commit SHA
		if err := createAuditLog(h.db, userCtx.UserID, namespaceID, secret.ID, "unpublish_secret", map[string]interface{}{
			"commit_sha":  commitSHA,
			"namespace":   namespace.Name,
			"secret_name": secretName,
		}); err != nil {
			logger.Error("Failed to create audit log", "error", err)
			// Don't fail the request if audit log creation fails
		}
	} else {
		logger.Warn("Secret file not found in Git, updating status only",
			"namespace", namespace.Name,
			"secret", secretName,
		)

		// Create audit log entry without commit SHA
		if err := createAuditLog(h.db, userCtx.UserID, namespaceID, secret.ID, "unpublish_secret", map[string]interface{}{
			"namespace":   namespace.Name,
			"secret_name": secretName,
			"note":        "file not found in Git",
		}); err != nil {
			logger.Error("Failed to create audit log", "error", err)
			// Don't fail the request if audit log creation fails
		}
	}

	// Update secret status to draft
	secret.Status = "draft"
	secret.CommitSHA = ""
	secret.PublishedBy = nil
	secret.PublishedAt = nil

	if err := h.db.Save(&secret).Error; err != nil {
		logger.Error("Failed to update secret status", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to update secret status")
		return
	}

	logger.Info("Secret unpublished successfully",
		"namespace", namespace.Name,
		"secret", secretName,
		"user", userCtx.Email,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(secret)
}

// convertToK8sSecret converts a SecretDraft to Kubernetes Secret YAML format
func convertToK8sSecret(draft *models.SecretDraft, namespaceName string) ([]byte, error) {
	// Unmarshal the JSON data
	var secretData map[string]interface{}
	if err := json.Unmarshal([]byte(draft.Data), &secretData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret data: %w", err)
	}

	// Base64 encode all values
	encodedData := make(map[string]string)
	for key, value := range secretData {
		// Convert value to string
		strValue := fmt.Sprintf("%v", value)
		// Base64 encode
		encodedData[key] = base64.StdEncoding.EncodeToString([]byte(strValue))
	}

	// Create Kubernetes Secret structure
	k8sSecret := K8sSecret{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata: K8sMetadata{
			Name:      draft.SecretName,
			Namespace: namespaceName,
		},
		Type: "Opaque",
		Data: encodedData,
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(&k8sSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	return yamlBytes, nil
}

// createAuditLog creates an audit log entry
func createAuditLog(db *gorm.DB, userID, namespaceID, secretID uuid.UUID, action string, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed to marshal audit log details: %w", err)
	}

	auditLog := models.AuditLog{
		UserID:       &userID,
		ActionType:   action,
		ResourceType: "secret",
		ResourceName: fmt.Sprintf("%v", details["secret_name"]),
		NamespaceID:  &namespaceID,
		Timestamp:    time.Now(),
		Metadata:     detailsJSON,
	}

	if err := db.Create(&auditLog).Error; err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}
