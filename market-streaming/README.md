# Market Streaming

A Go package for streaming real-time market data from Finnhub.io.

## Package Structure

```
market-streaming/
├── cmd/
│   └── streamer/       # Main application
│       └── main.go
├── internal/
│   └── stream/         # Market streaming package
│       ├── models.go   # Data models
│       └── streamer.go # Streaming implementation
├── go.mod
└── README.md
```

## Features

- Real-time market data streaming using WebSocket
- Support for multiple stock symbols
- Extensible handler system for processing trade data
- Clean shutdown on interrupt

## Usage

1. Set your Finnhub API key:
```bash
export FINNHUB_API_KEY=your_api_key_here
```

2. Install dependencies:
```bash
cd market-streaming
go mod tidy
```

3. Run the streamer:
```bash
go run cmd/streamer/main.go
```

## Example Output

```
Symbol: AAPL, Price: $182.50, Volume: 100.00
Symbol: MSFT, Price: $402.75, Volume: 50.00
Symbol: GOOGL, Price: $143.96, Volume: 75.00
...
```

Press Ctrl+C to stop the program.
