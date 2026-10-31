package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/scheduler"
)

// fakeManager 是 Manager 接口的内存实现（只覆盖 TUI 用到的读路径）。
type fakeManager struct {
	jobs     []scheduler.JobInfo
	hist     []scheduler.HistoryEntry
	ran      []string
	enabled  map[string]bool
	procs    []engine.ProcessInfo
}

func (f *fakeManager) List() []engine.ProcessInfo                    { return f.procs }
func (f *fakeManager) Get(name string) (*engine.ProcessInfo, error)  { return nil, nil }
func (f *fakeManager) Config(name string) (config.ProcessConfig, error) {
	return config.ProcessConfig{}, nil
}
func (f *fakeManager) Start(name string) error   { return nil }
func (f *fakeManager) Stop(name string) error    { return nil }
func (f *fakeManager) Restart(name string) error { return nil }
func (f *fakeManager) SetAutostart(name string, enabled bool) error { return nil }
func (f *fakeManager) SetAutorestart(name, value string) error      { return nil }
func (f *fakeManager) Logs(name string, lines int) []engine.LogEntry {
	return []engine.LogEntry{{Process: name, Stream: "stdout", Line: "log-line"}}
}
func (f *fakeManager) ServerLogs(lines int) []engine.LogEntry {
	return []engine.LogEntry{{Process: "server", Line: "server-line"}}
}
func (f *fakeManager) Schedules() []scheduler.JobInfo { return f.jobs }
func (f *fakeManager) RunSchedule(name string) error {
	f.ran = append(f.ran, name)
	return nil
}
func (f *fakeManager) SetScheduleEnabled(name string, enabled bool) error {
	if f.enabled == nil {
		f.enabled = map[string]bool{}
	}
	f.enabled[name] = enabled
	return nil
}
func (f *fakeManager) ScheduleHistory(name string, limit int) []scheduler.HistoryEntry {
	return f.hist
}
func (f *fakeManager) ScheduleDaily() scheduler.DailyReport { return scheduler.DailyReport{} }
func (f *fakeManager) Mode() string                         { return "local" }

func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
}

func schedModel(t *testing.T) (*Model, *fakeManager) {
	t.Helper()
	fm := &fakeManager{
		procs: []engine.ProcessInfo{{Name: "nginx", State: "RUNNING"}},
		jobs: []scheduler.JobInfo{
			{Name: "db-backup", Cron: "0 3 * * *", Type: "exec", Command: "/opt/backup.sh",
				Args: []string{"--full"}, Directory: "/opt", User: "deploy",
				Enabled: true, Timeout: "5m0s", NextRun: "2026-11-01 03:00:00",
				Labels: map[string]string{"group": "db"}},
			{Name: "ping", Cron: "@every 1m", Type: "http", URL: "http://x/health",
				Method: "HEAD", Enabled: false, Timeout: "10s"},
		},
		hist: []scheduler.HistoryEntry{
			{Name: "db-backup", StartTime: time.Now().Add(-time.Hour), Duration: 2 * time.Second,
				ExitCode: 0, Output: "backup ok\nsecond line"},
			{Name: "db-backup", StartTime: time.Now(), Duration: time.Second,
				ExitCode: 1, Error: "HTTP 503"},
		},
	}
	m := NewModel(fm)
	m.width, m.height = 120, 40
	m.tab = 1
	m.refreshSchedules()
	return &m, fm
}

// 两张表格必须用同一套选中样式（回归：schedules 曾用库默认的红色选中）。
func TestTableStylesShared(t *testing.T) {
	// 非 TTY 下 lipgloss 会退化成无色，强制 truecolor 才能断言颜色码。
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	a, b := TableStyles(), TableStyles()
	if !reflect.DeepEqual(a.Selected, b.Selected) {
		t.Errorf("TableStyles 两次调用不一致")
	}
	if !reflect.DeepEqual(a.Selected, table.DefaultStyles().Selected.Foreground(Primary).Background(BgCard)) {
		t.Errorf("选中样式应为 Primary 前景 + BgCard 背景")
	}
	rendered := a.Selected.Render("row")
	if !strings.Contains(rendered, "56;189;248") { // #38bdf8
		t.Errorf("选中前景不是 Primary(#38bdf8): %q", rendered)
	}
	if strings.Contains(rendered, "248;113;113") { // #f87171 红
		t.Errorf("选中样式不应是红色（库默认色）: %q", rendered)
	}
}

// ← 在任何子页面都应回到列表；在列表里才是切 tab。
func TestLeftArrowGoesBackFromSubViews(t *testing.T) {
	m, _ := schedModel(t)
	subViews := map[string]ViewMode{
		"detail": ViewDetail, "logs": ViewLogs,
		"history": ViewSchedHistory, "daily": ViewSchedDaily,
		"schedDetail": ViewSchedDetail,
	}
	for name, v := range subViews {
		mm := *m
		mm.viewMode = v
		mm.tab = 1
		next, _ := mm.Update(keyMsg("left"))
		got := next.(Model)
		if got.viewMode != ViewList {
			t.Errorf("%s 页按 ← 应回列表, got %d", name, got.viewMode)
		}
		if got.tab != 1 {
			t.Errorf("%s 页按 ← 不应改变 tab", name)
		}
	}
}

func TestLeftRightSwitchTabsOnlyInList(t *testing.T) {
	m, _ := schedModel(t)
	mm := *m
	mm.viewMode = ViewList
	mm.tab = 1
	next, _ := mm.Update(keyMsg("left"))
	if got := next.(Model); got.tab != 0 {
		t.Errorf("列表里 ← 应切到上一个 tab, got %d", got.tab)
	}
	mm = *m
	mm.viewMode = ViewList
	mm.tab = 0
	next, _ = mm.Update(keyMsg("right"))
	if got := next.(Model); got.tab != 1 {
		t.Errorf("列表里 → 应切到下一个 tab, got %d", got.tab)
	}
}

func TestSchedulesEnterOpensDetail(t *testing.T) {
	m, _ := schedModel(t)
	mm := *m
	mm.viewMode = ViewList
	mm.tab = 1
	mm.schedTable.SetCursor(0)
	next, _ := mm.Update(keyMsg("enter"))
	got := next.(Model)
	if got.viewMode != ViewSchedDetail {
		t.Fatalf("Enter 应进详情, got %d", got.viewMode)
	}
	if got.schedDetailName != "db-backup" {
		t.Errorf("详情任务=%q", got.schedDetailName)
	}
	out := got.viewSchedDetail()
	for _, want := range []string{
		"db-backup", "0 3 * * *", "/opt/backup.sh", "--full",
		"/opt", "deploy", "enabled", "group=db",
		"backup ok", "HTTP 503",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("详情缺少 %q\n%s", want, out)
		}
	}
}

func TestScheduleDetailHTTPShowsURL(t *testing.T) {
	m, _ := schedModel(t)
	mm := *m
	mm.schedDetail = m.manager.Schedules()[1]
	mm.schedDetailName = "ping"
	out := mm.viewSchedDetail()
	for _, want := range []string{"http://x/health", "HEAD", "disabled", "exec"} {
		if want == "exec" && strings.Contains(out, "command") {
			t.Errorf("http 任务不应显示 command 行")
		}
	}
	if !strings.Contains(out, "http://x/health") || !strings.Contains(out, "HEAD") {
		t.Errorf("http 详情缺少 url/method:\n%s", out)
	}
	if !strings.Contains(out, "disabled") {
		t.Errorf("停用状态未标红展示:\n%s", out)
	}
}

func TestSchedDetailKeys(t *testing.T) {
	m, fm := schedModel(t)
	mm := *m
	mm.viewMode = ViewSchedDetail
	mm.schedDetail = fm.jobs[0]
	mm.schedDetailName = "db-backup"

	// e 切换启用
	next, _ := mm.Update(keyMsg("e"))
	got := next.(Model)
	if fm.enabled["db-backup"] != false {
		t.Errorf("e 应把 enabled 置 false: %v", fm.enabled)
	}
	if got.schedDetail.Enabled {
		t.Errorf("详情里的 enabled 也应翻转")
	}
	// r 触发执行
	next, _ = got.Update(keyMsg("r"))
	if len(fm.ran) != 1 || fm.ran[0] != "db-backup" {
		t.Errorf("r 应触发执行: %v", fm.ran)
	}
	// h 看全部历史
	next, _ = next.Update(keyMsg("h"))
	if next.(Model).viewMode != ViewSchedHistory {
		t.Errorf("h 应进历史页")
	}
	// esc 返回
	mm2 := *m
	mm2.viewMode = ViewSchedDetail
	next, _ = mm2.Update(keyMsg("esc"))
	if next.(Model).viewMode != ViewList {
		t.Errorf("esc 应返回列表")
	}
}

// 子页面不该再有死代码 'q'（全局 q=quit 优先），确保返回键集合是 esc/backspace/←。
func TestSubViewsBackKeysAreReachable(t *testing.T) {
	m, _ := schedModel(t)
	for _, v := range []ViewMode{ViewDetail, ViewLogs, ViewSchedHistory, ViewSchedDaily, ViewSchedDetail} {
		mm := *m
		mm.viewMode = v
		next, _ := mm.Update(keyMsg("esc"))
		if next.(Model).viewMode != ViewList {
			t.Errorf("view %d 按 esc 未返回列表", v)
		}
	}
}

// lipgloss 在非 TTY 下不输出颜色码，渲染层至少要保证不 panic 且包含关键字段。
func TestViewSchedDetailRendersHistoryEmpty(t *testing.T) {
	m, _ := schedModel(t)
	mm := *m
	mm.schedDetailHist = nil
	mm.schedDetailName = "nothing"
	out := mm.viewSchedDetail()
	if !strings.Contains(out, "暂无执行记录") {
		t.Errorf("空历史应有提示:\n%s", out)
	}
	_ = lipgloss.NewStyle() // 保证样式包被引用（避免误删 import）
}
