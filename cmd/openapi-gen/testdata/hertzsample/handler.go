package hertzsample

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

type CreateRequest struct {
	Amount int `json:"amount" binding:"required"`
}

type CreateResponse struct {
	ID string `json:"id"`
}

type Handler struct{}

func (h *Handler) Create(ctx context.Context, c *app.RequestContext) {
	var request CreateRequest
	if err := c.BindAndValidate(&request); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, CreateResponse{ID: "1"})
}
