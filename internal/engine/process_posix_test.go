//go:build !windows

package engine

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"testing"
)

func TestConfigureUserSameUserIsNoop(t *testing.T) {
	cmd := exec.Command("true")
	// Our own user must be a no-op (an unprivileged process cannot setgroups).
	cur, err := user.Current()
	if err != nil {
		t.Skipf("cannot determine current user: %v", err)
	}
	if err := configureUser(cmd, cur.Username); err != nil {
		t.Fatalf("same-user configureUser: %v", err)
	}
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Credential != nil {
		t.Error("same-user must not set credentials (setgroups would EPERM)")
	}

	// Empty username is also a no-op.
	if err := configureUser(exec.Command("true"), ""); err != nil {
		t.Fatalf("empty user: %v", err)
	}
}

func TestConfigureUserUnknownUser(t *testing.T) {
	cmd := exec.Command("true")
	if err := configureUser(cmd, "definitely-not-a-real-user-xyz"); err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func TestConfigureUserDifferentUserNeedsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("only meaningful as root")
	}
	// As root, switching to a non-root user must install credentials.
	cmd := exec.Command("true")
	if err := configureUser(cmd, "nobody"); err != nil {
		t.Fatalf("root switching to nobody: %v", err)
	}
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Credential == nil {
		t.Fatal("expected Credential for root → nobody switch")
	}
	u, _ := user.Lookup("nobody")
	wantUID, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		t.Fatalf("parse uid: %v", err)
	}
	if cmd.SysProcAttr.Credential.Uid != uint32(wantUID) {
		t.Errorf("credential uid mismatch: got %d, want %d", cmd.SysProcAttr.Credential.Uid, wantUID)
	}
}
