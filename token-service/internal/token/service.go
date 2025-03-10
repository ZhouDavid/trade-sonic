package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
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
	deviceUUID := uuid.New().String()

	// Common headers used across requests
	headers := map[string]string{
		"sec-ch-ua-platform":      "macOS",
		"Referer":                 "https://robinhood.com/",
		"X-TimeZone-Id":           "America/Los_Angeles",
		"X-Robinhood-API-Version": "1.431.4",
		"sec-ch-ua":               "\"Not_A:Brand\";v=\"99\", \"Google Chrome\";v=\"133\", \"Chromium\";v=\"133\"",
		"Content-Type":            "application/json",
		"sec-ch-ua-mobile":        "?0",
		"User-Agent":              "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	}

	// Step 1: Initial token request
	tokenHeaders := map[string]string{
		"Content-Type": "application/json",
	}
	tokenData, err := s.getToken(creds, deviceUUID, tokenHeaders)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("initial token request failed: %w", err)
	}

	// First check for direct access token
	if accessToken, ok := tokenData["access_token"].(string); ok {
		expiresIn, _ := tokenData["expires_in"].(float64)
		expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
		return accessToken, expiresAt, nil
	}

	// If no access token, look for workflow ID
	workflowRaw, exists := tokenData["verification_workflow"]
	if !exists {
		return "", time.Time{}, fmt.Errorf("response missing both access_token and verification_workflow: %v", tokenData)
	}

	workflow, ok := workflowRaw.(map[string]interface{})
	if !ok {
		return "", time.Time{}, fmt.Errorf("verification_workflow is not a map: %v", tokenData)
	}

	workflowID, ok := workflow["id"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("workflow missing id field: %v", workflow)
	}

	// Step 2: Machine verification
	machineURL := "https://api.robinhood.com/pathfinder/user_machine/"
	machinePayload := map[string]interface{}{
		"device_id": deviceUUID,
		"flow":      "suv",
		"input":     map[string]string{"workflow_id": workflowID},
	}

	machineResp, err := s.makeRequest(http.MethodPost, machineURL, headers, machinePayload)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("machine verification failed: %w", err)
	}

	inquiryID, ok := machineResp.Body["id"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("no inquiry ID in response")
	}

	// Step 3: Get user view
	viewURL := fmt.Sprintf("https://api.robinhood.com/pathfinder/inquiries/%s/user_view/", inquiryID)
	viewResp, err := s.makeRequest(http.MethodGet, viewURL, headers, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("user view request failed: %w", err)
	}

	challengeID, ok := viewResp.Body["context"].(map[string]interface{})["sheriff_challenge"].(map[string]interface{})["id"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("no challenge ID in response")
	}

	// Step 4: Poll for prompt status
	promptURL := fmt.Sprintf("https://api.robinhood.com/push/%s/get_prompts_status/", challengeID)
	for attempt := 0; attempt < 30; attempt++ {
		promptResp, err := s.makeRequest(http.MethodGet, promptURL, headers, nil)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("prompt status check failed: %w", err)
		}

		// Handle non-200 responses
		if promptResp.StatusCode != http.StatusOK {
			return "", time.Time{}, fmt.Errorf("prompt status check failed with status %d: %v", promptResp.StatusCode, promptResp.Body)
		}

		status, _ := promptResp.Body["challenge_status"].(string)
		if status == "validated" {
			break
		} else if status != "issued" {
			return "", time.Time{}, fmt.Errorf("unexpected challenge status: %s", status)
		}

		time.Sleep(2 * time.Second)
	}

	// Step 5: Check workflow status
	viewPayload := map[string]interface{}{
		"sequence":   0,
		"user_input": map[string]string{"status": "continue"},
	}

	viewResp, err = s.makeRequest(http.MethodPost, viewURL, headers, viewPayload)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("workflow status check failed: %w", err)
	}

	workflowStatus, ok := viewResp.Body["type_context"].(map[string]interface{})["result"].(string)
	if !ok || workflowStatus != "workflow_status_approved" {
		return "", time.Time{}, fmt.Errorf("unexpected workflow status: %v", workflowStatus)
	}

	// Step 6: Final token request
	finalTokenData, err := s.getToken(creds, deviceUUID, tokenHeaders)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("final token request failed: %w", err)
	}

	// After workflow validation, we must get an access token
	accessToken, ok := finalTokenData["access_token"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("no access token in final response: %v", finalTokenData)
	}

	expiresIn, _ := finalTokenData["expires_in"].(float64)
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	return accessToken, expiresAt, nil
}

func (s *Service) getToken(creds accountCredentials, deviceUUID string, headers map[string]string) (map[string]interface{}, error) {
	tokenURL := "https://api.robinhood.com/oauth2/token/"
	payload := map[string]interface{}{
		"device_token":                     deviceUUID,
		"create_read_only_secondary_token": true,
		"client_id":                        "c82SH0WZOsabOXGP2sxqcj34FxkvfnWRZBKlBjFS",
		"grant_type":                       "password",
		"scope":                            "internal",
		"username":                         creds.username,
		"password":                         creds.password,
	}

	resp, err := s.makeRequest(http.MethodPost, tokenURL, headers, payload)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

type Response struct {
	StatusCode int
	Body       map[string]interface{}
}

func (s *Service) makeRequest(method, url string, headers map[string]string, payload interface{}) (*Response, error) {
	var body io.Reader
	if payload != nil {
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewBuffer(jsonPayload)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       result,
	}, nil
}
