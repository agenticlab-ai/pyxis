package wake

import (
	"math/rand"
	"testing"
	"testing/quick"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
)

// Stale is bound-exclusive: an age exactly at the bound is not yet stale, one
// tick older is — matching argusledger.WakeStaleness's own reading (the same
// bound the argus tick has always used), so a second consumer sharing this
// predicate inherits the identical edge behavior.
func TestStale_BoundIsExclusive(t *testing.T) {
	bound := 36 * time.Hour
	if Stale(bound, bound) {
		t.Fatal("age exactly at the bound reported stale")
	}
	if !Stale(bound+time.Nanosecond, bound) {
		t.Fatal("age one tick past the bound not reported stale")
	}
	if Stale(bound-time.Nanosecond, bound) {
		t.Fatal("age one tick short of the bound reported stale")
	}
}

// Two consumers with different bounds get consistent — independently
// correct — verdicts out of the ONE Stale implementation over a shared
// fixture of ages, proving the predicate is genuinely parametrized rather
// than closed over one consumer's bound (self-drive-wake/001 T6; wake-
// watch-v2 DINV-7's "no second implementation" bar).
func TestStale_TwoConsumersConsistentVerdicts(t *testing.T) {
	ages := []time.Duration{
		0,
		30 * time.Minute,
		time.Hour,
		90 * time.Minute,
		2 * time.Hour,
		25 * time.Hour,
		10 * 24 * time.Hour,
	}
	consumers := []struct {
		name  string
		bound time.Duration
	}{
		{"argus backstop (config.Channel default staleness bound)", 24 * time.Hour},
		{"a second consumer (wake-watch-v2 §3.8's review-wait threshold)", 2 * time.Hour},
	}
	for _, c := range consumers {
		for _, age := range ages {
			want := age > c.bound
			if got := Stale(age, c.bound); got != want {
				t.Errorf("%s: Stale(%s, %s) = %v, want %v", c.name, age, c.bound, got, want)
			}
		}
	}
}

// The shared predicate's verdict over a since/now pair agrees with
// argusledger.WakeStaleness's own verdict for the equivalent Await, on every
// fixture in the shared table — the argus stack's existing (untouched)
// implementation and this package's new parametrized primitive compute the
// same "too old?" answer, proving single-sourcing rather than a second,
// independently-drifting predicate (design-brainstorm:self-drive-wake-tech-
// design#D-1; the twin-predicate drift class this task closes).
func TestStale_AgreesWithArgusledgerWakeStaleness(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	bound := 24 * time.Hour
	fixture := []struct {
		name  string
		since string
	}{
		{"5 days old (well past a 24h bound)", "2026-07-10"},
		{"12h old (well within a 24h bound)", "2026-07-15"},
		{"36h old (just past a 24h bound)", "2026-07-14"},
		{"60h old (further past a 24h bound)", "2026-07-13"},
	}
	for _, c := range fixture {
		since, err := time.Parse("2006-01-02", c.since)
		if err != nil {
			t.Fatalf("fixture since %q: %v", c.since, err)
		}
		age := now.Sub(since.UTC())
		want := Stale(age, bound)

		awaits := []argusledger.Await{{GoalObject: "pr:1", Condition: "merged", Since: c.since}}
		defects := argusledger.WakeStaleness("wake", "docs/problems/foo/delivery.md", awaits, bound, now)
		got := len(defects) > 0
		if got != want {
			t.Errorf("%s: wake.Stale=%v vs argusledger.WakeStaleness stale=%v (single-source disagreement)", c.name, want, got)
		}
	}
}

// The bound>cadence relation refuses any pair where the bound does not
// strictly exceed the cadence — a property proven over random inputs (T6's
// own bar: "a property test: the bound>cadence relation refuses inverted
// parameters"), not just hand-picked edge cases. The random source is
// fixed-seeded so the test stays hermetic and deterministic across runs (the
// repo Go rules: "no clock/randomness without injection") — testing/quick's
// own default source is wall-clock-seeded, so it is never used bare here.
func TestValidateBound_PropertyRefusesNonStrictRelation(t *testing.T) {
	prop := func(boundMs, cadenceMs uint32) bool {
		bound := time.Duration(boundMs) * time.Millisecond
		cadence := time.Duration(cadenceMs) * time.Millisecond
		err := ValidateBound(bound, cadence)
		if bound > cadence {
			return err == nil
		}
		return err != nil
	}
	cfg := &quick.Config{MaxCount: 5000, Rand: rand.New(rand.NewSource(1))}
	if err := quick.Check(prop, cfg); err != nil {
		t.Error(err)
	}
}

// Hand-picked edges alongside the property test: equal bound/cadence and an
// inverted pair are both refused; a bound one tick past the cadence is not.
func TestValidateBound_Edges(t *testing.T) {
	cases := []struct {
		name           string
		bound, cadence time.Duration
		wantErr        bool
	}{
		{"equal bound and cadence", time.Hour, time.Hour, true},
		{"inverted: bound under cadence", 30 * time.Minute, time.Hour, true},
		{"valid: bound strictly exceeds cadence", 25 * time.Hour, 24 * time.Hour, false},
		{"one nanosecond past the cadence", time.Hour + time.Nanosecond, time.Hour, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateBound(c.bound, c.cadence)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateBound(%s, %s) err = %v, wantErr %v", c.bound, c.cadence, err, c.wantErr)
			}
		})
	}
}

// The relation as wired for the argus backstop's own config today
// (config.Defaults: 24h bound, 1h cadence) validates clean — proving the
// shared relation check agrees with the argus stack's live values, not just
// synthetic ones.
func TestValidateBound_ArgusDefaultsHold(t *testing.T) {
	if err := ValidateBound(24*time.Hour, time.Hour); err != nil {
		t.Fatalf("the argus stack's own default staleness bound/cadence pair failed the shared relation check: %v", err)
	}
}
