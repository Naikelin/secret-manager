package auth

// User represents an authenticated user from OAuth2 provider
type User struct {
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

// AuthProvider defines the interface for OAuth2 authentication providers
type AuthProvider interface {
	// GetLoginURL returns the OAuth2 authorization URL with state parameter
	GetLoginURL(state string) string

	// HandleCallback processes the OAuth2 callback and returns user information
	HandleCallback(code string) (*User, error)

	// ValidateToken validates an access token and returns user information
	ValidateToken(token string) (*User, error)
}
