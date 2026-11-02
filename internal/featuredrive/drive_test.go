package featuredrive

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func fixture() FeatureDrive {
	t := Transition{Kind: "create", NewVersion: 1}
	return FeatureDrive{Schema: Schema, Repository: "repo", FeatureID: "f-1", PlanFingerprint: "plan", TargetMainRef: "refs/heads/main", IntegrationRef: "refs/heads/feature/f-1", Version: 1, Created: t, Last: t, Ownership: Ownership{State: Active, Domain: "local", OwnerID: "machine", Epoch: 1}, Phase: Integrating}
}

type countingEffect struct {
	calls atomic.Int32
	mu    sync.Mutex
	seen  map[string]bool
}

func (e *countingEffect) Execute(_ context.Context, in EffectIntent) (EvidenceRef, error) {
	e.calls.Add(1)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.seen == nil {
		e.seen = map[string]bool{}
	}
	e.seen[in.ID] = true
	return EvidenceRef{Kind: in.Kind, ID: in.ID, Digest: in.InputDigest}, nil
}

func TestDecodeRejectsUnknownAndMalformedLineage(t *testing.T) {
	d := fixture()
	b := mustJSON(t, d)
	unknown := append(append([]byte(nil), b[:len(b)-1]...), []byte(`,"poison":true}`)...)
	if _, err := Decode(unknown); !errors.Is(err, ErrMalformed) {
		t.Fatalf("unknown field: %v", err)
	}
	if _, err := Decode(append(b, []byte(` {}`)...)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("trailing value: %v", err)
	}
	d.Last.PriorVersion = 1
	if err := d.Validate(); !errors.Is(err, ErrMalformed) {
		t.Fatal(err)
	}
}

func TestFeatureDriveValidationAndMemoryStoreBoundaries(t *testing.T) {
	base := fixture()
	mutations := []func(*FeatureDrive){
		func(d *FeatureDrive) { d.Schema = "" },
		func(d *FeatureDrive) { d.Created.NewVersion = 2 },
		func(d *FeatureDrive) { d.Ownership.OwnerID = "" },
		func(d *FeatureDrive) { d.Ownership = Ownership{State: Unowned, OwnerID: "unexpected"} },
		func(d *FeatureDrive) { d.Phase = Phase("unknown") },
		func(d *FeatureDrive) { d.Effects = []EffectIntent{{Status: Pending}} },
		func(d *FeatureDrive) { d.Frontier = []string{""} },
		func(d *FeatureDrive) {
			e := EffectIntent{ID: "same", Kind: "dispatch", RequiredEpoch: d.Ownership.Epoch, Status: Pending}
			d.Effects = []EffectIntent{e, e}
		},
		func(d *FeatureDrive) {
			d.Effects = []EffectIntent{{ID: "observed", Kind: "dispatch", RequiredEpoch: d.Ownership.Epoch, Status: Observed}}
		},
		func(d *FeatureDrive) {
			d.Ownership = Ownership{State: Unowned}
			d.Phase = Candidate
		},
	}
	for i, mutate := range mutations {
		drive := base
		mutate(&drive)
		if err := drive.Validate(); !errors.Is(err, ErrMalformed) {
			t.Fatalf("invalid drive mutation %d error = %v", i, err)
		}
	}
	if _, err := NewMemoryStore().Load(t.Context(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing drive error = %v", err)
	}
}
func TestReconcilePersistsIntentBeforeEffectAndConverges(t *testing.T) {
	s := NewMemoryStore()
	d := fixture()
	if err := s.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	e := &countingEffect{}
	r := Reconciler{Store: s, Executor: e, Plan: func(_ context.Context, d FeatureDrive, _ string) ([]EffectIntent, error) {
		if len(d.Effects) > 0 {
			return nil, nil
		}
		return []EffectIntent{{Kind: "dispatch", Subject: "task-1", InputDigest: "abc"}}, nil
	}}
	if err := r.Reconcile(context.Background(), d.FeatureID, "push"); err != nil {
		t.Fatal(err)
	}
	if err := r.Reconcile(context.Background(), d.FeatureID, "duplicate"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Load(context.Background(), d.FeatureID)
	if got.Effects[0].Status != Observed || e.calls.Load() != 1 {
		t.Fatalf("got %+v calls=%d", got, e.calls.Load())
	}
}
func TestConcurrentReconcileOneIntent(t *testing.T) {
	s := NewMemoryStore()
	d := fixture()
	if err := s.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	e := &countingEffect{}
	r := Reconciler{Store: s, Executor: e, Plan: func(_ context.Context, d FeatureDrive, _ string) ([]EffectIntent, error) {
		if len(d.Effects) > 0 {
			return nil, nil
		}
		return []EffectIntent{{Kind: "dispatch", Subject: "task-1", InputDigest: "abc"}}, nil
	}}
	var wg sync.WaitGroup
	for range 2 {
		// A losing contender may conflict; the retained intent is checked below.
		wg.Go(func() { _ = r.Reconcile(context.Background(), d.FeatureID, "touch") })
	}
	wg.Wait()
	got, _ := s.Load(context.Background(), d.FeatureID)
	if len(got.Effects) != 1 {
		t.Fatalf("effects=%d", len(got.Effects))
	}
}

type testQuiescer struct{ complete bool }

func (q testQuiescer) Quiesce(context.Context, string, Owner, uint64) (CheckpointEvidence, error) {
	return CheckpointEvidence{AttemptID: "attempt-1", Complete: q.complete, Evidence: EvidenceRef{Kind: "checkpoint", ID: "attempt-1", Digest: "d"}}, nil
}

type ownershipStore struct {
	d       FeatureDrive
	loadErr error
	casErr  error
}

func (s *ownershipStore) Load(context.Context, string) (FeatureDrive, error) { return s.d, s.loadErr }
func (s *ownershipStore) Create(context.Context, FeatureDrive) error         { return nil }
func (s *ownershipStore) List(context.Context) ([]FeatureDrive, error) {
	return []FeatureDrive{s.d}, nil
}
func (s *ownershipStore) CAS(_ context.Context, _ string, _ uint64, d FeatureDrive) error {
	if s.casErr != nil {
		return s.casErr
	}
	s.d = d
	return nil
}

type quiescerFunc func(context.Context, string, Owner, uint64) (CheckpointEvidence, error)

func (f quiescerFunc) Quiesce(ctx context.Context, id string, owner Owner, epoch uint64) (CheckpointEvidence, error) {
	return f(ctx, id, owner, epoch)
}

type routingReleaserFunc func(context.Context, FeatureRoutingScope) error

func (f routingReleaserFunc) Release(ctx context.Context, scope FeatureRoutingScope) error {
	return f(ctx, scope)
}

func TestOwnershipLifecycleFailsClosedAcrossBoundaryErrors(t *testing.T) {
	stop := errors.New("boundary stopped")
	active := fixture()

	if _, err := (OwnershipLifecycle{Store: &ownershipStore{loadErr: stop}}).RequestHandoff(t.Context(), RequestHandoff{FeatureID: active.FeatureID}); !errors.Is(err, stop) {
		t.Fatalf("handoff load error = %v", err)
	}
	if _, err := (OwnershipLifecycle{Store: &ownershipStore{d: active}}).RequestHandoff(t.Context(), RequestHandoff{FeatureID: active.FeatureID, From: OwnerEpoch{Epoch: 99}}); !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("handoff stale error = %v", err)
	}
	if _, err := (OwnershipLifecycle{Store: &ownershipStore{d: active}}).RequestHandoff(t.Context(), RequestHandoff{FeatureID: active.FeatureID, From: OwnerEpoch{Owner: Owner{ID: "machine"}, Epoch: 1}, To: ExecutionAssignment{Domain: "future", OwnerID: "worker"}}); err == nil {
		t.Fatal("invalid handoff target was accepted")
	}
	if _, err := (OwnershipLifecycle{Store: &ownershipStore{d: active, casErr: stop}}).RequestHandoff(t.Context(), RequestHandoff{FeatureID: active.FeatureID, From: OwnerEpoch{Owner: Owner{ID: "machine"}, Epoch: 1}, To: ExecutionAssignment{Domain: "remote", OwnerID: "worker"}}); !errors.Is(err, stop) {
		t.Fatalf("handoff CAS error = %v", err)
	}
	if _, err := (OwnershipLifecycle{Store: &ownershipStore{d: active}, Routing: routingReleaserFunc(func(context.Context, FeatureRoutingScope) error { return stop })}).RequestHandoff(t.Context(), RequestHandoff{FeatureID: active.FeatureID, From: OwnerEpoch{Owner: Owner{ID: "machine"}, Epoch: 1}, To: ExecutionAssignment{Domain: "remote", OwnerID: "worker"}}); !errors.Is(err, stop) {
		t.Fatalf("handoff release error = %v", err)
	}

	for name, lifecycle := range map[string]OwnershipLifecycle{
		"missing quiescer": {Store: &ownershipStore{d: active}},
		"quiescer error": {Store: &ownershipStore{d: active}, Quiescer: quiescerFunc(func(context.Context, string, Owner, uint64) (CheckpointEvidence, error) {
			return CheckpointEvidence{AttemptID: "attempt"}, stop
		})},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := lifecycle.RequestCancellation(t.Context(), RequestCancellation{FeatureID: active.FeatureID, Owner: OwnerEpoch{Owner: Owner{Domain: "local", ID: "machine"}, Epoch: 1}})
			if !errors.Is(err, ErrQuiescence) {
				t.Fatalf("cancellation error = %v", err)
			}
		})
	}
	if _, err := (OwnershipLifecycle{Store: &ownershipStore{d: active, casErr: stop}}).RequestCancellation(t.Context(), RequestCancellation{FeatureID: active.FeatureID, Owner: OwnerEpoch{Owner: Owner{Domain: "local", ID: "machine"}, Epoch: 1}}); !errors.Is(err, stop) {
		t.Fatalf("cancellation CAS error = %v", err)
	}
	if _, err := (OwnershipLifecycle{Store: &ownershipStore{d: active}}).RequestCancellation(t.Context(), RequestCancellation{FeatureID: active.FeatureID, Owner: OwnerEpoch{Epoch: 99}}); !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("cancellation stale error = %v", err)
	}
	if _, err := (OwnershipLifecycle{Store: &ownershipStore{loadErr: stop}}).AbandonFeature(t.Context(), AbandonFeature{FeatureID: active.FeatureID}); !errors.Is(err, stop) {
		t.Fatalf("abandon load error = %v", err)
	}
	if _, err := (OwnershipLifecycle{Store: &ownershipStore{d: active}}).ResumeFeature(t.Context(), ResumeFeature{FeatureID: active.FeatureID}); !errors.Is(err, ErrIneligible) {
		t.Fatalf("resume ineligible error = %v", err)
	}
}

func TestHandoffQuiescesBeforeChangingEpoch(t *testing.T) {
	s := NewMemoryStore()
	if err := s.Create(context.Background(), fixture()); err != nil {
		t.Fatal(err)
	}
	l := OwnershipLifecycle{Store: s, Quiescer: testQuiescer{complete: true}}
	d, err := l.RequestHandoff(context.Background(), RequestHandoff{FeatureID: "f-1", From: OwnerEpoch{Owner: Owner{Domain: "local", ID: "machine"}, Epoch: 1}, To: ExecutionAssignment{Domain: "remote", OwnerID: "worker-2"}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Ownership.OwnerID != "worker-2" || d.Ownership.Epoch != 2 || len(d.Evidence) != 1 {
		t.Fatalf("handoff=%+v", d)
	}
}

func TestFailedHandoffRetainsOwnerAndRecordsBlocker(t *testing.T) {
	s := NewMemoryStore()
	if err := s.Create(context.Background(), fixture()); err != nil {
		t.Fatal(err)
	}
	l := OwnershipLifecycle{Store: s, Quiescer: testQuiescer{complete: false}}
	_, err := l.RequestHandoff(context.Background(), RequestHandoff{FeatureID: "f-1", From: OwnerEpoch{Owner: Owner{Domain: "local", ID: "machine"}, Epoch: 1}, To: ExecutionAssignment{Domain: "remote", OwnerID: "worker-2"}})
	if !errors.Is(err, ErrQuiescence) {
		t.Fatalf("err=%v", err)
	}
	d, _ := s.Load(context.Background(), "f-1")
	if d.Ownership.OwnerID != "machine" || d.Ownership.Epoch != 1 || d.Blocker == nil {
		t.Fatalf("retained=%+v", d)
	}
}

func TestCancelResumeAndAbandonAreNonDestructive(t *testing.T) {
	s := NewMemoryStore()
	if err := s.Create(context.Background(), fixture()); err != nil {
		t.Fatal(err)
	}
	l := OwnershipLifecycle{Store: s, Quiescer: testQuiescer{complete: true}}
	d, err := l.RequestCancellation(context.Background(), RequestCancellation{FeatureID: "f-1", Owner: OwnerEpoch{Owner: Owner{Domain: "local", ID: "machine"}, Epoch: 1}})
	if err != nil || d.Phase != Cancelled || d.Ownership.State != Released {
		t.Fatalf("cancel=%+v err=%v", d, err)
	}
	d, err = l.ResumeFeature(context.Background(), ResumeFeature{FeatureID: "f-1"})
	if err != nil || d.Phase != Integrating || d.Ownership.State != Released {
		t.Fatalf("resume=%+v err=%v", d, err)
	}
	d, err = l.AbandonFeature(context.Background(), AbandonFeature{FeatureID: "f-1"})
	if err == nil {
		t.Fatal("abandon after resume should be refused")
	}
}

func TestCancellationRetryClearsRecoveredBlocker(t *testing.T) {
	s := NewMemoryStore()
	drive := fixture()
	if err := s.Create(t.Context(), drive); err != nil {
		t.Fatal(err)
	}
	lifecycle := OwnershipLifecycle{Store: s, Quiescer: testQuiescer{complete: false}}
	request := RequestCancellation{FeatureID: drive.FeatureID, Owner: OwnerEpoch{Owner: Owner{Domain: "local", ID: "machine"}, Epoch: 1}}
	blocked, err := lifecycle.RequestCancellation(t.Context(), request)
	if !errors.Is(err, ErrQuiescence) || blocked.Phase != CancelPending || blocked.Blocker == nil || blocked.Ownership.Epoch != 1 {
		t.Fatalf("incomplete attempt did not retain its cancellation blocker: %+v, %v", blocked, err)
	}
	lifecycle.Quiescer = testQuiescer{complete: true}
	cancelled, err := lifecycle.RequestCancellation(t.Context(), request)
	if err != nil || cancelled.Phase != Cancelled || cancelled.Blocker != nil || cancelled.Ownership.State != Released || cancelled.Ownership.Epoch != 2 {
		t.Fatalf("successful retry retained a resolved blocker: %+v, %v", cancelled, err)
	}
	resumed, err := lifecycle.ResumeFeature(t.Context(), ResumeFeature{FeatureID: drive.FeatureID})
	if err != nil || resumed.Phase != Integrating || resumed.Blocker != nil || resumed.Ownership != cancelled.Ownership {
		t.Fatalf("resume retained obsolete cancellation state: %+v, %v", resumed, err)
	}
}

func mustJSON(t *testing.T, d FeatureDrive) []byte {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
