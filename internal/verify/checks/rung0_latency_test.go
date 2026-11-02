package checks

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// TestRung0Latency_FixtureAndDogfoodRepo is a hermetic smoke + measurement for
// the rung-0 neutral checks: it proves they execute without error on both a
// controlled-size fixture and this repository's own (dogfood) history, and it
// records each check's measured Duration for the harness_run structured log
// (EVT-1).
//
// It deliberately makes NO wall-clock assertion. The basis's NFR-1 (p95 rung-0
// wall-time <= 10s, docs/prds/implementation-verification/000-base.md) is a
// statistical claim measured across many real dogfood runs (000-base.md §6's
// Phase-B), never a single-run budget: a wall-clock threshold is a *measurement*,
// not a correctness gate, and asserting one here stranded the suite red on a slow
// CI runner for identical code — incident I5. Measurements record; they never gate.
func TestRung0Latency_FixtureAndDogfoodRepo(t *testing.T) {
	checkIDs := []string{SecretScanCheckID, ForbiddenArtifactsCheckID, MachineryProtectionCheckID}
	manifest := &schema.Manifest{Version: "1", Stack: "go"}
	eco := ecosystem.New()

	t.Run("fixture", func(t *testing.T) {
		ds := syntheticFixtureDiff(50, 20)
		env := &Env{Manifest: manifest, Ecosystem: eco, Diff: ds}
		measureRung0(t, "fixture", env, checkIDs)
	})

	t.Run("dogfood", func(t *testing.T) {
		ds := dogfoodDiff(t)
		env := &Env{Manifest: manifest, Ecosystem: eco, Diff: ds}
		measureRung0(t, "dogfood", env, checkIDs)
	})
}

// measureRung0 runs each of checkIDs against env, asserts each executes without
// error (the hermetic smoke), and records every check's measured Duration plus
// the rung-0 total via t.Logf — measurement only, never a wall-clock assertion
// (incident I5).
func measureRung0(t *testing.T, label string, env *Env, checkIDs []string) {
	t.Helper()
	var total time.Duration
	rc := RunContext{
		Workspace: env.Workspace,
		Bindings:  env.Bindings,
		Manifest:  env.Manifest,
		Ecosystem: env.Ecosystem,
		Diff:      env.Diff,
	}
	for _, id := range checkIDs {
		fn := Get(id)
		if fn == nil {
			t.Fatalf("%s: %s: no executor registered", label, id)
		}
		outcome := fn(context.Background(), rc)
		if outcome.Result.State == "error" {
			t.Fatalf("%s: %s: %s", label, id, outcome.Result.Message)
		}
		d := time.Duration(outcome.Result.Duration)
		t.Logf("%s: %s took %s (%d finding(s))", label, id, d, len(outcome.Findings))
		total += d
	}
	t.Logf("%s: rung-0 neutral-check total: %s over %d file(s)", label, total, len(env.Diff.Files))
}

// syntheticFixtureDiff builds an in-memory DiffSet with numFiles added files
// of linesPerFile ordinary content lines each — a controlled-size stand-in
// for T3's future testdata fixture repo (out of T4's scope; see the plan's
// T3/T4 split).
func syntheticFixtureDiff(numFiles, linesPerFile int) *diffset.DiffSet {
	files := make([]diffset.FileChange, 0, numFiles)
	for i := 0; i < numFiles; i++ {
		lines := make([]diffset.AddedLine, 0, linesPerFile)
		for j := 0; j < linesPerFile; j++ {
			lines = append(lines, diffset.AddedLine{
				Line:    j + 1,
				Content: fmt.Sprintf("\tresult := compute(%d, %d) // ordinary line, no findings expected", i, j),
			})
		}
		files = append(files, diffset.FileChange{
			Status:     diffset.StatusAdded,
			Path:       fmt.Sprintf("pkg/synthetic/file_%03d.go", i),
			SizeBytes:  int64(linesPerFile * 60),
			AddedLines: lines,
		})
	}
	return &diffset.DiffSet{Base: "synthetic-base", Head: "synthetic-head", Files: files}
}

// dogfoodDiff extracts a real DiffSet from this repository's own history —
// the "dogfood repo" NFR-1 names. The base is the oldest commit within the
// last 10 (fewer if the checkout is shallow), so the diff is real, bounded,
// and available whether the checkout is full or shallow.
func dogfoodDiff(t *testing.T) *diffset.DiffSet {
	t.Helper()
	// The runner owns liveness cancellation; a local wall-clock deadline would
	// turn this measurement back into the machine-dependent gate incident I5 forbids.
	ctx := t.Context()

	root := strings.TrimSpace(mustGit(t, ctx, ".", "rev-parse", "--show-toplevel"))
	shas := strings.Fields(mustGit(t, ctx, root, "log", "--format=%H", "-n", "10", "HEAD"))
	if len(shas) == 0 {
		t.Skip("no commit history available to build a dogfood diff")
	}
	base := shas[len(shas)-1]

	ds, err := diffset.Extract(ctx, root, base, "HEAD")
	if err != nil {
		t.Fatalf("dogfood diffset.Extract: %v", err)
	}
	return ds
}

func mustGit(t *testing.T, ctx context.Context, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
