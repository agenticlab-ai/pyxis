package featuredrive

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DriveTouch is deliberately a signal, never an authority-bearing command.
type TouchReason string

const (
	TouchPush     TouchReason = "push"
	TouchManual   TouchReason = "manual"
	TouchBackstop TouchReason = "backstop"
)

type DriveTouch struct {
	FeatureID string       `json:"feature_id"`
	Reason    TouchReason  `json:"reason"`
	Owner     OwnerEpoch   `json:"owner"`
	Goal      *EvidenceRef `json:"goal,omitempty"`
	At        time.Time    `json:"at"`
}

// NewTouch normalizes all native wake sources to the same instruction-free
// signal.  The aggregate owner/epoch is copied into the signal as routing
// metadata only; Reconcile remains the authority check.
func NewTouch(featureID string, reason TouchReason, owner OwnerEpoch, goal *EvidenceRef, at time.Time) DriveTouch {
	return DriveTouch{FeatureID: featureID, Reason: reason, Owner: owner, Goal: goal, At: at}
}

func PushTouch(featureID string, owner OwnerEpoch, at time.Time) DriveTouch {
	return NewTouch(featureID, TouchPush, owner, nil, at)
}

func ManualTouch(featureID string, owner OwnerEpoch, goal EvidenceRef, at time.Time) DriveTouch {
	return NewTouch(featureID, TouchManual, owner, &goal, at)
}

type TouchRouter struct {
	Store     FeatureDriveStore
	Reconcile func(context.Context, string, string) error
}

func (r TouchRouter) Route(ctx context.Context, t DriveTouch) error {
	if t.FeatureID == "" || (t.Reason != TouchPush && t.Reason != TouchManual && t.Reason != TouchBackstop) {
		return errors.New("feature drive: invalid touch")
	}
	if r.Store == nil || r.Reconcile == nil {
		return errors.New("feature drive: incomplete touch router")
	}
	d, err := r.Store.Load(ctx, t.FeatureID)
	if err != nil {
		return err
	}
	// The router only selects the recorded domain. Reconcile re-verifies the
	// goal and epoch; stale or forged touch metadata cannot authorize work.
	if d.Ownership.State != Active || d.Ownership.Domain != t.Owner.Owner.Domain || d.Ownership.OwnerID != t.Owner.Owner.ID || d.Ownership.Epoch != t.Owner.Epoch {
		return ErrStaleEpoch
	}
	return r.Reconcile(ctx, t.FeatureID, string(t.Reason))
}

type OwnerBackstop struct {
	Store           FeatureDriveStore
	Router          TouchRouter
	Domain, OwnerID string
	Now             func() time.Time
}

func (b OwnerBackstop) Scan(ctx context.Context) ([]DriveTouch, error) {
	if b.Store == nil || b.Domain == "" || b.OwnerID == "" {
		return nil, errors.New("feature drive: incomplete owner backstop")
	}
	// Enumeration is a consumer-owned port: implementations must return only
	// aggregates registered with this domain. The in-memory adapter is supplied
	// for tests and makes that boundary explicit.
	e, ok := b.Store.(interface {
		List(context.Context) ([]FeatureDrive, error)
	})
	if !ok {
		return nil, errors.New("feature drive: store does not support owner scan")
	}
	drives, err := e.List(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if b.Now != nil {
		now = b.Now()
	}
	var due []DriveTouch
	for _, d := range drives {
		if d.Ownership.State == Active && d.Ownership.Domain == b.Domain && d.Ownership.OwnerID == b.OwnerID && d.Wait != nil {
			due = append(due, DriveTouch{FeatureID: d.FeatureID, Reason: TouchBackstop, Owner: OwnerEpoch{Owner: Owner{Domain: b.Domain, ID: b.OwnerID}, Epoch: d.Ownership.Epoch}, Goal: &d.Wait.Goal, At: now})
		}
	}
	return due, nil
}
func (b OwnerBackstop) Run(ctx context.Context) error {
	touches, err := b.Scan(ctx)
	if err != nil {
		return err
	}
	for _, t := range touches {
		if err := b.Router.Route(ctx, t); err != nil && !errors.Is(err, ErrStaleEpoch) {
			return err
		}
	}
	return nil
}

type FeatureDriveStatus struct {
	FeatureID         string               `json:"feature_id"`
	Owner             Ownership            `json:"owner"`
	Phase             Phase                `json:"phase"`
	Frontier          []string             `json:"frontier,omitempty"`
	PendingEffects    []EffectIntent       `json:"pending_effects,omitempty"`
	Wait              *Wait                `json:"wait,omitempty"`
	Blocker           *EvidenceRef         `json:"blocker,omitempty"`
	Evidence          []EvidenceRef        `json:"evidence,omitempty"`
	PrimaryFailure    *FailureDisposition  `json:"primary_failure,omitempty"`
	SecondaryFailures []FailureDisposition `json:"secondary_failures,omitempty"`
	Remedy            string               `json:"remedy,omitempty"`
	ClearCondition    string               `json:"clear_condition,omitempty"`
}

func ReadStatus(ctx context.Context, store FeatureDriveStore, featureID string) (FeatureDriveStatus, error) {
	d, err := store.Load(ctx, featureID)
	if err != nil {
		return FeatureDriveStatus{}, err
	}
	if err = d.Validate(); err != nil {
		return FeatureDriveStatus{}, err
	}
	s := FeatureDriveStatus{FeatureID: d.FeatureID, Owner: d.Ownership, Phase: d.Phase, Frontier: append([]string(nil), d.Frontier...), Evidence: append([]EvidenceRef(nil), d.Evidence...), PrimaryFailure: d.Failure, SecondaryFailures: append([]FailureDisposition(nil), d.SecondaryFailures...), Blocker: d.Blocker}
	if d.Wait != nil {
		wait := *d.Wait
		s.Wait = &wait
	}
	for _, e := range d.Effects {
		if e.Status == Pending {
			s.PendingEffects = append(s.PendingEffects, e)
		}
	}
	if d.Failure != nil {
		s.Remedy = d.Failure.Remedy
		s.ClearCondition = d.Failure.ClearCondition
	}
	return s, nil
}

type LifecycleEvent struct {
	Type        string              `json:"type"`
	FeatureID   string              `json:"feature_id"`
	Version     uint64              `json:"aggregate_version"`
	Epoch       uint64              `json:"owner_epoch"`
	EffectID    string              `json:"effect_id,omitempty"`
	Actor       string              `json:"actor,omitempty"`
	Domain      string              `json:"domain,omitempty"`
	Goal        *EvidenceRef        `json:"goal,omitempty"`
	Failure     *FailureDisposition `json:"failure,omitempty"`
	Correlation Correlation         `json:"correlation"`
	At          time.Time           `json:"at"`
}

// Correlation is the cold-readable join from a feature transition to the
// external goal/effect that caused it. Empty fields are valid for lifecycle
// events with no external object.
type Correlation struct {
	TransitionVersion uint64 `json:"transition_version"`
	GoalID            string `json:"goal_id,omitempty"`
	EffectID          string `json:"effect_id,omitempty"`
}

const (
	EventDriveCreated       = "feature_drive_created"
	EventOwnershipChanged   = "ownership_changed"
	EventEffectPlanned      = "effect_planned"
	EventEffectObserved     = "effect_observed"
	EventTaskRouted         = "task_routed"
	EventTaskLanded         = "task_landed"
	EventCandidatePublished = "candidate_published"
	EventScenarioOutcome    = "scenario_outcome"
	EventCandidateVerdict   = "candidate_verdict"
	EventRepairCreated      = "repair_created"
	EventFinalProposal      = "final_proposal_opened"
	EventDeliveryVerified   = "delivery_verified"
	EventDriveCancelled     = "drive_cancelled"
	EventDriveAbandoned     = "drive_abandoned"
	EventCleanupApplied     = "cleanup_applied"
)

type ActionNotification struct {
	FeatureID   string      `json:"feature_id"`
	Severity    string      `json:"severity"`
	Message     string      `json:"message"`
	Remedy      string      `json:"remedy"`
	Clear       string      `json:"clear_condition"`
	Owner       Ownership   `json:"owner"`
	Correlation Correlation `json:"correlation"`
}

func NotificationForStatus(s FeatureDriveStatus) (ActionNotification, bool) {
	if s.PrimaryFailure == nil {
		return ActionNotification{}, false
	}
	f := s.PrimaryFailure
	severity := "action-required"
	for _, critical := range []string{"ownership-invalid", "handoff-not-quiescent", "delivery-unverified"} {
		if f.Trigger == critical {
			severity = "critical"
		}
	}
	correlation := Correlation{}
	if len(f.Evidence) > 0 {
		correlation.GoalID = f.Evidence[0].ID
	}
	return ActionNotification{FeatureID: s.FeatureID, Severity: severity, Message: f.Disposition, Remedy: f.Remedy, Clear: f.ClearCondition, Owner: s.Owner, Correlation: correlation}, true
}

// ClassifyFailure is the closed, deterministic failure vocabulary. Callers
// pass normalized observations in priority order; the first match is primary
// and the remainder can be retained as secondary evidence by the aggregate.
func ClassifyFailure(trigger string, evidence []EvidenceRef) (FailureDisposition, error) {
	clear := map[string]string{
		"landing-unverified":      "re-observe reviewed landing base/head/check/ancestry proof",
		"candidate-conflict":      "a reviewed repair lands and a different candidate builds",
		"verification-unaccepted": "all required routes have accepted evidence on one current candidate",
		"candidate-stale":         "a new exact-head candidate completes the full required suite",
		"delivery-unverified":     "accepted candidate tree is proven on current main",
		"owner-unavailable":       "the owner resumes or a quiesced transition records a new owner",
		"ownership-invalid":       "restore or reconstruct verified transition lineage",
		"cancel-active":           "every named attempt has terminal/checkpoint evidence",
		"handoff-not-quiescent":   "the active attempt is durably checkpointed and no claim remains",
		"invalid-feature":         "a frozen, projected, approved feature contract passes precheck",
	}
	if _, ok := clear[trigger]; !ok || len(evidence) == 0 {
		return FailureDisposition{}, fmt.Errorf("feature drive: unknown or incomplete failure %q", trigger)
	}
	return FailureDisposition{Trigger: trigger, Disposition: "blocked: " + trigger, Evidence: evidence, Remedy: clear[trigger], ClearCondition: clear[trigger], BlockedEffect: "new routing and downstream effects"}, nil
}

const (
	EventFailureClassified = "feature_failure_classified"
	EventFailureCleared    = "feature_failure_cleared"
	EventTouch             = "drive_touch"
)

type EventSink interface {
	Emit(context.Context, LifecycleEvent) error
}
type Metrics struct {
	mu                                             sync.Mutex
	Touches, NoOpTouches, ModelTokens, LiveWorkers int64
}

func (m *Metrics) RecordTouch(noop bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Touches++
	if noop {
		m.NoOpTouches++
	}
}
func (m *Metrics) RecordInference(tokens int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tokens > 0 {
		m.ModelTokens += tokens
	}
}
func (m *Metrics) SetLiveWorkers(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if count < 0 {
		count = 0
	}
	m.LiveWorkers = count
}
func (m *Metrics) Snapshot() (touches, noops, tokens, workers int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Touches, m.NoOpTouches, m.ModelTokens, m.LiveWorkers
}
func (s FeatureDriveStatus) String() string {
	return fmt.Sprintf("%s phase=%s owner=%s/%s epoch=%d pending=%d", s.FeatureID, s.Phase, s.Owner.Domain, s.Owner.OwnerID, s.Owner.Epoch, len(s.PendingEffects))
}
