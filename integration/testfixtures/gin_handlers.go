package testfixtures

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type CreatePaymentInput struct {
	Amount   int    `json:"amount" binding:"required"`
	Currency string `json:"currency"`
}

type PaymentResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type PaymentHandler struct{}

func (h *PaymentHandler) Create(c *gin.Context) {
	var input CreatePaymentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, PaymentResponse{ID: "payment-1", Status: "created"})
}

func (h *PaymentHandler) List(c *gin.Context) {
	if c.Query("fail") != "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, []PaymentResponse{{ID: "payment-1", Status: "created"}})
}

func (h *PaymentHandler) Index(c *gin.Context) {
	if c.Query("fail") != "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, map[string]PaymentResponse{
		"payment-1": {ID: "payment-1", Status: "created"},
	})
}
