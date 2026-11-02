package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "lint-wiki",
		Help:           "wiki routing headers",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return LintWiki(c.Root, c.Cfg) },
	})
}

// LintWiki is FR-13's wiki half. The wiki-entry corpus root resolves from
// config (config-driven-portability-v1 §3.2, FR-1); its default is docs/wiki.
func LintWiki(root string, cfg config.Config) []schema.Defect {
	var defects []schema.Defect
	wiki := filepath.ToSlash(filepath.Clean(cfg.Wiki))
	dir := filepath.Join(root, filepath.FromSlash(cfg.Wiki))
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil // no wiki, nothing to lint
	}
	for _, it := range items {
		name := it.Name()
		if it.IsDir() || !strings.HasSuffix(name, ".md") || strings.EqualFold(name, "README.md") {
			continue
		}
		rel := wiki + "/" + name
		rows, err := headerRows(filepath.Join(dir, name))
		if err != nil {
			defects = append(defects, vdef("lint-wiki", rel, "read", err.Error(), "the entry must be readable"))
			continue
		}
		for _, f := range []string{"In one line", "Relevant-when", "Authority", "Sources", "Last reviewed"} {
			if _, ok := rows[f]; !ok {
				defects = append(defects, vdef("lint-wiki", rel, "header",
					fmt.Sprintf("missing header row %q", f), "wiki entries carry the routing header (wiki README)"))
			}
		}
		if v, ok := rows["Last reviewed"]; ok && !dateRe.MatchString(v) {
			defects = append(defects, vdef("lint-wiki", rel, "header",
				"Last reviewed carries no YYYY-MM-DD date", "date the review"))
		}
		if v, ok := rows["Authority"]; ok && !strings.Contains(strings.ToLower(v), "descriptive") {
			defects = append(defects, vdef("lint-wiki", rel, "header",
				"Authority row does not declare the entry descriptive", "the wiki is never authoritative"))
		}
	}
	return defects
}
