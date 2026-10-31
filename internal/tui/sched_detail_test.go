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

// 开关类配置必须带颜色（on 绿 / off 红），比裸 true/false 直观。
func TestToggleStylesHaveColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	on := OnOff(true)
	off := OnOff(false)
	if !strings.Contains(on, "on") || !strings.Contains(off, "off") {
		t.Errorf("文本: %q / %q", on, off)
	}
	if on == off {
		t.Errorf("开与关必须可区分")
	}
	// 真彩下应有 SGR 颜色码
	if !strings.Contains(on, "\x1b[") {
		t.Errorf("on 应带颜色码: %q", on)
	}
	always, unexpected, never := RestartPolicy("true"), RestartPolicy("unexpected"), RestartPolicy("false")
	if !strings.Contains(always, "always") || !strings.Contains(unexpected, "unexpected") || !strings.Contains(never, "never") {
		t.Errorf("策略文案: %q %q %q", always, unexpected, never)
	}
	if len(map[string]bool{always: true, unexpected: true, never: true}) != 3 {
		t.Errorf("三种策略必须可区分")
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
	if !strings.Contains(row[2], "on") || !strings.Contains(row[2], "\x1b[") {
		t.Errorf("BOOT 应为着色的 on: %q", row[2])
	}
	if !strings.Contains(row[6], "unexpected") {
		t.Errorf("AUTORESTART 应显示策略名: %q", row[6])
	}

	m.refreshSchedules()
	srow := m.schedTable.SelectedRow()
	if !strings.Contains(srow[3], "\x1b[") {
		t.Errorf("ENABLED 应着色: %q", srow[3])
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
