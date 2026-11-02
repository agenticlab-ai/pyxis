package eval

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// The compactness bound (§3.5, §3.10: "a compactness eval leg — a bound on
// payload size across the golden fixtures"). Verbosity is a SOURCE defect (a
// verbose lens/rendering prompt), NOT a persistence case — the design forbids
// splitting the artifact, so the artifact must stay small enough to be ONE
// self-contained artifact on the proposal's medium (DINV-9). This leg reds a
// payload trending large over the golden set, so the fix lands at the prompt.
//
// The bound is enforced over a REAL artifact: the harness builds a report.Report
// from a golden's seeded findings and composes the ACTUAL persisted artifact
// (persist.ComposeArtifact — the human rendering + the machine block), then
// bounds its byte size. This is the same artifact an adapter writes, so the bound
// is on the real thing, not an estimate.

// MaxArtifactBytes is the compactness bound: the largest a review artifact may be
// over the golden fixtures. It is well under any code-host's per-artifact limit
// (GitHub's comment body limit is 65536 bytes; this leaves wide margin so the
// artifact is always one comment — §3.2). A golden artifact over this is a
// verbosity defect the leg reds. This is a STARTING VALUE tuned only by a
// reviewed data change, like the calibration thresholds and the band budgets.
const MaxArtifactBytes = 16384

// CompactnessResult is one golden's compactness outcome. Deterministic over the
// composed artifact's size.
type CompactnessResult struct {
	// Passed is true iff the composed artifact is within MaxArtifactBytes.
	Passed bool
	// Bytes is the composed artifact's actual size.
	Bytes int
	// Bound is the bound it was checked against (MaxArtifactBytes).
	Bound int
}

// ScoreCompactness composes the real persisted artifact for a report and bounds
// its byte size (§3.5). It PASSES iff the artifact is within MaxArtifactBytes. A
// caller runs this over each golden's report; an over-large artifact reds the
// compactness leg (the payload is trending large — a prompt/verbosity defect,
// never worked around by splitting the artifact).
func ScoreCompactness(r *report.Report) (CompactnessResult, error) {
	body, err := persist.ComposeArtifact(r)
	if err != nil {
		return CompactnessResult{}, fmt.Errorf("eval: compactness: compose artifact: %w", err)
	}
	n := len(body)
	return CompactnessResult{
		Passed: n <= MaxArtifactBytes,
		Bytes:  n,
		Bound:  MaxArtifactBytes,
	}, nil
}

// GoldenReport builds a valid report.Report from a nit-golden's seeded findings —
// the payload the compactness leg composes and bounds. It is a real payload: one
// round, the golden's band, the findings as report.Finding, and a verdict derived
// so report.Validate's brief-iff-escalate rule holds (a golden never escalates, so
// no brief). The revision/context/provenance are deterministic fixed strings (a
// golden is a fixture, not a live run).
func GoldenReport(familyID string, goldenIdx int, g *Golden) *report.Report {
	findings := make([]report.Finding, 0, len(g.Findings))
	for i := range g.Findings {
		findings = append(findings, toReportFinding(fmt.Sprintf("FND-%d", i+1), &g.Findings[i]))
	}
	// A golden with only advisory (or zero) findings draws approve-with-advisories
	// or approve — never a blocking verdict (the goldens are clean by construction).
	blocking := false
	advisory := false
	for i := range findings {
		switch findings[i].Severity {
		case report.SeverityBlocking:
			blocking = true
		case report.SeverityAdvisory:
			advisory = true
		}
	}
	v := report.VerdictApprove
	switch {
	case blocking:
		v = report.VerdictRequestChanges
	case advisory:
		v = report.VerdictApproveWithAdvisories
	}
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   fmt.Sprintf("eval:%s/golden-%d", familyID, goldenIdx),
		ReviewID:      fmt.Sprintf("%s-golden-%d", familyID, goldenIdx),
		Band:          g.Band,
		Verdict:       v,
		Rounds: []report.Round{{
			ReviewedRevision: fmt.Sprintf("golden-%s-%d", familyID, goldenIdx),
			ContextID:        fmt.Sprintf("ctx-golden-%s-%d", familyID, goldenIdx),
			Provenance: report.Provenance{
				Model:          "eval-golden",
				PromptsVersion: "impl-review-lenses/v1",
				CalVersion:     "impl-review-calibration/v1",
			},
			Findings:      findings,
			FindingsCount: len(findings),
			Candidates:    len(findings),
			Killed:        0,
		}},
	}
}

// toReportFinding builds a report.Finding from a fixture Finding — the harness's
// compact fixture form projected onto the real payload finding. Used by the
// golden report builder and the fake runner.
func toReportFinding(id string, f *Finding) report.Finding {
	return report.Finding{
		ID:         id,
		Message:    f.Message,
		RuleID:     f.RuleID,
		Lens:       lensOfRuleID(f.RuleID),
		Severity:   f.Severity,
		CalRow:     f.CalRow,
		Acceptance: "seeded eval finding",
	}
}

// lensOfRuleID returns the lens a rule identity names — the token BEFORE the first
// "/" in a `<lens>/<class>` RuleID. It maps the fixture's rule identity onto the
// report finding's Lens field. A RuleID with no "/" yields the whole string (a
// malformed identity — the runtime stamps the lens anyway).
func lensOfRuleID(ruleID string) string {
	for i := 0; i < len(ruleID); i++ {
		if ruleID[i] == '/' {
			return ruleID[:i]
		}
	}
	return ruleID
}
