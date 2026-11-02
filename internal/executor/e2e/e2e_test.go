package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/drivetogreen"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/executor/workspace"
)

// fixtureConfig is a REAL executor config parsed through the closed-schema parser
// (config.Parse): a default backend, a routing-table entry (leaf -> codex, to
// prove per-task routing composes), and the caps. Unknown keys would be a parse
// error — the config surface is exercised for real, not stubbed.
const fixtureConfig = `executor:
  routing:
    default: claude
    table:
      leaf: codex
  caps:
    worker_turns: 40
    ci_self_heal_attempts: 3
    parallel_width: 2
`

// harness wires the REAL engine — config, dispatch.Dispatcher, worker.Worker,
// drivetogreen.Driver, report.Reporter + both real sinks — exactly as
// cmd/implement-now's run.go wires production, over a REAL git world. Three of
// the five ports are the production adapters running unmodified (the git-worktree
// Workspace, the git-ref CAS Lease, and — at the drive-to-green seam — a real git
// Pusher); only GitHub and the coding-agent CLI are stubbed. One Dispatcher + one
// Reporter serve every sweep (the Dispatcher carries no per-run state — the driver
// re-invokes Sweep across waves, §3.6).
type harness struct {
	git     *gitWorld
	tracker *memTracker
	lease   *countingLease
	space   *workspace.Workspace
	host    *gitHost
	runtime *scriptRuntime
	pusher  *gitPusher

	cfg   config.Config
	disp  *dispatch.Dispatcher
	runID string

	opBuf     bytes.Buffer // operator live view (slog text)
	journal   bytes.Buffer // argus JSONL delivery journal
	argus     *report.ArgusSink
	escalated []report.Event // fail-closed escalations the argus handler surfaced
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	cfg, err := config.Parse([]byte(fixtureConfig))
	if err != nil {
		t.Fatalf("parse fixture config: %v", err)
	}

	world := newGitWorld(t)
	h := &harness{
		git:     world,
		tracker: &memTracker{},
		lease:   world.newLease(t),
		space:   world.newWorkspace(t),
		cfg:     cfg,
		runID:   report.NewRunID(),
	}
	h.host = newGitHost(world.origin)
	h.runtime = &scriptRuntime{}
	h.pusher = &gitPusher{remote: world.remote}

	// The two REAL sinks over ONE event stream (§3.9, AP-2): the operator live view
	// (slog text) and the argus append-only JSONL journal with a fail-closed
	// escalation handler.
	op := report.NewOperatorSink(slog.New(slog.NewTextHandler(&h.opBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	h.argus = report.NewArgusSink(&h.journal, report.WithEscalationHandler(func(e report.Event) {
		h.escalated = append(h.escalated, e)
	}))
	reporter := report.NewReporter(h.runID, op, h.argus)

	// The per-task backend resolver: REAL config.ResolveHarness selects the backend
	// name; the shared fake runtime stands in for every backend's fresh process.
	resolver := func(task port.Task) (entity.Harness, port.AgentRuntime, error) {
		backend, err := h.cfg.ResolveHarness(task, "claude")
		if err != nil {
			return "", nil, err
		}
		return backend, h.runtime, nil
	}

	d, err := dispatch.New(
		dispatch.Deps{Tracker: h.tracker, Lease: h.lease, Space: h.space, Host: h.host},
		dispatch.Config{Width: h.cfg.ParallelWidth(), WorkerBudget: time.Minute},
		dispatch.WithRuntimeResolver(resolver),
		dispatch.WithGreenDriverFactory(newGreenFactory(h, reporter)),
		dispatch.WithReporter(reporter),
	)
	if err != nil {
		t.Fatalf("build dispatcher: %v", err)
	}
	h.disp = d
	return h
}

// newGreenFactory builds the Dispatcher's drive-to-green factory, wiring a REAL
// drivetogreen.Driver per task (the same composition cmd/implement-now uses,
// inspector included), bounded by the config's CI-self-heal cap, over a real local
// gate that mirrors CI and a REAL git re-push. Each self-heal attempt is emitted
// onto the one event stream through the reporter (§3.4). The *drivetogreen.Driver
// satisfies worker.GreenDriver, so it converts to the factory's interface return
// at return.
func newGreenFactory(h *harness, reporter report.Emitter) dispatch.GreenDriverFactory {
	return func(rt port.AgentRuntime) (worker.GreenDriver, error) {
		return drivetogreen.New(drivetogreen.Deps{
			Host: h.host,
			// The code host is the proposal inspector too — the production
			// arrangement — so the loop's head-identity fence (incident I10) runs
			// on the branch's real commit SHAs.
			Inspector: h.host,
			Runtime:   rt,
			Fetcher:   cannedLogFetcher("CI failed on required check `build`\nleaked token=ghp_0123456789abcdef0123456789abcdef0123"),
			Pusher:    h.pusher,
			Runner:    fileGateRunner,
		}, drivetogreen.Config{
			Cap:        h.cfg.CISelfHealAttempts(),
			GateChecks: []drivetogreen.Check{{Name: "impl-green", Argv: []string{"impl-green-check"}}},
			OnAttempt: func(e drivetogreen.AttemptEvent) {
				// The production mapping: a local-gate fix before any push is its
				// own event kind; a remote self-heal attempt stays ci_fix_attempt.
				if e.Local {
					reporter.Emit(report.LocalGateAttempt(e.TaskID, e.Attempt, e.Cap))
					return
				}
				reporter.Emit(report.CIFixAttempt(e.TaskID, e.Attempt, e.Cap))
			},
		})
	}
}

// sweep runs one real frontier-sweep and returns its classed report.
func (h *harness) sweep(t *testing.T) dispatch.RunReport {
	t.Helper()
	rep, err := h.disp.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	return rep
}

// foundationAndLeaves is the fixture plan: a foundation (T1, #1) and two leaves
// gated behind it (T2 #2, T3 #3, both BlockedBy #1). T3 is routed to codex (its
// "leaf" category hits the routing table) and starts RED, so the real
// local convergence loop must fix it before its first push; T1 and T2 implement
// cleanly to green.
func foundationAndLeaves() (foundation, leafA, leafB port.Task) {
	foundation = port.Task{
		ID: "T1", Number: 1, Title: "foundation", Harness: "",
		HeadBranch: "feat/T1-foundation", Brief: "Implement the foundation module. [[e2e:T1:green]]",
	}
	leafA = port.Task{
		ID: "T2", Number: 2, Title: "leaf A", Harness: "", BlockedBy: []int{1},
		HeadBranch: "feat/T2-leafA", Brief: "Implement leaf A on the foundation. [[e2e:T2:green]]",
	}
	leafB = port.Task{
		ID: "T3", Number: 3, Title: "leaf B", Harness: "leaf", BlockedBy: []int{1},
		HeadBranch: "feat/T3-leafB", Brief: "Implement leaf B on the foundation. [[e2e:T3:redfix]]",
	}
	return
}

// TestHermeticFullStack_FoundationThenLeavesToGreen is the headline drill: a
// multi-task fixture plan driven through real frontier-sweeps to green proposals,
// proving the whole stack composes end to end — foundation first, then the leaves
// (one fixed locally by the real convergence loop before its first push), a correct classed report at every
// sweep, the event stream on both sinks, and an idempotent re-invocation.
func TestHermeticFullStack_FoundationThenLeavesToGreen(t *testing.T) {
	h := newHarness(t)
	foundation, leafA, leafB := foundationAndLeaves()

	// --- Sweep 1: only the foundation is ready; both leaves are gated. ----------
	h.tracker.setFrontier(foundation, leafA, leafB)
	rep1 := h.sweep(t)

	if rep1.Succeeded != 1 || rep1.Blocked != 2 || rep1.Failed != 0 || rep1.Skipped != 0 {
		t.Fatalf("sweep 1 counts = %+v, want 1 succeeded / 2 blocked", rep1)
	}
	if c := classOf(t, rep1, 1); c != dispatch.ClassSucceeded {
		t.Errorf("foundation class = %v, want succeeded", c)
	}
	for _, n := range []int{2, 3} {
		if c := classOf(t, rep1, n); c != dispatch.ClassBlocked {
			t.Errorf("leaf #%d class after sweep 1 = %v, want blocked", n, c)
		}
	}
	// Foundation strictly before the leaves: exactly one fresh process ran, and the
	// gated leaves consumed no model call (DINV-8 for the blocked set).
	if got := h.runtime.launchCount(); got != 1 {
		t.Fatalf("model calls after sweep 1 = %d, want 1 (foundation only, leaves gated)", got)
	}
	h.requirePublishedGreen(t, "feat/T1-foundation")
	// The gated leaves were never claimed, so no branch of theirs exists on the
	// remote at all — blocked work leaves no git trace (DINV-8).
	for _, branch := range []string{"feat/T2-leafA", "feat/T3-leafB"} {
		if _, ok := h.git.branchHead(t, branch); ok {
			t.Errorf("gated leaf branch %s exists on the remote after sweep 1; blocked work must publish nothing", branch)
		}
	}

	// --- The driver lands the foundation; re-invoke. ----------------------------
	h.tracker.land(1)
	launchesBeforeLeaves := h.runtime.launchCount()

	rep2 := h.sweep(t)
	if rep2.Succeeded != 2 || rep2.Blocked != 0 || rep2.Failed != 0 || rep2.Skipped != 0 {
		t.Fatalf("sweep 2 counts = %+v, want 2 succeeded", rep2)
	}
	for _, n := range []int{2, 3} {
		if c := classOf(t, rep2, n); c != dispatch.ClassSucceeded {
			t.Errorf("leaf #%d class after sweep 2 = %v, want succeeded", n, c)
		}
	}
	// Both leaves reached a genuinely-green proposal; T3 got there through the
	// real bounded LOCAL fixer loop (its tree started red, and the local gate —
	// the CI mirror — caught it before the first push). "Green" here is the
	// artifact on the branch's real published head — the same object CI would
	// fetch.
	for _, branch := range []string{"feat/T2-leafA", "feat/T3-leafB"} {
		h.requirePublishedGreen(t, branch)
	}
	// The leaves ran only AFTER the foundation landed (foundation-first ordering),
	// and T3's red tree drove at least one local fix attempt — and zero remote
	// self-heal attempts, because the tree that reached the code host was
	// already green.
	if h.runtime.launchCount() <= launchesBeforeLeaves {
		t.Errorf("no fresh process ran for the leaves after landing the foundation")
	}
	localFixes := h.countJournalKind(t, report.KindLocalGateAttempt)
	if localFixes < 1 {
		t.Errorf("local_gate_attempt events = %d, want >= 1 (T3 started red and was fixed before its first push)", localFixes)
	}
	if remoteFixes := h.countJournalKind(t, report.KindCIFixAttempt); remoteFixes != 0 {
		t.Errorf("ci_fix_attempt events = %d, want 0 (the local gate mirrors CI, so a locally-fixed tree needs no remote heal)", remoteFixes)
	}

	// Across the whole run, three distinct proposals — one per task — all green.
	if got := h.host.distinctProposals(); got != 3 {
		t.Fatalf("distinct proposals = %d, want 3 (one per task)", got)
	}
	// Every claim was released on every terminal path (fire-or-release, DINV-5):
	// three tasks ran across the two sweeps, so acquires == releases — and the
	// shared remote agrees, carrying no leftover claim ref. The counters describe
	// real ref CAS operations; this second read is what proves it.
	if a, r := h.lease.acquireCount(), h.lease.releaseCount(); a != r || a < 3 {
		t.Errorf("claims acquired/released = %d/%d, want equal and >= 3 (fire-or-release)", a, r)
	}
	if refs := h.git.claimRefs(t); len(refs) != 0 {
		t.Errorf("claim refs left on the remote = %v, want none (every claim released, DINV-5)", refs)
	}
	// Every task branch was cut from the freshly-fetched mainline tip — never from
	// a sibling's branch, so no stacked proposal was created (DINV-2, AP-26).
	h.assertBranchesCutFromMainline(t, "feat/T1-foundation", "feat/T2-leafA", "feat/T3-leafB")
	// Each opened proposal posted a progress comment on its own issue.
	commented := h.tracker.commentedIssues()
	for _, n := range []int{1, 2, 3} {
		if !commented[n] {
			t.Errorf("no progress comment posted on issue #%d", n)
		}
	}
	// The charter and the resolved model and effort reached every fresh process.
	for _, s := range h.runtime.specSnapshot() {
		if s.Brief == "" || s.Model != "fake-model-r0" || s.Effort != "fake-effort" {
			t.Errorf("launch spec = %+v, want a non-empty brief and the resolved model/effort", s)
		}
	}
	// T3's red tree was fixed by the REAL local convergence loop BEFORE its first
	// push, so the remote self-heal loop never had to re-push: remote CI ran once
	// per candidate, and the whole run converged with NO fail-closed escalation.
	if h.pusher.pushCount() != 0 {
		t.Errorf("drive-to-green re-pushes = %d, want 0 (T3 was fixed locally before its first push)", h.pusher.pushCount())
	}
	if got := h.argus.Escalations(); got != 0 || len(h.escalated) != 0 {
		t.Errorf("fail-closed escalations = %d (handler saw %d), want 0 on an all-green run", got, len(h.escalated))
	}

	// --- Both sinks carried the identical stream (§3.9, AP-2). ------------------
	h.assertBothSinksCarryLifecycle(t)
	// T3 was dispatched on its ROUTED backend (codex via the routing table), not
	// its charter slot — the real config routing composed through the resolver.
	if e := h.requireJournal(t, report.KindTaskDispatched, "T3"); e.Harness != "codex" {
		t.Errorf("T3 dispatched backend = %q, want codex (routed via the config table)", e.Harness)
	}
	if e := h.requireJournal(t, report.KindTaskDispatched, "T1"); e.Harness != "claude" {
		t.Errorf("T1 dispatched backend = %q, want claude (config default)", e.Harness)
	}

	// --- Idempotent re-invocation: same ready frontier, no duplicate work. ------
	// The driver has NOT yet landed the leaves; a re-invocation (e.g. a killed run
	// respawned) must resume from the pushed branches and open no new proposal.
	proposalsBefore := h.host.distinctProposals()
	attemptsBefore := h.countJournalKind(t, report.KindCIFixAttempt)
	t2Num := h.host.proposalNumber("feat/T2-leafA")
	t3Num := h.host.proposalNumber("feat/T3-leafB")

	rep3 := h.sweep(t) // frontier is still [T2, T3]
	if rep3.Succeeded != 2 {
		t.Fatalf("re-invocation counts = %+v, want 2 succeeded (idempotent)", rep3)
	}
	if got := h.host.distinctProposals(); got != proposalsBefore {
		t.Errorf("distinct proposals after re-invocation = %d, want unchanged %d (no duplicate proposal, AP-22)", got, proposalsBefore)
	}
	if n := h.host.proposalNumber("feat/T2-leafA"); n != t2Num {
		t.Errorf("T2 proposal number changed on re-invocation: %d -> %d (must be stable)", t2Num, n)
	}
	if n := h.host.proposalNumber("feat/T3-leafB"); n != t3Num {
		t.Errorf("T3 proposal number changed on re-invocation: %d -> %d (must be stable)", t3Num, n)
	}
	if got := h.countJournalKind(t, report.KindCIFixAttempt); got != attemptsBefore {
		t.Errorf("ci_fix_attempt after re-invocation = %d, want unchanged %d (resume finds green, no new drive)", got, attemptsBefore)
	}
	// The leaves were genuinely re-opened on the re-invocation, yet each kept its
	// one proposal number — a re-open, never a duplicate (AP-22).
	if h.host.reopenCount("feat/T2-leafA") < 2 || h.host.reopenCount("feat/T3-leafB") < 2 {
		t.Errorf("leaves not re-opened on re-invocation: T2=%d T3=%d, want >= 2 each",
			h.host.reopenCount("feat/T2-leafA"), h.host.reopenCount("feat/T3-leafB"))
	}
	// Every sweep re-probed the tracker fresh: three sweeps, three reads (AP-15).
	if got := h.tracker.frontierReads(); got != 3 {
		t.Errorf("frontier reads = %d, want 3 (one fresh read per sweep)", got)
	}
}

// TestHermeticFullStack_HollowIsClassifiedFailedOnBothSinks proves verify-by-
// artifact end to end: a task whose fresh process opens no proposal (writes no
// artifact) is HOLLOW — classed failed, never succeeded — and the hollow verdict
// reaches both sinks (DINV-3).
func TestHermeticFullStack_HollowIsClassifiedFailedOnBothSinks(t *testing.T) {
	h := newHarness(t)
	hollow := port.Task{
		ID: "TH", Number: 7, Title: "hollow task",
		HeadBranch: "feat/TH-hollow", Brief: "Do the thing. [[e2e:TH:hollow]]",
	}
	h.tracker.setFrontier(hollow)

	rep := h.sweep(t)
	if rep.Failed != 1 || rep.Succeeded != 0 {
		t.Fatalf("counts = %+v, want 1 failed (hollow is never succeeded, DINV-3)", rep)
	}
	if got := rep.Results[0].Outcome; got.String() != "hollow" {
		t.Errorf("preserved outcome = %v, want hollow (kept visible for the hollow-rate metric)", got)
	}
	if got := h.host.distinctProposals(); got != 0 {
		t.Errorf("proposals opened = %d, want 0 (a hollow run opens none)", got)
	}
	// The run really was a no-op on the remote: the agent's worktree carried no
	// commit beyond its base, so Push published nothing and no junk branch exists.
	if _, ok := h.git.branchHead(t, "feat/TH-hollow"); ok {
		t.Errorf("a branch was published for a hollow task; nothing beyond base must ever be pushed (DINV-3)")
	}
	// The hollow verdict is on both sinks.
	if e := h.requireJournal(t, report.KindTaskVerified, "TH"); e.Verdict != report.VerdictHollow {
		t.Errorf("journaled verdict = %q, want hollow", e.Verdict)
	}
	if op := h.opBuf.String(); !strings.Contains(op, "verdict=hollow") {
		t.Errorf("operator view missing the hollow verdict:\n%s", op)
	}
}

// TestHermeticFullStack_EmptyFrontierZeroModelCalls proves DINV-8 through the full
// stack: a sweep over an empty frontier spawns no Worker and makes zero model
// calls, while the run is still bracketed on both sinks.
func TestHermeticFullStack_EmptyFrontierZeroModelCalls(t *testing.T) {
	h := newHarness(t)
	// frontier empty by default.
	rep := h.sweep(t)

	if !rep.NothingReady() {
		t.Fatalf("report = %+v, want NothingReady over an empty frontier", rep)
	}
	if got := h.runtime.launchCount(); got != 0 {
		t.Fatalf("model calls = %d, want 0 on an empty frontier (DINV-8, AP-19)", got)
	}
	// Nothing was claimed and no worktree was cut: the parent checkout still has
	// exactly one worktree (itself) and the remote carries no claim ref.
	if got := h.git.worktreeCount(t); got != 1 {
		t.Errorf("worktrees on the parent checkout = %d, want 1 (nothing prepared before a ready gate)", got)
	}
	if refs := h.git.claimRefs(t); len(refs) != 0 {
		t.Errorf("claim refs on the remote = %v, want none over an empty frontier", refs)
	}
	// The run is still bracketed on both sinks even with nothing to do.
	if h.countJournalKind(t, report.KindRunStarted) != 1 || h.countJournalKind(t, report.KindRunFinished) != 1 {
		t.Errorf("run not bracketed on the journal: started=%d finished=%d",
			h.countJournalKind(t, report.KindRunStarted), h.countJournalKind(t, report.KindRunFinished))
	}
	if op := h.opBuf.String(); !strings.Contains(op, "run started") || !strings.Contains(op, "run finished") {
		t.Errorf("operator view missing the run brackets:\n%s", op)
	}
}

// --- assertion helpers ---------------------------------------------------------

// requirePublishedGreen asserts the branch exists on the REAL remote and its
// published head carries green work — the same object CI would fetch. Reading
// the remote rather than the worktree is what makes this a publish assertion:
// work that was committed but never pushed fails here.
func (h *harness) requirePublishedGreen(t *testing.T, branch string) {
	t.Helper()
	content, published := h.git.publishedArtifact(t, branch)
	if !published {
		t.Errorf("branch %s was never published to the remote; a green proposal must have a pushed head", branch)
		return
	}
	if content != "green" {
		t.Errorf("published head of %s carries %q, want green", branch, content)
	}
}

// assertBranchesCutFromMainline asserts every named branch descends from the
// mainline tip and from no sibling task branch — the DINV-2 base rule, checked
// against real commit ancestry rather than a fake's bookkeeping. A dependent
// task based off its blocker's branch (the harbormaster #982 defect) fails here.
func (h *harness) assertBranchesCutFromMainline(t *testing.T, branches ...string) {
	t.Helper()
	heads := map[string]string{}
	for _, branch := range branches {
		head, ok := h.git.branchHead(t, branch)
		if !ok {
			t.Errorf("branch %s is absent from the remote", branch)
			continue
		}
		heads[branch] = head
		if !h.git.isAncestor(t, h.git.baseline, head) {
			t.Errorf("branch %s does not descend from the mainline tip it must be cut from (DINV-2)", branch)
		}
	}
	for branch, head := range heads {
		for sibling, siblingHead := range heads {
			// A sibling whose head is the SAME object is not stacked on this branch
			// — it is one commit off the same base that happens to be identical.
			// Only a PROPER ancestor means this branch was cut from that sibling.
			if sibling == branch || siblingHead == head {
				continue
			}
			if h.git.isAncestor(t, siblingHead, head) {
				t.Errorf("branch %s contains sibling %s's head; tasks must be cut from mainline, never stacked (DINV-2, AP-26)", branch, sibling)
			}
		}
	}
}

// classOf returns the reported class of the task with the given tracker number.
func classOf(t *testing.T, rep dispatch.RunReport, number int) dispatch.Class {
	t.Helper()
	for _, r := range rep.Results {
		if r.Task.Number == number {
			return r.Class
		}
	}
	t.Fatalf("task #%d absent from the report; the sweep must class every task", number)
	return dispatch.ClassBlocked
}

// assertBothSinksCarryLifecycle asserts the identical per-task lifecycle reached
// BOTH sinks — the argus JSONL journal and the operator live view — over the one
// event stream (AP-2). Called after the three tasks have run.
func (h *harness) assertBothSinksCarryLifecycle(t *testing.T) {
	t.Helper()
	// Journal (unattended sink): the full lifecycle for every task plus the run
	// brackets.
	for _, id := range []string{"T1", "T2", "T3"} {
		h.requireJournal(t, report.KindTaskDispatched, id)
		h.requireJournal(t, report.KindTaskVerified, id)
		h.requireJournal(t, report.KindTaskOutcome, id)
	}
	if h.countJournalKind(t, report.KindRunStarted) < 1 || h.countJournalKind(t, report.KindRunFinished) < 1 {
		t.Errorf("journal missing run brackets")
	}

	// Operator view (attended sink): the same events rendered as slog lines.
	op := h.opBuf.String()
	for _, want := range []string{
		"run started", "task dispatched", "task verified", "task outcome", "run finished",
		"task=T1", "task=T2", "task=T3", "verdict=green", "local gate fix attempt",
	} {
		if !strings.Contains(op, want) {
			t.Errorf("operator view missing %q\n---\n%s", want, op)
		}
	}
}

// journalEvents parses the accumulated argus JSONL journal into events.
func (h *harness) journalEvents(t *testing.T) []report.Event {
	t.Helper()
	var out []report.Event
	sc := bufio.NewScanner(bytes.NewReader(h.journal.Bytes()))
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var e report.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("journal line is not JSON: %q: %v", sc.Bytes(), err)
		}
		out = append(out, e)
	}
	return out
}

func (h *harness) countJournalKind(t *testing.T, kind report.Kind) int {
	t.Helper()
	n := 0
	for _, e := range h.journalEvents(t) {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func (h *harness) requireJournal(t *testing.T, kind report.Kind, taskID string) report.Event {
	t.Helper()
	for _, e := range h.journalEvents(t) {
		if e.Kind == kind && e.TaskID == taskID {
			return e
		}
	}
	t.Fatalf("journal missing a %s event for task %s", kind, taskID)
	return report.Event{}
}
