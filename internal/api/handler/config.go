package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/config"
)

// ConfigHandler handles configuration API requests.
type ConfigHandler struct {
	cfg      *config.Config // fallback snapshot when a fresh load fails
	loadFn   func() (*config.Config, error)
	reloadFn func() error
}

// NewConfigHandler creates a new ConfigHandler. loadFn reads the config
// from disk so GET always reflects the current files (not the snapshot
// taken at startup); reloadFn triggers a live reload.
func NewConfigHandler(cfg *config.Config, loadFn func() (*config.Config, error), reloadFn func() error) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, loadFn: loadFn, reloadFn: reloadFn}
}

// Get returns the current configuration (sanitized).
func (h *ConfigHandler) Get(c *gin.Context) {
	cfg := h.cfg
	if h.loadFn != nil {
		if fresh, err := h.loadFn(); err == nil {
			cfg = fresh
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": cfg.Sanitize()})
}

// Reload triggers a configuration reload.
func (h *ConfigHandler) Reload(c *gin.Context) {
	if h.reloadFn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reload not available"})
		return
	}
	if err := h.reloadFn(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "configuration reloaded"})
}
