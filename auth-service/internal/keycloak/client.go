package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Client struct {
	baseURL      string
	realm        string
	clientID     string
	clientSecret string
	adminUser    string
	adminPass    string
	httpClient   *http.Client

	mu          sync.Mutex
	adminToken  string
	tokenExpiry time.Time

	verifier *oidc.IDTokenVerifier
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type UserRepresentation struct {
	ID              string                     `json:"id,omitempty"`
	Username        string                     `json:"username"`
	Email           string                     `json:"email"`
	FirstName       string                     `json:"firstName,omitempty"`
	LastName        string                     `json:"lastName,omitempty"`
	Enabled         bool                       `json:"enabled"`
	EmailVerified   bool                       `json:"emailVerified"`
	Credentials     []CredentialRepresentation `json:"credentials,omitempty"`
	RequiredActions []string                   `json:"requiredActions"`
}

type CredentialRepresentation struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary bool   `json:"temporary"`
}

func NewClient(baseURL, realm, clientID, clientSecret, adminUser, adminPass string) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
		adminUser:    adminUser,
		adminPass:    adminPass,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Login authenticates a user and returns a Keycloak token response.
func (c *Client) Login(ctx context.Context, username, password string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, c.realm)

	data := url.Values{
		"grant_type":    {"password"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"username":      {username},
		"password":      {password},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("keycloak login failed (status %d): %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

// RefreshToken exchanges a refresh token for a new access token.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, c.realm)

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("keycloak refresh failed (status %d): %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

// CreateUser creates a user in Keycloak via the Admin REST API.
func (c *Client) CreateUser(ctx context.Context, email, password, name string) (string, error) {
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}

	usersURL := fmt.Sprintf("%s/admin/realms/%s/users", c.baseURL, c.realm)

	firstName, lastName := splitName(name)

	userRep := UserRepresentation{
		Username:        email,
		Email:           email,
		FirstName:       firstName,
		LastName:        lastName,
		Enabled:         true,
		EmailVerified:   true,
		RequiredActions: []string{},
		Credentials: []CredentialRepresentation{
			{
				Type:      "password",
				Value:     password,
				Temporary: false,
			},
		},
	}

	body, err := json.Marshal(userRep)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, usersURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return "", fmt.Errorf("user already exists")
	}

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create user (status %d): %s", resp.StatusCode, respBody)
	}

	// Extract user ID from Location header
	location := resp.Header.Get("Location")
	parts := strings.Split(location, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1], nil
	}

	return "", fmt.Errorf("could not extract user ID from location header")
}

// InitVerifier initializes the OIDC token verifier using the Keycloak JWKS endpoint.
func (c *Client) InitVerifier(ctx context.Context) error {
	issuerURL := fmt.Sprintf("%s/realms/%s", c.baseURL, c.realm)
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return fmt.Errorf("failed to create OIDC provider: %w", err)
	}
	c.verifier = provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})
	return nil
}

// ValidateToken verifies a JWT token using Keycloak OIDC and returns the claims.
func (c *Client) ValidateToken(ctx context.Context, token string) (string, string, error) {
	if c.verifier == nil {
		return "", "", fmt.Errorf("OIDC verifier not initialized")
	}

	idToken, err := c.verifier.Verify(ctx, token)
	if err != nil {
		return "", "", fmt.Errorf("token verification failed: %w", err)
	}

	var claims struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		PreferredUsername  string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", "", fmt.Errorf("failed to parse claims: %w", err)
	}

	return claims.Sub, claims.Email, nil
}

// ResetPassword changes a user's password in Keycloak via the Admin REST API.
func (c *Client) ResetPassword(ctx context.Context, email, newPassword string) error {
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	// Find user by email
	usersURL := fmt.Sprintf("%s/admin/realms/%s/users?email=%s&exact=true", c.baseURL, c.realm, url.QueryEscape(email))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usersURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to find user (status %d): %s", resp.StatusCode, body)
	}

	var users []UserRepresentation
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return err
	}
	if len(users) == 0 {
		return fmt.Errorf("user not found")
	}

	// Reset password
	resetURL := fmt.Sprintf("%s/admin/realms/%s/users/%s/reset-password", c.baseURL, c.realm, users[0].ID)
	cred := CredentialRepresentation{
		Type:      "password",
		Value:     newPassword,
		Temporary: false,
	}
	body, err := json.Marshal(cred)
	if err != nil {
		return err
	}

	resetReq, err := http.NewRequestWithContext(ctx, http.MethodPut, resetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	resetReq.Header.Set("Content-Type", "application/json")
	resetReq.Header.Set("Authorization", "Bearer "+adminToken)

	resetResp, err := c.httpClient.Do(resetReq)
	if err != nil {
		return err
	}
	defer resetResp.Body.Close()

	if resetResp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resetResp.Body)
		return fmt.Errorf("failed to reset password (status %d): %s", resetResp.StatusCode, respBody)
	}

	return nil
}

func splitName(name string) (string, string) {
	parts := strings.SplitN(name, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return name, name
}

func (c *Client) getAdminToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.adminToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.adminToken, nil
	}

	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", c.baseURL)

	data := url.Values{
		"grant_type": {"password"},
		"client_id":  {"admin-cli"},
		"username":   {c.adminUser},
		"password":   {c.adminPass},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("admin login failed (status %d): %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	c.adminToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-30) * time.Second)

	return c.adminToken, nil
}
