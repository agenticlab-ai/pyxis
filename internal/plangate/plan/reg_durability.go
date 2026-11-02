package plan

import (
	"fmt"
	"strings"
)

// durability — FR-13: charters describe behavior/interfaces, never paths, line
// numbers, or banned vagueness (all go stale or are undecidable).
func init() { RegisterTask(TaskCheck{Name: "durability", Run: checkDurability}) }

func checkDurability(_ *Env, t *Task, bad Bad) {
	regions := []struct{ name, text string }{
		{"Goal", t.Goal}, {"Current behavior", t.Current}, {"Desired behavior", t.Desired},
		{"Scope", strings.Join(t.Scope, "\n")}, {"Out of scope", strings.Join(t.OutOfScope, "\n")},
		{"Acceptance criteria", strings.Join(t.Acceptance, "\n")}, {"Key interfaces", strings.Join(t.KeyInterfaces, "\n")},
	}
	for _, r := range regions {
		for _, line := range strings.Split(r.text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") || urlishRe.MatchString(line) {
				continue
			}
			if pathExtRe.MatchString(line) || deepPathRe.MatchString(line) || lineRefRe.MatchString(line) {
				bad(t.ID, fmt.Sprintf("%s references a file path or line number: %q", r.name, strings.TrimSpace(line)),
					"charters describe behavior and interfaces — paths and line numbers go stale (PRD FR-13); plan-internal surfaces belong in Touches")
			}
		}
	}
	for _, ac := range t.Acceptance {
		if m := vagueRe.FindString(ac); m != "" {
			bad(t.ID, fmt.Sprintf("acceptance criterion carries banned vagueness %q: %q", m, ac),
				"criteria are independently decidable — running X yields Y (prd-template Appendix A)")
		}
	}
}
