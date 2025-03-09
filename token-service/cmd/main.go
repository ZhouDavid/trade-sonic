package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/trade-sonic/token-service/internal/token"
)

func main() {
	r := gin.Default()

	handler, err := token.NewHandler()
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	r.POST("/token", handler.GetToken)

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
