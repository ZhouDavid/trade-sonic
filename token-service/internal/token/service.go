package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type AccountType string

const (
	Robinhood AccountType = "robinhood"
)

type cachedToken struct {
	AccessToken string
	ExpiresAt   time.Time
}

type Service struct {
	client      *http.Client
	tokenCache  map[AccountType]*cachedToken
	cacheMutex  sync.RWMutex
	credentials map[AccountType]accountCredentials
}

type accountCredentials struct {
	username string
	password string
}

type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type config struct {
	Robinhood struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"robinhood"`
}

func NewService() (*Service, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	s := &Service{
		client: &http.Client{
			Timeout: time.Second * 30,
		},
		tokenCache:  make(map[AccountType]*cachedToken),
		credentials: make(map[AccountType]accountCredentials),
	}

	// Load credentials from config
	s.credentials[Robinhood] = accountCredentials{
		username: cfg.Robinhood.Username,
		password: cfg.Robinhood.Password,
	}

	return s, nil
}

// GetToken returns a valid token for the specified account type
func (s *Service) GetToken(accountType AccountType) (*TokenResponse, error) {
	// Check if we have a valid cached token
	s.cacheMutex.RLock()
	if token, exists := s.tokenCache[accountType]; exists {
		if time.Now().Before(token.ExpiresAt) {
			s.cacheMutex.RUnlock()
			return &TokenResponse{
				AccessToken: token.AccessToken,
				ExpiresAt:   token.ExpiresAt,
			}, nil
		}
	}
	s.cacheMutex.RUnlock()

	// Get credentials
	s.cacheMutex.RLock()
	creds, exists := s.credentials[accountType]
	s.cacheMutex.RUnlock()
	if !exists {
		return nil, fmt.Errorf("no credentials found for account type: %s", accountType)
	}

	// Get new token
	token, expiresAt, err := s.fetchNewToken(accountType, creds)
	if err != nil {
		return nil, err
	}

	// Cache the token
	s.cacheMutex.Lock()
	s.tokenCache[accountType] = &cachedToken{
		AccessToken: token,
		ExpiresAt:   expiresAt,
	}
	s.cacheMutex.Unlock()

	return &TokenResponse{
		AccessToken: token,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *Service) fetchNewToken(accountType AccountType, creds accountCredentials) (string, time.Time, error) {
	switch accountType {
	case Robinhood:
		return s.fetchRobinhoodToken(creds)
	default:
		return "", time.Time{}, fmt.Errorf("unsupported account type: %s", accountType)
	}
}

func (s *Service) fetchRobinhoodToken(creds accountCredentials) (string, time.Time, error) {
	const tokenURL = "https://api.robinhood.com/oauth2/token/"

	payload := map[string]interface{}{
		"device_token":                     "trade-sonic-token-service",
		"create_read_only_secondary_token": true,
		"client_id":                        "c82SH0WZOsabOXGP2sxqcj34FxkvfnWRZBKlBjFS",
		"grant_type":                       "password",
		"scope":                            "internal",
		"username":                         creds.username,
		"password":                         creds.password,
	}

	headers := map[string]string{
		"Content-Type":            "application/json",
		"X-Robinhood-API-Version": "1.431.4",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return result.AccessToken, expiresAt, nil
}
