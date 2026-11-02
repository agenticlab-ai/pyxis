package featuredrive

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type observationResultEffect struct {
	ref   EvidenceRef
	calls int
}

func (e *observationResultEffect) Execute(ctx context.Context, _ EffectIntent) (EvidenceRef, error) {
	e.calls++
	return e.ref, ctx.Err()
}

func TestReconcileRejectsEmptyObservationBeforeSharedStateMutation(t *testing.T) {
	s := NewMemoryStore()
	d := fixture()
	d.Effects = []EffectIntent{{ID: "pending", Kind: "dispatch", RequiredEpoch: d.Ownership.Epoch, Status: Pending}}
	if err := s.Create(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	e := &observationResultEffect{}
	r := Reconciler{Store: s, Executor: e, Plan: func(context.Context, FeatureDrive, string) ([]EffectIntent, error) { return nil, nil }}
	if err := r.Reconcile(t.Context(), d.FeatureID, "observe"); !errors.Is(err, ErrMalformed) {
		t.Fatalf("empty executor observation was accepted: %v", err)
	}
	got, err := s.Load(t.Context(), d.FeatureID)
	if err != nil || !reflect.DeepEqual(got, d) {
		t.Fatalf("invalid result mutated the shared pending effect before CAS: %+v %v", got, err)
	}
	e.ref = EvidenceRef{Kind: "task-proposal", ID: "actual-goal"}
	if err := r.Reconcile(t.Context(), d.FeatureID, "retry"); err != nil {
		t.Fatal(err)
	}
	if err := r.Reconcile(t.Context(), d.FeatureID, "already observed"); err != nil || e.calls != 2 {
		t.Fatalf("valid retry did not converge: calls=%d %v", e.calls, err)
	}
	got, err = s.Load(t.Context(), d.FeatureID)
	if err != nil || got.Effects[0].Status != Observed || got.Effects[0].Observation == nil || *got.Effects[0].Observation != e.ref {
		t.Fatalf("valid retry did not retain its goal observation: %+v %v", got, err)
	}
}
