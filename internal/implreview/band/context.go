package band

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// The deep-band mandatory context + the escalated critic (FR-10).
//
// In the DEEP band the lenses run with two extra, mandatory context sources —
// the served PRD's ABUSE rows and the incident registry — and the correctness-
// critic's calibration escalates so a load-bearing critic finding ships
// BLOCKING. The STANDARD band omits both (a cost audit: the deep-band cost is
// paid only where the lane's risk lives). The SKIP band runs no lenses, so it
// carries neither.
//
// This file provides:
//   - DeepContext: the two mandatory-context sources the deep band serves.
//   - EnrichInputs: folds that context into the T3 pack.ReviewInputs (the deep
//     band's "run the lenses with … as mandatory context") — a pure function of
//     (base inputs, band, deep context), enriching only in the deep band. It
//     reuses the pack's EXISTING governing-kind input channel (DesignBindings)
//     rather than adding a pack kind (T3 owns the pack; T6 does not edit it —
//     AP-11, reuse not rebuild).
//   - EscalateCritic: the band-aware report.CalLookup that escalates the
//     correctness-critic's advisory row to blocking in the deep band, so a
//     load-bearing critic finding the critic ships at the critic advisory row
//     validates as BLOCKING under report.Validate — "escalate the critic's
//     calibration" as a data-parameterized dial, not a code re-severity.

// deepContextHeader introduces the deep-band mandatory context appended into the
// governing input, so a lens reads it as clearly-delimited context (never
// mistaken for the design's own binding elements). It is a stable literal so the
// pack stays deterministic (the pack digest is content-addressed; §3.4).
const deepContextHeader = "\n\n=== DEEP-BAND MANDATORY CONTEXT (FR-10) ===\n"

// abuseRowsHeading / incidentRegistryHeading delimit the two mandatory sources
// within the appended context — legible, fixed markers a lens (and a test)
// keys on.
const (
	abuseRowsHeading        = "--- served PRD ABUSE rows ---\n"
	incidentRegistryHeading = "--- incident registry ---\n"
)

// DeepContext is the two mandatory-context sources the deep band serves to the
// lenses (FR-10): the served PRD's ABUSE rows and the incident registry. Both
// are carried as bytes the caller supplies (the ABUSE rows projected from the
// served PRD, the registry read from docs/observability/incidents.md) — this
// package composes them into the pack, it does not fetch them (the fetch is the
// skill's job; the composition and the band gate are here).
type DeepContext struct {
	// AbuseRows is the served PRD's ABUSE-row text — the risk cases the
	// security-abuse lens re-checks against the diff in the deep band (FR-10,
	// the CAL-8 "served abuse rows" the calibration row cites).
	AbuseRows []byte
	// IncidentRegistry is the incident registry text
	// (docs/observability/incidents.md) — the paid-for lessons the deep band
	// carries as mandatory context so a lens catches a known-broken pattern.
	IncidentRegistry []byte
}

// Empty reports whether the deep context carries neither source — a deep-band
// review with an empty DeepContext is a misconfiguration (the mandatory context
// is, by name, mandatory), which EnrichInputs refuses (fail closed) rather than
// silently running a deep band without its context.
func (d DeepContext) Empty() bool {
	return len(d.AbuseRows) == 0 && len(d.IncidentRegistry) == 0
}

// EnrichInputs folds the deep-band mandatory context into the T3 pack inputs for
// the given band (FR-10). It is a pure function: it returns a NEW ReviewInputs
// (the base is not mutated), enriched ONLY when band==deep. For standard and
// skip it returns the base inputs verbatim — the standard band's "extra context
// absent" the cost audit checks.
//
// The enrichment appends the two mandatory sources to the pack's GOVERNING input
// (DesignBindings), under a clearly-delimited header, so the deep-band context
// travels the pack's existing governing channel with NO new pack kind and NO
// edit to internal/implreview/pack or internal/docgate/contextpack (T3/AP-11:
// reuse the admitted governing kind, do not add one). The lenses read the whole
// governing region; the delimiter tells a lens (and a reader) which part is the
// design's own bindings and which is the deep-band mandatory context.
//
// A deep band with an EMPTY DeepContext is refused (ErrMissingDeepContext): the
// context is mandatory, so a deep review without it is a fail-closed error, not
// a silent standard-depth run.
func EnrichInputs(base pack.ReviewInputs, band Band, dc DeepContext) (pack.ReviewInputs, error) {
	if band != report.BandDeep {
		// Standard / skip: no extra context — the base inputs unchanged. This is
		// the observable the cost audit reads (the deep-only context is absent).
		return base, nil
	}
	if dc.Empty() {
		return pack.ReviewInputs{}, fmt.Errorf("%w", ErrMissingDeepContext)
	}

	out := base // struct copy — the byte slices are shared until we replace the one we append to.

	var b strings.Builder
	b.Write(base.DesignBindings)
	b.WriteString(deepContextHeader)
	if len(dc.AbuseRows) > 0 {
		b.WriteString(abuseRowsHeading)
		b.Write(dc.AbuseRows)
		b.WriteString("\n")
	}
	if len(dc.IncidentRegistry) > 0 {
		b.WriteString(incidentRegistryHeading)
		b.Write(dc.IncidentRegistry)
		b.WriteString("\n")
	}
	out.DesignBindings = []byte(b.String())
	return out, nil
}

// HasDeepContext reports whether an enriched governing input carries the
// deep-band mandatory-context header — the structural predicate the cost audit
// uses to assert the deep band carried the extra context and the standard band
// did not. It reads the pack's governing input bytes (DesignBindings) and checks
// for the delimiter EnrichInputs writes.
func HasDeepContext(in pack.ReviewInputs) bool {
	return strings.Contains(string(in.DesignBindings), strings.TrimSpace(deepContextHeader))
}

// EscalateCritic returns a report.CalLookup that escalates the correctness-
// critic's calibration for the given band (FR-10: "escalate the critic's
// calibration so a load-bearing critic finding ships blocking"). It wraps the
// base calibration lookup (the shipped calibration.Table, or any report.CalLookup)
// and, ONLY in the deep band, maps the critic's ADVISORY row to blocking — so a
// load-bearing critic finding the critic ships citing that row validates as
// BLOCKING under report.Validate. For every other band, and every non-critic /
// non-advisory row, it defers to the base lookup unchanged.
//
// This is the data-shaped realization of the escalation: the dial (the
// calibration table) is the same single source of truth (AP-11, one calibration
// home); the band parameterizes how the critic's advisory row resolves. It is
// NOT a per-finding severity rewrite in code — the finding still cites its row;
// the band-aware lookup is what report.Validate consults, so the escalated
// severity is auditable and reproducible.
//
// criticAdvisoryRow is the calibration row id of the correctness-critic's
// advisory tier (CAL-12 in the shipped table) — the row a load-bearing critic
// finding escalates FROM. It is passed in (not hard-coded) so a calibration
// re-numbering is a data change the caller threads, never a code edit here.
func EscalateCritic(base report.CalLookup, band Band, criticAdvisoryRow string) report.CalLookup {
	return report.CalLookupFunc(func(calRow string) (string, bool) {
		if band == report.BandDeep && calRow == criticAdvisoryRow {
			// In the deep band the critic's advisory row escalates to blocking:
			// a load-bearing critic finding citing it ships blocking. The row must
			// still resolve in the base table (a phantom row is never escalated).
			if _, ok := base.Severity(calRow); ok {
				return report.SeverityBlocking, true
			}
			return "", false
		}
		return base.Severity(calRow)
	})
}
