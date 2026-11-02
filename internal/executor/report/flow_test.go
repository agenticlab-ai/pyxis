package report

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestFlowSinkRendersReadableTimeline(t *testing.T) {
	var out bytes.Buffer
	sink := NewFlowSink(&out)
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	events := []Event{
		{Kind: KindTaskDispatched, Time: now, TaskID: "T1", Harness: "codex"},
		{Kind: KindLocalGateAttempt, Time: now, TaskID: "T1", Attempt: 1, Cap: 3},
		{Kind: KindLocalGateTerminal, Time: now, TaskID: "T1", Verdict: VerdictGreen, Attempt: 1, Cap: 3},
		{Kind: KindReviewRound, Time: now, TaskID: "T1", Round: 1, Verdict: "approve"},
		{Kind: KindTaskVerified, Time: now, TaskID: "T1", Verdict: VerdictGreen},
		{Kind: KindTaskOutcome, Time: now, TaskID: "T1", Class: "succeeded"},
	}
	for _, event := range events {
		if err := sink.Deliver(event); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"implementation started with codex", "local CI failed; recovery attempt 1/3", "local CI is green after 1 recovery", "local CI green; review round 1", "remote CI is green", "finished succeeded"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("flow log missing %q:\n%s", want, out.String())
		}
	}
}

func TestFlowSinkRendersRecoveryAndTerminalEvents(t *testing.T) {
	for _, tc := range []struct {
		name  string
		event Event
		want  string
	}{
		{"run started", Event{Kind: KindRunStarted, RunID: "r1"}, "run r1 started (unknown, scope unknown)"},
		{"session", Event{Kind: KindTaskSession, TaskID: "T1", Purpose: "implementation", Action: "resumed"}, "task T1: implementation session resumed"},
		{"local CI exhausted", Event{Kind: KindLocalGateTerminal, TaskID: "T1", Attempt: 3, Cap: 3, Verdict: VerdictRed, Reason: "tests failed"}, "task T1: local CI failed after 3/3 recovery attempt(s): tests failed"},
		{"review recovery", Event{Kind: KindRemediationAttempt, TaskID: "T1", Attempt: 1, Cap: 2}, "task T1: review recovery 1/2 started"},
		{"review terminal", Event{Kind: KindPrePublishReviewTerminal, TaskID: "T1", Verdict: "approve", Round: 2}, "task T1: review finished: approve after 2 round(s)"},
		{"remote CI recovery", Event{Kind: KindCIFixAttempt, TaskID: "T1", Attempt: 1, Cap: 3}, "task T1: remote CI failed; recovery attempt 1/3 started"},
		{"remote CI red", Event{Kind: KindTaskVerified, TaskID: "T1", Verdict: VerdictRed}, "task T1: proposal remote CI is red"},
		{"verification unknown", Event{Kind: KindTaskVerified, TaskID: "T1"}, "task T1: verification finished: unknown"},
		{"task failed", Event{Kind: KindTaskOutcome, TaskID: "T1", Class: "failed", Reason: "CI exhausted"}, "task T1: finished failed: CI exhausted"},
		{"configured review", Event{Kind: KindImplReviewTerminal, TaskID: "T1", Status: "completed", ObligationStatus: "verified"}, "task T1: configured review completed (verified)"},
		{"human feedback", Event{Kind: KindFeedbackRound, TaskID: "T1", Round: 1, Comments: 2}, "task T1: human feedback round 1 started (2 comment(s))"},
		{"run finished without counts", Event{Kind: KindRunFinished, RunID: "r1"}, "run r1 finished"},
		{"run counts", Event{Kind: KindRunFinished, RunID: "r1", Counts: &Counts{Succeeded: 1, Failed: 2, Blocked: 3, ReviewPending: 4}}, "run r1 finished: 1 succeeded, 2 failed, 3 blocked, 4 review pending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			tc.event.Time = time.Date(2026, 9, 9, 1, 2, 3, 0, time.FixedZone("west", -7*60*60))
			if err := NewFlowSink(&out).Deliver(tc.event); err != nil {
				t.Fatal(err)
			}
			if want := "2026-09-09T08:02:03Z | " + tc.want + "\n"; out.String() != want {
				t.Fatalf("flow line = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestFlowSinkAbsentUnknownAndFailedWrites(t *testing.T) {
	var absent *FlowSink
	for _, sink := range []*FlowSink{absent, NewFlowSink(nil)} {
		if err := sink.Deliver(Event{Kind: KindRunStarted}); err != nil {
			t.Fatalf("absent sink: %v", err)
		}
	}
	var out bytes.Buffer
	if err := NewFlowSink(&out).Deliver(Event{Kind: "future_event"}); err != nil || out.Len() != 0 {
		t.Fatalf("unknown event wrote %q, error %v", out.String(), err)
	}
	if err := NewFlowSink(flowFailedWriter{}).Deliver(Event{Kind: KindRunStarted}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write error = %v, want closed pipe", err)
	}
}

type flowFailedWriter struct{}

func (flowFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
