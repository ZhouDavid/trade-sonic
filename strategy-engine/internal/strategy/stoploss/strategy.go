package stoploss

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ZhouDavid/trade-sonic/strategy-engine/internal/strategy"
)

// StopLossStrategy implements a simple stop loss strategy based on maximum drawdown
type StopLossStrategy struct {
	mu sync.RWMutex

	// Strategy parameters
	maxDrawdownPercent float64             // Maximum allowed drawdown in percentage
	positions          map[string]Position // Current positions keyed by symbol

	name string
}

// Position tracks the position details for a symbol
type Position struct {
	EntryPrice     float64   // Price at which we entered the position
	HighestPrice   float64   // Highest price seen since entry
	Quantity       float64   // Current position quantity
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

	return &StopLossStrategy{
		maxDrawdownPercent: maxDrawdown,
		positions:          make(map[string]Position),
		name:               "stop_loss_strategy",
	}, nil
}

// Initialize implements strategy.Strategy
func (s *StopLossStrategy) Initialize(ctx context.Context) error {
	return nil
}

// ProcessData implements strategy.Strategy
func (s *StopLossStrategy) ProcessData(ctx context.Context, data strategy.MarketData) (*strategy.Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
					"reason":           "stop_loss",
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
	return nil
}
