package drift

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/config"
	"github.com/yourorg/secret-manager/internal/flux"
	"github.com/yourorg/secret-manager/internal/k8s"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/internal/notifications"
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
	WriteFile(path string, content []byte) error
	Commit(message, authorName string, files []string) (string, error)
	Push() error
	GetFilePath(clusterName, namespace, secretName string) string
	GetFilePathLegacy(namespace, secretName string) string
	ReadFileWithFallback(clusterName, namespace, secretName string) ([]byte, string, error)
}

// SOPSClientInterface defines the SOPS operations needed for drift detection
type SOPSClientInterface interface {
	DecryptYAML(encryptedYAML []byte) ([]byte, error)
	EncryptYAML(yamlContent []byte) ([]byte, error)
}

// K8sClientInterface defines the Kubernetes operations needed for drift detection
type K8sClientInterface interface {
	GetSecret(namespace, name string) (*corev1.Secret, error)
}

// FluxClientInterface defines the FluxCD operations needed for drift detection
type FluxClientInterface interface {
	TriggerKustomizationReconciliation(ctx context.Context, name, namespace string) error
	TriggerGitRepositoryReconciliation(ctx context.Context, name, namespace string) error
	WaitForKustomizationReconciliation(ctx context.Context, name, namespace string, timeout, pollInterval time.Duration) error
	GetKustomizationStatus(name, namespace string) (*flux.KustomizationStatus, error)
}

// ClientManagerInterface defines the operations for managing per-cluster K8s clients
type ClientManagerInterface interface {
	GetClient(clusterID uuid.UUID) (K8sClientInterface, error)
	HealthCheck(clusterID uuid.UUID) (bool, error)
}

// clientManagerAdapter adapts k8s.ClientManager to ClientManagerInterface
type clientManagerAdapter struct {
	inner k8s.ClientManager
}

func (a *clientManagerAdapter) GetClient(clusterID uuid.UUID) (K8sClientInterface, error) {
	return a.inner.GetClient(clusterID)
}

func (a *clientManagerAdapter) HealthCheck(clusterID uuid.UUID) (bool, error) {
	return a.inner.HealthCheck(clusterID)
}

// WrapClientManager wraps a k8s.ClientManager to satisfy ClientManagerInterface
func WrapClientManager(cm k8s.ClientManager) ClientManagerInterface {
	return &clientManagerAdapter{inner: cm}
}

// DriftDetector detects drift between Git (source of truth) and Kubernetes (actual state)
type DriftDetector struct {
	db            *gorm.DB
	k8sClient     K8sClientInterface // Deprecated: kept for backward compatibility
	clientManager ClientManagerInterface
	gitClient     GitClientInterface
	sopsClient    SOPSClientInterface
	webhookClient *notifications.WebhookClient
	fluxClient    FluxClientInterface
	cfg           *config.Config
}

// NewDriftDetector creates a new DriftDetector instance
// clientManager: per-cluster K8s client pool (required for multi-cluster)
// k8sClient: deprecated single client (kept for backward compatibility)
func NewDriftDetector(db *gorm.DB, clientManager ClientManagerInterface, gitClient GitClientInterface, sopsClient SOPSClientInterface, webhookClient *notifications.WebhookClient, fluxClient FluxClientInterface, cfg *config.Config) *DriftDetector {
	return &DriftDetector{
		db:            db,
		clientManager: clientManager,
		gitClient:     gitClient,
		sopsClient:    sopsClient,
		webhookClient: webhookClient,
		fluxClient:    fluxClient,
		cfg:           cfg,
	}
}

// NewDriftDetectorWithSingleClient creates a DriftDetector with a single K8s client (backward compatibility)
// Deprecated: Use NewDriftDetector with ClientManager for multi-cluster support
func NewDriftDetectorWithSingleClient(db *gorm.DB, k8sClient K8sClientInterface, gitClient GitClientInterface, sopsClient SOPSClientInterface, webhookClient *notifications.WebhookClient, fluxClient FluxClientInterface, cfg *config.Config) *DriftDetector {
	return &DriftDetector{
		db:            db,
		k8sClient:     k8sClient,
		gitClient:     gitClient,
		sopsClient:    sopsClient,
		webhookClient: webhookClient,
		fluxClient:    fluxClient,
		cfg:           cfg,
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

// DetectDriftForAllClusters checks drift across all clusters with per-cluster error isolation
// This is the primary entry point for the drift detection scheduler
func (d *DriftDetector) DetectDriftForAllClusters() (map[uuid.UUID][]models.DriftEvent, error) {
	if d.clientManager == nil {
		return nil, fmt.Errorf("clientManager not initialized - cannot perform multi-cluster drift detection")
	}

	logger.Info("Starting multi-cluster drift detection")

	// Load all clusters from database
	var clusters []models.Cluster
	if err := d.db.Find(&clusters).Error; err != nil {
		return nil, fmt.Errorf("failed to load clusters: %w", err)
	}

	logger.Info("Found clusters for drift detection", "count", len(clusters))

	// Ensure Git repository is up to date (once for all clusters)
	if d.gitClient != nil {
		if err := d.gitClient.EnsureRepo(); err != nil {
			return nil, fmt.Errorf("failed to sync Git repository: %w", err)
		}
	}

	// Results map: clusterID -> drift events
	allDriftEvents := make(map[uuid.UUID][]models.DriftEvent)

	// Iterate clusters with error isolation
	for _, cluster := range clusters {
		logger.Info("Processing cluster for drift detection", "cluster", cluster.Name, "cluster_id", cluster.ID)

		// Get K8s client for this cluster
		k8sClient, err := d.clientManager.GetClient(cluster.ID)
		if err != nil {
			logger.Error("Cluster unreachable - skipping drift detection", "cluster", cluster.Name, "cluster_id", cluster.ID, "error", err)
			// Mark cluster as unhealthy (already done in ClientManager)
			continue // Skip this cluster, continue with others
		}

		// Cluster client initialized successfully
		logger.Info("Cluster client initialized successfully", "cluster", cluster.Name)

		// Get all namespaces for this cluster
		var namespaces []models.Namespace
		if err := d.db.Where("cluster_id = ?", cluster.ID).Find(&namespaces).Error; err != nil {
			logger.Error("Failed to load namespaces for cluster", "cluster", cluster.Name, "error", err)
			continue // Skip this cluster
		}

		logger.Info("Found namespaces in cluster", "cluster", cluster.Name, "count", len(namespaces))

		// Track drift events for this cluster
		var clusterDriftEvents []models.DriftEvent

		// Iterate namespaces in this cluster
		for _, namespace := range namespaces {
			// Get all published secrets in namespace
			var secrets []models.SecretDraft
			if err := d.db.Where("namespace_id = ? AND status = ?", namespace.ID, "published").Find(&secrets).Error; err != nil {
				logger.Error("Failed to query secrets in namespace", "cluster", cluster.Name, "namespace", namespace.Name, "error", err)
				continue // Skip this namespace
			}

			logger.Info("Checking secrets for drift", "cluster", cluster.Name, "namespace", namespace.Name, "secret_count", len(secrets))

			// Check each secret for drift
			for _, secret := range secrets {
				event, err := d.detectDriftForSecretWithClient(k8sClient, &secret, &namespace, cluster.Name)
				if err != nil {
					logger.Error("Drift detection failed for secret", "cluster", cluster.Name, "namespace", namespace.Name, "secret", secret.SecretName, "error", err)
					continue // Skip this secret
				}

				if event != nil {
					clusterDriftEvents = append(clusterDriftEvents, *event)

					// Update secret status to "drifted"
					if err := d.db.Model(&secret).Update("status", "drifted").Error; err != nil {
						logger.Error("Failed to update secret status to drifted", "cluster", cluster.Name, "namespace", namespace.Name, "secret", secret.SecretName, "error", err)
					}

					logger.Info("Drift detected", "cluster", cluster.Name, "namespace", namespace.Name, "secret", secret.SecretName, "drift_event_id", event.ID)
				}
			}
		}

		// Store cluster drift events
		if len(clusterDriftEvents) > 0 {
			allDriftEvents[cluster.ID] = clusterDriftEvents
			logger.Info("Drift detection complete for cluster", "cluster", cluster.Name, "drift_count", len(clusterDriftEvents))
		} else {
			logger.Info("No drift detected in cluster", "cluster", cluster.Name)
		}
	}

	logger.Info("Multi-cluster drift detection complete", "total_clusters", len(clusters), "clusters_with_drift", len(allDriftEvents))

	return allDriftEvents, nil
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

	// Load namespace with cluster relationship
	var namespace models.Namespace
	if err := d.db.Preload("ClusterRef").First(&namespace, secret.NamespaceID).Error; err != nil {
		return nil, fmt.Errorf("failed to load namespace: %w", err)
	}

	// Validate cluster exists
	if namespace.ClusterRef == nil {
		return nil, fmt.Errorf("namespace has no cluster association")
	}

	// Get K8s client for this cluster
	if d.clientManager != nil {
		k8sClient, err := d.clientManager.GetClient(*namespace.ClusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to get K8s client for cluster %s: %w", namespace.ClusterRef.Name, err)
		}
		return d.detectDriftForSecretWithClient(k8sClient, &secret, &namespace, namespace.ClusterRef.Name)
	}

	// Backward compatibility: use old single k8sClient if clientManager not available
	if d.k8sClient != nil {
		return d.compareDrift(&secret, &namespace)
	}

	return nil, fmt.Errorf("neither clientManager nor k8sClient available")
}

// compareDrift performs the actual drift comparison (backward compatibility - uses single k8sClient)
func (d *DriftDetector) compareDrift(secret *models.SecretDraft, namespace *models.Namespace) (*models.DriftEvent, error) {
	// 1. Decrypt secret from Git using SOPS
	var encryptedYAML []byte
	var filePath string
	var err error

	// Try dual-path mode if enabled (backward compatibility during migration)
	if d.cfg.EnableDualPathMode {
		encryptedYAML, filePath, err = d.gitClient.ReadFileWithFallback(namespace.ClusterRef.Name, namespace.Name, secret.SecretName)
	} else {
		filePath = d.gitClient.GetFilePath(namespace.ClusterRef.Name, namespace.Name, secret.SecretName)
		encryptedYAML, err = d.gitClient.ReadFile(filePath)
	}

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

	// Parse YAML - use intermediate struct because SOPS-decrypted YAML has base64 strings,
	// not []byte which corev1.Secret.Data expects
	k8sSecretYAML, err := parseSecretYAML(decryptedYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse decrypted YAML: %w", err)
	}

	gitData := k8s.NormalizeSecretData(k8sSecretYAML)

	// 2. Get secret from Kubernetes (use deprecated single client)
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

			// Send webhook notification
			if d.webhookClient != nil {
				notification := notifications.DriftNotification{
					Namespace:  namespace.Name,
					SecretName: secret.SecretName,
					DriftType:  "missing_from_k8s",
					DetectedAt: event.DetectedAt,
					Message:    fmt.Sprintf("⚠️ Drift detected: %s/%s - Secret missing from Kubernetes cluster", namespace.Name, secret.SecretName),
				}

				if err := d.webhookClient.SendDriftNotification(notification); err != nil {
					logger.Error("Failed to send webhook notification", "error", err)
					// Don't fail drift detection if webhook fails
				}
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

		// Send webhook notification
		if d.webhookClient != nil {
			notification := notifications.DriftNotification{
				Namespace:  namespace.Name,
				SecretName: secret.SecretName,
				DriftType:  "data_mismatch",
				DetectedAt: event.DetectedAt,
				Message:    fmt.Sprintf("⚠️ Drift detected: %s/%s - Data mismatch between Git and Kubernetes", namespace.Name, secret.SecretName),
			}

			if err := d.webhookClient.SendDriftNotification(notification); err != nil {
				logger.Error("Failed to send webhook notification", "error", err)
				// Don't fail drift detection if webhook fails
			}
		}

		return event, nil
	}

	// No drift detected
	return nil, nil
}

// detectDriftForSecretWithClient is a helper that performs drift detection with a specific K8s client
// Used by DetectDriftForAllClusters to pass per-cluster clients
func (d *DriftDetector) detectDriftForSecretWithClient(k8sClient K8sClientInterface, secret *models.SecretDraft, namespace *models.Namespace, clusterName string) (*models.DriftEvent, error) {
	// 1. Decrypt secret from Git using SOPS
	var encryptedYAML []byte
	var filePath string
	var err error

	// Try dual-path mode if enabled (backward compatibility during migration)
	if d.cfg.EnableDualPathMode {
		encryptedYAML, filePath, err = d.gitClient.ReadFileWithFallback(clusterName, namespace.Name, secret.SecretName)
	} else {
		filePath = d.gitClient.GetFilePath(clusterName, namespace.Name, secret.SecretName)
		encryptedYAML, err = d.gitClient.ReadFile(filePath)
	}

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

	// Parse YAML - use intermediate struct because SOPS-decrypted YAML has base64 strings,
	// not []byte which corev1.Secret.Data expects
	k8sSecretYAML, err := parseSecretYAML(decryptedYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse decrypted YAML: %w", err)
	}

	gitData := k8s.NormalizeSecretData(k8sSecretYAML)

	// 2. Get secret from Kubernetes using the passed client
	k8sSecret, err := k8sClient.GetSecret(namespace.Name, secret.SecretName)
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

			// Send webhook notification
			if d.webhookClient != nil {
				notification := notifications.DriftNotification{
					Namespace:  namespace.Name,
					SecretName: secret.SecretName,
					DriftType:  "missing_from_k8s",
					DetectedAt: event.DetectedAt,
					Message:    fmt.Sprintf("⚠️ Drift detected: %s/%s - Secret missing from Kubernetes cluster", namespace.Name, secret.SecretName),
				}

				if err := d.webhookClient.SendDriftNotification(notification); err != nil {
					logger.Error("Failed to send webhook notification", "error", err)
					// Don't fail drift detection if webhook fails
				}
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

		// Send webhook notification
		if d.webhookClient != nil {
			notification := notifications.DriftNotification{
				Namespace:  namespace.Name,
				SecretName: secret.SecretName,
				DriftType:  "data_mismatch",
				DetectedAt: event.DetectedAt,
				Message:    fmt.Sprintf("⚠️ Drift detected: %s/%s - Data mismatch between Git and Kubernetes", namespace.Name, secret.SecretName),
			}

			if err := d.webhookClient.SendDriftNotification(notification); err != nil {
				logger.Error("Failed to send webhook notification", "error", err)
				// Don't fail drift detection if webhook fails
			}
		}

		return event, nil
	}

	// No drift detected
	return nil, nil
}

// SecretYAML is an intermediate struct for parsing SOPS-decrypted YAML
// SOPS decrypts to YAML with base64 strings, not []byte that corev1.Secret expects
type SecretYAML struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string            `yaml:"name"`
		Namespace string            `yaml:"namespace"`
		Labels    map[string]string `yaml:"labels,omitempty"`
	} `yaml:"metadata"`
	Type       corev1.SecretType `yaml:"type,omitempty"`
	Data       map[string]string `yaml:"data,omitempty"`       // Accept as base64 strings
	StringData map[string]string `yaml:"stringData,omitempty"` // Accept as plain strings
}

// parseSecretYAML parses SOPS-decrypted YAML and converts to corev1.Secret
// Handles base64-encoded strings in the data field by decoding them to []byte
func parseSecretYAML(decryptedYAML []byte) (*corev1.Secret, error) {
	// First parse into intermediate struct that accepts strings
	var intermediate SecretYAML
	if err := yaml.Unmarshal(decryptedYAML, &intermediate); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Convert to corev1.Secret
	secret := &corev1.Secret{
		Data: make(map[string][]byte),
	}

	// Copy metadata
	secret.Name = intermediate.Metadata.Name
	secret.Namespace = intermediate.Metadata.Namespace
	secret.Labels = intermediate.Metadata.Labels
	secret.Type = intermediate.Type

	// Decode base64 strings in Data field to []byte
	for key, base64Value := range intermediate.Data {
		decodedBytes, err := base64.StdEncoding.DecodeString(base64Value)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 for key %s: %w", key, err)
		}
		secret.Data[key] = decodedBytes
	}

	// Copy StringData as-is (plain text values)
	if len(intermediate.StringData) > 0 {
		secret.StringData = intermediate.StringData
	}

	return secret, nil
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

// SyncFromGit overwrites K8s secret with Git version (Git is source of truth)
func (d *DriftDetector) SyncFromGit(ctx context.Context, driftEventID uuid.UUID) error {
	// 1. Load drift event from DB
	var driftEvent models.DriftEvent
	if err := d.db.First(&driftEvent, driftEventID).Error; err != nil {
		return fmt.Errorf("failed to load drift event: %w", err)
	}

	// Check if already resolved
	if driftEvent.ResolvedAt != nil {
		return fmt.Errorf("drift event already resolved at %s", driftEvent.ResolvedAt)
	}

	// 2. Load namespace with cluster relationship
	var namespace models.Namespace
	if err := d.db.Preload("ClusterRef").First(&namespace, driftEvent.NamespaceID).Error; err != nil {
		return fmt.Errorf("failed to load namespace: %w", err)
	}

	// Validate cluster exists
	if namespace.ClusterRef == nil {
		return fmt.Errorf("namespace has no cluster association")
	}

	// 3. Get secret YAML from Git repo
	filePath := d.gitClient.GetFilePath(namespace.ClusterRef.Name, namespace.Name, driftEvent.SecretName)
	encryptedYAML, err := d.gitClient.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read secret from Git: %w", err)
	}

	// 4. Decrypt with SOPS
	decryptedYAML, err := d.sopsClient.DecryptYAML(encryptedYAML)
	if err != nil {
		return fmt.Errorf("failed to decrypt secret: %w", err)
	}

	// 5. Validate that decrypted YAML is valid K8s Secret format
	_, err = parseSecretYAML(decryptedYAML)
	if err != nil {
		return fmt.Errorf("failed to parse secret YAML: %w", err)
	}

	// 6. Trigger FluxCD reconciliation (GitOps approach)
	if d.fluxClient == nil {
		return fmt.Errorf("FluxClient not available - cannot trigger GitOps reconciliation")
	}

	// First, trigger GitRepository reconciliation to fetch latest Git content
	logger.Info("Triggering GitRepository reconciliation to fetch latest Git content")
	if err := d.fluxClient.TriggerGitRepositoryReconciliation(ctx, d.cfg.FluxGitRepositoryName, d.cfg.FluxKustomizationNS); err != nil {
		logger.Error("Failed to trigger GitRepository reconciliation", "error", err)
		return fmt.Errorf("failed to trigger GitRepository reconciliation: %w", err)
	}

	// Wait briefly for GitRepository to fetch
	logger.Info("Waiting for GitRepository to fetch latest content")
	time.Sleep(5 * time.Second)

	// Then trigger Kustomization reconciliation to apply secrets
	logger.Info("Triggering Kustomization reconciliation to apply secrets")
	if err := d.fluxClient.TriggerKustomizationReconciliation(ctx, d.cfg.FluxKustomizationName, d.cfg.FluxKustomizationNS); err != nil {
		logger.Error("Failed to trigger Kustomization reconciliation", "error", err)
		return fmt.Errorf("failed to trigger Kustomization reconciliation: %w", err)
	}

	// Wait for Kustomization reconciliation to complete
	logger.Info("Waiting for Kustomization reconciliation to complete", "timeout", d.cfg.FluxReconcileTimeout)
	if err := d.fluxClient.WaitForKustomizationReconciliation(ctx, d.cfg.FluxKustomizationName, d.cfg.FluxKustomizationNS, d.cfg.FluxReconcileTimeout, d.cfg.FluxPollInterval); err != nil {
		logger.Error("Flux reconciliation timeout", "error", err)
		return fmt.Errorf("Flux reconciliation timeout: %w", err)
	}

	// 7. Verify secret was applied to K8s by Flux
	logger.Info("Verifying secret exists in Kubernetes after Flux sync", "namespace", namespace.Name, "secret", driftEvent.SecretName)
	_, err = d.k8sClient.GetSecret(namespace.Name, driftEvent.SecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("secret not found in K8s after Flux reconciliation - Flux may have failed to apply")
		}
		return fmt.Errorf("failed to verify secret in K8s: %w", err)
	}

	logger.Info("Secret successfully synced from Git via FluxCD", "namespace", namespace.Name, "secret", driftEvent.SecretName)

	// 8. Mark drift event as resolved
	now := time.Now()
	resolutionAction := "sync_from_git"
	driftEvent.ResolvedAt = &now
	driftEvent.ResolutionAction = &resolutionAction
	if err := d.db.Save(&driftEvent).Error; err != nil {
		return fmt.Errorf("failed to update drift event: %w", err)
	}

	// 9. Update secret status back to published
	var secret models.SecretDraft
	if err := d.db.Where("namespace_id = ? AND secret_name = ?", driftEvent.NamespaceID, driftEvent.SecretName).
		First(&secret).Error; err == nil {
		if err := d.db.Model(&secret).Update("status", "published").Error; err != nil {
			logger.Error("Failed to update secret status", "error", err)
		}
	}

	// 10. Create audit log entry
	auditLog := models.AuditLog{
		ActionType:   "drift_sync_from_git",
		ResourceType: "secret",
		ResourceName: driftEvent.SecretName,
		NamespaceID:  &driftEvent.NamespaceID,
		Timestamp:    now,
		Metadata: mustMarshalJSON(map[string]interface{}{
			"drift_event_id": driftEvent.ID,
			"namespace":      namespace.Name,
		}),
	}
	if err := d.db.Create(&auditLog).Error; err != nil {
		logger.Error("Failed to create audit log", "error", err)
	}

	logger.Info("Synced secret from Git to Kubernetes",
		"namespace", namespace.Name,
		"secret", driftEvent.SecretName,
		"drift_event_id", driftEvent.ID)

	return nil
}

// ImportToGit imports K8s secret to Git (K8s is source of truth)
func (d *DriftDetector) ImportToGit(ctx context.Context, driftEventID uuid.UUID) error {
	// 1. Load drift event from DB
	var driftEvent models.DriftEvent
	if err := d.db.First(&driftEvent, driftEventID).Error; err != nil {
		return fmt.Errorf("failed to load drift event: %w", err)
	}

	// Check if already resolved
	if driftEvent.ResolvedAt != nil {
		return fmt.Errorf("drift event already resolved at %s", driftEvent.ResolvedAt)
	}

	// 2. Load namespace with cluster relationship
	var namespace models.Namespace
	if err := d.db.Preload("ClusterRef").First(&namespace, driftEvent.NamespaceID).Error; err != nil {
		return fmt.Errorf("failed to load namespace: %w", err)
	}

	// Validate cluster exists
	if namespace.ClusterRef == nil {
		return fmt.Errorf("namespace has no cluster association")
	}

	// 3. Get secret from K8s
	k8sSecret, err := d.k8sClient.GetSecret(namespace.Name, driftEvent.SecretName)
	if err != nil {
		return fmt.Errorf("failed to get secret from Kubernetes: %w", err)
	}

	// 4. Convert to YAML format
	yamlBytes, err := yaml.Marshal(k8sSecret)
	if err != nil {
		return fmt.Errorf("failed to marshal secret to YAML: %w", err)
	}

	// 5. Encrypt with SOPS
	encryptedYAML, err := d.sopsClient.EncryptYAML(yamlBytes)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	// 6. Commit to Git
	filePath := d.gitClient.GetFilePath(namespace.ClusterRef.Name, namespace.Name, driftEvent.SecretName)
	if err := d.gitClient.WriteFile(filePath, encryptedYAML); err != nil {
		return fmt.Errorf("failed to write secret to Git: %w", err)
	}

	commitMsg := fmt.Sprintf("Import secret %s from Kubernetes (drift resolution)", driftEvent.SecretName)
	if _, err := d.gitClient.Commit(commitMsg, "", []string{filePath}); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	if err := d.gitClient.Push(); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// 7. Mark drift event as resolved
	now := time.Now()
	resolutionAction := "import_to_git"
	driftEvent.ResolvedAt = &now
	driftEvent.ResolutionAction = &resolutionAction
	if err := d.db.Save(&driftEvent).Error; err != nil {
		return fmt.Errorf("failed to update drift event: %w", err)
	}

	// 8. Update secret status back to published
	var secret models.SecretDraft
	if err := d.db.Where("namespace_id = ? AND secret_name = ?", driftEvent.NamespaceID, driftEvent.SecretName).
		First(&secret).Error; err == nil {
		if err := d.db.Model(&secret).Update("status", "published").Error; err != nil {
			logger.Error("Failed to update secret status", "error", err)
		}
	}

	// 9. Create audit log entry
	auditLog := models.AuditLog{
		ActionType:   "drift_import_to_git",
		ResourceType: "secret",
		ResourceName: driftEvent.SecretName,
		NamespaceID:  &driftEvent.NamespaceID,
		Timestamp:    now,
		Metadata: mustMarshalJSON(map[string]interface{}{
			"drift_event_id": driftEvent.ID,
			"namespace":      namespace.Name,
		}),
	}
	if err := d.db.Create(&auditLog).Error; err != nil {
		logger.Error("Failed to create audit log", "error", err)
	}

	logger.Info("Imported secret from Kubernetes to Git",
		"namespace", namespace.Name,
		"secret", driftEvent.SecretName,
		"drift_event_id", driftEvent.ID)

	return nil
}

// MarkResolved marks drift as resolved without taking action (manual resolution)
func (d *DriftDetector) MarkResolved(ctx context.Context, driftEventID uuid.UUID, userID uuid.UUID) error {
	// 1. Load drift event
	var driftEvent models.DriftEvent
	if err := d.db.First(&driftEvent, driftEventID).Error; err != nil {
		return fmt.Errorf("failed to load drift event: %w", err)
	}

	// Check if already resolved
	if driftEvent.ResolvedAt != nil {
		return fmt.Errorf("drift event already resolved at %s", driftEvent.ResolvedAt)
	}

	// 2. Update resolved_at and resolved_by
	now := time.Now()
	resolutionAction := "ignore"
	driftEvent.ResolvedAt = &now
	driftEvent.ResolvedBy = &userID
	driftEvent.ResolutionAction = &resolutionAction

	// 3. Save to DB
	if err := d.db.Save(&driftEvent).Error; err != nil {
		return fmt.Errorf("failed to update drift event: %w", err)
	}

	// 4. Update secret status back to published
	var secret models.SecretDraft
	if err := d.db.Where("namespace_id = ? AND secret_name = ?", driftEvent.NamespaceID, driftEvent.SecretName).
		First(&secret).Error; err == nil {
		if err := d.db.Model(&secret).Update("status", "published").Error; err != nil {
			logger.Error("Failed to update secret status", "error", err)
		}
	}

	// 5. Create audit log entry
	auditLog := models.AuditLog{
		UserID:       &userID,
		ActionType:   "drift_mark_resolved",
		ResourceType: "secret",
		ResourceName: driftEvent.SecretName,
		NamespaceID:  &driftEvent.NamespaceID,
		Timestamp:    now,
		Metadata: mustMarshalJSON(map[string]interface{}{
			"drift_event_id": driftEvent.ID,
		}),
	}
	if err := d.db.Create(&auditLog).Error; err != nil {
		logger.Error("Failed to create audit log", "error", err)
	}

	logger.Info("Marked drift event as resolved",
		"drift_event_id", driftEvent.ID,
		"user_id", userID,
		"secret", driftEvent.SecretName)

	return nil
}

// GetComparisonData fetches and decodes secret data from both Git and K8s for visual comparison
// Returns two maps: gitData and k8sData with base64-decoded values
func (d *DriftDetector) GetComparisonData(namespace, secretName string) (map[string]string, map[string]string, error) {
	gitData := make(map[string]string)
	k8sData := make(map[string]string)

	// Load namespace from DB to get cluster name
	var ns models.Namespace
	if err := d.db.Preload("ClusterRef").Where("name = ?", namespace).First(&ns).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to load namespace: %w", err)
	}

	// Validate cluster exists
	if ns.ClusterRef == nil {
		return nil, nil, fmt.Errorf("namespace has no cluster association")
	}

	// 1. Fetch Git version
	filePath := d.gitClient.GetFilePath(ns.ClusterRef.Name, namespace, secretName)
	encryptedYAML, err := d.gitClient.ReadFile(filePath)
	if err != nil {
		// File missing from Git
		logger.Warn("Secret file missing from Git", "namespace", namespace, "secret", secretName, "error", err)
	} else {
		// Decrypt and parse
		decryptedYAML, err := d.sopsClient.DecryptYAML(encryptedYAML)
		if err != nil {
			logger.Error("Failed to decrypt Git secret", "error", err)
		} else {
			k8sSecret, err := parseSecretYAML(decryptedYAML)
			if err != nil {
				logger.Error("Failed to parse Git secret YAML", "error", err)
			} else {
				// Convert []byte values to strings for display
				for key, value := range k8sSecret.Data {
					gitData[key] = string(value)
				}
			}
		}
	}

	// 2. Fetch K8s version
	k8sSecret, err := d.k8sClient.GetSecret(namespace, secretName)
	if err != nil {
		// Secret missing from K8s
		logger.Warn("Secret missing from K8s", "namespace", namespace, "secret", secretName, "error", err)
	} else {
		// Convert []byte values to strings for display
		for key, value := range k8sSecret.Data {
			k8sData[key] = string(value)
		}
	}

	return gitData, k8sData, nil
}
