package wake

import (
	"context"
	"errors"
	"testing"
)

// stubLanding answers each proposal's landing from fixed maps and counts reads,
// so a test can prove exactly which proposals were consulted.
type stubLanding struct {
	merged   map[string]bool
	ancestor map[string]bool
	reads    []string
	err      error
}

func (s *stubLanding) Landing(_ context.Context, proposal string) (bool, bool, error) {
	s.reads = append(s.reads, proposal)
	if s.err != nil {
		return false, false, s.err
	}
	return s.merged[proposal], s.ancestor[proposal], nil
}

const ledger = "docs/problems/argus-unattended-drive/delivery.md"

// AC1: closure occurs only after every resulting proposal's content is verified
// an ancestor of the mainline.
func TestClosure_ClosesOnlyWhenAllLandedOnMainline(t *testing.T) {
	// All three merged AND ancestor-verified → closes.
	checker := &stubLanding{
		merged:   map[string]bool{"pr:1": true, "pr:2": true, "pr:3": true},
		ancestor: map[string]bool{"pr:1": true, "pr:2": true, "pr:3": true},
	}
	res, err := EvaluateClosure(context.Background(), ledger, []string{"pr:1", "pr:2", "pr:3"}, checker)
	if err != nil {
		t.Fatalf("EvaluateClosure: %v", err)
	}
	if !res.CanClose {
		t.Fatalf("CanClose = false, want true when every proposal is an ancestor landing")
	}
	for _, s := range res.Statuses {
		if s.State != StateLanded {
			t.Errorf("proposal %s state = %q, want landed", s.Proposal, s.State)
		}
	}
}

// One proposal still in flight (not merged) blocks closure — without an alarm.
func TestClosure_InFlightBlocksWithoutAlarm(t *testing.T) {
	checker := &stubLanding{
		merged:   map[string]bool{"pr:1": true, "pr:2": false},
		ancestor: map[string]bool{"pr:1": true, "pr:2": false},
	}
	res, err := EvaluateClosure(context.Background(), ledger, []string{"pr:1", "pr:2"}, checker)
	if err != nil {
		t.Fatalf("EvaluateClosure: %v", err)
	}
	if res.CanClose {
		t.Fatalf("CanClose = true, want false with a proposal still in flight")
	}
	if len(res.Stranded) != 0 {
		t.Fatalf("in-flight proposal raised %d alarms, want 0", len(res.Stranded))
	}
	if len(res.InFlight) != 1 || res.InFlight[0] != "pr:2" {
		t.Fatalf("InFlight = %v, want [pr:2]", res.InFlight)
	}
}

// AC2: a seeded stranded proposal — merged badge, absent mainline content —
// blocks closure and alarms on the delivery ledger.
func TestClosure_StrandedBlocksAndAlarms(t *testing.T) {
	checker := &stubLanding{
		merged:   map[string]bool{"pr:1": true, "pr:2": true},
		ancestor: map[string]bool{"pr:1": true, "pr:2": false}, // pr:2 merged badge, content absent from mainline
	}
	res, err := EvaluateClosure(context.Background(), ledger, []string{"pr:1", "pr:2"}, checker)
	if err != nil {
		t.Fatalf("EvaluateClosure: %v", err)
	}
	if res.CanClose {
		t.Fatalf("CanClose = true, want false with a stranded proposal")
	}
	if len(res.Stranded) != 1 || res.Stranded[0].Proposal != "pr:2" {
		t.Fatalf("Stranded = %+v, want one alarm on pr:2", res.Stranded)
	}
	if res.Stranded[0].Ledger != ledger {
		t.Errorf("alarm ledger = %q, want the affected delivery ledger", res.Stranded[0].Ledger)
	}
	// The stranded proposal is distinguished from a merely-in-flight one.
	var strandedState ProposalState
	for _, s := range res.Statuses {
		if s.Proposal == "pr:2" {
			strandedState = s.State
		}
	}
	if strandedState != StateStranded {
		t.Fatalf("pr:2 state = %q, want stranded", strandedState)
	}
}

// AC3: an unclaimed item ages under the existing bump arm with no new watcher
// introduced — the gate reads ONLY the proposals it is handed and consults
// nothing for an item that has produced no proposal.
func TestClosure_NoWatcherForUnclaimedItems(t *testing.T) {
	// The feature has one landed proposal; an unclaimed item (item:9) has produced
	// no proposal, so it is simply absent from the set — the gate never reads it.
	checker := &stubLanding{
		merged:   map[string]bool{"pr:1": true},
		ancestor: map[string]bool{"pr:1": true},
	}
	res, err := EvaluateClosure(context.Background(), ledger, []string{"pr:1"}, checker)
	if err != nil {
		t.Fatalf("EvaluateClosure: %v", err)
	}
	// The gate consulted exactly the proposals passed — no read for any unclaimed
	// item, and no new watcher spun up.
	if len(checker.reads) != 1 || checker.reads[0] != "pr:1" {
		t.Fatalf("landing reads = %v, want exactly [pr:1] (no watcher over unclaimed items)", checker.reads)
	}
	if !res.CanClose {
		t.Fatalf("CanClose = false; a fully-landed proposal set should close (unclaimed items are the bump arm's concern, not the gate's)")
	}
}

// An infrastructure fault reading a landing is a hard error.
func TestClosure_LandingReadErrorIsHard(t *testing.T) {
	checker := &stubLanding{err: errors.New("code host unreachable")}
	if _, err := EvaluateClosure(context.Background(), ledger, []string{"pr:1"}, checker); err == nil {
		t.Fatal("want a hard error when a landing read faults")
	}
}
