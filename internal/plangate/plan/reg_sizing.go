package plan

import "fmt"

// sizing — FR-5/FR-6 sizing thresholds (a recorded Size accepted opts out).
func init() { RegisterTask(TaskCheck{Name: "sizing", Run: checkSizing}) }

func checkSizing(e *Env, t *Task, bad Bad) {
	if t.SizeAccepted != "" {
		return // oversize is a recorded decision (PRD FR-5)
	}
	s := e.Cfg.Plan
	over := func(what string, got, max int) {
		bad(t.ID, fmt.Sprintf("%s %d exceeds the threshold %d", what, got, max),
			"split the task, or record `Size accepted: <why unsplittable>` (PRD FR-5; thresholds: .pyxis/config.yaml plan:)")
	}
	if n := len(t.Acceptance); n > s.MaxAcceptanceCriteria {
		over("acceptance-criteria count", n, s.MaxAcceptanceCriteria)
	}
	if n := len(t.Scope); n > s.MaxScopeBullets {
		over("scope-bullet count", n, s.MaxScopeBullets)
	}
	if t.Band.Hi > s.MaxEstLogicLines {
		over("size-band upper bound", t.Band.Hi, s.MaxEstLogicLines)
	}
}
