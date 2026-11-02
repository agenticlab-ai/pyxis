package localgit

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestTaskReceiptRequiresCompleteCurrentReviewPolicy(t *testing.T) {
	t.Parallel()
	for _, approved := range []bool{false, true} {
		t.Run(map[bool]string{false: "before declaration", true: "after declaration"}[approved], func(t *testing.T) {
			o, proposals, _, reader, requests, opened := taskReviewFixture(t)
			packet := seedTaskReview(t, o, reader, opened)
			receipts := NewTaskReceipts(proposals.options, requests)
			if approved {
				seedReceipt(t, o, packet.Binding)
			}
			original := reader.policy
			for _, changed := range []string{"checks", "prompts", "model", "calibration"} {
				t.Run(changed, func(t *testing.T) {
					reader.policy = original
					switch changed {
					case "checks":
						reader.policy.Required = append(append([]string(nil), original.Required...), "new-required-check")
					case "prompts":
						reader.policy.Prompts = "new-prompts"
					case "model":
						reader.policy.NativeModel = "new-native-model"
					case "calibration":
						reader.policy.Calibration = "new-calibration"
					}
					before := fixtureGit(t, o.Origin, "show-ref")
					readsBeforeRefusal := reader.snapshots
					if _, err := receipts.Binding(t.Context(), opened.Proposal.ID); err == nil {
						t.Fatal("obsolete policy presented as current approval intent")
					}
					if _, err := receipts.Record(t.Context(), packet.Binding, "operator", ""); err == nil {
						t.Fatal("obsolete policy accepted a receipt")
					}
					if _, err := receipts.Verify(t.Context(), packet.Binding); err == nil {
						t.Fatal("obsolete policy retained current approval")
					}
					if fixtureGit(t, o.Origin, "show-ref") != before || reader.snapshots != readsBeforeRefusal {
						t.Fatal("refused approval repaired shared state or reran review")
					}
				})
			}
			reader.policy = original
			unverified := NewTaskReceipts(proposals.options, nil)
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := unverified.Record(t.Context(), packet.Binding, "operator", ""); err != codehost.ErrReviewInvalid {
				t.Fatalf("missing current review verifier accepted: %v", err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("missing review verifier wrote an approval")
			}
		})
	}
}
