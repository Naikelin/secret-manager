package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/middleware"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	maxSecretDataSize = 1024 * 1024 // 1MB
)

// DNS-1123 subdomain format: lowercase alphanumeric, hyphens, max 253 chars
var dns1123Regex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// SecretHandlers contains handlers for secret operations
type SecretHandlers struct {
	db *gorm.DB
}

// NewSecretHandlers creates a new SecretHandlers instance
func NewSecretHandlers(db *gorm.DB) *SecretHandlers {
	return &SecretHandlers{db: db}
}

// CreateSecretRequest represents the request body for creating a secret
type CreateSecretRequest struct {
	Name string                 `json:"name"`
	Data map[string]interface{} `json:"data"`
}

// UpdateSecretRequest represents the request body for updating a secret
type UpdateSecretRequest struct {
	Data map[string]interface{} `json:"data"`
}

// CreateSecret handles POST /api/v1/namespaces/{namespace}/secrets
// @Summary Create a new secret
// @Description Creates a new secret draft in the specified namespace
// @Tags secrets
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID (UUID)"
// @Param secret body CreateSecretRequest true "Secret data"
// @Success 201 {object} models.SecretDraft "Created secret"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Authentication required"
// @Failure 404 {object} map[string]string "Namespace not found"
// @Failure 409 {object} map[string]string "Secret already exists"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces/{namespace}/secrets [post]
func (h *SecretHandlers) CreateSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespaceIDStr := chi.URLParam(r, "namespace")

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

	// Parse request body
	var req CreateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := validateSecretRequest(req.Name, req.Data); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check if namespace exists
	var namespace models.Namespace
	if err := h.db.First(&namespace, "id = ?", namespaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Namespace not found")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to check namespace")
		}
		return
	}

	// Check if secret with same name already exists in this namespace
	var existing models.SecretDraft
	if err := h.db.Where("secret_name = ? AND namespace_id = ?", req.Name, namespaceID).First(&existing).Error; err == nil {
		respondError(w, http.StatusConflict, "Secret with this name already exists in the namespace")
		return
	} else if err != gorm.ErrRecordNotFound {
		respondError(w, http.StatusInternalServerError, "Failed to check existing secret")
		return
	}

	// Convert data to JSON
	dataJSON, err := json.Marshal(req.Data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to serialize secret data")
		return
	}

	// Create secret draft
	secret := models.SecretDraft{
		SecretName:  req.Name,
		NamespaceID: namespaceID,
		Data:        datatypes.JSON(dataJSON),
		Status:      "draft",
		EditedBy:    &userCtx.UserID,
		EditedAt:    time.Now(),
	}

	if err := h.db.Create(&secret).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create secret")
		return
	}

	// Return created secret
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(secret)
}

// ListSecrets handles GET /api/v1/namespaces/{namespace}/secrets
// @Summary List secrets in namespace
// @Description Get all secrets in a namespace with optional status filter
// @Tags secrets
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID (UUID)"
// @Param status query string false "Filter by status (draft, published, drifted)"
// @Success 200 {array} models.SecretDraft "List of secrets"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "Namespace not found"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces/{namespace}/secrets [get]
func (h *SecretHandlers) ListSecrets(w http.ResponseWriter, r *http.Request) {
	namespaceIDStr := chi.URLParam(r, "namespace")

	// Parse namespace ID
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid namespace ID")
		return
	}

	// Check if namespace exists
	var namespace models.Namespace
	if err := h.db.First(&namespace, "id = ?", namespaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Namespace not found")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to check namespace")
		}
		return
	}

	// Build query
	query := h.db.Where("namespace_id = ?", namespaceID)

	// Optional status filter
	status := r.URL.Query().Get("status")
	if status != "" {
		if status != "draft" && status != "published" && status != "drifted" {
			respondError(w, http.StatusBadRequest, "Invalid status filter. Must be one of: draft, published, drifted")
			return
		}
		query = query.Where("status = ?", status)
	}

	// Fetch secrets
	var secrets []models.SecretDraft
	if err := query.Order("created_at DESC").Find(&secrets).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch secrets")
		return
	}

	// Return empty array instead of null if no results
	if secrets == nil {
		secrets = []models.SecretDraft{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(secrets)
}

// GetSecret handles GET /api/v1/namespaces/{namespace}/secrets/{name}
// @Summary Get secret by name
// @Description Retrieve a specific secret by name from a namespace
// @Tags secrets
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID (UUID)"
// @Param name path string true "Secret name"
// @Success 200 {object} models.SecretDraft "Secret details"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "Secret not found"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces/{namespace}/secrets/{name} [get]
func (h *SecretHandlers) GetSecret(w http.ResponseWriter, r *http.Request) {
	namespaceIDStr := chi.URLParam(r, "namespace")
	secretName := chi.URLParam(r, "name")

	// Parse namespace ID
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid namespace ID")
		return
	}

	// Fetch secret
	var secret models.SecretDraft
	if err := h.db.Where("secret_name = ? AND namespace_id = ?", secretName, namespaceID).First(&secret).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Secret not found")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to fetch secret")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(secret)
}

// UpdateSecret handles PUT /api/v1/namespaces/{namespace}/secrets/{name}
// @Summary Update secret
// @Description Update secret data. Reverts published/drifted secrets to draft status automatically.
// @Tags secrets
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID (UUID)"
// @Param name path string true "Secret name"
// @Param secret body UpdateSecretRequest true "Updated secret data"
// @Success 200 {object} models.SecretDraft "Updated secret"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Authentication required"
// @Failure 404 {object} map[string]string "Secret not found"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces/{namespace}/secrets/{name} [put]
func (h *SecretHandlers) UpdateSecret(w http.ResponseWriter, r *http.Request) {
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

	// Parse request body
	var req UpdateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate data
	if err := validateSecretData(req.Data); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch existing secret
	var secret models.SecretDraft
	if err := h.db.Where("secret_name = ? AND namespace_id = ?", secretName, namespaceID).First(&secret).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Secret not found")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to fetch secret")
		}
		return
	}

	// If secret is not in draft status, automatically revert it to draft
	// This allows editing published/drifted secrets by creating a new draft version
	originalStatus := secret.Status
	statusChanged := false
	if secret.Status != "draft" {
		secret.Status = "draft"
		statusChanged = true
	}

	// Convert data to JSON
	dataJSON, err := json.Marshal(req.Data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to serialize secret data")
		return
	}

	// Update secret
	secret.Data = datatypes.JSON(dataJSON)
	secret.EditedBy = &userCtx.UserID
	secret.EditedAt = time.Now()

	if err := h.db.Save(&secret).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update secret")
		return
	}

	// Create audit log if status was changed from published/drifted to draft
	if statusChanged {
		auditDetails := map[string]interface{}{
			"secret_name":     secret.SecretName,
			"original_status": originalStatus,
			"new_status":      "draft",
			"action":          "edit_revert_to_draft",
		}
		detailsJSON, err := json.Marshal(auditDetails)
		if err == nil {
			auditLog := models.AuditLog{
				UserID:       &userCtx.UserID,
				ActionType:   "update_secret",
				ResourceType: "secret",
				ResourceName: secret.SecretName,
				NamespaceID:  &namespaceID,
				Timestamp:    time.Now(),
				Metadata:     detailsJSON,
			}
			if err := h.db.Create(&auditLog).Error; err != nil {
				logger.Error("Failed to create audit log for status change", "error", err)
				// Don't fail the request if audit log creation fails
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(secret)
}

// DeleteSecret handles DELETE /api/v1/namespaces/{namespace}/secrets/{name}
// @Summary Delete secret
// @Description Delete a secret from the database. Note: Deleting published secrets may cause drift.
// @Tags secrets
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace ID (UUID)"
// @Param name path string true "Secret name"
// @Success 204 "Secret deleted successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Authentication required"
// @Failure 404 {object} map[string]string "Secret not found"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces/{namespace}/secrets/{name} [delete]
func (h *SecretHandlers) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespaceIDStr := chi.URLParam(r, "namespace")
	secretName := chi.URLParam(r, "name")

	// Parse namespace ID
	namespaceID, err := uuid.Parse(namespaceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid namespace ID")
		return
	}

	// Get user from context for audit logging
	userCtx, err := middleware.GetUserFromContext(ctx)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Fetch existing secret
	var secret models.SecretDraft
	if err := h.db.Where("secret_name = ? AND namespace_id = ?", secretName, namespaceID).First(&secret).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Secret not found")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to fetch secret")
		}
		return
	}

	// Allow deletion of secrets in any status
	// Note: Deleting a published secret will cause drift until unpublished from Git

	// Create audit log before deletion
	auditDetails := map[string]interface{}{
		"secret_name": secret.SecretName,
		"status":      secret.Status,
	}
	detailsJSON, err := json.Marshal(auditDetails)
	if err == nil {
		auditLog := models.AuditLog{
			UserID:       &userCtx.UserID,
			ActionType:   "delete_secret",
			ResourceType: "secret",
			ResourceName: secret.SecretName,
			NamespaceID:  &namespaceID,
			Timestamp:    time.Now(),
			Metadata:     detailsJSON,
		}
		if err := h.db.Create(&auditLog).Error; err != nil {
			logger.Error("Failed to create audit log for deletion", "error", err)
			// Don't fail the request if audit log creation fails
		}
	}

	// Delete secret
	if err := h.db.Delete(&secret).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete secret")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// validateSecretRequest validates the secret creation request
func validateSecretRequest(name string, data map[string]interface{}) error {
	// Validate name
	if name == "" {
		return fmt.Errorf("secret name is required")
	}

	if len(name) > 253 {
		return fmt.Errorf("secret name must not exceed 253 characters")
	}

	if !dns1123Regex.MatchString(name) {
		return fmt.Errorf("secret name must follow DNS-1123 subdomain format (lowercase alphanumeric with hyphens)")
	}

	// Validate data
	return validateSecretData(data)
}

// validateSecretData validates the secret data
func validateSecretData(data map[string]interface{}) error {
	if data == nil || len(data) == 0 {
		return fmt.Errorf("secret data cannot be empty")
	}

	// Calculate total data size
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("invalid secret data format")
	}

	if len(dataJSON) > maxSecretDataSize {
		return fmt.Errorf("secret data size exceeds maximum allowed size of 1MB")
	}

	return nil
}

func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
