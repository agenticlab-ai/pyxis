package featuredrive

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTouchRouterFencesStaleOwner(t *testing.T) {
	s := NewMemoryStore()
	if err := s.Create(context.Background(), fixture()); err != nil {
		t.Fatal(err)
	}
	called := false
	r := TouchRouter{Store: s, Reconcile: func(context.Context, string, string) error { called = true; return nil }}
	err := r.Route(context.Background(), DriveTouch{FeatureID: "f-1", Reason: TouchPush, Owner: OwnerEpoch{Owner: Owner{Domain: "remote", ID: "other"}, Epoch: 1}})
	if !errors.Is(err, ErrStaleEpoch) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestNativeTouchesNormalizeToInstructionFreeSignals(t *testing.T) {
	owner := OwnerEpoch{Owner: Owner{Domain: "local", ID: "machine"}, Epoch: 3}
	at := time.Unix(10, 0)
	push := PushTouch("f-1", owner, at)
	goal := EvidenceRef{Kind: "review", ID: "r1", Digest: "d"}
	manual := ManualTouch("f-1", owner, goal, at)
	if push.Reason != TouchPush || push.Goal != nil || push.Owner != owner || !push.At.Equal(at) {
		t.Fatalf("push=%+v", push)
	}
	if manual.Reason != TouchManual || manual.Goal == nil || *manual.Goal != goal {
		t.Fatalf("manual=%+v", manual)
	}
}
func TestOwnerBackstopOnlyScansAssignedWaits(t *testing.T) {
	s := NewMemoryStore()
	d := fixture()
	d.Wait = &Wait{Kind: "review", Goal: EvidenceRef{Kind: "review", ID: "r1"}}
	if err := s.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	other := fixture()
	other.FeatureID = "f-2"
	other.Ownership.OwnerID = "other"
	other.Wait = d.Wait
	if err := s.Create(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	b := OwnerBackstop{Store: s, Domain: "local", OwnerID: "machine", Now: func() time.Time { return time.Unix(1, 0) }}
	got, err := b.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].FeatureID != "f-1" || got[0].Reason != TouchBackstop {
		t.Fatalf("got=%+v", got)
	}
}
func TestStatusIsColdReadableAndShowsFailureAndPendingEffect(t *testing.T) {
	s := NewMemoryStore()
	d := fixture()
	d.Effects = []EffectIntent{{ID: "e1", Kind: "dispatch", Subject: "task", RequiredEpoch: 1, Status: Pending}}
	d.Failure = &FailureDisposition{Trigger: "landing-unverified", Disposition: "blocked: landing-verification", Remedy: "correct or dispose proposal", ClearCondition: "re-observe landing proof"}
	d.Frontier = []string{"T8"}
	d.SecondaryFailures = []FailureDisposition{{Trigger: "candidate-stale"}}
	if err := s.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	got, err := ReadStatus(context.Background(), s, "f-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.PendingEffects) != 1 || len(got.Frontier) != 1 || got.PrimaryFailure == nil || got.Remedy == "" || got.ClearCondition == "" || len(got.SecondaryFailures) != 1 {
		t.Fatalf("status=%+v", got)
	}
	n, ok := NotificationForStatus(got)
	if !ok || n.FeatureID != "f-1" || n.Severity != "action-required" || n.Clear == "" || n.Correlation.GoalID != "" {
		t.Fatalf("notification=%+v ok=%v", n, ok)
	}
}
func TestMetricsDeterministicLifecycleHasZeroInference(t *testing.T) {
	var m Metrics
	m.RecordTouch(true)
	touches, noops, tokens, workers := m.Snapshot()
	if touches != 1 || noops != 1 || tokens != 0 || workers != 0 {
		t.Fatalf("metrics=%d,%d,%d,%d", touches, noops, tokens, workers)
	}
}

func TestMetricsTrackInferenceAndNeverNegativeWorkers(t *testing.T) {
	var m Metrics
	m.RecordInference(17)
	m.SetLiveWorkers(-1)
	_, _, tokens, workers := m.Snapshot()
	if tokens != 17 || workers != 0 {
		t.Fatalf("metrics=%d,%d", tokens, workers)
	}
}
