//go:build !windows

package claude

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestOSProcess_KillReapsTheWholeTree proves the adapter's OWN start+kill path
// (osCommander/osProcess, not procgroup in isolation) terminates a GRANDCHILD —
// a process the direct child spawned. That grandchild is exactly the orphan the
// pre-fix direct-PID kill left behind: a headless CLI's own node/MCP child
// outliving a cancelled Worker (the incident this fix closes).
func TestOSProcess_KillReapsTheWholeTree(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	// sh backgrounds a long-lived sleep (the "grandchild"), records its PID, then
	// waits so the whole group stays alive until we kill it.
	proc, err := osCommander{}.start(context.Background(), commandSpec{
		Name: "sh",
		Args: []string{"-c", "sleep 600 & echo $! > " + pidFile + "; wait"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	go func() { _, _, _ = proc.wait() }() // drain the direct child's reap

	gpid := pgReadPID(t, pidFile)
	if !pgAlive(gpid) {
		t.Fatalf("grandchild %d should be alive before the kill", gpid)
	}

	if err := proc.kill(); err != nil {
		t.Fatalf("kill: %v", err)
	}

	// The grandchild must die with the group. Poll for the reap — a liveness
	// STATE, never a wall-clock budget assertion (incident I5).
	if pgWaitDead(gpid) {
		return
	}
	_ = syscall.Kill(gpid, syscall.SIGKILL) // don't leak it if the fix regressed
	t.Fatalf("grandchild %d survived the adapter's kill — it was orphaned", gpid)
}

func pgReadPID(t *testing.T, path string) int {
	t.Helper()
	for i := 0; i < 300; i++ {
		if b, err := os.ReadFile(path); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				if pid, err := strconv.Atoi(s); err == nil {
					return pid
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("grandchild pid never written to %s", path)
	return 0
}

func pgAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func pgWaitDead(pid int) bool {
	for i := 0; i < 300; i++ {
		if !pgAlive(pid) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return !pgAlive(pid)
}
