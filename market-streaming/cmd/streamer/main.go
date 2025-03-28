package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"
	"trade-sonic/market-streaming/internal/stream"
	"trade-sonic/market-streaming/internal/stream/crypto"
	"trade-sonic/market-streaming/internal/stream/stock"
)

// createTradeHandler returns a handler function for processing trades
func createTradeHandler(marketType string) stream.TradeHandler {
	return func(trade stream.Trade) {
		// Convert timestamp to local time
		tradeTime := time.Unix(trade.Timestamp/1000, 0).Local()

		// Clean up symbol name
		symbol := trade.Symbol
		if marketType == "crypto" {
			symbol = trade.Symbol[8:] // Remove "BINANCE:" prefix
		}

		fmt.Printf("[%s] %s %s: $%.2f, Volume: %.4f\n",
			tradeTime.Format("15:04:05"),
			marketType,
			symbol,
			trade.Price,
			trade.Volume)
	}
}

// main is the entry point of the program that sets up and runs both crypto and stock market data streams.
// It handles graceful shutdown on interrupt signal and displays real-time trade data from both markets.
func main() {
	// Get API key from environment
	apiKey := os.Getenv("FINNHUB_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set FINNHUB_API_KEY environment variable")
	}

	// Define crypto pairs to track
	cryptoPairs := []string{
		crypto.FormatSymbol("BTC", "USDT"), // Bitcoin
		crypto.FormatSymbol("ETH", "USDT"), // Ethereum
		crypto.FormatSymbol("BNB", "USDT"), // Binance Coin
	}

	// Define stock symbols to track
	stockSymbols := []string{
		"AAPL",  // Apple
		"MSFT",  // Microsoft
		"GOOGL", // Google
	}

	// Create crypto streamer with retry
	var cryptoStreamer *crypto.Streamer
	var err error
	for retries := 0; retries < 3; retries++ {
		cryptoStreamer, err = crypto.NewStreamer(apiKey, cryptoPairs)
		if err == nil {
			break
		}
		log.Printf("Attempt %d: Error creating crypto streamer: %v. Waiting 5 seconds...", retries+1, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Fatal("Failed to create crypto streamer after retries:", err)
	}
	defer cryptoStreamer.Close()

	// Wait before creating stock streamer to avoid rate limits
	time.Sleep(2 * time.Second)

	// Create stock streamer with retry
	var stockStreamer *stock.Streamer
	for retries := 0; retries < 3; retries++ {
		stockStreamer, err = stock.NewStreamer(apiKey, stockSymbols)
		if err == nil {
			break
		}
		log.Printf("Attempt %d: Error creating stock streamer: %v. Waiting 5 seconds...", retries+1, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Fatal("Failed to create stock streamer after retries:", err)
	}
	defer stockStreamer.Close()

	// Add handlers
	cryptoStreamer.AddHandler(createTradeHandler("crypto"))
	stockStreamer.AddHandler(createTradeHandler("stock"))

	// Subscribe to streams with delay between them
	if err := cryptoStreamer.Subscribe(); err != nil {
		log.Fatal("Error subscribing to crypto symbols:", err)
	}

	// Wait before subscribing to stock stream
	time.Sleep(2 * time.Second)

	if err := stockStreamer.Subscribe(); err != nil {
		log.Fatal("Error subscribing to stock symbols:", err)
	}

	// Handle interrupt signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Use WaitGroup to manage goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Start crypto streaming
	go func() {
		defer wg.Done()
		if err := cryptoStreamer.Stream(); err != nil {
			log.Printf("Crypto streaming error: %v", err)
			os.Exit(1)
		}
	}()

	// Start stock streaming
	go func() {
		defer wg.Done()
		if err := stockStreamer.Stream(); err != nil {
			log.Printf("Stock streaming error: %v", err)
			os.Exit(1)
		}
	}()

	log.Printf("Both streamers are running. Waiting for market data...\n")
	log.Printf("Crypto pairs: %v\n", cryptoPairs)
	log.Printf("Stock symbols: %v\n", stockSymbols)

	// Wait for interrupt signal
	<-interrupt
	log.Println("Received interrupt signal, closing connections...")
}
