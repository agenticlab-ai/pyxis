//go:build !windows

package procgroup_test

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/runtime/procgroup"
)

// TestKill_TerminatesTheWholeGroup proves a group kill reaches a GRANDCHILD — a
// process the direct child spawned — not just the direct child. That grandchild
// is exactly the orphan the runtime adapters left before procgroup: a headless
// CLI's own children outliving a cancelled or bounded-out Worker.
func TestKill_TerminatesTheWholeGroup(t *testing.T) {
	// sh backgrounds a long-lived sleep (the "grandchild"), prints its PID, then
	// waits — so the whole group stays alive until we kill it.
	cmd := exec.Command("sh", "-c", "sleep 600 & echo $!; wait")
	procgroup.Setpgid(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Wait() })

	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatalf("read grandchild pid: %v", err)
	}
	gpid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("parse grandchild pid %q: %v", line, err)
	}
	if !alive(gpid) {
		t.Fatalf("grandchild %d should be alive before the kill", gpid)
	}

	procgroup.Kill(cmd, true)

	// The grandchild must die with the group. Poll for the kernel to reap it —
	// a liveness STATE, never a wall-clock budget assertion (incident I5).
	if waitDead(gpid) {
		return
	}
	_ = syscall.Kill(gpid, syscall.SIGKILL) // don't leak it if the fix regressed
	t.Fatalf("grandchild %d survived a group kill — it was orphaned", gpid)
}

// TestSetpgid_PutsChildInItsOwnGroup pins that a Setpgid child leads its own
// group (PGID == PID), the precondition the negative-PID Kill relies on.
func TestSetpgid_PutsChildInItsOwnGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 600")
	procgroup.Setpgid(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { procgroup.Kill(cmd, true); _ = cmd.Wait() })

	pid := cmd.Process.Pid
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatalf("getpgid(%d): %v", pid, err)
	}
	if pgid != pid {
		t.Fatalf("child pgid = %d, want its own pid %d (Setpgid did not take)", pgid, pid)
	}
}

// alive reports whether pid exists (signal 0 probes without delivering one).
func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// waitDead polls until pid is gone, up to a generous bound. The bound is a
// flake-guard for reap latency, not a timing assertion.
func waitDead(pid int) bool {
	for i := 0; i < 200; i++ {
		if !alive(pid) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return !alive(pid)
}
