package crypto

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
	"trade-sonic/market-streaming/internal/stream"

	"github.com/gorilla/websocket"
)

// Streamer handles cryptocurrency data streaming
type Streamer struct {
	conn      *websocket.Conn
	apiKey    string
	symbols   []string
	handlers  []stream.TradeHandler
	connected bool
}

// NewStreamer creates a new crypto market data streamer
func NewStreamer(apiKey string, symbols []string) (*Streamer, error) {
	s := &Streamer{
		apiKey:    apiKey,
		symbols:   symbols,
		handlers:  make([]stream.TradeHandler, 0),
		connected: false,
	}

	if err := s.connect(); err != nil {
		return nil, err
	}

	return s, nil
}

// AddHandler adds a new trade handler
func (s *Streamer) AddHandler(handler stream.TradeHandler) {
	s.handlers = append(s.handlers, handler)
}

// Subscribe subscribes to the specified crypto symbols
func (s *Streamer) Subscribe() error {
	log.Printf("Subscribing to crypto symbols: %v", s.symbols)
	for _, symbol := range s.symbols {
		msg := fmt.Sprintf(`{"type":"subscribe","symbol":"%s"}`, symbol)
		if err := s.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return fmt.Errorf("error subscribing to symbol %s: %w", symbol, err)
		}
		log.Printf("Subscribed to crypto %s", symbol)
	}
	return nil
}

// connect establishes a new websocket connection
func (s *Streamer) connect() error {
	log.Printf("Connecting to Finnhub crypto websocket...")
	url := fmt.Sprintf("wss://ws.finnhub.io?token=%s", s.apiKey)
	c, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("error connecting to websocket: %w, response: %+v", err, resp)
	}
	s.conn = c
	s.connected = true
	log.Printf("Successfully connected to Finnhub crypto websocket")
	return nil
}

// Stream starts streaming crypto market data
func (s *Streamer) Stream() error {
	log.Printf("Starting to stream crypto market data...")

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			log.Printf("Connection error: %v. Attempting to reconnect...", err)
			s.conn.Close()
			s.connected = false

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
				if err := s.connect(); err != nil {
					log.Printf("Reconnection failed: %v", err)
					continue
				}

				// Resubscribe to symbols
				if err := s.Subscribe(); err != nil {
					log.Printf("Error resubscribing to symbols: %v", err)
					s.conn.Close()
					s.connected = false
					continue
				}

				// Reset backoff after successful reconnection
				backoff = time.Second
				break
			}
			continue
		}

		// Parse and handle the message
		var tradeData stream.TradeData
		err = json.Unmarshal(message, &tradeData)
		if err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		// Process trades if we have any
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

// FormatSymbol formats a crypto pair into Finnhub format
func FormatSymbol(base, quote string) string {
	return fmt.Sprintf("BINANCE:%s%s", base, quote)
}
