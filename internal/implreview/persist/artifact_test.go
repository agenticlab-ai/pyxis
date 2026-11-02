package persist

import (
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestComposeParseRoundTrip — a composed artifact parses back to a byte-identical
// payload, and its human region is exactly report.Render (DINV-2). This is the
// medium-agnostic core the adapters share.
func TestComposeParseRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		r    *report.Report
	}{
		{"normal", goldenReport()},
		{"oversize", oversizeReport()},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := ComposeArtifact(tc.r)
			if err != nil {
				t.Fatalf("compose: %v", err)
			}
			// Exactly one machine block in the composed body.
			if n := strings.Count(body, fenceOpen); n != 1 {
				t.Fatalf("composed body has %d %q fences, want 1", n, fenceOpen)
			}
			got, err := ParseArtifact(body)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			assertByteIdentical(t, tc.r, got)
		})
	}
}

// TestParseArtifactRejectsRenderDivergence — a body whose human region was
// hand-edited to diverge from the payload is rejected (DINV-2). The machine
// block is untouched; only the prose above it changed.
func TestParseArtifactRejectsRenderDivergence(t *testing.T) {
	t.Parallel()
	body, err := ComposeArtifact(goldenReport())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	// Corrupt the human region: prepend a hand-authored line the payload never
	// produced. The machine block is unchanged, so the payload still parses — the
	// divergence must be caught by render-fidelity, not by the parse.
	tampered := "a reviewer's hand-typed summary that is NOT from the payload\n" + body
	_, err = ParseArtifact(tampered)
	if !errors.Is(err, ErrRenderDivergence) {
		t.Fatalf("tampered human region: got %v, want ErrRenderDivergence", err)
	}
	// It also chains to the report layer's fidelity sentinel.
	if !errors.Is(err, report.ErrRenderFidelity) {
		t.Fatalf("divergence error does not chain report.ErrRenderFidelity: %v", err)
	}
}

// TestParseArtifactNoMachineBlock — a body with no ```impl-review block has
// nothing authoritative to read.
func TestParseArtifactNoMachineBlock(t *testing.T) {
	t.Parallel()
	_, err := ParseArtifact("just some prose, no fenced payload here\n")
	if !errors.Is(err, ErrNoMachineBlock) {
		t.Fatalf("got %v, want ErrNoMachineBlock", err)
	}
}

// TestParseArtifactMultipleMachineBlocks — one self-contained artifact carries
// exactly one payload; two blocks is invalid (DINV-1/DINV-9).
func TestParseArtifactMultipleMachineBlocks(t *testing.T) {
	t.Parallel()
	body, err := ComposeArtifact(goldenReport())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	// Append a second machine block (a split/mirror attempt).
	doubled := body + "\n" + body
	_, err = ParseArtifact(doubled)
	if !errors.Is(err, ErrMultipleMachineBlocks) {
		t.Fatalf("got %v, want ErrMultipleMachineBlocks", err)
	}
}

// assertByteIdentical fails unless two reports marshal to identical bytes.
func assertByteIdentical(t *testing.T, want, got *report.Report) {
	t.Helper()
	wb, err := want.Marshal()
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	gb, err := got.Marshal()
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if string(wb) != string(gb) {
		t.Fatalf("not byte-identical:\n--- want ---\n%s\n--- got ---\n%s", wb, gb)
	}
}

// ParseHandoff reads the machine block of an untrusted reviewer handoff without
// demanding render fidelity of the surrounding prose (which is never
// published), while still requiring exactly one block.
func TestParseHandoffAcceptsHandAuthoredProseAroundOneMachineBlock(t *testing.T) {
	r := &report.Report{SchemaVersion: report.SchemaVersion, ProposalRef: "local:o/r@feat/x", ReviewID: "proc-1",
		Band: report.BandStandard, Verdict: report.VerdictApproveWithAdvisories,
		Rounds: []report.Round{{ReviewedRevision: "abc123", ContextID: "ctx",
			Provenance: report.Provenance{Model: "m", PromptsVersion: "v1", CalVersion: "v1"}}}}
	payload, err := r.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	body := "# my own summary\n\nFour lenses ran; nothing blocks.\n\n```impl-review\n" + string(payload) + "\n```\n\nthanks!\n"
	if _, err := ParseArtifact(body); !errors.Is(err, ErrRenderDivergence) {
		t.Fatalf("ParseArtifact must still reject a diverged human region, got %v", err)
	}
	got, err := ParseHandoff(body)
	if err != nil {
		t.Fatalf("ParseHandoff: %v", err)
	}
	if got.ProposalRef != r.ProposalRef || got.ReviewID != r.ReviewID || got.Verdict != r.Verdict {
		t.Fatalf("ParseHandoff = %+v", got)
	}
	if _, err := ParseHandoff("no block here"); !errors.Is(err, ErrNoMachineBlock) {
		t.Fatalf("no block: %v", err)
	}
	if _, err := ParseHandoff(body + "```impl-review\n{}\n```\n"); !errors.Is(err, ErrMultipleMachineBlocks) {
		t.Fatalf("two blocks: %v", err)
	}
	if _, err := ParseHandoff("```impl-review\nnot json\n```\n"); err == nil {
		t.Fatal("malformed payload must fail")
	}
}
