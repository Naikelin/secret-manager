package middleware

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/internal/rbac"
	"gorm.io/gorm"
)

// RequireRead returns a middleware that checks if user has read permission for a namespace
// The namespaceID can be provided directly, or extracted from a function that gets it from the request
func RequireRead(db *gorm.DB, getNamespaceID func(r *http.Request) (uuid.UUID, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !checkPermission(w, r, db, getNamespaceID, rbac.CanReadSecret, "read") {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireWrite returns a middleware that checks if user has write permission for a namespace
func RequireWrite(db *gorm.DB, getNamespaceID func(r *http.Request) (uuid.UUID, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !checkPermission(w, r, db, getNamespaceID, rbac.CanWriteSecret, "write") {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePublish returns a middleware that checks if user has publish permission for a namespace
func RequirePublish(db *gorm.DB, getNamespaceID func(r *http.Request) (uuid.UUID, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !checkPermission(w, r, db, getNamespaceID, rbac.CanPublishSecret, "publish") {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireDelete returns a middleware that checks if user has delete permission for a namespace
func RequireDelete(db *gorm.DB, getNamespaceID func(r *http.Request) (uuid.UUID, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !checkPermission(w, r, db, getNamespaceID, rbac.CanDeleteSecret, "delete") {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin returns a middleware that checks if user has admin permission for a namespace
func RequireAdmin(db *gorm.DB, getNamespaceID func(r *http.Request) (uuid.UUID, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !checkPermission(w, r, db, getNamespaceID, rbac.CanManageNamespace, "admin") {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// checkPermission is a helper that performs the common permission check logic
// Returns true if permission check passes, false if it fails (and writes error response)
func checkPermission(
	w http.ResponseWriter,
	r *http.Request,
	db *gorm.DB,
	getNamespaceID func(r *http.Request) (uuid.UUID, error),
	checkFunc func([]models.GroupPermission, uuid.UUID) bool,
	action string,
) bool {
	// Get user from context (set by JWTMiddleware)
	userCtx, err := GetUserFromContext(r.Context())
	if err != nil {
		slog.Warn("Permission check failed: user not in context",
			"action", action,
			"error", err,
		)
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return false
	}

	// Get namespace ID from request
	namespaceID, err := getNamespaceID(r)
	if err != nil {
		slog.Warn("Permission check failed: invalid namespace ID",
			"user_id", userCtx.UserID,
			"action", action,
			"error", err,
		)
		respondError(w, http.StatusBadRequest, "Invalid namespace ID")
		return false
	}

	// Load user permissions
	permissions, err := rbac.GetUserPermissions(db, userCtx.UserID)
	if err != nil {
		slog.Error("Failed to load user permissions",
			"user_id", userCtx.UserID,
			"action", action,
			"error", err,
		)
		respondError(w, http.StatusInternalServerError, "Failed to check permissions")
		return false
	}

	// Check if user has required permission
	if !checkFunc(permissions, namespaceID) {
		slog.Warn("Permission denied",
			"user_id", userCtx.UserID,
			"email", userCtx.Email,
			"namespace_id", namespaceID,
			"action", action,
		)
		respondError(w, http.StatusForbidden, "You do not have permission to perform this action")
		return false
	}

	// Permission check passed
	return true
}
