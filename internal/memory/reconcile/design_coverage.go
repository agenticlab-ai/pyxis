package reconcile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

const (
	designCoverageFamily = "D-SERVES"
	designCoverageTag    = "reconcile:design-coverage"
)

// designCoverageDraft losslessly projects one approved design's declared
// Serves header into an explicit Decision. The header is itself the governing
// coverage assertion; this record does not guess which prose decision block
// owns each target. Only currently resolvable PRD heads become relations, so a
// stale source reference remains a visible reconciliation finding. Exact live
// NG/WR rows remain non-relational context because the PRD projection
// deliberately does not turn them into durable memory truth.
func designCoverageDraft(s Snapshot, d Design, set *store.Set, proj *resolve.Projection) (Draft, bool) {
	if d.Status != "live" || len(d.Serves) == 0 || !validDigest(d.SHA256) || !validGitObjectID(s.Commit) || set == nil || proj == nil {
		return Draft{}, false
	}
	subject := "design/" + d.Slug
	if proj.Head(subject, designCoverageFamily) != nil {
		return Draft{}, false
	}
	draft, ok := expectedDesignCoverage(s, d, set, proj)
	if !ok {
		return Draft{}, false
	}

	linked := map[string]bool{}
	for _, decision := range proj.Active(subject) {
		if decision.Kind != record.Decision {
			continue
		}
		for _, target := range decision.Rel(record.Serves) {
			linked[target] = true
		}
	}
	for _, relation := range draft.Relations {
		if relation.Type == record.Serves && !linked[relation.Target] {
			return draft, true
		}
	}
	return Draft{}, false
}

func expectedDesignCoverage(s Snapshot, d Design, set *store.Set, proj *resolve.Projection) (Draft, bool) {
	return designCoverageWithTargets(s, d, set, proj, func(subject, family string) *record.Record {
		head := proj.Head(subject, family)
		if head == nil || !exactPRDTarget(s, subject, family, head) {
			return nil
		}
		return head
	})
}

func designCoverageWithTargets(s Snapshot, d Design, set *store.Set, proj *resolve.Projection, targetFor func(string, string) *record.Record) (Draft, bool) {
	if d.Status != "live" || len(d.Serves) == 0 || !validDigest(d.SHA256) || set == nil || proj == nil {
		return Draft{}, false
	}
	document := exactDesignDocument(s, d, set, proj)
	if document == nil {
		return Draft{}, false
	}
	seen := map[string]bool{}
	var relations []record.Relation
	for _, ref := range d.Serves {
		prdSubject, family, ok := memoryPRDRef(ref)
		if !ok {
			continue
		}
		if nonDurablePRDContextRef(s, prdSubject, family) {
			continue
		}
		target := targetFor(prdSubject, family)
		if target == nil {
			return Draft{}, false
		}
		if seen[target.ID] {
			continue
		}
		seen[target.ID] = true
		relations = append(relations, record.Relation{Type: record.Serves, Target: target.ID})
	}
	if len(relations) == 0 {
		return Draft{}, false
	}
	relations = append(relations, record.Relation{Type: record.DependsOn, Target: document.ID})
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].Type != relations[j].Type {
			return relations[i].Type < relations[j].Type
		}
		return relations[i].Target < relations[j].Target
	})

	return Draft{
		Kind: record.Decision, Subject: "design/" + d.Slug, Family: designCoverageFamily,
		Statement:    "The approved design declares its Serves contract as: " + strings.Join(d.Serves, " · ") + ".",
		Intent:       "Preserve the approved design-to-requirement coverage contract without assigning targets to unrelated prose decisions.",
		Rationale:    "The source design's Serves header is the authoritative coverage declaration.",
		FitCriterion: "Every durable, memory-eligible PRD target in the declared header is carried by one serves relation; exact live NG/WR rows remain non-relational context and unresolved targets remain explicit reconciliation findings.",
		Tags:         []string{designCoverageTag, "source-design:" + d.SHA256, "source:tech-design"},
		Relations:    relations,
		Provenance: record.Provenance{
			Source: d.Source, Commit: s.Commit,
			Note: "projected from exact design header " + d.SHA256,
		},
	}, true
}

func validateDesignCoverageMemory(head *record.Record, expected Draft) error {
	if head.Kind != expected.Kind || head.Subject != expected.Subject || head.Family != expected.Family || head.Statement != expected.Statement || head.Intent != expected.Intent || head.Rationale != expected.Rationale || head.FitCriterion != expected.FitCriterion {
		return fmt.Errorf("memory head %s does not equal the approved design coverage declaration", head.ID)
	}
	if head.Consequence != "" || head.Expires != "" || len(head.LearnedFrom) != 0 || len(head.Alternatives) != 0 {
		return fmt.Errorf("memory head %s adds semantic fields absent from the design header", head.ID)
	}
	if !sameStringSet(head.Tags, expected.Tags) {
		return fmt.Errorf("memory head %s does not carry the exact design coverage tag set", head.ID)
	}
	if head.Provenance.Source != expected.Provenance.Source || !hasSourceLocator(head) || head.Provenance.Note != expected.Provenance.Note {
		return fmt.Errorf("memory head %s does not carry exact design source provenance", head.ID)
	}
	if !sameProjectedRelationSet(head.Relations, expected.Relations) {
		return fmt.Errorf("memory head %s does not carry the exact current depends_on and serves relation set", head.ID)
	}
	return nil
}

// sameProjectedRelationSet compares the source-projected semantic edges while
// allowing the one history edge required when an immutable record is updated.
// The resolver independently validates that supersedes is backward, same-kind,
// and same-family; all other extra relations remain semantic drift.
func sameProjectedRelationSet(actual, expected []record.Relation) bool {
	semantic := make([]record.Relation, 0, len(actual))
	supersedes := 0
	for _, relation := range actual {
		if relation.Type == record.Supersedes {
			supersedes++
			continue
		}
		semantic = append(semantic, relation)
	}
	return supersedes <= 1 && sameRelationSet(semantic, expected)
}

func exactPRDTarget(s Snapshot, subject, family string, head *record.Record) bool {
	feature := strings.TrimPrefix(subject, "prd/")
	if feature == subject {
		return false
	}
	for _, prd := range s.PRDs {
		if prd.Feature != feature {
			continue
		}
		for _, unit := range prd.Units {
			if unit.ID == family && unit.Status == "live" && unit.Mapped {
				return samePRDSemantics(head, unit)
			}
		}
		return false
	}
	return false
}

func sameRelationSet(a, b []record.Relation) bool {
	if len(a) != len(b) {
		return false
	}
	left, right := append([]record.Relation(nil), a...), append([]record.Relation(nil), b...)
	less := func(xs []record.Relation) func(int, int) bool {
		return func(i, j int) bool {
			if xs[i].Type != xs[j].Type {
				return xs[i].Type < xs[j].Type
			}
			return xs[i].Target < xs[j].Target
		}
	}
	sort.Slice(left, less(left))
	sort.Slice(right, less(right))
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func exactDesignDocument(s Snapshot, d Design, set *store.Set, proj *resolve.Projection) *record.Record {
	f, ok := s.Files[d.Source]
	if !ok {
		return nil
	}
	var exact *record.Record
	for _, candidate := range proj.Active("design/" + d.Slug) {
		if candidate.Kind != record.Document || candidate.Provenance.Source != d.Source || !documentMatchesSource(candidate, set.Bodies[candidate.ID], f) {
			continue
		}
		if exact != nil {
			return nil
		}
		exact = candidate
	}
	return exact
}
