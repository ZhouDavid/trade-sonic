package position

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Service handles position-related operations
type Service struct {
	client        *http.Client
	tokenService  TokenService
	positionCache map[AccountType]*PositionList
	cacheMutex    sync.RWMutex
	accountID     string // Robinhood account ID
}

// TokenService defines the interface for getting authentication tokens
type TokenService interface {
	GetToken(accountType AccountType) (string, error)
}

// NewService creates a new position service
func NewService(tokenService TokenService, accountID string) *Service {
	return &Service{
		client: &http.Client{
			Timeout: time.Second * 30,
		},
		tokenService:  tokenService,
		positionCache: make(map[AccountType]*PositionList),
		accountID:     accountID,
	}
}

// GetPositions retrieves positions for the specified account type
func (s *Service) GetPositions(accountType AccountType) (*PositionList, error) {
	// Check cache first
	s.cacheMutex.RLock()
	if cachedPositions, exists := s.positionCache[accountType]; exists {
		// You might want to add cache expiration logic here
		s.cacheMutex.RUnlock()
		return cachedPositions, nil
	}
	s.cacheMutex.RUnlock()

	// Get token for authentication
	token, err := s.tokenService.GetToken(accountType)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Fetch positions based on account type
	var positions *PositionList
	switch accountType {
	case Robinhood:
		positions, err = s.fetchRobinhoodPositions(token)
	default:
		return nil, fmt.Errorf("unsupported account type: %s", accountType)
	}

	if err != nil {
		return nil, err
	}

	// Cache the positions
	s.cacheMutex.Lock()
	s.positionCache[accountType] = positions
	s.cacheMutex.Unlock()

	return positions, nil
}

// fetchRobinhoodPositions fetches positions from Robinhood API
func (s *Service) fetchRobinhoodPositions(token string) (*PositionList, error) {
	// Use the account ID from the service configuration
	if s.accountID == "" {
		return nil, fmt.Errorf("account ID not configured")
	}

	// Use the configured account ID
	accountID := s.accountID

	// Now fetch positions using the account URL with the account ID
	// Build the URL with query parameters using net/url
	baseURL := "https://api.robinhood.com/options/positions/"
	params := url.Values{}
	params.Add("account_number", accountID)
	params.Add("nonzero", "true")

	// Construct the final URL with parameters
	positionsURL := baseURL + "?" + params.Encode()
	reqPositions, err := http.NewRequest("GET", positionsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating positions request: %w", err)
	}

	// Add authorization header
	reqPositions.Header.Add("Authorization", "Bearer "+token)

	// Execute the positions request
	respPositions, err := s.client.Do(reqPositions)
	if err != nil {
		return nil, fmt.Errorf("error fetching positions: %w", err)
	}
	defer respPositions.Body.Close()

	// Check if the response status code is OK
	if respPositions.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respPositions.Body)
		return nil, fmt.Errorf("error response from Robinhood positions API: %s, status: %d", string(body), respPositions.StatusCode)
	}

	// Read the response body
	respBody, err := io.ReadAll(respPositions.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Create a new reader from the response body for JSON decoding
	reader := bytes.NewReader(respBody)

	// Parse the positions response
	var positionsResp struct {
		Next     interface{} `json:"next"`
		Previous interface{} `json:"previous"`
		Results  []struct {
			Account                   string `json:"account"`
			AccountNumber             string `json:"account_number"`
			AveragePrice              string `json:"average_price"`
			ChainID                   string `json:"chain_id"`
			ChainSymbol               string `json:"chain_symbol"`
			ID                        string `json:"id"`
			Option                    string `json:"option"`
			Type                      string `json:"type"`
			PendingBuyQuantity        string `json:"pending_buy_quantity"`
			PendingExpiredQuantity    string `json:"pending_expired_quantity"`
			PendingExpirationQuantity string `json:"pending_expiration_quantity"`
			PendingExerciseQuantity   string `json:"pending_exercise_quantity"`
			PendingAssignmentQuantity string `json:"pending_assignment_quantity"`
			PendingSellQuantity       string `json:"pending_sell_quantity"`
			Quantity                  string `json:"quantity"`
			IntradayQuantity          string `json:"intraday_quantity"`
			IntradayAverageOpenPrice  string `json:"intraday_average_open_price"`
			CreatedAt                 string `json:"created_at"`
			ExpirationDate            string `json:"expiration_date"`
			TradeValueMultiplier      string `json:"trade_value_multiplier"`
			UpdatedAt                 string `json:"updated_at"`
			URL                       string `json:"url"`
			OptionID                  string `json:"option_id"`
			ClearingRunningQuantity   string `json:"clearing_running_quantity"`
			ClearingCostBasis         string `json:"clearing_cost_basis"`
			ClearingDirection         string `json:"clearing_direction"`
		} `json:"results"`
	}

	if err := json.NewDecoder(reader).Decode(&positionsResp); err != nil {
		return nil, fmt.Errorf("error decoding positions response: %w\nRaw response: %s", err, string(respBody))
	}

	// Create a list to hold our processed positions
	positionList := &PositionList{
		Positions:   []Position{},
		AccountID:   accountID,
		AccountType: Robinhood,
		UpdatedAt:   time.Now(),
	}

	// We'll collect option IDs to batch fetch their prices
	var optionIDs []string

	// First pass: collect all option IDs
	for _, posItem := range positionsResp.Results {
		// Skip positions with zero quantity
		quantity, err := strconv.ParseFloat(posItem.Quantity, 64)
		if err != nil || quantity <= 0 {
			continue
		}

		optionIDs = append(optionIDs, posItem.OptionID)
	}

	// Fetch option prices in batch
	optionPrices, err := s.fetchOptionPrices(optionIDs, token)
	if err != nil {
		// Log the error but continue with zero prices
		fmt.Printf("Error fetching option prices: %v\n", err)
	}

	// Reset option IDs for the second pass
	optionIDs = []string{}

	// Second pass: process positions with prices
	for _, posItem := range positionsResp.Results {
		// Skip positions with zero quantity
		quantity, err := strconv.ParseFloat(posItem.Quantity, 64)
		if err != nil || quantity <= 0 {
			continue
		}

		// For options, we'll use the chain symbol as the symbol
		symbol := posItem.ChainSymbol

		// Parse the average price
		averagePrice, err := strconv.ParseFloat(posItem.AveragePrice, 64)
		if err != nil {
			averagePrice = 0.0
		}

		// Parse the cost basis
		costBasis, err := strconv.ParseFloat(posItem.ClearingCostBasis, 64)
		if err != nil {
			fmt.Printf("Error parsing cost basis for %s: %v\n", posItem.OptionID, err)
			costBasis = 0.0
		}

		// Parse timestamps
		createdAt, _ := time.Parse(time.RFC3339, posItem.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, posItem.UpdatedAt)

		// Get current price from our price map
		currentPrice := 0.0
		if price, ok := optionPrices[posItem.OptionID]; ok {
			currentPrice = price
		}

		// Debug output for option price
		fmt.Printf("Option ID: %s, Symbol: %s, Price: $%.2f\n", posItem.OptionID, symbol, currentPrice)

		// Parse the trade value multiplier (typically 100 for options)
		multiplier, err := strconv.ParseFloat(posItem.TradeValueMultiplier, 64)
		if err != nil {
			multiplier = 100.0 // Default to standard option multiplier
		}

		// Calculate market value using current price and quantity
		marketValue := quantity * currentPrice * multiplier

		// Debug output for market value calculation
		fmt.Printf("  Quantity: %.2f, Multiplier: %.2f, Market Value: $%.2f\n", quantity, multiplier, marketValue)

		// Calculate unrealized P&L
		unrealizedPnL := marketValue - costBasis
		unrealizedPnLPercent := 0.0
		if costBasis > 0 {
			unrealizedPnLPercent = (unrealizedPnL / costBasis) * 100
		}

		// Debug output for P&L
		fmt.Printf("  Unrealized P&L: $%.2f (%.2f%%)\n", unrealizedPnL, unrealizedPnLPercent)

		// Create position object
		position := Position{
			ID:                   posItem.ID,
			AccountID:            accountID,
			Symbol:               symbol,
			Quantity:             quantity,
			AveragePrice:         averagePrice,
			CurrentPrice:         currentPrice,
			MarketValue:          marketValue,
			CostBasis:            costBasis,
			UnrealizedPnL:        unrealizedPnL,
			UnrealizedPnLPercent: unrealizedPnLPercent,
			InstrumentURL:        posItem.Option, // Use the option URL instead of instrument
			CreatedAt:            createdAt,
			UpdatedAt:            updatedAt,
			// Add additional option-specific fields if needed
			// You might want to extend your Position struct to include these
			// ExpirationDate: posItem.ExpirationDate,
			// OptionType: posItem.Type,
		}

		// Add to our list
		positionList.Positions = append(positionList.Positions, position)
	}

	return positionList, nil
}

// fetchOptionPrices fetches current prices for a batch of option IDs
func (s *Service) fetchOptionPrices(optionIDs []string, token string) (map[string]float64, error) {
	// If no option IDs, return empty map
	if len(optionIDs) == 0 {
		return map[string]float64{}, nil
	}

	// Build the URL with query parameters
	baseURL := "https://api.robinhood.com/marketdata/options/"
	params := url.Values{}

	// Add all option IDs as a comma-separated list
	params.Add("ids", strings.Join(optionIDs, ","))

	// Construct the final URL with parameters
	optionsURL := baseURL + "?" + params.Encode()

	// Create a request to get option prices
	req, err := http.NewRequest("GET", optionsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating option prices request: %w", err)
	}

	// Add authorization header
	req.Header.Add("Authorization", "Bearer "+token)

	// Execute the request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching option prices: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status code is OK
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error response from Robinhood option prices API: %s, status: %d", string(body), resp.StatusCode)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading option prices response body: %w", err)
	}

	// Create a new reader from the response body for JSON decoding
	reader := bytes.NewReader(respBody)

	// Parse the option prices response
	var optionPricesResp struct {
		Results []struct {
			AdjustedMarkPrice string `json:"adjusted_mark_price"`
			InstrumentID      string `json:"instrument_id"`
			MarkPrice         string `json:"mark_price"`
			LastTradePrice    string `json:"last_trade_price"`
		} `json:"results"`
	}

	if err := json.NewDecoder(reader).Decode(&optionPricesResp); err != nil {
		return nil, fmt.Errorf("error decoding option prices response: %w", err)
	}

	// Create a map to hold our option prices
	prices := make(map[string]float64)

	// Process each option price
	for _, option := range optionPricesResp.Results {
		// Use mark_price as the current price
		price, err := strconv.ParseFloat(option.MarkPrice, 64)
		if err != nil {
			// Try adjusted_mark_price if mark_price fails
			price, err = strconv.ParseFloat(option.AdjustedMarkPrice, 64)
			if err != nil {
				// Try last_trade_price as a last resort
				price, err = strconv.ParseFloat(option.LastTradePrice, 64)
				if err != nil {
					// Skip this option if we can't parse any price
					continue
				}
			}
		}

		// Debug output for fetched prices
		fmt.Printf("Fetched price for option ID %s: $%.2f\n", option.InstrumentID, price)

		// Add to our map
		prices[option.InstrumentID] = price
	}

	return prices, nil
}

// getInstrumentDetails fetches details about an instrument from Robinhood API
func (s *Service) getInstrumentDetails(instrumentURL string, token string) (string, float64, error) {
	// Create a request to get instrument details
	req, err := http.NewRequest("GET", instrumentURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("error creating instrument request: %w", err)
	}

	// Add authorization header
	req.Header.Add("Authorization", "Bearer "+token)

	// Execute the request
	resp, err := s.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("error fetching instrument details: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status code is OK
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("error response from Robinhood instrument API: %s, status: %d", string(body), resp.StatusCode)
	}

	// Parse the instrument response
	var instrumentResp struct {
		Symbol    string `json:"symbol"`
		Name      string `json:"name"`
		QuoteURL  string `json:"quote"`
		Tradeable bool   `json:"tradeable"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&instrumentResp); err != nil {
		return "", 0, fmt.Errorf("error decoding instrument response: %w", err)
	}

	// Now get the current price using the quote URL
	currentPrice, err := s.getCurrentPrice(instrumentResp.QuoteURL, token)
	if err != nil {
		return instrumentResp.Symbol, 0, fmt.Errorf("error getting current price: %w", err)
	}

	return instrumentResp.Symbol, currentPrice, nil
}

// getCurrentPrice fetches the current price of an instrument from Robinhood API
func (s *Service) getCurrentPrice(quoteURL string, token string) (float64, error) {
	// Create a request to get quote details
	req, err := http.NewRequest("GET", quoteURL, nil)
	if err != nil {
		return 0, fmt.Errorf("error creating quote request: %w", err)
	}

	// Add authorization header
	req.Header.Add("Authorization", "Bearer "+token)

	// Execute the request
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error fetching quote details: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status code is OK
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("error response from Robinhood quote API: %s, status: %d", string(body), resp.StatusCode)
	}

	// Parse the quote response
	var quoteResp struct {
		LastTradePrice    string `json:"last_trade_price"`
		AskPrice          string `json:"ask_price"`
		BidPrice          string `json:"bid_price"`
		LastExtendedHours string `json:"last_extended_hours_trade_price"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&quoteResp); err != nil {
		return 0, fmt.Errorf("error decoding quote response: %w", err)
	}

	// Try to get the last trade price first
	price, err := strconv.ParseFloat(quoteResp.LastTradePrice, 64)
	if err == nil && price > 0 {
		return price, nil
	}

	// If last trade price is not available, try the ask price
	price, err = strconv.ParseFloat(quoteResp.AskPrice, 64)
	if err == nil && price > 0 {
		return price, nil
	}

	// If ask price is not available, try the bid price
	price, err = strconv.ParseFloat(quoteResp.BidPrice, 64)
	if err == nil && price > 0 {
		return price, nil
	}

	// If bid price is not available, try the extended hours price
	price, err = strconv.ParseFloat(quoteResp.LastExtendedHours, 64)
	if err == nil && price > 0 {
		return price, nil
	}

	// If no price is available, return an error
	return 0, fmt.Errorf("no valid price found for instrument")
}
