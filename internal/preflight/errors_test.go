package preflight_test

// RefusalError's rendering and sentinel-matching contract (design
// init-hardening-v1 §3.1's Key interfaces: "the refusal renderer").

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

func TestRefusalErrorMessageIncludesReasonConflictsRemedy(t *testing.T) {
	err := &preflight.RefusalError{Outcome: preflight.Outcome{
		Check:     preflight.AdmissionID,
		Status:    preflight.StatusRefuse,
		Reason:    "the target holds content",
		Conflicts: []string{"a.txt", "b.txt"},
		Remedy:    "run `pyxis adopt`",
		Route:     "pyxis adopt",
	}}
	msg := err.Error()
	for _, want := range []string{"the target holds content", "a.txt", "b.txt", "pyxis adopt"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, missing %q", msg, want)
		}
	}
}

func TestRefusalErrorMessageWithoutConflictsOrRemedy(t *testing.T) {
	err := &preflight.RefusalError{Outcome: preflight.Outcome{Reason: "bare reason"}}
	if got := err.Error(); got != "bare reason" {
		t.Errorf("Error() = %q, want exactly the reason with nothing appended for absent fields", got)
	}
}

func TestErrRefusedMatchesAnyRefusalError(t *testing.T) {
	err := &preflight.RefusalError{Outcome: preflight.Outcome{Reason: "x"}}
	if !errors.Is(err, preflight.ErrRefused) {
		t.Error("errors.Is(err, ErrRefused) = false, want true for any *RefusalError")
	}
	if errors.Is(errors.New("unrelated"), preflight.ErrRefused) {
		t.Error("errors.Is matched an unrelated error")
	}
}

func TestErrorsAsExtractsOutcomeThroughAWrap(t *testing.T) {
	orig := preflight.Outcome{Check: preflight.AdmissionID, Remedy: "fix it"}
	wrapped := fmt.Errorf("init: %w", &preflight.RefusalError{Outcome: orig})

	var refusal *preflight.RefusalError
	if !errors.As(wrapped, &refusal) {
		t.Fatal("errors.As did not find *RefusalError through a %w wrap")
	}
	if refusal.Outcome.Check != orig.Check || refusal.Outcome.Remedy != orig.Remedy {
		t.Errorf("extracted Outcome = %+v, want Check/Remedy from %+v", refusal.Outcome, orig)
	}
	if !errors.Is(wrapped, preflight.ErrRefused) {
		t.Error("errors.Is did not match ErrRefused through the wrap")
	}
}
