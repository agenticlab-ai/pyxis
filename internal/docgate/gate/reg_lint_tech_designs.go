package gate

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

func init() {
	Register(Check{
		Verb:           "lint-tech-designs",
		Help:           "design header schema, Serves grammar, mandatory sections",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return LintTechDesigns(c.Root, c.MergeBase, c.Head, c.Cfg) },
	})
}

func LintTechDesigns(root, mergeBase, head string, cfg config.Config) []schema.Defect {
	designs, defects := LoadDesigns(root, cfg)
	return append(defects, techdesigns.Lint(designs, designEnv(root, mergeBase, head, cfg))...)
}
