//go:build !windows

package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
	"time"
)

// shellCommand runs a command through the POSIX shell (same semantics as
// processes' `command:` field: pipes, redirection, variable expansion all work).
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", command)
}

// setupExec makes the job's child run in its own process group and kills the
// whole group when the context is done. Without this, a timed-out shell leaves
// its grandchildren running (they keep the captured stdout pipe open too).
func setupExec(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			// negative pid = the whole process group
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = 3 * time.Second
}

// configureUser makes the scheduled job run as the configured user — the same
// semantics as processes' `user=` field. No-op when username is empty or is
// already the current effective user; switching to a *different* user requires
// root.
func configureUser(cmd *exec.Cmd, username string) error {
	if username == "" {
		return nil
	}
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", username, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("parse uid of %q: %w", username, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("parse gid of %q: %w", username, err)
	}
	// Already the target user: skip credentials entirely. An unprivileged
	// process cannot call setgroups even with its own groups, so setting
	// Credential here would make cmd.Start fail with EPERM.
	if os.Geteuid() == uid && os.Getegid() == gid {
		return nil
	}
	cred := &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	if gids, gerr := u.GroupIds(); gerr == nil {
		groups := make([]uint32, 0, len(gids))
		for _, g := range gids {
			if gi, convErr := strconv.Atoi(g); convErr == nil {
				groups = append(groups, uint32(gi))
			}
		}
		cred.Groups = groups
	} else {
		// Drop supplementary groups rather than inherit the parent's.
		cred.Groups = []uint32{}
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = cred
	return nil
}
