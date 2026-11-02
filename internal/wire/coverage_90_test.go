package wire

import (
	"context"
	"testing"
	"time"
)

func TestCoverage90FindMarkerRejectsMalformedJSON(t *testing.T) {
	if _, _, err := findMarker("not-json", WelcomeMarker); err == nil {
		t.Fatal("findMarker accepted malformed JSON")
	}
}

func TestCoverage90EnsureLabelsFailureClasses(t *testing.T) {
	op := findOp(t, Table(knobsConfig("github", "local", "use", "required-checks")), OpEnsureLabels)
	for _, tc := range []struct {
		name   string
		runner func(*testing.T) *fakeRunner
	}{
		{name: "permission list", runner: func(t *testing.T) *fakeRunner { return newFake(t).on(-1, "", "HTTP 403 forbidden", 1, "label list") }},
		{name: "transient list", runner: func(t *testing.T) *fakeRunner { return newFake(t).on(-1, "", "HTTP 503 unavailable", 1, "label list") }},
		{name: "unclassified list", runner: func(t *testing.T) *fakeRunner {
			return newFake(t).on(-1, "", "unexpected list failure", 1, "label list")
		}},
		{name: "malformed list", runner: func(t *testing.T) *fakeRunner { return newFake(t).on(-1, "not-json", "", 0, "label list") }},
		{name: "transient create", runner: func(t *testing.T) *fakeRunner {
			return newFake(t).on(-1, "[]", "", 0, "label list").on(-1, "", "HTTP 503 unavailable", 1, "label create")
		}},
		{name: "unclassified create", runner: func(t *testing.T) *fakeRunner {
			return newFake(t).on(-1, "[]", "", 0, "label list").on(-1, "", "unexpected create failure", 1, "label create")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := boundaryExecutor(tc.runner(t), func(context.Context, time.Duration) error { return nil })
			result := &OpResult{Name: op.Name, Transport: op.Transport}
			if err := x.opEnsureLabels(t.Context(), op, result, &walkState{}); err != nil {
				t.Fatal(err)
			}
			if result.Outcome != OutcomeHandedOff && result.Outcome != OutcomeFailed {
				t.Fatalf("outcome = %q, want handed-off or failed", result.Outcome)
			}
		})
	}
}

func TestCoverage90ProtectionAndWelcomeFailureClasses(t *testing.T) {
	ops := Table(knobsConfig("github", "local", "use", "required-checks"))
	protection := findOp(t, ops, OpSetProtection)
	x := boundaryExecutor(newFake(t).on(-1, "", "unexpected protection failure", 1, "branches/main/protection"), func(context.Context, time.Duration) error { return nil })
	result := &OpResult{Name: protection.Name, Transport: protection.Transport}
	if err := x.opSetProtection(t.Context(), protection, result, &walkState{}); err != nil || result.Outcome != OutcomeFailed {
		t.Fatalf("protection result=%+v err=%v", result, err)
	}

	welcome := findOp(t, ops, OpWelcomeItem)
	for _, tc := range []struct {
		name   string
		stdout string
		stderr string
		exit   int
	}{
		{name: "permission", stderr: "HTTP 403 forbidden", exit: 1},
		{name: "transient", stderr: "HTTP 503 unavailable", exit: 1},
		{name: "unclassified", stderr: "unexpected probe failure", exit: 1},
		{name: "malformed", stdout: "not-json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := newFake(t).on(-1, tc.stdout, tc.stderr, tc.exit, "issue list")
			x := boundaryExecutor(runner, func(context.Context, time.Duration) error { return nil })
			result := &OpResult{Name: welcome.Name, Transport: welcome.Transport}
			if err := x.opWelcomeItem(t.Context(), welcome, result, &walkState{}); err != nil {
				t.Fatal(err)
			}
			if result.Outcome != OutcomeHandedOff && result.Outcome != OutcomeFailed {
				t.Fatalf("outcome = %q, want handed-off or failed", result.Outcome)
			}
		})
	}
}
