package watch

import (
	"context"
	"errors"
	"testing"
	"time"
)

type boundaryRecoveryLedger struct {
	entries   []RecoveryEntry
	exactErr  error
	appendErr error
}

func (l *boundaryRecoveryLedger) Exact(context.Context, string) ([]RecoveryEntry, error) {
	return append([]RecoveryEntry(nil), l.entries...), l.exactErr
}

func (l *boundaryRecoveryLedger) Append(_ context.Context, entry RecoveryEntry) (string, error) {
	if l.appendErr != nil {
		return "", l.appendErr
	}
	sealed, err := sealRecoveryEntry(entry)
	if err != nil {
		return "", err
	}
	l.entries = append(l.entries, sealed)
	return sealed.EvidenceRef, nil
}

func TestReviewStateReaderBoundaryFailures(t *testing.T) {
	if _, err := (ReviewStateReader{}).DeriveReadiness(t.Context(), "scope", RecoveryPolicy{}); err == nil {
		t.Fatal("DeriveReadiness accepted an invalid recovery bound")
	}
	provider := &mutableReviewProvider{}
	wantErr := errors.New("recovery unavailable")
	state, err := (ReviewStateReader{Provider: provider, Recovery: &boundaryRecoveryLedger{exactErr: wantErr}}).SnapshotStatus(t.Context(), "scope")
	if err != nil || state.Status != ReviewStateUnknown || state.Unknown.Source != "recovery_ledger" {
		t.Fatalf("recovery read failure = %+v, %v", state, err)
	}
	if got := classifyReviewReadError(errors.New("unknown field")); got != "unknown_field" {
		t.Fatalf("unknown-field classification = %q", got)
	}
	if got := classifyReviewReadError(errors.New("malformed envelope")); got != "invalid_envelope" {
		t.Fatalf("invalid-envelope classification = %q", got)
	}
}

func TestReviewJoinRejectsInvalidProviderAndRecoveryStates(t *testing.T) {
	reader := ReviewStateReader{Recovery: &boundaryRecoveryLedger{}}
	if state, err := reader.join(t.Context(), "scope", []ReviewObligationState{{}}, nil, RecoveryPolicy{Bound: 2}, true); err != nil || state.Status != ReviewStateUnknown {
		t.Fatalf("invalid provider state = %+v, %v", state, err)
	}
	pending := pendingReview("o-1")
	if state, err := reader.join(t.Context(), "scope", []ReviewObligationState{pending, pending}, nil, RecoveryPolicy{Bound: 2}, true); err != nil || state.Status != ReviewStateUnknown {
		t.Fatalf("duplicate provider state = %+v, %v", state, err)
	}
	if state, err := reader.join(t.Context(), "scope", []ReviewObligationState{pending}, []RecoveryEntry{{}}, RecoveryPolicy{Bound: 2}, true); err != nil || state.Status != ReviewStateUnknown {
		t.Fatalf("invalid recovery state = %+v, %v", state, err)
	}

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	foreign := mustSealRecovery(t, RecoveryEntry{Scope: "scope", ObligationID: "other", RecoveryEpoch: "epoch-1", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryBound: 2, RecordedAtUTC: now})
	if state, err := reader.join(t.Context(), "scope", []ReviewObligationState{pending}, []RecoveryEntry{foreign}, RecoveryPolicy{Bound: 2}, true); err != nil || state.Status != ReviewStateUnknown {
		t.Fatalf("unknown recovery obligation = %+v, %v", state, err)
	}

	init := mustSealRecovery(t, RecoveryEntry{Scope: "scope", ObligationID: pending.ID, RecoveryEpoch: "epoch-1", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryBound: 2, RecordedAtUTC: now})
	verified := pending
	verified.ProviderState = ReviewVerified
	appendErr := errors.New("append failed")
	reader.Recovery = &boundaryRecoveryLedger{appendErr: appendErr}
	if state, err := reader.join(t.Context(), "scope", []ReviewObligationState{verified}, []RecoveryEntry{init}, RecoveryPolicy{Bound: 2}, true); err != nil || state.Status != ReviewStateUnknown {
		t.Fatalf("close append failure = %+v, %v", state, err)
	}

	handoff := pending
	handoff.ProviderState = ReviewHandoffPending
	reader.Recovery = &boundaryRecoveryLedger{}
	state, err := reader.join(t.Context(), "scope", []ReviewObligationState{handoff}, nil, RecoveryPolicy{Bound: 2}, true)
	if err != nil || len(state.Rows) != 1 || state.Rows[0].Eligibility != ReviewHandoffRepair {
		t.Fatalf("handoff row = %+v, %v", state, err)
	}
}

func TestReviewJoinInitializationAndClosedBoundaries(t *testing.T) {
	pending := pendingReview("o-1")
	wantErr := errors.New("epoch failed")
	reader := ReviewStateReader{
		Recovery: &boundaryRecoveryLedger{},
		Epoch:    func() (string, error) { return "", wantErr },
	}
	if _, err := reader.join(t.Context(), "scope", []ReviewObligationState{pending}, nil, RecoveryPolicy{Bound: 2}, true); !errors.Is(err, wantErr) {
		t.Fatalf("epoch error = %v", err)
	}
	reader.Epoch = func() (string, error) { return "epoch-1", nil }
	reader.Recovery = &boundaryRecoveryLedger{appendErr: wantErr}
	if state, err := reader.join(t.Context(), "scope", []ReviewObligationState{pending}, nil, RecoveryPolicy{Bound: 2}, true); err != nil || state.Status != ReviewStateUnknown {
		t.Fatalf("initialize append failure = %+v, %v", state, err)
	}

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	init := mustSealRecovery(t, RecoveryEntry{Scope: "scope", ObligationID: pending.ID, RecoveryEpoch: "epoch-1", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryBound: 2, RecordedAtUTC: now})
	closeEntry := mustSealRecovery(t, RecoveryEntry{Scope: "scope", ObligationID: pending.ID, RecoveryEpoch: "epoch-1", Action: RecoveryClose, FirstObservedAtUTC: now, RecoveryBound: 2, RecordedAtUTC: now})
	reader.Recovery = &boundaryRecoveryLedger{}
	if state, err := reader.join(t.Context(), "scope", []ReviewObligationState{pending}, []RecoveryEntry{init, closeEntry}, RecoveryPolicy{Bound: 2}, true); err != nil || state.Status != ReviewStateUnknown {
		t.Fatalf("closed pending recovery = %+v, %v", state, err)
	}
}

func TestValidateProviderStateRejectsEveryInvalidClass(t *testing.T) {
	if err := validateProviderState(ReviewObligationState{}); err == nil {
		t.Fatal("empty provider state was accepted")
	}
	state := pendingReview("o-1")
	state.ProviderState = "foreign"
	if err := validateProviderState(state); err == nil {
		t.Fatal("foreign provider state was accepted")
	}
	state = pendingReview("o-1")
	state.LastFailureClass = "foreign"
	if err := validateProviderState(state); err == nil {
		t.Fatal("foreign failure class was accepted")
	}
}

func mustSealRecovery(t *testing.T, entry RecoveryEntry) RecoveryEntry {
	t.Helper()
	sealed, err := sealRecoveryEntry(entry)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}
