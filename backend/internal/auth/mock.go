package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MockAuthProvider for DEVELOPMENT ONLY - bypasses real OAuth2 flow
// MockAuthProvider implements AuthProvider for local development
type MockAuthProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	authURL      string
	tokenURL     string
}

// NewMockAuthProvider creates a new mock OAuth2 provider
func NewMockAuthProvider(clientID, clientSecret, redirectURL string) *MockAuthProvider {
	return &MockAuthProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		authURL:      "http://localhost:9000/authorize",
		tokenURL:     "http://localhost:9000/token",
	}
}

// GetLoginURL returns the mock callback URL with email parameter
// For development, bypasses external OAuth2 server by directly calling the mock callback
func (m *MockAuthProvider) GetLoginURL(state string) string {
	// Return direct callback URL - frontend will prompt for email and call this endpoint
	return "http://localhost:8080/api/v1/auth/mock-callback?email={email}"
}

// HandleCallback processes the OAuth2 callback
func (m *MockAuthProvider) HandleCallback(code string) (*User, error) {
	// Exchange code for access token
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", m.redirectURL)
	data.Set("client_id", m.clientID)
	data.Set("client_secret", m.clientSecret)

	resp, err := http.Post(m.tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	// For mock provider, return hardcoded user based on token
	// In real implementation, would call userinfo endpoint
	return m.getUserFromToken(tokenResp.AccessToken)
}

// ValidateToken validates a token and returns user info
func (m *MockAuthProvider) ValidateToken(token string) (*User, error) {
	return m.getUserFromToken(token)
}

// getUserFromToken returns mock user data
// In dev mode, we simulate different users for testing
func (m *MockAuthProvider) getUserFromToken(token string) (*User, error) {
	// Mock: return different users based on token or default to dev user
	// This simulates the OAuth2 userinfo endpoint

	// Default dev user
	return &User{
		Email:  "dev@example.com",
		Name:   "Dev User",
		Groups: []string{"developers"},
	}, nil
}
