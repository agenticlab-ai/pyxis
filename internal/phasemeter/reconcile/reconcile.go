// Package reconcile is the pipeline-measurement reconciler
// (docs/tech-design/pipeline-measurement-v2.md §3.9, DINV-4): it
// cross-checks a phase's recorded per-model figures — the sum of a
// harness's own per-turn usage within the phase bracket, exactly what
// internal/phasemeter/spend.Source.Spend already computed — against that
// same harness's own authoritative total for the same span, where one is
// reported, and records how tightly each figure reconciles.
//
// The tolerance is tiered, never a single pass/fail (DINV-4):
// record.TierExact when the harness reports a byte-exact per-model figure
// that matches; record.TierHarnessPrecision when only a coarser or
// rounded figure exists and recorded falls within that harness-stated
// precision (never a tolerance this package invents); record.TierUnavailable
// when no total was reported at all, or when recorded does not even
// reconcile at the harness's own coarse precision — a genuine mismatch is
// non-reconcilable, not a downgraded-but-still-trusted figure. Every
// recorded figure gets exactly one of these three tiers, never a fourth
// and never the unset zero value.
package reconcile

import "github.com/agenticlab-ai/pyxis/internal/phasemeter/record"

// Total is one model's authoritative rollup for a phase span, as a
// harness reports it through a channel independent of the per-turn sum
// already summed into a phase's recorded per-model figure (design §3.9,
// D-4) — e.g. a headless result envelope's own usage summary, distinct
// from the transcript entries an adapter sums turn by turn. It is the
// reconciler's cross-check target for that model.
type Total struct {
	// Tokens is the harness's own reported figure for this model's span.
	Tokens record.TokenQuartet

	// Exact is true when Tokens is the harness's own byte-exact per-model
	// figure — the only case Tier can return record.TierExact from. false
	// means Tokens is a coarser or rounded figure: Precision names the
	// harness's own stated tolerance, and at most record.TierHarnessPrecision
	// is reachable — Tier never upgrades a coarse total to TierExact merely
	// because the numbers happen to line up (design §3.9: "the tolerance
	// is the harness's own precision — no invented digits").
	Exact bool

	// Precision is the harness's own stated +/- tolerance, applied to each
	// TokenQuartet field independently, that Tokens is accurate to when
	// Exact is false. It is read only when Exact is false; it is never a
	// tolerance Tier invents on the harness's behalf.
	Precision int64
}

// Tier reconciles one model's recorded figure — a phase's per-turn usage,
// already summed by a spend.Source — against total, the same harness's
// own authoritative rollup for that model and span, where found reports
// one was reported at all (design §3.9). It returns exactly one of
// record's three reconciliation tiers (DINV-4) — never a fourth, and
// never the unset zero value:
//
//   - record.TierExact — total.Exact, and recorded equals total.Tokens
//     field for field.
//   - record.TierHarnessPrecision — a total was found but does not
//     qualify for TierExact, and recorded falls within total's own stated
//     Precision.
//   - record.TierUnavailable — found is false (no total was reported for
//     this model/span at all), or recorded falls outside total's own
//     Exact/Precision bound (a genuine mismatch is non-reconcilable, never
//     a coarse match).
func Tier(recorded record.TokenQuartet, total Total, found bool) record.ReconciliationTier {
	if !found {
		return record.TierUnavailable
	}
	if total.Exact {
		if recorded == total.Tokens {
			return record.TierExact
		}
		return record.TierUnavailable
	}
	if withinPrecision(recorded, total.Tokens, total.Precision) {
		return record.TierHarnessPrecision
	}
	return record.TierUnavailable
}

// withinPrecision reports whether recorded and total agree within unit on
// every TokenQuartet field independently.
func withinPrecision(recorded, total record.TokenQuartet, unit int64) bool {
	return absDiff(recorded.Input, total.Input) <= unit &&
		absDiff(recorded.Output, total.Output) <= unit &&
		absDiff(recorded.CacheCreation, total.CacheCreation) <= unit &&
		absDiff(recorded.CacheRead, total.CacheRead) <= unit
}

func absDiff(a, b int64) int64 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}

// Reconcile returns recorded with every entry's ReconciliationTier set by
// cross-checking it (via Tier) against totals — the harness's own
// authoritative rollup for the same phase, keyed by model, where one is
// reported (design §3.9). recorded is never mutated; Reconcile returns a
// new map (or recorded itself, unchanged, when it is empty). A model
// present in recorded but absent from totals still gets a tier —
// record.TierUnavailable, DINV-4's correctly-recorded outcome for
// "nothing to check this figure against," never the unset zero value.
func Reconcile(recorded map[string]record.ModelSpend, totals map[string]Total) map[string]record.ModelSpend {
	if len(recorded) == 0 {
		return recorded
	}
	out := make(map[string]record.ModelSpend, len(recorded))
	for model, ms := range recorded {
		total, found := totals[model]
		ms.ReconciliationTier = Tier(ms.TokenQuartet, total, found)
		out[model] = ms
	}
	return out
}

// Join reconciles every record in records against totals — one run's
// worth of per-phase authoritative rollups, keyed first by phase id and
// then by model — returning a new slice; records itself is never mutated.
// A record whose PhaseID has no entry in totals still passes through
// Reconcile, so every one of its Spend entries is tiered
// record.TierUnavailable rather than left at the unset zero value,
// mirroring efficiency.Join's join-by-phase-id shape (design §3.9/§3.10
// share the run/phase id join key).
func Join(records []record.Record, totals map[string]map[string]Total) []record.Record {
	out := make([]record.Record, len(records))
	for i, rec := range records {
		rec.Spend = Reconcile(rec.Spend, totals[rec.PhaseID])
		out[i] = rec
	}
	return out
}
