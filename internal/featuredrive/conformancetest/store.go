package conformancetest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

// StoreFixture supplies the consumer's common state/CAS contract. Signature,
// transport, and restart proofs remain adapter-specific acceptance obligations.
type StoreFixture struct {
	Store           featuredrive.FeatureDriveStore
	ExecutionStore  featuredrive.FeatureDriveStore // Optional separate current-owner credentials.
	Initial         featuredrive.FeatureDrive
	Owner           featuredrive.Owner
	Reopen          func(*testing.T) featuredrive.FeatureDriveStore
	ReopenExecution func(*testing.T) featuredrive.FeatureDriveStore
}

func RunStore(t *testing.T, fixture func(*testing.T) StoreFixture) {
	t.Helper()
	t.Run("phase-journey", func(t *testing.T) { runPhaseJourney(t, fixture(t)) })
	f := fixture(t)
	execution := f.ExecutionStore
	if execution == nil {
		execution = f.Store
	}
	expect(t, f.Initial.Created.Kind == "create" && f.Initial.Last == f.Initial.Created, "fixture must use the core's canonical creation transition")
	_, err := f.Store.Load(t.Context(), f.Initial.FeatureID)
	expect(t, errors.Is(err, featuredrive.ErrNotFound), "uncreated feature read: %v", err)
	err = f.Store.Create(t.Context(), f.Initial)
	expect(t, err == nil, "create feature: %v", err)
	err = f.Store.Create(t.Context(), f.Initial)
	expect(t, errors.Is(err, featuredrive.ErrConflict), "duplicate creation replaced a feature: %v", err)
	current, err := f.Reopen(t).Load(t.Context(), f.Initial.FeatureID)
	expect(t, err == nil && current.Validate() == nil && current.Repository == f.Initial.Repository && current.FeatureID == f.Initial.FeatureID && current.PlanFingerprint == f.Initial.PlanFingerprint && current.TargetMainRef == f.Initial.TargetMainRef && current.IntegrationRef == f.Initial.IntegrationRef && current.Version == f.Initial.Version && current.Ownership == f.Initial.Ownership && current.Phase == f.Initial.Phase, "created identity was not retained: %+v, %v", current, err)
	assigned := current
	assigned.Version++
	assigned.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: f.Owner.Domain, OwnerID: f.Owner.ID, Epoch: 1}
	assigned.Last = featuredrive.Transition{Kind: "ownership-assigned", PriorVersion: current.Version, NewVersion: assigned.Version, NewEpoch: 1}
	err = f.Store.CAS(t.Context(), current.FeatureID, current.Version, assigned)
	expect(t, err == nil, "assign execution owner before effects: %v", err)
	current, err = f.Reopen(t).Load(t.Context(), current.FeatureID)
	expect(t, err == nil && current.Ownership == assigned.Ownership, "assigned owner was not retained: %+v, %v", current, err)
	frontier := current
	frontier.Version++
	frontier.Frontier = []string{"T1", "T2"}
	frontier.Last = featuredrive.Transition{Kind: "frontier-observed", PriorVersion: current.Version, NewVersion: frontier.Version, PriorEpoch: current.Ownership.Epoch, NewEpoch: current.Ownership.Epoch}
	err = execution.CAS(t.Context(), current.FeatureID, current.Version, frontier)
	expect(t, err == nil, "persist the core's observed frontier: %v", err)
	current, err = f.Reopen(t).Load(t.Context(), current.FeatureID)
	expect(t, err == nil && reflect.DeepEqual(current.Frontier, frontier.Frontier) && current.Ownership == assigned.Ownership, "observed frontier was not retained: %+v, %v", current, err)
	next := current
	next.Version++
	next.Last = featuredrive.Transition{Kind: "effect-intent", PriorVersion: current.Version, NewVersion: next.Version, PriorEpoch: current.Ownership.Epoch, NewEpoch: current.Ownership.Epoch}
	next.Effects = []featuredrive.EffectIntent{{ID: "intent-one", Kind: "request-proposal", Subject: "T1", InputDigest: "sha256:input", RequiredEpoch: current.Ownership.Epoch, Status: featuredrive.Pending}}
	err = execution.CAS(t.Context(), current.FeatureID, current.Version, next)
	expect(t, err == nil, "advance feature: %v", err)
	err = execution.CAS(t.Context(), current.FeatureID, current.Version, next)
	expect(t, errors.Is(err, featuredrive.ErrConflict), "stale writer replaced the winner: %v", err)
	retained, err := f.Reopen(t).Load(t.Context(), current.FeatureID)
	expect(t, err == nil && retained.Version == next.Version && reflect.DeepEqual(retained.Effects, next.Effects) && reflect.DeepEqual(retained.Created, current.Created), "CAS lost state or creation lineage: %+v, %v", retained, err)
}
