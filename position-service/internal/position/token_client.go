package position

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// TokenClient is a client for the token service
type TokenClient struct {
	client    *http.Client
	serviceURL string
}

// TokenResponse represents a response from the token service
type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

// NewTokenClient creates a new token client
func NewTokenClient(serviceURL string) *TokenClient {
	return &TokenClient{
		client:    &http.Client{},
		serviceURL: serviceURL,
	}
}

// GetToken retrieves a token from the token service
func (c *TokenClient) GetToken(accountType AccountType) (string, error) {
	// Create request body
	reqBody, err := json.Marshal(map[string]string{
		"account_type": string(accountType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", c.serviceURL+"/token", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token service returned error: %s", body)
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return tokenResp.AccessToken, nil
}
