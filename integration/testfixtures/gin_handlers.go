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

func (h *PaymentHandler) ListEnvelope(c *gin.Context) {
	endpoints := []PaymentResponse{{ID: "payment-1", Status: "created"}}
	if c.Query("fail") != "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "tenant not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": endpoints})
}

func (h *PaymentHandler) Delete(c *gin.Context) {
	if c.Query("fail") != "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "tenant not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// PageHandler serves HTML/form/markdown endpoints.
type PageHandler struct{}

func (h *PageHandler) Login(c *gin.Context) {
	_ = c.PostForm("username")
	_ = c.Query("next")
	c.HTML(http.StatusOK, "login.html", nil)
}

func (h *PageHandler) Logout(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (h *PageHandler) Report(c *gin.Context) {
	_ = c.DefaultQuery("refresh", "false")
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", nil)
}

func (h *PageHandler) BoundUpload(c *gin.Context) {
	var input struct {
		Name string `form:"name"`
	}
	_ = c.ShouldBind(&input)
	_, _ = c.FormFile("file")
}

func (h *PageHandler) MixedResponse(c *gin.Context) {
	c.String(http.StatusBadRequest, "invalid")
	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid"})
	c.HTML(http.StatusCreated, "created", nil)
}

func (h *PageHandler) RawBody(c *gin.Context) {
	_, _ = c.GetRawData()
}

func (h *PageHandler) DynamicPostForm(c *gin.Context) {
	field := c.Query("field")
	_ = c.PostForm(field)
}
