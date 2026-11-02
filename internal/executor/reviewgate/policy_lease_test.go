package reviewgate

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type leasedPolicyReader struct {
	*requestFixture
	before func()
	fail   error
	held   bool
}

func (r *leasedPolicyReader) WithSnapshot(ctx context.Context, binding Binding, expected Snapshot, mutate func() error) error {
	if r.fail != nil {
		return r.fail
	}
	if r.before != nil {
		r.before()
	}
	current, err := r.Snapshot(ctx, binding)
	if err != nil {
		return err
	}
	if binding.ProposalOID != r.g.OID || !reflect.DeepEqual(expected, current) || !reflect.DeepEqual(expected.Policy, r.policy) {
		return ErrPolicyStale
	}
	r.held = true
	defer func() { r.held = false }()
	return mutate()
}

func TestTaskReviewLeaseRequiresCurrentPolicyAndKeepsCallbackInsideGuard(t *testing.T) {
	for _, fault := range []string{"", "nil callback", "invalid binding", "missing lease", "lease unavailable", "policy changes before lease", "checks change before lease", "policy read unavailable", "different generation", "callback fails"} {
		t.Run(fault, func(t *testing.T) {
			f := newRequestFixture(t)
			reader := &leasedPolicyReader{requestFixture: f}
			var source Reader = reader
			if fault == "missing lease" {
				source = f
			}
			requests, err := NewTaskRequests(t.Context(), f, f, source)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID)
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
					return errTaskInput
				}
				return nil
			}
			switch fault {
			case "nil callback":
				mutate = nil
			case "invalid binding":
				binding = codehost.ReviewBinding{}
			case "lease unavailable":
				reader.fail = errTaskInput
			case "policy changes before lease":
				reader.before = func() { f.policy.Prompts += "-new" }
			case "checks change before lease":
				reader.before = func() { f.snapshot.Checks[0].State = port.CheckRed }
			case "policy read unavailable":
				f.fault = "policy unavailable"
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
			if fault == "callback fails" && !errors.Is(err, errTaskInput) {
				t.Fatalf("callback failure lost: %v", err)
			}
		})
	}
}
