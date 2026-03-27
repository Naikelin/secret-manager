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
	GetGitRepositoryStatus(name, namespace string) (*flux.GitRepositoryStatus, error)
}

// SyncHandlers handles FluxCD sync status endpoints
type SyncHandlers struct {
	db                         *gorm.DB
	fluxClient                 FluxClientInterface
	gitClient                  GitClientInterface
	fluxKustomizationName      string
	fluxGitRepositoryName      string
	fluxGitRepositoryNamespace string
}

// NewSyncHandlers creates a new SyncHandlers instance
func NewSyncHandlers(db *gorm.DB, fluxClient FluxClientInterface, gitClient GitClientInterface, fluxKustomizationName, fluxGitRepositoryName, fluxGitRepositoryNamespace string) *SyncHandlers {
	return &SyncHandlers{
		db:                         db,
		fluxClient:                 fluxClient,
		gitClient:                  gitClient,
		fluxKustomizationName:      fluxKustomizationName,
		fluxGitRepositoryName:      fluxGitRepositoryName,
		fluxGitRepositoryNamespace: fluxGitRepositoryNamespace,
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
	// New fields for Phase 17 frontend compatibility
	Namespace  string  `json:"namespace"`
	GitCommit  string  `json:"git_commit"`
	FluxCommit string  `json:"flux_commit"`
	Synced     bool    `json:"synced"`
	LastSync   string  `json:"last_sync"`
	Error      *string `json:"error"`

	// Backward compatibility fields (Phase 9)
	FluxReady         bool             `json:"flux_ready"`
	LastSyncTime      string           `json:"last_sync_time,omitempty"`
	LastAppliedCommit string           `json:"last_applied_commit"`
	Message           string           `json:"message,omitempty"`
	Secrets           []SecretSyncInfo `json:"secrets"`
}

// GetSyncStatus handles GET /api/v1/namespaces/{namespace}/sync-status
// Returns FluxCD sync status for a namespace
// @Summary Get FluxCD sync status
// @Description Get GitOps synchronization status between Git and Kubernetes via FluxCD
// @Tags sync
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID (UUID)"
// @Success 200 {object} NamespaceSyncStatus "Sync status details"
// @Failure 400 {object} map[string]string "Invalid namespace ID"
// @Failure 404 {object} map[string]string "Namespace not found"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces/{namespace}/sync-status [get]
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

	// Get Git commit SHA from FluxCD GitRepository if client is available
	var gitCommit string
	var gitError *string
	if h.fluxClient != nil {
		gitRepoStatus, err := h.fluxClient.GetGitRepositoryStatus(h.fluxGitRepositoryName, h.fluxGitRepositoryNamespace)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get GitRepository status: %v", err)
			gitError = &errMsg
		} else if gitRepoStatus != nil {
			gitCommit = gitRepoStatus.LastFetchedCommit
		}
	} else {
		// FluxCD not configured
		errMsg := "FluxCD not configured"
		gitError = &errMsg
	}
	response.GitCommit = gitCommit
	response.Error = gitError

	// Query FluxCD if client is available
	if h.fluxClient != nil {
		// Use configured Kustomization name (single Kustomization manages all namespaces)
		kustomizationName := h.fluxKustomizationName
		fluxStatus, err := h.fluxClient.GetKustomizationStatus(kustomizationName, "flux-system")
		if err == nil {
			response.FluxReady = fluxStatus.Ready
			response.LastAppliedCommit = fluxStatus.LastAppliedCommit
			response.FluxCommit = fluxStatus.LastAppliedCommit // Alias for frontend
			response.Message = fluxStatus.Message
			if !fluxStatus.LastSyncTime.IsZero() {
				lastSyncTime := fluxStatus.LastSyncTime.Format("2006-01-02T15:04:05Z07:00")
				response.LastSyncTime = lastSyncTime
				response.LastSync = lastSyncTime // Alias for frontend
			}

			// Determine if Git and Flux are synced
			if response.GitCommit != "" && response.FluxCommit != "" {
				// Compare full SHAs or handle short SHAs
				response.Synced = compareSHAs(response.GitCommit, response.FluxCommit)
			} else {
				response.Synced = false
			}
		} else {
			// If error querying FluxCD, set error message
			if response.Error != nil {
				// Append FluxCD error to existing Git error
				errMsg := fmt.Sprintf("%s; FluxCD error: %v", *response.Error, err)
				response.Error = &errMsg
			} else {
				errMsg := fmt.Sprintf("FluxCD error: %v", err)
				response.Error = &errMsg
			}
		}
		// If error querying FluxCD, leave FluxReady as false but continue
	} else {
		// FluxCD client not available
		if response.Error != nil {
			errMsg := fmt.Sprintf("%s; FluxCD not configured", *response.Error)
			response.Error = &errMsg
		} else {
			errMsg := "FluxCD not configured"
			response.Error = &errMsg
		}
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

// compareSHAs compares two Git commit SHAs, handling short SHA prefixes
func compareSHAs(sha1, sha2 string) bool {
	if sha1 == "" || sha2 == "" {
		return false
	}

	// If one is a prefix of the other, they match
	if len(sha1) <= len(sha2) {
		return sha2[:len(sha1)] == sha1
	}
	return sha1[:len(sha2)] == sha2
}
