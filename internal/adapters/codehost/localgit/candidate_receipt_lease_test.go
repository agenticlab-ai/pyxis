package localgit

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
)

func TestCandidateReceiptHoldsEvidenceThroughCommitAndFinalVerification(t *testing.T) {
	t.Parallel()
	o, proposals, _, reader, requests, opened := candidateReviewFixture(t)
	packet := seedCandidateReview(t, o, reader, opened)
	receipts := NewCandidateReceipts(proposals.options, requests)
	committed := false
	receipts.commit = func(ctx context.Context, origin string, body []byte) error {
		if _, err := proposalGit(ctx, origin, nil, nil, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID); err == nil {
			t.Fatal("evidence authority changed between verification and receipt commit")
		}
		_, err := proposalGit(ctx, origin, body, nil, "update-ref", "--stdin")
		committed = err == nil
		return err
	}
	record, err := receipts.Record(t.Context(), packet.Binding, "operator", "")
	if err != nil || !committed || record.Receipt.Binding != packet.Binding {
		t.Fatalf("leased candidate receipt failed: %+v %v", record, err)
	}
	fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
}

type unleasedCandidateReceiptSource struct{ candidatereview.Reader }

func TestCandidateReceiptRefusesChangedOrMissingEvidenceAuthorityWithoutPublication(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"moved source", "scenario changed", "results change before lease", "attribution unavailable", "missing lease"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			o, proposals, artifacts, reader, requests, opened := candidateReviewFixture(t)
			packet := seedCandidateReview(t, o, reader, opened)
			before := fixtureGit(t, o.Origin, "show-ref")
			if fault == "missing lease" {
				var err error
				requests, err = candidatereview.NewRequests(t.Context(), proposals, artifacts, unleasedCandidateReceiptSource{reader})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				reader.beforeLease = func() {
					switch fault {
					case "moved source":
						fixtureGit(t, o.Origin, "update-ref", reader.policyRef, packet.Binding.Head, reader.policyOID)
					case "scenario changed":
						reader.plan.ScenarioSetFingerprint += "-new"
					case "results change before lease":
						reader.evidence.Results[0].Outcome = featuredrive.OutcomeFailed
					case "attribution unavailable":
						reader.fault = fault
					}
					before = fixtureGit(t, o.Origin, "show-ref")
				}
			}
			if _, err := NewCandidateReceipts(proposals.options, requests).Record(t.Context(), packet.Binding, "operator", ""); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("obsolete or unleased authority published an approval receipt")
			}
			if fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", receiptRef(packet.Binding)) != "" {
				t.Fatal("refused approval left a reusable receipt ref")
			}
		})
	}
}
