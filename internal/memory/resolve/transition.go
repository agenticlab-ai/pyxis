package resolve

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// HistoricalTransitionViolations refuses newly effective edges carried by
// immutable historical records. A snapshot alone cannot establish this: the
// caller supplies the trusted before state and separately enforces immutability.
// Compare honored edges, not vanished diagnostics; an edge whose rejection
// reason changed is still rejected. Retiring the carrier is an explicit repair.
func HistoricalTransitionViolations(before, after *Projection) []Violation {
	if before == nil || after == nil || before.Set == nil || after.Set == nil {
		return []Violation{{Code: "historical-input-missing", What: "historical transition requires both trusted before and candidate after projections"}}
	}
	var out []Violation
	for _, id := range before.Set.IDs() {
		old := before.Set.Records[id]
		next := after.Set.Records[id]
		// Supersession preserves transitive effects from its carrier. Only an
		// explicit revocation retires those effects.
		if next == nil || after.Status(id) == Revoked || before.Set.Paths[id] != after.Set.Paths[id] ||
			!bytes.Equal(before.Set.Bytes[before.Set.Paths[id]], after.Set.Bytes[after.Set.Paths[id]]) {
			continue
		}
		for _, edge := range old.Relations {
			if before.Set.Get(edge.Target) != nil && after.Status(id) != Active {
				continue // Existing rejected acts may be explicitly superseded.
			}
			if before.relationHonored(id, edge.Type, edge.Target) || !after.relationHonored(id, edge.Type, edge.Target) {
				continue
			}
			reason := "dangling-target"
			related := []string{edge.Target}
			for _, prior := range before.Violations {
				if prior.Record == id && prior.Target == edge.Target {
					reason = prior.Code
					related = append(related, prior.Related...)
					break
				}
			}
			related = transitionPrerequisites(before, after, related)
			out = append(out, Violation{
				Code: "historical-act-reinterpreted", Record: id, Target: edge.Target, Related: related,
				What: fmt.Sprintf("%s carried a rejected %s edge to %s (%s) in the trusted history, but this change would make it effective; related prerequisites: %s; retire the invalid carrier and issue a fresh act after reconciling its prerequisites", id, edge.Type, edge.Target, reason, strings.Join(related, ", ")),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Record != b.Record {
			return a.Record < b.Record
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.What < b.What
	})
	return out
}

// Include candidate acts on the rejected edge's prerequisites, including
// supersession chains, so the refusal names the new input as well as history.
func transitionPrerequisites(before, after *Projection, initial []string) []string {
	related := map[string]bool{}
	for _, id := range initial {
		related[id] = true
	}
	for changed := true; changed; {
		changed = false
		for _, id := range after.Set.IDs() {
			if before.Set.Records[id] != nil || related[id] {
				continue
			}
			for _, edge := range after.Set.Records[id].Relations {
				if related[edge.Target] && (edge.Type == record.Supersedes || edge.Type == record.Revokes) {
					related[id] = true
					changed = true
					break
				}
			}
		}
	}
	out := make([]string, 0, len(related))
	for id := range related {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
