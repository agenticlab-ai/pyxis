package lock

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

// UnitVerdict describes what one compiled unit contributes to a new lock.
type UnitVerdict string

const (
	New         UnitVerdict = "new"
	Superseding UnitVerdict = "superseding"
	Unchanged   UnitVerdict = "unchanged"
	Revoked     UnitVerdict = "revoked"
)

type unitChange struct {
	Verdict  UnitVerdict
	Previous *record.Record
}

// The two unnumbered working files are per-version evidence. Their memory
// addresses must remain distinct so a later lock cannot retire their history.
func versionHistory(result compile.Result, version int) compile.Result {
	result.Records = slices.Clone(result.Records)
	for i := range result.Records {
		d := &result.Records[i].Draft
		switch {
		case d.Kind == record.Document && d.Subject == "prd/"+result.Feature && d.Family == "problems":
			d.Family += "-v" + strconv.Itoa(version)
		case d.Kind == record.Document && d.Subject == "feature/"+result.Feature && d.Family == "events":
			d.Family += "-v" + strconv.Itoa(version)
		}
	}
	return result
}

// compareRecords uses the binding projection, rather than a guessed first
// batch, to find each unit's active predecessor. A relation changes when its
// target is reissued even if the consumer's own prose is identical.
func compareRecords(entries []compile.Record, feature string, binding, mainline *resolve.Projection, order []int) (map[int]unitChange, []*record.Record, error) {
	changes := make(map[int]unitChange, len(entries))
	if binding == nil {
		for i := range entries {
			changes[i] = unitChange{Verdict: New}
		}
		return changes, nil, nil
	}
	local := map[string]int{}
	for i, entry := range entries {
		if entry.Draft.Family != "" {
			local[entry.Draft.Subject+"#"+entry.Draft.Family] = i
		}
	}
	revoke := map[string]*record.Record{}
	for _, i := range order {
		entry := entries[i]
		d := entry.Draft
		address := d.Subject + "#" + d.Family
		var previous *record.Record
		if d.Kind == record.Answer {
			if len(entry.Links) != 1 || entry.Links[0].Type != record.Answers {
				return nil, nil, fmt.Errorf("compiled Answer must identify one question")
			}
			subject, family, ok := strings.Cut(entry.Links[0].Target, "#")
			if !ok {
				return nil, nil, fmt.Errorf("invalid Answer target %s", entry.Links[0].Target)
			}
			if q := activeAt(binding, subject, family); q != nil {
				previous = binding.Answer(q.ID)
			}
		} else {
			previous = activeAt(binding, d.Subject, d.Family)
		}
		if previous != nil && previous.Kind != d.Kind {
			return nil, nil, fmt.Errorf("compiled %s changes kind of binding %s at %s", d.Kind, previous.ID, address)
		}
		if historicalUnit(d, feature) {
			if previous != nil {
				return nil, nil, fmt.Errorf("historical address %s was already used", address)
			}
			changes[i] = unitChange{Verdict: New}
			continue
		}
		if previous == nil {
			changes[i] = unitChange{Verdict: New}
			continue
		}
		related := make([]record.Relation, 0, len(entry.Links))
		targetChanged := false
		for _, link := range entry.Links {
			var target *record.Record
			if j, ok := local[link.Target]; ok {
				change, done := changes[j]
				if !done {
					return nil, nil, fmt.Errorf("compiled relation order at %s -> %s", address, link.Target)
				}
				if change.Verdict != Unchanged {
					targetChanged = true
				} else {
					target = change.Previous
				}
			} else {
				subject, family, ok := strings.Cut(link.Target, "#")
				if !ok {
					return nil, nil, fmt.Errorf("invalid compiled relation %s", link.Target)
				}
				if isRowAddress(subject, family, feature) && activeAt(binding, subject, family) != nil {
					return nil, nil, fmt.Errorf("compiled relation %s targets a row absent from reopened draft", link.Target)
				}
				if mainline != nil {
					target = activeAt(mainline, subject, family)
				}
			}
			if target == nil {
				targetChanged = true
			} else {
				related = append(related, record.Relation{Type: link.Type, Target: target.ID})
			}
		}
		if previous.Kind == record.Answer {
			// An Answer cannot supersede across question identities. Retire
			// the old Answer and issue a new one for a reworded Question.
			oldQ := previous.Rel(record.Answers)
			if len(related) == 0 || len(oldQ) != 1 || related[0].Target != oldQ[0] {
				revoke[previous.ID] = previous
				changes[i] = unitChange{Verdict: New}
				continue
			}
		}
		if targetChanged || !sameUnit(d, previous, binding, related) {
			changes[i] = unitChange{Verdict: Superseding, Previous: previous}
		} else {
			changes[i] = unitChange{Verdict: Unchanged, Previous: previous}
		}
	}
	for _, subject := range []string{"prd/" + feature, "design/" + feature} {
		for _, old := range binding.Active(subject) {
			if !isRowAddress(subject, old.Family, feature) || !record.IsClaim(old.Kind) && old.Kind != record.Question {
				continue
			}
			address := subject + "#" + old.Family
			index, kept := local[address]
			if !kept {
				revoke[old.ID] = old
			}
			if old.Kind == record.Question {
				if answer := binding.Answer(old.ID); answer != nil {
					if !kept || changes[index].Verdict != Unchanged || !hasAnswer(entries, address) {
						revoke[answer.ID] = answer
					}
				}
			}
		}
	}
	removed := make([]*record.Record, 0, len(revoke))
	for _, old := range revoke {
		removed = append(removed, old)
	}
	sort.Slice(removed, func(i, j int) bool {
		priority := func(r *record.Record) int {
			switch r.Kind {
			case record.Answer:
				return 0
			case record.Decision:
				return 1
			default:
				return 2
			}
		}
		if priority(removed[i]) != priority(removed[j]) {
			return priority(removed[i]) < priority(removed[j])
		}
		return removed[i].ID < removed[j].ID
	})
	return changes, removed, nil
}

func isRowAddress(subject, family, feature string) bool {
	if subject != "prd/"+feature && subject != "design/"+feature {
		return false
	}
	if subject == "design/"+feature && strings.HasPrefix(family, "D-") {
		n, err := strconv.Atoi(strings.TrimPrefix(family, "D-"))
		return err == nil && n > 0 && family == "D-"+strconv.Itoa(n)
	}
	id, prefix, ok := schema.ParseID(family)
	return ok && id == family && prefix != "P" && (subject == "prd/"+feature || prefix == "Q")
}

func activeAt(p *resolve.Projection, subject, family string) *record.Record {
	if p == nil {
		return nil
	}
	if head := p.Head(subject, family); head != nil {
		return head
	}
	var found *record.Record
	for _, candidate := range p.Active(subject) {
		if candidate.Family == family && (found == nil || found.ID < candidate.ID) {
			found = candidate
		}
	}
	return found
}

func historicalUnit(d reconcile.Draft, feature string) bool {
	if d.Kind == record.Document {
		if d.Subject == "prd/"+feature && d.Family == "prd" || d.Subject == "design/"+feature && d.Family == "design" {
			return false
		}
		return true
	}
	return d.Subject == "prd/"+feature && strings.HasPrefix(d.Family, "P-")
}

func hasAnswer(entries []compile.Record, address string) bool {
	for _, entry := range entries {
		if entry.Draft.Kind != record.Answer {
			continue
		}
		for _, link := range entry.Links {
			if link.Type == record.Answers && link.Target == address {
				return true
			}
		}
	}
	return false
}

func sameUnit(d reconcile.Draft, old *record.Record, binding *resolve.Projection, links []record.Relation) bool {
	if d.Kind != old.Kind || d.Subject != old.Subject || d.Family != old.Family ||
		d.Statement != old.Statement || d.Intent != old.Intent || d.Rationale != old.Rationale ||
		d.FitCriterion != old.FitCriterion || d.Consequence != old.Consequence ||
		d.LiteralBody != old.LiteralBody || !reflect.DeepEqual(d.Provenance, old.Provenance) {
		return false
	}
	tags := slices.Clone(d.Tags)
	sort.Strings(tags)
	oldTags := slices.Clone(old.Tags)
	sort.Strings(oldTags)
	if !slices.Equal(tags, oldTags) {
		return false
	}
	if d.Kind == record.Document && !bytes.Equal(record.NormalizeBody(d.Body), record.NormalizeBody(binding.Set.Bodies[old.ID])) {
		return false
	}
	oldLinks := make([]record.Relation, 0, len(old.Relations))
	for _, link := range old.Relations {
		if link.Type == record.Supersedes {
			if predecessor := binding.Set.Records[link.Target]; predecessor != nil && predecessor.Kind == old.Kind && predecessor.Subject == old.Subject && predecessor.Family == old.Family {
				continue
			}
		}
		oldLinks = append(oldLinks, link)
	}
	less := func(a, b record.Relation) int {
		if c := strings.Compare(a.Type, b.Type); c != 0 {
			return c
		}
		return strings.Compare(a.Target, b.Target)
	}
	slices.SortFunc(links, less)
	slices.SortFunc(oldLinks, less)
	return slices.Equal(links, oldLinks)
}
