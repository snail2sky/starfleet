//go:build windows

package scheduler

import (
	"context"
	"os/exec"
)

// shellCommand runs a command through cmd.exe — the Windows counterpart of
// `sh -c`, so `command:` keeps the same "one shell line" semantics everywhere.
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "cmd", "/C", command)
}

// setupExec is a no-op on Windows: there is no process-group/kill semantics
// matching POSIX; the default CommandContext behaviour (kill the direct child)
// applies.
func setupExec(cmd *exec.Cmd) {}

// configureUser is a no-op on Windows: the job runs as the current user
// regardless of the configured `user` field (same as processes).
func configureUser(cmd *exec.Cmd, username string) error {
	return nil
}
