package band_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// baseInputs is a minimal, complete set of the five T3 pack inputs — the base a
// band enriches (or leaves alone).
func baseInputs() pack.ReviewInputs {
	return pack.ReviewInputs{
		Subject:         "pr#42",
		Proposal:        []byte("diff --git a/x b/x\n+ a change\n"),
		CodegateVerdict: []byte("codegate: green\n"),
		Charter:         []byte("the task charter\n"),
		DesignBindings:  []byte("the governing design's binding elements\n"),
		Calibration:     []byte("the calibration table\n"),
	}
}

// deepContext is a non-empty DeepContext — the two mandatory sources FR-10
// serves in the deep band.
func deepContext() band.DeepContext {
	return band.DeepContext{
		AbuseRows:        []byte("ABUSE-1 | implementing agent optimizing past review | shapes the diff to evade\n"),
		IncidentRegistry: []byte("I2 (2026-07-09) [merge-train] a stacked-train relanding lesson\n"),
	}
}

// TestDeepBandCarriesMandatoryContext — FR-10: a deep-band run enriches the pack
// with the served ABUSE rows AND the incident registry, and the enriched pack
// actually builds and CONTAINS both.
func TestDeepBandCarriesMandatoryContext(t *testing.T) {
	t.Parallel()

	enriched, err := band.EnrichInputs(baseInputs(), report.BandDeep, deepContext())
	if err != nil {
		t.Fatalf("EnrichInputs (deep): %v", err)
	}

	if !band.HasDeepContext(enriched) {
		t.Fatal("deep-band enriched pack does not carry the mandatory-context marker")
	}

	// The two mandatory sources are present in the governing input.
	gov := string(enriched.DesignBindings)
	if !strings.Contains(gov, "ABUSE-1") {
		t.Error("deep-band pack input is missing the served ABUSE rows")
	}
	if !strings.Contains(gov, "I2 (2026-07-09)") {
		t.Error("deep-band pack input is missing the incident registry")
	}
	// The base governing content is preserved (the enrichment appends, never
	// replaces).
	if !strings.Contains(gov, "the governing design's binding elements") {
		t.Error("deep-band enrichment dropped the base design bindings")
	}

	// The enriched inputs still build a valid pack (the enrichment is honest pack
	// input, not a malformed one).
	if _, err := pack.Build(enriched); err != nil {
		t.Fatalf("enriched deep-band pack does not build: %v", err)
	}
}

// TestStandardBandOmitsExtraContext — FR-10 cost audit: a standard-band run does
// NOT carry the deep-band mandatory context; the inputs are the base verbatim.
func TestStandardBandOmitsExtraContext(t *testing.T) {
	t.Parallel()

	base := baseInputs()
	out, err := band.EnrichInputs(base, report.BandStandard, deepContext())
	if err != nil {
		t.Fatalf("EnrichInputs (standard): %v", err)
	}
	if band.HasDeepContext(out) {
		t.Fatal("standard-band pack carries the deep-band mandatory context (cost audit fails)")
	}
	if string(out.DesignBindings) != string(base.DesignBindings) {
		t.Fatal("standard-band enriched the governing input — the extra context is not absent")
	}
	// The skip band likewise omits it.
	skipOut, err := band.EnrichInputs(base, report.BandSkip, deepContext())
	if err != nil {
		t.Fatalf("EnrichInputs (skip): %v", err)
	}
	if band.HasDeepContext(skipOut) {
		t.Fatal("skip-band pack carries the deep-band mandatory context")
	}
}

// TestDeepBandEmptyContextFailsClosed — the deep-band context is MANDATORY: a
// deep band with an empty DeepContext is refused, never a silent standard-depth
// run.
func TestDeepBandEmptyContextFailsClosed(t *testing.T) {
	t.Parallel()
	_, err := band.EnrichInputs(baseInputs(), report.BandDeep, band.DeepContext{})
	if err == nil {
		t.Fatal("deep band with empty mandatory context should fail closed")
	}
	if !strings.Contains(err.Error(), "mandatory context") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// criticAdvisoryRow is the correctness-critic's advisory calibration row in the
// shipped table (CAL-12 — the row a load-bearing critic finding escalates from).
const criticAdvisoryRow = "CAL-12"

// shippedCal loads the real shipped calibration table — the base dial the
// escalation wraps, so the test proves the escalation over the ACTUAL data.
func shippedCal(t *testing.T) report.CalLookup {
	t.Helper()
	calPath := filepath.Join("..", "..", "..", ".pyxis", "skills", "impl-review", "calibration.md")
	tbl, err := calibration.LoadFile(calPath)
	if err != nil {
		t.Fatalf("load shipped calibration: %v", err)
	}
	// Sanity: the critic advisory row really is advisory in the base table (so the
	// escalation is a genuine change, not a no-op).
	sev, ok := tbl.Severity(criticAdvisoryRow)
	if !ok || sev != report.SeverityAdvisory {
		t.Fatalf("shipped %s severity = %q (ok=%t), want advisory", criticAdvisoryRow, sev, ok)
	}
	return tbl
}

// TestEscalatedCriticShipsLoadBearingFindingBlocking — FR-10: in the DEEP band
// the critic's calibration escalates, so a load-bearing critic finding citing
// the critic advisory row ships BLOCKING and VALIDATES blocking; in the standard
// band the same finding validates only as advisory.
func TestEscalatedCriticShipsLoadBearingFindingBlocking(t *testing.T) {
	t.Parallel()
	baseCal := shippedCal(t)

	// A seeded load-bearing correctness-critic finding — in the deep band it ships
	// BLOCKING (the escalation); it cites the critic advisory row.
	deepFinding := report.Finding{
		ID:         "FND-CRIT",
		Message:    "a load-bearing off-by-one behind passing happy-path tests",
		RuleID:     "correctness-critic/load-bearing-edge",
		Lens:       report.LensCorrectnessCritic,
		Severity:   report.SeverityBlocking, // escalated in the deep band
		CalRow:     criticAdvisoryRow,
		Acceptance: "A-1",
	}

	deepReport := &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "pr#42",
		ReviewID:      "rev-1",
		Band:          report.BandDeep,
		Verdict:       report.VerdictRequestChanges,
		Rounds: []report.Round{{
			ReviewedRevision: "deadbeef",
			Findings:         []report.Finding{deepFinding},
		}},
	}

	// In the DEEP band, the escalated lookup maps the critic advisory row to
	// blocking, so the blocking finding validates.
	deepCal := band.EscalateCritic(baseCal, report.BandDeep, criticAdvisoryRow)
	if err := report.Validate(deepReport, deepCal); err != nil {
		t.Fatalf("deep-band load-bearing critic finding should validate blocking: %v", err)
	}
	if sev, _ := deepCal.Severity(criticAdvisoryRow); sev != report.SeverityBlocking {
		t.Fatalf("deep-band escalated %s = %q, want blocking", criticAdvisoryRow, sev)
	}

	// In the STANDARD band, the SAME finding at BLOCKING severity must NOT validate
	// — the standard band does not escalate the critic (the row stays advisory).
	stdCal := band.EscalateCritic(baseCal, report.BandStandard, criticAdvisoryRow)
	if sev, _ := stdCal.Severity(criticAdvisoryRow); sev != report.SeverityAdvisory {
		t.Fatalf("standard-band %s = %q, want advisory (no escalation)", criticAdvisoryRow, sev)
	}
	stdReport := &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "pr#42",
		ReviewID:      "rev-1",
		Band:          report.BandStandard,
		Verdict:       report.VerdictRequestChanges,
		Rounds: []report.Round{{
			ReviewedRevision: "deadbeef",
			Findings:         []report.Finding{deepFinding}, // same blocking finding
		}},
	}
	if err := report.Validate(stdReport, stdCal); err == nil {
		t.Fatal("standard-band critic finding at blocking should NOT validate (no escalation)")
	}
}

// outsideProcessRaceRow is the correctness-critic row for a race that needs an
// unrelated outside process the contract does not guard against (CAL-13). No
// contract rests on such a race, so it is never a load-bearing critic finding.
const outsideProcessRaceRow = "CAL-13"

// TestOutsideProcessRaceStaysAdvisoryInDeepBand — the deep band's critic
// escalation promotes only the critic advisory row; a race that needs an
// unrelated outside process stays advisory, so it cannot return the worker for
// a redesign in either band.
func TestOutsideProcessRaceStaysAdvisoryInDeepBand(t *testing.T) {
	t.Parallel()
	tbl, err := calibration.LoadFile(filepath.Join("..", "..", "..", ".pyxis", "skills", "impl-review", "calibration.md"))
	if err != nil {
		t.Fatalf("load shipped calibration: %v", err)
	}
	row, ok := tbl.Row(outsideProcessRaceRow)
	if !ok || row.Lens != report.LensCorrectnessCritic || row.Severity != report.SeverityAdvisory || !strings.Contains(row.Escalation, "unrelated outside process") {
		t.Fatalf("shipped %s = %+v (ok=%t), want the critic's advisory outside-process race row", outsideProcessRaceRow, row, ok)
	}

	race := report.Finding{
		ID:         "FND-RACE",
		Message:    "another Git process could create the branch between the absence check and worktree add",
		RuleID:     "correctness-critic/outside-process-race",
		Lens:       report.LensCorrectnessCritic,
		Severity:   report.SeverityAdvisory,
		CalRow:     outsideProcessRaceRow,
		Acceptance: "A-1",
	}
	for _, bnd := range []band.Band{report.BandDeep, report.BandStandard} {
		cal := band.EscalateCritic(tbl, bnd, criticAdvisoryRow)
		if sev, _ := cal.Severity(outsideProcessRaceRow); sev != report.SeverityAdvisory {
			t.Fatalf("band %q %s = %q, want advisory", bnd, outsideProcessRaceRow, sev)
		}
		rep := &report.Report{
			SchemaVersion: report.SchemaVersion,
			ProposalRef:   "pr#42",
			ReviewID:      "rev-1",
			Band:          bnd,
			Verdict:       report.VerdictApproveWithAdvisories,
			Rounds: []report.Round{{
				ReviewedRevision: "deadbeef",
				Findings:         []report.Finding{race},
			}},
		}
		if err := report.Validate(rep, cal); err != nil {
			t.Fatalf("band %q advisory outside-process race should validate: %v", bnd, err)
		}
		outcome, err := verdict.Compute(rep.Rounds, verdict.Signals{})
		if err != nil || outcome.Verdict != report.VerdictApproveWithAdvisories || outcome.Downgrades() {
			t.Fatalf("band %q verdict = %+v err=%v, want approve-with-advisories with no return to the worker", bnd, outcome, err)
		}

		blocking := race
		blocking.Severity = report.SeverityBlocking
		rep.Verdict = report.VerdictRequestChanges
		rep.Rounds[0].Findings = []report.Finding{blocking}
		if err := report.Validate(rep, cal); err == nil {
			t.Fatalf("band %q outside-process race at blocking should NOT validate", bnd)
		}
	}
}

// TestEscalateCriticLeavesOtherRowsUnchanged — the escalation touches ONLY the
// critic advisory row in the deep band; every other row (a blocking critic row,
// a security row) resolves to the base severity unchanged, in every band.
func TestEscalateCriticLeavesOtherRowsUnchanged(t *testing.T) {
	t.Parallel()
	baseCal := shippedCal(t)

	for _, bnd := range []band.Band{report.BandDeep, report.BandStandard, report.BandSkip} {
		esc := band.EscalateCritic(baseCal, bnd, criticAdvisoryRow)
		// A non-critic-advisory row is untouched in every band.
		for _, row := range []string{"CAL-1", "CAL-8", "CAL-11"} {
			want, _ := baseCal.Severity(row)
			got, ok := esc.Severity(row)
			if !ok || got != want {
				t.Errorf("band %q row %q = %q (ok=%t), want base %q unchanged", bnd, row, got, ok, want)
			}
		}
	}
	// A phantom row is never escalated (it does not resolve in the base).
	esc := band.EscalateCritic(baseCal, report.BandDeep, criticAdvisoryRow)
	if _, ok := esc.Severity("CAL-DOES-NOT-EXIST"); ok {
		t.Error("escalation resolved a phantom row")
	}
}
