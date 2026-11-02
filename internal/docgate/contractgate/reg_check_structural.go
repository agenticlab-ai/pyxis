package contractgate

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// structural-completeness — the field-presence half of stage-6 readiness
// (prd:contract-gate/FR-5; design contract-gate-v1 §3.9, class `incomplete`).
// It reads ONLY the resolved PRD source map (the T1-enriched machine face:
// has_fit_criterion, verify_method) and the resolved DoD family — never
// re-parsing effective.md (DINV-3). For every LIVE requirement unit that should
// carry them (FR/NR/UB/NFR), it fails naming each that lacks a fit criterion or
// a declared verification method; it also fails if the definition of done is
// absent. This is field-PRESENCE, not adequacy — a bogus fit criterion or a
// verification method naming no real proof passes here; the adequacy judgment
// is the LLM follow-up's (prd:contract-gate/WR-1, ABUSE-1), so `clean` here is
// mechanically clean only (DINV-4).
//
// The §8 "contract readiness" section (FR-5's third clause) is a prose+checklist
// PRD section carrying no requirement rows, so it projects NO unit onto the
// source map and no unit's source_location references it — it is not decidable
// from the current machine faces. DINV-3 (single parser; the gate never
// re-parses effective.md for substance) governs on conflict, and §3.6 enriches
// only per-requirement fields, exposing no §8 signal. Rather than open the
// exact drift surface the reuse decision refused (a second parser reading the
// PRD body), this check verifies the machine-face-resolvable structural-
// readiness signals — the per-requirement fields and the DoD family's presence
// (the §5.8 projection FR-5 also requires) — and leaves §8-section presence to a
// future additive fold enrichment (a §8-present marker on the machine face),
// which would light up here as a single-file extension with no shared-line edit.
func init() {
	Register(Check{
		Name:          "structural-completeness",
		Class:         ClassIncomplete,
		SeverityClass: SeverityClassStructural,
		Run:           checkStructuralCompleteness,
	})
}

// structuralFamilies is the closed set of requirement prefixes that must carry
// a fit criterion + a verification method (FR-5). NG/WR/EVT/ABUSE/EVAL and the
// non-requirement families (G/J/M/Q/R/DOD) are out of scope — only the four
// requirement tables carry the two fields the readiness check reads.
var structuralFamilies = map[string]bool{
	"FR": true, "NR": true, "UB": true, "NFR": true,
}

// checkStructuralCompleteness reports every live requirement unit missing a fit
// criterion or a verification method, plus a missing definition of done. Sorted
// output (SortIDs / the fixed DoD defect) keeps the verdict deterministic
// (DINV-5). Read-only over the resolved contract (DINV-1).
func checkStructuralCompleteness(c *contract) Defects {
	var defects []schema.Defect

	// Per-requirement field presence, over the LIVE requirement units only — a
	// superseded/retired unit is not part of the live contract, so its missing
	// fields never bounce (liveness is read from the fold's status, never
	// re-derived; DINV-3).
	var ids []string
	for id := range c.Source.Units {
		ids = append(ids, id)
	}
	schema.SortIDs(ids)
	for _, id := range ids {
		u := c.Source.Units[id]
		_, prefix, ok := schema.ParseID(id)
		if !ok || !structuralFamilies[prefix] {
			continue
		}
		if u.Status != "live" {
			continue
		}
		if !u.HasFitCriterion {
			defects = append(defects, sdef(c,
				"requirement "+id+" carries no fit criterion",
				"give "+id+" a non-empty Fit criterion cell — a requirement with no fit criterion is not plannable (FR-5)"))
		}
		if u.VerifyMethod == "" {
			defects = append(defects, sdef(c,
				"requirement "+id+" declares no verification method",
				"give "+id+" a non-empty Verify cell — a requirement with no stated way to verify it is not plannable (FR-5)"))
		}
	}

	// The definition of done must be present — the §5.8 DoD family projects the
	// DOD units resolve.go collected into c.DoD (DINV-3: read from the machine
	// face, not the PRD body).
	if len(c.DoD) == 0 {
		defects = append(defects, sdef(c,
			"the contract's definition of done is absent — no DOD units in the source map",
			"the PRD must carry a §5.8 definition of done; re-fold after adding it (FR-5)"))
	}

	return defects
}

// sdef builds one structural-completeness defect naming the source map as the
// offending face (the requirement id / DoD gap is in the reason).
func sdef(c *contract, reason, remedy string) schema.Defect {
	return schema.Defect{
		Verb: "contract-gate", File: c.SourcePath, Op: "structural-completeness",
		Reason: reason, Remedy: remedy,
	}
}
