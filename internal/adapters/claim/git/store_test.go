package git

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim/conformancetest"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type errorControl struct{ err error }

func (c errorControl) RefStore(string) (*gitrefstore.Store, error) { return nil, c.err }

type memoryRefs struct {
	files       map[string][]byte
	updateErr   error
	readErr     error
	publish     bool
	skipUpdate  bool
	readReplace []byte
}

func (m *memoryRefs) Update(_ context.Context, path, _ string, mutate func([]byte) ([]byte, bool, error)) ([]byte, error) {
	if m.skipUpdate {
		return nil, m.updateErr
	}
	current := append([]byte(nil), m.files[path]...)
	next, changed, err := mutate(current)
	if err != nil {
		return nil, err
	}
	if changed && m.publish {
		m.files[path] = append([]byte(nil), next...)
	}
	return next, m.updateErr
}

func (m *memoryRefs) Read(_ context.Context, path string) ([]byte, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	if m.readReplace != nil {
		return append([]byte(nil), m.readReplace...), nil
	}
	return append([]byte(nil), m.files[path]...), nil
}

func TestClaimStorePortConformance(t *testing.T) {
	conformancetest.Run(t, func(t *testing.T) conformancetest.Fixture {
		refs := &memoryRefs{files: map[string][]byte{}, publish: true}
		initial := adapterRecord("2026-10-20", strings.Repeat("a", 64), strings.Repeat("b", 40))
		competitor := adapterRecord("2026-10-20", strings.Repeat("c", 64), strings.Repeat("d", 40))
		next := adapterRecord("2026-10-21", strings.Repeat("e", 64), strings.Repeat("f", 40))
		return conformancetest.Fixture{First: &Store{refs: refs, jobID: "daily"}, Second: &Store{refs: refs, jobID: "daily"}, Initial: initial,
			Competitor: competitor, NextDate: next, Terminal: claim.Terminal{Outcome: claim.OutcomeTimedOut, FinishedAt: initial.ClaimedAt.Add(time.Hour)}}
	})
}

func adapterRecord(date, nonce, config string) claim.Record {
	return claim.Record{SchemaVersion: claim.SchemaVersion, JobID: "daily", JobDate: date, Timezone: "America/Los_Angeles", OwnerNonce: nonce,
		ConfigCommit: config, ClaimedAt: time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC), Outcome: claim.OutcomeClaimed,
		Route: nativerouting.SelectedProfile{CatalogID: "catalog", PolicyIdentity: "policy", Harness: entity.Codex, Role: nativerouting.RoleImplementation,
			NativeProfileID: "frontier", NativeModel: "gpt", NativeEffort: "high", Provides: nativerouting.Provides{Capability: executiondemand.CapabilityFrontier},
			QualificationID: "qualification", QualificationDigest: "digest", AdapterGeneration: "adapter"}}
}

func TestClaimStorePortConformanceAndFailures(t *testing.T) {
	first := adapterRecord("2026-10-20", strings.Repeat("a", 64), strings.Repeat("b", 40))
	second := adapterRecord("2026-10-20", strings.Repeat("c", 64), strings.Repeat("d", 40))
	second.Route.NativeModel = "changed-model"
	refs := &memoryRefs{files: map[string][]byte{}, publish: true}
	store := &Store{refs: refs, jobID: "daily"}
	won, err := store.Claim(t.Context(), first)
	if err != nil || won.Disposition != claim.DispositionOwned || !claim.SameOwner(first, won.Record) {
		t.Fatalf("winning Claim() = (%+v, %v)", won, err)
	}
	lost, err := store.Claim(t.Context(), second)
	if err != nil || lost.Disposition != claim.DispositionConsumed || !claim.SameOwner(first, lost.Record) {
		t.Fatalf("losing Claim() = (%+v, %v)", lost, err)
	}
	// Even an injected nonce collision cannot turn a pre-existing date into a win.
	repeated, err := store.Claim(t.Context(), first)
	if err != nil || repeated.Disposition != claim.DispositionConsumed {
		t.Fatalf("repeated Claim() = (%+v, %v)", repeated, err)
	}

	finished := first.ClaimedAt.Add(time.Hour)
	failed, err := store.Transition(t.Context(), first, claim.Terminal{Outcome: claim.OutcomeFailed, FinishedAt: finished, Cause: "bounded"})
	if err != nil || failed.Outcome != claim.OutcomeFailed || !claim.SameOwner(first, failed) {
		t.Fatalf("Transition() = (%+v, %v)", failed, err)
	}
	if repeated, err := store.Transition(t.Context(), first, claim.Terminal{Outcome: claim.OutcomeFailed, FinishedAt: finished, Cause: "bounded"}); err != nil || repeated.Outcome != claim.OutcomeFailed {
		t.Fatalf("idempotent terminal proof = (%+v, %v)", repeated, err)
	}
	if _, err := store.Transition(t.Context(), second, claim.Terminal{Outcome: claim.OutcomeTimedOut, FinishedAt: finished}); err == nil {
		t.Fatal("different owner transitioned occurrence")
	}
	if _, err := store.Transition(t.Context(), first, claim.Terminal{Outcome: claim.OutcomeTimedOut, FinishedAt: finished}); err == nil {
		t.Fatal("different second terminal transition succeeded")
	}
}

func TestAdapterDependencyAndIdentityBoundaries(t *testing.T) {
	if _, err := New(nil, "daily"); err == nil {
		t.Fatal("nil control succeeded")
	}
	if _, err := New(errorControl{}, "INVALID"); err == nil {
		t.Fatal("invalid job ID succeeded")
	}
	if _, err := New(errorControl{err: errors.New("control closed")}, "daily"); err == nil {
		t.Fatal("control error was lost")
	}
	candidate := adapterRecord("2026-10-20", strings.Repeat("a", 64), strings.Repeat("b", 40))
	if _, err := (&Store{}).Claim(t.Context(), candidate); err == nil {
		t.Fatal("claim without refs succeeded")
	}
	wrong := &Store{refs: &memoryRefs{files: map[string][]byte{}, publish: true}, jobID: "other"}
	if _, err := wrong.Claim(t.Context(), candidate); err == nil {
		t.Fatal("claim on wrong job ref succeeded")
	}
	if _, err := wrong.Transition(t.Context(), candidate, claim.Terminal{}); err == nil {
		t.Fatal("transition on wrong job ref succeeded")
	}
}

func TestClaimNeverAuthorizesWithoutExactRemoteProof(t *testing.T) {
	candidate := adapterRecord("2026-10-20", strings.Repeat("a", 64), strings.Repeat("b", 40))
	for name, refs := range map[string]*memoryRefs{
		"failed push":      {files: map[string][]byte{}, publish: false, updateErr: errors.New("push failed")},
		"missing proof":    {files: map[string][]byte{}, publish: true, readErr: errors.New("confirmation unavailable")},
		"ambiguous absent": {files: map[string][]byte{}, publish: false, updateErr: errors.New("ambiguous transport")},
	} {
		t.Run(name, func(t *testing.T) {
			store := &Store{refs: refs, jobID: "daily"}
			if result, err := store.Claim(t.Context(), candidate); err == nil || result.Disposition == claim.DispositionOwned {
				t.Fatalf("Claim() = (%+v, %v)", result, err)
			}
		})
	}
	refs := &memoryRefs{files: map[string][]byte{}, publish: true, updateErr: errors.New("ambiguous transport")}
	store := &Store{refs: refs, jobID: "daily"}
	if result, err := store.Claim(t.Context(), candidate); err != nil || result.Disposition != claim.DispositionOwned {
		t.Fatalf("proved ambiguous publication = (%+v, %v)", result, err)
	}
	mismatch := candidate
	mismatch.ConfigCommit = strings.Repeat("e", 40)
	mismatchBytes, _ := claim.Encode(mismatch)
	refs = &memoryRefs{files: map[string][]byte{}, publish: true, readReplace: mismatchBytes}
	if result, err := (&Store{refs: refs, jobID: "daily"}).Claim(t.Context(), candidate); err == nil || result.Disposition == claim.DispositionOwned {
		t.Fatalf("same-nonce proof mismatch = (%+v, %v)", result, err)
	}
	data, _ := claim.Encode(candidate)
	refs = &memoryRefs{files: map[string][]byte{occurrencePath(candidate.JobDate): data}, skipUpdate: true, updateErr: errors.New("fetch failed")}
	if result, err := (&Store{refs: refs, jobID: "daily"}).Claim(t.Context(), candidate); err != nil || result.Disposition != claim.DispositionConsumed {
		t.Fatalf("pre-existing exact record after pre-mutation failure = (%+v, %v)", result, err)
	}
}

func TestClaimRefusesMalformedUnknownAndIdentityMismatch(t *testing.T) {
	candidate := adapterRecord("2026-10-20", strings.Repeat("a", 64), strings.Repeat("b", 40))
	wrong := candidate
	wrong.JobDate = "2026-10-21"
	wrongBytes, _ := claim.Encode(wrong)
	for name, existing := range map[string][]byte{
		"malformed":  []byte(`{"schema_version":`),
		"unknown":    []byte(`{"schema_version":"future"}`),
		"wrong date": wrongBytes,
	} {
		t.Run(name, func(t *testing.T) {
			refs := &memoryRefs{files: map[string][]byte{occurrencePath(candidate.JobDate): existing}, publish: true}
			if result, err := (&Store{refs: refs, jobID: "daily"}).Claim(t.Context(), candidate); err == nil || result.Disposition == claim.DispositionOwned {
				t.Fatalf("Claim() = (%+v, %v)", result, err)
			}
		})
	}
}

func TestTerminalWriteFailureRetainsClaim(t *testing.T) {
	owned := adapterRecord("2026-10-20", strings.Repeat("a", 64), strings.Repeat("b", 40))
	data, _ := claim.Encode(owned)
	refs := &memoryRefs{files: map[string][]byte{occurrencePath(owned.JobDate): data}, publish: false, updateErr: errors.New("push failed")}
	store := &Store{refs: refs, jobID: "daily"}
	if _, err := store.Transition(t.Context(), owned, claim.Terminal{Outcome: claim.OutcomeTimedOut, FinishedAt: owned.ClaimedAt.Add(time.Hour)}); err == nil {
		t.Fatal("failed terminal write succeeded")
	}
	retained, err := claim.Decode(refs.files[occurrencePath(owned.JobDate)])
	if err != nil || retained.Outcome != claim.OutcomeClaimed || !claim.SameOwner(owned, retained) {
		t.Fatalf("retained claim = (%+v, %v)", retained, err)
	}
}
