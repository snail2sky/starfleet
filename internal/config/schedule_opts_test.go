package config

import (
	"testing"
	"time"
)

// 调度任务的新字段（environment/directory/args/user）必须真正被解析——
// 在 v1.2.x 它们被 yaml 静默忽略，导致配置看起来生效实际没有。
func TestParseScheduleOptions(t *testing.T) {
	yaml := `
processes: {}
schedules:
  dynu-update:
    enabled: true
    cron: "*/5 * * * *"
    command: /data0/apps/dynu/dynu
    args: ["update", "--json"]
    directory: /data0/apps/dynu
    user: orangepi
    timeout: "120s"
    environment:
      ALL_PROXY: "http://127.0.0.1:1081"
      SECRET_TOKEN: "abc"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s, ok := cfg.Schedules["dynu-update"]
	if !ok {
		t.Fatal("缺少 dynu-update")
	}
	if s.Environment["ALL_PROXY"] != "http://127.0.0.1:1081" || s.Environment["SECRET_TOKEN"] != "abc" {
		t.Errorf("environment 未解析: %#v", s.Environment)
	}
	if s.Directory != "/data0/apps/dynu" {
		t.Errorf("directory=%q", s.Directory)
	}
	if s.User != "orangepi" {
		t.Errorf("user=%q", s.User)
	}
	if len(s.Args) != 2 || s.Args[0] != "update" || s.Args[1] != "--json" {
		t.Errorf("args=%#v", s.Args)
	}
	if s.Timeout != 120*time.Second {
		t.Errorf("timeout=%v", s.Timeout)
	}
	if !s.Enabled || s.Cron != "*/5 * * * *" {
		t.Errorf("enabled/cron: %#v", s)
	}
}

// 旧写法（只有 cron/command）必须继续可用——回归保护。
func TestParseScheduleMinimalBackCompat(t *testing.T) {
	yaml := `
processes: {}
schedules:
  backup:
    cron: "0 3 * * *"
    command: "/opt/backup.sh"
    timeout: 300s
    enabled: true
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s := cfg.Schedules["backup"]
	if s.Command != "/opt/backup.sh" || s.Timeout != 300*time.Second || !s.Enabled {
		t.Errorf("旧写法解析破坏: %#v", s)
	}
	if s.Environment != nil || s.Directory != "" || s.User != "" || s.Args != nil {
		t.Errorf("未配置的新字段应为零值: %#v", s)
	}
}
