package middleware

import (
	"context"
	"encoding/json"
	"fmt"
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
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondError(w, http.StatusUnauthorized, "Missing authorization header")
				return
			}

			// Expect "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				respondError(w, http.StatusUnauthorized, "Invalid authorization header format")
				return
			}

			tokenString := parts[1]

			// Parse and validate JWT
			token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
				// Validate signing method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil {
				respondError(w, http.StatusUnauthorized, fmt.Sprintf("Invalid token: %v", err))
				return
			}

			if !token.Valid {
				respondError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			// Extract claims
			claims, ok := token.Claims.(*Claims)
			if !ok {
				respondError(w, http.StatusUnauthorized, "Invalid token claims")
				return
			}

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
