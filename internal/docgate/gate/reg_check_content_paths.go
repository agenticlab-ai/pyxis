package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "check-content-paths",
		Help:           "content cites version pins, never .pyxis paths (FR-31)",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckContentPaths(c.Root, c.MergeBase, c.Head, c.Cfg) },
	})
}

// CheckContentPaths is FR-31 (repo-layout A-2/DINV-4 made real): content
// cites templates by version pin, never a system-footprint path — wiki
// entries and the flat corpus always; ledger entries and designs only
// when added since the merge base (frozen history is never retro-edited).
// Rollups, the root manual, and generated artifacts are out of scope.
func CheckContentPaths(root, mergeBase, head string, cfg config.Config) []schema.Defect {
	var files []string
	wiki := filepath.Join(root, filepath.FromSlash(cfg.Wiki))
	if items, err := os.ReadDir(wiki); err == nil {
		for _, it := range items {
			if !it.IsDir() && strings.HasSuffix(it.Name(), ".md") && !strings.EqualFold(it.Name(), "README.md") {
				files = append(files, filepath.Join(wiki, it.Name()))
			}
		}
	}
	prds := filepath.Join(root, filepath.FromSlash(cfg.PRDs))
	if items, err := os.ReadDir(prds); err == nil {
		for _, it := range items {
			if !it.IsDir() && strings.HasSuffix(it.Name(), ".md") {
				files = append(files, filepath.Join(prds, it.Name()))
			}
		}
	}
	if mergeBase != "" {
		args := append([]string{"-C", root, "diff", "--name-status"}, diffSpan(mergeBase, head)...)
		args = append(args, "--", filepath.FromSlash(cfg.PRDs), filepath.FromSlash(cfg.TechDesigns))
		out, err := exec.Command("git", args...).Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				parts := strings.Fields(line)
				if len(parts) < 2 || parts[0][0] != 'A' {
					continue
				}
				p := parts[len(parts)-1]
				base := filepath.Base(p)
				inLedger := strings.HasPrefix(p, cfg.PRDs+"/") && entryRe.MatchString(base)
				isDesign := strings.HasPrefix(p, cfg.TechDesigns+"/") && strings.HasSuffix(base, ".md") && !strings.HasPrefix(base, "index")
				if inLedger || isDesign {
					files = append(files, filepath.Join(root, filepath.FromSlash(p)))
				}
			}
		}
	}
	sort.Strings(files)
	var defects []schema.Defect
	seen := map[string]bool{}
	for _, path := range files {
		if seen[path] {
			continue
		}
		seen[path] = true
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, path)
		for i, l := range strings.Split(string(schema.Normalize(b)), "\n") {
			if strings.Contains(l, ".pyxis/") {
				defects = append(defects, vdef("check-content-paths", filepath.ToSlash(rel), "purity",
					fmt.Sprintf("line %d: system-footprint path reference — content cites version pins, never .pyxis paths", i+1),
					"cite the template by version pin; the root manual owns the footprint pointers (repo-layout A-2)"))
			}
		}
	}
	return defects
}
