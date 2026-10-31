package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// 开关类配置：单元格是纯文本（对齐保证），颜色在 View 阶段注入非选中行。
func TestToggleColorsAppliedInView(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	if OnOff(true) != "on" || OnOff(false) != "off" {
		t.Errorf("单元格必须是纯文本: %q %q", OnOff(true), OnOff(false))
	}
	if RestartPolicy("true") != "always" || RestartPolicy("unexpected") != "unexpected" || RestartPolicy("false") != "never" {
		t.Errorf("策略必须是语义化纯文本: %q %q %q",
			RestartPolicy("true"), RestartPolicy("unexpected"), RestartPolicy("false"))
	}
	fm := &fakeManager{
		procs: []engine.ProcessInfo{
			{Name: "app", State: "RUNNING", Autostart: true, Autorestart: "unexpected", Command: "run-app"},
			{Name: "down", State: "STOPPED", Autostart: false, Autorestart: "false", Command: "run-x"},
		},
		jobs: []scheduler.JobInfo{
			{Name: "j-on", Cron: "* * * * *", Enabled: true, Command: "c"},
			{Name: "j-off", Cron: "* * * * *", Enabled: false, Command: "c"},
		},
	}
	m := NewModel(fm)
	m.width, m.height = 160, 40
	m.resizeTables()
	m.refreshData()
	m.refreshSchedules()

	// 光标停在第0行 → 第0行保持统一（无内嵌色），第1行的单元格有色
	procView := colorizeTableView(m.procTable.View(), m.procTable.Columns(), TableStyles())
	if strings.Contains(strings.Split(procView, "\n")[1], "\x1b[38;2;") {
		t.Errorf("选中行不应内嵌单元格色:\n%s", strings.Split(procView, "\n")[1])
	}
	unsel := strings.Split(procView, "\n")[2]
	if !strings.Contains(unsel, "\x1b[38;2;") {
		t.Errorf("非选中行的 on/状态 应着色:\n%q", unsel)
	}
	schedView := colorizeTableView(m.schedTable.View(), m.schedTable.Columns(), TableStyles())
	if !strings.Contains(schedView, "\x1b[38;2;") {
		t.Errorf("schedules 非选中行也应着色:\n%s", schedView)
	}
}

// COMMAND 列要吃掉剩余屏幕宽度；其他列保持基准宽度。
func TestFitColumnsFlexesCommand(t *testing.T) {
	tbl := table.New(table.WithColumns(baseProcCols))
	// 120 列宽的屏幕：固定列共 (18+10+6+7+12+8+12)+7*2 = 87，flex ≈ 120-87-2
	flex := fitColumns(&tbl, baseProcCols, len(baseProcCols)-1, 120)
	cols := tbl.Columns()
	if cols[len(cols)-1].Width != flex {
		t.Errorf("末列宽度=%d want %d", cols[len(cols)-1].Width, flex)
	}
	if flex <= 30 {
		t.Errorf("120 宽下 COMMAND 应远大于30, got %d", flex)
	}
	for i := 0; i < len(cols)-1; i++ {
		if cols[i].Width != baseProcCols[i].Width {
			t.Errorf("固定列 %d 宽度被改动: %d", i, cols[i].Width)
		}
	}
	// 极窄屏幕：夹到下限30
	small := table.New(table.WithColumns(baseProcCols))
	if got := fitColumns(&small, baseProcCols, len(baseProcCols)-1, 40); got != 30 {
		t.Errorf("窄屏应夹到30, got %d", got)
	}
}

// 行数据：开关着色 + command 按动态宽度截断（而不是写死28）。
func TestRowsUseDynamicCmdWidthAndColoredToggles(t *testing.T) {
	long := "/usr/bin/someverylongcommand --with-a-lot-of-arguments --flag=1 --flag=2 --flag=3"
	fm := &fakeManager{
		procs: []engine.ProcessInfo{{Name: "app", State: "RUNNING", Autostart: true,
			Autorestart: "unexpected", Command: long}},
		jobs: []scheduler.JobInfo{{Name: "job1", Cron: "* * * * *", Enabled: true, Command: long}},
	}
	m := NewModel(fm)
	m.width, m.height = 200, 40
	m.resizeTables()

	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m.refreshData()
	row := m.procTable.SelectedRow()
	if len(row) == 0 {
		t.Fatal("无行")
	}
	if row[7] != long && len([]rune(row[7])) < 60 {
		t.Errorf("宽屏下 command 应保留更多: %q", row[7])
	}
	if row[2] != "on" {
		t.Errorf("BOOT 应为纯文本 on: %q", row[2])
	}
	if row[6] != "unexpected" {
		t.Errorf("AUTORESTART 应显示策略名: %q", row[6])
	}

	// 窄屏：截断生效
	m.width = 100
	m.resizeTables()
	narrow := m.cmdWidth
	m.refreshData()
	if len([]rune(m.procTable.SelectedRow()[7])) > narrow {
		t.Errorf("窄屏应按 cmdWidth=%d 截断: %q", narrow, m.procTable.SelectedRow()[7])
	}
}

// 窗口尺寸变化应触发表格重排与行刷新。
func TestWindowSizeTriggersRefit(t *testing.T) {
	fm := &fakeManager{
		procs: []engine.ProcessInfo{{Name: "app", State: "RUNNING"}},
		jobs:  []scheduler.JobInfo{},
	}
	m := NewModel(fm)
	m.width, m.height = 80, 24
	next, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	got := next.(Model)
	if got.width != 180 || got.height != 50 {
		t.Errorf("尺寸未记录: %dx%d", got.width, got.height)
	}
	if got.cmdWidth <= 30 {
		t.Errorf("180 宽下 cmdWidth 应 >30, got %d", got.cmdWidth)
	}
	lastCol := got.procTable.Columns()[len(got.procTable.Columns())-1]
	if lastCol.Width != got.cmdWidth {
		t.Errorf("表格末列宽=%d 与 cmdWidth=%d 不一致", lastCol.Width, got.cmdWidth)
	}
}

// 对齐回归：着色后的视图，每行的“显示宽度”必须完全一致
// （列错位的根因是 v1.0.0 用 runewidth.Truncate 计算含 ANSI 的单元格宽度）。
func TestColorizedViewRowsAligned(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	long := "/usr/bin/very-long-binary --flag-one --flag-two --flag-three --flag-four=1234567890 --tail"
	fm := &fakeManager{
		procs: []engine.ProcessInfo{
			{Name: "中文进程名", State: "RUNNING", Autostart: true, Autorestart: "unexpected", Command: long},
			{Name: "app2", State: "BACKOFF", Autostart: false, Autorestart: "never", Command: "x"},
		},
		jobs: []scheduler.JobInfo{
			{Name: "job1", Cron: "*/5 * * * *", Enabled: true, Command: long},
			{Name: "定时任务二", Cron: "@every 1m", Enabled: false, URL: "http://example.com/very/long/path"},
		},
	}
	m := NewModel(fm)
	m.width, m.height = 140, 40
	m.resizeTables()
	m.refreshData()
	m.refreshSchedules()

	for name, view := range map[string]string{
		"proc":  colorizeTableView(m.procTable.View(), m.procTable.Columns(), TableStyles()),
		"sched": colorizeTableView(m.schedTable.View(), m.schedTable.Columns(), TableStyles()),
	} {
		lines := strings.Split(view, "\n")
		var width int
		var seen bool
		for _, ln := range lines {
			if strings.TrimSpace(ansi.Strip(ln)) == "" {
				continue
			}
			w := ansi.StringWidth(ln)
			if !seen {
				width, seen = w, true
				continue
			}
			if w != width {
				t.Errorf("%s 视图行宽不一致: %d vs %d\n行: %q", name, width, w, ln)
			}
		}
	}
}

// 对齐不变量（本次 bug 的直接回归锁）：着色后的表格，每一行的可见宽度
// 必须完全一致。v1.7.0 曾因在单元格里放 ANSI + ●/○ 模糊宽度字符而错位。
func TestColoredViewKeepsAlignment(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// 混排：中文名（多字节）、长命令（触发截断）、各种开关值
	fm := &fakeManager{
		procs: []engine.ProcessInfo{
			{Name: "nginx", State: "RUNNING", Autostart: true, Autorestart: "true", Command: "nginx: master process /usr/sbin/nginx -g daemon off;"},
			{Name: "中文服务名", State: "BACKOFF", Autostart: false, Autorestart: "unexpected", Command: "python3 /opt/服务/启动.py --config /etc/服务/config.yaml --verbose"},
			{Name: "tiny", State: "STOPPED", Autostart: false, Autorestart: "false", Command: "true"},
		},
		jobs: []scheduler.JobInfo{
			{Name: "backup", Cron: "0 3 * * *", Enabled: true, Command: "/opt/backup.sh --all --compress"},
			{Name: "探活", Cron: "@every 1m", Enabled: false, URL: "http://192.168.1.1:8080/healthz/long/path"},
		},
	}
	m := NewModel(fm)
	m.width, m.height = 140, 40
	m.resizeTables()
	m.refreshData()
	m.refreshSchedules()

	checkAlignment := func(name, view string) {
		t.Helper()
		var want int
		var bad []string
		for _, ln := range strings.Split(view, "\n") {
			if strings.TrimSpace(ansi.Strip(ln)) == "" {
				continue
			}
			w := ansi.StringWidth(ln)
			if want == 0 {
				want = w
				continue
			}
			if w != want {
				bad = append(bad, ln)
			}
		}
		if len(bad) > 0 {
			t.Errorf("%s 行宽不一致（期望 %d），错位行:\n%s", name, want, strings.Join(bad, "\n"))
		}
	}
	checkAlignment("processes", colorizeTableView(m.procTable.View(), m.procTable.Columns(), TableStyles()))
	checkAlignment("schedules", colorizeTableView(m.schedTable.View(), m.schedTable.Columns(), TableStyles()))

	// 各种光标位置都不应破坏对齐
	for c := 0; c < 3; c++ {
		m.procTable.SetCursor(c)
		checkAlignment(fmt.Sprintf("processes/cursor=%d", c),
			colorizeTableView(m.procTable.View(), m.procTable.Columns(), TableStyles()))
	}
}
