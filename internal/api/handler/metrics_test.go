package handler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/scheduler"
)

func metricsEngine(procs func() []engine.ProcessInfo, scheds func() []scheduler.JobInfo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSystemHandler(nil)
	if procs != nil || scheds != nil {
		h.SetMetricsSources(procs, scheds)
	}
	r.GET("/api/v1/metrics", h.Metrics)
	return r
}

func getMetrics(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestMetricsSystemGauges(t *testing.T) {
	w := getMetrics(t, metricsEngine(nil, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type=%q 应为 Prometheus 文本格式", ct)
	}
	body := w.Body.String()
	for _, want := range []string{
		"# TYPE starfleet_memory_used_bytes gauge",
		"# TYPE starfleet_cpu_percent gauge",
		"starfleet_cpu_cores ",
		"starfleet_load1 ",
		"starfleet_disk_used_ratio ",
		"starfleet_uptime_seconds ",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("缺少 %q", want)
		}
	}
	// 每个样本行必须是 "metric [labels] value" 形式
	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Errorf("非法样本行: %q", line)
			continue
		}
		if _, err := strconv.ParseFloat(fields[1], 64); err != nil {
			t.Errorf("样本值不可解析: %q", line)
		}
	}
}

func TestMetricsProcessAndScheduleSamples(t *testing.T) {
	procs := func() []engine.ProcessInfo {
		return []engine.ProcessInfo{
			{Name: "nginx", State: "RUNNING", RetryCount: 0},
			{Name: "sshd", State: "STOPPED", RetryCount: 2},
		}
	}
	scheds := func() []scheduler.JobInfo {
		return []scheduler.JobInfo{
			{Name: "backup", Enabled: true},
			{Name: "report", Enabled: false},
		}
	}
	body := getMetrics(t, metricsEngine(procs, scheds)).Body.String()
	for _, want := range []string{
		`starfleet_process_state{name="nginx",state="RUNNING"} 1`,
		`starfleet_process_up{name="nginx"} 1`,
		`starfleet_process_up{name="sshd"} 0`,
		`starfleet_process_retries{name="sshd"} 2`,
		`starfleet_schedule_enabled{name="backup"} 1`,
		`starfleet_schedule_enabled{name="report"} 0`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("缺少样本 %q\n%s", want, body)
		}
	}
}

func TestMetricsWithoutSourcesOmitsProcessFamily(t *testing.T) {
	body := getMetrics(t, metricsEngine(nil, nil)).Body.String()
	if strings.Contains(body, "starfleet_process_") || strings.Contains(body, "starfleet_schedule_") {
		t.Errorf("未注入指标源时不应有进程/调度指标")
	}
}
