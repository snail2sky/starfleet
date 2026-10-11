package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/config"
)

// ConfigHandler handles configuration API requests.
type ConfigHandler struct {
	cfg      *config.Config
	reloadFn func() error
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(cfg *config.Config, reloadFn func() error) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, reloadFn: reloadFn}
}

// Get returns the current configuration (sanitized).
func (h *ConfigHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.cfg.Sanitize()})
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
