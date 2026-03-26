package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/cors"
)

// NewCORSHandler creates CORS middleware configured for frontend
func NewCORSHandler() func(http.Handler) http.Handler {
	// Default allowed origins
	allowedOrigins := []string{"http://localhost:3000"}

	// Add frontend URL from environment (for Docker/E2E)
	if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
		allowedOrigins = append(allowedOrigins, frontendURL)
	}

	// Add CORS_ALLOWED_ORIGINS for custom setups (comma-separated)
	if customOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); customOrigins != "" {
		origins := strings.Split(customOrigins, ",")
		for _, origin := range origins {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
			}
		}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
