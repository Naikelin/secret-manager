package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// NewCORSHandler creates CORS middleware configured for frontend
func NewCORSHandler() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
