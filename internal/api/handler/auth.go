package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/api/middleware"
)

// tokenTTL is how long a login token stays valid.
const tokenTTL = 24 * time.Hour

// Login hardening: body cap and per-IP failed-attempt rate limiting.
const (
	maxLoginBody = 64 * 1024       // bytes
	loginFails   = 10              // failed attempts allowed...
	loginWindow  = 5 * time.Minute // ...per IP within this window
)

// AuthHandler handles the login endpoint.
type AuthHandler struct {
	state *middleware.AuthState

	mu    sync.Mutex
	fails map[string][]time.Time // client IP -> recent failure timestamps
}

// NewAuthHandler creates a handler for POST /api/v1/auth/login.
func NewAuthHandler(state *middleware.AuthState) *AuthHandler {
	return &AuthHandler{state: state, fails: make(map[string][]time.Time)}
}

// Login validates credentials and issues a JWT for the WebUI.
func (h *AuthHandler) Login(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxLoginBody)

	ip := c.ClientIP()
	if !h.allowAttempt(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed login attempts, try again later"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	cfg := h.state.Load()
	if !cfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "authentication is disabled"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Username), []byte(cfg.Username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(req.Password), []byte(cfg.Password)) != 1 {
		h.recordFail(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	h.clearFails(ip)

	token, err := middleware.GenerateToken(cfg, req.Username, tokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": tokenTTL.Seconds()})
}

// allowAttempt reports whether a login attempt from ip may proceed;
// too many recent failures are rejected with 429 upstream.
func (h *AuthHandler) allowAttempt(ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	recent := h.fails[ip][:0]
	for _, t := range h.fails[ip] {
		if now.Sub(t) < loginWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) == 0 {
		delete(h.fails, ip)
	} else {
		h.fails[ip] = recent
	}
	// Opportunistic sweep so a spoofed-IP flood cannot grow the map forever.
	if len(h.fails) > 4096 {
		for k, ts := range h.fails {
			if now.Sub(ts[len(ts)-1]) >= loginWindow {
				delete(h.fails, k)
			}
		}
	}
	return len(recent) < loginFails
}

func (h *AuthHandler) recordFail(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fails[ip] = append(h.fails[ip], time.Now())
}

func (h *AuthHandler) clearFails(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.fails, ip)
}
