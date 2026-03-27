package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/middleware"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/gorm"
)

type NamespaceHandlers struct {
	db *gorm.DB
}

func NewNamespaceHandlers(db *gorm.DB) *NamespaceHandlers {
	return &NamespaceHandlers{db: db}
}

// ListNamespaces returns all namespaces the authenticated user has access to
// @Summary List accessible namespaces
// @Description Get all namespaces the authenticated user has access to via their groups. Optional cluster_id query param filters by cluster.
// @Tags namespaces
// @Accept json
// @Produce json
// @Param cluster_id query string false "Filter by cluster ID (UUID)"
// @Success 200 {array} models.Namespace "List of namespaces"
// @Failure 400 {object} map[string]string "Invalid cluster ID format"
// @Failure 401 {object} map[string]string "Authentication required"
// @Failure 500 {object} map[string]string "Server error"
// @Security BearerAuth
// @Router /namespaces [get]
func (h *NamespaceHandlers) ListNamespaces(w http.ResponseWriter, r *http.Request) {
	log.Printf("[NamespaceHandler] ListNamespaces called")

	// Get user from context (set by JWT middleware)
	user, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("[NamespaceHandler] Failed to get user from context: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("[NamespaceHandler] User from context: %s (%s)", user.Email, user.UserID)

	// Optional cluster_id filter
	clusterIDParam := r.URL.Query().Get("cluster_id")

	namespaces := []models.Namespace{}

	// Build query with optional cluster filter
	query := `
		SELECT DISTINCT n.* 
		FROM namespaces n
		INNER JOIN group_permissions gp ON gp.namespace_id = n.id
		INNER JOIN user_groups ug ON ug.group_id = gp.group_id
		WHERE ug.user_id = ?`

	args := []interface{}{user.UserID}

	// Add cluster filter if provided
	if clusterIDParam != "" {
		// Validate cluster ID format
		clusterID, err := uuid.Parse(clusterIDParam)
		if err != nil {
			http.Error(w, "Invalid cluster_id format", http.StatusBadRequest)
			return
		}
		query += ` AND n.cluster_id = ?`
		args = append(args, clusterID)
	}

	query += ` ORDER BY n.name`

	// Execute query
	err = h.db.Raw(query, args...).Scan(&namespaces).Error
	if err != nil {
		http.Error(w, "failed to fetch namespaces", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(namespaces)
}
