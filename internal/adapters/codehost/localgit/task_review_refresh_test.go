package localgit

import (
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
)

func TestTaskReviewPolicyChangePreparesFreshOriginProof(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"prompts", "required-check", "model"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			o, proposals, _, reader, requests, opened := taskReviewFixture(t)
			old := seedTaskReview(t, o, reader, opened)
			receipts := NewTaskReceipts(proposals.options, requests)
			seedReceipt(t, o, old.Binding)
			switch change {
			case "prompts":
				reader.policy.Prompts = "current-prompts-v2"
			case "required-check":
				reader.policy.Required = append(append([]string(nil), reader.policy.Required...), "new-required-check")
				reader.mutate = func(s *reviewgate.Snapshot) {
					s.Checks = append(s.Checks, reviewgate.Check{Name: "new-required-check", State: port.CheckGreen})
				}
			case "model":
				reader.policy.NativeModel = "current-native-model-v2"
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			readsBeforeRefresh := reader.snapshots
			if _, err := requests.Observe(t.Context(), opened.Proposal.ID); !errors.Is(err, reviewgate.ErrPolicyStale) || fixtureGit(t, o.Origin, "show-ref") != before || reader.snapshots != readsBeforeRefresh {
				t.Fatalf("observation replaced stale review: %v", err)
			}
			fresh, err := requests.Prepare(t.Context(), opened.Proposal.ID)
			if err != nil {
				t.Fatal(err)
			}
			if fresh.Binding.ProposalOID == old.Binding.ProposalOID || fresh.Binding.Sequence != old.Binding.Sequence+1 || fresh.Binding.Nonce == old.Binding.Nonce || fresh.Binding.Head != old.Binding.Head || fresh.Binding.TargetOID != old.Binding.TargetOID || reader.snapshots != readsBeforeRefresh+2 || !reflect.DeepEqual(fresh.Snapshot.Policy, reader.policy) {
				t.Fatal("new policy did not produce a fresh generation, nonce, and full proof")
			}
			for _, artifact := range []codehost.ReviewArtifact{old.Aggregate, old.Request} {
				if fixtureGit(t, o.Origin, "rev-parse", artifact.Identity.Ref) != artifact.Identity.OID {
					t.Fatal("fresh review replaced retained old proof")
				}
			}
			if fixtureGit(t, o.Origin, "rev-parse", proposals.targetRef()) != old.Binding.TargetOID || fixtureGit(t, o.Origin, "rev-parse", old.Binding.Branch) != old.Binding.Head {
				t.Fatal("review refresh moved the feature or task branch")
			}
			if _, err := receipts.Verify(t.Context(), old.Binding); err == nil {
				t.Fatal("old policy approval remained valid")
			}
			if _, err := receipts.Verify(t.Context(), fresh.Binding); !errors.Is(err, codehost.ErrReceiptNotFound) {
				t.Fatalf("fresh review inherited approval: %v", err)
			}
			if _, err := receipts.Record(t.Context(), fresh.Binding, "operator", "reviewed current policy"); err != nil {
				t.Fatal(err)
			}
			reader.failSnapshot = true
			before = fixtureGit(t, o.Origin, "show-ref")
			if _, err := requests.Prepare(t.Context(), opened.Proposal.ID); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("unavailable current aggregate reused or replaced a prepared request")
			}
			reader.failSnapshot = false
			readsBeforeResume := reader.snapshots
			again, err := requests.Prepare(t.Context(), opened.Proposal.ID)
			if err != nil || !reflect.DeepEqual(again, fresh) || reader.snapshots != readsBeforeResume+1 || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("current retained review was not resumed: %v", err)
			}
		})
	}
}
