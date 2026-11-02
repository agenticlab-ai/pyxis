package plan

import "fmt"

// share — the EPIC-SHAPE lint: one task swallowing a multi-task plan (FR-5).
func init() { RegisterDoc(DocCheck{Name: "share", Run: checkShare}) }

func checkShare(e *Env, bad Bad) {
	tasks := e.Doc.Tasks
	s := e.Cfg.Plan
	total, behavior := 0, 0
	for i := range tasks {
		if !tasks[i].Mechanical {
			total += tasks[i].Band.Mid()
			behavior++
		}
	}
	// The measured pathology is one task swallowing a multi-task plan
	// (predecessor E6); a pair splitting 60/40 is legitimate, so the gate
	// arms at three or more behavior tasks.
	if behavior < 3 || total == 0 {
		return
	}
	for i := range tasks {
		t := &tasks[i]
		if t.Mechanical || t.SizeAccepted != "" {
			continue
		}
		if t.Band.Mid()*100 > s.MaxTaskSharePct*total {
			bad(t.ID, fmt.Sprintf("task holds %d%% of the plan's estimated size (max %d%%)", t.Band.Mid()*100/total, s.MaxTaskSharePct),
				"one task swallowing its plan is the measured decomposition failure — re-split, or record `Size accepted:` (PRD FR-5)")
		}
	}
}
