package preflight_test

// The cross-knob coherence fixture matrix (design init-hardening-v1 §3.1
// check 6; plan init-hardening/001 T3's AC4): the wedged pair — ci_mode
// excluding hosted CI while wiring.protection requires the hosted check
// context — refuses pre-write naming both remedies; every other
// combination passes; the check is unconditionally applicable (it gates
// combinations, not gh transport).

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

func TestCrossKnobCoherenceAlwaysApplicable(t *testing.T) {
	for _, c := range preflight.Registry() {
		if c.ID == preflight.CrossKnobCoherenceID {
			if c.Applicability != nil {
				t.Error("cross-knob-coherence's Applicability is non-nil — it must run unconditionally, like the enum gate it complements")
			}
			return
		}
	}
	t.Fatal("Registry() carries no cross-knob-coherence check")
}

// TestCrossKnobCoherenceWedgedPairRefuses is T3's AC4: the wedged knob
// pair refuses pre-write naming both remedies.
func TestCrossKnobCoherenceWedgedPairRefuses(t *testing.T) {
	cases := []struct {
		name               string
		ciMode, protection string
	}{
		{"local excludes hosted CI + required-checks needs it", "local", "required-checks"},
		{"local excludes hosted CI + checks+review needs it", "local", "checks+review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := t.TempDir()
			runner := newHappyRunner(t)
			knobs := config.Knobs{Tracker: "github", CIMode: tc.ciMode, Wiring: config.Wiring{Repo: "use", Protection: tc.protection}}
			rep, err := preflight.Run(context.Background(), preflight.Input{
				Target: target, Knobs: knobs, Coordinate: "octo/widget", WireRunner: runner,
			})
			if err == nil {
				t.Fatal("Run = nil error, want a refusal")
			}
			out, ok := outcomeFor(t, rep, preflight.CrossKnobCoherenceID)
			if !ok || out.Status != preflight.StatusRefuse {
				t.Fatalf("cross-knob-coherence outcome = %+v (found=%v), want refuse", out, ok)
			}
			if !strings.Contains(out.Remedy, "actions or both") {
				t.Errorf("remedy %q does not name the hosted-CI remedy", out.Remedy)
			}
			if !strings.Contains(out.Remedy, "wiring.protection to none") {
				t.Errorf("remedy %q does not name the wiring.protection=none remedy", out.Remedy)
			}
		})
	}
}

// TestCrossKnobCoherencePassesWhenNotWedged covers every non-wedged
// combination touching the same two knobs.
func TestCrossKnobCoherencePassesWhenNotWedged(t *testing.T) {
	cases := []struct {
		name               string
		ciMode, protection string
	}{
		{"local + none — no hosted check required", "local", "none"},
		{"actions + required-checks — hosted CI included", "actions", "required-checks"},
		{"both + checks+review — hosted CI included", "both", "checks+review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := t.TempDir()
			runner := newHappyRunner(t)
			knobs := config.Knobs{Tracker: "github", CIMode: tc.ciMode, Wiring: config.Wiring{Repo: "use", Protection: tc.protection}}
			rep, err := preflight.Run(context.Background(), preflight.Input{
				Target: target, Knobs: knobs, Coordinate: "octo/widget", WireRunner: runner,
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			out, ok := outcomeFor(t, rep, preflight.CrossKnobCoherenceID)
			if !ok || out.Status != preflight.StatusPass {
				t.Fatalf("cross-knob-coherence outcome = %+v (found=%v), want pass", out, ok)
			}
		})
	}
}

// TestCrossKnobCoherenceRunsEvenWhenGitHubModeInapplicable proves the
// check's own unconditional applicability independent of any gh-transport
// concern: a knob set with NO declared gh op (so github-mode/scope-
// prediction never run) still evaluates coherence.
func TestCrossKnobCoherenceRunsEvenWhenGitHubModeInapplicable(t *testing.T) {
	target := t.TempDir()
	runner := &scriptedRunner{t: t, failOnCall: true}
	// tracker=carina, wiring.repo=use, protection=none: zero gh ops
	// declared (githubModeApplicable is false) — ci_mode=local alone
	// cannot wedge without a protection level requiring the hosted check.
	knobs := config.Knobs{Tracker: "carina", CIMode: "local", Wiring: config.Wiring{Repo: "use", Protection: "none"}}
	rep, err := preflight.Run(context.Background(), preflight.Input{Target: target, Knobs: knobs, WireRunner: runner})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, ok := outcomeFor(t, rep, preflight.CrossKnobCoherenceID)
	if !ok || out.Status != preflight.StatusPass {
		t.Fatalf("cross-knob-coherence outcome = %+v (found=%v), want pass — it must run regardless of gh applicability", out, ok)
	}
	if len(runner.calls) != 0 {
		t.Errorf("gh was called %d time(s) even though no gh op was declared", len(runner.calls))
	}
}
