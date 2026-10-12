package tui

import (
	"strings"
	"testing"
)

// TestVersionInHelp: the build version must surface in the Help tab.
func TestVersionInHelp(t *testing.T) {
	SetVersion("v9.9.9-test")
	defer SetVersion("dev")

	m := Model{}
	out := m.viewHelp()
	if !strings.Contains(out, "Starfleet v9.9.9-test") {
		t.Errorf("help view missing version:\n%s", out)
	}
}

// TestSetVersionIgnoresEmpty keeps "dev" when nothing is injected.
func TestSetVersionIgnoresEmpty(t *testing.T) {
	SetVersion("dev")
	SetVersion("")
	if version != "dev" {
		t.Errorf("empty SetVersion must not clobber, got %q", version)
	}
}
