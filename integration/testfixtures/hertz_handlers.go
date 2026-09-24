package testfixtures

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

type CreateInvoiceInput struct {
	Amount   int    `json:"amount" binding:"required"`
	Currency string `json:"currency"`
}

type InvoiceResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type InvoiceHandler struct{}

func (h *InvoiceHandler) Create(ctx context.Context, c *app.RequestContext) {
	var input CreateInvoiceInput
	if err := c.BindAndValidate(&input); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, InvoiceResponse{ID: "invoice-1", Status: "created"})
}

func (h *InvoiceHandler) List(ctx context.Context, c *app.RequestContext) {
	if len(c.Query("fail")) > 0 {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, []InvoiceResponse{{ID: "invoice-1", Status: "created"}})
}

func (h *InvoiceHandler) Index(ctx context.Context, c *app.RequestContext) {
	if len(c.Query("fail")) > 0 {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, map[string]InvoiceResponse{
		"invoice-1": {ID: "invoice-1", Status: "created"},
	})
}
