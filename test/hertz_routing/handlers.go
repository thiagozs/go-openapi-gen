package hertz_routing

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

type OauthHandler struct{}

func NewOauthHandler() *OauthHandler {
	return &OauthHandler{}
}

func (h *OauthHandler) Login(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, map[string]any{
		"auth_url": "https://oauth.provider.com/auth",
		"state":    "random-state-token",
	})
}

func (h *OauthHandler) Callback(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, map[string]any{
		"access_token":  "jwt-access-token",
		"refresh_token": "jwt-refresh-token",
		"expires_in":    3600,
		"is_new_user":   true,
		"user": map[string]any{
			"id":             "user-123",
			"email":          "user@example.com",
			"first_name":     "John",
			"last_name":      "Doe",
			"full_name":      "John Doe",
			"status":         "active",
			"email_verified": true,
			"mfa_enabled":    false,
			"last_login_at":  "2023-01-01T00:00:00Z",
			"created_at":     "2023-01-01T00:00:00Z",
			"updated_at":     "2023-01-01T00:00:00Z",
		},
	})
}

func (h *OauthHandler) GetProviders(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, map[string]any{
		"google": map[string]any{
			"name":      "Google",
			"client_id": "google-client-id",
			"auth_url":  "https://accounts.google.com/oauth/authorize",
			"token_url": "https://oauth2.googleapis.com/token",
			"user_info": "https://api.github.com/user",
		},
		"github": map[string]any{
			"name":      "GitHub",
			"client_id": "github-client-id",
			"auth_url":  "https://github.com/login/oauth/authorize",
			"token_url": "https://github.com/login/oauth/access_token",
			"user_info": "https://api.github.com/user",
		},
	})
}
