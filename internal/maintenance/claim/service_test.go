package claim_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

type authorityFixture struct {
	ready    installation.ReadyBinding
	guarded  bool
	guardErr error
}

func (a *authorityFixture) Ready(context.Context) (installation.ReadyBinding, error) {
	return a.ready, nil
}
func (*authorityFixture) ValidateReady(context.Context, installation.ReadyBinding) error { return nil }
func (a *authorityFixture) GuardReady(ctx context.Context, _ installation.ReadyBinding, run func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.guardErr != nil {
		return a.guardErr
	}
	a.guarded = true
	defer func() { a.guarded = false }()
	return run()
}

type controlFixture struct{}

func (controlFixture) RefStore(string) (*gitrefstore.Store, error) { return nil, errors.New("unused") }
func (controlFixture) Close() error                                { return nil }

type publisherFixture struct{ document jobs.Document }

func (p publisherFixture) Capture(context.Context, jobs.SourceBinding) (admission.Published, error) {
	encoded, err := jobs.Encode(p.document)
	return admission.Published{Commit: strings.Repeat("b", 40), Jobs: encoded, Config: []byte(harnessconfigtest.YAML), Control: controlFixture{}}, err
}

type claimStoreFixture struct {
	a            *authorityFixture
	called       bool
	unproved     bool
	disposition  claim.Disposition
	claimErr     error
	invalidDaily bool
}

func (s *claimStoreFixture) Claim(_ context.Context, record claim.Record) (claim.ClaimResult, error) {
	s.called = true
	if !s.a.guarded {
		return claim.ClaimResult{}, errors.New("claim ran outside ready guard")
	}
	if s.claimErr != nil {
		return claim.ClaimResult{}, s.claimErr
	}
	if s.invalidDaily {
		record.JobDate = "2026-10-18"
	}
	if s.unproved {
		record.ConfigCommit = strings.Repeat("c", 40)
	}
	disposition := s.disposition
	if disposition == "" {
		disposition = claim.DispositionOwned
	}
	return claim.ClaimResult{Disposition: disposition, Record: record}, nil
}

func (*claimStoreFixture) Transition(context.Context, claim.Record, claim.Terminal) (claim.Record, error) {
	return claim.Record{}, errors.New("unused")
}

func preparedFixture(t *testing.T) (*admission.Prepared, *authorityFixture) {
	t.Helper()
	job := jobs.Job{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "0 2 * * *", Timezone: "America/Los_Angeles"}, Timeout: "60m", Prompt: "not retained",
		Runner: jobs.Runner{Harness: entity.Codex, Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}}}
	document := jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{job}}
	schedules, err := installation.ScheduleFingerprints(document)
	if err != nil {
		t.Fatal(err)
	}
	authority := &authorityFixture{ready: installation.ReadyBinding{Generation: installation.InstallationGeneration(strings.Repeat("a", 32)), Source: document.Source, Schedules: schedules}}
	service := admission.Service{Authority: authority, Publisher: publisherFixture{document: document}, Registry: runtime.LiveRegistry(),
		ProcessSupport: func(context.Context) error { return nil }, Authenticate: func(context.Context, port.AgentRuntime) error { return nil }}
	result, err := service.Prepare(t.Context(), admission.Request{JobID: "daily", StartedUTC: time.Date(2026, 10, 20, 6, 30, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = result.Prepared.Close() })
	return result.Prepared, authority
}

func TestAcquireHoldsReadyGuardAndKeepsCapturedDateAcrossMidnight(t *testing.T) {
	prepared, authority := preparedFixture(t)
	store := &claimStoreFixture{a: authority}
	service := claim.Service{Store: store, Random: bytes.NewReader(bytes.Repeat([]byte{0x7a}, 32)), Now: func() time.Time {
		return time.Date(2026, 10, 22, 4, 0, 0, 0, time.UTC)
	}}
	result, err := service.Acquire(t.Context(), *prepared)
	if err != nil || result.Disposition != claim.DispositionOwned || result.Record.JobDate != "2026-10-19" || result.Record.ClaimedAt.Day() != 22 {
		t.Fatalf("Acquire() = (%+v, %v)", result, err)
	}
	if result.Record.OwnerNonce != strings.Repeat("7a", 32) || authority.guarded {
		t.Fatalf("nonce=%q guarded=%v", result.Record.OwnerNonce, authority.guarded)
	}
}

func TestAcquireRefusesChangedCanceledAndUnprovedAuthority(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		prepare  func(*authorityFixture, *claimStoreFixture) context.Context
		wantCall bool
	}{
		{"changed", func(a *authorityFixture, _ *claimStoreFixture) context.Context {
			a.guardErr = installation.ErrGenerationReplaced
			return context.Background()
		}, false},
		{"canceled", func(_ *authorityFixture, _ *claimStoreFixture) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}, false},
		{"unproved", func(_ *authorityFixture, s *claimStoreFixture) context.Context {
			s.unproved = true
			return context.Background()
		}, true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			prepared, authority := preparedFixture(t)
			store := &claimStoreFixture{a: authority}
			ctx := fixture.prepare(authority, store)
			service := claim.Service{Store: store, Random: bytes.NewReader(make([]byte, 32)), Now: func() time.Time { return time.Now().UTC() }}
			if result, err := service.Acquire(ctx, *prepared); err == nil || result.Disposition == claim.DispositionOwned || store.called != fixture.wantCall {
				t.Fatalf("Acquire() = (%+v, %v), called=%v", result, err, store.called)
			}
		})
	}
}

func TestAcquireDependencyRandomAndPortRefusals(t *testing.T) {
	prepared, authority := preparedFixture(t)
	if _, err := (claim.Service{}).Acquire(t.Context(), *prepared); err == nil {
		t.Fatal("acquire without store succeeded")
	}
	store := &claimStoreFixture{a: authority}
	if _, err := (claim.Service{Store: store, Random: errorReader{}}).Acquire(t.Context(), *prepared); err == nil || store.called {
		t.Fatalf("random failure = %v, called=%v", err, store.called)
	}
	for name, configure := range map[string]func(*claimStoreFixture){
		"port error":           func(s *claimStoreFixture) { s.claimErr = errors.New("claim unavailable") },
		"invalid daily record": func(s *claimStoreFixture) { s.invalidDaily = true },
		"invalid disposition":  func(s *claimStoreFixture) { s.disposition = "future" },
	} {
		t.Run(name, func(t *testing.T) {
			store := &claimStoreFixture{a: authority}
			configure(store)
			service := claim.Service{Store: store, Random: bytes.NewReader(make([]byte, 32)), Now: func() time.Time { return time.Now().UTC() }}
			if result, err := service.Acquire(t.Context(), *prepared); err == nil || result.Disposition == claim.DispositionOwned {
				t.Fatalf("Acquire() = (%+v, %v)", result, err)
			}
		})
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("random unavailable") }
