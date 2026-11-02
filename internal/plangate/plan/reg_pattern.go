package plan

import "fmt"

// pattern — FR-8 expand–contract shape: one expand, one+ migrate-batch, one
// contract, wired by the graph.
func init() { RegisterDoc(DocCheck{Name: "pattern", Run: checkPattern}) }

func checkPattern(e *Env, bad Bad) {
	tasks := e.Doc.Tasks
	reach := e.Reach
	var expand, contract []string
	var migrate []string
	for i := range tasks {
		switch tasks[i].Pattern {
		case PatternExpand:
			expand = append(expand, tasks[i].ID)
		case PatternMigrate:
			migrate = append(migrate, tasks[i].ID)
		case PatternContract:
			contract = append(contract, tasks[i].ID)
		}
	}
	if len(expand)+len(migrate)+len(contract) == 0 {
		return
	}
	if len(expand) != 1 || len(migrate) == 0 || len(contract) != 1 {
		bad("pattern", fmt.Sprintf("expand–contract shape is %d expand / %d migrate-batch / %d contract", len(expand), len(migrate), len(contract)),
			"one expand, one or more migrate-batches, one contract (PRD FR-8)")
		return
	}
	for _, m := range migrate {
		if !reach[m][expand[0]] {
			bad(m, "migrate-batch not blocked by the expand task", "every batch runs after expand — wire the edge (PRD FR-8)")
		}
	}
	for _, m := range migrate {
		if !reach[contract[0]][m] {
			bad(contract[0], fmt.Sprintf("contract not blocked by migrate-batch %s", m), "contract runs after every batch — wire the edge (PRD FR-8)")
		}
	}
}
