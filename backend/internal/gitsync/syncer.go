package gitsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gopkg.in/yaml.v3"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// GitClient interface for repository operations
type GitClient interface {
	EnsureRepo() error
	RepoPath() string
	GetCurrentSHA() (string, error)
	ReadFile(path string) ([]byte, error)
}

// SOPSClient interface for SOPS operations
type SOPSClient interface {
	DecryptYAML(encryptedYAML []byte) ([]byte, error)
}

// Syncer synchronizes secrets from Git repository to database
type Syncer struct {
	db         *gorm.DB
	gitClient  GitClient
	sopsClient SOPSClient
}

// NewSyncer creates a new Git-to-DB syncer
func NewSyncer(db *gorm.DB, gitClient GitClient, sopsClient SOPSClient) *Syncer {
	return &Syncer{
		db:         db,
		gitClient:  gitClient,
		sopsClient: sopsClient,
	}
}

// SyncAll synchronizes all secrets from Git to database
func (s *Syncer) SyncAll() error {
	logger.Info("[GitSync] Starting full sync from Git to DB")

	// Ensure repository is cloned/updated
	if err := s.gitClient.EnsureRepo(); err != nil {
		return fmt.Errorf("failed to ensure repository: %w", err)
	}

	// Get current Git commit SHA
	currentSHA, err := s.gitClient.GetCurrentSHA()
	if err != nil {
		return fmt.Errorf("failed to get current Git SHA: %w", err)
	}
	logger.Info("[GitSync] Current Git SHA", "sha", currentSHA)

	// Get all namespaces from DB
	var namespaces []models.Namespace
	if err := s.db.Find(&namespaces).Error; err != nil {
		return fmt.Errorf("failed to fetch namespaces: %w", err)
	}

	totalSynced := 0
	totalSkipped := 0
	totalErrors := 0

	// Sync secrets for each namespace
	for _, namespace := range namespaces {
		synced, skipped, errs := s.syncNamespace(namespace, currentSHA)
		totalSynced += synced
		totalSkipped += skipped
		totalErrors += errs
	}

	logger.Info("[GitSync] Sync completed",
		"synced", totalSynced,
		"skipped", totalSkipped,
		"errors", totalErrors,
	)

	return nil
}

// syncNamespace syncs all secrets for a specific namespace
func (s *Syncer) syncNamespace(namespace models.Namespace, gitSHA string) (synced, skipped, errors int) {
	logger.Info("[GitSync] Syncing namespace", "namespace", namespace.Name)

	// Build path to namespace secrets directory
	secretsDir := filepath.Join(s.gitClient.RepoPath(), "namespaces", namespace.Name, "secrets")

	// Check if directory exists
	if _, err := os.Stat(secretsDir); os.IsNotExist(err) {
		logger.Info("[GitSync] No secrets directory found for namespace", "namespace", namespace.Name)
		return 0, 0, 0
	}

	// Read all YAML files in secrets directory
	entries, err := os.ReadDir(secretsDir)
	if err != nil {
		logger.Error("[GitSync] Failed to read secrets directory",
			"namespace", namespace.Name,
			"error", err,
		)
		return 0, 0, 1
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		// Skip .gitkeep files
		if entry.Name() == ".gitkeep" {
			continue
		}

		secretName := strings.TrimSuffix(entry.Name(), ".yaml")
		filePath := filepath.Join(secretsDir, entry.Name())

		if err := s.syncSecret(namespace, secretName, filePath, gitSHA); err != nil {
			logger.Error("[GitSync] Failed to sync secret",
				"namespace", namespace.Name,
				"secret", secretName,
				"error", err,
			)
			errors++
		} else {
			synced++
		}
	}

	return synced, skipped, errors
}

// syncSecret syncs a single secret from Git to DB
func (s *Syncer) syncSecret(namespace models.Namespace, secretName, filePath, gitSHA string) error {
	// Read YAML file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Decrypt SOPS-encrypted content
	decryptedContent, err := s.sopsClient.DecryptYAML(data)
	if err != nil {
		logger.Warn("[GitSync] Failed to decrypt secret, skipping",
			"namespace", namespace.Name,
			"secret", secretName,
			"error", err)
		return nil // Skip this secret, don't fail the whole sync
	}

	// Parse YAML
	var secretData map[string]interface{}
	if err := yaml.Unmarshal(decryptedContent, &secretData); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Extract the actual secret data (skip metadata fields)
	cleanData := make(map[string]interface{})
	for key, value := range secretData {
		// Skip Kubernetes metadata fields
		if key == "apiVersion" || key == "kind" || key == "metadata" || key == "type" {
			continue
		}
		// The actual data is usually in a "data" or "stringData" field
		if key == "data" || key == "stringData" {
			if dataMap, ok := value.(map[string]interface{}); ok {
				cleanData = dataMap
			}
		}
	}

	// If we didn't find data/stringData, use the whole structure
	if len(cleanData) == 0 {
		cleanData = secretData
	}

	// Convert to JSON for database storage
	jsonData, err := json.Marshal(cleanData)
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// Check if secret already exists in DB
	var existingDraft models.SecretDraft
	result := s.db.Where("secret_name = ? AND namespace_id = ?", secretName, namespace.ID).First(&existingDraft)

	if result.Error == gorm.ErrRecordNotFound {
		// Secret doesn't exist in DB - create it
		now := time.Now()
		newDraft := models.SecretDraft{
			SecretName:  secretName,
			NamespaceID: namespace.ID,
			Data:        datatypes.JSON(jsonData),
			Status:      "published",
			GitBaseSHA:  gitSHA,
			CommitSHA:   gitSHA,
			EditedAt:    now,
			PublishedAt: &now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := s.db.Create(&newDraft).Error; err != nil {
			return fmt.Errorf("failed to create secret draft: %w", err)
		}

		logger.Info("[GitSync] Created new secret from Git",
			"namespace", namespace.Name,
			"secret", secretName,
			"sha", gitSHA[:7],
		)
		return nil
	}

	if result.Error != nil {
		return fmt.Errorf("failed to query existing secret: %w", result.Error)
	}

	// Secret exists - decide if we should update it
	// Don't overwrite drafts that are being edited locally
	if existingDraft.Status == "draft" {
		logger.Info("[GitSync] Skipping draft (local changes)",
			"namespace", namespace.Name,
			"secret", secretName,
		)
		return nil
	}

	// If Git SHA is different, update the record
	if existingDraft.CommitSHA != gitSHA {
		existingDraft.Data = datatypes.JSON(jsonData)
		existingDraft.CommitSHA = gitSHA
		existingDraft.GitBaseSHA = gitSHA
		existingDraft.Status = "published"
		existingDraft.UpdatedAt = time.Now()

		if err := s.db.Save(&existingDraft).Error; err != nil {
			return fmt.Errorf("failed to update secret draft: %w", err)
		}

		logger.Info("[GitSync] Updated secret from Git",
			"namespace", namespace.Name,
			"secret", secretName,
			"old_sha", existingDraft.CommitSHA[:7],
			"new_sha", gitSHA[:7],
		)
	} else {
		logger.Debug("[GitSync] Secret already up-to-date",
			"namespace", namespace.Name,
			"secret", secretName,
		)
	}

	return nil
}

// SyncSecret reloads a specific secret from Git, discarding any local changes
func (s *Syncer) SyncSecret(namespaceName, secretName string) error {
	logger.Info("[GitSync] Syncing single secret from Git", "namespace", namespaceName, "secret", secretName)

	// 1. Ensure Git repo is up to date
	if err := s.gitClient.EnsureRepo(); err != nil {
		return fmt.Errorf("failed to sync Git repository: %w", err)
	}

	// 2. Get current Git SHA
	sha, err := s.gitClient.GetCurrentSHA()
	if err != nil {
		return fmt.Errorf("failed to get Git SHA: %w", err)
	}

	// 3. Find namespace ID
	var namespace models.Namespace
	if err := s.db.Where("name = ?", namespaceName).First(&namespace).Error; err != nil {
		return fmt.Errorf("namespace not found: %w", err)
	}

	// 4. Read secret file from Git
	secretPath := fmt.Sprintf("namespaces/%s/secrets/%s.yaml", namespaceName, secretName)
	content, err := s.gitClient.ReadFile(secretPath)
	if err != nil {
		return fmt.Errorf("secret not found in Git: %w", err)
	}

	// 5. Decrypt SOPS-encrypted content
	decryptedContent, err := s.sopsClient.DecryptYAML(content)
	if err != nil {
		return fmt.Errorf("failed to decrypt secret from Git: %w", err)
	}

	// 6. Parse YAML (reuse logic from syncSecret)
	var secretData map[string]interface{}
	if err := yaml.Unmarshal(decryptedContent, &secretData); err != nil {
		return fmt.Errorf("failed to parse secret YAML: %w", err)
	}

	// Extract the actual secret data (skip metadata fields)
	cleanData := make(map[string]interface{})
	for key, value := range secretData {
		// Skip Kubernetes metadata fields
		if key == "apiVersion" || key == "kind" || key == "metadata" || key == "type" {
			continue
		}
		// The actual data is usually in a "data" or "stringData" field
		if key == "data" || key == "stringData" {
			if dataMap, ok := value.(map[string]interface{}); ok {
				cleanData = dataMap
			}
		}
	}

	// If we didn't find data/stringData, use the whole structure
	if len(cleanData) == 0 {
		cleanData = secretData
	}

	// Convert to JSON for database storage
	jsonData, err := json.Marshal(cleanData)
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// 7. Update or create DB record with status="published"
	now := time.Now()
	secret := models.SecretDraft{
		SecretName:  secretName,
		NamespaceID: namespace.ID,
		Data:        datatypes.JSON(jsonData),
		Status:      "published",
		GitBaseSHA:  sha,
		CommitSHA:   sha,
		EditedAt:    now,
		PublishedAt: &now,
		UpdatedAt:   now,
	}

	// Use GORM's Save which updates if exists, creates if not
	// First, check if it exists to properly update all fields
	var existingDraft models.SecretDraft
	result := s.db.Where("secret_name = ? AND namespace_id = ?", secretName, namespace.ID).First(&existingDraft)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new record
		secret.CreatedAt = now
		if err := s.db.Create(&secret).Error; err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
		logger.Info("[GitSync] Created secret from Git", "namespace", namespaceName, "secret", secretName)
	} else if result.Error != nil {
		return fmt.Errorf("failed to query existing secret: %w", result.Error)
	} else {
		// Update existing record - preserve ID and CreatedAt
		secret.ID = existingDraft.ID
		secret.CreatedAt = existingDraft.CreatedAt
		if err := s.db.Save(&secret).Error; err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}
		logger.Info("[GitSync] Reset secret to Git state", "namespace", namespaceName, "secret", secretName, "sha", sha[:7])
	}

	return nil
}

// ReadSecretFromGit reads and decrypts a secret from Git without modifying the database
func (s *Syncer) ReadSecretFromGit(namespaceName, secretName string) (map[string]string, error) {
	logger.Info("[GitSync] Reading secret from Git", "namespace", namespaceName, "secret", secretName)

	// 1. Ensure Git repo is up to date
	if err := s.gitClient.EnsureRepo(); err != nil {
		return nil, fmt.Errorf("failed to sync Git repository: %w", err)
	}

	// 2. Read secret file from Git
	secretPath := fmt.Sprintf("namespaces/%s/secrets/%s.yaml", namespaceName, secretName)
	content, err := s.gitClient.ReadFile(secretPath)
	if err != nil {
		return nil, fmt.Errorf("secret not found in Git: %w", err)
	}

	// 3. Decrypt SOPS-encrypted content
	decryptedContent, err := s.sopsClient.DecryptYAML(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret from Git: %w", err)
	}

	// 4. Parse YAML
	var secretData map[string]interface{}
	if err := yaml.Unmarshal(decryptedContent, &secretData); err != nil {
		return nil, fmt.Errorf("failed to parse secret YAML: %w", err)
	}

	// 5. Extract the actual data field (Kubernetes Secret structure)
	// The YAML has structure: { apiVersion, kind, metadata, data }
	// We only want the "data" field, and we ensure all values are strings
	if dataField, ok := secretData["data"].(map[string]interface{}); ok {
		// Convert interface{} to string
		result := make(map[string]string)
		for k, v := range dataField {
			if strVal, ok := v.(string); ok {
				result[k] = strVal
			} else {
				// Log warning if value is not a string
				logger.Warn("[GitSync] Non-string value in secret data field", "key", k, "type", fmt.Sprintf("%T", v))
			}
		}
		return result, nil
	} else if stringDataField, ok := secretData["stringData"].(map[string]interface{}); ok {
		// Some secrets use stringData instead of data
		result := make(map[string]string)
		for k, v := range stringDataField {
			if strVal, ok := v.(string); ok {
				result[k] = strVal
			} else {
				logger.Warn("[GitSync] Non-string value in secret stringData field", "key", k, "type", fmt.Sprintf("%T", v))
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("no data or stringData field found in secret")
}
