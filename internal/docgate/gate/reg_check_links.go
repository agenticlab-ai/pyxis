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
		Verb:           "check-links",
		Help:           "relative markdown links resolve (docs, AGENTS.md, templates)",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckLinks(c.Root, c.Cfg, c.Scan) },
	})
}

// CheckLinks is FR-21: every relative markdown link in the docs tree, the
// operating manual, and the templates directory resolves to an existing
// file. Fenced blocks, code spans, external URLs, and pure fragments are
// skipped; fragments on resolving files are ignored in v1
// (doc-link-integrity Q1). No network, no writes (DINV-1/3 there).
func CheckLinks(root string, cfg config.Config, scan ScanScope) []schema.Defect {
	// The whole docs tree is in scan (skipReviews=false): a broken relative link
	// is a defect wherever it appears, reviewer commentary included.
	files := corpusScanFiles(root, cfg, scan, false)
	var defects []schema.Defect
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		relSrc, _ := filepath.Rel(root, path)
		fenced := false
		for i, l := range strings.Split(string(schema.Normalize(b)), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "```") {
				fenced = !fenced
				continue
			}
			if fenced {
				continue
			}
			for _, target := range linkTargets(stripCodeSpans(l)) {
				if isExternal(target) || strings.HasPrefix(target, "#") {
					continue
				}
				file := target
				if k := strings.Index(file, "#"); k >= 0 {
					file = file[:k]
				}
				if file == "" {
					continue
				}
				resolved := filepath.Join(filepath.Dir(path), filepath.FromSlash(file))
				if _, err := os.Stat(resolved); err != nil {
					defects = append(defects, vdef("check-links", filepath.ToSlash(relSrc), "link",
						fmt.Sprintf("line %d: target %q does not resolve (tried %s)", i+1, target, filepath.ToSlash(resolved)),
						"fix or remove the link; targets must exist at their relative path"))
				}
			}
		}
	}
	return defects
}
