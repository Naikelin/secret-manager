package drift

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/k8s"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gopkg.in/yaml.v3"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// GitClientInterface defines the Git operations needed for drift detection
type GitClientInterface interface {
	EnsureRepo() error
	ReadFile(path string) ([]byte, error)
	GetFilePath(namespace, secretName string) string
}

// SOPSClientInterface defines the SOPS operations needed for drift detection
type SOPSClientInterface interface {
	DecryptYAML(encryptedYAML []byte) ([]byte, error)
}

// K8sClientInterface defines the Kubernetes operations needed for drift detection
type K8sClientInterface interface {
	GetSecret(namespace, name string) (*corev1.Secret, error)
}

// DriftDetector detects drift between Git (source of truth) and Kubernetes (actual state)
type DriftDetector struct {
	db         *gorm.DB
	k8sClient  K8sClientInterface
	gitClient  GitClientInterface
	sopsClient SOPSClientInterface
}

// NewDriftDetector creates a new DriftDetector instance
func NewDriftDetector(db *gorm.DB, k8sClient K8sClientInterface, gitClient GitClientInterface, sopsClient SOPSClientInterface) *DriftDetector {
	return &DriftDetector{
		db:         db,
		k8sClient:  k8sClient,
		gitClient:  gitClient,
		sopsClient: sopsClient,
	}
}

// DetectDriftForNamespace checks all published secrets in a namespace for drift
func (d *DriftDetector) DetectDriftForNamespace(namespaceID uuid.UUID) ([]models.DriftEvent, error) {
	// Load namespace
	var namespace models.Namespace
	if err := d.db.First(&namespace, namespaceID).Error; err != nil {
		return nil, fmt.Errorf("failed to load namespace: %w", err)
	}

	// Get all published secrets in namespace
	var secrets []models.SecretDraft
	if err := d.db.Where("namespace_id = ? AND status = ?", namespaceID, "published").Find(&secrets).Error; err != nil {
		return nil, fmt.Errorf("failed to query secrets: %w", err)
	}

	// Ensure Git repository is up to date
	if d.gitClient != nil {
		if err := d.gitClient.EnsureRepo(); err != nil {
			return nil, fmt.Errorf("failed to sync Git repository: %w", err)
		}
	}

	// Check each secret for drift
	var driftEvents []models.DriftEvent
	for _, secret := range secrets {
		event, err := d.DetectDriftForSecret(secret.ID)
		if err != nil {
			logger.Error("Drift detection failed for secret", "secret", secret.SecretName, "error", err)
			continue
		}

		if event != nil {
			driftEvents = append(driftEvents, *event)

			// Update secret status to "drifted"
			if err := d.db.Model(&secret).Update("status", "drifted").Error; err != nil {
				logger.Error("Failed to update secret status to drifted", "secret", secret.SecretName, "error", err)
			}
		}
	}

	return driftEvents, nil
}

// DetectDriftForSecret checks a single secret for drift
func (d *DriftDetector) DetectDriftForSecret(secretID uuid.UUID) (*models.DriftEvent, error) {
	// Load secret from DB
	var secret models.SecretDraft
	if err := d.db.First(&secret, secretID).Error; err != nil {
		return nil, fmt.Errorf("failed to load secret: %w", err)
	}

	// Only check published secrets
	if secret.Status != "published" {
		return nil, nil
	}

	// Load namespace
	var namespace models.Namespace
	if err := d.db.First(&namespace, secret.NamespaceID).Error; err != nil {
		return nil, fmt.Errorf("failed to load namespace: %w", err)
	}

	// Check for drift
	return d.compareDrift(&secret, &namespace)
}

// compareDrift performs the actual drift comparison
func (d *DriftDetector) compareDrift(secret *models.SecretDraft, namespace *models.Namespace) (*models.DriftEvent, error) {
	// 1. Decrypt secret from Git using SOPS
	filePath := d.gitClient.GetFilePath(namespace.Name, secret.SecretName)
	encryptedYAML, err := d.gitClient.ReadFile(filePath)
	if err != nil {
		// File missing from Git - this is drift!
		gitVersion := make(map[string]interface{})
		k8sVersion := map[string]interface{}{
			"status": "file_missing_from_git",
		}

		event := &models.DriftEvent{
			SecretName:  secret.SecretName,
			NamespaceID: secret.NamespaceID,
			DetectedAt:  time.Now(),
			GitVersion:  mustMarshalJSON(gitVersion),
			K8sVersion:  mustMarshalJSON(k8sVersion),
			Diff:        mustMarshalJSON(map[string]string{"error": fmt.Sprintf("Secret file missing from Git repository: %s", filePath)}),
		}

		// Save drift event to database
		if err := d.db.Create(event).Error; err != nil {
			return nil, fmt.Errorf("failed to save drift event: %w", err)
		}

		return event, nil
	}

	// Decrypt the YAML
	decryptedYAML, err := d.sopsClient.DecryptYAML(encryptedYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret from Git: %w", err)
	}

	// Parse YAML to extract data field
	var k8sSecretYAML corev1.Secret
	if err := yaml.Unmarshal(decryptedYAML, &k8sSecretYAML); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted YAML: %w", err)
	}

	gitData := k8s.NormalizeSecretData(&k8sSecretYAML)

	// 2. Get secret from Kubernetes
	k8sSecret, err := d.k8sClient.GetSecret(namespace.Name, secret.SecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Secret missing from Kubernetes cluster
			gitVersion := map[string]interface{}{
				"keys": getMapKeys(gitData),
			}
			k8sVersion := map[string]interface{}{
				"status": "not_found",
			}

			event := &models.DriftEvent{
				SecretName:  secret.SecretName,
				NamespaceID: secret.NamespaceID,
				DetectedAt:  time.Now(),
				GitVersion:  mustMarshalJSON(gitVersion),
				K8sVersion:  mustMarshalJSON(k8sVersion),
				Diff:        mustMarshalJSON(map[string]string{"error": "Secret missing from Kubernetes cluster"}),
			}

			// Save drift event to database
			if err := d.db.Create(event).Error; err != nil {
				return nil, fmt.Errorf("failed to save drift event: %w", err)
			}

			return event, nil
		}
		return nil, fmt.Errorf("failed to get secret from Kubernetes: %w", err)
	}

	// 3. Compare data
	k8sData := k8s.NormalizeSecretData(k8sSecret)
	if !k8s.CompareSecretData(k8sSecret, gitData) {
		// Drift detected - compute detailed diff
		differences := k8s.ComputeDiff(gitData, k8sData)

		gitVersion := map[string]interface{}{
			"keys": getMapKeys(gitData),
			"hash": k8s.CalculateSecretHash(gitData),
		}
		k8sVersion := map[string]interface{}{
			"keys": getMapKeys(k8sData),
			"hash": k8s.CalculateSecretHash(k8sData),
		}
		diffMap := map[string]interface{}{
			"differences": differences,
		}

		event := &models.DriftEvent{
			SecretName:  secret.SecretName,
			NamespaceID: secret.NamespaceID,
			DetectedAt:  time.Now(),
			GitVersion:  mustMarshalJSON(gitVersion),
			K8sVersion:  mustMarshalJSON(k8sVersion),
			Diff:        mustMarshalJSON(diffMap),
		}

		// Save drift event to database
		if err := d.db.Create(event).Error; err != nil {
			return nil, fmt.Errorf("failed to save drift event: %w", err)
		}

		return event, nil
	}

	// No drift detected
	return nil, nil
}

// mustMarshalJSON marshals data to JSON or panics (for internal use)
func mustMarshalJSON(data interface{}) datatypes.JSON {
	bytes, err := json.Marshal(data)
	if err != nil {
		// This should never happen with simple map[string]interface{}
		logger.Error("Failed to marshal JSON", "error", err, "data", data)
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(bytes)
}

// getMapKeys returns a sorted list of map keys
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
