package reconcile

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

const prdDispositionTag = "reconcile:prd-disposition"

type prdDisposition struct {
	Record *record.Record
	Answer *record.Record
}

func projectUnit(u fold.ResolvedUnit) PRDUnit {
	out := PRDUnit{
		ID: u.ID, Prefix: u.Prefix, Status: u.Status, IntroducedBy: u.IntroducedBy,
		Supersedes: u.Supersedes, SupersededBy: u.SupersededBy,
		Columns: append([]string(nil), u.Columns...), Cells: append([]string(nil), u.Cells...),
	}
	legacySourceSHA256 := digestJSON(struct {
		ID, Prefix, Status, IntroducedBy string
		Supersedes, SupersededBy         *string
		Columns, AuthoredColumns, Cells  []string
	}{out.ID, out.Prefix, out.Status, out.IntroducedBy, out.Supersedes, out.SupersededBy, out.Columns, u.AuthoredColumns, out.Cells})
	out.LegacySourceSHA256s = []string{legacySourceSHA256}
	// This digest identifies the authored row's semantic payload, not its
	// ledger-position metadata. A verified semantic-no-op rebase may re-anchor
	// IntroducedBy and lineage while preserving the row byte-for-byte; making
	// that bookkeeping part of the tag would manufacture memory drift for no
	// change in meaning. The closeout receipt binds the complete PRD fingerprint
	// and source-map digest separately.
	out.SourceSHA256 = digestJSON(struct {
		Domain                          string
		ID, Prefix                      string
		Columns, AuthoredColumns, Cells []string
	}{"pyxis-memory-prd-unit/v2", out.ID, out.Prefix, out.Columns, u.AuthoredColumns, out.Cells})
	cell := func(name string) string {
		for i, col := range u.Columns {
			if col == name && i < len(u.Cells) {
				return cleanCell(u.Cells[i])
			}
		}
		return ""
	}
	risk := cell("Risk")
	if risk == "" {
		risk = cell("Severity")
	}
	out.Consequence = consequence(risk)
	out.Priority = cell("Priority")
	switch u.Prefix {
	case "FR":
		out.Kind, out.Statement, out.Rationale, out.FitCriterion = record.Promise, cell("Requirement (EARS)"), cell("Rationale"), cell("Fit criterion")
	case "NR":
		out.Kind, out.Statement, out.Rationale, out.FitCriterion = record.Promise, cell("Existing behavior protected"), cell("Why protected"), cell("Fit criterion")
	case "NFR":
		out.Kind, out.Statement, out.Intent, out.FitCriterion = record.Promise, cell("Requirement"), cell("Area"), cell("Fit criterion")
	case "UB":
		trigger, result := cell("If… (trigger)"), cell("Then the system shall…")
		if trigger == "" || result == "" {
			return out
		}
		out.Kind, out.Statement, out.Intent, out.FitCriterion = record.Promise, fmt.Sprintf("If %s, then the system shall %s", trigger, result), cell("User sees"), cell("Fit criterion")
	case "ABUSE":
		actor, vector := cell("Actor and intent"), cell("Vector")
		if actor == "" {
			return out
		}
		out.Kind = record.Promise
		out.Statement = actor
		if vector != "" {
			out.Statement += " — vector: " + vector
		}
		out.Intent, out.FitCriterion = cell("Impact"), cell("Mitigated by")
	case "EVT":
		event, trigger := cell("Event name"), cell("Fires when")
		if event == "" || trigger == "" {
			return out
		}
		out.Kind = record.Promise
		out.Statement = event + " fires when " + trigger
		out.Intent, out.FitCriterion = cell("Properties (typed)"), cell("Feeds")
	case "DOD":
		out.Kind, out.Statement, out.Intent, out.FitCriterion = record.Expectation, cell("Done criterion"), cell("Source"), cell("Required evidence")
	case "EVAL":
		out.Kind, out.Statement, out.Intent, out.FitCriterion = record.Expectation, cell("Behavior"), cell("Eval set / harness"), cell("Threshold")
	case "G":
		belief, result := cell("We believe that…"), cell("Will result in…")
		if belief == "" || result == "" {
			return out
		}
		out.Kind = record.Need
		out.Statement = "We believe " + belief + "; expected result: " + result
		out.FitCriterion = cell("Measured by…")
	case "J":
		situation, motivation, outcome := cell("When… (situation)"), cell("I want to… (motivation)"), cell("So I can… (outcome)")
		if situation == "" || motivation == "" || outcome == "" {
			return out
		}
		out.Kind = record.Need
		out.Statement = "When " + situation + ", I want " + motivation + ", so I can " + outcome
		out.Rationale = cell("Evidence")
	case "M":
		metric, goal := cell("Metric"), cell("Goal")
		if metric == "" || goal == "" {
			return out
		}
		out.Kind, out.Statement, out.Intent, out.FitCriterion = record.Need, metric+": "+goal, cell("Signal"), cell("Target (at checkpoint)")
	case "Q":
		// Questions deliberately remain outside the claim projection: an open
		// question is not product truth. Preserve its exact authored fields so
		// reconciliation can losslessly capture the question without guessing
		// an answer or treating it as a designed requirement.
		out.Statement = cell("Question")
		out.Intent = cell("Owner")
		out.Rationale = cell("Status")
		out.FitCriterion = cell("Needed by")
		return out
	default:
		return out
	}
	out.Statement = strings.TrimSpace(out.Statement)
	out.Mapped = out.Kind != "" && out.Statement != ""
	return out
}

// ProjectDraftRow uses the ledger reconciler's one row-to-claim mapping for a
// draft working file. Callers use the semantic fields; ledger source identities
// on the returned value are never attached to a native draft record.
func ProjectDraftRow(id, prefix string, columns, cells []string) PRDUnit {
	return projectUnit(fold.ResolvedUnit{
		ID: id, Prefix: prefix, Status: "live", Columns: columns,
		AuthoredColumns: columns, Cells: cells,
	})
}

// requiresDesignServe mirrors the contract gate's implementation-coverage
// boundary: only a live requirement explicitly ranked Must has to be served by
// a live design. Jobs, goals, metrics, done/eval criteria, and non-Must rows are
// still durable memory, but forcing a design edge onto each of them invents a
// relationship the approved PRD/design contract never declared.
func requiresDesignServe(u PRDUnit) bool {
	return u.Status == "live" && strings.EqualFold(strings.TrimSpace(u.Priority), "Must")
}

func cleanCell(s string) string {
	return strings.TrimSpace(s)
}

func consequence(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "high":
		return record.High
	case "med", "medium":
		return record.Medium
	case "low":
		return record.Low
	default:
		return ""
	}
}

func prdDraft(prd PRD, commit string, u PRDUnit) Draft {
	return Draft{
		Kind: u.Kind, Subject: "prd/" + prd.Feature, Family: u.ID, Statement: u.Statement,
		Intent: u.Intent, Rationale: u.Rationale, FitCriterion: u.FitCriterion,
		Consequence: u.Consequence,
		Tags:        []string{"reconcile:prd", "source-unit:" + u.SourceSHA256, "source:prd-ledger"},
		Provenance: record.Provenance{
			Source: prd.Source, Commit: commit,
			Note: "compiled from " + prd.Fingerprint,
		},
	}
}

func prdQuestionDraft(prd PRD, commit string, u PRDUnit) (Draft, bool) {
	if u.Prefix != "Q" || strings.TrimSpace(u.Statement) == "" || !validDigest(u.SourceSHA256) {
		return Draft{}, false
	}
	return Draft{
		Kind: record.Question, Subject: "prd/" + prd.Feature, Family: u.ID,
		Statement: u.Statement, Intent: u.Intent, Rationale: u.Rationale,
		FitCriterion: u.FitCriterion,
		Tags:         []string{prdDispositionTag, "source-unit:" + u.SourceSHA256, "source:prd-ledger"},
		Provenance: record.Provenance{
			Source: prd.Source, Commit: commit,
			Note: "captured from " + prd.Fingerprint,
		},
	}, true
}

// explicitPRDDisposition validates the deliberately human-authored bridge for
// PRD rows that have no lossless claim mapping. The v1 vocabulary is closed:
// an authored question is a Question with exactly one Answer before closeout;
// a risk is a Decision that states how it was accepted, mitigated, or
// transferred. Both carry the exact current row digest and deterministic row
// address, so an older or neighboring disposition cannot satisfy the unit.
func explicitPRDDisposition(prd PRD, u PRDUnit, proj *resolve.Projection) (prdDisposition, error) {
	if proj == nil {
		return prdDisposition{}, fmt.Errorf("memory projection is unavailable")
	}
	supported := u.Prefix == "Q" || u.Prefix == "R"
	if supported && !validDigest(u.SourceSHA256) {
		return prdDisposition{}, fmt.Errorf("live PRD unit %s has no exact source-unit digest", u.ID)
	}
	subject := "prd/" + prd.Feature
	wantSourceTags := sourceUnitTags(u)
	var candidates []*record.Record
	for _, r := range proj.Active(subject) {
		if !r.HasTag(prdDispositionTag) {
			continue
		}
		if r.Family == u.ID || hasAnyTag(r, wantSourceTags) {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) == 0 {
		return prdDisposition{}, nil
	}
	if len(candidates) != 1 {
		return prdDisposition{}, fmt.Errorf("live PRD unit %s has %d active disposition records", u.ID, len(candidates))
	}
	r := candidates[0]
	if !supported {
		return prdDisposition{}, fmt.Errorf("live PRD unit %s has unsupported disposition prefix %s", u.ID, u.Prefix)
	}
	if r.Subject != subject || r.Family != u.ID {
		return prdDisposition{}, fmt.Errorf("disposition %s does not use exact address %s#%s", r.ID, subject, u.ID)
	}
	if countTag(r, prdDispositionTag) != 1 || countSourceUnitTags(r, wantSourceTags) != 1 {
		return prdDisposition{}, fmt.Errorf("disposition %s must carry exactly one %s tag and one recognized source-unit tag", r.ID, prdDispositionTag)
	}
	if r.Provenance.Source != prd.Source || !hasSourceLocator(r) {
		return prdDisposition{}, fmt.Errorf("disposition %s does not carry exact PRD source provenance", r.ID)
	}

	out := prdDisposition{Record: r}
	switch u.Prefix {
	case "Q":
		if r.Kind != record.Question {
			return prdDisposition{}, fmt.Errorf("question unit %s is dispositioned by %s, not a Question", u.ID, r.ID)
		}
		if r.Statement != u.Statement || r.Intent != u.Intent || r.Rationale != u.Rationale || r.FitCriterion != u.FitCriterion {
			return prdDisposition{}, fmt.Errorf("question disposition %s does not equal the exact current source row", r.ID)
		}
		var answers []*record.Record
		for _, answer := range proj.Set.From(r.ID, record.Answers) {
			if proj.Status(answer.ID) == resolve.Active {
				answers = append(answers, answer)
			}
		}
		if len(answers) > 1 {
			return prdDisposition{}, fmt.Errorf("question disposition %s has %d active Answers", r.ID, len(answers))
		}
		if len(answers) == 1 {
			if answers[0].Subject != subject {
				return prdDisposition{}, fmt.Errorf("question disposition %s is answered outside %s", r.ID, subject)
			}
			out.Answer = answers[0]
		}
	case "R":
		if r.Kind != record.Decision || strings.TrimSpace(r.Rationale) == "" || strings.TrimSpace(r.FitCriterion) == "" {
			return prdDisposition{}, fmt.Errorf("risk unit %s requires a Decision disposition with rationale and fit criterion", u.ID)
		}
	}
	return out, nil
}

func countTag(r *record.Record, want string) int {
	count := 0
	for _, tag := range r.Tags {
		if tag == want {
			count++
		}
	}
	return count
}

func sourceUnitTags(u PRDUnit) map[string]bool {
	out := map[string]bool{"source-unit:" + u.SourceSHA256: true}
	for _, digest := range u.LegacySourceSHA256s {
		if validDigest(digest) {
			out["source-unit:"+digest] = true
		}
	}
	return out
}

func hasAnyTag(r *record.Record, wants map[string]bool) bool {
	for _, tag := range r.Tags {
		if wants[tag] {
			return true
		}
	}
	return false
}

func countSourceUnitTags(r *record.Record, wants map[string]bool) int {
	count, exact := 0, 0
	for _, tag := range r.Tags {
		if !strings.HasPrefix(tag, "source-unit:") {
			continue
		}
		count++
		if wants[tag] {
			exact++
		}
	}
	if count != 1 || exact != 1 {
		return 0
	}
	return 1
}
