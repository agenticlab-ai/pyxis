package gate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "lint-rollups",
		Help:           "rollup metadata headers",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return LintRollups(c.Root, c.Cfg) },
	})
}

// LintRollups is FR-13's rollup half.
//
// Fresh-operand vacuous pass (installer-v1 §3.5 step 4; AGENTS.md rule 3): the
// live rollups (docs/prd.md + docs/techspec.md) do NOT exist until a capability
// actually ships — a freshly `pyxis init`-ed operand has none, and that absence
// is the correct pre-first-ship state, not a defect. When BOTH rollups are
// absent the check passes vacuously: no capability has shipped, so there is no
// rollup metadata to lint. Once EITHER rollup exists (the first capability has
// seeded them) the header contract is enforced on both — a half-present pair is
// an inconsistent, named state, exactly as before. The retired FR-10 per-PR
// guard (amendment 017) read "has a capability shipped" from git history; this
// check reads it from the tree — both rollups present ⇒ shipped, both absent ⇒
// not yet — so the freshness posture holds without a merge base (the green
// proof runs base-free on the seeded tree).
func LintRollups(root string, cfg config.Config) []schema.Defect {
	var defects []schema.Defect
	rollups := []string{cfg.RollupPRD, cfg.RollupTechspec}
	if !anyRollupExists(root, rollups) {
		return nil // pre-first-ship: rollups legitimately absent (rule 3)
	}
	for _, rel := range rollups {
		rows, err := headerRows(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			defects = append(defects, vdef("lint-rollups", rel, "read",
				"rollup missing or unreadable", "rule 3: the rollups exist once capability ships — once either rollup is present both must be, with the metadata header"))
			continue
		}
		for _, f := range []string{"In one line", "Relevant-when", "Authority", "Update rule", "Seeded"} {
			if _, ok := rows[f]; !ok {
				defects = append(defects, vdef("lint-rollups", rel, "header",
					fmt.Sprintf("missing header row %q", f), "rollups carry the metadata header"))
			}
		}
	}
	return defects
}

// anyRollupExists reports whether at least one of the rollup files is present
// on disk — the "a capability has shipped" signal LintRollups keys its
// fresh-operand vacuous pass on (installer-v1 §3.5 step 4). A stat error other
// than not-exist (a permission fault, say) is treated as present, so the header
// contract still runs and surfaces the real read failure by name rather than
// silently vacuous-passing.
func anyRollupExists(root string, rollups []string) bool {
	for _, rel := range rollups {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			return true
		}
	}
	return false
}
