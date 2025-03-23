package position

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for positions
type Handler struct {
	service *Service
}

// PositionRequest represents a request for positions
type PositionRequest struct {
	AccountType AccountType `json:"account_type" binding:"required"`
}

// NewHandler creates a new position handler
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// GetPositions handles requests to get positions
func (h *Handler) GetPositions(c *gin.Context) {
	var req PositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	positions, err := h.service.GetPositions(req.AccountType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, positions)
}
