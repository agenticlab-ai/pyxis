package gate

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/shapes"
)

func init() {
	Register(Check{
		Verb:           "check-shape-indexed",
		Help:           "no solution-shape sits in the design corpus/index (DINV-3)",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckShapeIndexedAsDesign(c.Root, c.Cfg) },
	})
}

// CheckShapeIndexedAsDesign is solution-shape #135 (DINV-3, §3.4/§3.8): a
// solution-shape must never reach the technical-design corpus. There LoadDesigns
// would parse it and gen-index would admit it into the design index as a design
// — the shape indexed as a design defect. Index generation stays blind to the
// shapes root (the two corpus roots are config-guaranteed disjoint, config.Parse),
// so this verb guards the other direction and fails closed with a
// shape-indexed-as-design defect naming the misfiled file. The shape's home is
// the solution-shape corpus (paths.solution_shapes).
func CheckShapeIndexedAsDesign(root string, cfg config.Config) []schema.Defect {
	dir := filepath.Join(root, filepath.FromSlash(cfg.TechDesigns))
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil // absent corpus — LoadDesigns already reports the read defect
	}
	var defects []schema.Defect
	for _, it := range items {
		name := it.Name()
		if it.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "index.") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue // a read failure is LoadDesigns's to report
		}
		if shapes.LooksLikeShape(b) {
			defects = append(defects, vdef("shape-indexed-as-design", cfg.TechDesigns+"/"+name, "corpus",
				"a solution-shape sits in the technical-design corpus — gen-index would admit it to the design index as a design (DINV-3)",
				"move the shape to the solution-shape corpus (paths.solution_shapes); a shape is never a design and is never indexed"))
		}
	}
	return defects
}
