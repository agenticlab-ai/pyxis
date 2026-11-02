package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestParseHintClosedSchema(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		ok   bool
	}{
		{"empty object", "{}", true},
		{"empty object with trailing whitespace", "{}\n", true},
		{"empty string", "", false},
		{"not json", "check now", false},
		{"extra field", `{"note":"urgent"}`, false},
		{"trailing data", "{}{}", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, defect := ParseHint(tc.raw)
			if tc.ok {
				if defect != nil {
					t.Fatalf("ParseHint(%q) = defect %+v, want ok", tc.raw, defect)
				}
				return
			}
			if defect == nil {
				t.Fatalf("ParseHint(%q) = ok, want a named defect", tc.raw)
			}
			if defect.Reason == "" || defect.Remedy == "" {
				t.Fatalf("defect for %q is not named: %+v", tc.raw, defect)
			}
		})
	}
}

func TestStubDeliverRejectsPayload(t *testing.T) {
	s := NewStub()

	if defect := s.Deliver(context.Background(), `{"reason":"merge"}`); defect == nil {
		t.Fatal("Deliver accepted a hint carrying extra payload")
	}
	select {
	case <-s.Hints():
		t.Fatal("a rejected hint must never reach the loop's channel")
	default:
	}

	if defect := s.Deliver(context.Background(), "{}"); defect != nil {
		t.Fatalf("Deliver rejected a well-formed hint: %+v", defect)
	}
	select {
	case <-s.Hints():
	default:
		t.Fatal("a well-formed hint must reach the loop's channel")
	}
}

// waitForReads observes the completed cycle after the loop parks on cadence.
func waitForReads(t *testing.T, calls *atomic.Int64, want int64) {
	t.Helper()
	synctest.Wait()
	if got := calls.Load(); got != want {
		t.Fatalf("reads = %d, want %d after the loop parked", got, want)
	}
}

// TestRunLiveHintAdvancesNextCheckOnly is the plan's first T10 drill: with the
// stub wired, a delivered hint advances the next check to now and nothing
// else. cadence is an hour and the fake clock is never fed a tick, so the
// only thing that can end the wait is the hint — proving the acceleration is
// real — and the read/sweep counts settle at exactly one extra
// read-then-forced-sweep cycle, the same shape one cadence tick would have
// produced (state and awaits never change; this fixture's derived frontier
// is constant throughout).
//
// The fixture pairs a ready-unclaimed task with an open proposal: the ready
// task is what makes the frontier actionable at all (only ready-unclaimed
// awaits invoke a sweep — readyKey), while the proposal is what gives the
// reader something to read, so both counters this drill measures move.
func TestRunLiveHintAdvancesNextCheckOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ref := port.ProposalRef{Repo: "r", Number: 1}
		reader := &fakeReader{states: []port.ProposalState{{Landing: port.LandingOpen}}, cadence: time.Hour}
		clock := &fakeClock{ticks: make(chan time.Time)} // never fed: only a hint can end a wait
		stub := NewStub()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var sweeps atomic.Int64
		done := make(chan error, 1)
		go func() {
			plan := Plan{Tasks: []Task{{ID: "open", Proposal: &ref}, {ID: "ready"}}}
			_, err := RunLive(ctx, constantSource(plan), reader, func(context.Context) error { sweeps.Add(1); return nil }, clock, stub.Hints())
			done <- err
		}()

		// The initial derivation finds a ready frontier and sweeps; the follow-up
		// derivation finds the same state and settles into the blocking wait (no
		// cadence tick, no hint delivered yet).
		waitForReads(t, &reader.calls, 2)
		if got := sweeps.Load(); got != 1 {
			t.Fatalf("sweeps before any hint = %d, want 1 (the initial derivation only)", got)
		}

		if defect := stub.Deliver(ctx, "{}"); defect != nil {
			t.Fatalf("Deliver rejected a well-formed hint: %+v", defect)
		}

		// The hint ends the wait at once: one more read-and-forced-sweep cycle,
		// then a follow-up read that again settles into the blocking wait.
		waitForReads(t, &reader.calls, 4)
		if got := sweeps.Load(); got != 2 {
			t.Fatalf("sweeps after one hint = %d, want 2 (one extra cycle, nothing more)", got)
		}

		// Quiescent afterward: no runaway, no phantom second wake from one hint.
		synctest.Wait()
		if got := reader.calls.Load(); got != 4 {
			t.Fatalf("reads settled at %d, want exactly 4 — the hint must not cause more than one cycle", got)
		}
		if got := sweeps.Load(); got != 2 {
			t.Fatalf("sweeps settled at %d, want exactly 2", got)
		}

		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("RunLive error = %v, want cancellation", err)
		}

	})
}

// TestKillStubMatchesNoSlotAtAll is the plan's kill-stub drill: FR-11 claims
// an absent push adapter and a dead one are behaviorally the same, so this
// drives the loop through an identical cadence-only sequence twice — once
// with hints=nil (no slot ever existed) and once with a constructed Stub
// that nothing ever calls Deliver on (dead throughout) — and asserts the two
// wake-events traces are byte-identical. Because "dead" and "absent" are
// provably the same state at every point in the drive (neither one's hints
// channel ever fires), this equivalence holds no matter when an adapter is
// killed, including mid-drive — there is no history-dependent divergence to
// exploit by picking a different kill moment.
func TestKillStubMatchesNoSlotAtAll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixed := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

		run := func(t *testing.T, hints <-chan struct{}) string {
			t.Helper()
			dir := t.TempDir()
			sink := NewSink(filepath.Join(dir, "wake-events"), 0, func(string, ...any) { t.Fatal("no warning expected") })
			sink.clock = func() time.Time { return fixed }
			ref := port.ProposalRef{Repo: "r", Number: 1}
			inner := &fakeReader{states: []port.ProposalState{{Landing: port.LandingOpen}}, cadence: time.Second}
			reader := &InstrumentReader{Inner: inner, Sink: sink, Medium: "test"}
			clock := &fakeClock{ticks: make(chan time.Time, 4)}
			sweep := WrapSweep(sink, "self-drive-wake/001", func(context.Context) error { return nil })

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				plan := Plan{Tasks: []Task{{ID: "open", Proposal: &ref}, {ID: "ready"}}}
				_, err := RunLive(ctx, constantSource(plan), reader, sweep, clock, hints)
				done <- err
			}()

			// Two cadence ticks, each fed only once the loop has settled from the
			// previous one — deterministic, since with no hint ever delivered the
			// only thing that can end a wait is the tick just sent.
			waitForReads(t, &inner.calls, 2)
			clock.ticks <- fixed
			waitForReads(t, &inner.calls, 4)
			clock.ticks <- fixed
			waitForReads(t, &inner.calls, 6)

			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("RunLive error = %v, want cancellation", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "wake-events", "current.jsonl"))
			if err != nil {
				t.Fatalf("read wake-events trace: %v", err)
			}
			return string(data)
		}

		traceAbsent := run(t, nil)
		deadStub := NewStub() // constructed and wired, but Deliver is never called — dead throughout
		traceDead := run(t, deadStub.Hints())

		if traceAbsent == "" {
			t.Fatal("trace is empty — the drive produced no events")
		}
		if traceAbsent != traceDead {
			t.Fatalf("a dead push slot must be byte-identical to no slot at all:\n--- absent ---\n%s--- dead stub ---\n%s", traceAbsent, traceDead)
		}

	})
}
