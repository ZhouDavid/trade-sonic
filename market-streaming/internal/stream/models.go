package stream

import "fmt"

// TradeData represents the structure of incoming trade data from the websocket
type TradeData struct {
	Data []Trade `json:"data"`
	Type string  `json:"type"`
}

// Trade represents a single trade transaction
type Trade struct {
	Price     float64 `json:"p"` // Price
	Symbol    string  `json:"s"` // Symbol
	Timestamp int64   `json:"t"` // Timestamp
	Volume    float64 `json:"v"` // Volume
}

// FormatSymbol formats a crypto pair into Finnhub format
func FormatSymbol(base, quote string) string {
	return fmt.Sprintf("BINANCE:%s%s", base, quote)
}
