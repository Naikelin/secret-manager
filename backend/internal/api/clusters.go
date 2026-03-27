package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/k8s"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/gorm"
)

type ClusterHandlers struct {
	db            *gorm.DB
	clientManager k8s.ClientManager
}

func NewClusterHandlers(db *gorm.DB, clientManager k8s.ClientManager) *ClusterHandlers {
	return &ClusterHandlers{
		db:            db,
		clientManager: clientManager,
	}
}

// CreateClusterRequest represents the request body for creating a cluster
type CreateClusterRequest struct {
	Name          string `json:"name" binding:"required"`
	KubeconfigRef string `json:"kubeconfig_ref" binding:"required"`
	Environment   string `json:"environment" binding:"required,oneof=development staging production"`
}

// ListClusters returns all clusters with their health status
// @Summary List all clusters
// @Description Get all Kubernetes clusters configured in the system
// @Tags clusters
// @Accept json
// @Produce json
// @Success 200 {array} models.Cluster "List of clusters"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /clusters [get]
func (h *ClusterHandlers) ListClusters(w http.ResponseWriter, r *http.Request) {
	var clusters []models.Cluster

	// Load all clusters from database
	if err := h.db.Order("name ASC").Find(&clusters).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch clusters: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusters)
}

// GetCluster returns a single cluster by ID
// @Summary Get cluster by ID
// @Description Get details of a specific cluster including health status
// @Tags clusters
// @Accept json
// @Produce json
// @Param id path string true "Cluster ID (UUID)"
// @Success 200 {object} models.Cluster "Cluster details"
// @Failure 400 {object} map[string]string "Invalid cluster ID format"
// @Failure 404 {object} map[string]string "Cluster not found"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /clusters/{id} [get]
func (h *ClusterHandlers) GetCluster(w http.ResponseWriter, r *http.Request) {
	// Parse cluster ID from URL parameter
	clusterIDStr := chi.URLParam(r, "id")
	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		http.Error(w, "Invalid cluster ID format", http.StatusBadRequest)
		return
	}

	var cluster models.Cluster
	if err := h.db.First(&cluster, "id = ?", clusterID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Cluster not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to fetch cluster: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cluster)
}

// CreateCluster creates a new cluster
// @Summary Create a new cluster
// @Description Register a new Kubernetes cluster in the system
// @Tags clusters
// @Accept json
// @Produce json
// @Param cluster body CreateClusterRequest true "Cluster data"
// @Success 201 {object} models.Cluster "Cluster created successfully"
// @Failure 400 {object} map[string]string "Invalid request body or kubeconfig validation failed"
// @Failure 409 {object} map[string]string "Cluster name already exists"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /clusters [post]
func (h *ClusterHandlers) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.KubeconfigRef == "" || req.Environment == "" {
		http.Error(w, "Missing required fields: name, kubeconfig_ref, environment", http.StatusBadRequest)
		return
	}

	// Validate environment enum (must match models.Cluster enum)
	validEnvironments := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  true,
	}
	if !validEnvironments[req.Environment] {
		http.Error(w, "Invalid environment. Must be one of: development, staging, production", http.StatusBadRequest)
		return
	}

	// Validate kubeconfig_ref format (prevent directory traversal)
	if err := validateKubeconfigPath(req.KubeconfigRef); err != nil {
		http.Error(w, fmt.Sprintf("Invalid kubeconfig_ref: %v", err), http.StatusBadRequest)
		return
	}

	// Create cluster model
	cluster := models.Cluster{
		Name:          req.Name,
		KubeconfigRef: req.KubeconfigRef,
		Environment:   req.Environment,
		IsHealthy:     true, // Assume healthy initially
	}

	// Check for duplicate cluster name
	var existingCluster models.Cluster
	err := h.db.Where("name = ?", req.Name).First(&existingCluster).Error
	if err == nil {
		http.Error(w, "Cluster name already exists", http.StatusConflict)
		return
	} else if err != gorm.ErrRecordNotFound {
		http.Error(w, fmt.Sprintf("Failed to check cluster uniqueness: %v", err), http.StatusInternalServerError)
		return
	}

	// Insert into database
	if err := h.db.Create(&cluster).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to create cluster: %v", err), http.StatusInternalServerError)
		return
	}

	// Attempt to initialize the K8s client to validate kubeconfig
	// This will also cache the client for future use
	if _, err := h.clientManager.GetClient(cluster.ID); err != nil {
		// Log the error but don't fail cluster creation
		// The cluster will be marked as unhealthy by the ClientManager
		fmt.Printf("[WARN] Failed to initialize K8s client for cluster %s: %v\n", cluster.Name, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cluster)
}

// DeleteCluster deletes a cluster by ID
// @Summary Delete a cluster
// @Description Delete a cluster from the system. Fails if the cluster has associated namespaces.
// @Tags clusters
// @Accept json
// @Produce json
// @Param id path string true "Cluster ID (UUID)"
// @Success 204 "Cluster deleted successfully"
// @Failure 400 {object} map[string]string "Invalid cluster ID format"
// @Failure 404 {object} map[string]string "Cluster not found"
// @Failure 409 {object} map[string]string "Cluster has namespaces (cannot delete)"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /clusters/{id} [delete]
func (h *ClusterHandlers) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	// Parse cluster ID from URL parameter
	clusterIDStr := chi.URLParam(r, "id")
	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		http.Error(w, "Invalid cluster ID format", http.StatusBadRequest)
		return
	}

	// Check if cluster exists
	var cluster models.Cluster
	if err := h.db.First(&cluster, "id = ?", clusterID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Cluster not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to fetch cluster: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if cluster has associated namespaces
	var namespaceCount int64
	if err := h.db.Model(&models.Namespace{}).Where("cluster_id = ?", clusterID).Count(&namespaceCount).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to check for associated namespaces: %v", err), http.StatusInternalServerError)
		return
	}

	if namespaceCount > 0 {
		http.Error(w, "Cannot delete cluster with existing namespaces. Delete namespaces first.", http.StatusConflict)
		return
	}

	// Remove client from ClientManager cache
	h.clientManager.RemoveClient(clusterID)

	// Delete cluster from database
	if err := h.db.Delete(&cluster).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete cluster: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListClusterNamespaces returns all namespaces in a specific cluster
// @Summary List namespaces in a cluster
// @Description Get all namespaces belonging to a specific cluster
// @Tags clusters
// @Accept json
// @Produce json
// @Param id path string true "Cluster ID (UUID)"
// @Success 200 {array} models.Namespace "List of namespaces in the cluster"
// @Failure 400 {object} map[string]string "Invalid cluster ID format"
// @Failure 404 {object} map[string]string "Cluster not found"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /clusters/{id}/namespaces [get]
func (h *ClusterHandlers) ListClusterNamespaces(w http.ResponseWriter, r *http.Request) {
	// Parse cluster ID from URL parameter
	clusterIDStr := chi.URLParam(r, "id")
	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		http.Error(w, "Invalid cluster ID format", http.StatusBadRequest)
		return
	}

	// Verify cluster exists
	var cluster models.Cluster
	if err := h.db.First(&cluster, "id = ?", clusterID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Cluster not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to fetch cluster: %v", err), http.StatusInternalServerError)
		return
	}

	// Fetch all namespaces for this cluster
	var namespaces []models.Namespace
	if err := h.db.Where("cluster_id = ?", clusterID).Order("name ASC").Find(&namespaces).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch namespaces: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(namespaces)
}

// GetClusterHealth checks the health of a specific cluster
// @Summary Check cluster health
// @Description Perform a health check on a cluster's K8s API connectivity
// @Tags clusters
// @Accept json
// @Produce json
// @Param id path string true "Cluster ID (UUID)"
// @Success 200 {object} map[string]interface{} "Health check result"
// @Failure 400 {object} map[string]string "Invalid cluster ID format"
// @Failure 404 {object} map[string]string "Cluster not found"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /clusters/{id}/health [get]
func (h *ClusterHandlers) GetClusterHealth(w http.ResponseWriter, r *http.Request) {
	// Parse cluster ID from URL parameter
	clusterIDStr := chi.URLParam(r, "id")
	clusterID, err := uuid.Parse(clusterIDStr)
	if err != nil {
		http.Error(w, "Invalid cluster ID format", http.StatusBadRequest)
		return
	}

	// Verify cluster exists
	var cluster models.Cluster
	if err := h.db.First(&cluster, "id = ?", clusterID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Cluster not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to fetch cluster: %v", err), http.StatusInternalServerError)
		return
	}

	// Perform health check via ClientManager
	isHealthy, err := h.clientManager.HealthCheck(clusterID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cluster_id": clusterID,
			"healthy":    false,
			"error":      err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"cluster_id": clusterID,
		"healthy":    isHealthy,
	})
}

// validateKubeconfigPath validates the kubeconfig reference path
// Prevents directory traversal attacks and ensures valid file paths
func validateKubeconfigPath(path string) error {
	// Check for empty path
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Ensure path is relative (not absolute)
	if filepath.IsAbs(path) {
		return fmt.Errorf("path must be relative, not absolute")
	}

	// Check for directory traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains directory traversal sequence")
	}

	// Ensure path is clean (no suspicious sequences)
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		return fmt.Errorf("path contains suspicious sequences")
	}

	// Check for common malicious patterns
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null byte")
	}

	return nil
}
