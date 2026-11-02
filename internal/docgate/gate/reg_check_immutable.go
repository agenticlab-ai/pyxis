package gate

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "check-immutable",
		Help:           "merged ledger entries are add-only vs --merge-base",
		NeedsMergeBase: true,
		Run:            func(c Ctx) Defects { return CheckImmutable(c.Root, c.MergeBase, c.Head, c.Cfg) },
	})
}

func CheckImmutable(root, mergeBase, head string, cfg config.Config) []schema.Defect {
	args := append([]string{"-C", root, "diff", "--name-status"}, diffSpan(mergeBase, head)...)
	args = append(args, "--", filepath.FromSlash(cfg.PRDs), filepath.FromSlash(cfg.TechDesigns))
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return []schema.Defect{def("check-immutable", cfg.PRDs, "git",
			fmt.Sprintf("git diff against %q failed: %v", mergeBase, err),
			"pass a resolvable --merge-base ref (e.g. origin/main)")}
	}
	var defects []schema.Defect
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		status := parts[0]
		path := parts[len(parts)-1]
		base := filepath.Base(path)
		if status[0] != 'M' && status[0] != 'D' && status[0] != 'R' {
			continue
		}
		switch {
		case strings.HasPrefix(path, cfg.PRDs+"/") && entryRe.MatchString(base):
			defects = append(defects, def("check-immutable", path, "history",
				fmt.Sprintf("ledger entry %s is %s relative to the merge base — merged entries are immutable", base, statusWord(status[0])),
				"revert the change; evolve the contract with a new amendment instead"))
		case strings.HasPrefix(path, cfg.TechDesigns+"/") && strings.HasSuffix(base, ".md") && !strings.HasPrefix(base, "index"):
			defects = append(defects, def("check-immutable", path, "history",
				fmt.Sprintf("design %s is %s relative to the merge base — merged designs are immutable (INV-41)", base, statusWord(status[0])),
				"revert the change; supersede with a new design that cites the old"))
		}
	}
	return defects
}
