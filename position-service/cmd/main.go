package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/trade-sonic/position-service/internal/position"
)

func main() {
	// Create a new Gin router
	r := gin.Default()

	// Initialize the token client
	// Assuming the token service is running on localhost:8080
	tokenClient := position.NewTokenClient("http://localhost:8080")

	// Initialize the position service
	positionService := position.NewService(tokenClient)

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
