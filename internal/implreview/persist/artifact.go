package persist

import (
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// FenceTag is the info string on the machine block's fence: the artifact body
// is the human rendering followed by a ```impl-review fenced block carrying the
// payload JSON (§3.2's marker-block realization for a medium lacking a native
// typed record — RW-3). One tag, one block, per artifact.
const FenceTag = "impl-review"

const (
	// fenceOpen / fenceClose bracket the machine block. The close is a bare
	// triple-backtick on its own line (the standard fenced-code close).
	fenceOpen  = "```" + FenceTag
	fenceClose = "```"
)

// artifact-composition sentinels — part of the port's contract so an adapter (or
// its test) can branch on WHY a body failed to compose or parse.
var (
	// ErrNoMachineBlock — the artifact body carries no ```impl-review fenced
	// block; there is nothing authoritative to read (the human region alone is
	// never the source — DINV-2).
	ErrNoMachineBlock = errors.New("impl-review persist: artifact has no impl-review machine block")
	// ErrMultipleMachineBlocks — the body carries more than one ```impl-review
	// block; one self-contained artifact has exactly one payload (DINV-1/DINV-9).
	ErrMultipleMachineBlocks = errors.New("impl-review persist: artifact has multiple impl-review machine blocks")
	// ErrRenderDivergence — the artifact's human region is not report.Render of
	// the parsed payload; a hand-authored region diverged from the payload, which
	// governs (DINV-2). Wraps report.ErrRenderFidelity.
	ErrRenderDivergence = errors.New("impl-review persist: artifact human region diverges from the payload")
)

// ComposeArtifact builds the ONE self-contained artifact body for a review: the
// human rendering (report.Render — never authored separately, DINV-2) followed
// by a single ```impl-review fenced block carrying the payload JSON
// (report.Marshal). Every adapter composes through this one function so a
// proposal reviewed on either medium serializes identically (§3.2, the one
// parser). It never splits the payload; compactness (§3.5) keeps the whole
// artifact within any code-host's per-artifact budget.
//
// The payload is marshaled once here and embedded verbatim; ParseArtifact
// recovers those exact bytes, so a Compose→Parse round-trip is byte-identical.
func ComposeArtifact(r *report.Report) (string, error) {
	if r == nil {
		return "", fmt.Errorf("impl-review persist: compose artifact: nil report")
	}
	payload, err := r.Marshal()
	if err != nil {
		return "", fmt.Errorf("impl-review persist: compose artifact: %w", err)
	}
	// The rendering is the pure human face; the fenced block is authoritative.
	// A trailing newline after the rendering separates it from the fence; the
	// payload sits between the open and close fences on their own lines.
	var b strings.Builder
	b.WriteString(report.Render(r))
	b.WriteString("\n")
	b.WriteString(fenceOpen)
	b.WriteString("\n")
	b.Write(payload)
	b.WriteString("\n")
	b.WriteString(fenceClose)
	b.WriteString("\n")
	return b.String(), nil
}

// ParseArtifact recovers a review from one artifact body: it extracts the single
// ```impl-review fenced block, report.Unmarshal's it back to a *report.Report
// (the machine block is authoritative — §3.2), and enforces render-fidelity —
// the body's human region MUST equal report.Render of the recovered payload
// (DINV-2), so a hand-edited region that diverged is rejected with
// ErrRenderDivergence. It returns ErrNoMachineBlock / ErrMultipleMachineBlocks
// when the body is not exactly one self-contained artifact.
//
// Because ComposeArtifact embedded the marshaled bytes verbatim and Unmarshal is
// the inverse of Marshal on a valid payload, the recovered *report.Report
// re-marshals byte-identically to the persisted payload (the round-trip
// byte-identity the conformance suite asserts).
func ParseArtifact(body string) (*report.Report, error) {
	human, block, err := splitArtifact(body)
	if err != nil {
		return nil, err
	}
	r, err := report.Unmarshal([]byte(block))
	if err != nil {
		return nil, fmt.Errorf("impl-review persist: parse artifact: %w", err)
	}
	// DINV-2: the human region is a pure rendering of the payload. Compare the
	// stored region against a fresh Render; a divergence is invalid.
	if ferr := report.ValidateRenderFidelity(r, human); ferr != nil {
		return nil, fmt.Errorf("%w: %w", ErrRenderDivergence, ferr)
	}
	return r, nil
}

// ParseHandoff recovers a review from an UNTRUSTED reviewer handoff — the
// final message of a fresh reviewer process, before the host has published
// anything. Like ParseArtifact it requires exactly one ```impl-review machine
// block and reads the payload from it (the block is authoritative), but it does
// NOT require the surrounding prose to be a byte-exact report.Render of the
// payload: a language-model reviewer cannot reproduce the renderer verbatim,
// and the host never publishes the handoff's prose anyway — every publisher
// recomposes the artifact from the parsed payload through ComposeArtifact, so
// DINV-2 (the human face is a pure rendering of the payload) holds for what is
// published. Render fidelity stays enforced by ParseArtifact where it matters:
// reading back an artifact the host itself composed.
func ParseHandoff(body string) (*report.Report, error) {
	_, block, err := splitArtifact(body)
	if err != nil {
		return nil, err
	}
	r, err := report.Unmarshal([]byte(block))
	if err != nil {
		return nil, fmt.Errorf("impl-review persist: parse handoff: %w", err)
	}
	return r, nil
}

// splitArtifact separates the body into its human region (everything before the
// single machine block, trailing newline trimmed) and the machine block's raw
// payload bytes (everything strictly between the open and close fences). It
// fails closed on zero or multiple ```impl-review blocks so "one self-contained
// artifact" is a parse invariant, not an assumption.
func splitArtifact(body string) (human, payload string, err error) {
	lines := strings.Split(body, "\n")

	// Find the open-fence line(s). The open fence is a line whose trimmed value
	// is exactly the ```impl-review info string (a bare ``` would be a different,
	// untagged block and is not ours).
	openIdxs := make([]int, 0, 2)
	for i, ln := range lines {
		if strings.TrimSpace(ln) == fenceOpen {
			openIdxs = append(openIdxs, i)
		}
	}
	switch {
	case len(openIdxs) == 0:
		return "", "", ErrNoMachineBlock
	case len(openIdxs) > 1:
		return "", "", ErrMultipleMachineBlocks
	}
	open := openIdxs[0]

	// Find the matching close fence — the first bare ``` line after the open.
	close := -1
	for i := open + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == fenceClose {
			close = i
			break
		}
	}
	if close == -1 {
		return "", "", fmt.Errorf("%w: unterminated machine block", ErrNoMachineBlock)
	}

	// The human region is everything before the open fence. ComposeArtifact wrote
	// report.Render(r) — which ends in its own "\n" — then a single "\n" before
	// the fence. Splitting on "\n" turns that trailing "\n\n" into a final empty
	// line element; rejoining lines[:open] with "\n" reconstructs exactly
	// report.Render(r) (the empty element restores Render's own trailing newline
	// via the join separator). No trim is needed or correct — a trim would drop
	// Render's trailing newline and spuriously fail render-fidelity.
	human = strings.Join(lines[:open], "\n")

	// The payload is the lines strictly between the fences, rejoined with "\n"
	// (byte-identical to the marshaled JSON ComposeArtifact embedded).
	payload = strings.Join(lines[open+1:close], "\n")
	return human, payload, nil
}
