// Package pack assembles the one shared read-only review context pack an
// impl-review hands to every lens (docs/tech-design/impl-review-v1.md §3.4,
// the T-1 seam; DINV-5). It is the cost move (NFR-1): one build, read by the
// four lens subagents identically, never four re-reads.
//
// The pack is NOT a new machinery. It REUSES context-pack-v1's
// deterministic-projection assembler (AP-11, reuse not rebuild) over that
// assembler's EXISTING closed admitted-kind set — governing · subject ·
// calibration — with NO new kind and NO edit to
// internal/docgate/contextpack. Each review input maps onto an existing kind:
//
//   - the proposal at its reviewed revision (the diff) + the codegate verdict
//     → subject (the frozen artifact under work, exactly as a PRD is
//     prd-review's subject)
//   - the task charter + the governing design's binding elements → governing
//   - the calibration table → calibration
//
// Finding-blindness (DINV-5) is structural, inherited from contextpack: the
// admitted-kind set has NO authoring-trail kind, so a prior finding or
// disposition has no kind a declaration can name and no path that reads it
// into the pack. This package MIRRORS that boundary at its own interface:
// ReviewInputs carries no field for prior findings/dispositions, so the
// reviewer can never receive an answer key. The exclusion is enforced by
// construction, not by a filter — there is nothing to filter.
//
// Stdlib only, per the repo Go rules.
package pack

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/contextpack"
)

// Stage is the fixed pipeline-stage name recorded in the pack manifest. The
// review context pack is built at one stage — the impl-review boundary — so
// the stage token is a constant, not a caller input.
const Stage = "impl-review"

// The repo-relative paths the review inputs are materialized to inside the
// build root, one per admitted-kind source. They are internal to the pack
// build (a throwaway root, never a persisted tree), fixed for determinism:
// contextpack records them in the manifest and hashes their content into the
// digest, so a stable layout keeps two builds at one input state byte-identical.
const (
	proposalPath    = "review/proposal.diff"      // subject: the diff at its reviewed revision
	verdictPath     = "review/codegate.txt"       // subject: the deterministic codegate verdict
	charterPath     = "review/charter.md"         // governing: the task charter
	designPath      = "review/design-bindings.md" // governing: the design's binding elements
	calibrationPath = "review/calibration.md"     // calibration: the calibration table
)

// ReviewInputs is the five review inputs the pack projects — the whole input
// surface, and NOTHING else. It carries content as bytes (the runtime holds
// them in memory: a diff computed from the proposal, the codegate verdict, the
// charter and design text, the calibration table), plus the Subject slug the
// manifest records.
//
// There is DELIBERATELY no field for prior findings, dispositions, or any
// authoring-trail input (DINV-5): the reviewer is finding-blind, and the
// absence is the enforcement — a caller has no parameter through which an
// answer key could enter the pack.
type ReviewInputs struct {
	// Subject is the review subject slug recorded in the pack manifest
	// (e.g. the proposal reference). Required.
	Subject string

	// Proposal is the proposal at its reviewed revision — the diff. Mapped
	// onto the subject kind (the frozen subject under work). Required.
	Proposal []byte
	// CodegateVerdict is the deterministic codegate verdict — part of the
	// frozen subject, mapped onto the subject kind. Required.
	CodegateVerdict []byte

	// Charter is the task charter — mapped onto the governing kind. Required.
	Charter []byte
	// DesignBindings is the governing design's binding elements — mapped onto
	// the governing kind. Required.
	DesignBindings []byte

	// Calibration is the calibration table — mapped onto the calibration kind.
	// Required.
	Calibration []byte
}

// declaration is the fixed mapping of the five review inputs onto the three
// EXISTING contextpack admitted kinds. Order is the pack's read order and is
// preserved verbatim by contextpack; no authoring-trail kind appears because
// none is admissible (DINV-5, inherited from contextpack's closed set). This
// is data over contextpack's grammar, not a change to it.
func declaration() contextpack.Declaration {
	return contextpack.Declaration{
		Stage: {
			{Kind: contextpack.KindSubject, Selector: proposalPath},
			{Kind: contextpack.KindSubject, Selector: verdictPath},
			{Kind: contextpack.KindGoverning, Selector: charterPath},
			{Kind: contextpack.KindGoverning, Selector: designPath},
			{Kind: contextpack.KindCalibration, Selector: calibrationPath},
		},
	}
}

// DeclarationText supplies the canonical mapping to source-less reviewers.
// It is rendered from the same declaration Build uses, preserving input order.
func DeclarationText() string {
	b := fmt.Appendf(nil, "%s:\n", Stage)
	for _, source := range declaration()[Stage] {
		b = fmt.Appendf(b, "  - %s %s\n", source.Kind, source.Selector)
	}
	return string(b)
}

// Build assembles the review context pack for the given inputs. It returns the
// pack bytes — the same deterministic, manifest-headed artifact contextpack
// produces (a fixed manifest, verbatim bodies in declared order, a
// content-addressed digest, NO wall-clock — so two builds at one input state
// are byte-identical). A missing required input is a named error and no
// artifact (fail closed).
//
// It reuses contextpack.Build VERBATIM: the five inputs are materialized into
// a throwaway build root under a temp dir, a Declaration maps each onto an
// existing admitted kind, and contextpack.Build reads them back and assembles
// the pack. contextpack's determinism, manifest, and digest are reused
// wholesale — this package reimplements none of them, and edits contextpack
// not at all. The temp root is always cleaned up.
func Build(in ReviewInputs) ([]byte, error) {
	if in.Subject == "" {
		return nil, fmt.Errorf("review context pack: empty subject")
	}
	// Each of the five inputs is required — the pack is the whole review input
	// surface, and a nil one would silently drop a source. Fail closed, name it.
	for _, req := range []struct {
		name    string
		content []byte
	}{
		{"proposal", in.Proposal},
		{"codegate verdict", in.CodegateVerdict},
		{"charter", in.Charter},
		{"design bindings", in.DesignBindings},
		{"calibration", in.Calibration},
	} {
		if req.content == nil {
			return nil, fmt.Errorf("review context pack: missing required input %q", req.name)
		}
	}

	// A throwaway root the five inputs are written into so contextpack can read
	// them back — the pack is build-fresh and never stored, so the root exists
	// only for this call and is removed after.
	root, err := os.MkdirTemp("", "implreview-pack-")
	if err != nil {
		return nil, fmt.Errorf("review context pack: temp root: %w", err)
	}
	defer func() {
		// Best-effort cleanup of the throwaway root; a failure to remove it
		// does not corrupt the returned pack, so it is logged-by-return only.
		_ = os.RemoveAll(root)
	}()

	for _, src := range []struct {
		relPath string
		content []byte
	}{
		{proposalPath, in.Proposal},
		{verdictPath, in.CodegateVerdict},
		{charterPath, in.Charter},
		{designPath, in.DesignBindings},
		{calibrationPath, in.Calibration},
	} {
		if err := writeSource(root, src.relPath, src.content); err != nil {
			return nil, err
		}
	}

	pack, defects := contextpack.Build(root, Stage, in.Subject, declaration())
	if len(defects) > 0 {
		return nil, fmt.Errorf("review context pack: %s", defects[0].String())
	}
	return pack, nil
}

// writeSource writes one input to root at a repo-relative slash path, creating
// parent dirs — the throwaway-root materializer contextpack.Build reads from.
func writeSource(root, relPath string, content []byte) error {
	p := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("review context pack: mkdir %q: %w", relPath, err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		return fmt.Errorf("review context pack: write %q: %w", relPath, err)
	}
	return nil
}
