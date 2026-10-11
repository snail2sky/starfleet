package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/snail-sky/starfleet/internal/engine"
)

// ProcessHandler handles process-related API requests.
type ProcessHandler struct {
	manager *engine.Manager
}

// NewProcessHandler creates a new ProcessHandler.
func NewProcessHandler(mgr *engine.Manager) *ProcessHandler {
	return &ProcessHandler{manager: mgr}
}

// List returns all processes.
func (h *ProcessHandler) List(c *gin.Context) {
	processes := h.manager.List()
	if processes == nil {
		processes = []engine.ProcessInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"data": processes})
}

// Get returns a single process by name.
func (h *ProcessHandler) Get(c *gin.Context) {
	name := c.Param("name")
	info, err := h.manager.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
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