package reviewgate

import (
	"errors"
	"reflect"
	"testing"
)

func TestTaskPreparationReplacesValidProofUnderChangedPolicy(t *testing.T) {
	f := newRequestFixture(t)
	r, err := NewTaskRequests(t.Context(), f, f, f)
	if err != nil {
		t.Fatal(err)
	}
	old, err := r.Prepare(t.Context(), f.g.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.policy.Prompts = "new-current-prompts"
	if _, err := r.Observe(t.Context(), f.g.Proposal.ID); !errors.Is(err, ErrPolicyStale) || f.writes != 1 {
		t.Fatalf("observation repaired stale policy: %v", err)
	}
	f.snapshot.Policy = f.policy
	f.snapshot.Review.Aggregate.Report.Rounds[0].Provenance.PromptsVersion = f.policy.Prompts
	fresh, err := r.Prepare(t.Context(), f.g.Proposal.ID)
	if err != nil || fresh.Binding.ProposalOID == old.Binding.ProposalOID || fresh.Binding.Nonce == old.Binding.Nonce || fresh.Binding.Head != old.Binding.Head || fresh.Binding.TargetOID != old.Binding.TargetOID || f.writes != 2 {
		t.Fatalf("stale request did not receive fresh proof: %v", err)
	}
	if !reflect.DeepEqual(f.rows[old.Request.Identity.Ref], old.Request) || !reflect.DeepEqual(f.rows[old.Aggregate.Identity.Ref], old.Aggregate) {
		t.Fatal("refresh replaced immutable old proof")
	}
	if err := r.ValidateReview(t.Context(), old.Binding); err == nil {
		t.Fatal("old approval remained current")
	}
	f.fault = "native unavailable"
	if _, err := r.Prepare(t.Context(), f.g.Proposal.ID); err == nil || f.writes != 2 {
		t.Fatal("unavailable current aggregate reused or replaced a prepared request")
	}
	f.fault = ""
	again, err := r.Prepare(t.Context(), f.g.Proposal.ID)
	if err != nil || !reflect.DeepEqual(again, fresh) || f.writes != 2 {
		t.Fatalf("fresh complete request was not resumed: %v", err)
	}
}

func TestTaskPreparationDoesNotReplaceCorruptOrUnavailableRetainedProof(t *testing.T) {
	for _, fault := range []string{"malformed aggregate", "aggregate read unavailable", "policy unavailable"} {
		t.Run(fault, func(t *testing.T) {
			f := newRequestFixture(t)
			r, err := NewTaskRequests(t.Context(), f, f, f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.Prepare(t.Context(), f.g.Proposal.ID); err != nil {
				t.Fatal(err)
			}
			old := f.g
			f.policy.Prompts = "new-current-prompts"
			f.fault = fault
			if _, err := r.Prepare(t.Context(), f.g.Proposal.ID); err == nil || f.writes != 1 || !reflect.DeepEqual(f.g, old) {
				t.Fatalf("untrusted input was treated as stale policy: %v", err)
			}
		})
	}
}
