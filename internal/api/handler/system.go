package handler

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/snail2sky/starfleet/internal/api/middleware"
)

// Version is the build version reported by HealthDetailed; set from main
// via SetVersion (ldflags carry the real value).
var Version = "dev"

// SetVersion records the build version for API responses.
func SetVersion(v string) {
	if v != "" {
		Version = v
	}
}

// SystemHandler handles system information API requests.
type SystemHandler struct {
	auth *middleware.AuthState
}

// NewSystemHandler creates a new SystemHandler. The live auth state is
// reported by Health so unauthenticated clients (the WebUI) know whether
// to show login — including after a config reload toggles auth.
func NewSystemHandler(auth *middleware.AuthState) *SystemHandler {
	return &SystemHandler{auth: auth}
}

// Overview returns system resource overview.
func (h *SystemHandler) Overview(c *gin.Context) {
	vm, _ := mem.VirtualMemory()
	diskUsage, _ := disk.Usage("/")
	cpuPercent, _ := cpu.Percent(0, false)
	loadAvg, _ := load.Avg()
	hostInfo, _ := host.Info()

	data := gin.H{
		"host": gin.H{
			"hostname":   hostInfo.Hostname,
			"os":         hostInfo.OS,
			"platform":   hostInfo.Platform,
			"kernel":     hostInfo.KernelVersion,
			"uptime":     hostInfo.Uptime,
			"go_version": runtime.Version(),
		},
		"cpu": gin.H{
			"cores":   runtime.NumCPU(),
			"percent": cpuPercent,
		},
		"memory": gin.H{
			"total":     vm.Total,
			"used":      vm.Used,
			"available": vm.Available,
			"percent":   vm.UsedPercent,
		},
		"disk": gin.H{
			"total":   diskUsage.Total,
			"used":    diskUsage.Used,
			"free":    diskUsage.Free,
			"percent": diskUsage.UsedPercent,
		},
		"load": gin.H{
			"load1":  loadAvg.Load1,
			"load5":  loadAvg.Load5,
			"load15": loadAvg.Load15,
		},
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Health returns a simple health check response. Mounted unauthenticated
// at /health — deliberately WITHOUT the version to avoid fingerprinting
// the build for anonymous visitors.
func (h *SystemHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "auth": h.auth.Load().Enabled})
}

// HealthDetailed is mounted behind auth at /api/v1/system/health and adds
// the build version for the WebUI/TUI.
func (h *SystemHandler) HealthDetailed(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "auth": h.auth.Load().Enabled, "version": Version})
}
