package tui

import "testing"

func TestFormatUptime(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "—"},
		{"0s", "—"},
		{"3.456s", "0s"}, // <1m rounded to 5s bucket
		{"7.2s", "5s"},
		{"59.9s", "55s"},
		{"65.5s", "1m"}, // <1h → minutes only (stable for a whole minute)
		{"3599s", "59m"},
		{"3600s", "1h00m"},
		{"8130.123456s", "2h15m"},
		{"not-a-duration", "not-a-duration"},
	}
	for _, tt := range tests {
		if got := formatUptime(tt.in); got != tt.want {
			t.Errorf("formatUptime(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
