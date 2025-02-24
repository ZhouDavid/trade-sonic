package stream

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

// Streamer handles the websocket connection and data streaming
type Streamer struct {
	conn     *websocket.Conn
	apiKey   string
	symbols  []string
	handlers []TradeHandler
}

// NewStreamer creates a new market data streamer
func NewStreamer(apiKey string, symbols []string) (*Streamer, error) {
	log.Printf("Connecting to Finnhub websocket...")
	url := fmt.Sprintf("wss://ws.finnhub.io?token=%s", apiKey)
	c, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("error connecting to websocket: %w, response: %+v", err, resp)
	}
	log.Printf("Successfully connected to Finnhub websocket")

	return &Streamer{
		conn:     c,
		apiKey:   apiKey,
		symbols:  symbols,
		handlers: make([]TradeHandler, 0),
	}, nil
}

// AddHandler adds a new trade handler
func (s *Streamer) AddHandler(handler TradeHandler) {
	s.handlers = append(s.handlers, handler)
}

// Subscribe subscribes to the specified symbols
func (s *Streamer) Subscribe() error {
	log.Printf("Subscribing to symbols: %v", s.symbols)
	for _, symbol := range s.symbols {
		msg := fmt.Sprintf(`{"type":"subscribe","symbol":"%s"}`, symbol)
		if err := s.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return fmt.Errorf("error subscribing to symbol %s: %w", symbol, err)
		}
		log.Printf("Subscribed to %s", symbol)
	}
	return nil
}

// Stream starts streaming market data
func (s *Streamer) Stream() error {
	log.Printf("Starting to stream market data...")
	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("error reading message: %w", err)
		}
		log.Printf("Received message: %s", string(message))

		var tradeData TradeData
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
