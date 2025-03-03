package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ZhouDavid/trade-sonic/strategy-engine/internal/engine"
	"github.com/ZhouDavid/trade-sonic/strategy-engine/internal/strategy"
	"github.com/ZhouDavid/trade-sonic/strategy-engine/internal/strategy/stoploss"
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
		var strat strategy.Strategy
		var err error

		switch stratCfg.Type {
		case "stop_loss":
			strat, err = stoploss.NewStopLossStrategy(stratCfg.Parameters)
		default:
			log.Printf("Unknown strategy type: %s\n", stratCfg.Type)
			continue
		}

		if err != nil {
			log.Printf("Error initializing strategy %s: %v\n", stratCfg.Name, err)
			continue
		}

		if err := strategyEngine.RegisterStrategy(strat); err != nil {
			log.Printf("Error registering strategy %s: %v\n", stratCfg.Name, err)
			continue
		}

		log.Printf("Successfully initialized and registered strategy: %s\n", stratCfg.Name)
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
	// Try to load config file from the same directory as the binary
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Could not get executable path: %v, using default config", err)
		return getDefaultConfig()
	}

	configFile := filepath.Join(filepath.Dir(execPath), "config.json")
	// Also check in the current directory as fallback
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		configFile = "strategy-engine/cmd/engine/config.json"
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("Could not read config file: %v, using default config", err)
		return getDefaultConfig()
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Could not parse config file: %v, using default config", err)
		return getDefaultConfig()
	}

	return &config
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() *Config {
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
