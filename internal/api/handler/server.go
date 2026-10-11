package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/selflog"
)

// ServerLogs returns starfleet's own recent log lines in the same
// LogEntry shape as process logs (process="starfleet", stream=level).
// GET /api/v1/server/logs?lines=200
func ServerLogs(c *gin.Context) {
	lines := parseLines(c, 200, 2000)
	entries := selflog.Default().AsLogEntries(lines)
	if entries == nil {
		entries = []engine.LogEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}
