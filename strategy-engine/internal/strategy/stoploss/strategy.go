package stoploss

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ZhouDavid/trade-sonic/strategy-engine/internal/strategy"
)

// StopLossStrategy implements a simple stop loss strategy based on maximum drawdown
type StopLossStrategy struct {
	mu sync.RWMutex

	// Strategy parameters
	maxDrawdownPercent float64                   // Maximum allowed drawdown in percentage
	positions          map[string]Position       // Current positions keyed by symbol
	optionPositions    map[string]OptionPosition // Option positions keyed by option ID
	positionServiceURL string                    // URL of the position service
	positionFetchTimer *time.Ticker              // Timer for fetching positions
	client             *http.Client              // HTTP client for API calls

	name string
}

// Position tracks the position details for a symbol
type Position struct {
	EntryPrice     float64   // Price at which we entered the position
	HighestPrice   float64   // Highest price seen since entry
	Quantity       float64   // Current position quantity
	LastUpdateTime time.Time // Last time this position was updated
}

// OptionPosition tracks the details for an option position
type OptionPosition struct {
	ID             string    // Option ID
	Symbol         string    // Symbol (e.g., AAPL)
	EntryPrice     float64   // Price at which we entered the position
	HighestPrice   float64   // Highest price seen since entry
	CurrentPrice   float64   // Current price
	Quantity       float64   // Current position quantity
	Multiplier     float64   // Option multiplier (typically 100)
	CostBasis      float64   // Total cost basis
	LastUpdateTime time.Time // Last time this position was updated
}

// NewStopLossStrategy creates a new instance of StopLossStrategy
func NewStopLossStrategy(params map[string]interface{}) (*StopLossStrategy, error) {
	maxDrawdown, ok := params["max_drawdown_percent"].(float64)
	if !ok {
		return nil, fmt.Errorf("max_drawdown_percent must be a float64")
	}

	if maxDrawdown <= 0 || maxDrawdown >= 100 {
		return nil, fmt.Errorf("max_drawdown_percent must be between 0 and 100")
	}

	positionServiceURL, ok := params["position_service_url"].(string)
	if !ok || positionServiceURL == "" {
		positionServiceURL = "http://localhost:8081" // Default position service URL
	}

	return &StopLossStrategy{
		maxDrawdownPercent: maxDrawdown,
		positions:          make(map[string]Position),
		optionPositions:    make(map[string]OptionPosition),
		positionServiceURL: positionServiceURL,
		client:             &http.Client{Timeout: 10 * time.Second},
		name:               "option_stop_loss_strategy",
	}, nil
}

// Initialize implements strategy.Strategy
func (s *StopLossStrategy) Initialize(ctx context.Context) error {
	// Fetch initial positions
	if err := s.fetchOptionPositions(); err != nil {
		fmt.Printf("Error fetching initial option positions: %v\n", err)
		// Continue even if initial fetch fails
	}

	// Start a ticker to fetch positions every minute
	s.positionFetchTimer = time.NewTicker(1 * time.Minute)

	// Start a goroutine to periodically fetch positions
	go func() {
		for {
			select {
			case <-s.positionFetchTimer.C:
				if err := s.fetchOptionPositions(); err != nil {
					fmt.Printf("Error fetching option positions: %v\n", err)
				}
			case <-ctx.Done():
				// Context canceled, stop the ticker
				s.positionFetchTimer.Stop()
				return
			}
		}
	}()

	return nil
}

// ProcessData implements strategy.Strategy
func (s *StopLossStrategy) ProcessData(ctx context.Context, data strategy.MarketData) (*strategy.Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First, check if this is an option symbol we're tracking
	for optionID, optPos := range s.optionPositions {
		// If the option symbol matches the market data symbol
		if optPos.Symbol == data.Symbol {
			// Update the current price
			optPos.CurrentPrice = data.Price
			optPos.LastUpdateTime = data.Timestamp

			// Update highest price if needed
			if data.Price > optPos.HighestPrice {
				optPos.HighestPrice = data.Price
			}

			// Save the updated position
			s.optionPositions[optionID] = optPos

			// Check for stop loss
			if optPos.Quantity > 0 {
				currentDrawdown := (optPos.HighestPrice - data.Price) / optPos.HighestPrice * 100

				fmt.Printf("Option %s (%s): Current price: $%.2f, Highest: $%.2f, Drawdown: %.2f%%\n",
					optionID, optPos.Symbol, data.Price, optPos.HighestPrice, currentDrawdown)

				if currentDrawdown >= s.maxDrawdownPercent {
					// Generate sell signal - stop loss triggered
					signal := &strategy.Signal{
						Symbol:      optPos.Symbol,
						Action:      strategy.SignalActionSell,
						Price:       data.Price,
						Quantity:    optPos.Quantity,
						Confidence:  1.0, // High confidence for stop loss
						GeneratedAt: data.Timestamp,
						ExpiresAt:   data.Timestamp.Add(time.Minute), // Signal expires in 1 minute
						Metadata: map[string]interface{}{
							"reason":           "option_stop_loss",
							"option_id":        optionID,
							"entry_price":      optPos.EntryPrice,
							"highest_price":    optPos.HighestPrice,
							"current_drawdown": currentDrawdown,
							"cost_basis":       optPos.CostBasis,
						},
					}

					// Reset position tracking
					delete(s.optionPositions, optionID)
					return signal, nil
				}
			}
		}
	}

	// Also handle regular stock positions
	pos, exists := s.positions[data.Symbol]
	if !exists {
		// No position for this symbol yet, track it as a potential entry
		s.positions[data.Symbol] = Position{
			EntryPrice:     data.Price,
			HighestPrice:   data.Price,
			Quantity:       0, // No position yet
			LastUpdateTime: data.Timestamp,
		}
		return nil, nil
	}

	// Update position tracking
	if data.Price > pos.HighestPrice {
		pos.HighestPrice = data.Price
		s.positions[data.Symbol] = pos
	}

	// If we have an active position, check for stop loss
	if pos.Quantity > 0 {
		currentDrawdown := (pos.HighestPrice - data.Price) / pos.HighestPrice * 100

		if currentDrawdown >= s.maxDrawdownPercent {
			// Generate sell signal - stop loss triggered
			signal := &strategy.Signal{
				Symbol:      data.Symbol,
				Action:      strategy.SignalActionSell,
				Price:       data.Price,
				Quantity:    pos.Quantity,
				Confidence:  1.0, // High confidence for stop loss
				GeneratedAt: data.Timestamp,
				ExpiresAt:   data.Timestamp.Add(time.Minute), // Signal expires in 1 minute
				Metadata: map[string]interface{}{
					"reason":           "stock_stop_loss",
					"entry_price":      pos.EntryPrice,
					"highest_price":    pos.HighestPrice,
					"current_drawdown": currentDrawdown,
				},
			}

			// Reset position tracking
			delete(s.positions, data.Symbol)
			return signal, nil
		}
	}

	return nil, nil
}

// Name implements strategy.Strategy
func (s *StopLossStrategy) Name() string {
	return s.name
}

// Parameters implements strategy.Strategy
func (s *StopLossStrategy) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"max_drawdown_percent": s.maxDrawdownPercent,
		"position_service_url": s.positionServiceURL,
	}
}

// UpdateParameters implements strategy.Strategy
func (s *StopLossStrategy) UpdateParameters(params map[string]interface{}) error {
	maxDrawdown, ok := params["max_drawdown_percent"].(float64)
	if !ok {
		return fmt.Errorf("max_drawdown_percent must be a float64")
	}

	if maxDrawdown <= 0 || maxDrawdown >= 100 {
		return fmt.Errorf("max_drawdown_percent must be between 0 and 100")
	}

	s.mu.Lock()
	s.maxDrawdownPercent = maxDrawdown
	s.mu.Unlock()

	return nil
}

// Cleanup implements strategy.Strategy
func (s *StopLossStrategy) Cleanup(ctx context.Context) error {
	// Stop the position fetch timer if it exists
	if s.positionFetchTimer != nil {
		s.positionFetchTimer.Stop()
	}
	return nil
}

// fetchOptionPositions fetches option positions from the position service
func (s *StopLossStrategy) fetchOptionPositions() error {
	// Create the request body with account type
	reqBody := []byte(`{"account_type": "robinhood"}`)

	// Create a POST request to get positions
	req, err := http.NewRequest("POST", s.positionServiceURL+"/positions", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("error creating position request: %w", err)
	}

	// Execute the request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error fetching positions: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status code is OK
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error response from position service: %s, status: %d", string(body), resp.StatusCode)
	}

	// Parse the positions response
	var positionsResp struct {
		Positions []struct {
			ID            string  `json:"id"`
			Symbol        string  `json:"symbol"`
			Quantity      float64 `json:"quantity"`
			AveragePrice  float64 `json:"average_price"`
			CurrentPrice  float64 `json:"current_price"`
			MarketValue   float64 `json:"market_value"`
			CostBasis     float64 `json:"cost_basis"`
			InstrumentURL string  `json:"instrument_url"`
		} `json:"positions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&positionsResp); err != nil {
		return fmt.Errorf("error decoding positions response: %w", err)
	}

	// Lock for updating positions
	s.mu.Lock()
	defer s.mu.Unlock()

	// Process each option position
	for _, pos := range positionsResp.Positions {
		// Use the position ID as the option ID
		optionID := pos.ID

		// Skip positions with invalid IDs
		if optionID == "" {
			continue
		}

		// Check if we already have this option position
		existingPos, exists := s.optionPositions[optionID]
		if exists {
			// Update existing position
			existingPos.CurrentPrice = pos.CurrentPrice
			existingPos.Quantity = pos.Quantity
			existingPos.LastUpdateTime = time.Now()

			// Only update highest price if it's higher
			if pos.CurrentPrice > existingPos.HighestPrice {
				existingPos.HighestPrice = pos.CurrentPrice
			}

			// Save updated position
			s.optionPositions[optionID] = existingPos
		} else {
			// Create new option position
			s.optionPositions[optionID] = OptionPosition{
				ID:             optionID,
				Symbol:         pos.Symbol,
				EntryPrice:     pos.AveragePrice,
				HighestPrice:   pos.CurrentPrice,
				CurrentPrice:   pos.CurrentPrice,
				Quantity:       pos.Quantity,
				Multiplier:     100.0, // Default multiplier for options
				CostBasis:      pos.CostBasis,
				LastUpdateTime: time.Now(),
			}
			fmt.Printf("Added new option position: %s (%s), Price: $%.2f, Quantity: %.2f\n",
				optionID, pos.Symbol, pos.CurrentPrice, pos.Quantity)
		}
	}

	return nil
}
