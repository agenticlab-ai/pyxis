package featuredrive

import (
	"context"
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"sync"
)

var (
	ErrIneligible        = errors.New("feature kickoff: feature is not eligible")
	ErrUnassigned        = errors.New("feature kickoff: ownership assignment is required")
	ErrOwnershipConflict = errors.New("feature kickoff: feature is already owned; use handoff or cancellation")
)

// FrozenFeature is the resolver's projection of a merged, frozen plan. The
// kickoff path consumes this projection, never draft plan documents.
type FrozenFeature struct {
	ID              string
	Repository      string
	PlanFingerprint string
	TargetMainRef   string
	Eligible        bool
}

type FeatureResolver interface {
	Resolve(context.Context, string) (FrozenFeature, error)
}

type OwnerIdentity interface {
	Authenticate(context.Context, ExecutionAssignment) Owner
}
type Owner struct{ Domain, ID string }

type ExecutionAssignment struct {
	Domain  string
	OwnerID string
	Policy  string
}

type OwnerEpoch struct {
	Owner Owner
	Epoch uint64
}
type FeatureRoutingScope struct {
	FeatureID      string
	IntegrationRef string
	BaseRef        string
	Policy         string
	Owner          Owner
	Epoch          uint64
}

type IntegrationRef interface {
	CreateOnly(context.Context, string, string) error
}

type RoutingProjector interface {
	Project(context.Context, FrozenFeature, FeatureRoutingScope) ([]port.Task, error)
}

// TaskExecutor is deliberately narrower than the feature lifecycle. It gets
// ordinary task charters and routing metadata; it cannot advance, merge, or
// release a feature drive.
type TaskExecutor interface {
	Execute(context.Context, []port.Task, ExecutionAssignment, FeatureRoutingScope) error
}

type KickoffResult struct {
	Scope FeatureRoutingScope
	Tasks []port.Task
}

type Kickoff struct {
	Resolver FeatureResolver
	Identity OwnerIdentity
	Refs     IntegrationRef
	Routing  RoutingProjector
	Executor TaskExecutor
	Drives   FeatureDriveStore
}

// kickoffLocks only narrows the local race around the create-only ref effect.
// The store CAS remains the authority across processes; this prevents a burst
// of contenders in one process from each attempting the same external write.
var kickoffLocks sync.Map

func kickoffLock(featureID string) *sync.Mutex {
	lock := &sync.Mutex{}
	actual, _ := kickoffLocks.LoadOrStore(featureID, lock)
	return actual.(*sync.Mutex)
}

func (k Kickoff) KickoffFeature(ctx context.Context, featureID string, assignment ExecutionAssignment) (KickoffResult, error) {
	if assignment.Domain != "local" && assignment.Domain != "remote" {
		return KickoffResult{}, fmt.Errorf("%w: domain must be local or remote", ErrUnassigned)
	}
	if assignment.OwnerID == "" {
		return KickoffResult{}, ErrUnassigned
	}
	if k.Resolver == nil || k.Identity == nil || k.Refs == nil || k.Routing == nil || k.Executor == nil || k.Drives == nil {
		return KickoffResult{}, fmt.Errorf("feature kickoff: incomplete kickoff dependencies")
	}
	feature, err := k.Resolver.Resolve(ctx, featureID)
	if err != nil {
		return KickoffResult{}, fmt.Errorf("resolve frozen feature: %w", err)
	}
	if !feature.Eligible || feature.ID != featureID || feature.Repository == "" || feature.PlanFingerprint == "" || feature.TargetMainRef == "" {
		return KickoffResult{}, ErrIneligible
	}
	owner := k.Identity.Authenticate(ctx, assignment)
	if owner.Domain == "" || owner.ID == "" {
		return KickoffResult{}, fmt.Errorf("authenticate owner: %w", ErrUnassigned)
	}
	authenticated := assignment
	authenticated.Domain, authenticated.OwnerID = owner.Domain, owner.ID
	// Serialize local contenders through the ref-create/CAS pair. A remote
	// contender is still fenced by the durable CAS below.
	lock := kickoffLock(featureID)
	lock.Lock()
	defer lock.Unlock()

	drive, err := k.Drives.Load(ctx, featureID)
	if err != nil {
		return KickoffResult{}, fmt.Errorf("load feature drive: %w", err)
	}
	if drive.FeatureID != featureID || drive.Repository != feature.Repository || drive.PlanFingerprint != feature.PlanFingerprint || drive.TargetMainRef != feature.TargetMainRef {
		return KickoffResult{}, fmt.Errorf("%w: projected feature does not match durable drive", ErrIneligible)
	}
	if drive.Ownership.State == Active {
		return KickoffResult{}, ErrOwnershipConflict
	}
	if drive.Phase != Integrating {
		return KickoffResult{}, fmt.Errorf("%w: phase is %s", ErrIneligible, drive.Phase)
	}
	epoch := drive.Ownership.Epoch + 1
	if epoch == 0 {
		epoch = 1
	}
	ref := "refs/heads/feature/" + featureID
	if err := k.Refs.CreateOnly(ctx, ref, feature.TargetMainRef); err != nil {
		return KickoffResult{}, fmt.Errorf("create integration branch: %w", err)
	}
	next := drive
	next.Repository, next.PlanFingerprint, next.TargetMainRef, next.IntegrationRef = feature.Repository, feature.PlanFingerprint, feature.TargetMainRef, ref
	next.Ownership = Ownership{State: Active, Domain: owner.Domain, OwnerID: owner.ID, Epoch: epoch}
	next.Version++
	next.Last = Transition{Kind: "ownership-assigned", PriorVersion: drive.Version, NewVersion: next.Version, PriorEpoch: drive.Ownership.Epoch, NewEpoch: epoch, Actor: owner.ID}
	if err := k.Drives.CAS(ctx, featureID, drive.Version, next); err != nil {
		if errors.Is(err, ErrConflict) {
			return KickoffResult{}, ErrOwnershipConflict
		}
		return KickoffResult{}, fmt.Errorf("assign feature owner: %w", err)
	}
	scope := FeatureRoutingScope{FeatureID: featureID, IntegrationRef: ref, BaseRef: ref, Policy: assignment.Policy, Owner: owner, Epoch: epoch}
	tasks, err := k.Routing.Project(ctx, feature, scope)
	if err != nil {
		return KickoffResult{}, fmt.Errorf("project feature frontier: %w", err)
	}
	if err := k.Executor.Execute(ctx, tasks, authenticated, scope); err != nil {
		return KickoffResult{}, fmt.Errorf("execute feature frontier: %w", err)
	}
	return KickoffResult{Scope: scope, Tasks: tasks}, nil
}
