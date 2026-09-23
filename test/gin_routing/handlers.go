package gin_routing

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type OauthHandler struct{}

func NewOauthHandler() *OauthHandler {
	return &OauthHandler{}
}

func (h *OauthHandler) Login(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"auth_url": "https://oauth.provider.com/auth",
		"state":    "random-state-token",
	})
}

func (h *OauthHandler) Callback(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"access_token":  "jwt-access-token",
		"refresh_token": "jwt-refresh-token",
		"expires_in":    3600,
		"is_new_user":   true,
		"user": gin.H{
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

func (h *OauthHandler) GetProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"google": gin.H{
			"name":      "Google",
			"client_id": "google-client-id",
			"auth_url":  "https://accounts.google.com/oauth/authorize",
			"token_url": "https://oauth2.googleapis.com/token",
			"user_info": "https://www.googleapis.com/oauth2/v2/userinfo",
		},
		"github": gin.H{
			"name":      "GitHub",
			"client_id": "github-client-id",
			"auth_url":  "https://github.com/login/oauth/authorize",
			"token_url": "https://github.com/login/oauth/access_token",
			"user_info": "https://api.github.com/user",
		},
	})
}
