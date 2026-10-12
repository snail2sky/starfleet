package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/scheduler"
)

// ScheduleHandler handles schedule-related API requests.
type ScheduleHandler struct {
	scheduler *scheduler.Scheduler
	// persistEnabled writes the new enabled value back to the config file
	// that defines the schedule. Optional (nil = runtime only).
	persistEnabled func(name string, enabled bool) error
}

// NewScheduleHandler creates a new ScheduleHandler.
func NewScheduleHandler(sched *scheduler.Scheduler) *ScheduleHandler {
	return &ScheduleHandler{scheduler: sched}
}

// SetEnabledPersister injects the config-file persistence hook.
func (h *ScheduleHandler) SetEnabledPersister(fn func(name string, enabled bool) error) {
	h.persistEnabled = fn
}

// List returns all scheduled jobs.
func (h *ScheduleHandler) List(c *gin.Context) {
	jobs := h.scheduler.List()
	if jobs == nil {
		jobs = []scheduler.JobInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"data": jobs})
}

// Get returns a single scheduled job.
func (h *ScheduleHandler) Get(c *gin.Context) {
	name := c.Param("name")
	info, err := h.scheduler.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// setEnabled enables/disables a schedule and persists it to the config.
func (h *ScheduleHandler) setEnabled(c *gin.Context, enabled bool) {
	name := c.Param("name")
	if err := h.scheduler.SetEnabled(name, enabled); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if h.persistEnabled != nil {
		if err := h.persistEnabled(name, enabled); err != nil {
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
	c.JSON(http.StatusOK, gin.H{"message": name + " " + state, "enabled": enabled})
}

// Enable turns a schedule on.
func (h *ScheduleHandler) Enable(c *gin.Context) { h.setEnabled(c, true) }

// Disable turns a schedule off (definition kept, cron entry removed).
func (h *ScheduleHandler) Disable(c *gin.Context) { h.setEnabled(c, false) }

// Run triggers a scheduled job immediately.
func (h *ScheduleHandler) Run(c *gin.Context) {
	name := c.Param("name")
	if err := h.scheduler.Run(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "job triggered", "name": name})
}

// History returns execution history for a job.
func (h *ScheduleHandler) History(c *gin.Context) {
	name := c.Param("name")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	entries := h.scheduler.History(name, limit)
	if entries == nil {
		entries = []scheduler.HistoryEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}
