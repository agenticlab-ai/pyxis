package featuredrive

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrQuiescence = errors.New("feature drive: active attempt did not quiesce")
	ErrStaleEpoch = errors.New("feature drive: stale owner epoch")
	ErrTerminal   = errors.New("feature drive: terminal phase")
)

// CheckpointEvidence is the positive proof required before an owner epoch can
// be released. An empty attempt id is invalid: timeouts and heartbeats are
// deliberately not evidence of quiescence.
type CheckpointEvidence struct {
	AttemptID string
	Evidence  EvidenceRef
	Complete  bool
}

type AttemptQuiescer interface {
	Quiesce(context.Context, string, Owner, uint64) (CheckpointEvidence, error)
}

type RoutingReleaser interface {
	Release(context.Context, FeatureRoutingScope) error
}

type RequestHandoff struct {
	FeatureID string
	From      OwnerEpoch
	To        ExecutionAssignment
}
type RequestCancellation struct {
	FeatureID string
	Owner     OwnerEpoch
}
type ResumeFeature struct {
	FeatureID string
}
type AbandonFeature struct {
	FeatureID string
}

// RetentionDecision is intentionally separate from lifecycle release. A
// caller must positively authorize cleanup; cancellation and abandonment do
// not imply deletion.
type RetentionDecision struct {
	Confirmed bool
	Delete    bool
}

type OwnershipLifecycle struct {
	Store    FeatureDriveStore
	Quiescer AttemptQuiescer
	Routing  RoutingReleaser
}

func (l OwnershipLifecycle) release(ctx context.Context, d FeatureDrive) error {
	if l.Routing == nil || (d.Ownership.State != Active && d.Ownership.State != Quiescing) {
		return nil
	}
	s := FeatureRoutingScope{FeatureID: d.FeatureID, IntegrationRef: d.IntegrationRef, BaseRef: d.IntegrationRef,
		Owner: Owner{Domain: d.Ownership.Domain, ID: d.Ownership.OwnerID}, Epoch: d.Ownership.Epoch}
	return l.Routing.Release(ctx, s)
}

func (l OwnershipLifecycle) quiesce(ctx context.Context, d FeatureDrive) (CheckpointEvidence, error) {
	if l.Quiescer == nil {
		return CheckpointEvidence{}, fmt.Errorf("%w: no quiescer", ErrQuiescence)
	}
	e, err := l.Quiescer.Quiesce(ctx, d.FeatureID, Owner{Domain: d.Ownership.Domain, ID: d.Ownership.OwnerID}, d.Ownership.Epoch)
	if err != nil {
		return e, fmt.Errorf("%w: %v", ErrQuiescence, err)
	}
	if e.AttemptID == "" || !e.Complete || e.Evidence.Validate() != nil {
		return e, fmt.Errorf("%w: attempt %q", ErrQuiescence, e.AttemptID)
	}
	return e, nil
}

func (l OwnershipLifecycle) change(ctx context.Context, id string, expected uint64, mutate func(*FeatureDrive) error) (FeatureDrive, error) {
	d, err := l.Store.Load(ctx, id)
	if err != nil {
		return d, err
	}
	if d.Ownership.Epoch != expected {
		return d, ErrStaleEpoch
	}
	if err := mutate(&d); err != nil {
		return d, err
	}
	prior := d.Version
	d.Version++
	d.Last = Transition{Kind: "ownership-changed", PriorVersion: prior, NewVersion: d.Version, PriorEpoch: expected, NewEpoch: d.Ownership.Epoch}
	if err := l.Store.CAS(ctx, id, prior, d); err != nil {
		return FeatureDrive{}, err
	}
	return d, nil
}

func (l OwnershipLifecycle) RequestHandoff(ctx context.Context, req RequestHandoff) (FeatureDrive, error) {
	d, err := l.Store.Load(ctx, req.FeatureID)
	if err != nil {
		return d, err
	}
	if d.Ownership.State != Active || d.Ownership.OwnerID != req.From.Owner.ID || d.Ownership.Epoch != req.From.Epoch {
		return d, ErrStaleEpoch
	}
	if req.To.Domain != "local" && req.To.Domain != "remote" || req.To.OwnerID == "" {
		return d, fmt.Errorf("feature drive: invalid handoff target")
	}
	// Persist quiescing before touching routing. This closes the window in
	// which a concurrent reconciler could re-project work for the old owner.
	quiescing := d
	quiescing.Ownership.State = Quiescing
	quiescing.Version++
	quiescing.Last = Transition{Kind: "handoff-quiescing", PriorVersion: d.Version, NewVersion: quiescing.Version, PriorEpoch: d.Ownership.Epoch, NewEpoch: d.Ownership.Epoch}
	if err := l.Store.CAS(ctx, req.FeatureID, d.Version, quiescing); err != nil {
		return d, err
	}
	d = quiescing
	if err := l.release(ctx, d); err != nil {
		return d, err
	}
	e, err := l.quiesce(ctx, d)
	if err != nil {
		// Persist the actionable blocker without changing owner or epoch.
		blocked := d
		blocked.Version++
		blocked.Blocker = &EvidenceRef{Kind: "active-attempt", ID: e.AttemptID}
		blocked.Last = Transition{Kind: "handoff-blocked", PriorVersion: d.Version, NewVersion: blocked.Version, PriorEpoch: d.Ownership.Epoch, NewEpoch: d.Ownership.Epoch}
		if casErr := l.Store.CAS(ctx, req.FeatureID, d.Version, blocked); casErr != nil && !errors.Is(casErr, ErrConflict) {
			return d, fmt.Errorf("record handoff blocker: %w", casErr)
		}
		return blocked, fmt.Errorf("handoff blocked by %s: %w", e.AttemptID, err)
	}
	return l.change(ctx, req.FeatureID, req.From.Epoch, func(d *FeatureDrive) error {
		d.Evidence = append(d.Evidence, e.Evidence)
		d.Ownership = Ownership{State: Active, Domain: req.To.Domain, OwnerID: req.To.OwnerID, Epoch: req.From.Epoch + 1}
		d.Blocker = nil
		return nil
	})
}

func (l OwnershipLifecycle) RequestCancellation(ctx context.Context, req RequestCancellation) (FeatureDrive, error) {
	d, err := l.Store.Load(ctx, req.FeatureID)
	if err != nil {
		return d, err
	}
	if d.Ownership.Epoch != req.Owner.Epoch {
		return d, ErrStaleEpoch
	}
	if d.Ownership.State != Active || d.Ownership.Domain != req.Owner.Owner.Domain || d.Ownership.OwnerID != req.Owner.Owner.ID {
		return d, ErrStaleEpoch
	}
	priorVersion := d.Version
	d.Phase = CancelPending
	d.Version++
	d.Last = Transition{Kind: "cancel-pending", PriorVersion: priorVersion, NewVersion: d.Version, PriorEpoch: d.Ownership.Epoch, NewEpoch: d.Ownership.Epoch}
	if err := l.Store.CAS(ctx, req.FeatureID, priorVersion, d); err != nil {
		return d, err
	}
	if err := l.release(ctx, d); err != nil {
		return d, err
	}
	e, err := l.quiesce(ctx, d)
	if err != nil {
		blocked := d
		blocked.Blocker = &EvidenceRef{Kind: "active-attempt", ID: e.AttemptID}
		blocked.Version++
		blocked.Last = Transition{Kind: "cancel-blocked", PriorVersion: d.Version, NewVersion: blocked.Version, PriorEpoch: d.Ownership.Epoch, NewEpoch: d.Ownership.Epoch}
		if casErr := l.Store.CAS(ctx, req.FeatureID, d.Version, blocked); casErr != nil && !errors.Is(casErr, ErrConflict) {
			return d, fmt.Errorf("record cancellation blocker: %w", casErr)
		}
		return blocked, fmt.Errorf("cancellation blocked by %s: %w", e.AttemptID, err)
	}
	return l.change(ctx, req.FeatureID, req.Owner.Epoch, func(d *FeatureDrive) error {
		d.Phase = Cancelled
		d.Ownership = Ownership{State: Released, Epoch: req.Owner.Epoch + 1}
		d.Evidence = append(d.Evidence, e.Evidence)
		d.Blocker = nil
		return nil
	})
}

func (l OwnershipLifecycle) ResumeFeature(ctx context.Context, req ResumeFeature) (FeatureDrive, error) {
	return l.change(ctx, req.FeatureID, func() uint64 { d, _ := l.Store.Load(ctx, req.FeatureID); return d.Ownership.Epoch }(), func(d *FeatureDrive) error {
		if d.Phase != Cancelled {
			return ErrIneligible
		}
		d.Phase = Integrating
		return nil
	})
}

func (l OwnershipLifecycle) AbandonFeature(ctx context.Context, req AbandonFeature) (FeatureDrive, error) {
	d, err := l.Store.Load(ctx, req.FeatureID)
	if err != nil {
		return d, err
	}
	return l.change(ctx, req.FeatureID, d.Ownership.Epoch, func(d *FeatureDrive) error {
		if d.Phase != Cancelled {
			return ErrIneligible
		}
		d.Phase = Abandoned
		d.Ownership = Ownership{State: Released, Epoch: d.Ownership.Epoch}
		return nil
	})
}
