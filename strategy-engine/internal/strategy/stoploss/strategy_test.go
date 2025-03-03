package stoploss

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewStopLossStrategy(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]interface{}
		expectedError bool
	}{
		{
			name: "valid parameters",
			params: map[string]interface{}{
				"max_drawdown_percent": 5.0,
			},
			expectedError: false,
		},
		{
			name: "invalid parameter type",
			params: map[string]interface{}{
				"max_drawdown_percent": "5.0", // string instead of float64
			},
			expectedError: true,
		},
		{
			name: "invalid drawdown value - negative",
			params: map[string]interface{}{
				"max_drawdown_percent": -1.0,
			},
			expectedError: true,
		},
		{
			name: "invalid drawdown value - too large",
			params: map[string]interface{}{
				"max_drawdown_percent": 100.1,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewStopLossStrategy(tt.params)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, strategy)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, strategy)
			}
		})
	}
}

func TestStopLossStrategy_ProcessData(t *testing.T) {
	// Create a strategy with 5% max drawdown
	s, err := NewStopLossStrategy(map[string]interface{}{
		"max_drawdown_percent": 5.0,
	})
	assert.NoError(t, err)

	ctx := context.Background()
	now := time.Now()

	// Helper function to create market data
	createMarketData := func(price float64, timestamp time.Time) struct {
		Symbol    string
		Price     float64
		Volume    float64
		Timestamp time.Time
	} {
		return struct {
			Symbol    string
			Price     float64
			Volume    float64
			Timestamp time.Time
		}{
			Symbol:    "BTC-USD",
			Price:     price,
			Volume:    1.0,
			Timestamp: timestamp,
		}
	}

	// Test scenario 1: Initial position setup
	data := createMarketData(50000.0, now)
	s.positions[data.Symbol] = Position{
		EntryPrice:     data.Price,
		HighestPrice:   data.Price,
		Quantity:       1.0,
		LastUpdateTime: data.Timestamp,
	}
	signal, err := s.ProcessData(ctx, data)
	assert.NoError(t, err)
	assert.Nil(t, signal)
	// Test scenario 2: Price increase (no signal)
	data = createMarketData(51000.0, now.Add(time.Minute))
	signal, err = s.ProcessData(ctx, data)
	assert.NoError(t, err)
	assert.Nil(t, signal)
	assert.Equal(t, 51000.0, s.positions[data.Symbol].HighestPrice)

	// Test scenario 3: Small drawdown (no signal)
	data = createMarketData(48500.0, now.Add(2*time.Minute)) // 4.9% drawdown
	signal, err = s.ProcessData(ctx, data)
	assert.NoError(t, err)
	assert.Nil(t, signal)

	// Test scenario 4: Large drawdown (sell signal)
	data = createMarketData(48000.0, now.Add(3*time.Minute)) // 5.88% drawdown
	signal, err = s.ProcessData(ctx, data)
	assert.NoError(t, err)
	assert.NotNil(t, signal)
	if signal != nil {
		assert.Equal(t, "SELL", string(signal.Action))
		assert.Equal(t, data.Price, signal.Price)
		assert.Equal(t, 1.0, signal.Quantity)
		assert.Equal(t, data.Symbol, signal.Symbol)
		assert.Equal(t, "stop_loss", signal.Metadata["reason"])
		assert.Equal(t, 50000.0, signal.Metadata["entry_price"])
		assert.Equal(t, 51000.0, signal.Metadata["highest_price"])
		drawdown, ok := signal.Metadata["current_drawdown"].(float64)
		assert.True(t, ok)
		assert.InDelta(t, 5.88, drawdown, 0.01)
	}
	// Test scenario 5: After stop loss (no position, no signal)
	data = createMarketData(47000.0, now.Add(4*time.Minute))
	signal, err = s.ProcessData(ctx, data)
	assert.NoError(t, err)
	assert.Nil(t, signal)
}

func TestStopLossStrategy_UpdateParameters(t *testing.T) {
	strategy, err := NewStopLossStrategy(map[string]interface{}{
		"max_drawdown_percent": 5.0,
	})
	assert.NoError(t, err)

	tests := []struct {
		name          string
		params        map[string]interface{}
		expectedError bool
	}{
		{
			name: "valid update",
			params: map[string]interface{}{
				"max_drawdown_percent": 10.0,
			},
			expectedError: false,
		},
		{
			name: "invalid type",
			params: map[string]interface{}{
				"max_drawdown_percent": "10.0",
			},
			expectedError: true,
		},
		{
			name: "invalid value",
			params: map[string]interface{}{
				"max_drawdown_percent": -1.0,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := strategy.UpdateParameters(tt.params)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				params := strategy.Parameters()
				assert.Equal(t, tt.params["max_drawdown_percent"], params["max_drawdown_percent"])
			}
		})
	}
}
