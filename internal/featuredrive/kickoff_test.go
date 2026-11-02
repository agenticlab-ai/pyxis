package featuredrive

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type kickoffIdentity struct{}

func (kickoffIdentity) Authenticate(_ context.Context, a ExecutionAssignment) Owner {
	return Owner{Domain: "authenticated-" + a.Domain, ID: "auth-" + a.OwnerID}
}

type kickoffRefs struct{ calls atomic.Int32 }

func (r *kickoffRefs) CreateOnly(_ context.Context, _, _ string) error { r.calls.Add(1); return nil }

type kickoffRouting struct{ calls atomic.Int32 }

func (r *kickoffRouting) Project(_ context.Context, _ FrozenFeature, s FeatureRoutingScope) ([]port.Task, error) {
	r.calls.Add(1)
	return []port.Task{{ID: "task-1", HeadBranch: s.IntegrationRef}}, nil
}

type kickoffExecutor struct {
	calls      atomic.Int32
	assignment ExecutionAssignment
	mu         sync.Mutex
}

func (e *kickoffExecutor) Execute(_ context.Context, _ []port.Task, a ExecutionAssignment, _ FeatureRoutingScope) error {
	e.calls.Add(1)
	e.mu.Lock()
	e.assignment = a
	e.mu.Unlock()
	return nil
}

type kickoffResolver struct {
	feature FrozenFeature
	err     error
}

func (r kickoffResolver) Resolve(_ context.Context, _ string) (FrozenFeature, error) {
	return r.feature, r.err
}

type emptyKickoffIdentity struct{}

func (emptyKickoffIdentity) Authenticate(context.Context, ExecutionAssignment) Owner { return Owner{} }

func kickoffFixture() (FeatureDrive, FrozenFeature) {
	t := Transition{Kind: "create", NewVersion: 1}
	d := FeatureDrive{Schema: Schema, Repository: "repo", FeatureID: "f-1", PlanFingerprint: "plan", TargetMainRef: "refs/heads/main", IntegrationRef: "refs/heads/feature/f-1", Version: 1, Created: t, Last: t, Ownership: Ownership{State: Unowned}, Phase: Integrating}
	f := FrozenFeature{ID: "f-1", Repository: "repo", PlanFingerprint: "plan", TargetMainRef: "refs/heads/main", Eligible: true}
	return d, f
}

func TestKickoffAuthenticatesAndRoutesOnlyAfterAssignment(t *testing.T) {
	d, f := kickoffFixture()
	store := NewMemoryStore()
	if err := store.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	refs, routing, executor := &kickoffRefs{}, &kickoffRouting{}, &kickoffExecutor{}
	k := Kickoff{Resolver: kickoffResolver{feature: f}, Identity: kickoffIdentity{}, Refs: refs, Routing: routing, Executor: executor, Drives: store}
	result, err := k.KickoffFeature(context.Background(), f.ID, ExecutionAssignment{Domain: "local", OwnerID: "machine", Policy: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scope.Owner.Domain != "authenticated-local" || result.Scope.BaseRef != result.Scope.IntegrationRef || result.Scope.Policy != "p" || executor.assignment.OwnerID != "auth-machine" {
		t.Fatalf("authentication not propagated: scope=%+v assignment=%+v", result.Scope, executor.assignment)
	}
	if routing.calls.Load() != 1 || executor.calls.Load() != 1 {
		t.Fatalf("routing=%d executor=%d", routing.calls.Load(), executor.calls.Load())
	}
}

func TestKickoffMismatchedProjectedDriveIsInert(t *testing.T) {
	d, f := kickoffFixture()
	f.PlanFingerprint = "different-plan"
	store := NewMemoryStore()
	if err := store.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	refs, routing, executor := &kickoffRefs{}, &kickoffRouting{}, &kickoffExecutor{}
	k := Kickoff{Resolver: kickoffResolver{feature: f}, Identity: kickoffIdentity{}, Refs: refs, Routing: routing, Executor: executor, Drives: store}
	if _, err := k.KickoffFeature(context.Background(), f.ID, ExecutionAssignment{Domain: "local", OwnerID: "machine"}); !errors.Is(err, ErrIneligible) {
		t.Fatalf("err=%v", err)
	}
	if refs.calls.Load() != 0 || routing.calls.Load() != 0 || executor.calls.Load() != 0 {
		t.Fatal("mismatched kickoff performed side effects")
	}
}

func TestKickoffConcurrentConflictsHaveOneOwner(t *testing.T) {
	d, f := kickoffFixture()
	store := NewMemoryStore()
	if err := store.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	refs, routing, executor := &kickoffRefs{}, &kickoffRouting{}, &kickoffExecutor{}
	k := Kickoff{Resolver: kickoffResolver{feature: f}, Identity: kickoffIdentity{}, Refs: refs, Routing: routing, Executor: executor, Drives: store}
	var wg sync.WaitGroup
	var successes atomic.Int32
	var conflicts atomic.Int32
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := k.KickoffFeature(context.Background(), f.ID, ExecutionAssignment{Domain: "remote", OwnerID: "worker", Policy: "p"})
			if err == nil {
				successes.Add(1)
			} else if errors.Is(err, ErrOwnershipConflict) {
				conflicts.Add(1)
			} else {
				t.Errorf("unexpected kickoff error: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 || conflicts.Load() != 99 {
		t.Fatalf("successes=%d conflicts=%d", successes.Load(), conflicts.Load())
	}
	if executor.calls.Load() != 1 || routing.calls.Load() != 1 || refs.calls.Load() != 1 {
		t.Fatalf("side effects refs=%d routing=%d executor=%d", refs.calls.Load(), routing.calls.Load(), executor.calls.Load())
	}
}

func TestKickoffInvalidIsInert(t *testing.T) {
	d, f := kickoffFixture()
	f.Eligible = false
	store := NewMemoryStore()
	if err := store.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	refs, routing, executor := &kickoffRefs{}, &kickoffRouting{}, &kickoffExecutor{}
	k := Kickoff{Resolver: kickoffResolver{feature: f}, Identity: kickoffIdentity{}, Refs: refs, Routing: routing, Executor: executor, Drives: store}
	if _, err := k.KickoffFeature(context.Background(), f.ID, ExecutionAssignment{Domain: "local", OwnerID: "machine"}); !errors.Is(err, ErrIneligible) {
		t.Fatalf("err=%v", err)
	}
	if refs.calls.Load() != 0 || routing.calls.Load() != 0 || executor.calls.Load() != 0 {
		t.Fatal("invalid kickoff performed side effects")
	}
}

func TestKickoffRejectsUnsupportedDomainBeforeResolving(t *testing.T) {
	d, f := kickoffFixture()
	store := NewMemoryStore()
	if err := store.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	refs, routing, executor := &kickoffRefs{}, &kickoffRouting{}, &kickoffExecutor{}
	resolver := &countingResolver{feature: f}
	k := Kickoff{Resolver: resolver, Identity: kickoffIdentity{}, Refs: refs, Routing: routing, Executor: executor, Drives: store}
	if _, err := k.KickoffFeature(context.Background(), f.ID, ExecutionAssignment{Domain: "unknown", OwnerID: "worker"}); !errors.Is(err, ErrUnassigned) {
		t.Fatalf("err=%v", err)
	}
	if resolver.calls.Load() != 0 || refs.calls.Load() != 0 || routing.calls.Load() != 0 || executor.calls.Load() != 0 {
		t.Fatal("unsupported domain reached kickoff dependencies")
	}
}

type countingResolver struct {
	feature FrozenFeature
	calls   atomic.Int32
}

func (r *countingResolver) Resolve(_ context.Context, _ string) (FrozenFeature, error) {
	r.calls.Add(1)
	return r.feature, nil
}

func TestKickoffRejectsMissingResolverWithoutPanic(t *testing.T) {
	_, err := (Kickoff{}).KickoffFeature(context.Background(), "f-1", ExecutionAssignment{Domain: "local", OwnerID: "machine"})
	if err == nil {
		t.Fatal("missing resolver was accepted")
	}
}

func TestKickoffBoundaryFailures(t *testing.T) {
	drive, feature := kickoffFixture()
	refs, routing, executor := &kickoffRefs{}, &kickoffRouting{}, &kickoffExecutor{}
	assignment := ExecutionAssignment{Domain: "local", OwnerID: "machine"}

	if _, err := (Kickoff{}).KickoffFeature(t.Context(), feature.ID, ExecutionAssignment{Domain: "local"}); !errors.Is(err, ErrUnassigned) {
		t.Fatalf("missing owner error = %v", err)
	}

	resolveErr := errors.New("resolve failed")
	store := NewMemoryStore()
	k := Kickoff{Resolver: kickoffResolver{err: resolveErr}, Identity: kickoffIdentity{}, Refs: refs, Routing: routing, Executor: executor, Drives: store}
	if _, err := k.KickoffFeature(t.Context(), feature.ID, assignment); !errors.Is(err, resolveErr) {
		t.Fatalf("resolver error = %v", err)
	}

	k.Resolver = kickoffResolver{feature: feature}
	k.Identity = emptyKickoffIdentity{}
	if _, err := k.KickoffFeature(t.Context(), feature.ID, assignment); !errors.Is(err, ErrUnassigned) {
		t.Fatalf("authentication error = %v", err)
	}

	k.Identity = kickoffIdentity{}
	if _, err := k.KickoffFeature(t.Context(), feature.ID, assignment); err == nil {
		t.Fatal("kickoff hid the missing durable drive")
	}

	store = NewMemoryStore()
	drive.Phase = Blocked
	if err := store.Create(t.Context(), drive); err != nil {
		t.Fatal(err)
	}
	k.Drives = store
	if _, err := k.KickoffFeature(t.Context(), feature.ID, assignment); !errors.Is(err, ErrIneligible) {
		t.Fatalf("wrong-phase error = %v", err)
	}
}
