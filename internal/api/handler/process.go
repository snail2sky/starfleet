package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/engine"
)

// ProcessHandler handles process-related API requests.
type ProcessHandler struct {
	manager *engine.Manager
	// persistAutostart writes the new autostart value back to the config
	// file that defines the process. Optional (nil = runtime only).
	persistAutostart func(name string, enabled bool) error
}

// NewProcessHandler creates a new ProcessHandler.
func NewProcessHandler(mgr *engine.Manager) *ProcessHandler {
	return &ProcessHandler{manager: mgr}
}

// SetAutostartPersister injects the config-file persistence hook.
func (h *ProcessHandler) SetAutostartPersister(fn func(name string, enabled bool) error) {
	h.persistAutostart = fn
}

// List returns all processes.
func (h *ProcessHandler) List(c *gin.Context) {
	processes := h.manager.List()
	if processes == nil {
		processes = []engine.ProcessInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"data": processes})
}

// Get returns a single process by name (runtime info + full config).
func (h *ProcessHandler) Get(c *gin.Context) {
	name := c.Param("name")
	info, err := h.manager.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	resp := gin.H{"data": info}
	if cfg, cfgErr := h.manager.GetConfig(name); cfgErr == nil {
		resp["config"] = cfg
	}
	c.JSON(http.StatusOK, resp)
}

// Start starts a process.
func (h *ProcessHandler) Start(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.Start(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "process started", "name": name})
}

// Stop stops a process.
func (h *ProcessHandler) Stop(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.Stop(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "process stopped", "name": name})
}

// Restart restarts a process.
func (h *ProcessHandler) Restart(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.Restart(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "process restarted", "name": name})
}

// setAutostart enables/disables boot-start and persists it to the config.
func (h *ProcessHandler) setAutostart(c *gin.Context, enabled bool) {
	name := c.Param("name")
	if err := h.manager.SetAutostart(name, enabled); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if h.persistAutostart != nil {
		if err := h.persistAutostart(name, enabled); err != nil {
			// Runtime state already changed; report the persistence failure
			// so the user knows the change may not survive a restart.
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   err.Error(),
				"warning": "changed in memory but not persisted to config file",
			})
			return
		}
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	c.JSON(http.StatusOK, gin.H{"message": name + " " + state, "autostart": enabled})
}

// Enable turns on boot-start for a process.
func (h *ProcessHandler) Enable(c *gin.Context) { h.setAutostart(c, true) }

// Disable turns off boot-start for a process (stays running until stopped).
func (h *ProcessHandler) Disable(c *gin.Context) { h.setAutostart(c, false) }

// parseLines extracts the ?lines= query parameter with a default and cap.
func parseLines(c *gin.Context, def, max int) int {
	n := def
	if v := c.Query("lines"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	if n > max {
		n = max
	}
	return n
}

// Logs returns the recent log lines for a process.
// GET /api/v1/processes/:name/logs?lines=200&stream=stdout|stderr
func (h *ProcessHandler) Logs(c *gin.Context) {
	name := c.Param("name")
	if _, err := h.manager.Get(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	lines := parseLines(c, 200, 2000)
	entries := engine.DefaultCollector().LastN(name, lines)
	if stream := c.Query("stream"); stream != "" {
		filtered := entries[:0]
		for _, e := range entries {
			if e.Stream == stream {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	if entries == nil {
		entries = []engine.LogEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// AllLogs returns recent log lines across all processes.
// GET /api/v1/logs?lines=200
func AllLogs(c *gin.Context) {
	lines := parseLines(c, 200, 2000)
	entries := engine.DefaultCollector().AllProcesses(lines)
	if entries == nil {
		entries = []engine.LogEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}
