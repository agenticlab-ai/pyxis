package readiness

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

// checkNumbering reads the complete binding history. A number cannot be
// reused even when its old record was retired; a row ID can continue only
// while its prior record is still active at that address.
func checkNumbering(result compile.Result, binding *resolve.Projection) []Unmet {
	if binding == nil {
		return nil
	}
	feature := result.Feature
	var unmet []Unmet
	for _, group := range []struct {
		subject, prefix, path string
	}{
		{"brainstorm/" + feature, "record-", "brainstorm"},
		{"review/" + feature, "report-", "reviews"},
	} {
		used, next := usedNumbers(binding, group.subject, group.prefix)
		for _, entry := range result.Records {
			if entry.Draft.Kind != record.Document || entry.Draft.Subject != group.subject {
				continue
			}
			number, ok := strings.CutPrefix(entry.Draft.Family, group.prefix)
			if !ok || !used[number] {
				continue
			}
			unmet = append(unmet, Unmet{"UB-7", group.path + "/" + number + ".md", fmt.Sprintf("number %s was used by an earlier lock; use %s/%03d.md", number, group.path, next)})
		}
	}
	used, next := usedNumbers(binding, "prd/"+feature, "P-")
	for _, problem := range result.Problems {
		number := strings.TrimPrefix(problem.ID, "P-")
		if used[number] {
			unmet = append(unmet, Unmet{"UB-7", "problems.md#" + problem.ID, fmt.Sprintf("problem number %s was used by an earlier lock; use P-%d", problem.ID, next)})
		}
	}
	for _, entry := range result.Records {
		d := entry.Draft
		if d.Kind == record.Document || d.Kind == record.Answer || d.Family == "" || d.Subject != "prd/"+feature && d.Subject != "design/"+feature || strings.HasPrefix(d.Family, "P-") {
			continue
		}
		seen := false
		active := false
		for _, old := range binding.Set.BySubject[d.Subject] {
			if old.Family != d.Family || old.Kind == record.Revocation {
				continue
			}
			seen = true
			if binding.Status(old.ID) == resolve.Active {
				active = true
			}
		}
		if seen && !active {
			unmet = append(unmet, Unmet{"UB-7", d.Subject + "#" + d.Family, "row ID was retired by an earlier lock; choose a new ID"})
		}
	}
	return unmet
}

func usedNumbers(binding *resolve.Projection, subject, prefix string) (map[string]bool, int) {
	used := map[string]bool{}
	maximum := 0
	for _, old := range binding.Set.BySubject[subject] {
		if old.Kind != record.Document && (old.Kind != record.Question || prefix != "P-") {
			continue
		}
		raw, ok := strings.CutPrefix(old.Family, prefix)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			continue
		}
		used[raw] = true
		maximum = max(maximum, n)
	}
	return used, maximum + 1
}
