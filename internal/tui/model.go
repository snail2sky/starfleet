// Package tui provides a terminal user interface for starfleet.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/snail2sky/starfleet/internal/engine"
)

// View modes.
type ViewMode int

const (
	ViewList ViewMode = iota
	ViewDetail
	ViewLogs
)

// Model is the main TUI model.
type Model struct {
	manager  Manager
	width    int
	height   int
	viewMode ViewMode
	tab      int // 0=processes, 1=schedules, 2=help

	// Process list
	procTable table.Model
	lastRows  []table.Row // cache: skip re-render when unchanged

	// Schedules list
	lastSchedRows []table.Row

	// Detail view
	detailName  string
	detailLines []string

	// Log view
	logName    string
	logServer  bool // viewing starfleet's own logs instead of a process
	logEntries []engine.LogEntry
	logScroll  int
	logLoaded  bool // true once first fetch completed for current process

	// Schedule table
	schedTable table.Model

	// Common
	spinner  spinner.Model
	message  string
	msgTimer int
}

var tabs = []string{"Processes", "Schedules", "Help"}

// procCols defines the process table columns.
// BOOT = starts automatically when starfleet starts (autostart);
// AUTORESTART = restart policy when the process exits.
var procCols = []table.Column{
	{Title: "NAME", Width: 18},
	{Title: "STATE", Width: 10},
	{Title: "BOOT", Width: 6},
	{Title: "PID", Width: 7},
	{Title: "UPTIME", Width: 12},
	{Title: "RETRIES", Width: 8},
	{Title: "AUTORESTART", Width: 12},
	{Title: "COMMAND", Width: 30},
}

// NewModel creates a new TUI model.
func NewModel(manager Manager) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Primary)

	// Process table with rich columns.
	pt := table.New(
		table.WithColumns(procCols),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	ts := table.DefaultStyles()
	ts.Header = ts.Header.Bold(true).Foreground(Primary)
	ts.Selected = ts.Selected.Foreground(Primary).Background(lipgloss.Color("#1e293b"))
	pt.SetStyles(ts)

	// Schedule table.
	schedCols := []table.Column{
		{Title: "NAME", Width: 18},
		{Title: "CRON", Width: 14},
		{Title: "TYPE", Width: 6},
		{Title: "ENABLED", Width: 8},
		{Title: "NEXT RUN", Width: 20},
		{Title: "TIMEOUT", Width: 10},
		{Title: "COMMAND / URL", Width: 30},
	}
	st := table.New(
		table.WithColumns(schedCols),
		table.WithHeight(10),
	)

	return Model{
		manager:    manager,
		viewMode:   ViewList,
		procTable:  pt,
		schedTable: st,
		spinner:    s,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tick())
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type tickMsg time.Time

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		tableH := m.height - 12
		if tableH < 5 {
			tableH = 5
		}
		m.procTable.SetHeight(tableH)
		m.schedTable.SetHeight(tableH)
		return m, nil

	case tickMsg:
		m.refreshData()
		// Auto-refresh the log view every tick (1s) — both server and
		// process sources (fetchLogs picks the right one).
		if m.viewMode == ViewLogs {
			m.refreshLogs()
		}
		if m.msgTimer > 0 {
			m.msgTimer--
			if m.msgTimer == 0 {
				m.message = ""
			}
		}
		return m, m.tick()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Update sub-components.
	switch m.viewMode {
	case ViewList:
		if m.tab == 0 {
			var cmd tea.Cmd
			m.procTable, cmd = m.procTable.Update(msg)
			cmds = append(cmds, cmd)
		} else if m.tab == 1 {
			var cmd tea.Cmd
			m.schedTable, cmd = m.schedTable.Update(msg)
			cmds = append(cmds, cmd)
		}
	case ViewLogs:
		// Scroll handled in handleKey
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys.
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
		return m, tea.Quit

	case key.Matches(msg, key.NewBinding(key.WithKeys("1"))):
		m.tab = 0
		m.viewMode = ViewList
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
		m.tab = 1
		m.viewMode = ViewList
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("3"))):
		m.tab = 2
		m.viewMode = ViewList
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
		m.tab = (m.tab + 1) % len(tabs)
		m.viewMode = ViewList
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("0"))):
		// Starfleet's own server logs.
		m.logServer = true
		m.logName = ""
		m.logEntries = m.fetchLogs("")
		m.logScroll = 0
		m.viewMode = ViewLogs
		return m, nil

	// ← → switch tabs from anywhere (leaving detail/log views first).
	case key.Matches(msg, key.NewBinding(key.WithKeys("left"))):
		if m.tab > 0 {
			m.tab--
			m.viewMode = ViewList
		}
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("right"))):
		if m.tab < len(tabs)-1 {
			m.tab++
			m.viewMode = ViewList
		}
		return m, nil
	}

	// View-specific keys.
	switch m.viewMode {
	case ViewList:
		return m.handleListKey(msg)
	case ViewDetail:
		return m.handleDetailKey(msg)
	case ViewLogs:
		return m.handleLogKey(msg)
	}
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.tab {
	case 0: // Processes
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			m.procTable.MoveUp(1)
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			m.procTable.MoveDown(1)

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			// Show detail view for selected process.
			name := m.selectedProcess()
			if name != "" {
				m.detailName = name
				m.detailLines = m.buildDetail(name)
				m.viewMode = ViewDetail
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("l"))):
			// Show logs for selected process.
			name := m.selectedProcess()
			if name != "" {
				m.logServer = false
				m.logName = name
				m.logEntries = m.fetchLogs(name)
				m.logScroll = 0
				m.viewMode = ViewLogs
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			name := m.selectedProcess()
			if name != "" {
				if err := m.manager.Start(name); err != nil {
					m.showMessage(fmt.Sprintf("✗ %v", err))
				} else {
					m.showMessage(fmt.Sprintf("✓ Started %s", name))
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("t"))):
			name := m.selectedProcess()
			if name != "" {
				if err := m.manager.Stop(name); err != nil {
					m.showMessage(fmt.Sprintf("✗ %v", err))
				} else {
					m.showMessage(fmt.Sprintf("✓ Stopped %s", name))
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			name := m.selectedProcess()
			if name != "" {
				if err := m.manager.Restart(name); err != nil {
					m.showMessage(fmt.Sprintf("✗ %v", err))
				} else {
					m.showMessage(fmt.Sprintf("✓ Restarted %s", name))
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
			// Toggle boot-start (autostart) for the selected process.
			name := m.selectedProcess()
			if name != "" {
				info, err := m.manager.Get(name)
				if err != nil {
					m.showMessage(fmt.Sprintf("✗ %v", err))
				} else if err := m.manager.SetAutostart(name, !info.Autostart); err != nil {
					m.showMessage(fmt.Sprintf("✗ %v", err))
				} else if info.Autostart {
					m.showMessage(fmt.Sprintf("✓ %s will not start on boot", name))
				} else {
					m.showMessage(fmt.Sprintf("✓ %s starts on boot", name))
				}
			}
		}

	case 1: // Schedules
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			m.schedTable.MoveUp(1)
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			m.schedTable.MoveDown(1)
		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			row := m.schedTable.SelectedRow()
			if len(row) > 0 {
				if err := m.manager.RunSchedule(row[0]); err != nil {
					m.showMessage(fmt.Sprintf("✗ %v", err))
				} else {
					m.showMessage(fmt.Sprintf("✓ triggered %s", row[0]))
				}
			}
		}
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "backspace", "q"))):
		m.viewMode = ViewList
	case key.Matches(msg, key.NewBinding(key.WithKeys("l"))):
		// Switch from detail view to the log view of the same process.
		if m.detailName != "" {
			m.logServer = false
			m.logName = m.detailName
			m.logEntries = m.fetchLogs(m.logName)
			m.logScroll = 0
			m.viewMode = ViewLogs
		}
	}
	return m, nil
}

func (m Model) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxScroll := len(m.logEntries) - (m.height - 10)
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "backspace", "q"))):
		m.viewMode = ViewList
	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		// Toggle source: starfleet server logs <-> process logs.
		m.logServer = !m.logServer
		if !m.logServer && m.logName == "" {
			m.logName = m.detailName
		}
		m.logEntries = m.fetchLogs(m.logName)
		m.logScroll = 0
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.logScroll > 0 {
			m.logScroll--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.logScroll < maxScroll {
			m.logScroll++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
		m.logScroll = 0
	case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
		if maxScroll > 0 {
			m.logScroll = maxScroll
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	switch m.viewMode {
	case ViewDetail:
		return m.viewDetail()
	case ViewLogs:
		return m.viewLogs()
	}

	// List view.
	modeBadge := MutedStyle.Render("[local]")
	if m.manager.Mode() == "remote" {
		modeBadge = SuccessStyle.Render("[connected to server]")
	}
	title := lipgloss.JoinHorizontal(lipgloss.Center,
		TitleStyle.Render("🚀 Starfleet"), " ", modeBadge)
	tabsRow := m.renderTabs()
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", tabsRow)

	var content string
	switch m.tab {
	case 0:
		content = m.procTable.View()
	case 1:
		content = m.schedTable.View()
	case 2:
		content = m.viewHelp()
	}

	status := m.renderStatusBar()
	main := lipgloss.JoinVertical(lipgloss.Left, header, "", content)
	return pinFooter(main, status, m.height)
}

// pinFooter renders main and then pads blank lines so footer sits on the
// LAST row of the viewport (instead of floating right below short content).
func pinFooter(main, footer string, height int) string {
	if height <= 0 {
		return main + "\n\n" + footer
	}
	used := strings.Count(main, "\n") + 1
	footerRows := strings.Count(footer, "\n") + 1
	pad := height - used - footerRows
	if pad < 1 {
		pad = 1
	}
	return main + strings.Repeat("\n", pad) + footer
}

func (m Model) renderTabs() string {
	var parts []string
	for i, name := range tabs {
		style := TabStyle
		if m.tab == i {
			style = ActiveTabStyle
		}
		parts = append(parts, style.Render(fmt.Sprintf("[%d] %s", i+1, name)))
	}
	return strings.Join(parts, " ")
}

func (m Model) renderStatusBar() string {
	var left string
	switch m.tab {
	case 0:
		left = HelpStyle.Render("↑↓ move  ←→ tabs  Enter detail  l logs  0 srv logs  s start  t stop  r restart  e enable/disable  q quit")
	case 1:
		left = HelpStyle.Render("↑↓ move  r run now  ←→ tabs  q quit")
	default:
		left = HelpStyle.Render("q quit")
	}
	right := ""
	if m.message != "" {
		right = SuccessStyle.Render(m.message)
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return StatusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

// Detail view - shows everything about a process.
func (m Model) viewDetail() string {
	title := TitleStyle.Render(fmt.Sprintf("📋 Process: %s", m.detailName))

	content := lipgloss.NewStyle().
		Foreground(Text).
		Padding(0, 2).
		Render(strings.Join(m.detailLines, "\n"))

	footer := HelpStyle.Render("  Esc/backspace: back  l: logs  s: start  t: stop  r: restart")

	main := lipgloss.JoinVertical(lipgloss.Left, title, "", content)
	return pinFooter(main, footer, m.height)
}

func (m *Model) buildDetail(name string) []string {
	info, err := m.manager.Get(name)
	if err != nil {
		return []string{ErrorStyle.Render(fmt.Sprintf("Error: %v", err))}
	}

	cfg, cfgErr := m.manager.Config(name)

	var lines []string
	add := func(label, value string) {
		lines = append(lines, fmt.Sprintf("  %s %s",
			lipgloss.NewStyle().Foreground(Muted).Width(16).Render(label),
			value))
	}

	lines = append(lines, BoldStyle.Render("  ── Basic Info ──"))
	add("Name:", info.Name)
	add("State:", StateBadge(info.State))
	add("PID:", fmt.Sprintf("%d", info.PID))
	add("Uptime:", info.Uptime)
	add("Exit Code:", fmt.Sprintf("%d", info.ExitCode))
	add("Retry Count:", fmt.Sprintf("%d", info.RetryCount))
	add("Autorestart:", info.Autorestart)
	add("Autostart:", fmt.Sprintf("%v", info.Autostart))

	lines = append(lines, "")
	lines = append(lines, BoldStyle.Render("  ── Command ──"))
	add("Command:", lipgloss.NewStyle().Foreground(Primary).Render(info.Command))
	add("Directory:", info.Directory)

	// Config details from the manager (local or remote).
	if cfgErr == nil {
		if cfg.User != "" {
			add("User:", cfg.User)
		}
		add("Start Secs:", fmt.Sprintf("%d", cfg.StartSecs))
		add("Start Retries:", fmt.Sprintf("%d", cfg.StartRetries))
		add("Stop Signal:", cfg.StopSignal)
		add("Stop Wait:", fmt.Sprintf("%ds", cfg.StopWaitSecs))
		add("Restart Delay:", cfg.RestartDelay.String())
		add("Backoff Limit:", cfg.BackoffLimit.String())

		if len(cfg.Environment) > 0 {
			lines = append(lines, "")
			lines = append(lines, BoldStyle.Render("  ── Environment ──"))
			for k, v := range cfg.Environment {
				add(k+":", lipgloss.NewStyle().Foreground(Purple).Render(v))
			}
		}

		if cfg.StdoutLog.Path != "" {
			lines = append(lines, "")
			lines = append(lines, BoldStyle.Render("  ── Logging ──"))
			add("Stdout:", cfg.StdoutLog.Path)
			add("Stdout Max:", cfg.StdoutLog.MaxBytes)
			add("Stderr:", cfg.StderrLog.Path)
			add("Stderr Max:", cfg.StderrLog.MaxBytes)
		}

		if cfg.HealthCheck != nil {
			lines = append(lines, "")
			lines = append(lines, BoldStyle.Render("  ── Health Check ──"))
			add("Type:", cfg.HealthCheck.Type)
			if cfg.HealthCheck.URL != "" {
				add("URL:", cfg.HealthCheck.URL)
			}
			if cfg.HealthCheck.Address != "" {
				add("Address:", cfg.HealthCheck.Address)
			}
			add("Interval:", cfg.HealthCheck.Interval.String())
			add("Timeout:", cfg.HealthCheck.Timeout.String())
			add("Retries:", fmt.Sprintf("%d", cfg.HealthCheck.Retries))
		}
	}

	if len(info.Labels) > 0 {
		lines = append(lines, "")
		lines = append(lines, BoldStyle.Render("  ── Labels ──"))
		for k, v := range info.Labels {
			add(k+":", lipgloss.NewStyle().Foreground(Orange).Render(v))
		}
	}

	return lines
}

// Log view — renders captured log entries (same source as the WebUI).
func (m Model) viewLogs() string {
	count := len(m.logEntries)
	source := m.logName
	if m.logServer {
		source = "starfleet (server)"
	}
	title := TitleStyle.Render(fmt.Sprintf("📜 Logs: %s", source))
	meta := MutedStyle.Render(fmt.Sprintf("  %d lines · auto-refresh 1s", count))

	maxLines := m.height - 11
	if maxLines < 3 {
		maxLines = 3
	}
	start := m.logScroll
	if start > count-maxLines {
		start = count - maxLines
	}
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > count {
		end = count
	}

	var body string
	if count == 0 {
		body = HelpStyle.Render("  No output captured yet — lines appear as soon as the process writes.")
	} else {
		tsStyle := lipgloss.NewStyle().Foreground(Subtle)
		streamStyle := lipgloss.NewStyle().Foreground(Muted).Width(6)
		errStyle := lipgloss.NewStyle().Foreground(Error).Width(6)
		lineStyle := lipgloss.NewStyle().Foreground(Text)
		var sb strings.Builder
		for _, e := range m.logEntries[start:end] {
			t := time.UnixMilli(e.Timestamp).Format("15:04:05.000")
			sb.WriteString(tsStyle.Render(t) + " ")
			if e.Stream == "stderr" {
				sb.WriteString(errStyle.Render("err") + " ")
			} else {
				sb.WriteString(streamStyle.Render(e.Stream) + " ")
			}
			sb.WriteString(lineStyle.Render(e.Line))
			sb.WriteString("\n")
		}
		body = lipgloss.NewStyle().Padding(0, 2).Render(strings.TrimRight(sb.String(), "\n"))
	}

	footer := HelpStyle.Render("  ↑↓ scroll  g/G top/bottom  s: toggle source  Esc: back  (auto-refreshes 1s)")

	main := lipgloss.JoinVertical(lipgloss.Left, title+meta, "", body)
	return pinFooter(main, footer, m.height)
}

// fetchLogs pulls recent lines from the active manager — starfleet's own
// server logs when logServer is set, otherwise the process's output.
// The same data sources the WebUI shows.
func (m *Model) fetchLogs(name string) []engine.LogEntry {
	if m.manager == nil {
		return nil
	}
	if m.logServer {
		return m.manager.ServerLogs(500)
	}
	if name == "" {
		return nil
	}
	return m.manager.Logs(name, 500)
}

// refreshLogs re-fetches while the log view is open.
func (m *Model) refreshLogs() {
	m.logEntries = m.fetchLogs(m.logName)
	m.logLoaded = true
}

// Help view.
func (m Model) viewHelp() string {
	help := `
  Keyboard Shortcuts

  Navigation
    ↑/k, ↓/j        Move cursor up/down
    ←/→             Previous / next tab (from any view)
    Tab              Switch tab (forward)
    1, 2, 3          Jump to tab
    Enter            View process details
    l                View process logs
    0                View starfleet's own server logs

  Process Actions
    s                Start selected process (list view)
    t                Stop selected process
    r                Restart selected process

  Schedule Actions
    r                Trigger the selected job immediately

  Detail / Log View
    Esc/backspace    Back to list
    ↑↓               Scroll content
    g / G            Jump to top / bottom
    l                Switch from detail to logs
    s                Toggle log source: server <-> process (log view)

  General
    q / Ctrl+C       Quit
`
	return InfoBoxStyle.Render(help)
}

func (m *Model) selectedProcess() string {
	row := m.procTable.SelectedRow()
	if len(row) > 0 {
		return row[0]
	}
	return ""
}

func (m *Model) refreshData() {
	if m.manager == nil {
		return
	}
	processes := m.manager.List()
	rows := make([]table.Row, len(processes))
	for i, p := range processes {
		boot := "off"
		if p.Autostart {
			boot = "on"
		}
		rows[i] = table.Row{
			p.Name,
			p.State,
			boot,
			fmt.Sprintf("%d", p.PID),
			formatUptime(p.Uptime),
			fmt.Sprintf("%d", p.RetryCount),
			p.Autorestart,
			truncate(p.Command, 28),
		}
	}
	// Only touch the table when something actually changed — constant
	// SetRows calls (nanosecond uptimes) caused visible row flicker.
	if rowsEqual(m.lastRows, rows) {
		m.refreshSchedules()
		return
	}
	m.lastRows = rows
	m.procTable.SetRows(rows)
	m.refreshSchedules()
}

// refreshSchedules keeps the schedules table populated (it used to be
// created but never filled, so the tab was always blank).
func (m *Model) refreshSchedules() {
	jobs := m.manager.Schedules()
	rows := make([]table.Row, 0, len(jobs))
	for _, j := range jobs {
		cmdOrURL := j.Command
		if cmdOrURL == "" {
			cmdOrURL = j.URL
		}
		jobType := j.Type
		if jobType == "" {
			jobType = "exec"
		}
		next := j.NextRun
		if next == "" {
			next = "—"
		}
		rows = append(rows, table.Row{
			j.Name,
			j.Cron,
			jobType,
			fmt.Sprintf("%t", j.Enabled),
			next,
			j.Timeout,
			truncate(cmdOrURL, 28),
		})
	}
	if rowsEqual(m.lastSchedRows, rows) {
		return
	}
	m.lastSchedRows = rows
	m.schedTable.SetRows(rows)
}

// rowsEqual reports whether two table row sets are identical.
func rowsEqual(a, b []table.Row) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

// formatUptime renders a coarse uptime so row content stays stable across
// 1s refresh ticks ("2h15m" instead of "2h15m30.123456789s" which changed
// every frame and made the table look like it was flickering).
func formatUptime(raw string) string {
	if raw == "" || raw == "0s" {
		return "—"
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return raw
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()/5)*5)
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func (m *Model) showMessage(msg string) {
	m.message = msg
	m.msgTimer = 3
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}

// RunTUI starts the TUI with the given manager (local or remote).
func RunTUI(manager Manager, apiURL string) error {
	m := NewModel(manager)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
