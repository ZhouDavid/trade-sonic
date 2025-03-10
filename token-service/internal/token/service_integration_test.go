package token

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestFetchRobinhoodToken_Integration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Read credentials from config.json
	data, err := os.ReadFile("/Users/jianyu.zhou/personal-projects/trade-sonic/token-service/config.json")
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	s := &Service{
		client: &http.Client{
			Timeout: time.Second * 30,
		},
		credentials: map[AccountType]accountCredentials{
			Robinhood: {
				username: cfg.Robinhood.Username,
				password: cfg.Robinhood.Password,
			},
		},
		tokenCache: make(map[AccountType]*cachedToken),
	}

	token, err := s.GetToken(Robinhood)
	if err != nil {
		t.Fatalf("Failed to fetch token: %v", err)
	}

	if token.AccessToken == "" {
		t.Error("Expected non-empty access token")
	}
}
