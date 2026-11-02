package gate

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "verify-index",
		Help:           "byte-compare committed index vs recomputed",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return VerifyIndex(c.Root, c.Cfg) },
	})
}

// VerifyIndex byte-compares the committed index against the recomputed
// one (FR-12; DINV-1 of the v2 design).
func VerifyIndex(root string, cfg config.Config) []schema.Defect {
	md, js, defects := GenIndexBytes(root, cfg)
	if len(defects) > 0 {
		return defects
	}
	dir := filepath.Join(root, filepath.FromSlash(cfg.TechDesigns))
	for name, want := range map[string][]byte{"index.md": md, "index.json": js} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			defects = append(defects, vdef("verify-index", cfg.TechDesigns+"/"+name, "compare",
				"index artifact missing", "run `docgate gen-index` and commit both artifacts"))
			continue
		}
		if !bytes.Equal(got, want) {
			defects = append(defects, vdef("verify-index", cfg.TechDesigns+"/"+name, "compare",
				"committed index differs from the recomputed one",
				"never hand-edit the index: run `docgate gen-index` and recommit"))
		}
	}
	return defects
}
