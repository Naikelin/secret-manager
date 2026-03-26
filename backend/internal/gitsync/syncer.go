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
}

// Syncer synchronizes secrets from Git repository to database
type Syncer struct {
	db        *gorm.DB
	gitClient GitClient
}

// NewSyncer creates a new Git-to-DB syncer
func NewSyncer(db *gorm.DB, gitClient GitClient) *Syncer {
	return &Syncer{
		db:        db,
		gitClient: gitClient,
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

	// Parse YAML
	var secretData map[string]interface{}
	if err := yaml.Unmarshal(data, &secretData); err != nil {
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
