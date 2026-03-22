package rbac

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/models"
	"gorm.io/gorm"
)

// Role represents an RBAC role
type Role string

const (
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleAdmin  Role = "admin"
)

// roleHierarchy defines the hierarchy levels for comparison
var roleHierarchy = map[Role]int{
	RoleViewer: 1,
	RoleEditor: 2,
	RoleAdmin:  3,
}

// hasRole checks if the user has at least the required role in the namespace
func hasRole(userGroups []models.GroupPermission, namespaceID uuid.UUID, requiredRole Role) bool {
	requiredLevel := roleHierarchy[requiredRole]

	for _, gp := range userGroups {
		if gp.NamespaceID == namespaceID {
			userLevel := roleHierarchy[Role(gp.Role)]
			if userLevel >= requiredLevel {
				return true
			}
		}
	}
	return false
}

// CanReadSecret checks if user can read secrets in a namespace
// Permission: viewer, editor, or admin
func CanReadSecret(userGroups []models.GroupPermission, namespaceID uuid.UUID) bool {
	return hasRole(userGroups, namespaceID, RoleViewer)
}

// CanWriteSecret checks if user can create/update secrets in a namespace
// Permission: editor or admin (viewers cannot write)
func CanWriteSecret(userGroups []models.GroupPermission, namespaceID uuid.UUID) bool {
	return hasRole(userGroups, namespaceID, RoleEditor)
}

// CanPublishSecret checks if user can publish secrets to Git in a namespace
// Permission: editor or admin (viewers cannot publish)
func CanPublishSecret(userGroups []models.GroupPermission, namespaceID uuid.UUID) bool {
	return hasRole(userGroups, namespaceID, RoleEditor)
}

// CanDeleteSecret checks if user can delete secrets in a namespace
// Permission: admin only (editors and viewers cannot delete)
func CanDeleteSecret(userGroups []models.GroupPermission, namespaceID uuid.UUID) bool {
	return hasRole(userGroups, namespaceID, RoleAdmin)
}

// CanManageNamespace checks if user can manage namespace settings and permissions
// Permission: admin only
func CanManageNamespace(userGroups []models.GroupPermission, namespaceID uuid.UUID) bool {
	return hasRole(userGroups, namespaceID, RoleAdmin)
}

// GetUserPermissions retrieves all group permissions for a user
// This loads the user's groups and then fetches all permissions for those groups
func GetUserPermissions(db *gorm.DB, userID uuid.UUID) ([]models.GroupPermission, error) {
	var user models.User

	// Load user with their groups
	if err := db.Preload("Groups").First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("failed to load user: %w", err)
	}

	// If user has no groups, return empty permissions
	if len(user.Groups) == 0 {
		return []models.GroupPermission{}, nil
	}

	// Extract group IDs
	groupIDs := make([]uuid.UUID, len(user.Groups))
	for i, group := range user.Groups {
		groupIDs[i] = group.ID
	}

	// Load all permissions for these groups
	var permissions []models.GroupPermission
	if err := db.Where("group_id IN ?", groupIDs).Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("failed to load group permissions: %w", err)
	}

	return permissions, nil
}
