package scheduler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/event"
)

// runJob 是公共小工具：装载配置 → 手动触发 → 等历史。
func runJob(t *testing.T, name string, scfg config.ScheduleConfig) HistoryEntry {
	t.Helper()
	bus := event.NewBus(4)
	t.Cleanup(bus.Close)
	s := NewScheduler(bus)

	cfg := config.DefaultConfig()
	if scfg.Type == "" {
		scfg.Type = "exec"
	}
	scfg.Enabled = true
	cfg.Schedules[name] = scfg
	if err := s.LoadFromConfig(cfg); err != nil {
		t.Fatalf("load: %v", err)
	}
	s.Start()
	t.Cleanup(func() { s.Stop() })

	if err := s.Run(name); err != nil {
		t.Fatalf("run: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hist := s.History(name, 10); len(hist) > 0 {
			return hist[0]
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for job history entry")
	return HistoryEntry{}
}

func TestScheduleEnvironmentInjected(t *testing.T) {
	out := filepath.Join(t.TempDir(), "env.txt")
	got := runJob(t, "env", config.ScheduleConfig{
		Cron:        "@every 24h",
		Command:     "echo $STARFLEET_TEST_VAR > " + out,
		Environment: map[string]string{"STARFLEET_TEST_VAR": "injected-value"},
	})
	if got.Error != "" {
		t.Fatalf("job failed: %s", got.Error)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("env file: %v", err)
	}
	if !strings.Contains(string(data), "injected-value") {
		t.Errorf("环境变量未注入: %q", string(data))
	}
}

func TestScheduleEnvironmentDoesNotWipeInherited(t *testing.T) {
	// 继承的 PATH 必须仍在（environment 是叠加而非替换）。
	got := runJob(t, "inherit", config.ScheduleConfig{
		Cron:        "@every 24h",
		Command:     `echo "PATH_SET=${PATH:+yes}"`,
		Environment: map[string]string{"EXTRA_KEY": "1"},
	})
	if got.Error != "" {
		t.Fatalf("job failed: %s", got.Error)
	}
	if !strings.Contains(got.Output, "PATH_SET=yes") {
		t.Errorf("继承环境丢失: %q", got.Output)
	}
}

func TestScheduleDirectory(t *testing.T) {
	dir := t.TempDir()
	got := runJob(t, "cwd", config.ScheduleConfig{
		Cron:       "@every 24h",
		Command:    "pwd",
		Directory:  dir,
	})
	if got.Error != "" {
		t.Fatalf("job failed: %s", got.Error)
	}
	if !strings.Contains(got.Output, filepath.Base(dir)) {
		t.Errorf("工作目录未生效: output=%q want含 %q", got.Output, filepath.Base(dir))
	}
}

func TestScheduleArgsExecDirectly(t *testing.T) {
	// 有 args 时不经 shell：echo 的参数被原样传递（$VAR 不会被展开）。
	got := runJob(t, "args", config.ScheduleConfig{
		Cron:    "@every 24h",
		Command: "echo",
		Args:    []string{"hello", "$STARFLEET_UNEXPANDED"},
	})
	if got.Error != "" {
		t.Fatalf("job failed: %s", got.Error)
	}
	out := strings.TrimSpace(got.Output)
	if out != "hello $STARFLEET_UNEXPANDED" {
		t.Errorf("args 直 exec 应不展开变量: %q", out)
	}
}

func TestScheduleUserUnknownFails(t *testing.T) {
	got := runJob(t, "baduser", config.ScheduleConfig{
		Cron:    "@every 24h",
		Command: "echo hi",
		User:    "starfleet-no-such-user",
	})
	if got.Error == "" || !strings.Contains(got.Error, "starfleet-no-such-user") {
		t.Errorf("未知用户应报错且带用户名: %q", got.Error)
	}
}

func TestScheduleUserCurrentSucceeds(t *testing.T) {
	// 以当前用户运行（engine 的 no-op 分支），任务应正常成功。
	me := currentUsername(t)
	got := runJob(t, "sameuser", config.ScheduleConfig{
		Cron:    "@every 24h",
		Command: "echo ok-as-user",
		User:    me,
	})
	if got.Error != "" {
		t.Fatalf("以当前用户运行应成功: %s", got.Error)
	}
	if !strings.Contains(got.Output, "ok-as-user") {
		t.Errorf("output=%q", got.Output)
	}
}

func currentUsername(t *testing.T) string {
	t.Helper()
	// 直接用环境变量，避免再引 os/user（跨平台测试友好）
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("USERNAME"); u != "" {
		return u
	}
	return "root"
}

func TestScheduleTimeoutKillsGroup(t *testing.T) {
	// 超时后必须留不下 sleep 孤儿：命令 sleep 60，超时1s。
	got := runJob(t, "timeout", config.ScheduleConfig{
		Cron:    "@every 24h",
		Command: "sleep 60",
		Timeout: time.Second,
	})
	if got.Error == "" {
		t.Errorf("超时应报错")
	}
	if got.ExitCode == 0 {
		t.Errorf("超时 ExitCode 不应为0: %d", got.ExitCode)
	}
}

func TestScheduleHTTPSuccess(t *testing.T) {
	type received struct{ method, header, body, ctype string }
	got := make(chan received, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- received{r.Method, r.Header.Get("X-Token"), string(b), r.Header.Get("Content-Type")}
		w.WriteHeader(200)
		w.Write([]byte("pong"))
	}))
	defer ts.Close()

	entry := runJob(t, "http-ok", config.ScheduleConfig{
		Cron:    "@every 24h",
		Type:    "http",
		URL:     ts.URL + "/ping",
		Method:  "POST",
		Headers: map[string]string{"X-Token": "s3cret"},
		Body:    `{"hello":"world"}`,
	})
	if entry.Error != "" {
		t.Fatalf("http 任务失败: %s", entry.Error)
	}
	if entry.ExitCode != 0 {
		t.Errorf("ExitCode=%d", entry.ExitCode)
	}
	if !strings.Contains(entry.Output, "HTTP 200") || !strings.Contains(entry.Output, "pong") {
		t.Errorf("output=%q", entry.Output)
	}
	r := <-got
	if r.method != "POST" || r.header != "s3cret" || r.body != `{"hello":"world"}` {
		t.Errorf("服务端收到: %+v", r)
	}
	if r.ctype != "application/json" {
		t.Errorf("Body 存在时应默认 Content-Type: application/json, got %q", r.ctype)
	}
}

func TestScheduleHTTPGetDefault(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("默认方法应为 GET, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("无 body 不应设 Content-Type")
		}
		w.WriteHeader(204)
	}))
	defer ts.Close()
	entry := runJob(t, "http-get", config.ScheduleConfig{Cron: "@every 24h", Type: "http", URL: ts.URL})
	if entry.Error != "" || entry.ExitCode != 0 {
		t.Errorf("GET 默认方法失败: %+v", entry)
	}
}

func TestScheduleHTTPServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte("busy"))
	}))
	defer ts.Close()
	entry := runJob(t, "http-503", config.ScheduleConfig{Cron: "@every 24h", Type: "http", URL: ts.URL})
	if entry.Error != "HTTP 503" {
		t.Errorf("错误信息=%q want 'HTTP 503'", entry.Error)
	}
	if entry.ExitCode != 1 {
		t.Errorf("ExitCode=%d want 1", entry.ExitCode)
	}
	if !strings.Contains(entry.Output, "busy") {
		t.Errorf("output 应含响应体: %q", entry.Output)
	}
}

func TestScheduleHTTPTransportError(t *testing.T) {
	entry := runJob(t, "http-dead", config.ScheduleConfig{
		Cron: "@every 24h", Type: "http",
		URL: "http://127.0.0.1:1", // 无服务
	})
	if entry.Error == "" || entry.ExitCode != -1 {
		t.Errorf("传输错误应 -1: %+v", entry)
	}
}

func TestScheduleHTTPMissingURL(t *testing.T) {
	entry := runJob(t, "http-nourl", config.ScheduleConfig{Cron: "@every 24h", Type: "http"})
	if entry.Error == "" || !strings.Contains(entry.Error, "url") {
		t.Errorf("缺 url 应明确报错: %+v", entry)
	}
}
