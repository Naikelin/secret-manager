package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/yourorg/secret-manager/internal/config"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// UserContextKey is the context key for authenticated user
	UserContextKey ContextKey = "user"
)

// Claims represents the JWT claims
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Name   string    `json:"name"`
	Groups []string  `json:"groups"`
	jwt.RegisteredClaims
}

// UserContext is the authenticated user stored in request context
type UserContext struct {
	UserID uuid.UUID
	Email  string
	Name   string
	Groups []string
}

// JWTMiddleware validates JWT tokens and attaches user to request context
func JWTMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[JWT Middleware] Called for %s %s", r.Method, r.URL.Path)

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Printf("[JWT Middleware] Missing authorization header")
				respondError(w, http.StatusUnauthorized, "Missing authorization header")
				return
			}

			// Expect "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Printf("[JWT Middleware] Invalid authorization header format: %s", authHeader)
				respondError(w, http.StatusUnauthorized, "Invalid authorization header format")
				return
			}

			tokenString := parts[1]
			log.Printf("[JWT Middleware] Token extracted (length: %d, first 50 chars: %s...)", len(tokenString), tokenString[:min(50, len(tokenString))])
			log.Printf("[JWT Middleware] JWT Secret length: %d", len(cfg.JWTSecret))

			// Parse and validate JWT
			token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
				// Validate signing method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				log.Printf("[JWT Middleware] Signing method validated: %v", token.Method)
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil {
				log.Printf("[JWT Middleware] Token parsing error: %v", err)
				respondError(w, http.StatusUnauthorized, fmt.Sprintf("Invalid token: %v", err))
				return
			}

			if !token.Valid {
				log.Printf("[JWT Middleware] Token is invalid")
				respondError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			// Extract claims
			claims, ok := token.Claims.(*Claims)
			if !ok {
				log.Printf("[JWT Middleware] Failed to extract claims")
				respondError(w, http.StatusUnauthorized, "Invalid token claims")
				return
			}

			log.Printf("[JWT Middleware] Successfully validated token for user: %s (ID: %s)", claims.Email, claims.UserID)

			// Attach user to request context
			userCtx := &UserContext{
				UserID: claims.UserID,
				Email:  claims.Email,
				Name:   claims.Name,
				Groups: claims.Groups,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, userCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetUserFromContext extracts the user from request context
func GetUserFromContext(ctx context.Context) (*UserContext, error) {
	user, ok := ctx.Value(UserContextKey).(*UserContext)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}
	return user, nil
}

// GenerateJWT creates a new JWT token for a user
func GenerateJWT(userID uuid.UUID, email, name string, groups []string, secret string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Name:   name,
		Groups: groups,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "secret-manager",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
