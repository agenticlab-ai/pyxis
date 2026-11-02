package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/tracker/carina"
	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/watch"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func TestWatchVerbRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing required", []string{}, "--plan and --repo-slug are required"},
		{"unknown flag", []string{"--unknown"}, "flag provided but not defined"},
		{"flag after positional", []string{"--plan", "docs/plans/x", "--repo-slug", "o/r", "stray", "--late"}, "flags precede positionals"},
		{"unexpected positional", []string{"--plan", "docs/plans/x", "--repo-slug", "o/r", "stray"}, "unexpected positional"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errB bytes.Buffer
			if got := watchVerb(tc.args, &out, &errB); got != 2 {
				t.Fatalf("exit = %d, want 2 (stderr: %s)", got, errB.String())
			}
			if !strings.Contains(errB.String(), tc.want) {
				t.Fatalf("stderr %q missing %q", errB.String(), tc.want)
			}
		})
	}
}

// fakeReport writes an executable that prints human lines then the projection
// JSON — the mixed output shape plangate report actually emits.
func fakeReport(t *testing.T, dir string, items string) string {
	t.Helper()
	script := filepath.Join(dir, "fake-report.sh")
	body := "#!/bin/sh\necho 'plangate: task T1 exists'\ncat <<'EOF'\n" +
		`{"schema_version":"plan-projection-report/v1","feature":"self-drive-wake","plan_version":"001","items":[` + items + `]}` + "\nEOF\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func writePlanJSON(t *testing.T, repo string) {
	t.Helper()
	dir := filepath.Join(repo, "docs", "plans", "self-drive-wake")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{"tasks":[{"id":"T1","blocked_by":[]},{"id":"T2","blocked_by":["T1"]}]}`
	if err := os.WriteFile(filepath.Join(dir, "001-plan.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlanMappingParsesReportAndEdges(t *testing.T) {
	repo := t.TempDir()
	writePlanJSON(t, repo)
	script := fakeReport(t, repo, `{"task_id":"T1","issue":834,"status":"existing"},{"task_id":"T2","issue":835,"status":"existing"}`)

	m, err := planMapping(context.Background(), repo, "docs/plans/self-drive-wake", "gh", "", script)
	if err != nil {
		t.Fatal(err)
	}
	if m.planRef != "self-drive-wake/001" {
		t.Fatalf("planRef = %q", m.planRef)
	}
	if m.issueByTask["T1"] != 834 || m.issueByTask["T2"] != 835 {
		t.Fatalf("mapping = %+v", m.issueByTask)
	}
	if len(m.blockedBy["T2"]) != 1 || m.blockedBy["T2"][0] != "T1" {
		t.Fatalf("edges = %+v", m.blockedBy)
	}
}

func TestPlanMappingRefusesUnprojectedTask(t *testing.T) {
	repo := t.TempDir()
	writePlanJSON(t, repo)
	script := fakeReport(t, repo, `{"task_id":"T1","status":"dry-run"}`)
	_, err := planMapping(context.Background(), repo, "docs/plans/self-drive-wake", "gh", "", script)
	if err == nil || !strings.Contains(err.Error(), "no projected issue") {
		t.Fatalf("want unprojected refusal, got %v", err)
	}
}

func TestWatchVerbWiresLoopAndCompletes(t *testing.T) {
	repo := t.TempDir()
	writePlanJSON(t, repo)
	script := fakeReport(t, repo, `{"task_id":"T1","issue":834,"status":"existing"}`)

	restore := watchRunLive
	defer func() { watchRunLive = restore }()
	var gotSource watch.PlanSource
	var gotSweep watch.Sweep
	watchRunLive = func(ctx context.Context, source watch.PlanSource, reader watch.Reader, sweep watch.Sweep, clock watch.Clock, hints <-chan struct{}) (watch.Summary, error) {
		if source == nil || reader == nil || sweep == nil {
			t.Fatal("verb must wire source, reader and sweep")
		}
		gotSource, gotSweep = source, sweep
		return watch.Summary{Completed: true, Sweeps: 3}, nil
	}

	var out, errB bytes.Buffer
	code := watchVerb([]string{
		"--repo", repo, "--repo-slug", "agenticlab-ai/pyxis",
		"--plan", "docs/plans/self-drive-wake",
		"--ambient-backend", "codex",
		"--report-cmd", script, "--sweep-cmd", "true",
	}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, errB.String())
	}
	if !strings.Contains(out.String(), "watch: complete") || !strings.Contains(out.String(), "3 sweep(s)") {
		t.Fatalf("completion summary missing: %s", out.String())
	}
	if gotSource == nil || gotSweep == nil {
		t.Fatal("loop wiring not captured")
	}
	// The wake event lands before the sweep's effect (DINV-8): running the
	// wired sweep appends wake_delivered to the repo's gitignored sink.
	if err := gotSweep(context.Background()); err != nil {
		t.Fatalf("wired sweep: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repo, watch.DefaultEventsDir, "current.jsonl"))
	if err != nil {
		t.Fatalf("wake-events sink not written: %v", err)
	}
	if !strings.Contains(string(data), "wake_delivered") || !strings.Contains(string(data), "self-drive-wake/001") {
		t.Fatalf("sink line incomplete: %s", data)
	}
}

func TestSubprocessSweepCarriesAttribution(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "fake-sweep.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+capture+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sweep := subprocessSweep(sweepArgs{override: script, repo: dir, repoSlug: "agenticlab-ai/pyxis", label: "local-agent", scope: "self-drive-wake/001", ambientBackend: "codex", trackerKind: "gh"}, os.Stdout, os.Stderr)
	if err := sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--unattended", "--initiator drive-wake", "--scope self-drive-wake/001", "--plan-ref self-drive-wake/001", "--label local-agent", "--ambient-backend codex"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("sweep args %q missing %q", got, want)
		}
	}
}

func TestSubprocessReviewSweepCarriesClosedSelectedFrontier(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "fake-review-sweep.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+capture+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sweep := subprocessReviewSweep(sweepArgs{override: script, repo: dir, repoSlug: "agenticlab-ai/pyxis", label: "local-agent", scope: "self-drive-wake/001", ambientBackend: "codex", trackerKind: "gh"}, os.Stdout, os.Stderr)
	if err := sweep(context.Background(), watch.ReviewRecoveryRequest{DriveRoundID: "round-1", ObligationIDs: []string{"o-1", "o-2"}}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--review-only", "--drive-round-id round-1", "--plan-ref self-drive-wake/001", "--review-obligation o-1 --review-obligation o-2"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("review sweep args %q missing %q", got, want)
		}
	}
}

func TestWatchVerbConfiguredReviewUsesCombinedFrontier(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "-C", repo, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	writePlanJSON(t, repo)
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("executor:\n  review:\n    backend: claude\nwake:\n  review_recovery_bound: 4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reportScript := fakeReport(t, repo, `{"task_id":"T1","issue":834,"status":"existing"}`)
	capture := filepath.Join(repo, "review-args.txt")
	sweepScript := filepath.Join(repo, "fake-sweep.sh")
	if err := os.WriteFile(sweepScript, []byte("#!/bin/sh\necho \"$@\" > "+capture+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	restore := watchRunCombinedLive
	defer func() { watchRunCombinedLive = restore }()
	var gotScope string
	var gotPolicy watch.RecoveryPolicy
	var gotReviewSweep watch.ReviewSweep
	watchRunCombinedLive = func(_ context.Context, _ watch.PlanSource, _ watch.Reader, reviews watch.ReviewStateReader, policy watch.RecoveryPolicy, scope string, _ watch.Sweep, reviewSweep watch.ReviewSweep, _ watch.RoundIDSource, _ *watch.Sink, _ watch.Clock, _ <-chan struct{}) (watch.Summary, error) {
		if reviews.Provider == nil || reviews.Recovery == nil {
			t.Fatal("configured review must wire both durable state sources")
		}
		gotScope, gotPolicy, gotReviewSweep = scope, policy, reviewSweep
		return watch.Summary{Completed: true}, nil
	}

	var stdout, stderr bytes.Buffer
	code := watchVerb([]string{"--repo", repo, "--repo-slug", "agenticlab-ai/pyxis", "--plan", "docs/plans/self-drive-wake", "--report-cmd", reportScript, "--sweep-cmd", sweepScript}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("watch exit=%d stderr=%s", code, stderr.String())
	}
	if gotScope != "self-drive-wake/001" || gotPolicy.Bound != 4 || gotReviewSweep == nil {
		t.Fatalf("combined wiring scope=%q policy=%+v sweep=%v", gotScope, gotPolicy, gotReviewSweep != nil)
	}
	if err := gotReviewSweep(context.Background(), watch.ReviewRecoveryRequest{DriveRoundID: "round-1", ObligationIDs: []string{"o-1"}}); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(capture)
	if err != nil || !strings.Contains(string(args), "--review-only") {
		t.Fatalf("review subprocess args=%q err=%v", args, err)
	}
}

func TestHeadIssue(t *testing.T) {
	cases := map[string]int{
		"feat/t1-define-the-reader-port-its-typed-state-model-834":  834,
		"feat/t8-ship-the-watch-verb-executor-wiring-wake-even-841": 841,
		"main":            0,
		"feat/no-number-": 0,
		"":                0,
	}
	for head, want := range cases {
		if got := headIssue(head); got != want {
			t.Fatalf("headIssue(%q) = %d, want %d", head, got, want)
		}
	}
}

func TestResolveTracker(t *testing.T) {
	cases := []struct {
		name       string
		flag       string
		cfgTracker string
		wantKind   string
		wantErr    bool
	}{
		{"config carina", "", "carina", "carina", false},
		{"config github", "", "github", "gh", false},
		{"config empty defaults gh", "", "", "gh", false},
		{"flag overrides config", "gh", "carina", "gh", false},
		{"flag carina over gh config", "carina", "github", "carina", false},
		{"bad flag", "svn", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := repoconfig.Config{}
			cfg.Knobs.Tracker = tc.cfgTracker
			kind, root, err := resolveTracker(tc.flag, "", "/repo", cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTracker: %v", err)
			}
			if kind != tc.wantKind {
				t.Fatalf("kind = %q, want %q", kind, tc.wantKind)
			}
			if root != "/repo" { // no --carina-root, no config work_items → repo
				t.Fatalf("carinaRoot = %q, want /repo", root)
			}
		})
	}
}

func TestResolveTracker_CarinaRootFromConfig(t *testing.T) {
	cfg := repoconfig.Config{WorkItems: "/store/items"}
	cfg.Knobs.Tracker = "carina"
	_, root, err := resolveTracker("", "", "/repo", cfg)
	if err != nil || root != "/store/items" {
		t.Fatalf("carinaRoot = %q (err %v), want /store/items from config work_items", root, err)
	}
	// An explicit flag still wins.
	_, root, _ = resolveTracker("", "/explicit", "/repo", cfg)
	if root != "/explicit" {
		t.Fatalf("carinaRoot = %q, want the explicit flag /explicit", root)
	}
}

// TestSubprocessSweep_ForwardsTrackerSelection is #895's core: the tracker
// watch resolved from config reaches every sweep. A gh sweep carries
// --tracker gh and no --carina-root; a carina sweep carries --tracker carina
// AND --carina-root, so watch and its sweeps never disagree on the medium.
func TestSubprocessSweep_ForwardsTrackerSelection(t *testing.T) {
	for _, tc := range []struct {
		kind       string
		carinaRoot string
		wantArgs   []string
		absentArgs []string
	}{
		{"gh", "", []string{"--tracker gh", "--plan-ref self-drive-wake/001"}, []string{"--carina-root"}},
		{"carina", "/store", []string{"--tracker carina", "--carina-root /store"}, nil},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			dir := t.TempDir()
			capture := filepath.Join(dir, "args.txt")
			script := filepath.Join(dir, "fake-sweep.sh")
			if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+capture+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			sweep := subprocessSweep(sweepArgs{
				override: script, repo: dir, repoSlug: "agenticlab-ai/pyxis", label: "local-agent",
				scope: "self-drive-wake/001", trackerKind: tc.kind, carinaRoot: tc.carinaRoot,
			}, os.Stdout, os.Stderr)
			if err := sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.wantArgs {
				if !strings.Contains(string(got), want) {
					t.Errorf("sweep args %q missing %q", got, want)
				}
			}
			for _, absent := range tc.absentArgs {
				if strings.Contains(string(got), absent) {
					t.Errorf("gh sweep args %q must not carry %q", got, absent)
				}
			}
		})
	}
}

// TestCarinaPlanSource_DerivesStateFromStore proves the Carina loop path
// derives the plan the same shape ghPlanSource does, medium-swapped: a landed
// item is Closed; an open claimed item carries its own proposal; an open
// unclaimed item is ready (no proposal, unclaimed).
func TestCarinaPlanSource_DerivesStateFromStore(t *testing.T) {
	dir := t.TempDir()
	clock := func() time.Time { return time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC) }
	store := localfs.New(dir, clock)
	ctx := context.Background()

	// item 1: done (closed/landed); item 2: still open.
	done, defs := store.Create(ctx, model.CreateSpec{Title: "[T1] done"})
	if len(defs) > 0 {
		t.Fatalf("create T1: %v", defs)
	}
	if _, defs := store.Transition(ctx, done.ID, model.StateClosed, model.CloseCompleted, "tester"); len(defs) > 0 {
		t.Fatalf("land T1: %v", defs)
	}
	open, defs := store.Create(ctx, model.CreateSpec{Title: "[T2] open"})
	if len(defs) > 0 {
		t.Fatalf("create T2: %v", defs)
	}

	// No git remote in this temp store: substitute the claims lister (the same
	// package-var seam watchRunLive uses). Empty claims → item 2 is ready.
	restore := listClaims
	defer func() { listClaims = restore }()
	listClaims = func(context.Context, string, string) (map[int]bool, error) { return map[int]bool{}, nil }

	sr := carinatracker.NewStateReader(dir)
	m := planMap{
		issueByTask: map[string]int{"T1": done.ID, "T2": open.ID},
		blockedBy:   map[string][]string{"T1": nil, "T2": {"T1"}},
	}
	// No claims: item 2 open+unclaimed → not closed, no proposal (ready);
	// item 1 landed → closed.
	plan, err := carinaPlanSource(dir, "origin", sr, m)(ctx)
	if err != nil {
		t.Fatalf("carinaPlanSource: %v", err)
	}
	closed := map[string]bool{}
	hasProposal := map[string]bool{}
	for _, task := range plan.Tasks {
		closed[task.ID] = task.Closed
		hasProposal[task.ID] = task.Proposal != nil
	}
	if !closed["T1"] {
		t.Error("T1 (landed) should be Closed")
	}
	if closed["T2"] {
		t.Error("T2 (open) should not be Closed")
	}
	if hasProposal["T2"] {
		t.Error("T2 (open, unclaimed) should carry no proposal — it is ready-unclaimed")
	}
}
