package handler

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/load"
)

// SystemHandler handles system information API requests.
type SystemHandler struct{}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
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
			"hostname":  hostInfo.Hostname,
			"os":        hostInfo.OS,
			"platform":  hostInfo.Platform,
			"kernel":    hostInfo.KernelVersion,
			"uptime":    hostInfo.Uptime,
			"go_version": runtime.Version(),
		},
		"cpu": gin.H{
			"cores":  runtime.NumCPU(),
			"percent": cpuPercent,
		},
		"memory": gin.H{
			"total":     vm.Total,
			"used":      vm.Used,
			"available":  vm.Available,
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

// Health returns a simple health check response.
func (h *SystemHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}