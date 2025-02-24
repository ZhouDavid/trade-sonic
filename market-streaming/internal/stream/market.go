package stream

// MarketStreamer defines the interface for market data streaming
type MarketStreamer interface {
	// Subscribe subscribes to the specified symbols
	Subscribe() error
	// Stream starts streaming market data
	Stream() error
	// AddHandler adds a new trade handler
	AddHandler(handler TradeHandler)
	// Close closes the connection
	Close() error
}

// TradeHandler is a function type that handles incoming trade data
type TradeHandler func(Trade)
