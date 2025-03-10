package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// Unit tests for the token service

func TestGetToken_CachedToken(t *testing.T) {
	s := &Service{
		client: &http.Client{},
		tokenCache: map[AccountType]*cachedToken{
			Robinhood: {
				AccessToken: "test-token",
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	}

	token, err := s.GetToken(Robinhood)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if token.AccessToken != "test-token" {
		t.Errorf("Expected token 'test-token', got %s", token.AccessToken)
	}
}

func TestGetToken_ExpiredToken_DirectToken(t *testing.T) {
	// Mock client that returns a successful token response with direct access token
	mockClient := newMockClient([]mockResponse{
		newMockResponse(http.StatusOK, map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}),
	})

	// Create a service with an expired token
	s := &Service{
		client: mockClient,
		tokenCache: map[AccountType]*cachedToken{
			Robinhood: {
				AccessToken: "expired-token",
				ExpiresAt:   time.Now().Add(-time.Hour),
			},
		},
		credentials: map[AccountType]accountCredentials{
			Robinhood: {
				username: "test",
				password: "test",
			},
		},
	}

	// Call GetToken - it should fetch a new token
	token, err := s.GetToken(Robinhood)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify we got the new token
	if token.AccessToken != "new-token" {
		t.Errorf("Expected new token 'new-token', got %s", token.AccessToken)
	}

	// Verify token was cached
	cachedToken := s.tokenCache[Robinhood]
	if cachedToken == nil {
		t.Fatal("Expected token to be cached")
	}
	if cachedToken.AccessToken != "new-token" {
		t.Errorf("Expected cached token 'new-token', got %s", cachedToken.AccessToken)
	}
	if cachedToken.ExpiresAt.IsZero() {
		t.Error("Expected non-zero expiration time")
	}
}

func TestGetToken_ExpiredToken_WithWorkflow(t *testing.T) {
	// Mock client that returns a successful token response with workflow
	mockClient := newMockClient([]mockResponse{
		// First call should return verification workflow
		newMockResponse(http.StatusOK, map[string]interface{}{
			"verification_workflow": map[string]interface{}{
				"id": "test-workflow",
			},
		}),
		// Machine verification response
		newMockResponse(http.StatusOK, map[string]interface{}{
			"id": "test-inquiry",
		}),
		// User view response
		newMockResponse(http.StatusOK, map[string]interface{}{
			"context": map[string]interface{}{
				"sheriff_challenge": map[string]interface{}{
					"id": "test-challenge",
				},
			},
		}),
		// Prompt status response
		newMockResponse(http.StatusOK, map[string]interface{}{
			"challenge_status": "validated",
		}),
		// Workflow status response
		newMockResponse(http.StatusOK, map[string]interface{}{
			"type_context": map[string]interface{}{
				"result": "workflow_status_approved",
			},
		}),
		// Final token response
		newMockResponse(http.StatusOK, map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}),
	})

	// Create a service with an expired token
	s := &Service{
		client: mockClient,
		tokenCache: map[AccountType]*cachedToken{
			Robinhood: {
				AccessToken: "expired-token",
				ExpiresAt:   time.Now().Add(-time.Hour),
			},
		},
		credentials: map[AccountType]accountCredentials{
			Robinhood: {
				username: "test",
				password: "test",
			},
		},
	}

	// Call GetToken - it should fetch a new token
	token, err := s.GetToken(Robinhood)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify we got the new token
	if token.AccessToken != "new-token" {
		t.Errorf("Expected new token 'new-token', got %s", token.AccessToken)
	}

	// Verify token was cached
	cachedToken := s.tokenCache[Robinhood]
	if cachedToken == nil {
		t.Fatal("Expected token to be cached")
	}
	if cachedToken.AccessToken != "new-token" {
		t.Errorf("Expected cached token 'new-token', got %s", cachedToken.AccessToken)
	}
	if cachedToken.ExpiresAt.IsZero() {
		t.Error("Expected non-zero expiration time")
	}
}

func TestGetToken_NoCredentials(t *testing.T) {
	s := &Service{
		client: &http.Client{},
	}

	_, err := s.GetToken(Robinhood)
	if err == nil {
		t.Error("Expected error for missing credentials")
	}
}

func TestGetToken_InvalidAccountType(t *testing.T) {
	s := &Service{
		client: &http.Client{},
	}

	_, err := s.GetToken("invalid")
	if err == nil {
		t.Error("Expected error for invalid account type")
	}
}

// mockHttpClient implements a mock HTTP client for testing
type mockHttpClient struct {
	responses []mockResponse
	current   int
	*http.Client
}

type mockResponse struct {
	response *http.Response
	err      error
}

func newMockClient(responses []mockResponse) *http.Client {
	return &http.Client{
		Transport: &mockTransport{responses: responses},
	}
}

type mockTransport struct {
	responses []mockResponse
	current   int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.current >= len(m.responses) {
		return nil, fmt.Errorf("no more responses")
	}
	resp := m.responses[m.current]
	m.current++
	return resp.response, resp.err
}

func newMockResponse(statusCode int, body map[string]interface{}) mockResponse {
	bodyBytes, _ := json.Marshal(body)
	return mockResponse{
		response: &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(bytes.NewBuffer(bodyBytes)),
		},
		err: nil,
	}
}

func TestFetchRobinhoodToken_DirectSuccess(t *testing.T) {
	// Mock client that returns a successful token response immediately
	mockClient := newMockClient([]mockResponse{
		newMockResponse(http.StatusOK, map[string]interface{}{
			"access_token": "test-token",
			"expires_in":   3600,
		}),
	})

	s := &Service{
		client: mockClient,
	}

	token, expiresAt, err := s.fetchRobinhoodToken(accountCredentials{
		username: "test",
		password: "test",
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got %s", token)
	}
	if expiresAt.IsZero() {
		t.Error("Expected non-zero expiration time")
	}
}

func TestFetchRobinhoodToken_WorkflowSuccess(t *testing.T) {
	// Mock client that simulates the full workflow
	mockClient := newMockClient([]mockResponse{
		// Initial token request returns workflow ID
		newMockResponse(http.StatusOK, map[string]interface{}{
			"verification_workflow": map[string]interface{}{
				"id": "workflow-123",
			},
		}),
		// Machine verification returns inquiry ID
		newMockResponse(http.StatusOK, map[string]interface{}{
			"id": "inquiry-123",
		}),
		// User view returns challenge ID
		newMockResponse(http.StatusOK, map[string]interface{}{
			"context": map[string]interface{}{
				"sheriff_challenge": map[string]interface{}{
					"id": "challenge-123",
				},
			},
		}),
		// Prompt status returns validated
		newMockResponse(http.StatusOK, map[string]interface{}{
			"challenge_status": "validated",
		}),
		// Workflow status check returns approved
		newMockResponse(http.StatusOK, map[string]interface{}{
			"type_context": map[string]interface{}{
				"result": "workflow_status_approved",
			},
		}),
		// Final token request returns token
		newMockResponse(http.StatusOK, map[string]interface{}{
			"access_token": "test-token",
			"expires_in":   3600,
		}),
	})

	s := &Service{
		client: mockClient,
	}

	token, expiresAt, err := s.fetchRobinhoodToken(accountCredentials{
		username: "test",
		password: "test",
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got %s", token)
	}
	if expiresAt.IsZero() {
		t.Error("Expected non-zero expiration time")
	}
}
