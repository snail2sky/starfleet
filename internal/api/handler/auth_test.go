package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/api/middleware"
)

func setupAuthRoutes(cfg middleware.AuthConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	state := middleware.NewAuthState(cfg)
	login := NewAuthHandler(state)
	r.POST("/api/v1/auth/login", login.Login)
	r.GET("/api/v1/protected", middleware.Authenticate(state), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func postLogin(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestLoginSuccessAndTokenWorks(t *testing.T) {
	cfg := middleware.AuthConfig{Enabled: true, Username: "admin", Password: "secret", Secret: "jwt-key"}
	r := setupAuthRoutes(cfg)

	w := postLogin(r, `{"username":"admin","password":"secret"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Token == "" {
		t.Fatalf("expected token in response, got: %s", w.Body.String())
	}

	// The issued token must pass the Authenticate middleware.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+resp.Token)
	pw := httptest.NewRecorder()
	r.ServeHTTP(pw, req)
	if pw.Code != http.StatusOK {
		t.Errorf("Bearer token rejected: %d", pw.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	cfg := middleware.AuthConfig{Enabled: true, Username: "admin", Password: "secret", Secret: "jwt-key"}
	r := setupAuthRoutes(cfg)
	if w := postLogin(r, `{"username":"admin","password":"nope"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLoginAuthDisabled(t *testing.T) {
	cfg := middleware.AuthConfig{Enabled: false}
	r := setupAuthRoutes(cfg)
	if w := postLogin(r, `{"username":"a","password":"b"}`); w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when auth disabled, got %d", w.Code)
	}
}
