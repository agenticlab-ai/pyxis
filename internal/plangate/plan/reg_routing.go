package plan

import (
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
)

func init() { RegisterTask(TaskCheck{Name: "routing", Run: checkRouting}) }

func checkRouting(e *Env, t *Task, bad Bad) {
	if e.Doc.Schema == NativeSchemaVersion {
		if t.Requirements == nil {
			bad(t.ID, "task requires complete requirements", "provide Requirements for implementation and review")
			return
		}
		if err := t.Requirements.Validate(taskrequirements.Sources(t.Traces)); err != nil {
			bad(t.ID, err.Error(), "provide complete assessments and matching derived profiles")
		}
		return
	}
}
