package localgit

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
)

// Adapter fault tests assume the consumer granted review eligibility. Read the
// real receipt on every call so deletion/replacement and the adapter's proof
// leases remain exercised, but do not recursively run the consumer's entire
// review workflow. Policy, attribution and lease integration tests use the real
// ReceiptStore and review consumer through ready{Task,Candidate}Landing.
type landingReceiptFixture struct {
	origin  string
	binding codehost.ReviewBinding
}

func (f landingReceiptFixture) Binding(ctx context.Context, id string) (codehost.ReviewBinding, error) {
	if err := ctx.Err(); err != nil {
		return codehost.ReviewBinding{}, err
	}
	if id != f.binding.ProposalID {
		return codehost.ReviewBinding{}, codehost.ErrReviewInvalid
	}
	return f.binding, nil
}

func (f landingReceiptFixture) Verify(ctx context.Context, binding codehost.ReviewBinding) (codehost.ReceiptRecord, error) {
	if binding != f.binding {
		return codehost.ReceiptRecord{}, codehost.ErrReviewInvalid
	}
	return (&ReceiptStore{}).read(ctx, f.origin, binding)
}

func (f landingReceiptFixture) WithReview(ctx context.Context, binding codehost.ReviewBinding, mutate func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if binding != f.binding || mutate == nil {
		return codehost.ErrReviewInvalid
	}
	// Grant review eligibility only: the adapter must itself re-read the
	// receipt inside this callback and reject deletion or replacement.
	return mutate()
}

func readyTaskAdapter(t *testing.T) (Options, *TaskLandingStore, reviewgate.Packet) {
	t.Helper()
	o, store, _, _, packet := readyTaskLanding(t)
	store.receipts = landingReceiptFixture{o.Origin, packet.Binding}
	return o, store, packet
}

func readyCandidateAdapter(t *testing.T) (Options, *CandidateLandingStore, candidatereview.Packet) {
	t.Helper()
	o, store, _, _, packet := readyCandidateLanding(t)
	store.receipts = landingReceiptFixture{o.Origin, packet.Binding}
	return o, store, packet
}
