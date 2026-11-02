package e2e

// The composed per-stage-failure rollback matrix (plan init-hardening/001
// T11; design init-hardening-v1 §3.10, A-3): the three stage-failure classes
// the design's acceptance table names — a converge fetch death, a wiring
// transport abort (UB-1's "a wiring operation fails ... after the checks
// passed" — the rollback leg of that row; UB-4's classified permission/
// transient gaps hand off and stay green on a separate, pre-existing path
// this design does not touch), and a signal mid-apply — driven through the
// real init flow and asserting the target restores byte-identical (here:
// empty — every fixture creates the repository this run owns), a planted
// foreign entry survives and is named, and init_rolled_back (EVT-3) fires
// with its typed properties. internal/installer's own rollback_test.go
// already proves the unwind mechanism exhaustively via an in-package stage
// injection seam; this proves the REAL pipeline stages (real converge, real
// wiring dispatch, the real signal handler cmd/pyxis installs) route their
// failures into that same mechanism.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestConvergeFetchDeathRollsBackAndRerunSucceeds is UB-2's rollback leg: the
// artifact source becomes unreachable after mutation begins (here, the
// payload archive specifically — the front-loaded artifact-source CHECK only
// probes the manifest's own reachability, so a missing payload is invisible
// to it and only surfaces once converge actually tries to fetch it, deep in
// the apply pipeline). Asserts the target restores to empty (init created
// this repository) and that restoring the artifact lets a re-run succeed.
func TestConvergeFetchDeathRollsBackAndRerunSucceeds(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo := t.TempDir()
	env := pyxisEnv(t, repo)

	payloadPath := filepath.Join(r.src, "payload-"+r.version+".tar.gz")
	payloadBytes, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatalf("read the fixture payload before deleting it: %v", err)
	}
	if err := os.Remove(payloadPath); err != nil {
		t.Fatalf("remove the payload archive (simulate a mid-converge fetch death): %v", err)
	}

	res := pyxis(t, repo, env, initArgs(r)...)
	if res.code != 1 {
		t.Fatalf("init over a fetch-dead payload = exit %d, want 1 (rolled back)\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
	}
	for _, want := range []string{"msg=init_rolled_back", "failed_stage=converge", "restored=true"} {
		if !strings.Contains(res.stderr, want) {
			t.Errorf("rollback stderr does not contain %q:\n%s", want, res.stderr)
		}
	}
	assertEmptyDir(t, repo, "converge-fetch-death rollback")
	failedLog := assertCLILogEvent(t, repo, "init", "init_rolled_back")
	failedBytes, err := os.ReadFile(failedLog)
	if err != nil {
		t.Fatal(err)
	}

	// UB-2's other half: restoring the artifact lets a re-run proceed past
	// admission and end green.
	writeFile(t, payloadPath, payloadBytes, 0o644)
	res2 := pyxis(t, repo, env, initArgs(r)...)
	if res2.code != 0 {
		t.Fatalf("re-run after restoring the payload = exit %d, want 0\nstdout: %s\nstderr: %s", res2.code, res2.stdout, res2.stderr)
	}
	if !strings.Contains(res2.stdout, "converged main") {
		t.Fatalf("re-run summary lacks the converged line:\n%s", res2.stdout)
	}
	retained, err := os.ReadFile(failedLog)
	if err != nil || !bytes.Equal(failedBytes, retained) {
		t.Fatalf("retry did not retain the failed run's diagnostic: %v", err)
	}
	check := exec.Command("git", "-C", repo, "check-ignore", failedLog)
	check.Env = env
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("installed ignore rules do not exclude the retained diagnostic: %v %s", err, output)
	}
}

// TestWiringTransportAbortRollsBackAndPreservesForeign is UB-1's rollback
// leg: a wiring operation aborts at the transport level (the ONE gh failure
// shape that hard-fails the whole walk rather than handing off per UB-4 —
// internal/wire/execute.go's Execute doc comment calls this "gh vanished
// mid-walk"; UB-4's classified permission/transient gaps are a separate,
// pre-existing hand-off path this design does not touch) after every
// earlier stage — converge, seed, hooks — has already
// mutated the tree. A foreign file planted the instant the abort fires
// proves DINV-3's foreign-entry preservation survives through the REAL
// pipeline, not just T4's in-package stage-body seam.
func TestWiringTransportAbortRollsBackAndPreservesForeign(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	target := t.TempDir()

	const foreignRel = "sub/planted-foreign.txt"
	runner := &abortingCreateRunner{ghTranscript: &ghTranscript{}, target: target, foreignRel: foreignRel}

	ans := ghDefectAnswers("octo/widget")
	ans.Tracker = "github"
	ans.WiringRepo = "create"
	ans.WiringProtection = "none"

	rep, logJSON, err := runInitChecks(t, r, target, ans, runner)
	if err == nil {
		t.Fatal("Init = nil error, want the injected wiring transport abort")
	}
	if rep.Rollback == nil {
		t.Fatal("Report.Rollback is nil")
	}
	if rep.Rollback.FailedStage != "wiring" {
		t.Errorf("FailedStage = %q, want wiring", rep.Rollback.FailedStage)
	}
	if !rep.Rollback.Restored {
		t.Errorf("Restored = false, want true (errors: %v)", rep.Rollback.Errors)
	}
	if !rep.Rollback.RepoRemoved {
		t.Error("RepoRemoved = false, want true — init created this repository")
	}
	if !slices.Contains(rep.Rollback.ForeignPreserved, foreignRel) {
		t.Errorf("ForeignPreserved = %v, want it to contain %q", rep.Rollback.ForeignPreserved, foreignRel)
	}
	if _, statErr := os.Stat(filepath.Join(target, filepath.FromSlash(foreignRel))); statErr != nil {
		t.Errorf("the foreign file did not survive the unwind: %v", statErr)
	}
	entries, rerr := os.ReadDir(target)
	if rerr != nil {
		t.Fatalf("readdir target: %v", rerr)
	}
	if len(entries) != 1 || entries[0].Name() != "sub" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("unexpected top-level survivors: %v, want only [sub] (init's own writes must be gone)", names)
	}

	evt := findLogEvent(t, logJSON, "init_rolled_back")
	if evt == nil {
		t.Fatalf("no init_rolled_back event in the captured log:\n%s", logJSON)
	}
	if evt["failed_stage"] != "wiring" {
		t.Errorf("init_rolled_back failed_stage = %v, want wiring", evt["failed_stage"])
	}
	if evt["restored"] != true {
		t.Errorf("init_rolled_back restored = %v, want true", evt["restored"])
	}

	// A survivor is REAL content, not silently swallowed: a re-run correctly
	// refuses at admission naming it, exactly as it would for any other
	// foreign file (T4's own precedent test makes the same call — a
	// foreign-content target is adopt's door, not a "re-run succeeds" case).
	_, _, rerunErr := runInitChecks(t, r, target, ans, &ghTranscript{})
	if rerunErr == nil {
		t.Fatal("re-run over the surviving foreign file = nil error, want the admission refusal")
	}
}

// TestSignalMidApplyEndsCleanEitherWay is UB-3's signal-interruption clause,
// proven against the REAL production signal handler cmd/pyxis installs
// (signal.NotifyContext in initVerb) rather than a simulated context
// cancellation: a real SIGTERM sent shortly after the subprocess starts must
// never leave a stranded half-built tree — either the run finished green
// before the signal could interrupt anything (tolerated exactly like
// kill_test.go's own best-effort SIGKILL leg), or it ends with the target
// restored empty (a checks-phase refusal wrote nothing; an apply-phase
// interruption rolled back) — and, in the latter case, a subsequent run
// completes cleanly.
func TestSignalMidApplyEndsCleanEitherWay(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo := t.TempDir()
	env := pyxisEnv(t, repo)

	cmd := exec.Command(builtBins["pyxis"], initArgs(r)...)
	cmd.Dir = repo
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start init: %v", err)
	}
	time.Sleep(8 * time.Millisecond) // best-effort: land the signal somewhere mid-pipeline
	_ = cmd.Process.Signal(syscall.SIGTERM)
	waitErr := cmd.Wait()

	if waitErr == nil {
		// The signal arrived too late to interrupt anything — a vacuous
		// pass: a full green install is still a CLEAN outcome, never a
		// partial one, so nothing further to recover.
		if !strings.Contains(stdout.String(), "converged main") {
			t.Fatalf("process exited 0 but the summary lacks the converged line:\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
		}
		return
	}

	// The signal landed during checks (zero writes) or during apply (rolled
	// back) — either way UB-3 promises a clean, empty folder, never a
	// stranded half-built tree.
	assertEmptyDir(t, repo, "signal mid-apply")

	res := pyxis(t, repo, env, initArgs(r)...)
	if res.code != 0 {
		t.Fatalf("re-run after the signal did not end green: exit %d\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
	}
}

// assertEmptyDir fatals unless root holds no project content. A rollback
// retains local diagnostics so its failure can be investigated after a retry.
func assertEmptyDir(t *testing.T, root, what string) {
	t.Helper()
	if entries := snap(t, root); len(entries) != 0 {
		t.Fatalf("%s: project content remains: %v", what, entries)
	}
}

// abortingCreateRunner wraps a happy-path ghTranscript unchanged for every
// call EXCEPT the wiring stage's own `repo create` mutation: there, it first
// plants a foreign file directly under target — synchronously, on the same
// goroutine that is executing the wiring stage, so there is no timing race
// to land it "mid-apply" — then returns a transport-level abort (a Go error
// from RunGH itself, the shape Execute's own contract treats as "gh vanished
// mid-walk", hard-failing the whole walk rather than UB-4's classified
// hand-off).
type abortingCreateRunner struct {
	*ghTranscript
	target     string
	foreignRel string
	planted    bool
}

func (a *abortingCreateRunner) RunGH(ctx context.Context, args ...string) (string, string, int, error) {
	if len(args) >= 2 && args[0] == "repo" && args[1] == "create" {
		if !a.planted {
			a.planted = true
			p := filepath.Join(a.target, filepath.FromSlash(a.foreignRel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", "", 0, err
			}
			if err := os.WriteFile(p, []byte("not init's\n"), 0o644); err != nil {
				return "", "", 0, err
			}
		}
		return "", "", 0, errors.New("gh: transport reset while creating the repository (simulated: authorization revoked mid-run)")
	}
	return a.ghTranscript.RunGH(ctx, args...)
}
