package report

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// The continuity and convergence-loop events carry their fields and render on
// the operator sink — in-package, since the coverage floor measures each
// package by its own tests.
func TestSessionAndLoopEventConstructorsAndOperatorRendering(t *testing.T) {
	events := []Event{
		TaskSession("T1", 1, "claude", "resumed", "sess-1", "feedback", ""),
		TaskSession("T1", 1, "claude", "degraded", "", "fix", "ledger locked"),
		FeedbackRound("T1", 1, 77, 2, 3, true),
		LocalGateAttempt("T1", 1, 3),
		ReviewRound(ReviewRoundFields{TaskID: "T1", Issue: 1, Round: 2, Verdict: "request-changes", Revision: "sha", ProposalRef: "local:o/r@b", ReviewID: "p", Blocking: 2, Model: "m", Effort: "high"}),
		ReviewRound(ReviewRoundFields{TaskID: "T1", Issue: 1, Round: 1, Phase: "artifact", FailureClass: "artifact_unverified", Reason: "no artifact"}),
		RemediationAttempt("T1", 1, 1, 3),
		PrePublishReviewTerminal(PrePublishReviewTerminalFields{TaskID: "T1", Issue: 1, Rounds: 2, Remediations: 1, Cap: 3, Verdict: "approve", Revision: "sha", ReviewID: "p", Covered: true, ArtifactRef: "github-comment:9"}),
		PrePublishReviewTerminal(PrePublishReviewTerminalFields{TaskID: "T1", Issue: 1, Rounds: 1, Verdict: "escalate", Reason: "brief", Escalated: true}),
	}
	if events[0].Kind != KindTaskSession || events[0].SessionID != "sess-1" || events[0].Purpose != "feedback" || events[0].Action != "resumed" {
		t.Fatalf("task_session = %+v", events[0])
	}
	if events[2].Kind != KindFeedbackRound || events[2].ProposalRef != "#77" || events[2].Comments != 3 || !events[2].ChangesRequested || events[2].Round != 2 {
		t.Fatalf("feedback_round = %+v", events[2])
	}
	if events[3].Kind != KindLocalGateAttempt || events[3].Attempt != 1 || events[3].Cap != 3 {
		t.Fatalf("local_gate_attempt = %+v", events[3])
	}
	if events[4].Blocking != 2 || events[4].ReviewProcessID != "p" || events[4].Revision != "sha" {
		t.Fatalf("review_round = %+v", events[4])
	}
	if events[7].Round != 2 || !events[7].Covered || events[7].ArtifactRef != "github-comment:9" {
		t.Fatalf("pre_publish_review_terminal = %+v", events[7])
	}

	var buf bytes.Buffer
	sink := NewOperatorSink(slog.New(slog.NewTextHandler(&buf, nil)))
	for _, e := range events {
		if err := sink.Deliver(e); err != nil {
			t.Fatalf("deliver %s: %v", e.Kind, err)
		}
	}
	out := buf.String()
	for _, want := range []string{"task session", "action=resumed", "session=sess-1", "reason=\"ledger locked\"",
		"human feedback round", "proposal=#77", "comments=3", "changes_requested=true",
		"local gate fix attempt", "pre-publish review round", "verdict=request-changes", "blocking=2",
		"failure_class=artifact_unverified", "review remediation attempt",
		"pre-publish review terminal", "covered=true", "artifact=github-comment:9", "escalated=true"} {
		if !strings.Contains(out, want) {
			t.Errorf("operator view missing %q\n---\n%s", want, out)
		}
	}
}
