package handler

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/snail2sky/starfleet/internal/api/middleware"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/scheduler"
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
	// 可选指标源（/metrics 用）：进程与调度状态。
	processes func() []engine.ProcessInfo
	schedules func() []scheduler.JobInfo
}

// SetMetricsSources 注入进程/调度指标源（留空则 /metrics 只含系统指标）。
func (h *SystemHandler) SetMetricsSources(procs func() []engine.ProcessInfo, scheds func() []scheduler.JobInfo) {
	h.processes = procs
	h.schedules = scheds
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

// Metrics exposes Prometheus text-format metrics: system resources plus
// per-process state and per-schedule enablement (when the sources are wired).
func (h *SystemHandler) Metrics(c *gin.Context) {
	var b strings.Builder
	metric := func(name, help, typ string, samples ...string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
		for _, s := range samples {
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}
	gauge := func(name, help string, samples ...string) { metric(name, help, "gauge", samples...) }

	if vm, err := mem.VirtualMemory(); err == nil {
		gauge("starfleet_memory_used_bytes", "Physical memory used",
			fmt.Sprintf("starfleet_memory_used_bytes %d", vm.Used))
		gauge("starfleet_memory_total_bytes", "Physical memory total",
			fmt.Sprintf("starfleet_memory_total_bytes %d", vm.Total))
		gauge("starfleet_memory_used_ratio", "Physical memory used ratio (0-1)",
			fmt.Sprintf("starfleet_memory_used_ratio %.4f", vm.UsedPercent/100))
	}
	if cpuPct, err := cpu.Percent(0, false); err == nil && len(cpuPct) > 0 {
		gauge("starfleet_cpu_percent", "CPU usage percent",
			fmt.Sprintf("starfleet_cpu_percent %.2f", cpuPct[0]))
	}
	gauge("starfleet_cpu_cores", "Number of logical CPUs",
		fmt.Sprintf("starfleet_cpu_cores %d", runtime.NumCPU()))
	if du, err := disk.Usage("/"); err == nil {
		gauge("starfleet_disk_used_ratio", "Root filesystem used ratio (0-1)",
			fmt.Sprintf("starfleet_disk_used_ratio %.4f", du.UsedPercent/100))
		gauge("starfleet_disk_used_bytes", "Root filesystem used bytes",
			fmt.Sprintf("starfleet_disk_used_bytes %d", du.Used))
	}
	if la, err := load.Avg(); err == nil {
		gauge("starfleet_load1", "Load average (1 min)", fmt.Sprintf("starfleet_load1 %.2f", la.Load1))
		gauge("starfleet_load5", "Load average (5 min)", fmt.Sprintf("starfleet_load5 %.2f", la.Load5))
		gauge("starfleet_load15", "Load average (15 min)", fmt.Sprintf("starfleet_load15 %.2f", la.Load15))
	}
	if hi, err := host.Info(); err == nil {
		gauge("starfleet_uptime_seconds", "Host uptime in seconds",
			fmt.Sprintf("starfleet_uptime_seconds %d", hi.Uptime))
	}

	if h.processes != nil {
		var stateSamples, upSamples, retrySamples []string
		for _, p := range h.processes() {
			name := p.Name
			stateSamples = append(stateSamples,
				fmt.Sprintf("starfleet_process_state{name=%q,state=%q} 1", name, p.State))
			up := 0
			if p.State == "RUNNING" {
				up = 1
			}
			upSamples = append(upSamples, fmt.Sprintf("starfleet_process_up{name=%q} %d", name, up))
			retrySamples = append(retrySamples, fmt.Sprintf("starfleet_process_retries{name=%q} %d", name, p.RetryCount))
		}
		if len(stateSamples) > 0 {
			gauge("starfleet_process_state", "Process state (1 for the current state)", stateSamples...)
			gauge("starfleet_process_up", "1 when the process is RUNNING", upSamples...)
			gauge("starfleet_process_retries", "Current retry count", retrySamples...)
		}
	}
	if h.schedules != nil {
		var enabledSamples []string
		for _, s := range h.schedules() {
			enabled := 0
			if s.Enabled {
				enabled = 1
			}
			enabledSamples = append(enabledSamples, fmt.Sprintf("starfleet_schedule_enabled{name=%q} %d", s.Name, enabled))
		}
		if len(enabledSamples) > 0 {
			gauge("starfleet_schedule_enabled", "1 when the schedule is enabled", enabledSamples...)
		}
	}

	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(b.String()))
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
