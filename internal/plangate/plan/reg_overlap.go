package plan

import (
	"fmt"
	"strings"
)

// overlap — FR-11: two tasks whose Touches intersect must be graph-ordered or
// carry an explicit Overlap accepted. This is the plan-layer safety net the
// distributed-check-registration design leans on for genuinely shared surfaces.
func init() { RegisterDoc(DocCheck{Name: "overlap", Run: checkOverlap}) }

func checkOverlap(e *Env, bad Bad) {
	tasks := e.Doc.Tasks
	reach := e.Reach
	norm := func(p string) string { return strings.TrimSuffix(p, "/") }
	overlaps := func(a, b string) bool {
		a, b = norm(a), norm(b)
		return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
	}
	for i := range tasks {
		for j := i + 1; j < len(tasks); j++ {
			a, b := &tasks[i], &tasks[j]
			if a.OverlapAccepted != "" || b.OverlapAccepted != "" {
				continue
			}
			if reach[a.ID][b.ID] || reach[b.ID][a.ID] {
				continue // ordered — collisions serialize
			}
			for _, ta := range a.Touches {
				for _, tb := range b.Touches {
					if overlaps(ta, tb) {
						bad("overlap", fmt.Sprintf("%s and %s touch %q as parallel siblings", a.ID, b.ID, norm(ta)),
							"sequence them (Blocked by), insert a shared prep task, or record `Overlap accepted:` (PRD FR-11)")
					}
				}
			}
		}
	}
}
