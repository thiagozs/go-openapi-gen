package sample

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type CreateRequest struct {
	Amount int `json:"amount" binding:"required"`
}

type CreateResponse struct {
	ID string `json:"id"`
}

type Handler struct{}

func (h *Handler) Create(c *gin.Context) {
	var request CreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, CreateResponse{ID: "1"})
}
