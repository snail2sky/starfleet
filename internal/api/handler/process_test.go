package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/api/middleware"
	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
)

func setupTestRouter(mgr *engine.Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProcessHandler(mgr)
	r.GET("/api/v1/processes", h.List)
	r.GET("/api/v1/processes/:name", h.Get)
	r.POST("/api/v1/processes/:name/start", h.Start)
	r.POST("/api/v1/processes/:name/stop", h.Stop)
	r.POST("/api/v1/processes/:name/restart", h.Restart)
	return r
}

func TestProcessListEmpty(t *testing.T) {
	mgr := engine.NewManager()
	mgr.LoadFromConfig(&config.Config{Processes: map[string]config.ProcessConfig{}})
	r := setupTestRouter(mgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/processes", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("expected 0 processes, got %d", len(data))
	}
}

func TestProcessListWithConfig(t *testing.T) {
	mgr := engine.NewManager()
	cfg := &config.Config{
		Processes: map[string]config.ProcessConfig{
			"nginx": {Command: "nginx", Autostart: true},
		},
	}
	mgr.LoadFromConfig(cfg)
	r := setupTestRouter(mgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/processes", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 process, got %d", len(data))
	}
}

func TestProcessGetNotFound(t *testing.T) {
	mgr := engine.NewManager()
	mgr.LoadFromConfig(&config.Config{Processes: map[string]config.ProcessConfig{}})
	r := setupTestRouter(mgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/processes/nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProcessGetFound(t *testing.T) {
	mgr := engine.NewManager()
	cfg := &config.Config{
		Processes: map[string]config.ProcessConfig{
			"test": {Command: "echo hello"},
		},
	}
	mgr.LoadFromConfig(cfg)
	r := setupTestRouter(mgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/processes/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if data["name"] != "test" {
		t.Errorf("expected name test, got %s", data["name"])
	}
}

func TestProcessStartNotFound(t *testing.T) {
	mgr := engine.NewManager()
	mgr.LoadFromConfig(&config.Config{Processes: map[string]config.ProcessConfig{}})
	r := setupTestRouter(mgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/processes/nonexistent/start", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSystemHandler(middleware.NewAuthState(middleware.AuthConfig{Enabled: false}))
	r.GET("/health", h.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp["auth"] != false {
		t.Errorf("expected auth=false, got %v", resp["auth"])
	}
}

func TestSystemOverview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSystemHandler(middleware.NewAuthState(middleware.AuthConfig{Enabled: false}))
	r.GET("/api/v1/system/overview", h.Overview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/system/overview", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if _, ok := data["host"]; !ok {
		t.Error("expected host in response")
	}
	if _, ok := data["cpu"]; !ok {
		t.Error("expected cpu in response")
	}
	if _, ok := data["memory"]; !ok {
		t.Error("expected memory in response")
	}
}

// TestHealthVersionVisibility: the build version must appear on the
// authenticated health endpoint (WebUI sidebar) but NOT on the public
// probe (no unauthenticated fingerprinting).
func TestHealthVersionVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetVersion("9.9.9-test")
	defer SetVersion("dev")

	state := middleware.NewAuthState(middleware.AuthConfig{Enabled: false})
	h := NewSystemHandler(state)
	r := gin.New()
	r.GET("/health", h.Health)
	r.GET("/api/v1/system/health", h.HealthDetailed)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	var pub map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &pub); err != nil {
		t.Fatalf("public health json: %v", err)
	}
	if _, dup := pub["version"]; dup {
		t.Errorf("public /health must not leak version: %v", pub)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/system/health", nil))
	var det map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &det); err != nil {
		t.Fatalf("detailed health json: %v", err)
	}
	if det["version"] != "9.9.9-test" {
		t.Errorf("expected version in detailed health, got %v", det["version"])
	}
}
