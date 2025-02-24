package stock

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
	"trade-sonic/market-streaming/internal/stream"

	"github.com/gorilla/websocket"
)

// Streamer handles stock market data streaming
type Streamer struct {
	conn     *websocket.Conn
	apiKey   string
	symbols  []string
	handlers []stream.TradeHandler
}

// NewStreamer creates a new stock market data streamer
func NewStreamer(apiKey string, symbols []string) (*Streamer, error) {
	log.Printf("Connecting to Finnhub stock websocket...")
	url := fmt.Sprintf("wss://ws.finnhub.io?token=%s", apiKey)
	c, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("error connecting to websocket: %w, response: %+v", err, resp)
	}
	log.Printf("Successfully connected to Finnhub stock websocket")

	return &Streamer{
		conn:     c,
		apiKey:   apiKey,
		symbols:  symbols,
		handlers: make([]stream.TradeHandler, 0),
	}, nil
}

// AddHandler adds a new trade handler
func (s *Streamer) AddHandler(handler stream.TradeHandler) {
	s.handlers = append(s.handlers, handler)
}

// IsTrading checks if the stock market is currently trading
func IsTrading() bool {
	now := time.Now()
	
	// Check if it's weekend
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	
	// Convert current time to Eastern Time
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("Error loading timezone: %v", err)
		return false
	}
	
	etNow := now.In(et)
	
	// Trading hours are 9:30 AM - 4:00 PM ET
	open := time.Date(etNow.Year(), etNow.Month(), etNow.Day(), 9, 30, 0, 0, et)
	close := time.Date(etNow.Year(), etNow.Month(), etNow.Day(), 16, 0, 0, 0, et)
	
	return etNow.After(open) && etNow.Before(close)
}

// Subscribe subscribes to the specified stock symbols
func (s *Streamer) Subscribe() error {
	if !IsTrading() {
		log.Printf("Warning: Stock market is currently closed. Regular trading hours are:")
		log.Printf("Monday-Friday, 9:30 AM - 4:00 PM Eastern Time")
		log.Printf("You may still connect to the stream but might not receive any data")
		log.Printf("")
	}

	log.Printf("Subscribing to stock symbols: %v", s.symbols)
	for _, symbol := range s.symbols {
		msg := fmt.Sprintf(`{"type":"subscribe","symbol":"%s"}`, symbol)
		if err := s.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return fmt.Errorf("error subscribing to symbol %s: %w", symbol, err)
		}
		log.Printf("Subscribed to stock %s", symbol)
	}
	return nil
}

// Stream starts streaming stock market data
func (s *Streamer) Stream() error {
	log.Printf("Starting to stream stock market data...")
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			log.Printf("Connection error: %v. Attempting to reconnect...", err)
			s.conn.Close()

			// Reconnection loop
			for {
				log.Printf("Waiting %v before reconnecting...", backoff)
				time.Sleep(backoff)

				// Exponential backoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}

				// Try to reconnect
				url := fmt.Sprintf("wss://ws.finnhub.io?token=%s", s.apiKey)
				newConn, _, err := websocket.DefaultDialer.Dial(url, nil)
				if err != nil {
					log.Printf("Reconnection failed: %v", err)
					continue
				}

				// Reconnected successfully
				s.conn = newConn
				log.Printf("Successfully reconnected to Finnhub stock websocket")

				// Resubscribe to symbols
				if err := s.Subscribe(); err != nil {
					log.Printf("Error resubscribing to symbols: %v", err)
					s.conn.Close()
					continue
				}

				// Reset backoff after successful reconnection
				backoff = time.Second
				break
			}
			continue
		}

		var tradeData stream.TradeData
		if err := json.Unmarshal(message, &tradeData); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		if tradeData.Type == "trade" {
			for _, trade := range tradeData.Data {
				for _, handler := range s.handlers {
					handler(trade)
				}
			}
		}
	}
}

// Close closes the websocket connection
func (s *Streamer) Close() error {
	return s.conn.Close()
}
