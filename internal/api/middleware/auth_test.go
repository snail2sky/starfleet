package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupAuthRouter(cfg AuthConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Authenticate(NewAuthState(cfg)))
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func doAuthReq(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestAuthenticateDisabled(t *testing.T) {
	r := setupAuthRouter(AuthConfig{Enabled: false})
	if w := doAuthReq(r, ""); w.Code != http.StatusOK {
		t.Errorf("expected 200 when auth disabled, got %d", w.Code)
	}
}

func TestAuthenticateNoCredentials(t *testing.T) {
	r := setupAuthRouter(AuthConfig{Enabled: true, Username: "u", Password: "p"})
	w := doAuthReq(r, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthenticateWrongPassword(t *testing.T) {
	r := setupAuthRouter(AuthConfig{Enabled: true, Username: "u", Password: "p"})
	// base64("u:wrong")
	w := doAuthReq(r, "Basic dTp3cm9uZw==")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", w.Code)
	}
}

func TestAuthenticateBasicOK(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Username: "admin", Password: "secret"}
	r := setupAuthRouter(cfg)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.SetBasicAuth("admin", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid basic auth, got %d", w.Code)
	}
}

func TestAuthenticateJWT(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Username: "admin", Password: "secret", Secret: "jwt-key"}
	token, err := GenerateToken(cfg, "admin", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	r := setupAuthRouter(cfg)
	if w := doAuthReq(r, "Bearer "+token); w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid JWT, got %d", w.Code)
	}
}

func TestAuthenticateInvalidJWT(t *testing.T) {
	cfg := AuthConfig{Enabled: true, Username: "admin", Password: "secret", Secret: "jwt-key"}
	r := setupAuthRouter(cfg)
	if w := doAuthReq(r, "Bearer not-a-token"); w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for garbage JWT, got %d", w.Code)
	}
	// Token signed with a different secret must be rejected.
	other, _ := GenerateToken(AuthConfig{Secret: "wrong-key"}, "admin", time.Hour)
	if w := doAuthReq(r, "Bearer "+other); w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for JWT signed with wrong secret, got %d", w.Code)
	}
}

// TestAuthStateHotUpdate is the regression test for config-reload auth:
// toggling/storing new credentials on the shared state must affect the
// next request without rebuilding the middleware.
func TestAuthStateHotUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state := NewAuthState(AuthConfig{Enabled: false})
	r := gin.New()
	r.Use(Authenticate(state))
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	// Auth disabled: open.
	if w := doAuthReq(r, ""); w.Code != http.StatusOK {
		t.Fatalf("disabled: expected 200, got %d", w.Code)
	}

	// Reload enables auth: next request must be rejected.
	state.Store(AuthConfig{Enabled: true, Username: "u", Password: "p", Secret: "s"})
	if w := doAuthReq(r, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("after enable: expected 401, got %d", w.Code)
	}

	// Reload rotates credentials: old password rejected, new accepted.
	state.Store(AuthConfig{Enabled: true, Username: "u", Password: "new-pass", Secret: "s"})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.SetBasicAuth("u", "p")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old password: expected 401, got %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.SetBasicAuth("u", "new-pass")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("new password: expected 200, got %d", w.Code)
	}

	// Reload disables auth: open again.
	state.Store(AuthConfig{Enabled: false})
	if w := doAuthReq(r, ""); w.Code != http.StatusOK {
		t.Fatalf("after disable: expected 200, got %d", w.Code)
	}
}
