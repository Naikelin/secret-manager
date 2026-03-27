package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/secret-manager/internal/k8s"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
)

// K8sSecretHandlers contains handlers for K8s secret operations
type K8sSecretHandlers struct {
	db            *gorm.DB
	clientManager k8s.ClientManager
}

// K8sSecretInfo represents a K8s secret metadata (without actual data values)
type K8sSecretInfo struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Type            string            `json:"type"`
	CreatedAt       time.Time         `json:"created_at"`
	DataKeys        []string          `json:"data_keys"`
	ManagedByGitOps bool              `json:"managed_by_gitops"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

// K8sSecretsListResponse represents the list of secrets in a namespace
type K8sSecretsListResponse struct {
	Namespace string          `json:"namespace"`
	Secrets   []K8sSecretInfo `json:"secrets"`
}

// NewK8sSecretHandlers creates new K8s secret handlers
func NewK8sSecretHandlers(db *gorm.DB, clientManager k8s.ClientManager) *K8sSecretHandlers {
	return &K8sSecretHandlers{
		db:            db,
		clientManager: clientManager,
	}
}

// ListK8sSecrets lists all secrets in a K8s namespace
// GET /api/v1/namespaces/{namespace}/k8s-secrets
// @Summary List Kubernetes secrets
// @Description List all secrets currently deployed in the Kubernetes namespace (metadata only, no values)
// @Tags kubernetes
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID (UUID)"
// @Success 200 {object} K8sSecretsListResponse "List of Kubernetes secrets"
// @Failure 404 {object} map[string]string "Namespace not found"
// @Failure 503 {object} map[string]string "Kubernetes cluster not available"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces/{namespace}/k8s-secrets [get]
func (h *K8sSecretHandlers) ListK8sSecrets(w http.ResponseWriter, r *http.Request) {
	// Get namespace ID from route params
	namespaceIDStr := chi.URLParam(r, "namespace")

	// Look up namespace by ID to get the actual K8s namespace name and cluster
	var namespace models.Namespace
	if err := h.db.Where("id = ?", namespaceIDStr).First(&namespace).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, `{"error":"Namespace not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}

	// Check if namespace has a cluster assigned
	if namespace.ClusterID == nil {
		http.Error(w, `{"error":"Namespace has no cluster assigned"}`, http.StatusBadRequest)
		return
	}

	// Get K8s client for the namespace's cluster
	k8sClient, err := h.clientManager.GetClient(*namespace.ClusterID)
	if err != nil {
		http.Error(w, `{"error":"Kubernetes cluster not available"}`, http.StatusServiceUnavailable)
		return
	}

	// List secrets from K8s
	k8sSecrets, err := k8sClient.ListSecrets(namespace.Name)
	if err != nil {
		http.Error(w, `{"error":"Failed to list secrets from Kubernetes"}`, http.StatusInternalServerError)
		return
	}

	// Get all published secrets from our database for this namespace
	var publishedSecrets []models.SecretDraft
	h.db.Where("namespace_id = ? AND status = ?", namespace.ID, "published").Find(&publishedSecrets)

	// Build map of published secret names for quick lookup
	managedSecrets := make(map[string]bool)
	for _, secret := range publishedSecrets {
		managedSecrets[secret.SecretName] = true
	}

	// Convert K8s secrets to response format
	secretInfos := make([]K8sSecretInfo, 0, len(k8sSecrets))
	for _, k8sSecret := range k8sSecrets {
		secretInfos = append(secretInfos, convertToK8sSecretInfo(&k8sSecret, managedSecrets))
	}

	response := K8sSecretsListResponse{
		Namespace: namespace.Name,
		Secrets:   secretInfos,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetK8sSecret retrieves a single K8s secret details (metadata only, no data values)
// GET /api/v1/namespaces/{namespace}/k8s-secrets/{name}
// @Summary Get Kubernetes secret details
// @Description Get metadata for a specific Kubernetes secret (keys only, no values)
// @Tags kubernetes
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID (UUID)"
// @Param name path string true "Secret name"
// @Success 200 {object} K8sSecretInfo "Kubernetes secret metadata"
// @Failure 404 {object} map[string]string "Namespace or secret not found"
// @Failure 503 {object} map[string]string "Kubernetes cluster not available"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces/{namespace}/k8s-secrets/{name} [get]
func (h *K8sSecretHandlers) GetK8sSecret(w http.ResponseWriter, r *http.Request) {
	// Get params from route
	namespaceIDStr := chi.URLParam(r, "namespace")
	secretName := chi.URLParam(r, "name")

	// Look up namespace by ID to get cluster
	var namespace models.Namespace
	if err := h.db.Where("id = ?", namespaceIDStr).First(&namespace).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, `{"error":"Namespace not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}

	// Check if namespace has a cluster assigned
	if namespace.ClusterID == nil {
		http.Error(w, `{"error":"Namespace has no cluster assigned"}`, http.StatusBadRequest)
		return
	}

	// Get K8s client for the namespace's cluster
	k8sClient, err := h.clientManager.GetClient(*namespace.ClusterID)
	if err != nil {
		http.Error(w, `{"error":"Kubernetes cluster not available"}`, http.StatusServiceUnavailable)
		return
	}

	// Get secret from K8s
	k8sSecret, err := k8sClient.GetSecret(namespace.Name, secretName)
	if err != nil {
		http.Error(w, `{"error":"Secret not found in Kubernetes"}`, http.StatusNotFound)
		return
	}

	// Check if secret is managed by our GitOps system
	var publishedSecret models.SecretDraft
	managedByGitOps := false
	if err := h.db.Where("namespace_id = ? AND secret_name = ? AND status = ?",
		namespace.ID, secretName, "published").First(&publishedSecret).Error; err == nil {
		managedByGitOps = true
	}

	managedSecrets := map[string]bool{secretName: managedByGitOps}
	secretInfo := convertToK8sSecretInfo(k8sSecret, managedSecrets)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(secretInfo)
}

// convertToK8sSecretInfo converts a K8s secret to our API response format
// It excludes actual secret data values for security
func convertToK8sSecretInfo(k8sSecret *corev1.Secret, managedSecrets map[string]bool) K8sSecretInfo {
	// Extract keys from secret data (but not values!)
	dataKeys := make([]string, 0, len(k8sSecret.Data))
	for key := range k8sSecret.Data {
		dataKeys = append(dataKeys, key)
	}

	// Check if this secret is managed by GitOps
	managedByGitOps := managedSecrets[k8sSecret.Name]

	return K8sSecretInfo{
		Name:            k8sSecret.Name,
		Namespace:       k8sSecret.Namespace,
		Type:            string(k8sSecret.Type),
		CreatedAt:       k8sSecret.CreationTimestamp.Time,
		DataKeys:        dataKeys,
		ManagedByGitOps: managedByGitOps,
		Labels:          k8sSecret.Labels,
		Annotations:     k8sSecret.Annotations,
	}
}
