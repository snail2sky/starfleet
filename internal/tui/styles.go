package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// Color palette (matching web UI dark theme).
var (
	Primary = lipgloss.Color("#38bdf8")
	Success = lipgloss.Color("#4ade80")
	Warning = lipgloss.Color("#facc15")
	Error   = lipgloss.Color("#f87171")
	Purple  = lipgloss.Color("#a78bfa")
	Orange  = lipgloss.Color("#fb923c")
	Muted   = lipgloss.Color("#64748b")
	Subtle  = lipgloss.Color("#334155")
	BgDark  = lipgloss.Color("#0f172a")
	BgCard  = lipgloss.Color("#1e293b")
	Text    = lipgloss.Color("#e2e8f0")
	TextDim = lipgloss.Color("#94a3b8")
)

// Common styles.
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(0, 2)

	TabStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Padding(0, 2)

	ActiveTabStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true).
			Padding(0, 2)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(TextDim).
			Background(BgCard).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(Muted)

	InfoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Subtle).
			Padding(1, 2)

	SuccessStyle = lipgloss.NewStyle().Foreground(Success)
	WarningStyle = lipgloss.NewStyle().Foreground(Warning)
	ErrorStyle   = lipgloss.NewStyle().Foreground(Error)
	MutedStyle   = lipgloss.NewStyle().Foreground(Muted)
	BoldStyle    = lipgloss.NewStyle().Bold(true)
)

// TableStyles is the single source of truth for table styling.
// Every table must use it — before v1.5 the schedules table used the
// library default (red selection) while processes used blue, which looked
// broken when switching tabs.
func TableStyles() table.Styles {
	ts := table.DefaultStyles()
	ts.Header = ts.Header.Bold(true).Foreground(Primary)
	ts.Selected = ts.Selected.Foreground(Primary).Background(BgCard)
	return ts
}

// StateColor returns the color for a process state.
func StateColor(state string) lipgloss.Color {
	switch state {
	case "RUNNING":
		return Success
	case "STOPPED", "EXITED":
		return Muted
	case "FATAL":
		return Error
	case "BACKOFF", "STARTING", "STOPPING":
		return Warning
	default:
		return TextDim
	}
}

// StateBadge returns a styled state badge.
func StateBadge(state string) string {
	style := lipgloss.NewStyle().
		Foreground(StateColor(state)).
		Bold(true).
		Width(10)
	return style.Render(state)
}
