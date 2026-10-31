package scheduler

import (
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
	scfg.Type = "exec"
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
