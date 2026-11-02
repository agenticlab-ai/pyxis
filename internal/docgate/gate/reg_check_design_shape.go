package gate

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

func init() {
	Register(Check{
		Verb:           "check-design-shape",
		Help:           "a design body carries no concrete solution-shape (DINV-1)",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckDesignCarriesShape(c.Root, c.Cfg) },
	})
}

// CheckDesignCarriesShape is solution-shape T3 (DINV-1, §3.8): every
// technical-design body is free of concrete solution-shape content — a
// frozen design references its shape, never contains it. Load-parse defects
// are LintTechDesigns's to report; this verb reports only the shape defects.
func CheckDesignCarriesShape(root string, cfg config.Config) []schema.Defect {
	designs, _ := LoadDesigns(root, cfg)
	return techdesigns.CarriesShape(designs)
}
