//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestRunRelaying_RelaysSignalToTheChild proves the source-mode launcher relays
// a signal to the tool it runs as a child — the fix for the `go run` launch
// gotcha, where killing `pyxis exec` directly left the tool orphaned because a
// bare child never receives the launcher's own signal. The child installs a
// SIGTERM trap that exits with a distinctive code; a relayed SIGTERM must
// produce exactly that code.
func TestRunRelaying_RelaysSignalToTheChild(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	// sh installs a SIGTERM trap (exit 42), marks itself ready by creating a
	// file, then waits on a BACKGROUND sleep — `& wait` (not a foreground sleep)
	// so the trap fires promptly on the signal instead of after the sleep, which
	// is bash's real behavior and mimics the Go tool's async signal handling.
	cmd := exec.Command("sh", "-c", "trap 'exit 42' TERM; : > "+ready+"; sleep 60 & wait")
	sigs := make(chan os.Signal, 1)

	type result struct {
		code int
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		code, err := runRelaying(cmd, sigs)
		resCh <- result{code, err}
	}()

	// Wait for the child to install its trap (the readiness file), then relay a
	// SIGTERM as the launcher would.
	for i := 0; ; i++ {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if i > 500 {
			t.Fatal("child never signalled readiness")
		}
		time.Sleep(10 * time.Millisecond)
	}
	sigs <- syscall.SIGTERM

	select {
	case res := <-resCh:
		if res.err != nil {
			t.Fatalf("runRelaying: %v", res.err)
		}
		if res.code != 42 {
			t.Fatalf("child exit = %d, want 42 (its SIGTERM trap) — the signal was not relayed", res.code)
		}
	case <-time.After(15 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		t.Fatal("child never exited — SIGTERM was not relayed to it")
	}
}

// TestRunRelaying_RelaysExitCodeVerbatim pins that with no signal, the child's
// own exit code is relayed unchanged — the verdict contract goRun depends on.
func TestRunRelaying_RelaysExitCodeVerbatim(t *testing.T) {
	code, err := runRelaying(exec.Command("sh", "-c", "exit 7"), make(chan os.Signal))
	if err != nil {
		t.Fatalf("runRelaying: %v", err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7 (relayed verbatim)", code)
	}
}
