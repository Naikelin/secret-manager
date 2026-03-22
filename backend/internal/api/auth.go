package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/yourorg/secret-manager/internal/auth"
	"github.com/yourorg/secret-manager/internal/config"
	"github.com/yourorg/secret-manager/internal/middleware"
	"github.com/yourorg/secret-manager/internal/models"
	"github.com/yourorg/secret-manager/pkg/logger"
	"gorm.io/gorm"
)

// AuthHandlers contains HTTP handlers for authentication
type AuthHandlers struct {
	db       *gorm.DB
	provider auth.AuthProvider
	cfg      *config.Config
}

// NewAuthHandlers creates auth handlers based on configuration
func NewAuthHandlers(db *gorm.DB, cfg *config.Config) *AuthHandlers {
	var provider auth.AuthProvider

	// Select authentication provider based on config
	switch cfg.AuthProvider {
	case "azure":
		// TODO: Get Azure AD config from environment
		provider = auth.NewAzureADProvider("", "", "", "http://localhost:3000/api/auth/callback")
	case "mock":
		fallthrough
	default:
		provider = auth.NewMockAuthProvider("dev-client", "dev-secret", "http://localhost:3000/api/auth/callback")
	}

	return &AuthHandlers{
		db:       db,
		provider: provider,
		cfg:      cfg,
	}
}

// HandleLogin redirects to OAuth2 provider
func (h *AuthHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Parse request body to extract email
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("Failed to parse login request body", "error", err)
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	// Validate email is provided
	if req.Email == "" {
		logger.Warn("Login request missing email")
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Email is required",
		})
		return
	}

	// Generate random state for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		logger.Error("Failed to generate state", "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to initiate login",
		})
		return
	}

	// Store state in session/cookie for validation (simplified for MVP)
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes
	})

	// Get OAuth2 authorization URL
	loginURL := h.provider.GetLoginURL(state)

	// For mock provider, replace {email} placeholder with actual email
	loginURL = strings.ReplaceAll(loginURL, "{email}", url.QueryEscape(req.Email))

	// Return JSON with redirect URL (frontend will handle redirect)
	respondJSON(w, http.StatusOK, map[string]string{
		"redirect_url": loginURL,
	})
}

// HandleCallback processes OAuth2 callback
func (h *AuthHandlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state parameter
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != state {
		logger.Warn("Invalid OAuth state", "state", state)
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid state parameter",
		})
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Missing authorization code",
		})
		return
	}

	// Exchange code for user info
	oauthUser, err := h.provider.HandleCallback(code)
	if err != nil {
		logger.Error("OAuth callback failed", "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Authentication failed",
		})
		return
	}

	// Sync user to database (cache OAuth user)
	user, err := h.syncUser(oauthUser)
	if err != nil {
		logger.Error("Failed to sync user", "error", err, "email", oauthUser.Email)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to create user session",
		})
		return
	}

	// Generate JWT token
	token, err := middleware.GenerateJWT(user.ID, user.Email, user.Name, oauthUser.Groups, h.cfg.JWTSecret)
	if err != nil {
		logger.Error("Failed to generate JWT", "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to create session",
		})
		return
	}

	logger.Info("User authenticated", "email", user.Email, "user_id", user.ID)

	// Return JWT token
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":     user.ID,
			"email":  user.Email,
			"name":   user.Name,
			"groups": oauthUser.Groups,
		},
	})
}

// HandleLogout clears the session
func (h *AuthHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// In JWT-based auth, logout is client-side (delete token)
	// Server-side logout would require token blacklisting (future enhancement)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

// HandleMockCallback processes the mock auth callback (DEVELOPMENT ONLY)
// Bypasses external OAuth2 server by directly looking up user and issuing JWT
func (h *AuthHandlers) HandleMockCallback(w http.ResponseWriter, r *http.Request) {
	// Extract email from query parameter
	email := r.URL.Query().Get("email")
	if email == "" {
		logger.Warn("Mock callback called without email parameter")
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Missing email parameter",
		})
		return
	}

	// Look up user with groups preloaded
	var user models.User
	result := h.db.Preload("Groups").Where("email = ?", email).First(&user)
	if result.Error != nil {
		logger.Warn("User not found in mock callback", "email", email, "error", result.Error)
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "User not found",
		})
		return
	}

	// Extract group names from user.Groups
	groups := make([]string, len(user.Groups))
	for i, group := range user.Groups {
		groups[i] = group.Name
	}

	// Generate JWT token
	token, err := middleware.GenerateJWT(user.ID, user.Email, user.Name, groups, h.cfg.JWTSecret)
	if err != nil {
		logger.Error("Failed to generate JWT in mock callback", "error", err, "email", email)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to create session",
		})
		return
	}

	logger.Info("Mock authentication successful", "email", user.Email, "user_id", user.ID, "groups", groups)

	// Redirect to frontend with token in URL
	redirectURL := fmt.Sprintf("http://localhost:3000/auth/callback?token=%s", token)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// syncUser creates or updates user in database
func (h *AuthHandlers) syncUser(oauthUser *auth.User) (*models.User, error) {
	var user models.User

	// Try to find existing user by email
	result := h.db.Where("email = ?", oauthUser.Email).First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new user
		user = models.User{
			Email: oauthUser.Email,
			Name:  oauthUser.Name,
		}

		if err := h.db.Create(&user).Error; err != nil {
			return nil, err
		}

		logger.Info("Created new user", "email", user.Email, "id", user.ID)
	} else if result.Error != nil {
		return nil, result.Error
	} else {
		// Update existing user
		user.Name = oauthUser.Name
		if err := h.db.Save(&user).Error; err != nil {
			return nil, err
		}
	}

	return &user, nil
}

// generateRandomState generates a random state parameter for OAuth2
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func respondJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
