package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ZhouDavid/trade-sonic/strategy-engine/internal/engine"
	"github.com/ZhouDavid/trade-sonic/strategy-engine/internal/strategy"
)

// Config holds the configuration for the strategy engine
type Config struct {
	QueueConfig struct {
		// Add your queue configuration here (e.g., Redis, RabbitMQ, etc.)
		Address string `json:"address"`
		Channel string `json:"channel"`
		GroupID string `json:"groupId"`
	} `json:"queue"`
	Strategies []struct {
		Name       string                 `json:"name"`
		Type       string                 `json:"type"`
		Parameters map[string]interface{} `json:"parameters"`
	} `json:"strategies"`
}

// SignalProcessor implements the strategy.SignalHandler interface
type SignalProcessor struct {
	// Add fields for signal processing (e.g., order execution client)
}

func (sp *SignalProcessor) HandleSignal(ctx context.Context, signal *strategy.Signal) error {
	// Implement signal handling logic (e.g., send to order execution service)
	log.Printf("Processing signal: %+v\n", signal)
	return nil
}

func main() {
	// Load configuration
	config := loadConfig()

	// Create signal handler
	signalHandler := &SignalProcessor{}

	// Create strategy engine
	strategyEngine := engine.NewEngine(signalHandler)

	// Initialize strategies from config
	for _, stratCfg := range config.Strategies {
		// In a real implementation, you would have a strategy factory
		// that creates the appropriate strategy based on stratCfg.Type
		// For now, we'll just log
		log.Printf("Would initialize strategy: %s of type %s\n", stratCfg.Name, stratCfg.Type)
	}

	// Create context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// WaitGroup for coordinating shutdown
	var wg sync.WaitGroup

	// Start market data consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumeMarketData(ctx, strategyEngine, config)
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal")

	// Cancel context to initiate shutdown
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()
	log.Println("Strategy engine shutdown complete")
}

func loadConfig() *Config {
	// Try to load config from file
	configFile := "config.json"
	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("Could not read config file: %v, using default config", err)
		// Return default config if file cannot be read
		return &Config{
			QueueConfig: struct {
				Address string `json:"address"`
				Channel string `json:"channel"`
				GroupID string `json:"groupId"`
			}{
				Address: "localhost:6379",
				Channel: "market_data",
				GroupID: "strategy_engine",
			},
		}
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Could not parse config file: %v, using default config", err)
		// Return default config if file cannot be parsed
		return &Config{
			QueueConfig: struct {
				Address string `json:"address"`
				Channel string `json:"channel"`
				GroupID string `json:"groupId"`
			}{
				Address: "localhost:6379",
				Channel: "market_data",
				GroupID: "strategy_engine",
			},
		}
	}

	return &config
}

func consumeMarketData(ctx context.Context, e *engine.Engine, cfg *Config) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// In a real implementation, you would:
			// 1. Read from your queue (Redis, RabbitMQ, etc.)
			// 2. Deserialize the market data
			// 3. Process it through the engine

			// For now, we'll just simulate with dummy data
			data := strategy.MarketData{
				Symbol:    "BTC-USD",
				Price:     50000.0,
				Volume:    1.5,
				Timestamp: time.Now(),
			}

			if err := e.ProcessMarketData(ctx, data); err != nil {
				log.Printf("Error processing market data: %v\n", err)
			}
		}
	}
}
