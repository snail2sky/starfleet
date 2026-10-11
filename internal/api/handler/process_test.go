package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/snail-sky/starfleet/internal/config"
	"github.com/snail-sky/starfleet/internal/engine"
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
	json.Unmarshal(w.Body.Bytes(), &resp)
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
	json.Unmarshal(w.Body.Bytes(), &resp)
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
	json.Unmarshal(w.Body.Bytes(), &resp)
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
	h := NewSystemHandler()
	r.GET("/health", h.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSystemOverview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSystemHandler()
	r.GET("/api/v1/system/overview", h.Overview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/system/overview", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
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