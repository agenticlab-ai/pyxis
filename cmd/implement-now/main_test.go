package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/watch"
)

func fixtureHarnessConfig(t *testing.T) harnessconfig.Config {
	t.Helper()
	cfg, err := harnessconfigtest.Config()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// formatSummary renders the headline counts, a line per failed task with its
// reason, and the fail-closed escalation count.
func TestFormatSummary(t *testing.T) {
	rep := dispatch.RunReport{
		Results: []dispatch.TaskResult{
			{Task: port.Task{ID: "T1", Number: 1}, Class: dispatch.ClassSucceeded},
			{Task: port.Task{ID: "T2", Number: 2}, Class: dispatch.ClassFailed, Err: dispatch.ErrRespawnCapExhausted},
			{Task: port.Task{ID: "T3", Number: 3}, Class: dispatch.ClassBlocked},
		},
	}
	rep.Succeeded, rep.Failed, rep.Blocked, rep.FeedbackRounds = 1, 1, 1, 2

	out := formatSummary("run-abc", rep, 1)
	for _, want := range []string{
		"run run-abc finished",
		"succeeded: 1", "failed:    1", "blocked:   1", "skipped:   0",
		"FAILED T2 (#2)", "respawn cap exhausted",
		"fail-closed escalations: 1",
		"feedback-rounds: 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q\n---\n%s", want, out)
		}
	}
}

func TestReviewFailureMakesRunExitNonZero(t *testing.T) {
	if got := runExitCode(dispatch.RunReport{Succeeded: 1, ReviewFailed: 1}); got != 1 {
		t.Fatalf("review-failed exit = %d, want 1", got)
	}
	if got := runExitCode(dispatch.RunReport{Succeeded: 1, ReviewCompleted: 1}); got != 0 {
		t.Fatalf("review-completed exit = %d, want 0", got)
	}
}

func TestReviewOnlyRequiresClosedNonEmptySelection(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	var stdout, stderr bytes.Buffer
	got := run([]string{"--repo-slug", "owner/repo", "--review-only"}, &stdout, &stderr)
	if got != 2 || !strings.Contains(stderr.String(), "requires --plan-ref") {
		t.Fatalf("exit=%d stderr=%q", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	got = run([]string{"--repo-slug", "owner/repo", "--review-obligation", "o-1"}, &stdout, &stderr)
	if got != 2 || !strings.Contains(stderr.String(), "require --review-only") {
		t.Fatalf("exit=%d stderr=%q", got, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(cwd, ".pyxis")); !os.IsNotExist(err) {
		t.Fatalf("usage error wrote logs under cwd: %v", err)
	}
}

func TestResolveRepoDefaultFindsConfigRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "cmd", "implement-now")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	got, err := resolveRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err = filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolveRepo(.) from nested cwd = %q, want config root %q", got, want)
	}
}

func TestResolveRepoExplicitStaysExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolveRepo(%q) = %q, want %q", dir, got, want)
	}
}

func TestRunReportsRepoDiscoveryFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{"--repo-slug", "owner/repo"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--repo:") {
		t.Fatalf("run = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestFormatSummaryReviewFailureClasses(t *testing.T) {
	rep := dispatch.RunReport{ReviewFailed: 3, ReviewPending: 3, ReviewArtifactUnverified: 1, ReviewArtifactUntrusted: 1, ReviewInfrastructureFailed: 1}
	out := formatSummary("review-run", rep, 3)
	for _, want := range []string{"review-pending:   3", "review-artifact-unverified: 1", "review-artifact-untrusted: 1", "review-infrastructure-failed: 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "nothing ready") {
		t.Fatal("selected review recovery rendered as empty frontier")
	}
}

func TestFormatSummaryNamesProposalProbeAndReviewFailures(t *testing.T) {
	rep := dispatch.RunReport{
		AwaitingProposal: 1,
		ProbeUnavailable: 1,
		ReviewFailed:     1,
		Results: []dispatch.TaskResult{
			{Task: port.Task{ID: "T1", Number: 1}, Class: dispatch.ClassAwaitingProposal, Err: errors.New("proposal open")},
			{Task: port.Task{ID: "T2", Number: 2}, Class: dispatch.ClassProbeUnavailable, Err: errors.New("probe stopped")},
			{Task: port.Task{ID: "T3", Number: 3}, Review: &worker.ReviewResult{Harness: "codex", Phase: worker.ReviewPhaseArtifact, Status: report.ReviewStatusFailed, Err: errors.New("review stopped")}},
		},
	}
	out := formatSummary("boundaries", rep, 0)
	for _, want := range []string{"awaiting-proposal: 1", "probe-unavailable: 1", "AWAITING-PROPOSAL T1", "PROBE-UNAVAILABLE T2", "REVIEW-FAILED T3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
}

// An empty-frontier run summarizes as nothing-ready (the zero-cost exit).
func TestFormatSummary_NothingReady(t *testing.T) {
	out := formatSummary("run-empty", dispatch.RunReport{}, 0)
	if !strings.Contains(out, "nothing ready") {
		t.Errorf("empty report summary should note nothing-ready:\n%s", out)
	}
	if strings.Contains(out, "escalations") {
		t.Errorf("no escalations should be reported for a clean empty run:\n%s", out)
	}
}

// buildSinks always wires the operator and readable flow views, and adds the
// argus journal sink only when its writer is present.
func TestBuildSinks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	var flow bytes.Buffer

	// Attended (no machine journal): operator + readable flow log.
	sinks, argus := buildSinks(logger, nil, &flow)
	if len(sinks) != 2 {
		t.Fatalf("attended sinks = %d, want 2", len(sinks))
	}
	if _, ok := sinks[0].(*report.OperatorSink); !ok {
		t.Errorf("attended sink 0 = %T, want *report.OperatorSink", sinks[0])
	}
	if argus != nil {
		t.Error("attended mode must not build an argus sink")
	}

	if _, ok := sinks[1].(*report.FlowSink); !ok {
		t.Errorf("attended sink 1 = %T, want *report.FlowSink", sinks[1])
	}
	// Unattended: operator + flow + argus.
	var journal bytes.Buffer
	sinks, argus = buildSinks(logger, &journal, &flow)
	if len(sinks) != 3 {
		t.Fatalf("unattended sinks = %d, want 3", len(sinks))
	}
	if argus == nil {
		t.Fatal("unattended mode must build an argus sink")
	}
	if _, ok := sinks[2].(*report.ArgusSink); !ok {
		t.Errorf("unattended sink 2 = %T, want *report.ArgusSink", sinks[2])
	}
	if err := argus.Deliver(report.TaskOutcome("T1", 1, "failed", true, "stuck")); err != nil {
		t.Fatal(err)
	}
	if argus.Escalations() != 1 {
		t.Fatalf("escalations = %d, want 1", argus.Escalations())
	}
}

func TestRunObserversExposeDeliveryAndBudgetPressure(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	reportDeliveryErrorLogger(logger)(nil, report.TaskOutcome("T1", 1, "failed", true, "stuck"), errors.New("sink stopped"))

	events := watch.NewSink(filepath.Join(t.TempDir(), "events"), 0, nil)
	observe := contentBudgetObserver(logger, events)
	observe(budget.Event{Granted: true, QueueDepth: 2, Deferred: time.Second})
	reset := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	observe(budget.Event{Ledger: budget.LedgerUmbrella, QueueDepth: 3, Deferred: 2 * time.Minute, ResetAt: reset})

	for _, want := range []string{"event delivery failed", "proceeding after backpressure", "content write deferred"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("observer log missing %q: %s", want, logs.String())
		}
	}
}

// The runtime resolver routes a task through config.ResolveHarness to a registered
// backend runtime, reporting the resolved backend name. With no repo override,
// the invoking harness supplies the default.
func TestNewRuntimeResolver_InheritsAmbientBackend(t *testing.T) {
	resolve := newRuntimeResolver(context.Background(), config.Defaults(), "codex", fixtureHarnessConfig(t), nil)
	backend, rt, err := resolve(port.Task{
		ID:            "T1",
		DemandID:      executiondemand.DemandID(strings.Repeat("a", 64)),
		DemandProfile: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityStandard, Deliberation: executiondemand.DeliberationStandard, Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceOrdinary},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if backend != "codex" {
		t.Errorf("resolved backend = %q, want ambient codex", backend)
	}
	if rt == nil {
		t.Error("resolved a nil runtime for the default backend")
	}
	route, err := port.RouteOf(rt)
	if err != nil {
		t.Fatal(err)
	}
	if route.Model == "" || route.Effort == "" {
		t.Errorf("route omitted the native model or effort: %+v", route)
	}
}

func TestResolveAmbientBackend(t *testing.T) {
	getenv := func(string) string { return "codex" }
	if got := resolveAmbientBackend(" cursor ", getenv); got != "cursor" {
		t.Fatalf("flag precedence = %q, want cursor", got)
	}
	if got := resolveAmbientBackend("", getenv); got != "codex" {
		t.Fatalf("environment fallback = %q, want codex", got)
	}
}

// buildBackend constructs each self-registered backend (claude threads the native
// turn cap; the rest resolve through the registry) and fails loudly on an unknown
// one.
func TestBuildBackend(t *testing.T) {
	cfg := config.Defaults()
	hc := fixtureHarnessConfig(t)
	for _, backend := range []entity.Harness{entity.Claude, entity.Codex, entity.Cursor, entity.OpenCode} {
		rt, err := buildBackend(backend, cfg, hc)
		if err != nil {
			t.Errorf("buildBackend(%q): %v", backend, err)
		}
		if rt == nil {
			t.Errorf("buildBackend(%q): nil runtime", backend)
		}
	}
	if _, err := buildBackend("nonexistent", cfg, hc); err == nil {
		t.Error("buildBackend(nonexistent) must error, not return a half-built runtime")
	}
}

// codeGateRunner runs the code-gate verb in-process over a workdir: a config
// whose checks pass yields Passed; a failing required check yields not-Passed with
// the failure captured for the fixer (a check failure, not a run error); a
// malformed config or a cancelled run fails the gate closed via Err (never a false
// pass); an unconfigured code-gate vacuously passes.
func TestCodeGateRunner(t *testing.T) {
	ctx := context.Background()
	t.Run("scratch unavailable", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
		if result := codeGateRunner(t.Context(), dir, nil); result.Err == nil || result.Passed {
			t.Fatalf("scratch failure accepted: %+v", result)
		}
	})

	pass := codeGateWorkdir(t, "code-gate:\n  ok:\n    command: true\n")
	if res := codeGateRunner(ctx, pass, nil); !res.Passed || res.Err != nil {
		t.Errorf("passing gate: got %+v, want Passed with no Err", res)
	}

	fail := codeGateWorkdir(t, "code-gate:\n  must:\n    command: false\n")
	res := codeGateRunner(ctx, fail, nil)
	if res.Passed || res.Err != nil {
		t.Errorf("failing gate: got %+v, want not-Passed and no Err (a check failure)", res)
	}
	if !strings.Contains(res.Output, "must") {
		t.Errorf("failing gate output = %q, want the failing check named for the fixer", res.Output)
	}

	bad := codeGateWorkdir(t, "code-gate:\n  x:\n    command: true\n    bogus-key: y\n")
	if res := codeGateRunner(ctx, bad, nil); res.Err == nil {
		t.Errorf("malformed config: got %+v, want a run error (fail closed)", res)
	}

	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res := codeGateRunner(cctx, pass, nil); res.Err == nil {
		t.Errorf("cancelled: want a run error (fail closed), got %+v", res)
	}

	empty := codeGateWorkdir(t, "system:\n  mode: source\n")
	if res := codeGateRunner(ctx, empty, nil); !res.Passed || res.Err != nil {
		t.Errorf("unconfigured gate: got %+v, want a vacuous pass", res)
	}
}

// codeGateWorkdir creates a temp workdir holding .pyxis/config.yaml with body, for
// codeGateRunner to load.
func codeGateWorkdir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".pyxis"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pyxis", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

// Both execution modes retain the default append-only event journal.
func TestOpenJournalFor(t *testing.T) {
	// Attended, no explicit path: retain the default journal.
	w, closeFn, err := openJournalFor("", t.TempDir())
	if err != nil {
		t.Fatalf("attended: %v", err)
	}
	if w == nil {
		t.Error("attended runs must retain their event journal")
	}
	closeFn() // must be safe to call

	// Unattended: the default journal path is created under the repo.
	repo := t.TempDir()
	w, closeFn, err = openJournalFor("", repo)
	if err != nil {
		t.Fatalf("unattended: %v", err)
	}
	if w == nil {
		t.Fatal("unattended must open a journal writer")
	}
	defer closeFn()
	if _, err := os.Stat(filepath.Join(repo, defaultJournalRel)); err != nil {
		t.Errorf("default journal file was not created: %v", err)
	}
}

// A run with no --repo-slug is a usage error (exit 2), never a silent default.
func TestRun_RequiresRepoSlug(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--repo", t.TempDir()}, &stdout, &stderr); code != 2 {
		t.Errorf("run without --repo-slug = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "repo-slug") {
		t.Errorf("stderr should name the missing --repo-slug:\n%s", stderr.String())
	}
}

func TestResolveCarinaBinaryKeepsAnExplicitOverride(t *testing.T) {
	got, err := resolveCarinaBinary(t.Context(), t.TempDir(), "/explicit/carina")
	if err != nil || got != "/explicit/carina" {
		t.Fatalf("explicit override = %q, %v", got, err)
	}
	if _, err := resolveCarinaBinary(t.Context(), t.TempDir(), ""); err == nil {
		t.Fatal("empty checkout without carina succeeded")
	}
}

func TestRun_RefusesInvalidPlanScopeBeforeWiring(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "malformed plan ref, gh",
			args: []string{"--repo-slug", "agenticlab-ai/pyxis", "--plan-ref", "routing/002/T1"},
			want: "invalid plan reference",
		},
		{
			// Carina is no longer blanket-rejected: it goes through the SAME
			// tracker-neutral scope validation, so a malformed ref refuses with
			// the identical message (the reason plan scope now works for both).
			name: "malformed plan ref, carina",
			args: []string{"--repo-slug", "agenticlab-ai/pyxis", "--tracker", "carina", "--plan-ref", "routing/002/T1"},
			want: "invalid plan reference",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != 2 {
				t.Fatalf("exit = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr %q missing %q", stderr.String(), tc.want)
			}
		})
	}
}

func TestStringListIsClosedAndPrintable(t *testing.T) {
	var values stringList
	if err := values.Set(" first "); err != nil {
		t.Fatal(err)
	}
	if err := values.Set("second"); err != nil {
		t.Fatal(err)
	}
	if got := values.String(); got != "first,second" {
		t.Fatalf("String = %q", got)
	}
	if err := values.Set("  "); err == nil {
		t.Fatal("empty list value succeeded")
	}
}

func TestRunReportsConfigHarnessAndJournalWiringFailures(t *testing.T) {
	writeConfig := func(t *testing.T, body string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "executor", body: "executor:\n  unknown: value\n", want: "load executor config"},
		{name: "harness", body: "harness-config:\n  unknown-harness:\n    models:\n", want: "load harness-config"},
		{name: "unknown-key", body: "harness-config:\n  codex:\n    surprise: value\n", want: "unknown key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeConfig(t, tc.body)
			var stdout, stderr bytes.Buffer
			if code := run([]string{"--repo", root, "--repo-slug", "owner/repo"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("run = %d stderr=%q", code, stderr.String())
			}
		})
	}

	root := writeConfig(t, "")
	parentFile := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parentFile, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--repo", root, "--repo-slug", "owner/repo", "--journal", filepath.Join(parentFile, "journal.jsonl")}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "open argus journal") {
		t.Fatalf("journal run = %d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown flag run = %d", code)
	}
}

func TestRunRefusesUnwritableOperatorLog(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Dir(filepath.Join(root, defaultJournalRel))
	if err := os.MkdirAll(filepath.Dir(blocked), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocked, []byte("existing content"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--repo", root, "--repo-slug", "owner/repo"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "create log directory") {
		t.Fatalf("run = %d stderr=%q", code, stderr.String())
	}
	if data, err := os.ReadFile(blocked); err != nil || string(data) != "existing content" {
		t.Fatalf("logging refusal changed existing content: %q %v", data, err)
	}
}

func TestOpenJournalFailureShapes(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parentFile, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, closeFn, err := openJournalFor(filepath.Join(parentFile, "journal"), t.TempDir()); err == nil {
		closeFn()
		t.Fatal("journal below a file succeeded")
	}
	repo := t.TempDir()
	dir := filepath.Join(repo, defaultJournalRel)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, closeFn, err := openJournalFor(dir, repo); err == nil || !strings.Contains(err.Error(), "open journal") {
		closeFn()
		t.Fatalf("directory did not produce a journal open failure: %v", err)
	}
	explicit := filepath.Join(repo, ".pyxis/logs/executor/explicit.jsonl")
	w, closeFn, err := openJournalFor(explicit, repo)
	if err != nil || w == nil {
		t.Fatalf("explicit journal = %T, %v", w, err)
	}
	closeFn()
}

func TestEmitReviewResultAndCounts(t *testing.T) {
	emitter := &capturingEmitter{}
	errReview := worker.ReviewResult{DriveRoundID: "round-1", AttemptID: "attempt-1", ObligationID: "obligation-1", Harness: "codex",
		ProposalRef: "12", Revision: "abc", Phase: worker.ReviewPhaseArtifact, Status: report.ReviewStatusFailed,
		ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureArtifactUnverified, Err: errors.New("artifact missing")}
	emitReviewResult(emitter, errReview, "T1")
	if len(emitter.events) != 1 || emitter.events[0].Reason != "artifact missing" {
		t.Fatalf("failed review events = %+v", emitter.events)
	}

	emitter.events = nil
	complete := errReview
	complete.Err = nil
	complete.Publication = &worker.ReviewPublicationTerminal{ReviewAttemptID: "attempt-1", ReviewObligationID: "obligation-1", ProposalRef: "12", Revision: "abc", Status: "verified", ArtifactRef: "artifact", NativeReceipt: "receipt", AttemptProofRef: "proof"}
	complete.ImplReviewTerminal = &implreview.PublicationTerminal{ReviewProcessID: "process-1", ReviewObligationID: "obligation-1", PublicationAttemptID: "publication-1", ProposalID: "12", ReviewedRevision: "abc", Medium: "github", IdempotencyID: "idem", Status: implreview.PublicationVerified, ArtifactRef: "artifact", NativeReceipt: "receipt", AttemptProofRef: "proof"}
	emitReviewResult(emitter, complete, "T1")
	if len(emitter.events) != 3 {
		t.Fatalf("complete review events = %d, want 3", len(emitter.events))
	}

	rep := dispatch.RunReport{Tokens: 17, ReviewCompleted: 1, ReviewFailed: 2, ReviewPending: 2, ReviewArtifactUnverified: 1, ReviewArtifactUntrusted: 1, ReviewInfrastructureFailed: 0}
	counts := reviewCounts(rep)
	if counts.Tokens != 17 || counts.ReviewCompleted != 1 || counts.ReviewFailed != 2 || counts.ReviewPending != 2 || counts.ReviewArtifactUnverified != 1 || counts.ReviewArtifactUntrusted != 1 {
		t.Fatalf("reviewCounts = %+v", counts)
	}
}

func TestRunSelectedReviewsReportsStoreWiringFailure(t *testing.T) {
	emitter := &capturingEmitter{}
	var stdout, stderr bytes.Buffer
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runSelectedReviews(t.Context(), emitter, &stdout, &stderr, config.Defaults(), fixtureHarnessConfig(t), "", "owner/repo", "", "main", "feature/001", nil, "round-1", []string{"o-1", "o-2"}, logger, nil)
	if code != 1 || !strings.Contains(stderr.String(), "review store") || !strings.Contains(stdout.String(), "review-failed:    2") {
		t.Fatalf("runSelectedReviews = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if len(emitter.events) != 4 {
		t.Fatalf("events = %d, want run start + two failures + finish", len(emitter.events))
	}
}

func TestRunSelectedReviewsValidatesDurableSelectionBeforeLaunching(t *testing.T) {
	repo := implementReviewGitFixture(t)
	store, err := reviewobligation.NewGitStore(repo, "origin", reviewobligation.DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	obligation := reviewobligation.Obligation{SchemaVersion: reviewobligation.Schema, ID: "o-1", Repository: "owner/repo",
		Scope: "feature/001", TaskID: "T1", TaskNumber: 1, ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "codex"}
	if err := store.Append(t.Context(), obligation); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(t.Context(), obligation.ID); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runOne := func(scope string, selected []string) (int, string, string, *capturingEmitter) {
		t.Helper()
		emitter := &capturingEmitter{}
		var stdout, stderr bytes.Buffer
		code := runSelectedReviews(t.Context(), emitter, &stdout, &stderr, config.Defaults(), fixtureHarnessConfig(t), repo,
			"owner/repo", "origin", "main", scope, nil, "round-1", selected, logger, nil)
		return code, stdout.String(), stderr.String(), emitter
	}

	if code, stdout, stderr, emitter := runOne("feature/001", nil); code != 0 || stderr != "" || !strings.Contains(stdout, "nothing ready") || len(emitter.events) != 2 {
		t.Fatalf("empty recovery = %d stdout=%q stderr=%q events=%d", code, stdout, stderr, len(emitter.events))
	}
	if code, _, stderr, _ := runOne("feature/001", []string{"missing"}); code != 1 || !strings.Contains(stderr, "unavailable or out of scope") {
		t.Fatalf("missing selection = %d stderr=%q", code, stderr)
	}
	if code, _, stderr, _ := runOne("feature/other", []string{"o-1"}); code != 1 || !strings.Contains(stderr, "out of scope") {
		t.Fatalf("cross-scope selection = %d stderr=%q", code, stderr)
	}
	if code, _, stderr, _ := runOne("feature/001", []string{"o-1", "o-1"}); code != 2 || !strings.Contains(stderr, "duplicate review obligation") {
		t.Fatalf("duplicate selection = %d stderr=%q", code, stderr)
	}
	if code, stdout, stderr, emitter := runOne("feature/001", []string{"o-1"}); code != 1 || stderr != "" || !strings.Contains(stdout, "review-failed:    1") || len(emitter.events) != 3 {
		t.Fatalf("unconfigured reviewer = %d stdout=%q stderr=%q events=%d", code, stdout, stderr, len(emitter.events))
	}

	pairObligation := obligation
	pairObligation.ID, pairObligation.TaskID, pairObligation.ProposalRef = "o-pair", "T2", "github:owner/repo#2"
	if err := store.Append(t.Context(), pairObligation); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(t.Context(), pairObligation.ID); err != nil {
		t.Fatal(err)
	}
	attempt := reviewobligation.Attempt{SchemaVersion: reviewobligation.AttemptSchema, ObligationID: pairObligation.ID,
		AttemptID: "attempt-pair", RunID: "round-0", AttemptProofRef: "attempt-proof"}
	lease, err := store.Begin(t.Context(), attempt)
	if err != nil {
		t.Fatal(err)
	}
	pair := reviewobligation.PublicationPair{SchemaVersion: reviewobligation.PublicationPairSchema, ObligationID: pairObligation.ID,
		AttemptID: attempt.AttemptID, PublicationAttemptID: "publication-1", ProposalRef: pairObligation.ProposalRef,
		Revision: pairObligation.Revision, Medium: "github", IdempotencyID: "idem-1", ArtifactRef: "artifact-1",
		NativeReceipt: "native-receipt", AttemptProofRef: attempt.AttemptProofRef, EVT13Ref: "evt-13", EVT17Ref: "evt-17"}
	if err := store.RecordPair(t.Context(), pair); err != nil {
		_ = lease.Close()
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	state, ok, err := store.State(t.Context(), pairObligation.ID)
	if err != nil || !ok || state.Status != "pair_verified" {
		t.Fatalf("pair state = %+v, %v, %v", state, ok, err)
	}
	if code, stdout, _, emitter := runOne("feature/001", []string{pairObligation.ID}); code != 1 || !strings.Contains(stdout, "review-failed:    1") || len(emitter.events) != 3 {
		t.Fatalf("pair recovery = %d stdout=%q events=%d", code, stdout, len(emitter.events))
	}
}

type stoppedProposalInspector struct{}

func (stoppedProposalInspector) InspectProposal(context.Context, port.Proposal) (port.Landing, string, port.CheckState, error) {
	return 0, "", 0, errors.New("proposal inspection stopped")
}

func TestRunSelectedReviewsExercisesConfiguredInvokerFailure(t *testing.T) {
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement, Review: true})
	cfg, err := config.Load(fixture.repo)
	if err != nil {
		t.Fatal(err)
	}
	store, err := reviewobligation.NewGitStore(fixture.repo, "origin", cfg.ReviewEvidenceRef())
	if err != nil {
		t.Fatal(err)
	}
	obligation := reviewobligation.Obligation{
		SchemaVersion: reviewobligation.Schema, ID: "review-selected", Repository: e2eRepository,
		Scope: "feature/001", TaskID: "T1", TaskNumber: 1, ProposalRef: "github:owner/repository#77",
		Revision: "abc", Harness: "codex",
	}
	if err := store.Append(t.Context(), obligation); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(t.Context(), obligation.ID); err != nil {
		t.Fatal(err)
	}

	emitter := &capturingEmitter{}
	var stdout, stderr bytes.Buffer
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runSelectedReviews(t.Context(), emitter, &stdout, &stderr, cfg, fixtureHarnessConfig(t), fixture.repo,
		e2eRepository, "origin", "main", obligation.Scope, stoppedProposalInspector{}, "round-1", []string{obligation.ID}, logger, nil)
	if code != 1 || !strings.Contains(stdout.String(), "review-failed:    1") {
		t.Fatalf("configured recovery = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func implementReviewGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	repo := filepath.Join(root, "repo")
	runRoutingGit(t, root, "init", "--bare", "-q", remote)
	runRoutingGit(t, root, "init", "-q", repo)
	runRoutingGit(t, repo, "config", "user.name", "Pyxis Test")
	runRoutingGit(t, repo, "config", "user.email", "pyxis@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runRoutingGit(t, repo, "add", "README")
	runRoutingGit(t, repo, "commit", "-qm", "base")
	runRoutingGit(t, repo, "remote", "add", "origin", remote)
	runRoutingGit(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
	return repo
}

type capturingEmitter struct{ events []report.Event }

func (e *capturingEmitter) Emit(event report.Event) { e.events = append(e.events, event) }
