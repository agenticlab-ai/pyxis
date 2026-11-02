package plan

// traces — FR-2/UB-2: every task cites the requirement IDs and design anchors
// it serves, and each reference resolves. Folds the former inline no-traces
// check and the per-reference checkRef loop that lived in Validate.
func init() { RegisterTask(TaskCheck{Name: "traces", Run: checkTraces}) }

func checkTraces(e *Env, t *Task, bad Bad) {
	if len(t.Traces) == 0 {
		bad(t.ID, "no trace references", "every task cites the requirement IDs and design anchors it serves (PRD FR-2, §4.5 row 13)")
	}
	for _, ref := range t.Traces {
		checkRef(ref, t.ID, e.DesignBody, e.Doc.Design, e.Corpus, e.Root, e.Cfg, e.Locked, bad)
	}
}
