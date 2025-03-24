package position

import (
	"time"
)

// AccountType represents the type of brokerage account
type AccountType string

const (
	// Robinhood account type
	Robinhood AccountType = "robinhood"
)

// Position represents a trading position
type Position struct {
	ID                   string    `json:"id"`
	AccountID            string    `json:"account_id"`
	Symbol               string    `json:"symbol"`
	Quantity             float64   `json:"quantity"`
	AveragePrice         float64   `json:"average_price"`
	CurrentPrice         float64   `json:"current_price"`
	MarketValue          float64   `json:"market_value"`
	CostBasis            float64   `json:"cost_basis"`
	UnrealizedPnL        float64   `json:"unrealized_pnl"`
	UnrealizedPnLPercent float64   `json:"unrealized_pnl_percent"`
	InstrumentURL        string    `json:"instrument_url"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// PositionList represents a list of positions
type PositionList struct {
	Positions   []Position  `json:"positions"`
	AccountID   string      `json:"account_id"`
	AccountType AccountType `json:"account_type"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
