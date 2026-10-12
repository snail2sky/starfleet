package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/config"
)

// TestConfigGetReadsFresh proves GET /config reflects the loadFn result
// (disk) rather than the startup snapshot — the config-reload staleness bug.
func TestConfigGetReadsFresh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	startup := &config.Config{Server: config.ServerConfig{Port: 8080}}
	fresh := &config.Config{Server: config.ServerConfig{Port: 9999}}

	h := NewConfigHandler(startup,
		func() (*config.Config, error) { return fresh, nil },
		nil)
	r := gin.New()
	r.GET("/api/v1/config", h.Get)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Data config.Config `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp.Data.Server.Port != 9999 {
		t.Errorf("expected fresh port 9999, got %d", resp.Data.Server.Port)
	}
}

// TestConfigGetFallsBackOnLoadError: if disk read fails, serve the snapshot.
func TestConfigGetFallsBackOnLoadError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	startup := &config.Config{Server: config.ServerConfig{Port: 8080}}
	h := NewConfigHandler(startup,
		func() (*config.Config, error) { return nil, os.ErrNotExist },
		nil)
	r := gin.New()
	r.GET("/api/v1/config", h.Get)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config", nil))
	var resp struct {
		Data config.Config `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp.Data.Server.Port != 8080 {
		t.Errorf("expected fallback port 8080, got %d", resp.Data.Server.Port)
	}
}
