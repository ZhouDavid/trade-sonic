package engine

import (
	"context"
	"sync"

	"github.com/ZhouDavid/trade-sonic/strategy-engine/internal/strategy"
)

// Engine manages the lifecycle of strategies and signal processing
type Engine struct {
	strategies     map[string]strategy.Strategy
	signalHandler  strategy.SignalHandler
	mu             sync.RWMutex
}

// NewEngine creates a new strategy engine
func NewEngine(signalHandler strategy.SignalHandler) *Engine {
	return &Engine{
		strategies:    make(map[string]strategy.Strategy),
		signalHandler: signalHandler,
	}
}

// RegisterStrategy adds a new strategy to the engine
func (e *Engine) RegisterStrategy(s strategy.Strategy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.strategies[s.Name()]; exists {
		return ErrStrategyAlreadyExists
	}

	e.strategies[s.Name()] = s
	return nil
}

// UnregisterStrategy removes a strategy from the engine
func (e *Engine) UnregisterStrategy(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if s, exists := e.strategies[name]; exists {
		if err := s.Cleanup(context.Background()); err != nil {
			return err
		}
		delete(e.strategies, name)
		return nil
	}
	return ErrStrategyNotFound
}

// ProcessMarketData sends market data to all registered strategies
func (e *Engine) ProcessMarketData(ctx context.Context, data strategy.MarketData) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, s := range e.strategies {
		signal, err := s.ProcessData(ctx, data)
		if err != nil {
			// Log error but continue processing other strategies
			continue
		}
		if signal != nil {
			if err := e.signalHandler.HandleSignal(ctx, signal); err != nil {
				// Log error but continue processing
				continue
			}
		}
	}
	return nil
}

// GetStrategy returns a strategy by name
func (e *Engine) GetStrategy(name string) (strategy.Strategy, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s, exists := e.strategies[name]
	return s, exists
}

// ListStrategies returns all registered strategy names
func (e *Engine) ListStrategies() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	names := make([]string, 0, len(e.strategies))
	for name := range e.strategies {
		names = append(names, name)
	}
	return names
}
