package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/config"
	"github.com/yourorg/secret-manager/internal/git"
	mw "github.com/yourorg/secret-manager/internal/middleware"
	"github.com/yourorg/secret-manager/internal/sops"
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

	// Initialize Git and SOPS clients for publish handlers
	gitClient, err := initGitClient(cfg)
	if err != nil {
		// Log error but don't fail - publish operations will fail gracefully
		// logger.Error("Failed to initialize Git client", "error", err)
	}

	sopsClient, err := initSOPSClient(cfg)
	if err != nil {
		// Log error but don't fail
		// logger.Error("Failed to initialize SOPS client", "error", err)
	}

	publishHandlers := NewPublishHandlers(db, gitClient, sopsClient)

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

					// Publish secret - require publish permission (editor or admin)
					r.With(mw.RequirePublish(db, getNamespaceFromParam)).Post("/publish", publishHandlers.PublishSecret)

					// Unpublish secret - require delete permission (admin only)
					r.With(mw.RequireDelete(db, getNamespaceFromParam)).Post("/unpublish", publishHandlers.UnpublishSecret)
				})
			})
		})
	})

	return r
}

// initGitClient initializes the Git client from config
func initGitClient(cfg *config.Config) (GitClientInterface, error) {
	if cfg.GitRepoURL == "" {
		return nil, nil // Git not configured
	}

	// Create auth method based on config
	var auth transport.AuthMethod
	var err error

	if cfg.GitAuthType == "ssh" {
		auth, err = git.NewSSHAuth(cfg.GitSSHKeyPath)
	} else if cfg.GitAuthType == "token" {
		auth = git.NewTokenAuth(cfg.GitToken, "git")
	} else {
		return nil, fmt.Errorf("invalid git auth type: %s", cfg.GitAuthType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Git auth: %w", err)
	}

	gitClient, err := git.NewGitClient(cfg.GitRepoPath, cfg.GitRepoURL, cfg.GitBranch, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create Git client: %w", err)
	}

	// Set author information
	gitClient.SetAuthor(cfg.GitAuthorName, cfg.GitAuthorEmail)

	return gitClient, nil
}

// initSOPSClient initializes the SOPS client from config
func initSOPSClient(cfg *config.Config) (SOPSClientInterface, error) {
	if !cfg.SOPSEnabled {
		return nil, nil // SOPS not enabled
	}

	sopsClient, err := sops.NewSOPSClient(cfg.SOPSEncryptType, cfg.SOPSAgeKeyPath, cfg.SOPSKMSKeyARN, cfg.SOPSConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOPS client: %w", err)
	}

	return sopsClient, nil
}
