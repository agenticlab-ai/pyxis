package candidatereview

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

type leasedEvidenceReader struct {
	*requestSource
	before func()
	fail   error
	held   bool
}

func (r *leasedEvidenceReader) WithEvidence(_ context.Context, binding Binding, expected Snapshot, mutate func() error) error {
	if r.fail != nil {
		return r.fail
	}
	if r.before != nil {
		r.before()
	}
	if binding.Generation.OID != r.g.OID || !reflect.DeepEqual(expected.Plan, r.plan) || !reflect.DeepEqual(expected.Evidence, r.evidence) {
		return ErrIncomplete
	}
	r.held = true
	defer func() { r.held = false }()
	return mutate()
}

func TestCandidateReviewLeaseRequiresCurrentPolicyAndKeepsCallbackInsideGuard(t *testing.T) {
	for _, fault := range []string{"", "nil callback", "invalid binding", "missing lease", "lease unavailable", "policy changes before lease", "results change before lease", "policy read unavailable", "different generation", "callback fails"} {
		t.Run(fault, func(t *testing.T) {
			f := newRequestSource(t)
			reader := &leasedEvidenceReader{requestSource: f}
			var source Reader = reader
			if fault == "missing lease" {
				source = f
			}
			requests, err := NewRequests(t.Context(), f, f, source)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID, codehost.FinalCandidate(f.plan.Candidate), f.plan.ScenarioSetFingerprint)
			if err != nil {
				t.Fatal(err)
			}
			binding, calls := packet.Binding, 0
			mutate := func() error {
				if !reader.held {
					t.Fatal("mutation ran outside the policy lease")
				}
				calls++
				if fault == "callback fails" {
					return errRequestInput
				}
				return nil
			}
			switch fault {
			case "nil callback":
				mutate = nil
			case "invalid binding":
				binding = codehost.ReviewBinding{}
			case "lease unavailable":
				reader.fail = errRequestInput
			case "policy changes before lease":
				reader.before = func() { f.plan.ScenarioSetFingerprint += "-new" }
			case "results change before lease":
				reader.before = func() { f.evidence.Results[0].Outcome = featuredrive.OutcomeFailed }
			case "policy read unavailable":
				f.source.fault = "plan"
			case "different generation":
				binding.Sequence++
			}
			err = requests.WithReview(t.Context(), binding, mutate)
			wantCalls := 0
			if fault == "" || fault == "callback fails" {
				wantCalls = 1
			}
			if calls != wantCalls || reader.held || (err == nil) != (fault == "") {
				t.Fatalf("lease result: error=%v calls=%d held=%v", err, calls, reader.held)
			}
			if fault == "callback fails" && !errors.Is(err, errRequestInput) {
				t.Fatalf("callback failure lost: %v", err)
			}
		})
	}
}
