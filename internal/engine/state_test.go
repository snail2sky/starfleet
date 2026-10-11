package engine

import "testing"

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateStopped, "STOPPED"},
		{StateStarting, "STARTING"},
		{StateRunning, "RUNNING"},
		{StateStopping, "STOPPING"},
		{StateBackoff, "BACKOFF"},
		{StateFatal, "FATAL"},
		{StateExited, "EXITED"},
		{State(99), "UNKNOWN(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.expected)
			}
		})
	}
}

func TestStateIsTerminal(t *testing.T) {
	tests := []struct {
		state    State
		terminal bool
	}{
		{StateStopped, true},
		{StateStarting, false},
		{StateRunning, false},
		{StateStopping, false},
		{StateBackoff, false},
		{StateFatal, true},
		{StateExited, true},
	}
	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.terminal {
				t.Errorf("State(%s).IsTerminal() = %v, want %v", tt.state, got, tt.terminal)
			}
		})
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		input   string
		want    State
		wantErr bool
	}{
		{"STOPPED", StateStopped, false},
		{"RUNNING", StateRunning, false},
		{"FATAL", StateFatal, false},
		{"INVALID", StateStopped, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseState(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseState(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseState(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestShouldFatal(t *testing.T) {
	tests := []struct {
		name          string
		startFailures int
		maxRetries    int
		startedOK     bool
		want          bool
	}{
		{"healthy run never fatalizes", 99, 3, true, false},
		{"start failures below limit", 2, 3, false, false},
		{"start failures at limit", 3, 3, false, true},
		{"start failures above limit", 5, 3, false, true},
		{"zero limit defaults to one", 1, 0, false, true},
		{"first healthy run clears", 0, 3, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldFatal(tt.startFailures, tt.maxRetries, tt.startedOK); got != tt.want {
				t.Errorf("shouldFatal(%d, %d, %v) = %v, want %v",
					tt.startFailures, tt.maxRetries, tt.startedOK, got, tt.want)
			}
		})
	}
}
