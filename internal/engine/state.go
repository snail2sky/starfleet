// Package engine implements the process management engine.
package engine

import "fmt"

// State represents the current state of a managed process.
type State int

const (
	// StateStopped means the process is not running.
	StateStopped State = iota
	// StateStarting means the process is being started.
	StateStarting
	// StateRunning means the process is running normally.
	StateRunning
	// StateStopping means the process is being stopped.
	StateStopping
	// StateBackoff means the process failed to start and is waiting before retry.
	StateBackoff
	// StateFatal means the process has exceeded start retries and will not be restarted.
	StateFatal
	// StateExited means the process exited normally (autorestart=false).
	StateExited
)

var stateNames = map[State]string{
	StateStopped:  "STOPPED",
	StateStarting: "STARTING",
	StateRunning:  "RUNNING",
	StateStopping: "STOPPING",
	StateBackoff:  "BACKOFF",
	StateFatal:    "FATAL",
	StateExited:   "EXITED",
}

// String returns the string representation of the state.
func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", int(s))
}

// IsTerminal returns true if the process is in a terminal state.
func (s State) IsTerminal() bool {
	return s == StateStopped || s == StateFatal || s == StateExited
}

// ParseState parses a string into a State.
func ParseState(s string) (State, error) {
	for state, name := range stateNames {
		if name == s {
			return state, nil
		}
	}
	return StateStopped, fmt.Errorf("unknown state: %s", s)
}
