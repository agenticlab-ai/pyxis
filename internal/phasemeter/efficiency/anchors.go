// Package efficiency is the pipeline-measurement efficiency reader: it
// derives a phase's efficiency signals deterministically from the frozen
// corpus + git — no model invocation, no runtime hook
// (docs/tech-design/pipeline-measurement-v2.md §3.10, DINV-1). It counts
// the corpus's own typed anchors — D-n decisions, G-n fog, FND-n review
// findings, PRD-ledger amendments — and reads git history for latency, so
// the same frozen artifacts yield the same signals regardless of which
// harness produced them (FR-4).
package efficiency

import (
	"regexp"
	"strings"
)

// The typed-anchor headings this reader counts. Brainstorm-class records
// (.pyxis/templates/brainstorm-record-template.md,
// design-brainstorm-record-template.md) write decisions as "### D-<n> —
// <gist>" and review reports (prd-review-report-template.md,
// design-review-report-template.md) write findings as "### FND-<n> —
// <gist>" — the same heading convention, different prefix.
var (
	decisionHeadingRE = regexp.MustCompile(`(?m)^###[ \t]+D-\d+\b`)
	findingHeadingRE  = regexp.MustCompile(`(?m)^###[ \t]+FND-\d+\b`)
	fogEntryRE        = regexp.MustCompile(`(?m)^-[ \t]+\*\*G-\d+\*\*`)
	statusLineRE      = regexp.MustCompile(`(?m)^-[ \t]+\*\*Status\*\*:[ \t]*(.+)$`)
)

// Questions counts a brainstorm-class artifact's D-n decision anchors —
// each is one question the session resolved (design §3.10: "questions
// from D-n"; brainstorm-record-model.md §3.2).
func Questions(doc []byte) int {
	return len(decisionHeadingRE.FindAll(doc, -1))
}

// Fog counts a brainstorm-class artifact's G-n fog anchors (design §3.10:
// "fog from G-n"; brainstorm-record-model.md §3.2).
func Fog(doc []byte) int {
	return len(fogEntryRE.FindAll(doc, -1))
}

// SupersededDecisions counts a brainstorm-class artifact's D-n entries
// whose Status line marks them superseded — a ruling the session had to
// revise (design §3.10: "rework from superseded D-n"). Both the live and
// the superseded entry stay in the document (DINV-5 of the record
// models); this counts only the superseded side, once per revision. Status
// is a closed two-way enum (brainstorm-record-template.md: "live |
// superseded by D-m — <reason>"), so only a status *value* that itself
// starts with "superseded" counts — a live decision's reasoning is free to
// mention "superseded" about some other row without being miscounted (a
// real case: docs/brainstorms/autonomous-developer.md's D-3, "live — row 6
// superseded by D-4 ...").
func SupersededDecisions(doc []byte) int {
	n := 0
	for _, m := range statusLineRE.FindAllSubmatch(doc, -1) {
		status := strings.ToLower(strings.TrimSpace(string(m[1])))
		if strings.HasPrefix(status, "superseded") {
			n++
		}
	}
	return n
}

// Defects counts a review report's FND-n finding anchors — each is one
// defect the reviewer raised against the subject (design §3.10: "defects
// from review FND-n"; prd-review-artifacts.md §3.1, shared by the
// tech-design-review sibling class).
func Defects(doc []byte) int {
	return len(findingHeadingRE.FindAll(doc, -1))
}
