package verdict

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// TestSignalWellFormed — the emitted downgrade signal is well-formed: an
// escalate forces HUMAN, a request-changes downgrades (to the worker) but does
// NOT route the lane HUMAN, an approve/approve-with-advisories downgrades
// nothing.
func TestSignalWellFormed(t *testing.T) {
	cases := []struct {
		name          string
		verdict       string
		wantPresent   bool
		wantDowngrade bool // DowngradeSignal.Downgrades() — any downgrade
		wantToHuman   bool // DowngradesToHuman() — lane-HUMAN routing
	}{
		{"escalate → to-HUMAN", report.VerdictEscalate, true, true, true},
		{"request-changes → downgrades (worker), not to-HUMAN", report.VerdictRequestChanges, true, true, false},
		{"approve-with-advisories → no downgrade", report.VerdictApproveWithAdvisories, true, false, false},
		{"approve → no downgrade", report.VerdictApprove, true, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := reportAt(tc.verdict, "sha-1")
			s := SignalFor(r, "sha-1")
			if s.Present != tc.wantPresent {
				t.Fatalf("Present = %v, want %v", s.Present, tc.wantPresent)
			}
			if s.ProposalRef != r.ProposalRef {
				t.Errorf("ProposalRef = %q, want %q", s.ProposalRef, r.ProposalRef)
			}
			if s.Verdict != tc.verdict {
				t.Errorf("signal verdict = %q, want %q", s.Verdict, tc.verdict)
			}
			if s.ReviewedRevision != "sha-1" {
				t.Errorf("ReviewedRevision = %q, want sha-1", s.ReviewedRevision)
			}
			if s.Downgrades() != tc.wantDowngrade {
				t.Errorf("Downgrades() = %v, want %v", s.Downgrades(), tc.wantDowngrade)
			}
			down, reason := s.DowngradesToHuman()
			if down != tc.wantToHuman {
				t.Errorf("DowngradesToHuman() = %v, want %v", down, tc.wantToHuman)
			}
			if reason == "" {
				t.Errorf("signal reason is empty; a signal must name why it downgrades or not")
			}
		})
	}
}

// TestSignalStaleIsAbsent — a stale verdict emits an ABSENT signal that
// downgrades nothing (UB-6/DINV-7 folded into the emit).
func TestSignalStaleIsAbsent(t *testing.T) {
	r := reportAt(report.VerdictEscalate, "sha-1", "sha-2")

	// At head == latest, an escalate signal routes HUMAN.
	live := SignalFor(r, "sha-2")
	if !live.Present || !mustDowngradeToHuman(t, live) {
		t.Fatalf("a current escalate must route HUMAN; got present=%v", live.Present)
	}

	// Head moved past the reviewed tree → stale → absent → downgrades nothing,
	// even though the persisted verdict is escalate.
	stale := SignalFor(r, "sha-3")
	if stale.Present {
		t.Errorf("a stale signal must be absent (Present=false)")
	}
	if stale.Downgrades() {
		t.Errorf("a stale (absent) signal must downgrade nothing")
	}
	if down, _ := stale.DowngradesToHuman(); down {
		t.Errorf("a stale (absent) signal must not route HUMAN")
	}
	if stale.Reason == "" {
		t.Errorf("a stale signal must name WHY it is absent")
	}
}

// TestSignalDeterministic — SignalFor is a pure function of (report, head).
func TestSignalDeterministic(t *testing.T) {
	r := reportAt(report.VerdictRequestChanges, "sha-1")
	a := SignalFor(r, "sha-1")
	b := SignalFor(r, "sha-1")
	if a != b {
		t.Errorf("SignalFor is not deterministic: %+v vs %+v", a, b)
	}
}

// TestSignalNilReport — a nil report emits an absent signal, never a downgrade.
func TestSignalNilReport(t *testing.T) {
	s := SignalFor(nil, "sha-1")
	if s.Present || s.Downgrades() {
		t.Errorf("nil report must emit an absent, non-downgrading signal; got %+v", s)
	}
}

func mustDowngradeToHuman(t *testing.T, s DowngradeSignal) bool {
	t.Helper()
	down, _ := s.DowngradesToHuman()
	return down
}

// policyStub builds the minimal all-AUTO policy the lane-readability test drives
// the classifier with: one AUTO class whose perimeter contains the `docs/x.md`
// diff, no floor, no kill switch — so the classifier routes AUTO unless a
// downgrader downgrades. It is the test's own fixture (not production data).
type policyStub struct{}

func (policyStub) policy() *policy.Policy {
	return &policy.Policy{
		SchemaVersion: policy.SchemaVersion,
		AutoClasses: []policy.AutoClass{
			{Name: "docs-only", Paths: []string{"docs/x.md"}},
		},
	}
}

// --- Lane-readability, without the production package importing the lane. ---
//
// The production verdict package imports NO internal/lane code (design §3.6, the
// consumption is the owed Q4 amendment). This TEST demonstrates that the emitted
// signal is nonetheless a well-formed input a lane's downgrade-only stage could
// read: a tiny LOCAL adapter — the shape the owed lane amendment would add —
// wraps a DowngradeSignal and satisfies classifier.Downgrader. The adapter lives
// HERE in the test, not in the production package, so the decoupling holds while
// the compatibility is proven concretely.

// signalDowngrader is the one-line adapter the owed lane amendment would write:
// it holds an emitted signal and answers the lane's downgrade-only question from
// it. It is defined in the test to keep the production package lane-free.
type signalDowngrader struct {
	sig DowngradeSignal
}

// Downgrade satisfies classifier.Downgrader — proving the emitted signal's shape
// maps directly onto the lane's downgrade-only seam. The lane consults it only
// for an otherwise-AUTO proposal; a force-HUMAN escalate downgrades it, anything
// else leaves the lane's own decision intact. It can NEVER upgrade to AUTO (the
// interface has no such return), matching the verdict's downgrade-only authority.
func (d signalDowngrader) Downgrade(_ classifier.Diff) (bool, string, error) {
	down, reason := d.sig.DowngradesToHuman()
	return down, reason, nil
}

// Compile-time proof: the local adapter satisfies the lane's Downgrader seam.
var _ classifier.Downgrader = signalDowngrader{}

// TestEmittedSignalSatisfiesLaneDowngraderShape drives the local adapter through
// the REAL classifier to show the emitted verdict, once wrapped, downgrades an
// otherwise-AUTO proposal to HUMAN on an escalate and leaves it AUTO on an
// approve — the lane-readability the emit promises, with the lane consumption
// still deferred (the wrapping lives in the test, not the package).
func TestEmittedSignalSatisfiesLaneDowngraderShape(t *testing.T) {
	// A minimal all-AUTO policy: one class whose perimeter contains the diff, no
	// floor. The classifier will route AUTO unless the downgrader downgrades.
	pol := &policyStub{}
	autoDiff := classifier.Inputs{
		Diff: classifier.Diff{Paths: []string{"docs/x.md"}},
		Base: "b", Head: "h",
	}

	t.Run("escalate signal downgrades an AUTO candidate to HUMAN", func(t *testing.T) {
		sig := SignalFor(reportAt(report.VerdictEscalate, "h"), "h")
		v := classifier.Classify(autoDiff, pol.policy(), signalDowngrader{sig: sig})
		if v.Lane != classifier.Human {
			t.Errorf("escalate signal did not downgrade to HUMAN: lane = %q, reason = %q", v.Lane, v.Reason)
		}
	})

	t.Run("approve signal leaves the AUTO candidate AUTO", func(t *testing.T) {
		sig := SignalFor(reportAt(report.VerdictApprove, "h"), "h")
		v := classifier.Classify(autoDiff, pol.policy(), signalDowngrader{sig: sig})
		if v.Lane != classifier.Auto {
			t.Errorf("approve signal must not force a downgrade: lane = %q, reason = %q", v.Lane, v.Reason)
		}
	})

	t.Run("request-changes downgrades to worker, not lane-HUMAN (lane stays AUTO from THIS seam)", func(t *testing.T) {
		// request-changes returns to the worker, not the lane's HUMAN routing —
		// so through the lane's HUMAN-only downgrade seam it does not downgrade.
		// (The worker-return is enforced elsewhere; the lane seam is HUMAN-only.)
		sig := SignalFor(reportAt(report.VerdictRequestChanges, "h"), "h")
		v := classifier.Classify(autoDiff, pol.policy(), signalDowngrader{sig: sig})
		if v.Lane != classifier.Auto {
			t.Errorf("request-changes must not route the LANE HUMAN via the downgrade seam: lane = %q", v.Lane)
		}
	})

	t.Run("stale escalate is absent → no downgrade → AUTO", func(t *testing.T) {
		// The persisted verdict is escalate, but the head moved: absent → the
		// signal downgrades nothing, so the lane's own decision (AUTO) stands.
		sig := SignalFor(reportAt(report.VerdictEscalate, "old"), "h")
		v := classifier.Classify(autoDiff, pol.policy(), signalDowngrader{sig: sig})
		if v.Lane != classifier.Auto {
			t.Errorf("a stale (absent) escalate must not downgrade: lane = %q", v.Lane)
		}
	})
}
