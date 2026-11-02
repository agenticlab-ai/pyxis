package preflight_test

// The registry/Run contract (design init-hardening-v1 §3.1; plan
// init-hardening/001 T1): the report-every-outcome, first-refusal-wins
// shape every consumer (init today; preview/doctor later, T9/T10) relies
// on. The admission fixture matrix itself lives in admission_test.go;
// errors.go's rendering lives in errors_test.go.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

func TestRegistryAdmissionFirstAndAlwaysApplicable(t *testing.T) {
	reg := preflight.Registry()
	if len(reg) == 0 {
		t.Fatal("Registry() is empty — admission must always be seeded")
	}
	first := reg[0]
	if first.ID != preflight.AdmissionID {
		t.Errorf("Registry()[0].ID = %q, want %q (admission runs before any knob exists — design §3.1)", first.ID, preflight.AdmissionID)
	}
	if first.Applicability != nil {
		t.Error("the admission check's Applicability is non-nil — it must run unconditionally, before elicitation resolves any knob")
	}
}

// TestRunPassReportsEveryOutcomeNoError proves AP-16 over a clean target: every
// registered check (admission, toolchain, artifact-source — T2) reports a
// passing outcome, and Run returns no error. It does not hardcode the
// registry's length, so it survives T3 appending knob-gated checks.
func TestRunPassReportsEveryOutcomeNoError(t *testing.T) {
	target := t.TempDir()
	rep, err := preflight.Run(context.Background(), preflight.Input{Target: target})
	if err != nil {
		t.Fatalf("Run over an empty directory: %v", err)
	}
	// Zero-value Knobs (this call runs before elicitation, admitTarget's own
	// shape): the gh-facing checks are inapplicable (no gh-transport op is
	// declared by an empty Knobs), while the always-applicable four —
	// admission, toolchain, artifact-source (registered unconditionally; its
	// probe self-skips on a warm cache), and cross-knob-coherence — all
	// evaluate.
	if len(rep) != 5 {
		t.Fatalf("Report has %d outcomes, want admission + toolchain + process-boundary + artifact-source + cross-knob-coherence", len(rep))
	}
	var sawAdmission bool
	for _, out := range rep {
		boundaryAdvisory := out.Check == preflight.ProcessBoundaryID && out.Status == preflight.StatusAdvise && out.Remedy != ""
		if out.Status != preflight.StatusPass && !boundaryAdvisory {
			t.Errorf("outcome %+v is not a pass over a clean target with a working toolchain", out)
		}
		if out.Check == preflight.AdmissionID {
			sawAdmission = true
			if !out.GitInit {
				t.Error("an empty directory's admission outcome must set GitInit — the caller must git init")
			}
		}
	}
	if !sawAdmission {
		t.Fatal("Report carries no admission outcome")
	}
	last := rep[len(rep)-1]
	if last.Check != preflight.CrossKnobCoherenceID || last.Status != preflight.StatusPass {
		t.Errorf("last outcome = %+v, want a passing cross-knob-coherence outcome (§3.1 check 6, evaluated last)", last)
	}
}

// TestRunRefuseReturnsRefusalErrorMatchingTheReport proves the returned error
// carries the FIRST refusing outcome (admission — it is registered first),
// even though Run keeps evaluating and reporting every later applicable check
// (AP-16) rather than stopping at the first refusal.
func TestRunRefuseReturnsRefusalErrorMatchingTheReport(t *testing.T) {
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	rep, err := preflight.Run(context.Background(), preflight.Input{Target: target})
	if err == nil {
		t.Fatal("Run over a content-holding non-git directory = nil error, want a refusal")
	}
	if len(rep) != 5 || rep[0].Check != preflight.AdmissionID || rep[0].Status != preflight.StatusRefuse {
		t.Fatalf("Report = %+v, want five outcomes with admission's refusing outcome first", rep)
	}
	var refusal *preflight.RefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v (%T), want *preflight.RefusalError", err, err)
	}
	if refusal.Outcome.Check != rep[0].Check || refusal.Outcome.Status != rep[0].Status {
		t.Errorf("the returned error's Outcome %+v does not match the report's %+v", refusal.Outcome, rep[0])
	}
	if !errors.Is(err, preflight.ErrRefused) {
		t.Error("errors.Is(err, preflight.ErrRefused) = false, want true")
	}
}
