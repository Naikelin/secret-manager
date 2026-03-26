package api

import (
	"encoding/json"
	"log"
	"net/http"

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
// @Description Get all namespaces the authenticated user has access to via their groups
// @Tags namespaces
// @Accept json
// @Produce json
// @Success 200 {array} models.Namespace "List of namespaces"
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

	namespaces := []models.Namespace{}

	// Get all namespaces the user has access to via their groups
	err = h.db.Raw(`
		SELECT DISTINCT n.* 
		FROM namespaces n
		INNER JOIN group_permissions gp ON gp.namespace_id = n.id
		INNER JOIN user_groups ug ON ug.group_id = gp.group_id
		WHERE ug.user_id = ?
		ORDER BY n.name
	`, user.UserID).Scan(&namespaces).Error

	if err != nil {
		http.Error(w, "failed to fetch namespaces", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(namespaces)
}
