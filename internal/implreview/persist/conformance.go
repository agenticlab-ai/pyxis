package persist

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// Conformance is the port-conformance suite EVERY persistence adapter must pass
// (§3.2's port-conformance idea, A-3, R1): one contract, run against each
// adapter, so a proposal reviewed on either medium round-trips the SAME
// impl-review-report/v1 bytes and the drift risk (R1) is a per-adapter
// conformance failure, never a cross-medium reconciliation.
//
// An adapter's test calls RunConformance with a factory that builds a FRESH,
// empty Port (hermetic — a fake client, no network). The suite exercises:
//   - round-trip byte-identity on a normal payload,
//   - round-trip byte-identity on an OVERSIZE payload (the A-3 oversize leg),
//   - render-from-payload: the recovered artifact's human region is
//     report.Render of the payload (DINV-2),
//   - one-artifact: a Persist writes exactly one self-contained artifact
//     (asserted via the factory's ArtifactCount hook).
//
// The suite does NOT assert medium-specific behavior (comment ids, retry) — an
// adapter tests those itself; the suite pins the shared contract.
type Conformance struct {
	// NewPort builds a fresh, empty Port for one sub-test (hermetic). Called once
	// per case.
	NewPort func(t *testing.T) LegacyPort
	// ArtifactCount reports how many self-contained artifacts a Port has
	// persisted — the one-artifact assertion (exactly one per Persist, never a
	// split or a second-medium write). An adapter whose backing store can count
	// its artifacts supplies this; if nil, the one-artifact case is skipped
	// (the adapter must then assert it directly).
	ArtifactCount func(p LegacyPort) int
}

// RunConformance runs the shared suite against the adapter the Conformance
// factory builds. Call it from an adapter's _test.go:
//
//	persist.RunConformance(t, persist.Conformance{NewPort: ..., ArtifactCount: ...})
func RunConformance(t *testing.T, c Conformance) {
	t.Helper()
	if c.NewPort == nil {
		t.Fatal("conformance: NewPort is required")
	}

	t.Run("round-trip byte-identity (normal payload)", func(t *testing.T) {
		roundTrip(t, c.NewPort(t), goldenReport())
	})

	t.Run("round-trip byte-identity (oversize payload)", func(t *testing.T) {
		roundTrip(t, c.NewPort(t), oversizeReport())
	})

	t.Run("render-from-payload (human region is Render)", func(t *testing.T) {
		if err := validateRenderRoundTrip(c.NewPort(t), goldenReport()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("one self-contained artifact per persist", func(t *testing.T) {
		if c.ArtifactCount == nil {
			t.Skip("adapter supplied no ArtifactCount hook; it asserts one-artifact directly")
		}
		p := c.NewPort(t)
		if n := c.ArtifactCount(p); n != 0 {
			t.Fatalf("fresh port already has %d artifacts, want 0", n)
		}
		if _, err := p.Persist(context.Background(), goldenReport()); err != nil {
			t.Fatalf("persist: %v", err)
		}
		if n := c.ArtifactCount(p); n != 1 {
			t.Fatalf("after one Persist the port has %d artifacts, want exactly 1 "+
				"(one self-contained artifact — DINV-9; no split, no second medium)", n)
		}
	})
}

// roundTrip asserts Persist→Read recovers a byte-identical payload: the
// recovered report re-marshals to the SAME bytes the input marshals to (the
// round-trip byte-identity contract, A-3).
func roundTrip(t *testing.T, p LegacyPort, r *report.Report) {
	t.Helper()
	if err := validateRoundTrip(p, r); err != nil {
		t.Fatal(err)
	}
}

// validateRoundTrip carries the assertion mechanics behind the shared suite.
// Returning a diagnostic keeps every adapter's public conformance behavior the
// same while making transport and corruption failures independently testable.
func validateRoundTrip(p LegacyPort, r *report.Report) error {
	want, err := r.Marshal()
	if err != nil {
		return fmt.Errorf("marshal input: %w", err)
	}
	ref, err := p.Persist(context.Background(), r)
	if err != nil {
		return fmt.Errorf("persist: %w", err)
	}
	got, err := p.Read(context.Background(), ref)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	gotBytes, err := got.Marshal()
	if err != nil {
		return fmt.Errorf("marshal recovered: %w", err)
	}
	if !bytes.Equal(want, gotBytes) {
		return fmt.Errorf("payload not byte-identical through the adapter:\n--- want ---\n%s\n--- got ---\n%s", want, gotBytes)
	}
	return nil
}

func validateRenderRoundTrip(p LegacyPort, r *report.Report) error {
	return validateRenderRoundTripWithRenderer(p, r, report.Render)
}

func validateRenderRoundTripWithRenderer(p LegacyPort, r *report.Report, render func(*report.Report) string) error {
	ref, err := p.Persist(context.Background(), r)
	if err != nil {
		return fmt.Errorf("persist: %w", err)
	}
	got, err := p.Read(context.Background(), ref)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	// The recovered payload re-renders identically — the human face is a pure
	// function of the payload (DINV-2). ValidateRenderFidelity of the recovered
	// payload against its own Render must hold.
	if err := report.ValidateRenderFidelity(got, render(got)); err != nil {
		return fmt.Errorf("render fidelity of recovered payload: %w", err)
	}
	return nil
}

// goldenReport is a small, valid, distinct-content report the suite round-trips
// — the shared fixture (the shape mirrors report/testdata/golden-valid.json,
// kept in Go so the persist package has no cross-package testdata dependency).
func goldenReport() *report.Report {
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "github:agenticlab-ai/pyxis#412",
		ReviewID:      "rv-conf-1",
		Band:          report.BandStandard,
		Verdict:       report.VerdictRequestChanges,
		Rounds: []report.Round{
			{
				ReviewedRevision: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
				ContextID:        "ctx-reviewer-root-001",
				Provenance: report.Provenance{
					Model:          "claude-opus-4-8",
					PromptsVersion: "impl-review-prompts/v1",
					CalVersion:     "impl-review-calibration/v1",
				},
				Findings: []report.Finding{
					{
						ID:         "FND-1",
						Path:       "internal/implreview/report/validate.go",
						Line:       42,
						Message:    "the reviewed_revision check trusts a whitespace-only sha as present",
						LogRef:     "run/round-1/spec-fidelity.log",
						RuleID:     "spec-fidelity/revision-presence",
						Lens:       report.LensSpecFidelity,
						Severity:   report.SeverityBlocking,
						CalRow:     "CAL-3",
						Acceptance: "A-2",
						Disposition: &report.Disposition{
							Actor:   report.ActorWorker,
							Verdict: report.VerdictApplied,
						},
					},
				},
				FindingsCount: 1,
				Candidates:    4,
				Killed:        3,
			},
		},
	}
}

// oversizeReport is a valid report whose payload is DELIBERATELY large — a
// findings set with long messages — exercising the A-3 oversize leg: the adapter
// must round-trip it byte-identically as ONE artifact, never splitting it.
// (In the real system compactness — §3.5 — keeps payloads small; the oversize
// leg proves the persistence contract holds even when it is not.)
func oversizeReport() *report.Report {
	const findingsN = 400
	bigMsg := strings.Repeat("a load-bearing evidence sentence anchoring this finding to its span; ", 40)
	findings := make([]report.Finding, 0, findingsN)
	for i := 0; i < findingsN; i++ {
		findings = append(findings, report.Finding{
			ID:         "FND-" + itoa(i+1),
			Path:       "internal/implreview/report/validate.go",
			Line:       i + 1,
			Message:    bigMsg,
			RuleID:     "correctness-critic/oversize-leg",
			Lens:       report.LensCorrectnessCritic,
			Severity:   report.SeverityAdvisory,
			CalRow:     "CAL-7",
			Acceptance: "A-3",
		})
	}
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "github:agenticlab-ai/pyxis#412",
		ReviewID:      "rv-oversize",
		Band:          report.BandDeep,
		Verdict:       report.VerdictApproveWithAdvisories,
		Rounds: []report.Round{
			{
				ReviewedRevision: "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2",
				ContextID:        "ctx-reviewer-root-oversize",
				Provenance: report.Provenance{
					Model:          "claude-opus-4-8",
					PromptsVersion: "impl-review-prompts/v1",
					CalVersion:     "impl-review-calibration/v1",
				},
				Findings:      findings,
				FindingsCount: findingsN,
				Candidates:    findingsN + 10,
				Killed:        10,
			},
		},
	}
}

// itoa is a tiny stdlib-free int→string for fixture ids (avoids importing strconv
// into a fixture builder for one use). Positive input only.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
