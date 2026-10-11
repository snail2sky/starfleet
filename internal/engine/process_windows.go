//go:build windows

package engine

import (
	"os/exec"
	"syscall"
)

// setProcessGroup is a no-op on Windows.
func setProcessGroup(cmd *exec.Cmd) {
	// Windows does not support Setpgid.
}

// configureUser is a no-op on Windows: the child runs as the current user
// regardless of the configured `user` field.
func configureUser(cmd *exec.Cmd, username string) error {
	return nil
}

// parseSignal parses a signal name to os.Signal.
func parseSignal(name string) syscall.Signal {
	switch name {
	case "SIGTERM", "SIGINT", "SIGKILL":
		return syscall.SIGTERM
	default:
		return syscall.SIGTERM
	}
}
