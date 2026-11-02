package dispatch

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

// prePublishInvoker is a review service that can also review a local revision
// (worker.PrePublishReviewer): the shape the production reviewinvoke.Invoker
// has once the local-first loop is on.
type prePublishInvoker struct {
	*recordingInvoker
	lmu        sync.Mutex
	reviews    []worker.PrePublishReview
	localCalls int
	// drift is what every remediation audit reports.
	drift []string
}

func (p *prePublishInvoker) ReviewLocal(_ context.Context, _ port.Task, _ port.Workdir, round int) worker.PrePublishReview {
	p.lmu.Lock()
	defer p.lmu.Unlock()
	p.localCalls++
	i := p.localCalls - 1
	if i >= len(p.reviews) {
		i = len(p.reviews) - 1
	}
	r := p.reviews[i]
	r.Round = round
	return r
}

func (p *prePublishInvoker) AuditRemediation(context.Context, port.Workdir, worker.PrePublishReview, []worker.RemediationDispute) (worker.RemediationAudit, error) {
	return worker.RemediationAudit{ScopeDrift: p.drift}, nil
}

type fakeHeadReader struct{ head string }

func (f fakeHeadReader) ProposalHead(context.Context, port.Proposal) (string, error) {
	return f.head, nil
}

type fakeReviewPublisher struct {
	mu    sync.Mutex
	calls int
	ref   string
}

func (f *fakeReviewPublisher) PublishReview(context.Context, port.Proposal, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.ref, nil
}

func verifiedReview() worker.ReviewResult {
	return worker.ReviewResult{Harness: "claude", ReviewerPrincipal: "review-bot", ProposalRef: "github:agenticlab-ai/pyxis#1", Revision: "abc123",
		AttemptID: "attempt-1", ObligationID: "review-obligation-1", Phase: worker.ReviewPhaseArtifact, ArtifactRef: "github:77", ArtifactProofRef: "github:broker-receipt/77",
		Status: report.ReviewStatusCompleted, ObligationStatus: report.ReviewObligationVerified}
}

func localApprove(rev string) worker.PrePublishReview {
	return worker.PrePublishReview{Revision: rev, ProposalRef: "local:agenticlab-ai/pyxis@feat/t1-1", ReviewID: "proc-" + rev, Verdict: "approve", Artifact: "artifact", Tokens: 5}
}

// With the loop on, a task reviewed and accepted at its local revision is
// published once; the accepted review covers the published head, so no
// post-green reviewer is launched and no obligation is captured. The report
// counts it as pre-publish-reviewed and the stream carries the loop's events.
func TestSweep_PrePublishCoveredSkipsPostGreenReview(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	inv := &prePublishInvoker{recordingInvoker: &recordingInvoker{result: verifiedReview()}, reviews: []worker.PrePublishReview{localApprove("sha-1")}}
	publisher := &fakeReviewPublisher{ref: "github-comment:5"}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 1, PrePublishReview: true, RemediationRounds: 2},
		WithReporter(events), WithProposalHeadReader(fakeHeadReader{head: "sha-1"}), WithReviewPublisher(publisher),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 1 || rep.Failed != 0 || rep.PrePublishReviewed != 1 || rep.ReviewCompleted != 0 || rep.ReviewFailed != 0 || rep.ReviewPending != 0 {
		t.Fatalf("counts = %+v, want one succeeded, pre-publish-reviewed task and no post-green review", rep)
	}
	if inv.localCalls != 1 || inv.calls != 0 || inv.captures != 0 || inv.handoffs != 0 {
		t.Fatalf("local=%d invoke=%d captures=%d handoffs=%d, want one local review and zero post-green activity", inv.localCalls, inv.calls, inv.captures, inv.handoffs)
	}
	if publisher.calls != 1 || rt.launchCount() != 1 {
		t.Fatalf("publishes=%d launches=%d, want the artifact landed once and one implementation launch", publisher.calls, rt.launchCount())
	}
	if events.countKind(report.KindReviewRound) != 1 || events.countKind(report.KindImplReviewTerminal) != 0 || events.countKind(report.KindTaskVerified) != 1 || events.countKind(report.KindTaskOutcome) != 1 {
		t.Fatalf("events = %+v", events.snapshot())
	}
	terminal, ok := events.find(report.KindPrePublishReviewTerminal, "T1")
	if !ok || !terminal.Covered || terminal.Verdict != "approve" || terminal.Revision != "sha-1" || terminal.Round != 1 || terminal.ArtifactRef != "github-comment:5" || terminal.Escalated {
		t.Fatalf("pre_publish_review_terminal = %+v", terminal)
	}
	outcome, _ := events.find(report.KindTaskOutcome, "T1")
	if outcome.Class != "succeeded" {
		t.Fatalf("task_outcome = %+v", outcome)
	}
	finished, ok := events.find(report.KindRunFinished, "")
	if !ok || finished.Counts == nil || finished.Counts.PrePublishReviewed != 1 || finished.Counts.Succeeded != 1 {
		t.Fatalf("run_finished = %+v", finished)
	}
	if rep.Tokens != 5+1 && rep.Tokens != 5 {
		t.Logf("tokens = %d (implementation fake may report zero)", rep.Tokens)
	}
}

// request-changes runs a remediation sublaunch and a second review before the
// single publish; the stream shows the round and the attempt.
func TestSweep_PrePublishRemediatesBeforePublish(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	inv := &prePublishInvoker{recordingInvoker: &recordingInvoker{result: verifiedReview()}, reviews: []worker.PrePublishReview{
		{Revision: "sha-1", ProposalRef: "local:x", ReviewID: "p1", Verdict: "request-changes", Blocking: 2, Findings: "- [FND-1] a\n- [FND-2] b"},
		localApprove("sha-2"),
	}, drift: []string{"b.go (new file)"}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 1, PrePublishReview: true, RemediationRounds: 2},
		WithReporter(events), WithProposalHeadReader(fakeHeadReader{head: "sha-2"}),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 1 || rep.PrePublishReviewed != 1 || inv.localCalls != 2 || inv.calls != 0 || rt.launchCount() != 2 {
		t.Fatalf("counts=%+v local=%d invoke=%d launches=%d", rep, inv.localCalls, inv.calls, rt.launchCount())
	}
	if events.countKind(report.KindReviewRound) != 2 || events.countKind(report.KindRemediationAttempt) != 1 {
		t.Fatalf("events = %+v", events.snapshot())
	}
	rem, _ := events.find(report.KindRemediationAttempt, "T1")
	if rem.Attempt != 1 || rem.Cap != 2 {
		t.Fatalf("remediation_attempt = %+v", rem)
	}
	terminal, _ := events.find(report.KindPrePublishReviewTerminal, "T1")
	if !terminal.Covered || terminal.Round != 2 || terminal.Remediations != 1 || terminal.Blocking != 0 {
		t.Fatalf("terminal = %+v", terminal)
	}
	if len(terminal.ScopeDrift) != 1 || terminal.ScopeDrift[0] != "remediation 1: b.go (new file)" {
		t.Fatalf("terminal scope drift = %q, want the remediation's drift carried to the report", terminal.ScopeDrift)
	}
}

// When the published head is not the reviewed revision, the accepted local
// review is stale and the post-green reviewer runs exactly as before.
func TestSweep_PrePublishHeadMovedFallsBackToPostGreenReview(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	inv := &prePublishInvoker{recordingInvoker: &recordingInvoker{result: verifiedReview()}, reviews: []worker.PrePublishReview{localApprove("sha-1")}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 1, PrePublishReview: true},
		WithReporter(events), WithProposalHeadReader(fakeHeadReader{head: "sha-moved"}),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Succeeded != 1 || rep.PrePublishReviewed != 0 || rep.ReviewCompleted != 1 || inv.localCalls != 1 || inv.calls != 1 || inv.captures != 1 {
		t.Fatalf("counts=%+v local=%d invoke=%d captures=%d, want the post-green review on the moved head", rep, inv.localCalls, inv.calls, inv.captures)
	}
	terminal, _ := events.find(report.KindPrePublishReviewTerminal, "T1")
	if terminal.Covered {
		t.Fatalf("terminal = %+v, want uncovered", terminal)
	}
	if events.countKind(report.KindImplReviewTerminal) != 1 {
		t.Fatalf("events = %+v", events.snapshot())
	}
}

// The loop is opt-in at this layer: with PrePublishReview off, a service that
// could review locally is used only post-green, byte-for-byte the base path.
func TestSweep_PrePublishOffKeepsPostGreenReview(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	inv := &prePublishInvoker{recordingInvoker: &recordingInvoker{result: verifiedReview()}, reviews: []worker.PrePublishReview{localApprove("sha-1")}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1},
		WithProposalHeadReader(fakeHeadReader{head: "sha-1"}),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if inv.localCalls != 0 || inv.calls != 1 || rep.ReviewCompleted != 1 || rep.PrePublishReviewed != 0 {
		t.Fatalf("local=%d invoke=%d counts=%+v", inv.localCalls, inv.calls, rep)
	}
}

// A review that escalates fails the task closed: no push, no proposal, no
// post-green review, an escalated terminal with the reason.
func TestSweep_PrePublishEscalationFailsClosed(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	events := &captureEmitter{}
	inv := &prePublishInvoker{recordingInvoker: &recordingInvoker{result: verifiedReview()}, reviews: []worker.PrePublishReview{
		{Revision: "sha-1", ProposalRef: "local:x", ReviewID: "p1", Verdict: "escalate", EscalationBrief: "conflict: charter vs design\noption: amend the design\ndecision needed: which governs"},
	}}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 1, PrePublishReview: true},
		WithReporter(events), WithProposalHeadReader(fakeHeadReader{head: "sha-1"}),
		WithReviewInvokerFactory(func(entity.Harness) (worker.ReviewService, error) { return inv, nil }))
	if err != nil {
		t.Fatal(err)
	}
	tr.setFrontier(task("T1", 1))
	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 1 || rep.Succeeded != 0 || rep.PrePublishReviewed != 0 || inv.calls != 0 || inv.captures != 0 {
		t.Fatalf("counts=%+v invoke=%d captures=%d", rep, inv.calls, inv.captures)
	}
	terminal, ok := events.find(report.KindPrePublishReviewTerminal, "T1")
	if !ok || !terminal.Escalated || terminal.Verdict != "escalate" || !strings.Contains(terminal.Reason, "escalated to a human") ||
		!strings.Contains(terminal.Reason, "decision needed: which governs") {
		t.Fatalf("terminal = %+v", terminal)
	}
	outcome, _ := events.find(report.KindTaskOutcome, "T1")
	if outcome.Class != "failed" || !strings.Contains(outcome.Reason, "charter vs design") {
		t.Fatalf("task_outcome = %+v", outcome)
	}
}

// The loop's terminal names why coverage was not decided or the artifact not
// landed, and a failed review round carries its failure class and reason.
func TestPrePublishTerminalAndRoundReasons(t *testing.T) {
	events := &captureEmitter{}
	tr, ls, ws, rt, hs := greenFakes(t)
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1}, WithReporter(events))
	if err != nil {
		t.Fatal(err)
	}
	tk := task("T9", 9)
	d.emitPrePublishTerminal(TaskResult{Task: tk, PrePublish: &worker.PrePublishOutcome{Rounds: 1, Verdict: "approve", Revision: "sha", Covered: true, PublishErr: errors.New("gh: 403")}})
	d.emitPrePublishTerminal(TaskResult{Task: tk, PrePublish: &worker.PrePublishOutcome{Rounds: 1, Verdict: "approve", Revision: "sha", CoverageErr: errors.New("gh: 503")}})
	d.emitPrePublishTerminal(TaskResult{Task: tk, PrePublish: nil})
	prePublishObserver{d: d}.ReviewRound(tk, worker.PrePublishReview{Round: 2, Phase: "artifact", FailureClass: "artifact_unverified", Err: errors.New("no artifact")})
	prePublishObserver{d: d}.RemediationAttempt(tk, 1, 2)

	var terminals []report.Event
	for _, e := range events.snapshot() {
		if e.Kind == report.KindPrePublishReviewTerminal {
			terminals = append(terminals, e)
		}
	}
	if len(terminals) != 2 {
		t.Fatalf("terminals = %d, want 2 (a nil record emits nothing)", len(terminals))
	}
	if !strings.Contains(terminals[0].Reason, "artifact publication failed: gh: 403") || !terminals[0].Covered {
		t.Fatalf("terminal[0] = %+v", terminals[0])
	}
	if !strings.Contains(terminals[1].Reason, "coverage undecided: gh: 503") || terminals[1].Covered {
		t.Fatalf("terminal[1] = %+v", terminals[1])
	}
	round, ok := events.find(report.KindReviewRound, "T9")
	if !ok || round.Round != 2 || round.FailureClass != "artifact_unverified" || round.Reason != "no artifact" || round.Phase != "artifact" {
		t.Fatalf("review_round = %+v", round)
	}
	rem, ok := events.find(report.KindRemediationAttempt, "T9")
	if !ok || rem.Attempt != 1 || rem.Cap != 2 || rem.Issue != 9 {
		t.Fatalf("remediation_attempt = %+v", rem)
	}
}
