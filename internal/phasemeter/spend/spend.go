// Package spend is the pipeline-measurement spend-source port: the
// consumer-side interface the recorder reads per-model spend through, and
// one adapter per harness implements — a headless result envelope, a
// session transcript, or OTEL, whichever channel that harness exposes
// (docs/tech-design/pipeline-measurement-v2.md §3.7, S-2). The recorder
// core (this package) imports no adapter package (AP-1, AP-3): each
// harness-specific adapter lives in its own
// internal/adapters/spend/<harness> package and is wired in by a
// caller — the recorder main.
//
// A Source reads its harness's own accounting for a phase bracket into a
// per-model token quartet, or returns a declared Unavailable naming the
// harness and the missing figure — never a fabricated value (DINV-3).
package spend

import (
	"fmt"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

// Source is the spend-source port (design §3.7, S-2). A caller builds one
// Source per measured session (typically by loading one harness's own
// session record once), then:
//
//   - feeds Turns() to phase.Detect to bracket the session into phases;
//   - calls Spend once per resulting phase.Phase to capture that phase's
//     per-model token quartet, or a declared Unavailable.
//
// Turns and Spend read the SAME underlying harness record — never two
// divergent parses — so phase-boundary detection and spend capture never
// drift apart (design §3.8: "rides S-2, not a new seam").
type Source interface {
	// Harness is the harness id this Source reads accounting for (e.g.
	// "claude-code") — stamped onto every record this Source's Spend
	// feeds, so a harness-switching run never blends spend across
	// harnesses (DINV-7, UB-4).
	Harness() string

	// Turns returns the harness's own session record, normalized to the
	// turn sequence phase.Detect brackets into phases. Turns is read-only
	// and safe to call more than once; it always returns the same
	// sequence for the lifetime of this Source.
	Turns() []phase.Turn

	// Spend sums p's own turns' usage into a per-model token quartet. A
	// phase this Source could not capture any usage for — every turn's
	// usage was absent, or p carries no turns at all — returns a nil map
	// and exactly one Unavailable entry naming this Source's harness and
	// the missing figure; it never returns a silent zero (DINV-3).
	//
	// Spend need not defend against its own bugs: Records guards every
	// call's result against a zero-valued or undeclared figure reaching a
	// record regardless (prd:pipeline-measurement ABUSE-1) — but a Source
	// should still never deliberately rely on that backstop.
	Spend(p phase.Phase) (spend map[string]record.ModelSpend, unavailable []record.Unavailable)
}

// Records is the recorder's core measure step: it brackets src's own
// session record into phases (phase.Detect) and, for each phase, captures
// that phase's spend through src, returning one record.Record per phase —
// stamped with runID, featureID, src's own Harness, and the captured
// Spend/Unavailable — in the order phase.Detect found them. Records
// performs no IO of its own (src already read whatever it needed to build
// Turns) and writes nothing; a caller appends the result to a store.
//
// Records never drops a phase — including one phase.Detect found
// incomplete (no close marker arrived) — so an aborted phase still yields
// its own record carrying whatever spend it accrued (UB-3's "never
// dropped", carried through to the record-write path).
func Records(src Source, runID, featureID string) []record.Record {
	harness := src.Harness()
	phases := phase.Detect(src.Turns())
	recs := make([]record.Record, 0, len(phases))
	for _, p := range phases {
		spend, unavailable := src.Spend(p)
		spend, unavailable = guardSilentZero(harness, spend, unavailable)
		recs = append(recs, record.Record{
			SchemaVersion: record.SchemaVersion,
			RunID:         runID,
			FeatureID:     featureID,
			PhaseID:       p.ID,
			Harness:       harness,
			Spend:         spend,
			Unavailable:   unavailable,
		})
	}
	return recs
}

// guardSilentZero enforces DINV-3 against the Source itself, not merely
// against a well-behaved adapter's own discipline (prd:pipeline-measurement
// ABUSE-1: "a flawed adapter... yields a zero that reads as this phase was
// free"). Two ways a Source could let that happen, both closed here:
//
//   - a model's TokenQuartet is the all-zero value — indistinguishable from
//     "never measured" — without Source itself declaring it Unavailable;
//   - spend carries nothing and unavailable declares nothing, so the phase
//     would read as cleanly zero-cost rather than as unmeasured.
//
// Either way, the affected figure is pulled out of spend (never left to be
// presented as measured) and named in unavailable instead — the only
// outcome DINV-3 allows for a figure that cannot be trusted. Models found
// zero-valued are named in sorted order so the result never depends on Go's
// randomized map iteration.
func guardSilentZero(harness string, spend map[string]record.ModelSpend, unavailable []record.Unavailable) (map[string]record.ModelSpend, []record.Unavailable) {
	var zeroed []string
	for model, ms := range spend {
		if ms.TokenQuartet == (record.TokenQuartet{}) {
			zeroed = append(zeroed, model)
		}
	}
	sort.Strings(zeroed)
	for _, model := range zeroed {
		delete(spend, model)
		unavailable = append(unavailable, record.Unavailable{
			Harness:       harness,
			MissingFigure: fmt.Sprintf("per-model spend (%s)", model),
		})
	}
	if len(spend) == 0 {
		spend = nil
	}
	if spend == nil && len(unavailable) == 0 {
		unavailable = []record.Unavailable{{Harness: harness, MissingFigure: "per-model spend"}}
	}
	return spend, unavailable
}
