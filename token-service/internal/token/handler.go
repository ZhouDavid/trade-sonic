package token

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

type TokenRequest struct {
	AccountType AccountType `json:"account_type" binding:"required"`
}

func NewHandler() (*Handler, error) {
	service, err := NewService()
	if err != nil {
		return nil, err
	}

	return &Handler{
		service: service,
	}, nil
}

// GetToken returns a token for the specified account type
func (h *Handler) GetToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.GetToken(req.AccountType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
