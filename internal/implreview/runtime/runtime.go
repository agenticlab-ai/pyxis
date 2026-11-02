// Package runtime is the impl-review reviewer runtime — the orchestration the
// /impl-review skill drives (docs/tech-design/impl-review-v1.md §3.4, plan T4).
// It runs the four fresh-context lenses over ONE shared review context pack,
// each lens blind to the others' findings, screens their candidates
// adversarially, and assembles the survivors into a report.Report round
// carrying the fresh-instance provenance.
//
// What lives HERE (the testable Go core, T4):
//   - the LensRunner SEAM — the interface the real runtime satisfies by
//     spawning a sibling subagent per lens, and a fake satisfies in a hermetic
//     test; Review accepts the interface and returns concrete report types.
//   - the fresh-instance provenance (provenance.go): a reviewer-root-derived
//     context_id and CheckFreshInstance, the would-fail-if-inherited leg (A-6).
//   - the orchestration: Review runs the four lenses, gives each ONLY
//     (pack, its own instruction) — finding-blind (DINV-5/FR-3) — applies the
//     adversarial screen (kills a plausible-but-wrong candidate before it
//     ships), and assembles survivors into one round with provenance/context_id.
//
// What lives in the SKILL guide, not here (.pyxis/authoring/impl-review.md):
//   - the fact that a lens "Run" is a REAL sibling-child subagent spawn — Go
//     never spawns a subagent; the LensRunner does at runtime (this package's
//     tests inject a fake). The Go core models the seam and the boundaries.
//   - the sibling-vs-child SESSION topology the host enforces when it invokes
//     the skill (Go asserts the provenance consequence via CheckFreshInstance;
//     the skill guide states the spawn rule the host obeys).
//
// What lives in T5, not here: the four lenses' JUDGMENT content, the
// calibration table, and the mechanical-floor exclusion. This package
// references the four lenses by the report.Lens enum only and takes each lens's
// instruction as an opaque input — T5 fills the instructions later.
//
// Stdlib only, per the repo Go rules.
package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// ErrNoInstruction is returned when a lens has no instruction — the runtime
// cannot run a lens with nothing to evaluate under, and fails closed rather
// than run an empty lens.
var ErrNoInstruction = errors.New("impl-review runtime: lens has no instruction")

// ErrEmptyPack is returned when the shared review context pack is empty — every
// lens reads the pack, so an empty one would run four lenses over nothing.
var ErrEmptyPack = errors.New("impl-review runtime: empty review context pack")

// Lenses is the fixed four-lens order the runtime runs, referenced by the
// report.Lens enum (§3.1). T4 owns this STRUCTURE — the four fresh-context
// lenses and their identity; T5 fills each lens's judgment instruction. The
// order is fixed for determinism (a run evaluates the lenses in this order),
// and it is exactly the four enum values with no duplicate and no omission.
var Lenses = [4]string{
	report.LensSpecFidelity,
	report.LensDesignConformance,
	report.LensSecurityAbuse,
	report.LensCorrectnessCritic,
}

// LensRunner is THE seam (§3.4, the four-subagent spawn). Its real
// implementation spawns a fresh-context sibling-child subagent for one lens,
// hands it the shared pack plus that lens's own instruction, and returns the
// lens's candidate findings. A hermetic test injects a fake — no network, no
// real subagent.
//
// The signature is the finding-blindness boundary made structural (DINV-5/FR-3):
// Run receives ONLY (lens, pack, instruction). It has NO parameter through
// which another lens's findings could enter, so a lens is finding-blind by
// construction — Review never passes any lens's output to another lens's Run.
// The pack is the one shared read-only input; the instruction is this lens's
// own; the returned findings are this lens's candidates, pre-screen.
type LensRunner interface {
	// Run evaluates one lens over the shared pack under the lens's own
	// instruction and returns its candidate findings. ctx carries cancellation.
	// pack is the identical shared pack every lens reads. instruction is this
	// lens's own instruction (T5 content). The returned findings are candidates
	// the adversarial screen filters before assembly.
	Run(ctx context.Context, lens string, pack []byte, instruction string) ([]report.Finding, error)
}

// Screener is the adversarial survival screen seam (§3.4, "a screening pass
// filters survivors"): it decides whether one candidate finding survives to the
// payload, killing a plausible-but-wrong candidate before it ships (§3.1's
// candidates→killed→findings EVT counts). T4 owns the screen's STRUCTURE — a
// pass that can kill a candidate before assembly, driven by an injectable
// Screener; the DefaultScreener below is the structural default, and T5's
// judgment content refines what "plausible-but-wrong" means. Review invokes the
// screen per candidate after the lenses run, so a killed candidate never
// reaches a round.
type Screener interface {
	// Survives reports whether the candidate finding survives the adversarial
	// screen. A false return KILLS the candidate — it is counted in the round's
	// Killed total and never assembled into the round's finding set.
	Survives(lens string, candidate report.Finding) bool
}

// ScreenerFunc adapts a plain function to Screener — the ergonomic form a test
// or a caller uses to seed a screen without declaring a whole type.
type ScreenerFunc func(lens string, candidate report.Finding) bool

// Survives implements Screener.
func (f ScreenerFunc) Survives(lens string, candidate report.Finding) bool {
	return f(lens, candidate)
}

// DefaultScreener is the structural adversarial screen T4 ships (§3.4). It kills
// a candidate that is not a well-formed, self-supporting finding — a
// plausible-looking candidate missing the evidence a real finding must carry:
//
//   - no message (a claim with no statement), or
//   - no cal_row citation (a severity with no auditable basis — the payload
//     would fail report.Validate's DINV-3 anyway; the screen kills it BEFORE
//     assembly rather than emitting an invalid round).
//
// This is the T4 structural floor of the screen: it removes a candidate that
// could not survive the payload's own integrity rules. T5 layers the lenses'
// judgment content (the "plausible-but-WRONG" defect-class discrimination) on
// top by supplying a richer Screener; the seam is the same.
type DefaultScreener struct{}

// Survives implements Screener — the structural default described on
// DefaultScreener.
func (DefaultScreener) Survives(_ string, candidate report.Finding) bool {
	if candidate.Message == "" {
		return false
	}
	if candidate.CalRow == "" {
		return false
	}
	return true
}

// ReviewRequest is the input to one round of Review. It carries the sibling-vs-
// child session identities the provenance check reads, the shared pack every
// lens reads, the reviewed revision this round evaluates (DINV-7), the per-lens
// instructions (T5 content, opaque here), the round's model+versions
// provenance, and the LensRunner + Screener seams.
type ReviewRequest struct {
	// ReviewerRoot is the reviewer session's root identity — a fresh top-level
	// SIBLING of the implementing worker (§3.4). The round's context_id derives
	// from it (provenance.go).
	ReviewerRoot string
	// ImplementerRoot is the implementing session's root identity. Review FAILS
	// (ErrInheritedContext) if the reviewer context equals or derives from it —
	// the would-fail-if-inherited leg (A-6, DINV-5).
	ImplementerRoot string

	// Pack is the one shared read-only review context pack (from
	// internal/implreview/pack). Every lens reads THIS identical pack — the cost
	// move (NFR-1) and the shared-input-not-shared-judgment boundary (DINV-5).
	Pack []byte

	// ReviewedRevision is the head sha this round evaluates (DINV-7) — recorded
	// on the assembled round; required.
	ReviewedRevision string

	// Instructions maps each of the four lenses to its own instruction (T5
	// content). Every lens in Lenses must have a non-empty instruction; a lens
	// receives ONLY its own instruction (blindness). Keyed by the report.Lens
	// enum value.
	Instructions map[string]string

	// Provenance is the round's model + prompts version + calibration version
	// (§3.1). The context_id is derived from ReviewerRoot and paired with this.
	Provenance report.Provenance

	// Runner is the LensRunner seam (real: sibling-child subagents; test: a
	// fake). Required.
	Runner LensRunner
	// Screener is the adversarial survival screen. Optional — nil uses
	// DefaultScreener.
	Screener Screener
}

// RoundResult is what one Review call produces: the assembled round (survivors,
// counts, provenance, context_id) and the killed candidates the screen removed,
// surfaced for the debugging surface (§3.8, "why did the reviewer miss X" reads
// candidates-vs-killed). The round is ready to append to a report.Report.
type RoundResult struct {
	// Round is the assembled report.Round — survivors as its finding set, the
	// EVT counts (findings/candidates/killed), the reviewed revision, and the
	// fresh-instance context_id + provenance.
	Round report.Round
	// Killed is the candidates the adversarial screen removed, in lens order —
	// the killed set the counts summarize, kept for observability.
	Killed []report.Finding
}

// Review runs one round of the reviewer over the shared pack (§3.4): it checks
// the fresh-instance provenance, runs each of the four lenses via the
// LensRunner giving each ONLY (pack, its own instruction) — blind to the other
// lenses' findings — screens every candidate adversarially, and assembles the
// survivors into a report.Round carrying the round's context_id and provenance.
//
// It does NOT spawn real subagents — that is the LensRunner's job at runtime;
// Review orchestrates the seam. It also does NOT set the report envelope's
// verdict or band — that is T7/T6; Review produces the round the report carries.
//
// The blindness is structural: Review calls Runner.Run four times, once per
// lens, and passes each call ONLY that lens's instruction and the shared pack —
// no lens's returned findings are ever passed into another lens's Run. The
// screen runs AFTER all four lenses, over their combined candidates, so a
// plausible-but-wrong candidate is killed before it is assembled.
func Review(ctx context.Context, req ReviewRequest) (*RoundResult, error) {
	// Fail-closed provenance check FIRST: a run whose reviewer context is
	// inherited from the implementer is not independent, so it never runs the
	// lenses (A-6, DINV-5).
	if err := CheckFreshInstance(req.ReviewerRoot, req.ImplementerRoot); err != nil {
		return nil, err
	}
	if len(req.Pack) == 0 {
		return nil, fmt.Errorf("%w", ErrEmptyPack)
	}
	if req.ReviewedRevision == "" {
		return nil, fmt.Errorf("impl-review runtime: missing reviewed revision (DINV-7)")
	}
	if req.Runner == nil {
		return nil, fmt.Errorf("impl-review runtime: nil LensRunner")
	}
	// Every lens must have a non-empty instruction — a lens run over no
	// instruction is meaningless, so fail closed before spawning anything.
	for _, lens := range Lenses {
		if req.Instructions[lens] == "" {
			return nil, fmt.Errorf("%w: %s", ErrNoInstruction, lens)
		}
	}

	// The fresh-instance context_id for this round, derived from the reviewer
	// root (provenance.go). Every lens subagent's context derives from this
	// reviewer root; the round records it.
	contextID, err := ContextID(req.ReviewerRoot)
	if err != nil {
		return nil, err
	}

	screener := req.Screener
	if screener == nil {
		screener = DefaultScreener{}
	}

	var (
		survivors  []report.Finding
		killed     []report.Finding
		candidates int
	)

	// Run each lens in the fixed order. Each Run receives ONLY the shared pack
	// and THIS lens's own instruction — never another lens's findings
	// (blindness, DINV-5/FR-3). The screen runs per candidate as they come back;
	// a killed candidate is counted and dropped, a survivor is assembled.
	for _, lens := range Lenses {
		instruction := req.Instructions[lens]
		lensCandidates, err := req.Runner.Run(ctx, lens, req.Pack, instruction)
		if err != nil {
			return nil, fmt.Errorf("impl-review runtime: lens %s: %w", lens, err)
		}
		for _, c := range lensCandidates {
			candidates++
			// A lens must attribute its own findings to itself — the runtime
			// stamps the lens so a candidate cannot claim another lens (the Run
			// contract owns the lens; the candidate does not override it).
			c.Lens = lens
			if screener.Survives(lens, c) {
				survivors = append(survivors, c)
			} else {
				killed = append(killed, c)
			}
		}
	}

	round := report.Round{
		ReviewedRevision: req.ReviewedRevision,
		ContextID:        contextID,
		Provenance:       req.Provenance,
		Findings:         survivors,
		FindingsCount:    len(survivors),
		Candidates:       candidates,
		Killed:           len(killed),
	}

	return &RoundResult{Round: round, Killed: killed}, nil
}
