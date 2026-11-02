package featuredrive

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func completedEffect(d FeatureDrive) EffectIntent {
	const kind, subject, input = "request-task-proposal", "T1", "sha256:input"
	return EffectIntent{ID: EffectID(d.Repository, d.FeatureID, d.Ownership.Epoch, kind, subject, input), Kind: kind, Subject: subject,
		InputDigest: input, RequiredEpoch: d.Ownership.Epoch, Status: Observed,
		Observation: &EvidenceRef{Kind: "proposal", ID: "retained-proposal", Digest: "sha256:observed"}}
}

func TestOwnershipTransitionsRetainObservedEffectHistory(t *testing.T) {
	for _, action := range []string{"handoff", "cancellation"} {
		t.Run(action, func(t *testing.T) {
			d := fixture()
			d.Effects = []EffectIntent{completedEffect(d)}
			store := NewMemoryStore()
			if err := store.Create(t.Context(), d); err != nil {
				t.Fatal(err)
			}
			lifecycle := OwnershipLifecycle{Store: store, Quiescer: testQuiescer{complete: true}}
			owner := OwnerEpoch{Owner: Owner{Domain: d.Ownership.Domain, ID: d.Ownership.OwnerID}, Epoch: d.Ownership.Epoch}
			var next FeatureDrive
			var err error
			if action == "handoff" {
				next, err = lifecycle.RequestHandoff(t.Context(), RequestHandoff{FeatureID: d.FeatureID, From: owner, To: ExecutionAssignment{Domain: "remote", OwnerID: "worker-2"}})
			} else {
				next, err = lifecycle.RequestCancellation(t.Context(), RequestCancellation{FeatureID: d.FeatureID, Owner: owner})
			}
			if err != nil {
				t.Fatalf("completed effect prevented %s: %v", action, err)
			}
			retained, err := store.Load(t.Context(), d.FeatureID)
			if err != nil || next.Validate() != nil || retained.Ownership.Epoch != owner.Epoch+1 || !reflect.DeepEqual(retained.Effects, d.Effects) {
				t.Fatalf("transition changed completed effect identity or evidence: %+v, %v", retained, err)
			}
			if action == "handoff" {
				executor := &countingEffect{}
				reconciler := Reconciler{Store: store, Executor: executor, Plan: func(context.Context, FeatureDrive, string) ([]EffectIntent, error) { return nil, nil }}
				if err := reconciler.Reconcile(t.Context(), d.FeatureID, "push"); err != nil || executor.calls.Load() != 0 {
					t.Fatalf("new owner replayed completed history: calls=%d, %v", executor.calls.Load(), err)
				}
			} else {
				if next.Phase != Cancelled || next.Ownership.State != Released {
					t.Fatalf("cancellation did not release ownership: %+v", next)
				}
				resumed, err := lifecycle.ResumeFeature(t.Context(), ResumeFeature{FeatureID: d.FeatureID})
				if err != nil || resumed.Phase != Integrating || !reflect.DeepEqual(resumed.Effects, d.Effects) {
					t.Fatalf("resume lost completed effect history: %+v, %v", resumed, err)
				}
			}
		})
	}
}

func TestDeliveredStateRetainsCompletedEffectsAtTheirExecutionEpoch(t *testing.T) {
	d := fixture()
	d.Phase = DeliveryVerifying
	d.Effects = []EffectIntent{completedEffect(d)}
	store := NewMemoryStore()
	if err := store.Create(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	next := d
	next.Version++
	next.Phase = Delivered
	next.Ownership = Ownership{State: Released, Epoch: d.Ownership.Epoch + 1}
	next.Last = Transition{Kind: "effect-observed", PriorVersion: d.Version, NewVersion: next.Version, PriorEpoch: d.Ownership.Epoch, NewEpoch: next.Ownership.Epoch}
	if err := store.CAS(t.Context(), d.FeatureID, d.Version, next); err != nil {
		t.Fatalf("completed effect prevented delivered-state CAS: %v", err)
	}
	retained, err := store.Load(t.Context(), d.FeatureID)
	if err != nil || retained.Phase != Delivered || !reflect.DeepEqual(retained.Effects, d.Effects) {
		t.Fatalf("delivered state lost completed effect history: %+v, %v", retained, err)
	}
}

func TestEffectHistoryRejectsStalePendingAndFutureEpochs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status EffectStatus
		epoch  uint64
	}{
		{name: "stale pending", status: Pending, epoch: 1},
		{name: "future pending", status: Pending, epoch: 3},
		{name: "future observation", status: Observed, epoch: 3},
		{name: "unassigned observation", status: Observed, epoch: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := fixture()
			d.Ownership.Epoch = 2
			effect := completedEffect(d)
			effect.RequiredEpoch, effect.Status = tc.epoch, tc.status
			d.Effects = []EffectIntent{effect}
			if err := d.Validate(); !errors.Is(err, ErrMalformed) {
				t.Fatalf("invalid effect epoch was accepted: %+v, %v", effect, err)
			}
			body, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Decode(body); !errors.Is(err, ErrMalformed) {
				t.Fatalf("decoded invalid effect epoch: %+v, %v", effect, err)
			}
			if err := NewMemoryStore().Create(t.Context(), d); !errors.Is(err, ErrMalformed) {
				t.Fatalf("stored invalid effect epoch: %+v, %v", effect, err)
			}
		})
	}
}
