package featuredrive

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestOwnershipLifecycleRequiresTypedQuiescenceEvidence(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"handoff", "cancel"} {
		for _, tc := range []struct {
			name string
			ref  EvidenceRef
		}{
			{"zero", EvidenceRef{}},
			{"missing kind", EvidenceRef{ID: "attempt-one"}},
			{"missing id", EvidenceRef{Kind: "checkpoint"}},
			{"blank", EvidenceRef{Kind: " ", ID: "\t"}},
			{"valid typed id", EvidenceRef{Kind: "checkpoint", ID: "attempt-one"}},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				initial := fixture()
				store := NewMemoryStore()
				if err := store.Create(t.Context(), initial); err != nil {
					t.Fatal(err)
				}
				lifecycle := OwnershipLifecycle{Store: store, Quiescer: quiescerFunc(func(context.Context, string, Owner, uint64) (CheckpointEvidence, error) {
					return CheckpointEvidence{AttemptID: "attempt-one", Complete: true, Evidence: tc.ref}, nil
				})}
				owner := OwnerEpoch{Owner: Owner{Domain: initial.Ownership.Domain, ID: initial.Ownership.OwnerID}, Epoch: initial.Ownership.Epoch}
				var err error
				if operation == "handoff" {
					_, err = lifecycle.RequestHandoff(t.Context(), RequestHandoff{FeatureID: initial.FeatureID, From: owner, To: ExecutionAssignment{Domain: "remote", OwnerID: "worker-two"}})
				} else {
					_, err = lifecycle.RequestCancellation(t.Context(), RequestCancellation{FeatureID: initial.FeatureID, Owner: owner})
				}
				retained, loadErr := store.Load(t.Context(), initial.FeatureID)
				if loadErr != nil {
					t.Fatal(loadErr)
				}
				if tc.name == "valid typed id" {
					if err != nil || retained.Ownership.Epoch != owner.Epoch+1 || len(retained.Evidence) != 1 || retained.Evidence[0] != tc.ref {
						t.Fatalf("valid checkpoint without optional digest was refused: %+v %v", retained, err)
					}
					return
				}
				if !errors.Is(err, ErrQuiescence) || retained.Ownership.Epoch != owner.Epoch || retained.Ownership.OwnerID != owner.Owner.ID || !reflect.DeepEqual(retained.Evidence, initial.Evidence) || retained.Blocker == nil || retained.Blocker.ID != "attempt-one" {
					t.Fatalf("malformed checkpoint released the owner instead of retaining its blocker: %+v %v", retained, err)
				}
				if operation == "handoff" && retained.Ownership.State != Quiescing || operation == "cancel" && retained.Phase != CancelPending {
					t.Fatal("malformed checkpoint permitted execution or completed cancellation")
				}
			})
		}
	}
}
