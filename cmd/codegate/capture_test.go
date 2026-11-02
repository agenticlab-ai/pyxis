package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
)

// TestCaptureBaseline_ProducesBaselineRunBinds is A-11 end to end: capture-
// baseline reduces the workspace's current failures to a baseline; once
// committed, a run binds it and the pre-existing failure is no longer "new" —
// rung 2 passes despite a genuinely red test. It also audits DINV-6: run writes
// no baseline.
func TestCaptureBaseline_ProducesBaselineRunBinds(t *testing.T) {
	ws := gitFixtureWorkspace(t, func(dir string) {
		// A pre-existing failing test, and a manifest that declares where its
		// baseline lives. (fixture-go is `package main`, module fixture-go.)
		if err := os.WriteFile(filepath.Join(dir, "preexisting_test.go"),
			[]byte("package main\n\nimport \"testing\"\n\nfunc TestPreexisting(t *testing.T) { t.Fatal(\"known red\") }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".pyxis", "codegate.json"),
			[]byte("{\"version\":\"1.0.0\",\"stack\":\"go\",\"baseline_path\":\"baseline.json\"}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})

	// Capture the current failures, then commit the baseline so the tree is
	// clean and the baseline is bound for the run.
	if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitPass {
		t.Fatalf("capture-baseline exit = %d, want %d", code, invoke.ExitPass)
	}
	baselinePath := filepath.Join(ws, "baseline.json")
	captured, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read captured baseline: %v", err)
	}
	gitOK(t, ws, "add", "baseline.json")
	gitOK(t, ws, "commit", "-q", "-m", "baseline")

	// The failing test is now baselined — a run binds it and rung 2 passes.
	if code := runCmd([]string{"--workspace", ws, "--diff-base", "HEAD"}); code != invoke.ExitPass {
		t.Fatalf("run exit = %d, want %d (the failing test is baselined, not new)", code, invoke.ExitPass)
	}
	v := readVerdict(t, ws)
	r2 := v.Rungs["2"]
	if r2 == nil {
		t.Fatal("verdict has no rung 2 — the no-new-failures check must have run")
	}
	if c := r2.Checks["no-new-failures"]; c == nil || c.State != "pass" {
		t.Errorf("rung-2 no-new-failures = %+v, want state=pass (the baselined failure is not new)", c)
	}

	// DINV-6: run mode wrote no baseline.
	after, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("baseline gone after run: %v", err)
	}
	if string(after) != string(captured) {
		t.Error("run mutated the baseline file — run mode must never write a baseline (DINV-6)")
	}
}

// TestCaptureBaseline_RefusesWithoutBaselinePath: nothing to capture into.
func TestCaptureBaseline_RefusesWithoutBaselinePath(t *testing.T) {
	ws := gitFixtureWorkspace(t, nil) // fixture-go declares no baseline_path
	if code := captureBaselineCmd([]string{"--workspace", ws}); code != invoke.ExitConfigErr {
		t.Errorf("exit = %d, want %d (no baseline_path declared)", code, invoke.ExitConfigErr)
	}
}
