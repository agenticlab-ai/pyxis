package localgit

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
)

func TestTaskReceiptHoldsPolicyThroughCommitAndFinalVerification(t *testing.T) {
	t.Parallel()
	o, proposals, _, reader, requests, opened := taskReviewFixture(t)
	packet := seedTaskReview(t, o, reader, opened)
	receipts := NewTaskReceipts(proposals.options, requests)
	committed := false
	receipts.commit = func(ctx context.Context, origin string, body []byte) error {
		if _, err := proposalGit(ctx, origin, nil, nil, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID); err == nil {
			t.Fatal("policy changed between review verification and receipt commit")
		}
		_, err := proposalGit(ctx, origin, body, nil, "update-ref", "--stdin")
		committed = err == nil
		return err
	}
	record, err := receipts.Record(t.Context(), packet.Binding, "operator", "")
	if err != nil || !committed || record.Receipt.Binding != packet.Binding {
		t.Fatalf("leased receipt failed: %+v %v", record, err)
	}
	fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
}

func TestTaskReceiptRefusesMovedOrMissingPolicyLeaseWithoutPublication(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"moved source", "checks change before lease", "missing lease"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			o, proposals, artifacts, reader, requests, opened := taskReviewFixture(t)
			packet := seedTaskReview(t, o, reader, opened)
			var before string
			if fault != "missing lease" {
				reader.beforeLease = func() {
					if fault == "moved source" {
						fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
					} else {
						reader.mutate = func(s *reviewgate.Snapshot) { s.Checks[0].State = port.CheckRed }
					}
					before = fixtureGit(t, o.Origin, "show-ref")
				}
			} else {
				var err error
				requests, err = reviewgate.NewTaskRequests(t.Context(), proposals, artifacts, unleasedTaskReader{reader})
				if err != nil {
					t.Fatal(err)
				}
				before = fixtureGit(t, o.Origin, "show-ref")
			}
			if _, err := NewTaskReceipts(proposals.options, requests).Record(t.Context(), packet.Binding, "operator", ""); err == nil || before == "" || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("unleased or obsolete policy published an approval receipt")
			}
			if fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", receiptRef(packet.Binding)) != "" {
				t.Fatal("refused receipt left a reusable approval ref")
			}
		})
	}
}

var _ codehost.ReviewLease = (*ReceiptStore)(nil)
