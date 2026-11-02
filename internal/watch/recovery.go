package watch

// Configured-review readiness is a second durable frontier. This file owns
// the consumer-side join and recovery counter; the executor remains the
// provider of immutable review obligations and attempts.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
)

const (
	ReviewWakeSchema     = "review-wake-state/v1"
	ReviewRecoverySchema = "review-recovery/v1"
)

type ReviewProviderState string

const (
	ReviewHandoffPending ReviewProviderState = "handoff_pending"
	ReviewPending        ReviewProviderState = "review_pending"
	ReviewVerified       ReviewProviderState = "verified"
)

type ReviewEligibility string

const (
	ReviewHandoffRepair ReviewEligibility = "handoff_repair"
	ReviewEligible      ReviewEligibility = "eligible"
	ReviewExhausted     ReviewEligibility = "exhausted"
	ReviewUnobserved    ReviewEligibility = "unobserved"
)

type ReviewObligationState struct {
	ID               string              `json:"review_obligation_id"`
	TaskID           string              `json:"task_id"`
	ProposalRef      string              `json:"proposal_ref"`
	Revision         string              `json:"revision"`
	Harness          entity.Harness      `json:"backend"`
	ProviderState    ReviewProviderState `json:"provider_state"`
	AttemptCount     int                 `json:"review_attempt_count"`
	LastFailureClass string              `json:"last_failure_class,omitempty"`
}

type ReviewObligationStateReader interface {
	Open(context.Context, string) ([]ReviewObligationState, error)
}

type RecoveryAction string

const (
	RecoveryInitialize RecoveryAction = "initialize"
	RecoveryAdmit      RecoveryAction = "admit"
	RecoveryExhaust    RecoveryAction = "exhaust"
	RecoveryClose      RecoveryAction = "close"
	RecoveryRearm      RecoveryAction = "rearm"
)

// RecoveryEntry is one append-only consumer decision. FirstObservedAtUTC and
// RecoveryBound are repeated on every row so each row is self-validating and
// a partial/corrupt history fails closed rather than inheriting defaults.
type RecoveryEntry struct {
	SchemaVersion      string         `json:"schema_version"`
	Scope              string         `json:"scope"`
	ObligationID       string         `json:"review_obligation_id"`
	RecoveryEpoch      string         `json:"recovery_epoch"`
	Action             RecoveryAction `json:"action"`
	FirstObservedAtUTC time.Time      `json:"first_observed_at_utc"`
	RecoveryCount      int            `json:"recovery_count"`
	RecoveryBound      int            `json:"recovery_bound"`
	RecordedAtUTC      time.Time      `json:"recorded_at_utc"`
	DriveRoundID       string         `json:"drive_round_id,omitempty"`
	Principal          string         `json:"principal,omitempty"`
	Reason             string         `json:"reason,omitempty"`
	PriorEvidenceRef   string         `json:"prior_evidence_ref,omitempty"`
	EvidenceRef        string         `json:"evidence_ref"`
}

type ReviewRecoveryLedger interface {
	Exact(context.Context, string) ([]RecoveryEntry, error)
	Append(context.Context, RecoveryEntry) (string, error)
}

type RecoveryPolicy struct{ Bound int }

func (p RecoveryPolicy) validate() error {
	if p.Bound < 1 || p.Bound > 100 {
		return fmt.Errorf("watch: review recovery bound %d outside [1,100]", p.Bound)
	}
	return nil
}

type ReviewWakeRow struct {
	ReviewObligationState
	FirstObservedAtUTC time.Time         `json:"first_observed_at_utc"`
	AgeS               int64             `json:"age_s"`
	RecoveryEpoch      string            `json:"recovery_epoch,omitempty"`
	RecoveryCount      int               `json:"recovery_count"`
	RecoveryBound      int               `json:"recovery_bound"`
	Eligibility        ReviewEligibility `json:"eligibility"`
	EvidenceRef        string            `json:"evidence_ref,omitempty"`
}

type ReviewStateStatus string

const (
	ReviewStateKnown   ReviewStateStatus = "known"
	ReviewStateUnknown ReviewStateStatus = "unknown"
)

type ReviewUnknown struct {
	Source string `json:"source"`
	Kind   string `json:"failure_kind"`
	Reason string `json:"reason"`
}

type ReviewWakeState struct {
	SchemaVersion string            `json:"schema_version"`
	Status        ReviewStateStatus `json:"status"`
	Rows          []ReviewWakeRow   `json:"rows"`
	Unknown       *ReviewUnknown    `json:"unknown,omitempty"`
}

func (s ReviewWakeState) Complete() bool {
	return s.Status == ReviewStateKnown && len(s.Rows) == 0
}

type RecoveryClock interface{ Now() time.Time }

type EpochSource func() (string, error)

// ReviewStateReader exact-joins provider truth and the consumer ledger. It
// may initialize first observation only on DeriveReadiness; SnapshotStatus is
// mechanically read-only.
type ReviewStateReader struct {
	Provider ReviewObligationStateReader
	Recovery ReviewRecoveryLedger
	Clock    RecoveryClock
	Epoch    EpochSource
}

func (r ReviewStateReader) DeriveReadiness(ctx context.Context, scope string, policy RecoveryPolicy) (ReviewWakeState, error) {
	if err := policy.validate(); err != nil {
		return ReviewWakeState{}, err
	}
	return r.derive(ctx, scope, policy, true)
}

func (r ReviewStateReader) SnapshotStatus(ctx context.Context, scope string) (ReviewWakeState, error) {
	return r.derive(ctx, scope, RecoveryPolicy{}, false)
}

func (r ReviewStateReader) derive(ctx context.Context, scope string, policy RecoveryPolicy, initialize bool) (ReviewWakeState, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" || r.Provider == nil || r.Recovery == nil {
		return ReviewWakeState{}, errors.New("watch: review state reader requires scope, provider, and recovery ledger")
	}
	provider, err := r.Provider.Open(ctx, scope)
	if err != nil {
		return unknownReviewState("obligation_store", err), nil
	}
	recovery, err := r.Recovery.Exact(ctx, scope)
	if err != nil {
		return unknownReviewState("recovery_ledger", err), nil
	}
	return r.join(ctx, scope, provider, recovery, policy, initialize)
}

func unknownReviewState(source string, err error) ReviewWakeState {
	return ReviewWakeState{SchemaVersion: ReviewWakeSchema, Status: ReviewStateUnknown, Unknown: &ReviewUnknown{Source: source, Kind: classifyReviewReadError(err), Reason: err.Error()}}
}

func classifyReviewReadError(err error) string {
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "permission"):
		return "permission_denied"
	case strings.Contains(s, "unknown"):
		return "unknown_field"
	case strings.Contains(s, "malformed"), strings.Contains(s, "invalid"), strings.Contains(s, "conflict"):
		return "invalid_envelope"
	default:
		return "unavailable"
	}
}

type reducedRecovery struct {
	FirstObservedAtUTC time.Time
	Epoch              string
	Count              int
	Bound              int
	Closed             bool
	ExhaustedEvidence  string
}

func (r ReviewStateReader) join(ctx context.Context, scope string, provider []ReviewObligationState, entries []RecoveryEntry, policy RecoveryPolicy, initialize bool) (ReviewWakeState, error) {
	byProvider := make(map[string]ReviewObligationState, len(provider))
	for _, state := range provider {
		if err := validateProviderState(state); err != nil {
			return unknownReviewState("obligation_store", err), nil
		}
		if _, duplicate := byProvider[state.ID]; duplicate {
			return unknownReviewState("obligation_store", fmt.Errorf("duplicate obligation %s", state.ID)), nil
		}
		byProvider[state.ID] = state
	}
	reduced, err := reduceRecovery(scope, entries)
	if err != nil {
		return unknownReviewState("recovery_ledger", err), nil
	}
	for id := range reduced {
		if _, ok := byProvider[id]; !ok {
			return unknownReviewState("recovery_ledger", fmt.Errorf("recovery entry references unknown obligation %s", id)), nil
		}
	}

	now := time.Now().UTC()
	if r.Clock != nil {
		now = r.Clock.Now().UTC()
	}
	state := ReviewWakeState{SchemaVersion: ReviewWakeSchema, Status: ReviewStateKnown}
	for _, p := range provider {
		if p.ProviderState == ReviewVerified {
			if rr, observed := reduced[p.ID]; initialize && observed && !rr.Closed {
				entry := RecoveryEntry{Scope: scope, ObligationID: p.ID, RecoveryEpoch: rr.Epoch, Action: RecoveryClose, FirstObservedAtUTC: rr.FirstObservedAtUTC, RecoveryCount: rr.Count, RecoveryBound: rr.Bound, RecordedAtUTC: now, Reason: "authenticated provider verification"}
				if _, appendErr := r.Recovery.Append(ctx, entry); appendErr != nil {
					return unknownReviewState("recovery_ledger", appendErr), nil
				}
			}
			continue
		}
		if p.ProviderState == ReviewHandoffPending {
			state.Rows = append(state.Rows, ReviewWakeRow{ReviewObligationState: p, Eligibility: ReviewHandoffRepair})
			continue
		}
		rr, observed := reduced[p.ID]
		if !observed {
			if !initialize {
				state.Rows = append(state.Rows, ReviewWakeRow{ReviewObligationState: p, Eligibility: ReviewUnobserved})
				continue
			}
			epoch, epochErr := r.newEpoch()
			if epochErr != nil {
				return ReviewWakeState{}, epochErr
			}
			entry := RecoveryEntry{SchemaVersion: ReviewRecoverySchema, Scope: scope, ObligationID: p.ID, RecoveryEpoch: epoch, Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryCount: 0, RecoveryBound: policy.Bound, RecordedAtUTC: now}
			if _, appendErr := r.Recovery.Append(ctx, entry); appendErr != nil {
				return unknownReviewState("recovery_ledger", appendErr), nil
			}
			rr = reducedRecovery{FirstObservedAtUTC: now, Epoch: epoch, Bound: policy.Bound}
		}
		if rr.Closed {
			return unknownReviewState("recovery_ledger", fmt.Errorf("recovery closed while provider obligation %s remains pending", p.ID)), nil
		}
		age := max(now.Sub(rr.FirstObservedAtUTC), 0)
		eligibility := ReviewEligible
		if rr.Count >= rr.Bound {
			eligibility = ReviewExhausted
		}
		state.Rows = append(state.Rows, ReviewWakeRow{ReviewObligationState: p, FirstObservedAtUTC: rr.FirstObservedAtUTC, AgeS: int64(age / time.Second), RecoveryEpoch: rr.Epoch, RecoveryCount: rr.Count, RecoveryBound: rr.Bound, Eligibility: eligibility, EvidenceRef: rr.ExhaustedEvidence})
	}
	sort.Slice(state.Rows, func(i, j int) bool { return state.Rows[i].ID < state.Rows[j].ID })
	return state, nil
}

func (r ReviewStateReader) newEpoch() (string, error) {
	if r.Epoch != nil {
		return r.Epoch()
	}
	return randomReviewID("recovery-epoch")
}

func validateProviderState(s ReviewObligationState) error {
	if s.ID == "" || s.TaskID == "" || s.ProposalRef == "" || s.Revision == "" || s.Harness == "" || s.AttemptCount < 0 {
		return errors.New("invalid provider review state")
	}
	switch s.ProviderState {
	case ReviewHandoffPending, ReviewPending, ReviewVerified:
	default:
		return fmt.Errorf("invalid provider state %q", s.ProviderState)
	}
	if s.LastFailureClass != "" && s.LastFailureClass != "artifact_unverified" && s.LastFailureClass != "artifact_untrusted" && s.LastFailureClass != "infrastructure" {
		return fmt.Errorf("invalid review failure class %q", s.LastFailureClass)
	}
	return nil
}

func reduceRecovery(scope string, entries []RecoveryEntry) (map[string]reducedRecovery, error) {
	out := map[string]reducedRecovery{}
	seen := map[string]RecoveryEntry{}
	for _, entry := range entries {
		if err := validateRecoveryEntry(entry); err != nil {
			return nil, err
		}
		if entry.Scope != scope {
			return nil, fmt.Errorf("recovery entry scope %q does not match %q", entry.Scope, scope)
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%s", entry.Scope, entry.ObligationID, entry.RecoveryEpoch, entry.RecoveryCount, entry.Action)
		if prior, ok := seen[key]; ok {
			if prior != entry {
				return nil, fmt.Errorf("conflicting recovery replay for %s", entry.ObligationID)
			}
			continue
		}
		seen[key] = entry
		state, exists := out[entry.ObligationID]
		switch entry.Action {
		case RecoveryInitialize:
			if exists {
				return nil, fmt.Errorf("duplicate recovery initialization for %s", entry.ObligationID)
			}
			state = reducedRecovery{FirstObservedAtUTC: entry.FirstObservedAtUTC, Epoch: entry.RecoveryEpoch, Count: 0, Bound: entry.RecoveryBound}
		case RecoveryAdmit:
			if !exists || entry.RecoveryEpoch != state.Epoch || entry.FirstObservedAtUTC != state.FirstObservedAtUTC || entry.RecoveryBound != state.Bound || entry.RecoveryCount != state.Count+1 || state.Closed {
				return nil, fmt.Errorf("invalid recovery admission sequence for %s", entry.ObligationID)
			}
			state.Count = entry.RecoveryCount
		case RecoveryExhaust:
			if !exists || entry.RecoveryEpoch != state.Epoch || entry.RecoveryCount != state.Count || state.Count != state.Bound || entry.RecoveryBound != state.Bound || entry.FirstObservedAtUTC != state.FirstObservedAtUTC {
				return nil, fmt.Errorf("invalid recovery exhaustion for %s", entry.ObligationID)
			}
			state.ExhaustedEvidence = entry.EvidenceRef
		case RecoveryClose:
			if !exists || entry.RecoveryEpoch != state.Epoch || entry.RecoveryCount != state.Count || entry.RecoveryBound != state.Bound || entry.FirstObservedAtUTC != state.FirstObservedAtUTC {
				return nil, fmt.Errorf("invalid recovery closure for %s", entry.ObligationID)
			}
			state.Closed = true
		case RecoveryRearm:
			if !exists || state.ExhaustedEvidence == "" || entry.PriorEvidenceRef != state.ExhaustedEvidence || entry.RecoveryEpoch == state.Epoch || entry.RecoveryCount != 0 {
				return nil, fmt.Errorf("invalid review recovery rearm for %s", entry.ObligationID)
			}
			state = reducedRecovery{FirstObservedAtUTC: entry.FirstObservedAtUTC, Epoch: entry.RecoveryEpoch, Count: 0, Bound: entry.RecoveryBound}
		}
		out[entry.ObligationID] = state
	}
	return out, nil
}

func validateRecoveryEntry(entry RecoveryEntry) error {
	if entry.SchemaVersion != ReviewRecoverySchema || entry.Scope == "" || entry.ObligationID == "" || entry.RecoveryEpoch == "" || entry.FirstObservedAtUTC.IsZero() || entry.RecordedAtUTC.IsZero() || entry.RecoveryCount < 0 || entry.RecoveryBound < 1 || entry.RecoveryBound > 100 {
		return errors.New("invalid review-recovery envelope")
	}
	switch entry.Action {
	case RecoveryInitialize:
		if entry.RecoveryCount != 0 || entry.DriveRoundID != "" {
			return errors.New("invalid review-recovery initialization")
		}
	case RecoveryAdmit:
		if entry.RecoveryCount < 1 || entry.RecoveryCount > entry.RecoveryBound || entry.DriveRoundID == "" {
			return errors.New("invalid review-recovery admission")
		}
	case RecoveryExhaust:
		if entry.RecoveryCount != entry.RecoveryBound || entry.DriveRoundID == "" || entry.Reason == "" {
			return errors.New("invalid review-recovery exhaustion")
		}
	case RecoveryClose:
	case RecoveryRearm:
		if entry.RecoveryCount != 0 || entry.DriveRoundID != "" || entry.Principal == "" || entry.Reason == "" || entry.PriorEvidenceRef == "" {
			return errors.New("invalid review-recovery rearm")
		}
	default:
		return fmt.Errorf("unknown review-recovery action %q", entry.Action)
	}
	want, err := recoveryEvidenceRef(entry)
	if err != nil {
		return err
	}
	if entry.EvidenceRef != want {
		return errors.New("invalid review-recovery evidence reference")
	}
	return nil
}

func sealRecoveryEntry(entry RecoveryEntry) (RecoveryEntry, error) {
	entry.SchemaVersion = ReviewRecoverySchema
	entry.FirstObservedAtUTC = entry.FirstObservedAtUTC.UTC()
	entry.RecordedAtUTC = entry.RecordedAtUTC.UTC()
	entry.EvidenceRef = ""
	ref, err := recoveryEvidenceRef(entry)
	if err != nil {
		return RecoveryEntry{}, err
	}
	entry.EvidenceRef = ref
	return entry, validateRecoveryEntry(entry)
}

func recoveryEvidenceRef(entry RecoveryEntry) (string, error) {
	entry.EvidenceRef = ""
	b, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("watch: encode review recovery evidence: %w", err)
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func randomReviewID(prefix string) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("watch: read identity entropy: %w", err)
	}
	return prefix + "-" + hex.EncodeToString(b[:]), nil
}

// Admit append-first spends one consumer recovery unit. The returned row is
// the exact event/request identity to use for the one selected invocation.
func (r ReviewStateReader) Admit(ctx context.Context, scope, driveRoundID string, row ReviewWakeRow) (ReviewWakeRow, error) {
	if row.Eligibility != ReviewEligible || driveRoundID == "" || row.RecoveryCount >= row.RecoveryBound {
		return ReviewWakeRow{}, errors.New("watch: review recovery admission is not eligible")
	}
	now := time.Now().UTC()
	if r.Clock != nil {
		now = r.Clock.Now().UTC()
	}
	entry := RecoveryEntry{Scope: scope, ObligationID: row.ID, RecoveryEpoch: row.RecoveryEpoch, Action: RecoveryAdmit, FirstObservedAtUTC: row.FirstObservedAtUTC, RecoveryCount: row.RecoveryCount + 1, RecoveryBound: row.RecoveryBound, RecordedAtUTC: now, DriveRoundID: driveRoundID}
	if _, err := r.Recovery.Append(ctx, entry); err != nil {
		return ReviewWakeRow{}, err
	}
	row.RecoveryCount++
	if row.RecoveryCount >= row.RecoveryBound {
		row.Eligibility = ReviewExhausted
	}
	return row, nil
}

// RecordExhaustion persists the immutable evidence required before the drive
// may emit or render an exhausted decision.
func (r ReviewStateReader) RecordExhaustion(ctx context.Context, scope, driveRoundID string, row ReviewWakeRow) (string, error) {
	if row.RecoveryCount != row.RecoveryBound || row.RecoveryBound < 1 {
		return "", errors.New("watch: cannot record non-exhausted review obligation")
	}
	if row.EvidenceRef != "" {
		return row.EvidenceRef, nil
	}
	now := time.Now().UTC()
	if r.Clock != nil {
		now = r.Clock.Now().UTC()
	}
	entry := RecoveryEntry{Scope: scope, ObligationID: row.ID, RecoveryEpoch: row.RecoveryEpoch, Action: RecoveryExhaust, FirstObservedAtUTC: row.FirstObservedAtUTC, RecoveryCount: row.RecoveryCount, RecoveryBound: row.RecoveryBound, RecordedAtUTC: now, DriveRoundID: driveRoundID, Reason: "consumer recovery bound exhausted"}
	return r.Recovery.Append(ctx, entry)
}

// Rearm is the explicit attributable operator surface for an exhausted
// obligation. It starts a fresh epoch without deleting or editing the exhausted
// one and freezes the current positive bound into the new initialization.
func (r ReviewStateReader) Rearm(ctx context.Context, scope, obligationID, principal, reason string, policy RecoveryPolicy) (string, error) {
	if err := policy.validate(); err != nil {
		return "", err
	}
	scope, obligationID = strings.TrimSpace(scope), strings.TrimSpace(obligationID)
	principal, reason = strings.TrimSpace(principal), strings.TrimSpace(reason)
	if scope == "" || obligationID == "" || principal == "" || reason == "" || r.Provider == nil || r.Recovery == nil {
		return "", errors.New("watch: rearm requires scope, obligation, principal, reason, provider, and recovery ledger")
	}
	provider, err := r.Provider.Open(ctx, scope)
	if err != nil {
		return "", err
	}
	pending := false
	for _, state := range provider {
		if state.ID == obligationID {
			pending = state.ProviderState == ReviewPending
			break
		}
	}
	if !pending {
		return "", errors.New("watch: only a review-pending obligation can be rearmed")
	}
	entries, err := r.Recovery.Exact(ctx, scope)
	if err != nil {
		return "", err
	}
	reduced, err := reduceRecovery(scope, entries)
	if err != nil {
		return "", err
	}
	prior, ok := reduced[obligationID]
	if !ok || prior.Count != prior.Bound || prior.ExhaustedEvidence == "" {
		return "", errors.New("watch: review obligation is not exhausted with durable evidence")
	}
	epoch, err := r.newEpoch()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	if r.Clock != nil {
		now = r.Clock.Now().UTC()
	}
	entry := RecoveryEntry{Scope: scope, ObligationID: obligationID, RecoveryEpoch: epoch, Action: RecoveryRearm, FirstObservedAtUTC: now, RecoveryCount: 0, RecoveryBound: policy.Bound, RecordedAtUTC: now, Principal: principal, Reason: reason, PriorEvidenceRef: prior.ExhaustedEvidence}
	return r.Recovery.Append(ctx, entry)
}

type ReviewRecoveryRequest struct {
	DriveRoundID  string   `json:"drive_round_id"`
	ObligationIDs []string `json:"review_obligation_ids"`
}

type ReviewSweep func(context.Context, ReviewRecoveryRequest) error

type RoundIDSource func() (string, error)

func NewRoundID() (string, error) { return randomReviewID("drive-round") }
