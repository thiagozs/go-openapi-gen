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

func (h *Handler) List(c *gin.Context) {
	if c.Query("fail") != "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, []CreateResponse{{ID: "1"}})
}

func (h *Handler) Index(c *gin.Context) {
	if c.Query("fail") != "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, map[string]CreateResponse{"first": {ID: "1"}})
}
