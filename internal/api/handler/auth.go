package handler

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/api/middleware"
)

// tokenTTL is how long a login token stays valid.
const tokenTTL = 24 * time.Hour

// AuthHandler handles the login endpoint.
type AuthHandler struct {
	state *middleware.AuthState
}

// NewAuthHandler creates a handler for POST /api/v1/auth/login.
func NewAuthHandler(state *middleware.AuthState) *AuthHandler {
	return &AuthHandler{state: state}
}

// Login validates credentials and issues a JWT for the WebUI.
func (h *AuthHandler) Login(c *gin.Context) {
	cfg := h.state.Load()
	if !cfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "authentication is disabled"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Username), []byte(cfg.Username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(req.Password), []byte(cfg.Password)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	token, err := middleware.GenerateToken(cfg, req.Username, tokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": tokenTTL.Seconds()})
}
