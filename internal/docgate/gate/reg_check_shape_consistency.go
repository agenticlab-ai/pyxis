package gate

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

func init() {
	Register(Check{
		Verb:           "check-shape-consistency",
		Help:           "witness/trace/completeness of a design vs its shape (DINV-4)",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckShapeConsistency(c.Root, c.Cfg) },
	})
}

// CheckShapeConsistency is solution-shape T4 (DINV-4, §3.8): the witness /
// trace / completeness cross-check between each design's binding markup and
// its paired shape — an unwitnessed DINV-n, an untraced shape element, or a
// stateful design missing its shape. Load-parse defects are the design and
// shape lints' to report; this verb reports only the cross-check defects.
func CheckShapeConsistency(root string, cfg config.Config) []schema.Defect {
	designs, _ := LoadDesigns(root, cfg)
	shps, _ := LoadShapes(root, cfg)
	return techdesigns.ShapeConsistency(designs, shps, cfg.SolutionShapes)
}
