package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/snail-sky/starfleet/internal/scheduler"
)

// ScheduleHandler handles schedule-related API requests.
type ScheduleHandler struct {
	scheduler *scheduler.Scheduler
}

// NewScheduleHandler creates a new ScheduleHandler.
func NewScheduleHandler(sched *scheduler.Scheduler) *ScheduleHandler {
	return &ScheduleHandler{scheduler: sched}
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