package contractgate

import (
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

// inverse-coverage — the feature-level "complete enough to plan" bar
// (prd:contract-gate/FR-3; design contract-gate-v1 §3.9, class `incomplete`):
// every LIVE Must requirement of the feature must be served by ≥1 live design.
// It fails naming each unserved Must.
//
// It is a REUSE of GenCoverage's requirement-universe↔live-serves join
// (computeCoverage), feature-scoped in the verb: techdesigns.FeatureUnserved
// runs the SAME join the tree-wide coverage view runs, sliced to this feature's
// `prd:<feature>` family, over the SAME live serves-inverse LiveGraph produced
// at resolution (c.Inverse) — never a fork of the join (prd:contract-gate/NR-2).
// The join decides "is this anchor served"; the verb applies the Must filter
// (T1's per-unit `priority` on the source map — verb-side unit selection, §3.9).
//
// UB-5: a feature with a PRD but no live serving design has an empty inverse, so
// every live unit is unserved and every Must reds `incomplete` — never treated
// as vacuously clean.
func init() {
	Register(Check{
		Name:          "inverse-coverage",
		Class:         ClassIncomplete,
		SeverityClass: SeverityClassCoverage,
		Run:           checkInverseCoverage,
	})
}

// checkInverseCoverage reports every live Must requirement of the feature that
// no live design serves. The unserved determination is the shared coverage join
// (FeatureUnserved over c.Inverse); the Must selection reads T1's priority field
// off the resolved source map. Sorted output (SortIDs) keeps the verdict
// deterministic (DINV-5); read-only over the resolved contract (DINV-1/DINV-3).
func checkInverseCoverage(c *contract) Defects {
	// The shared join: the feature's live requirement anchors no live design
	// serves (prd:<feature>/<unit> tokens), computed by the SAME computeCoverage
	// the tree-wide view uses. Empty inverse ⇒ every live unit here (UB-5).
	unserved := map[string]bool{}
	for _, anchor := range techdesigns.FeatureUnserved(c.Env, c.Inverse, c.Feature) {
		unserved[strings.TrimPrefix(anchor, "prd:"+c.Feature+"/")] = true
	}

	// The Must selection (verb-side): a live requirement unit whose priority is
	// Must. A Must that the join marked unserved reds incomplete, named.
	var ids []string
	for id := range c.Source.Units {
		ids = append(ids, id)
	}
	schema.SortIDs(ids)

	var defects []schema.Defect
	for _, id := range ids {
		u := c.Source.Units[id]
		if u.Status != "live" || u.Priority != "Must" {
			continue
		}
		if unserved[id] {
			defects = append(defects, covdef(c,
				"live Must requirement "+id+" is served by no live design",
				"a Must must be served by ≥1 live design before planning; add a serving design or re-scope "+id+" (FR-3)"))
		}
	}
	return defects
}

// covdef builds one inverse-coverage defect naming the PRD source map as the
// offending face (the unserved Must id is in the reason).
func covdef(c *contract, reason, remedy string) schema.Defect {
	return schema.Defect{
		Verb: "contract-gate", File: c.SourcePath, Op: "inverse-coverage",
		Reason: reason, Remedy: remedy,
	}
}
