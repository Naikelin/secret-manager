package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/flux"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/gorm"
)

// FluxClientInterface defines methods for interacting with FluxCD
type FluxClientInterface interface {
	GetKustomizationStatus(name, namespace string) (*flux.KustomizationStatus, error)
	ListKustomizations(namespace string) ([]flux.KustomizationStatus, error)
}

// SyncHandlers handles FluxCD sync status endpoints
type SyncHandlers struct {
	db         *gorm.DB
	fluxClient FluxClientInterface
}

// NewSyncHandlers creates a new SyncHandlers instance
func NewSyncHandlers(db *gorm.DB, fluxClient FluxClientInterface) *SyncHandlers {
	return &SyncHandlers{
		db:         db,
		fluxClient: fluxClient,
	}
}

// SecretSyncInfo represents sync status for a single secret
type SecretSyncInfo struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	CommitSHA   string `json:"commit_sha"`
	SyncedToK8s bool   `json:"synced_to_k8s"`
}

// NamespaceSyncStatus represents the complete sync status for a namespace
type NamespaceSyncStatus struct {
	Namespace         string           `json:"namespace"`
	FluxReady         bool             `json:"flux_ready"`
	LastSyncTime      string           `json:"last_sync_time,omitempty"`
	LastAppliedCommit string           `json:"last_applied_commit"`
	Message           string           `json:"message,omitempty"`
	Secrets           []SecretSyncInfo `json:"secrets"`
}

// GetSyncStatus handles GET /api/v1/namespaces/{namespace}/sync-status
// Returns FluxCD sync status for a namespace
func (h *SyncHandlers) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	// Parse namespace ID from URL
	namespaceIDStr := chi.URLParam(r, "namespace")
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid namespace ID"}`, http.StatusBadRequest)
		return
	}

	// Fetch namespace from database
	var namespace models.Namespace
	if err := h.db.First(&namespace, "id = ?", namespaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, `{"error": "Namespace not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		return
	}

	// Initialize response
	response := NamespaceSyncStatus{
		Namespace: namespace.Name,
		FluxReady: false,
		Secrets:   []SecretSyncInfo{},
	}

	// Query FluxCD if client is available
	if h.fluxClient != nil {
		kustomizationName := fmt.Sprintf("secrets-%s", namespace.Name)
		fluxStatus, err := h.fluxClient.GetKustomizationStatus(kustomizationName, "flux-system")
		if err == nil {
			response.FluxReady = fluxStatus.Ready
			response.LastAppliedCommit = fluxStatus.LastAppliedCommit
			response.Message = fluxStatus.Message
			if !fluxStatus.LastSyncTime.IsZero() {
				response.LastSyncTime = fluxStatus.LastSyncTime.Format("2006-01-02T15:04:05Z07:00")
			}
		}
		// If error querying FluxCD, leave FluxReady as false but continue
	}

	// Fetch all published secrets for this namespace
	var secrets []models.SecretDraft
	if err := h.db.Where("namespace_id = ? AND status = ?", namespaceID, "published").Find(&secrets).Error; err != nil {
		http.Error(w, `{"error": "Failed to fetch secrets"}`, http.StatusInternalServerError)
		return
	}

	// Build secret sync info
	for _, secret := range secrets {
		syncInfo := SecretSyncInfo{
			Name:        secret.SecretName,
			Status:      secret.Status,
			CommitSHA:   secret.CommitSHA,
			SyncedToK8s: false,
		}

		// Check if secret's commit SHA matches FluxCD's last applied commit
		if response.LastAppliedCommit != "" && secret.CommitSHA != "" {
			// If the secret's commit SHA is a prefix of the last applied commit, it's synced
			// (Git commits can be represented as short SHAs)
			if len(secret.CommitSHA) <= len(response.LastAppliedCommit) {
				syncInfo.SyncedToK8s = (response.LastAppliedCommit[:len(secret.CommitSHA)] == secret.CommitSHA)
			} else if len(response.LastAppliedCommit) <= len(secret.CommitSHA) {
				syncInfo.SyncedToK8s = (secret.CommitSHA[:len(response.LastAppliedCommit)] == response.LastAppliedCommit)
			}
		}

		response.Secrets = append(response.Secrets, syncInfo)
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
