//go:build !linux

package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestConfine_NonLinux_NeverRewritesCmd proves the named residual (design
// codegate-v1 §3.5): on every platform besides Linux, Confine leaves cmd
// completely untouched and reports confinement as not attempted, so
// finalizeConfinement never scans this platform's verb output for a
// violation signature.
func TestConfine_NonLinux_NeverRewritesCmd(t *testing.T) {
	t.Setenv("CODEGATE_LANDLOCK", "1") // Linux's opt-in cannot activate its adapter here.
	cmd := exec.Command("/bin/echo", "hello")
	wantPath, wantArgs := cmd.Path, append([]string(nil), cmd.Args...)

	confined, err := Confine(cmd, []string{"/some/workspace"})
	if err != nil {
		t.Fatalf("Confine: %v", err)
	}
	if confined {
		t.Error("confined = true, want false on a non-Linux platform")
	}
	if cmd.Path != wantPath {
		t.Errorf("cmd.Path = %q, want unchanged %q", cmd.Path, wantPath)
	}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("cmd.Args = %v, want unchanged %v", cmd.Args, wantArgs)
	}
	for i := range wantArgs {
		if cmd.Args[i] != wantArgs[i] {
			t.Errorf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], wantArgs[i])
		}
	}
}

func TestRunConfinedExec_NonLinuxRefusesInternalMarker(t *testing.T) {
	restore := unsupportedConfineExit
	defer func() { unsupportedConfineExit = restore }()
	got := 0
	unsupportedConfineExit = func(code int) { got = code }
	RunConfinedExec([]string{"ignored"})
	if got != 1 {
		t.Fatalf("exit = %d, want 1", got)
	}
}

// TestExec_NonLinux_NoWriteScopeEnforcement documents the residual end to
// end through the real Exec entry point: a verb writing outside the
// workspace it was confined to still succeeds on this platform (no
// enforcement exists here — A-6's write-scope leg is pinned to the Linux CI
// matrix, per design §3.5 and §6 risk R6).
func TestExec_NonLinux_NoWriteScopeEnforcement(t *testing.T) {
	workspace := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "out-of-scope.txt")

	verb := helperVerb(nil, "write-file", outsidePath)
	res := Exec(context.Background(), workspace, verb)

	if res.Class == ClassViolation {
		t.Error("Class = violation, want no enforcement (and so no violation classification) on this platform")
	}
	if !res.Passed {
		t.Fatalf("unexpected failure: %+v", res)
	}
	if _, err := os.Stat(outsidePath); err != nil {
		t.Errorf("expected the out-of-workspace write to have succeeded (no enforcement here): %v", err)
	}
}

// TestExec_NonLinux_ConfinedWriteInsideWorkspaceStillWorks proves the
// residual does not somehow ALSO break the ordinary case: a verb writing
// inside its own workspace is unaffected.
func TestExec_NonLinux_ConfinedWriteInsideWorkspaceStillWorks(t *testing.T) {
	workspace := t.TempDir()
	insidePath := filepath.Join(workspace, "in-scope.txt")

	verb := helperVerb(nil, "write-file", insidePath)
	res := Exec(context.Background(), workspace, verb)

	if !res.Passed {
		t.Fatalf("unexpected failure: %+v", res)
	}
	if _, err := os.Stat(insidePath); err != nil {
		t.Errorf("expected the in-workspace write to have succeeded: %v", err)
	}
}
