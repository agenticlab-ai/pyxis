package contractgate

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

// currency — the point-in-time-validity half of stage-6 readiness
// (prd:contract-gate/FR-2; design contract-gate-v1 §3.9, class `stale`). A
// contract's design plane drifts after its dependents freeze: a live serving
// design may come to serve a requirement that was later superseded/retired, or
// to build on a design that was later superseded. This check fails naming each
// such stale serving design and the offending anchor.
//
// It is a REUSE of the design index's stale-serves / stale-builds-on
// computation, feature-scoped in the verb: it reads the SAME live graph
// techdesigns.LiveGraph produced at resolution (c.Entries — every live and
// superseded design's computed state) via the shared StaleServeAnchors /
// StaleBuildsOnAnchors predicates the index's flag is itself set from. The
// index's flag stays non-fatal for its other readers (gen-index renders it
// advisory); the fatality lives here, in the gate, over the SAME computation —
// never a re-derivation and never a change to the shared logic
// (prd:contract-gate/NR-2). The scope is the feature's own serving designs
// (c.Serving), the ones whose Serves target this feature's requirements; a
// design serving the feature is judged on its whole currency (all its Serves /
// Builds-on), since an out-of-date serving design is not a sound contract input.
func init() {
	Register(Check{
		Name:          "currency",
		Class:         ClassStale,
		SeverityClass: SeverityClassCurrency,
		Run:           checkCurrency,
	})
}

// checkCurrency reports every live serving design of the feature that serves a
// superseded/retired requirement or builds on a superseded design, naming the
// design and each offending anchor. Deterministic: c.Serving is already sorted
// by slug and each design's anchors are read in Serves/Builds-on order, which is
// the order the graph recorded them (DINV-5). Read-only over the resolved
// contract (DINV-1/DINV-3).
func checkCurrency(c *contract) Defects {
	var defects []schema.Defect
	for _, sd := range c.Serving {
		e, ok := c.Entries[sd.Slug]
		if !ok {
			continue // a serving design always has an entry; be defensive
		}
		for _, ref := range techdesigns.StaleServeAnchors(e, c.Entries, c.Env) {
			defects = append(defects, curdef(sd.Slug,
				"live serving design "+sd.Slug+" serves "+ref+", a superseded or retired requirement",
				"the design serves a requirement that is no longer live; supersede the design or re-point its Serves at a live anchor (FR-2)"))
		}
		for _, bo := range techdesigns.StaleBuildsOnAnchors(e, c.Entries) {
			defects = append(defects, curdef(sd.Slug,
				"live serving design "+sd.Slug+" builds on "+bo+", a superseded design",
				"the design builds on a superseded design; supersede it or re-point its Builds-on at the live successor (FR-2)"))
		}
	}
	return defects
}

// curdef builds one currency defect naming the offending serving design file
// (the stale anchor is in the reason).
func curdef(slug, reason, remedy string) schema.Defect {
	return schema.Defect{
		Verb: "contract-gate", File: slug + ".md", Op: "currency",
		Reason: reason, Remedy: remedy,
	}
}
