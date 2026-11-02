// Package band is the impl-review depth-band layer — the review reads its depth
// from the merge lane's OWN risk class, never a second review-side risk model
// (docs/tech-design/impl-review-v1.md §3.6, plan T6; FR-9, FR-10, UB-7, A-8).
//
// What lives HERE (the testable Go core, T6):
//   - Band selection (FR-9, band.go): Select maps a proposal's lane risk class —
//     read from the classifier's Verdict (floor hit · AUTO class · neither) — to
//     skip · standard · deep, deterministically. A skip records `skipped:<class>`.
//     There is NO review-side risk derivation: the only risk input is the lane's
//     Verdict, and the class-data-absent case fail-closes to STANDARD (recorded),
//     never a guessed depth (the PRD §5.4 rule "class-data absent ⇒ standard
//     band, recorded").
//   - The deep-band mandatory context + escalated critic (FR-10, context.go): in
//     the deep band the review runs with the served ABUSE rows + the incident
//     registry as mandatory context (enriching the T3 pack's inputs), and the
//     correctness-critic's calibration escalates so a load-bearing critic finding
//     ships BLOCKING; the standard band omits that extra context (a cost audit).
//   - The dormant second-instance verify (A-8, verify.go): a data-gated flag OFF
//     by default; when on it re-judges a deep-band BLOCKING finding through a
//     SECOND independent subagent (reusing T4's runtime.LensRunner seam) and
//     downgrades one the second instance does not confirm. Built, shipped OFF.
//   - Oversize escalation (UB-7, oversize.go): a diff exceeding the band's honest
//     read budget records a size-escalation routing the proposal HUMAN, and NEVER
//     an approve over a partially-read diff.
//
// This package READS the lane's classification (internal/lane/classifier) — it
// imports it read-only and derives nothing of its own (AP-11: one risk predicate,
// the lane's; the review consumes the class, it does not re-classify). It sets
// the report envelope's Band (report.BandSkip/Standard/Deep); the verdict layer
// (T7) sets the verdict, and the runtime (T4) produces the rounds. Stdlib only,
// per the repo Go rules.
package band

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
)

// Band is one of the three review depths — exactly the report.Band values
// (report.BandSkip/Standard/Deep). The band layer selects it from the lane's
// risk class; the report envelope carries it.
type Band = string

// Decision is the outcome of band selection for one proposal (FR-9): the chosen
// Band, the recorded reason, and — for a skip — the `skipped:<class>` marker the
// report records (so a skip is never a silent cap; §5.4 flow row F1). Every
// field derives from the lane's Verdict alone; there is no review-side risk
// computation here.
type Decision struct {
	// Band is the selected depth (skip/standard/deep).
	Band Band
	// Class is the lane risk class the band was selected from — the floor class
	// hit (e.g. "path:AGENTS.md", "class:governing-doc-edit"), the matched AUTO
	// class name, or "" when the lane emitted neither (the fail-closed standard
	// case).
	Class string
	// SkipMarker is the `skipped:<class>` marker recorded on a skip, and "" for a
	// non-skip band. A skip ALWAYS names its class (no silent caps).
	SkipMarker string
	// Reason is the recorded, human-readable basis for the selection.
	Reason string
}

// skipMarkerPrefix is the marker a skip records ("skipped:<class>") — the
// recorded, harvestable skip outcome (§5.4 flow F1, "skip-band records
// skipped:<class>").
const skipMarkerPrefix = "skipped:"

// SkipClass returns the class token a skip marker names — the inverse of the
// `skipped:<class>` composition, for a harvester/reader that reads the marker
// back. It returns ("", false) for a non-skip marker.
func SkipClass(marker string) (string, bool) {
	if len(marker) <= len(skipMarkerPrefix) || marker[:len(skipMarkerPrefix)] != skipMarkerPrefix {
		return "", false
	}
	return marker[len(skipMarkerPrefix):], true
}

// Select maps a proposal's lane risk class to a review depth (FR-9),
// deterministically, from the lane's Verdict ALONE — the single risk input
// (AP-11). It NEVER computes a second risk model: the only signals it reads are
// the Verdict's Lane, FloorHits, and AutoClass, all produced by the lane's
// classifier. The mapping:
//
//   - A FLOOR hit (Lane==HUMAN with FloorHits) is a hard-gated, high-risk
//     surface → DEEP. The floor is the lane's own "this is dangerous" signal, so
//     the review concentrates its scrutiny there (the deep band's mandatory
//     context + escalated critic, FR-10). The class recorded is the first floor
//     hit (a floor hit lists every hit; the first names the surface).
//   - A trivial AUTO-eligible class (Lane==AUTO with an AutoClass whose name is
//     in the skip set below) → SKIP, recording `skipped:<class>`. An AUTO class
//     the lane deems whole-diff-safe (a mechanical/docs-only perimeter) needs no
//     judgment review; a skip records its class so non-invocation is visible.
//   - Everything else → STANDARD: a HUMAN verdict WITHOUT a floor hit (ambiguous
//     membership, out-of-perimeter, a downgrade, an empty diff — the lane's
//     residual risk that is neither hard-gated nor trivially safe), and an AUTO
//     class NOT in the skip set. This is also the FAIL-CLOSED default: a Verdict
//     the band cannot positively map to skip or deep is STANDARD, recorded
//     (§5.4: "class-data absent ⇒ standard band, recorded") — never a guessed
//     skip (which would silently drop review) nor a guessed deep.
//
// The skip set is DATA on the lane's OWN class names (skipClasses below), read
// from the policy's ratified AUTO classes — not a review-side risk judgment. A
// class name the lane does not carry simply never matches, so the mapping stays
// honest to whatever classes the loaded policy ratifies.
func Select(v classifier.Verdict) Decision {
	switch v.Lane {
	case classifier.Human:
		// A floor hit is the lane's hard-gated high-risk signal → DEEP.
		if len(v.FloorHits) > 0 {
			cls := v.FloorHits[0]
			return Decision{
				Band:   report.BandDeep,
				Class:  cls,
				Reason: fmt.Sprintf("the lane's floor hit a hard-gated surface (%s) — deep-band review", cls),
			}
		}
		// A HUMAN verdict with no floor hit is residual risk that is neither
		// hard-gated nor trivially safe → STANDARD (also the fail-closed default:
		// an unmapped lane outcome reviews at standard depth, recorded).
		return Decision{
			Band:   report.BandStandard,
			Class:  "",
			Reason: standardReason(v),
		}
	case classifier.Auto:
		// A trivial AUTO class → SKIP, recording its class.
		if isSkipClass(v.AutoClass) {
			return Decision{
				Band:       report.BandSkip,
				Class:      v.AutoClass,
				SkipMarker: skipMarkerPrefix + v.AutoClass,
				Reason:     fmt.Sprintf("the lane classed the whole diff as trivial AUTO class %q — skip-band", v.AutoClass),
			}
		}
		// A non-trivial AUTO class still gets the standard four-lens pass.
		return Decision{
			Band:   report.BandStandard,
			Class:  v.AutoClass,
			Reason: fmt.Sprintf("the lane classed the diff AUTO (%s) — standard-band review", autoClassLabel(v.AutoClass)),
		}
	default:
		// A Verdict carrying neither lane value is a malformed input the band
		// cannot read a class from — fail closed to STANDARD, recorded, never a
		// guessed depth (§5.4's class-data-absent rule, generalized).
		return Decision{
			Band:   report.BandStandard,
			Class:  "",
			Reason: "the lane verdict carries no readable lane; fail-closed standard-band (§5.4)",
		}
	}
}

// standardReason names WHY a floor-clear HUMAN verdict maps to standard — the
// lane's own recorded reason threaded through, so the band decision is
// traceable to the lane classification it read (never a review-side re-judgment).
func standardReason(v classifier.Verdict) string {
	if v.Reason != "" {
		return "the lane routed HUMAN without a floor hit (" + v.Reason + ") — standard-band review"
	}
	return "the lane routed HUMAN without a floor hit — standard-band review"
}

// autoClassLabel renders an AUTO class name for a reason, tolerating an empty
// one (a well-formed AUTO verdict always names its class, but the reason stays
// legible if it does not).
func autoClassLabel(name string) string {
	if name == "" {
		return "class unnamed"
	}
	return "class " + name
}
