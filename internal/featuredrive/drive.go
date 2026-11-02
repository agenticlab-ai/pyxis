// Package featuredrive owns the durable, medium-neutral feature lifecycle.
package featuredrive

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
)

const Schema = "feature-drive/v1"

var (
	ErrNotFound  = errors.New("feature drive: not found")
	ErrConflict  = errors.New("feature drive: compare-and-swap conflict")
	ErrMalformed = errors.New("feature drive: malformed state")
)

type OwnerState string

const (
	Unowned   OwnerState = "unowned"
	Active    OwnerState = "active"
	Quiescing OwnerState = "quiescing"
	Released  OwnerState = "released"
)

type Phase string

const (
	Integrating       Phase = "integrating"
	Candidate         Phase = "candidate"
	Verifying         Phase = "verifying"
	Repairing         Phase = "repairing"
	FinalReview       Phase = "final-review"
	DeliveryVerifying Phase = "delivery-verifying"
	CancelPending     Phase = "cancel-pending"
	Cancelled         Phase = "cancelled"
	Abandoned         Phase = "abandoned"
	Delivered         Phase = "delivered"
	Blocked           Phase = "blocked"
)

type EffectStatus string

const (
	Pending  EffectStatus = "pending"
	Observed EffectStatus = "observed"
)

type EvidenceRef struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// Validate requires the typed goal-object identity of an observation. Some
// objects are identified directly by ID and have no separate content digest.
func (e EvidenceRef) Validate() error {
	if strings.TrimSpace(e.Kind) == "" || strings.TrimSpace(e.ID) == "" {
		return ErrMalformed
	}
	return nil
}

type FailureDisposition struct {
	Trigger        string        `json:"trigger"`
	Disposition    string        `json:"disposition"`
	Evidence       []EvidenceRef `json:"evidence"`
	Remedy         string        `json:"remedy"`
	ClearCondition string        `json:"clear_condition"`
	BlockedEffect  string        `json:"blocked_effect"`
}
type Wait struct {
	Kind string      `json:"kind"`
	Goal EvidenceRef `json:"goal"`
}
type Ownership struct {
	State   OwnerState `json:"state"`
	Domain  string     `json:"domain,omitempty"`
	OwnerID string     `json:"owner_id,omitempty"`
	Epoch   uint64     `json:"epoch"`
}
type EffectIntent struct {
	ID            string       `json:"id"`
	Kind          string       `json:"kind"`
	Subject       string       `json:"subject"`
	InputDigest   string       `json:"input_digest"`
	RequiredEpoch uint64       `json:"required_epoch"`
	Status        EffectStatus `json:"status"`
	Attempts      uint32       `json:"attempts"`
	Observation   *EvidenceRef `json:"observation,omitempty"`
}
type Transition struct {
	Kind         string `json:"kind"`
	PriorVersion uint64 `json:"prior_version"`
	NewVersion   uint64 `json:"new_version"`
	PriorEpoch   uint64 `json:"prior_epoch"`
	NewEpoch     uint64 `json:"new_epoch"`
	Actor        string `json:"actor"`
	Signature    string `json:"signature"`
}
type FeatureDrive struct {
	Schema            string               `json:"schema"`
	Repository        string               `json:"repository"`
	FeatureID         string               `json:"feature_id"`
	PlanFingerprint   string               `json:"plan_fingerprint"`
	TargetMainRef     string               `json:"target_main_ref"`
	IntegrationRef    string               `json:"integration_ref"`
	Version           uint64               `json:"version"`
	Created           Transition           `json:"created"`
	Last              Transition           `json:"last"`
	Ownership         Ownership            `json:"ownership"`
	Phase             Phase                `json:"phase"`
	Wait              *Wait                `json:"wait,omitempty"`
	Frontier          []string             `json:"frontier,omitempty"`
	Blocker           *EvidenceRef         `json:"blocker,omitempty"`
	Evidence          []EvidenceRef        `json:"evidence,omitempty"`
	Effects           []EffectIntent       `json:"effects,omitempty"`
	Failure           *FailureDisposition  `json:"failure,omitempty"`
	SecondaryFailures []FailureDisposition `json:"secondary_failures,omitempty"`
}

func (d FeatureDrive) Validate() error {
	if d.Schema != Schema || d.Repository == "" || d.FeatureID == "" || d.PlanFingerprint == "" || d.TargetMainRef == "" || d.IntegrationRef == "" || d.Version == 0 {
		return ErrMalformed
	}
	if d.Created.NewVersion != 1 || d.Last.NewVersion != d.Version || d.Last.PriorVersion != d.Version-1 {
		return ErrMalformed
	}
	if (d.Ownership.State == Active || d.Ownership.State == Quiescing) && (d.Ownership.OwnerID == "" || d.Ownership.Domain == "" || d.Ownership.Epoch == 0) {
		return ErrMalformed
	}
	if d.Ownership.State != Active && d.Ownership.State != Quiescing && d.Ownership.OwnerID != "" {
		return ErrMalformed
	}
	validPhase := map[Phase]bool{Integrating: true, Candidate: true, Verifying: true, Repairing: true, FinalReview: true, DeliveryVerifying: true, CancelPending: true, Cancelled: true, Abandoned: true, Delivered: true, Blocked: true}
	if !validPhase[d.Phase] {
		return ErrMalformed
	}
	// Completed observations retain their execution epoch and stable effect
	// identity. Only pending work must present the current fencing epoch.
	for _, e := range d.Effects {
		if e.ID == "" || e.Kind == "" || e.RequiredEpoch > d.Ownership.Epoch ||
			(e.Status == Observed && e.RequiredEpoch == 0) ||
			(e.Status == Pending && e.RequiredEpoch != d.Ownership.Epoch) || (e.Status != Pending && e.Status != Observed) {
			return ErrMalformed
		}
	}
	if slices.Contains(d.Frontier, "") {
		return ErrMalformed
	}
	for i, e := range d.Effects {
		for _, prior := range d.Effects[:i] {
			if e.ID == prior.ID {
				return ErrMalformed
			}
		}
		if e.Status == Observed && (e.Observation == nil || e.Observation.Validate() != nil) {
			return ErrMalformed
		}
	}
	if d.Ownership.State != Active && (d.Phase == Candidate || d.Phase == Verifying || d.Phase == Repairing || d.Phase == FinalReview || d.Phase == DeliveryVerifying) {
		return ErrMalformed
	}
	return nil
}

// Decode validates the closed wire contract and rejects unknown JSON fields.
func Decode(b []byte) (FeatureDrive, error) {
	var d FeatureDrive
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return d, ErrMalformed
	}
	return d, d.Validate()
}

func EffectID(repository, feature string, epoch uint64, kind, subject, input string) string {
	h := sha256.Sum256(fmt.Appendf(nil, "%s\x00%s\x00%d\x00%s\x00%s\x00%s", repository, feature, epoch, kind, subject, input))
	return hex.EncodeToString(h[:])
}
func SignTransition(t Transition, key ed25519.PrivateKey) Transition {
	t.Signature = ""
	b, _ := json.Marshal(t)
	t.Signature = hex.EncodeToString(ed25519.Sign(key, b))
	return t
}
func VerifyTransition(t Transition, key ed25519.PublicKey) bool {
	sig, err := hex.DecodeString(t.Signature)
	if err != nil {
		return false
	}
	got := t.Signature
	t.Signature = ""
	b, _ := json.Marshal(t)
	t.Signature = got
	return ed25519.Verify(key, b, sig)
}

type FeatureDriveStore interface {
	Load(context.Context, string) (FeatureDrive, error)
	Create(context.Context, FeatureDrive) error
	CAS(context.Context, string, uint64, FeatureDrive) error
}
type MemoryStore struct {
	mu     sync.Mutex
	drives map[string]FeatureDrive
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{drives: map[string]FeatureDrive{}} }
func (s *MemoryStore) Load(_ context.Context, id string) (FeatureDrive, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.drives[id]
	if !ok {
		return FeatureDrive{}, ErrNotFound
	}
	return d, nil
}
func (s *MemoryStore) Create(_ context.Context, d FeatureDrive) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := d.Validate(); err != nil {
		return err
	}
	if _, ok := s.drives[d.FeatureID]; ok {
		return ErrConflict
	}
	s.drives[d.FeatureID] = d
	return nil
}
func (s *MemoryStore) CAS(_ context.Context, id string, version uint64, d FeatureDrive) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.drives[id]
	if !ok {
		return ErrNotFound
	}
	if cur.Version != version {
		return ErrConflict
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if d.Last.Kind == PhaseObserved {
		if err := ValidatePhaseTransition(cur, d); err != nil {
			return err
		}
	}
	s.drives[id] = d
	return nil
}
func (s *MemoryStore) List(_ context.Context) ([]FeatureDrive, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]FeatureDrive, 0, len(s.drives))
	for _, d := range s.drives {
		out = append(out, d)
	}
	return out, nil
}

type EffectExecutor interface {
	Execute(context.Context, EffectIntent) (EvidenceRef, error)
}
type Planner func(context.Context, FeatureDrive, string) ([]EffectIntent, error)
type Reconciler struct {
	Store       FeatureDriveStore
	Executor    EffectExecutor
	Plan        Planner
	MaxAttempts int
}

// effectMu closes the in-process duplicate-touch window. The durable intent
// remains the authority across processes and restarts; this lock only prevents
// two callers in one process from concurrently delivering the same pending
// intent before either can record its observation.
var effectMu sync.Mutex

// Reconcile is safe to call repeatedly and concurrently. CAS losers reload and retry.
func (r Reconciler) Reconcile(ctx context.Context, featureID, reason string) error {
	for range 8 {
		d, err := r.Store.Load(ctx, featureID)
		if err != nil {
			return err
		}
		if err = d.Validate(); err != nil {
			return err
		}
		for _, e := range d.Effects {
			if e.Status == Pending {
				effectMu.Lock()
				// A concurrent caller may have completed this intent while we
				// were waiting for the lock; reload before delivering it.
				latest, loadErr := r.Store.Load(ctx, featureID)
				if loadErr != nil {
					effectMu.Unlock()
					return loadErr
				}
				if latest.Version != d.Version {
					effectMu.Unlock()
					continue
				}
				ref, exerr := r.Executor.Execute(ctx, e)
				if exerr != nil {
					effectMu.Unlock()
					return exerr
				}
				if err := ref.Validate(); err != nil {
					effectMu.Unlock()
					return err
				}
				next := d
				for j := range next.Effects {
					if next.Effects[j].ID == e.ID {
						next.Effects[j].Status = Observed
						next.Effects[j].Observation = &ref
					}
				}
				next.Version++
				next.Last = Transition{Kind: "effect-observed", PriorVersion: d.Version, NewVersion: next.Version, PriorEpoch: d.Ownership.Epoch, NewEpoch: d.Ownership.Epoch}
				if err = r.Store.CAS(ctx, featureID, d.Version, next); err == ErrConflict {
					effectMu.Unlock()
					continue
				}
				effectMu.Unlock()
				return err
			}
		}
		planned, err := r.Plan(ctx, d, reason)
		if err != nil {
			return err
		}
		if len(planned) == 0 {
			return nil
		}
		next := d
		next.Version++
		next.Last = Transition{Kind: "effect-intent", PriorVersion: d.Version, NewVersion: next.Version, PriorEpoch: d.Ownership.Epoch, NewEpoch: d.Ownership.Epoch}
		for _, e := range planned {
			if e.ID == "" {
				e.ID = EffectID(d.Repository, d.FeatureID, d.Ownership.Epoch, e.Kind, e.Subject, e.InputDigest)
			}
			e.RequiredEpoch = d.Ownership.Epoch
			e.Status = Pending
			next.Effects = append(next.Effects, e)
		}
		if err = r.Store.CAS(ctx, featureID, d.Version, next); err == ErrConflict {
			continue
		}
		return err
	}
	return ErrConflict
}
