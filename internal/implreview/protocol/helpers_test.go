package protocol

import (
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// roundOf builds a report.Round at a revision with the given findings — the test
// analogue of the runtime's assembled round.
func roundOf(revision string, findings ...report.Finding) report.Round {
	return report.Round{ReviewedRevision: revision, Findings: findings}
}

// computeWith runs verdict.Compute over the rounds with an optional escalation
// and returns just the verdict string — the boundary a protocol test crosses to
// prove its detected condition drives T7's computation. A nil escalation means
// no escalate signal (the verdict derives from the latest round's findings).
func computeWith(rounds []report.Round, esc *verdict.Escalation) (string, error) {
	sig := verdict.Signals{}
	if esc != nil {
		sig.Escalation = esc
	}
	out, err := verdict.Compute(rounds, sig)
	if err != nil {
		return "", err
	}
	return out.Verdict, nil
}

// stubCal is a report.CalLookup for the two calibration rows the fixtures use —
// CAL-1 → blocking, CAL-2 → advisory — so report.Validate (used by RecordRound)
// can certify DINV-3 hermetically.
func stubCal() report.CalLookup {
	return report.CalLookupFunc(func(row string) (string, bool) {
		switch row {
		case "CAL-1":
			return report.SeverityBlocking, true
		case "CAL-2":
			return report.SeverityAdvisory, true
		}
		return "", false
	})
}

// baseReport builds a minimal valid report.Report envelope carrying the rounds —
// the shape RecordRound validates.
func baseReport(verdictStr string, rounds ...report.Round) *report.Report {
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "github:agenticlab-ai/pyxis#412",
		ReviewID:      "rv-1",
		Band:          report.BandStandard,
		Verdict:       verdictStr,
		Rounds:        rounds,
	}
}
