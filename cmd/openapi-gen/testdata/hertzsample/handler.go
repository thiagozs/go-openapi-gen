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

func (h *Handler) List(ctx context.Context, c *app.RequestContext) {
	if len(c.Query("fail")) > 0 {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, []CreateResponse{{ID: "1"}})
}

func (h *Handler) Index(ctx context.Context, c *app.RequestContext) {
	if len(c.Query("fail")) > 0 {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, map[string]CreateResponse{"first": {ID: "1"}})
}
