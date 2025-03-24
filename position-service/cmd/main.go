package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/trade-sonic/position-service/internal/position"
)

func main() {
	// Create a new Gin router
	r := gin.Default()

	// Get Robinhood account ID from environment variable or use a default for development
	accountID := os.Getenv("ROBINHOOD_ACCOUNT_ID")
	if accountID == "" {
		accountID = "507617876"
		log.Printf("Warning: Using default account ID. Set ROBINHOOD_ACCOUNT_ID environment variable for production.")
	}

	// Initialize the token client
	// Assuming the token service is running on localhost:8080
	tokenClient := position.NewTokenClient("http://localhost:8080")

	// Initialize the position service with the account ID
	positionService := position.NewService(tokenClient, accountID)

	// Initialize the position handler
	handler := position.NewHandler(positionService)

	// Register routes
	r.POST("/positions", handler.GetPositions)

	// Add a health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
		})
	})

	// Start the server
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
