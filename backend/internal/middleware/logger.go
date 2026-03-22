package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/yourorg/secret-manager/pkg/logger"
)

// RequestLogger is a middleware that logs HTTP requests with structured logging
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Call next handler
		next.ServeHTTP(ww, r)

		// Log request details
		duration := time.Since(start)
		logger.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}
