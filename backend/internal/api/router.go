package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
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

	// Initialize handlers
	authHandlers := NewAuthHandlers(db, cfg)
	secretHandlers := NewSecretHandlers(db)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public)
		r.Post("/auth/login", authHandlers.HandleLogin)
		r.Get("/auth/callback", authHandlers.HandleCallback)
		r.Get("/auth/mock-callback", authHandlers.HandleMockCallback) // Mock callback (dev only) - bypasses external OAuth2 server
		r.Post("/auth/logout", authHandlers.HandleLogout)

		// Protected routes (require JWT)
		r.Group(func(r chi.Router) {
			r.Use(mw.JWTMiddleware(cfg))

			// Helper to extract namespace ID from URL parameter
			getNamespaceFromParam := func(r *http.Request) (uuid.UUID, error) {
				nsID := chi.URLParam(r, "namespace")
				return uuid.Parse(nsID)
			}

			// Secrets CRUD with RBAC
			r.Route("/namespaces/{namespace}/secrets", func(r chi.Router) {
				// List secrets - require read permission
				r.With(mw.RequireRead(db, getNamespaceFromParam)).Get("/", secretHandlers.ListSecrets)

				// Create secret - require write permission
				r.With(mw.RequireWrite(db, getNamespaceFromParam)).Post("/", secretHandlers.CreateSecret)

				// Get, update, delete specific secret
				r.Route("/{name}", func(r chi.Router) {
					// Get secret - require read permission
					r.With(mw.RequireRead(db, getNamespaceFromParam)).Get("/", secretHandlers.GetSecret)

					// Update secret - require write permission
					r.With(mw.RequireWrite(db, getNamespaceFromParam)).Put("/", secretHandlers.UpdateSecret)

					// Delete secret - require delete permission (admin only)
					r.With(mw.RequireDelete(db, getNamespaceFromParam)).Delete("/", secretHandlers.DeleteSecret)
				})
			})
		})
	})

	return r
}
