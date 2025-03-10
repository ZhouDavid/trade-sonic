# Token Service

A service that manages and caches authentication tokens for various trading platforms.

## Configuration

1. Copy `config.json.example` to `config.json`:
```bash
cp config.json.example config.json
```

2. Update `config.json` with your credentials:
```json
{
    "robinhood": {
        "username": "your_robinhood_username",
        "password": "your_robinhood_password"
    }
}
```

## Running the Service

```bash
go run cmd/main.go
```

## API

### Get Token
```bash
curl -X POST http://localhost:8080/token \
  -H "Content-Type: application/json" \
  -d '{"account_type": "robinhood"}'
```

Response:
```json
{
    "access_token": "your_access_token",
    "expires_at": "2025-03-09T23:52:25Z"
}
```
