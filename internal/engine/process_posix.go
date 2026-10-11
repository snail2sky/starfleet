//go:build !windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// setProcessGroup sets the process to run in its own process group.
// Must be called before configureUser (it replaces SysProcAttr).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// configureUser makes the child run as the configured user (supervisord's
// `user=` equivalent). No-op when username is empty or already the current
// effective user; switching to a *different* user requires root.
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
			gi, convErr := strconv.Atoi(g)
			if convErr == nil {
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

// parseSignal parses a signal name to os.Signal.
func parseSignal(name string) syscall.Signal {
	switch name {
	case "SIGTERM":
		return syscall.SIGTERM
	case "SIGINT":
		return syscall.SIGINT
	case "SIGQUIT":
		return syscall.SIGQUIT
	case "SIGHUP":
		return syscall.SIGHUP
	case "SIGKILL":
		return syscall.SIGKILL
	case "SIGUSR1":
		return syscall.SIGUSR1
	case "SIGUSR2":
		return syscall.SIGUSR2
	default:
		return syscall.SIGTERM
	}
}
