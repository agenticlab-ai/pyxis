// The contract-gate planning-entry precondition (plan-new FR-27; contract-gate
// design §3.10). Before plan-new drafts, freezes, or projects a plan, the
// planner must gate the feature's assembled contract through the mechanical
// stage-6 gate and refuse on any non-clean verdict — this is that gate's
// mechanical, fail-closed realization, invoked as `plangate precheck <feature>`.
//
// It is a thin, read-only delegation to the docgate contract-gate verb
// (contractgate.Run) — the single predicate, single parser (AP-11): the
// verdict and its per-check defects come from the one gate, never a second
// implementation here. plangate makes ZERO mutation on this path; the gate is
// read-only by construction (contract-gate DINV-1). A non-clean verdict yields
// the gate's own named defects (each check id / offender / unblock), surfaced
// as plangate defects so the bounce carries the same defect report the design
// requires (§3.8, §3.10). A clean verdict yields no defect and planning proceeds.
package plan

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/contractgate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Precheck runs the mechanical stage-6 contract gate over feature's assembled
// contract (read-only) and returns the gate's blocking defects as plangate
// defects — empty iff the verdict is non-blocking (clean, or advisory-only under
// the active severity split). A non-empty result is the planning-entry bounce:
// the caller refuses to draft, freeze, or project, surfacing the report (FR-27).
//
// It uses the planning-entry trigger (contractgate.Run's default), under which
// every contract-scope class is blocking (design §3.10) — the hard-gate posture
// planning entry requires, distinct from the continuous surface's advisory
// coverage. A fail-closed edge (missing / unmerged / malformed contract) is
// itself blocking and surfaces here, so a broken contract never slips the gate.
func Precheck(root, feature string) []schema.Defect {
	v := contractgate.Run(root, feature)
	if !v.Blocked() {
		return nil
	}
	return precheckDefects(feature, v)
}

// precheckDefects renders a blocking verdict as plangate defects: one framing
// defect naming the bounce and the feature, then every named offender the gate
// found, re-attributed to plangate's precheck op so a reader sees the same
// check id / offender / unblock the gate emitted. Deterministic: the verdict's
// check results are already stably ordered (contract-gate DINV-5).
func precheckDefects(feature string, v contractgate.Verdict) []schema.Defect {
	reason := v.Reason
	if reason == "" {
		reason = "contract-gate verdict is " + string(v.Class)
	}
	out := []schema.Defect{def(
		"docs/prds/"+feature,
		"precheck",
		"contract-gate ["+string(v.Trigger)+"] reds the feature contract ("+string(v.Class)+"): "+reason,
		"resolve every named contract defect below and re-run `docgate contract-gate "+feature+"` until it is clean — a plan may not draft, freeze, or project over a non-clean contract (FR-27)",
	)}
	for _, r := range v.CheckResults {
		if r.Passed {
			continue
		}
		// An advisory failure under the planning-entry trigger cannot happen
		// (every class blocks there), but guard the general case: only blocking
		// defects gate planning; advisory ones are surfaced by the framing line's
		// verdict reason, not re-raised as plangate defects.
		if r.Severity == contractgate.SeverityAdvisory {
			continue
		}
		for _, d := range r.Defects {
			out = append(out, schema.Defect{
				Verb:   "plangate",
				File:   d.File,
				Op:     "precheck/" + r.Check,
				Reason: d.Reason,
				Remedy: d.Remedy,
			})
		}
	}
	return out
}
