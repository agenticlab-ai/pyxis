package worker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// --- fakes for the local-first convergence loop --------------------------------

// fakePrePublishReviewer scripts one PrePublishReview per round (the last entry
// repeats), recording the rounds it was asked for.
type fakePrePublishReviewer struct {
	reviews []PrePublishReview
	calls   int
	rounds  []int
	sawDirs []string
	// audit and auditErr answer every AuditRemediation; audited records what
	// each call was handed.
	audit    RemediationAudit
	auditErr error
	audited  []auditCall
}

type auditCall struct {
	revision string
	disputes []RemediationDispute
}

func (f *fakePrePublishReviewer) AuditRemediation(_ context.Context, _ port.Workdir, review PrePublishReview, disputes []RemediationDispute) (RemediationAudit, error) {
	f.audited = append(f.audited, auditCall{revision: review.Revision, disputes: disputes})
	return f.audit, f.auditErr
}

func (f *fakePrePublishReviewer) ReviewLocal(_ context.Context, _ port.Task, work port.Workdir, round int) PrePublishReview {
	f.calls++
	f.rounds = append(f.rounds, round)
	if work != nil {
		f.sawDirs = append(f.sawDirs, work.Dir())
	}
	i := f.calls - 1
	if i >= len(f.reviews) {
		i = len(f.reviews) - 1
	}
	r := f.reviews[i]
	r.Round = round
	return r
}

type fakeHeadReader struct {
	head  string
	err   error
	calls int
}

func (f *fakeHeadReader) ProposalHead(context.Context, port.Proposal) (string, error) {
	f.calls++
	return f.head, f.err
}

type fakeReviewPublisher struct {
	ref      string
	err      error
	calls    int
	proposal port.Proposal
	artifact string
}

func (f *fakeReviewPublisher) PublishReview(_ context.Context, p port.Proposal, artifact string) (string, error) {
	f.calls++
	f.proposal, f.artifact = p, artifact
	return f.ref, f.err
}

type fakePrePublishObserver struct {
	rounds       []PrePublishReview
	remediations [][2]int
}

func (o *fakePrePublishObserver) ReviewRound(_ port.Task, r PrePublishReview) {
	o.rounds = append(o.rounds, r)
}
func (o *fakePrePublishObserver) RemediationAttempt(_ port.Task, attempt, cap int) {
	o.remediations = append(o.remediations, [2]int{attempt, cap})
}

// loopCapturer is the post-green configured-review capturer: the loop must skip
// it when the pre-publish review covers the published head, and fall back to
// it otherwise.
type loopCapturer struct{ captures, handoffs int }

func (c *loopCapturer) Capture(_ context.Context, _ port.Task, _ port.Proposal) ReviewCapture {
	c.captures++
	return ReviewCapture{ProposalRef: "github:o/r#7", Revision: "sha-published", ObligationID: "ob-1", Harness: "claude", Persisted: true}
}

func (c *loopCapturer) CompleteHandoff(context.Context, ReviewCapture) error {
	c.handoffs++
	return nil
}

func approve(rev string) PrePublishReview {
	return PrePublishReview{Revision: rev, ProposalRef: "local:o/r@feat/x", ReviewID: "proc-" + rev, Verdict: "approve", Artifact: "artifact-" + rev, Tokens: 10}
}

func requestChanges(rev, findings string) PrePublishReview {
	return PrePublishReview{Revision: rev, ProposalRef: "local:o/r@feat/x", ReviewID: "proc-" + rev, Verdict: "request-changes", Blocking: 1, Findings: findings, Artifact: "artifact-" + rev, Tokens: 10}
}

// --- ensure local CI is green, before any push ----------------------------------

// The local gate runs before the first push, over the committed tree, and a
// green tree publishes once.
func TestRun_LocalGateRunsBeforeFirstPush(t *testing.T) {
	deps, _, _, space, _, host := greenDeps(t)
	host.checkState = port.CheckGreen
	driver := &fakeGreenDriver{}
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v, want succeeded", outcome, err)
	}
	if driver.localCalled != 1 || len(driver.localSawDirs) != 1 || driver.localSawDirs[0] != space.dir {
		t.Fatalf("DriveLocal calls=%d dirs=%v, want exactly one over the task worktree", driver.localCalled, driver.localSawDirs)
	}
	if space.last.pushes != 1 || space.last.checkpoints < 1 {
		t.Fatalf("pushes=%d checkpoints=%d, want one push after a checkpoint", space.last.pushes, space.last.checkpoints)
	}
	if driver.called != 0 {
		t.Fatalf("remote Drive called %d times for an already-green proposal, want 0", driver.called)
	}
}

// A local gate that cannot converge fails the task closed with NOTHING pushed
// and no proposal — the remote CI never sees a red tree.
func TestRun_LocalGateEscalationFailsClosedWithoutPush(t *testing.T) {
	deps, _, lease, space, _, host := greenDeps(t)
	escErr := errors.New("not green after 3/3 self-heal attempts: local gate red: test-race")
	driver := &fakeGreenDriver{localErr: escErr}
	w := mustWorker(t, deps, WithGreenDriver(driver))

	outcome, err := w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed || !errors.Is(err, escErr) {
		t.Fatalf("outcome=%v err=%v, want failed wrapping the escalation", outcome, err)
	}
	if space.last.pushes != 0 || len(host.openedTasks) != 0 || len(host.checksRead) != 0 {
		t.Fatalf("pushes=%d opened=%d checks=%d — a locally red tree must never reach the code host", space.last.pushes, len(host.openedTasks), len(host.checksRead))
	}
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once: %+v", lease.claim)
	}
}

// A local fixer that ran leaves its changes committed before the tree is
// reviewed or pushed.
func TestRun_LocalFixIsCheckpointedBeforePublish(t *testing.T) {
	deps, _, _, space, _, _ := greenDeps(t)
	driver := &fakeGreenDriver{localResult: &LocalDriveResult{Green: true, Attempts: 2, Cap: 3}}
	w := mustWorker(t, deps, WithGreenDriver(driver))
	if outcome, err := w.Run(context.Background(), newTestTask()); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	// One checkpoint before the gate, one after the fixer — then the push.
	if space.last.checkpoints != 2 || space.last.pushes != 1 {
		t.Fatalf("checkpoints=%d pushes=%d, want 2 and 1", space.last.checkpoints, space.last.pushes)
	}
}

// A run that produced nothing beyond base is hollow before any gate, review or
// push is spent.
func TestRun_EmptyTreeIsHollowBeforeLocalGate(t *testing.T) {
	deps, _, _, space, _, host := greenDeps(t)
	space.noWork = true
	driver := &fakeGreenDriver{}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{approve("sha-1")}}
	w := mustWorker(t, deps, WithGreenDriver(driver), WithPrePublishReview(reviewer, 2))
	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeHollow {
		t.Fatalf("outcome=%v err=%v, want hollow", outcome, err)
	}
	if driver.localCalled != 0 || reviewer.calls != 0 || space.last.pushes != 0 || len(host.openedTasks) != 0 {
		t.Fatalf("gate=%d review=%d pushes=%d opened=%d — an empty run must spend nothing", driver.localCalled, reviewer.calls, space.last.pushes, len(host.openedTasks))
	}
}

// --- the review → remediate → local green loop ------------------------------------

// An accepted first review publishes once; the covered proposal receives the
// artifact and no post-green obligation is captured.
func TestRun_PrePublishApproveThenPublishOnceAndCover(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	host.proposal = port.Proposal{Number: 7}
	driver := &fakeGreenDriver{}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{approve("sha-1")}}
	head := &fakeHeadReader{head: "sha-1"}
	publisher := &fakeReviewPublisher{ref: "github-comment:1"}
	capturer := &loopCapturer{}
	obs := &fakePrePublishObserver{}
	w := mustWorker(t, deps, WithGreenDriver(driver), WithPrePublishReview(reviewer, 2),
		WithProposalHeadReader(head), WithReviewPublisher(publisher), WithReviewCapturer(capturer), WithPrePublishObserver(obs))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v, want succeeded", outcome, err)
	}
	if reviewer.calls != 1 || reviewer.rounds[0] != 1 || reviewer.sawDirs[0] != space.dir {
		t.Fatalf("reviewer calls=%d rounds=%v dirs=%v, want one round-1 review over the worktree", reviewer.calls, reviewer.rounds, reviewer.sawDirs)
	}
	if len(runtime.launches) != 1 || space.last.pushes != 1 || len(host.openedTasks) != 1 {
		t.Fatalf("launches=%d pushes=%d opened=%d, want 1/1/1 (no remediation, one publish)", len(runtime.launches), space.last.pushes, len(host.openedTasks))
	}
	if driver.localCalled != 1 {
		t.Fatalf("DriveLocal calls=%d, want 1 (before the review)", driver.localCalled)
	}
	out, ok := w.PrePublishOutcome()
	if !ok || !out.Covered || out.Rounds != 1 || out.Remediations != 0 || out.Verdict != "approve" || out.Revision != "sha-1" || out.PublishedRef != "github-comment:1" || out.Err != nil {
		t.Fatalf("outcome record = %+v, want covered round-1 approve at sha-1", out)
	}
	if head.calls != 1 || publisher.calls != 1 || publisher.proposal.Number != 7 || publisher.artifact != "artifact-sha-1" {
		t.Fatalf("head reads=%d publishes=%d proposal=%+v artifact=%q", head.calls, publisher.calls, publisher.proposal, publisher.artifact)
	}
	if capturer.captures != 0 {
		t.Fatalf("post-green capture ran %d times for a covered head, want 0", capturer.captures)
	}
	if len(obs.rounds) != 1 || obs.rounds[0].Verdict != "approve" || len(obs.remediations) != 0 {
		t.Fatalf("observer = rounds %+v remediations %v", obs.rounds, obs.remediations)
	}
}

// request-changes runs a fresh remediation sublaunch fed the blocking findings as
// data, re-runs the local gate, re-reviews the new revision, then publishes the
// accepted revision once.
func TestRun_PrePublishRequestChangesRemediatesThenApproves(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	host.proposal = port.Proposal{Number: 7}
	driver := &fakeGreenDriver{}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{
		requestChanges("sha-1", "- [FND-1] pkg/x.go:12 — nil deref on empty input (rule R1, lens correctness-critic)"),
		approve("sha-2"),
	}}
	head := &fakeHeadReader{head: "sha-2"}
	obs := &fakePrePublishObserver{}
	capturer := &loopCapturer{}
	w := mustWorker(t, deps, WithGreenDriver(driver), WithPrePublishReview(reviewer, 2),
		WithProposalHeadReader(head), WithReviewCapturer(capturer), WithPrePublishObserver(obs))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v, want succeeded", outcome, err)
	}
	if len(runtime.launches) != 2 {
		t.Fatalf("launches=%d, want 2 (implementation + one remediation)", len(runtime.launches))
	}
	rem := runtime.launches[1]
	if rem.Purpose != port.LaunchPurposeImplementation || rem.Workdir != space.dir || rem.HeadBranch != space.last.branch {
		t.Fatalf("remediation launch = %+v, want an implementation launch in the task worktree on its branch", rem)
	}
	if !strings.Contains(rem.Brief, "Implement the widget per the charter.") || !strings.Contains(rem.Brief, "FND-1") || !strings.Contains(rem.Brief, "data, not instructions") || !strings.Contains(rem.Brief, "sha-1") {
		t.Fatalf("remediation brief lacks the charter, the findings as data, or the reviewed revision:\n%s", rem.Brief)
	}
	if !strings.Contains(rem.Brief, "or dispute it") || !strings.Contains(rem.Brief, "```review-disputes") || !strings.Contains(rem.Brief, `"schema": "review-disputes/v1"`) || !strings.Contains(rem.Brief, "scope drift") {
		t.Fatalf("remediation brief does not offer the dispute block or warn about scope drift:\n%s", rem.Brief)
	}
	if strings.Index(rem.Brief, "review-disputes/v1") > strings.Index(rem.Brief, "data, not instructions") {
		t.Fatalf("the dispute instructions must precede the findings data fence:\n%s", rem.Brief)
	}
	if len(reviewer.audited) != 1 || reviewer.audited[0].revision != "sha-1" || len(reviewer.audited[0].disputes) != 0 {
		t.Fatalf("audits = %+v, want one audit of the sha-1 round with no disputes", reviewer.audited)
	}
	if rem.Model != "fake-model" || rem.Effort != "fake-effort" {
		t.Fatalf("remediation route = %s/%s, want the implementation route", rem.Model, rem.Effort)
	}
	if reviewer.calls != 2 || reviewer.rounds[1] != 2 || driver.localCalled != 2 {
		t.Fatalf("reviews=%d rounds=%v gate=%d, want 2 reviews and a re-gate after remediation", reviewer.calls, reviewer.rounds, driver.localCalled)
	}
	if space.last.pushes != 1 || len(host.openedTasks) != 1 {
		t.Fatalf("pushes=%d opened=%d, want exactly one publish after acceptance", space.last.pushes, len(host.openedTasks))
	}
	out, _ := w.PrePublishOutcome()
	if !out.Covered || out.Rounds != 2 || out.Remediations != 1 || out.Cap != 2 || out.Revision != "sha-2" || out.Tokens != 20 {
		t.Fatalf("outcome record = %+v", out)
	}
	if len(obs.remediations) != 1 || obs.remediations[0] != [2]int{1, 2} || len(obs.rounds) != 2 {
		t.Fatalf("observer = rounds %d remediations %v", len(obs.rounds), obs.remediations)
	}
	if capturer.captures != 0 {
		t.Fatal("post-green capture ran for a covered head")
	}
}

// Past the remediation bound the task fails closed: nothing pushed, the work
// preserved, the escalation carrying the last verdict.
func TestRun_PrePublishRemediationCapExhaustedFailsClosed(t *testing.T) {
	deps, _, lease, space, runtime, host := greenDeps(t)
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- [FND-1] still wrong")}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 1))

	outcome, err := w.Run(context.Background(), newTestTask())
	var esc *ReviewEscalationError
	if outcome != OutcomeFailed || !errors.As(err, &esc) {
		t.Fatalf("outcome=%v err=%v, want failed with *ReviewEscalationError", outcome, err)
	}
	if esc.Remediations != 1 || esc.Cap != 1 || esc.Rounds != 2 || esc.Verdict != "request-changes" {
		t.Fatalf("escalation = %+v, want 1/1 remediations over 2 rounds", esc)
	}
	if reviewer.calls != 2 || len(runtime.launches) != 2 {
		t.Fatalf("reviews=%d launches=%d, want 2 reviews and one remediation before the bound", reviewer.calls, len(runtime.launches))
	}
	if space.last.pushes != 0 || len(host.openedTasks) != 0 {
		t.Fatalf("pushes=%d opened=%d — nothing may be published when review never accepts", space.last.pushes, len(host.openedTasks))
	}
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once: %+v", lease.claim)
	}
	out, _ := w.PrePublishOutcome()
	if out.Covered || out.Err == nil {
		t.Fatalf("outcome record = %+v, want uncovered with the escalation recorded", out)
	}
}

// An escalate verdict is a human decision: fail closed immediately with the
// brief, no remediation, nothing pushed.
func TestRun_PrePublishEscalateFailsClosedWithBrief(t *testing.T) {
	deps, _, _, space, runtime, _ := greenDeps(t)
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{{Revision: "sha-1", Verdict: "escalate",
		EscalationBrief: "conflict: the charter contradicts the design on retries\noptions: ...", ProposalRef: "local:o/r@feat/x", ReviewID: "p"}}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))

	outcome, err := w.Run(context.Background(), newTestTask())
	var esc *ReviewEscalationError
	if outcome != OutcomeFailed || !errors.As(err, &esc) || esc.Verdict != "escalate" || !strings.Contains(esc.Brief, "contradicts") {
		t.Fatalf("outcome=%v err=%v, want failed with an escalate brief", outcome, err)
	}
	if !strings.Contains(err.Error(), "escalated to a human") || !strings.Contains(err.Error(), "conflict: the charter") {
		t.Fatalf("error text = %q", err)
	}
	if len(runtime.launches) != 1 || space.last.pushes != 0 {
		t.Fatalf("launches=%d pushes=%d, want no remediation and no push on escalate", len(runtime.launches), space.last.pushes)
	}
}

// A worker that disputes a blocking finding stops the loop for the operator:
// the audit's brief is the escalation, the remediated work stays committed, and
// no further gate, review or push runs (prd:impl-review/FR-20 → FR-31).
func TestRun_PrePublishDisputedFindingEscalatesWithoutPush(t *testing.T) {
	deps, tracker, lease, space, runtime, host := greenDeps(t)
	runtime.result = port.RunResult{Output: "I fixed nothing.\n```review-disputes\n" +
		`{"schema": "review-disputes/v1", "disputes": [{"finding": "FND-1", "reason": "D-1 names git worktree add; the atomic creation the finding asks for replaces it"}]}` +
		"\n```\n"}
	driver := &fakeGreenDriver{}
	reviewer := &fakePrePublishReviewer{
		reviews: []PrePublishReview{requestChanges("sha-1", "- [FND-1] a.go:12 — racy ownership check"), approve("sha-2")},
		audit:   RemediationAudit{Escalation: "conflict: the worker disputes 1 blocking finding(s)\noption: uphold the finding\ndecision needed: whether each disputed finding stands\n"},
	}
	w := mustWorker(t, deps, WithGreenDriver(driver), WithPrePublishReview(reviewer, 2))

	outcome, err := w.Run(context.Background(), newTestTask())
	var esc *ReviewEscalationError
	if outcome != OutcomeFailed || !errors.As(err, &esc) {
		t.Fatalf("outcome=%v err=%v, want failed with *ReviewEscalationError", outcome, err)
	}
	if esc.Verdict != "escalate" || esc.Rounds != 1 || esc.Remediations != 1 || !strings.Contains(esc.Brief, "disputes 1 blocking finding") || !strings.Contains(err.Error(), "escalated to a human") {
		t.Fatalf("escalation = %+v (%v), want an escalate carrying the dispute brief", esc, err)
	}
	if len(reviewer.audited) != 1 || len(reviewer.audited[0].disputes) != 1 || reviewer.audited[0].disputes[0].FindingID != "FND-1" ||
		!strings.Contains(reviewer.audited[0].disputes[0].Reason, "D-1 names git worktree add") {
		t.Fatalf("audits = %+v, want the FND-1 dispute with its reason", reviewer.audited)
	}
	if reviewer.calls != 1 || driver.localCalled != 1 || len(runtime.launches) != 2 {
		t.Fatalf("reviews=%d gates=%d launches=%d, want no gate or review after the dispute", reviewer.calls, driver.localCalled, len(runtime.launches))
	}
	if space.last.pushes != 0 || len(host.openedTasks) != 0 || space.last.checkpoints < 2 {
		t.Fatalf("pushes=%d opened=%d checkpoints=%d, want the remediated work committed and nothing published", space.last.pushes, len(host.openedTasks), space.last.checkpoints)
	}
	if lease.claim == nil || lease.claim.releases != 1 {
		t.Fatalf("claim not released exactly once: %+v", lease.claim)
	}
	out, _ := w.PrePublishOutcome()
	if out.Err == nil || out.Covered {
		t.Fatalf("outcome record = %+v, want the escalation recorded", out)
	}
	task := newTestTask()
	if len(tracker.comments) != 1 || tracker.comments[0].issue != task.Number ||
		!strings.Contains(tracker.comments[0].body, "worker disputed blocking review finding(s)") ||
		!strings.Contains(tracker.comments[0].body, "branch "+space.last.branch) ||
		!strings.Contains(tracker.comments[0].body, "decision needed: whether each disputed finding stands") {
		t.Fatalf("tracker comments = %+v, want the full brief on the task's item", tracker.comments)
	}
}

// A dispute brief that cannot reach the task's tracker item still escalates,
// and the error says the shared record is missing.
func TestRun_PrePublishDisputeTrackerFailureStillEscalates(t *testing.T) {
	deps, tracker, _, space, runtime, _ := greenDeps(t)
	tracker.commentErr = errors.New("tracker: 503")
	runtime.result = port.RunResult{Output: "```review-disputes\n" + `{"schema": "review-disputes/v1", "disputes": [{"finding": "FND-1", "reason": "r"}]}` + "\n```"}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- [FND-1] x")}, audit: RemediationAudit{Escalation: "conflict: c\n"}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
	outcome, err := w.Run(context.Background(), newTestTask())
	var esc *ReviewEscalationError
	if outcome != OutcomeFailed || !errors.As(err, &esc) || !strings.Contains(err.Error(), "record the dispute on tracker item") || !strings.Contains(err.Error(), "tracker: 503") || space.last.pushes != 0 {
		t.Fatalf("outcome=%v err=%v pushes=%d, want the escalation joined with the tracker failure", outcome, err, space.last.pushes)
	}
}

// Scope drift the audit reports is kept on the outcome, labelled with its
// remediation, and does not stop an otherwise accepted loop.
func TestRun_PrePublishRemediationScopeDriftIsRecorded(t *testing.T) {
	deps, _, _, space, _, _ := greenDeps(t)
	reviewer := &fakePrePublishReviewer{
		reviews: []PrePublishReview{requestChanges("sha-1", "- [FND-1] a.go:12 — nil deref"), approve("sha-2")},
		audit:   RemediationAudit{ScopeDrift: []string{"b.go (new file)", "a.go:140-151"}},
	}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2), WithProposalHeadReader(&fakeHeadReader{head: "sha-2"}))
	if outcome, err := w.Run(context.Background(), newTestTask()); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	out, _ := w.PrePublishOutcome()
	want := []string{"remediation 1: b.go (new file)", "remediation 1: a.go:140-151"}
	if strings.Join(out.ScopeDrift, "|") != strings.Join(want, "|") || space.last.pushes != 1 {
		t.Fatalf("scope drift = %q pushes=%d, want %q and one publish", out.ScopeDrift, space.last.pushes, want)
	}
}

// An unreadable dispute block, or an audit that cannot be made, fails the task
// closed with nothing pushed.
func TestRun_PrePublishUnreadableDisputeOrAuditFailureFailsClosed(t *testing.T) {
	deps, _, _, space, runtime, _ := greenDeps(t)
	runtime.result = port.RunResult{Output: "```review-disputes\n{\"disputes\": []}\n```"}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- fix"), approve("sha-2")}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
	outcome, err := w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "read remediation disputes") || len(reviewer.audited) != 0 || space.last.pushes != 0 {
		t.Fatalf("outcome=%v err=%v audits=%d pushes=%d, want a failed read before any audit", outcome, err, len(reviewer.audited), space.last.pushes)
	}

	deps, _, _, space, _, _ = greenDeps(t)
	reviewer = &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- fix"), approve("sha-2")}, auditErr: errors.New("FND-9 is not a blocking finding")}
	w = mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
	outcome, err = w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "audit remediation") || !strings.Contains(err.Error(), "FND-9") || reviewer.calls != 1 || space.last.pushes != 0 {
		t.Fatalf("outcome=%v err=%v reviews=%d pushes=%d, want a failed audit and no further review", outcome, err, reviewer.calls, space.last.pushes)
	}
}

func TestParseRemediationDisputes(t *testing.T) {
	block := func(body string) string { return "done\n```review-disputes\n" + body + "\n```\n" }
	for _, tc := range []struct {
		name    string
		output  string
		want    []RemediationDispute
		wantErr string
	}{
		{name: "no block", output: "fixed FND-1"},
		{name: "empty output"},
		{name: "one dispute", output: block(`{"schema": "review-disputes/v1", "disputes": [{"finding": " FND-2 ", "reason": "the charter forbids it"}]}`),
			want: []RemediationDispute{{FindingID: "FND-2", Reason: "the charter forbids it"}}},
		{name: "indented fences", output: "  ```review-disputes\n" + `{"schema": "review-disputes/v1", "disputes": [{"finding": "FND-1", "reason": "r"}]}` + "\n  ```",
			want: []RemediationDispute{{FindingID: "FND-1", Reason: "r"}}},
		{name: "no disputes listed", output: block(`{"schema": "review-disputes/v1", "disputes": []}`)},
		{name: "unterminated", output: "```review-disputes\n{}", wantErr: "not terminated"},
		{name: "two blocks", output: strings.Repeat("```review-disputes\n"+`{"schema": "review-disputes/v1", "disputes": []}`+"\n```\n", 2), wantErr: "2 review-disputes blocks"},
		{name: "wrong schema", output: block(`{"schema": "review-disputes/v2", "disputes": []}`), wantErr: "schema"},
		{name: "unknown field", output: block(`{"schema": "review-disputes/v1", "disputes": [{"finding": "FND-1", "reason": "r", "severity": "advisory"}]}`), wantErr: "unknown field"},
		{name: "trailing value", output: block(`{"schema": "review-disputes/v1", "disputes": []} {}`), wantErr: "more than one JSON value"},
		{name: "text after the block", output: block(`{"schema": "review-disputes/v1", "disputes": [{"finding": "FND-1", "reason": "r"}]}`) + "On reflection I fixed FND-1 instead.\n", wantErr: "must end the final message"},
		{name: "blank lines after the block", output: block(`{"schema": "review-disputes/v1", "disputes": [{"finding": "FND-1", "reason": "r"}]}`) + "\n  \n",
			want: []RemediationDispute{{FindingID: "FND-1", Reason: "r"}}},
		{name: "no finding", output: block(`{"schema": "review-disputes/v1", "disputes": [{"finding": " ", "reason": "r"}]}`), wantErr: "names no finding"},
		{name: "not json", output: block(`dispute FND-1`), wantErr: "parse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRemediationDisputes(tc.output)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("disputes = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("dispute %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// A review that produced no verdict (infrastructure, unverified artifact) fails
// the task closed rather than publishing unreviewed work.
func TestRun_PrePublishReviewFailureFailsClosed(t *testing.T) {
	deps, _, _, space, _, _ := greenDeps(t)
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{{Revision: "sha-1", Phase: ReviewPhaseArtifact, FailureClass: ReviewFailureArtifactUnverified, Err: errors.New("no artifact in handoff")}}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))

	outcome, err := w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "no artifact in handoff") || !strings.Contains(err.Error(), ReviewFailureArtifactUnverified) {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	if space.last.pushes != 0 {
		t.Fatal("a failed review must not publish")
	}
}

// When the remote self-heal loop moved the head after the review, the review is
// stale (AP-13): the post-green configured review is captured on the new head.
func TestRun_PrePublishHeadMovedKeepsPostGreenReview(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.checkState = port.CheckRed
	driver := &fakeGreenDriver{result: GreenDriveResult{Green: true, Attempts: 1, Cap: 3}}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{approve("sha-1")}}
	head := &fakeHeadReader{head: "sha-2"}
	publisher := &fakeReviewPublisher{ref: "x"}
	capturer := &loopCapturer{}
	w := mustWorker(t, deps, WithGreenDriver(driver), WithPrePublishReview(reviewer, 2),
		WithProposalHeadReader(head), WithReviewPublisher(publisher), WithReviewCapturer(capturer))

	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	out, _ := w.PrePublishOutcome()
	if out.Covered || publisher.calls != 0 || capturer.captures != 1 {
		t.Fatalf("covered=%v publishes=%d captures=%d — a moved head must fall back to the post-green review", out.Covered, publisher.calls, capturer.captures)
	}
}

// Without an exact head read the Worker never assumes coverage.
func TestRun_PrePublishWithoutHeadReaderKeepsPostGreenReview(t *testing.T) {
	deps, _, _, _, _, _ := greenDeps(t)
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{approve("sha-1")}}
	capturer := &loopCapturer{}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2), WithReviewCapturer(capturer))
	if outcome, err := w.Run(context.Background(), newTestTask()); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	out, _ := w.PrePublishOutcome()
	if out.Covered || capturer.captures != 1 {
		t.Fatalf("covered=%v captures=%d, want the post-green review owed", out.Covered, capturer.captures)
	}
}

// A head read failure is recorded and falls back; a publication failure is
// recorded and never un-succeeds a green task.
func TestRun_PrePublishCoverageAndPublishFailuresAreRecordedNotFatal(t *testing.T) {
	deps, _, _, _, _, _ := greenDeps(t)
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{approve("sha-1")}}
	capturer := &loopCapturer{}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2),
		WithProposalHeadReader(&fakeHeadReader{err: errors.New("gh: 503")}), WithReviewCapturer(capturer))
	if outcome, err := w.Run(context.Background(), newTestTask()); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	out, _ := w.PrePublishOutcome()
	if out.Covered || out.CoverageErr == nil || capturer.captures != 1 {
		t.Fatalf("record = %+v captures=%d, want uncovered with the read error and the post-green capture", out, capturer.captures)
	}

	deps, _, _, _, _, _ = greenDeps(t)
	reviewer = &fakePrePublishReviewer{reviews: []PrePublishReview{approve("sha-1")}}
	capturer = &loopCapturer{}
	w = mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2),
		WithProposalHeadReader(&fakeHeadReader{head: "sha-1"}), WithReviewPublisher(&fakeReviewPublisher{err: errors.New("gh: 403")}), WithReviewCapturer(capturer))
	if outcome, err := w.Run(context.Background(), newTestTask()); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	out, _ = w.PrePublishOutcome()
	if !out.Covered || out.PublishErr == nil || capturer.captures != 0 {
		t.Fatalf("record = %+v captures=%d, want covered with the publish error recorded", out, capturer.captures)
	}
}

// The remediation sublaunch is bounded by the same wall-clock budget as the
// implementation run; a truncated remediation is checkpointed and judged by
// the gate and the next review, not treated as an error.
func TestRun_PrePublishRemediationBudgetTruncationIsJudgedByArtifact(t *testing.T) {
	deps, _, _, space, runtime, _ := greenDeps(t)
	runtime.result = port.RunResult{Incomplete: false}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- fix"), approve("sha-2")}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2), WithProposalHeadReader(&fakeHeadReader{head: "sha-2"}))
	if outcome, err := w.Run(context.Background(), newTestTask()); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	// checkpoint before gate, after remediation; the fixer (0 attempts) adds none.
	if space.last.checkpoints < 2 || len(runtime.launches) != 2 {
		t.Fatalf("checkpoints=%d launches=%d", space.last.checkpoints, len(runtime.launches))
	}
}

// Without a pre-publish reviewer the publish path is byte-for-byte the base
// path: no loop record exists and the post-green capture runs as before.
func TestRun_NoPrePublishReviewerKeepsBasePath(t *testing.T) {
	deps, _, _, _, _, _ := greenDeps(t)
	capturer := &loopCapturer{}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithReviewCapturer(capturer), WithProposalHeadReader(&fakeHeadReader{head: "sha-1"}))
	if outcome, err := w.Run(context.Background(), newTestTask()); err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("outcome=%v err=%v", outcome, err)
	}
	if _, ok := w.PrePublishOutcome(); ok {
		t.Fatal("no loop record expected without a reviewer")
	}
	if capturer.captures != 1 {
		t.Fatalf("captures=%d, want the base post-green capture", capturer.captures)
	}
}

func TestWithPrePublishReview_NilAndCapDefaults(t *testing.T) {
	w := &Worker{}
	WithPrePublishReview(nil, 5)(w)
	if w.prePublish != nil {
		t.Fatal("nil reviewer must not install the loop")
	}
	WithPrePublishReview(&fakePrePublishReviewer{}, 0)(w)
	if w.remediationCap != DefaultRemediationRounds {
		t.Fatalf("cap = %d, want the default %d", w.remediationCap, DefaultRemediationRounds)
	}
	WithPrePublishReview(&fakePrePublishReviewer{}, 3)(w)
	if w.remediationCap != 3 {
		t.Fatalf("cap = %d, want 3", w.remediationCap)
	}
}

// --- coverage of the loop's remaining branches ----------------------------------

func TestPrePublishReview_AcceptedAndFailedPredicates(t *testing.T) {
	for _, tc := range []struct {
		r        PrePublishReview
		accepted bool
		failed   bool
	}{
		{approve("sha"), true, false},
		{PrePublishReview{Revision: "sha", Verdict: "approve-with-advisories"}, true, false},
		{requestChanges("sha", "x"), false, false},
		{PrePublishReview{Revision: "sha", Verdict: "escalate"}, false, false},
		{PrePublishReview{Revision: "sha"}, false, true},
		{PrePublishReview{Verdict: "approve"}, true, true}, // accepted verdict but no revision: unusable
		{PrePublishReview{Revision: "sha", Verdict: "approve", Err: errors.New("x")}, true, true},
	} {
		if tc.r.Accepted() != tc.accepted || tc.r.Failed() != tc.failed {
			t.Errorf("%+v: accepted=%v failed=%v", tc.r, tc.r.Accepted(), tc.r.Failed())
		}
	}
	esc := &ReviewEscalationError{TaskID: "T", Rounds: 3, Remediations: 2, Cap: 2, Verdict: "request-changes", Reason: "1 blocking finding(s) remain"}
	if !strings.Contains(esc.Error(), "still requests changes after 2/2") {
		t.Errorf("Error() = %q", esc.Error())
	}
	if firstLine("\n\n  hello \nworld", "fb") != "hello" || firstLine("  \n", "fb") != "fb" {
		t.Error("firstLine")
	}
}

// A remediation launch that cannot start, or a remediation that dies, fails the
// task closed with nothing pushed.
func TestRun_PrePublishRemediationLaunchAndAwaitFailuresFailClosed(t *testing.T) {
	deps, _, _, space, runtime, _ := greenDeps(t)
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- fix"), approve("sha-2")}}
	// The implementation launch must succeed; only the second (remediation)
	// launch fails, so fail it by counting launches.
	launches := 0
	failingRuntime := &launchCountingRuntime{fakeRuntime: runtime, failAt: 2, counter: &launches}
	deps.Runtime = failingRuntime
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
	outcome, err := w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "launch remediation") {
		t.Fatalf("outcome=%v err=%v, want a failed remediation launch", outcome, err)
	}
	if space.last.pushes != 0 {
		t.Fatal("nothing may be pushed when remediation cannot start")
	}

	deps, _, _, space, runtime, _ = greenDeps(t)
	reviewer = &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- fix"), approve("sha-2")}}
	awaits := 0
	deps.Runtime = &awaitCountingRuntime{fakeRuntime: runtime, failAt: 2, counter: &awaits}
	w = mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
	outcome, err = w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "await remediation") {
		t.Fatalf("outcome=%v err=%v, want a failed remediation await", outcome, err)
	}
	if space.last.pushes != 0 {
		t.Fatal("nothing may be pushed when remediation dies")
	}
}

// launchCountingRuntime fails the Nth Launch; awaitCountingRuntime fails the
// Nth Await — so the implementation run succeeds and the remediation fails.
type launchCountingRuntime struct {
	*fakeRuntime
	failAt  int
	counter *int
}

func (r *launchCountingRuntime) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	*r.counter++
	if *r.counter == r.failAt {
		return nil, errors.New("claude: cannot start")
	}
	return r.fakeRuntime.Launch(ctx, spec)
}

type awaitCountingRuntime struct {
	*fakeRuntime
	failAt  int
	counter *int
}

func (r *awaitCountingRuntime) Await(ctx context.Context, h port.Handle) (port.RunResult, error) {
	*r.counter++
	if *r.counter == r.failAt {
		return port.RunResult{}, errors.New("claude: process died")
	}
	return r.fakeRuntime.Await(ctx, h)
}

// A local gate that goes red after a remediation and cannot be fixed fails the
// task closed; an unknown verdict is refused; a cancelled context stops the loop.
func TestRun_PrePublishPostRemediationGateFailureUnknownVerdictAndCancel(t *testing.T) {
	deps, _, _, space, _, _ := greenDeps(t)
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- fix"), approve("sha-2")}}
	driver := &fakeGreenDriver{localErrAt: 2, localErr: errors.New("local gate red: vet")}
	w := mustWorker(t, deps, WithGreenDriver(driver), WithPrePublishReview(reviewer, 2))
	outcome, err := w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "local gate red: vet") || space.last.pushes != 0 {
		t.Fatalf("outcome=%v err=%v pushes=%d", outcome, err, space.last.pushes)
	}

	deps, _, _, space, _, _ = greenDeps(t)
	reviewer = &fakePrePublishReviewer{reviews: []PrePublishReview{{Revision: "sha-1", Verdict: "shrug"}}}
	w = mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
	outcome, err = w.Run(context.Background(), newTestTask())
	if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "unsupported review verdict") || space.last.pushes != 0 {
		t.Fatalf("outcome=%v err=%v pushes=%d", outcome, err, space.last.pushes)
	}

	deps, _, _, _, _, _ = greenDeps(t)
	cancelling := &cancellingReviewer{}
	w = mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(cancelling, 2))
	ctx, cancel := context.WithCancel(context.Background())
	cancelling.cancel = cancel
	outcome, err = w.Run(ctx, newTestTask())
	if outcome != OutcomeFailed || !errors.Is(err, context.Canceled) {
		t.Fatalf("outcome=%v err=%v, want cancellation surfaced", outcome, err)
	}
}

// cancellingReviewer requests changes and cancels the run, so the next loop
// iteration observes the cancelled context.
type cancellingReviewer struct{ cancel context.CancelFunc }

func (c *cancellingReviewer) ReviewLocal(context.Context, port.Task, port.Workdir, int) PrePublishReview {
	c.cancel()
	return requestChanges("sha-1", "- fix")
}

func (c *cancellingReviewer) AuditRemediation(context.Context, port.Workdir, PrePublishReview, []RemediationDispute) (RemediationAudit, error) {
	return RemediationAudit{}, nil
}
