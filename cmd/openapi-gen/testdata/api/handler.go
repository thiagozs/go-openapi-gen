package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/thiagozs/go-openapi-gen/cmd/openapi-gen/testdata/api/store"
)

type handler struct{ store store.Store }

func problem(c *gin.Context, status int, title, detail string) {
	c.JSON(status, gin.H{"title": title, "detail": detail})
}
func internalError(c *gin.Context, op string, err error) { problem(c, 500, op, err.Error()) }

type aliasRequest struct {
	URL string `json:"url" binding:"required"`
}

func (h handler) putAlias(c *gin.Context) {
	name := c.Param("name")
	var in aliasRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		problem(c, 400, "JSON inválido", err.Error())
		return
	}
	if err := h.store.UpsertAlias(c, name, in.URL, in.URL); err != nil {
		internalError(c, "salvar apelido", err)
		return
	}
	alias, err := h.store.GetAlias(c, name)
	if err != nil {
		internalError(c, "buscar apelido", err)
		return
	}
	c.JSON(http.StatusOK, alias)
}

func (h handler) getAlias(c *gin.Context) {
	alias, err := h.store.GetAlias(c, c.Param("name"))
	if store.IsNotFound(err) {
		problem(c, 404, "apelido não encontrado", "")
		return
	}
	if err != nil {
		internalError(c, "buscar apelido", err)
		return
	}
	c.JSON(http.StatusOK, alias)
}

func (h handler) deleteAlias(c *gin.Context) {
	if err := h.store.DeleteAlias(c, c.Param("name")); err != nil && !store.IsNotFound(err) {
		internalError(c, "remover apelido", err)
		return
	}
	c.Status(http.StatusNoContent)
}

type retentionResponse struct {
	KeepLast int `json:"keep_last"`
	TTLDays  int `json:"ttl_days"`
}

type retentionRequest struct {
	KeepLast *int `json:"keep_last"`
	TTLDays  *int `json:"ttl_days"`
}

func (h handler) getRetention(c *gin.Context) {
	policy, err := h.store.GetRetentionPolicy(c)
	if err != nil {
		internalError(c, "buscar política de retenção", err)
		return
	}
	c.JSON(http.StatusOK, retentionResponse{KeepLast: policy.Keep, TTLDays: int(policy.TTL.Hours() / 24)})
}

func (h handler) putRetention(c *gin.Context) {
	var in retentionRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		problem(c, 400, "JSON inválido", err.Error())
		return
	}
	policy, err := h.store.GetRetentionPolicy(c)
	if err != nil {
		internalError(c, "buscar política de retenção", err)
		return
	}
	if in.KeepLast != nil {
		policy.Keep = *in.KeepLast
	}
	if in.TTLDays != nil {
		policy.TTL = time.Duration(*in.TTLDays) * 24 * time.Hour
	}
	if err := h.store.SetRetentionPolicy(c, policy); err != nil {
		internalError(c, "salvar política de retenção", err)
		return
	}
	c.JSON(http.StatusOK, retentionResponse{KeepLast: policy.Keep, TTLDays: int(policy.TTL.Hours() / 24)})
}

func redirectTo(c *gin.Context, target string) {
	if c.GetHeader("HX-Request") == "true" {
		c.Status(http.StatusOK)
	} else {
		c.Redirect(http.StatusFound, target)
	}
}

func redirectWithMessage(c *gin.Context, msg string) { redirectTo(c, "/x?m="+msg) }

// formThenRedirect reads a form and redirects through nested helpers.
func (h handler) formThenRedirect(c *gin.Context) {
	if c.PostForm("name") == "" {
		redirectWithMessage(c, "error")
		return
	}
	redirectWithMessage(c, "ok")
}

func (h handler) directRedirect(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "/done")
}
