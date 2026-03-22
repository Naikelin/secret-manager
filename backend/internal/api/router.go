package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yourorg/secret-manager/internal/config"
	mw "github.com/yourorg/secret-manager/internal/middleware"
	"gorm.io/gorm"
)

// NewRouter creates and configures the HTTP router
func NewRouter(db *gorm.DB, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(mw.RequestLogger) // Custom structured logging middleware
	r.Use(middleware.Recoverer)

	// CORS configuration
	r.Use(mw.NewCORSHandler())

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Initialize auth handlers
	authHandlers := NewAuthHandlers(db, cfg)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public)
		r.Post("/auth/login", authHandlers.HandleLogin)
		r.Get("/auth/callback", authHandlers.HandleCallback)
		r.Get("/auth/mock-callback", authHandlers.HandleMockCallback) // Mock callback (dev only) - bypasses external OAuth2 server
		r.Post("/auth/logout", authHandlers.HandleLogout)

		// Protected routes (require JWT)
		// Example: Secrets CRUD with RBAC (uncomment when implementing Phase 5)
		// r.Group(func(r chi.Router) {
		// 	r.Use(mw.JWTMiddleware(cfg))
		//
		// 	// Helper to extract namespace ID from URL parameter
		// 	getNamespaceFromParam := func(r *http.Request) (uuid.UUID, error) {
		// 		nsID := chi.URLParam(r, "namespaceID")
		// 		return uuid.Parse(nsID)
		// 	}
		//
		// 	// Secrets - require write permission
		// 	r.Post("/namespaces/{namespaceID}/secrets", mw.RequireWrite(db, getNamespaceFromParam), handlers.CreateSecret)
		// 	r.Put("/secrets/{id}", mw.RequireWrite(db, getNamespaceFromParam), handlers.UpdateSecret)
		//
		// 	// Publish - require publish permission
		// 	r.Post("/secrets/{id}/publish", mw.RequirePublish(db, getNamespaceFromParam), handlers.PublishSecret)
		//
		// 	// Delete - require admin permission
		// 	r.Delete("/secrets/{id}", mw.RequireDelete(db, getNamespaceFromParam), handlers.DeleteSecret)
		//
		// 	// Namespace management - require admin permission
		// 	r.Put("/namespaces/{namespaceID}/permissions", mw.RequireAdmin(db, getNamespaceFromParam), handlers.UpdateNamespacePermissions)
		// })
	})

	return r
}
