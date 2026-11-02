package gate

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "check-standalone-docs",
		Help:           "a governed doc (PRD/design/incident) ships without code in document-pipeline repositories",
		NeedsMergeBase: true,
		Run: func(c Ctx) Defects {
			// Pyxis is moving its own contracts into memory. The operator retired
			// the one-document-class-per-PR rule there; installed repositories
			// still use this check while they follow the document pipeline.
			if pyxis, err := pyxisAtMergeBase(c.Root, c.MergeBase); err == nil && pyxis {
				return nil
			}
			return CheckStandaloneDocs(c.Root, c.MergeBase, c.Head, c.Cfg)
		},
	})
}

// CheckStandaloneDocs fails a diff that bundles a governed contract class — a
// PRD ledger entry, an authored technical design, or the incident registry —
// with implementation/capability paths, or with a second governed class
// (FR-30). Each governed doc ships as its own PR; implementation stacks on the
// approved contract. Generated projections (effective.*, index.*) ride with
// their source, and rollups (techspec.md/prd.md) ride with code — both exempt.
func CheckStandaloneDocs(root, mergeBase, head string, cfg config.Config) []schema.Defect {
	out, err := exec.Command("git", append([]string{"-C", root, "diff", "--name-only", "-z"}, diffSpan(mergeBase, head)...)...).Output()
	if err != nil {
		return []schema.Defect{def("check-standalone-docs", "docs", "git",
			fmt.Sprintf("git diff against %q failed: %v", mergeBase, err),
			"pass a resolvable --merge-base ref (e.g. origin/main)")}
	}
	prdsRoot := filepath.Base(cfg.PRDs)
	tdPrefix := filepath.FromSlash(cfg.TechDesigns) + "/"
	classes := map[string]bool{}
	var code []string
	for path := range strings.SplitSeq(strings.TrimRight(string(out), "\x00"), "\x00") {
		if path == "" {
			continue
		}
		base := filepath.Base(path)
		switch {
		// Generated projections + live rollups ride along — exempt (first).
		case base == "effective.md" || base == "effective.source.json":
		case path == cfg.RollupPRD || path == cfg.RollupTechspec:
		case strings.HasPrefix(path, tdPrefix) && strings.HasPrefix(base, "index."):
		// Governed contract classes. The PRD-ledger match is anchored to the
		// real PRDs root, not merely any grandparent dir named like it, so a
		// test fixture that mimics the layout under a nested path
		// (…/testdata/…/docs/prds/demo/000-base.md) is not misread as a
		// governed ledger entry bundled with code.
		case entryRe.MatchString(base) && strings.HasPrefix(path, filepath.FromSlash(cfg.PRDs)+"/") && filepath.Base(filepath.Dir(filepath.Dir(path))) == prdsRoot:
			classes["PRD ledger"] = true
		case strings.HasPrefix(path, tdPrefix) && strings.HasSuffix(base, ".md"):
			classes["technical design"] = true
		case path == cfg.Incidents:
			classes["incident registry"] = true
		// Implementation / capability paths.
		case hasAnyPrefix(path, cfg.CapabilityPaths):
			code = append(code, path)
		}
	}
	if len(classes) == 0 {
		return nil // pure code / rollup / non-governed change
	}
	names := make([]string, 0, len(classes))
	for c := range classes {
		names = append(names, c)
	}
	sort.Strings(names)
	if len(names) > 1 {
		return []schema.Defect{def("check-standalone-docs", "docs", "diff",
			fmt.Sprintf("PR bundles %d governed doc classes (%s) — each ships as its own PR", len(names), strings.Join(names, ", ")),
			"split into one PR per class, stacked: contract → design → implementation")}
	}
	if len(code) > 0 {
		sort.Strings(code)
		shown := strings.Join(code, ", ")
		if len(code) > 3 {
			shown = strings.Join(code[:3], ", ") + fmt.Sprintf(", +%d more", len(code)-3)
		}
		return []schema.Defect{def("check-standalone-docs", "docs", "diff",
			fmt.Sprintf("PR bundles a %s change with implementation code (%s) — the doc ships standalone", names[0], shown),
			"land the contract/design PR first, then stack the implementation on it")}
	}
	return nil
}
