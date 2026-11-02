package contractgate

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// link resolution — traceability made real (prd:contract-gate/FR-4; design
// contract-gate-v1 §3.9, class `unresolved`): every typed cross-document
// citation the feature's own contract depends on — the PRD ledger's and its
// serving designs' `prd:`/`design:`/`corpus:`/`brainstorm:` references and every
// `Serves` target — must resolve to a live-or-superseded anchor. A dangling
// reference is a silent hole in the contract; this check fails naming each
// dangling citation and its source location.
//
// It is a REUSE of the citation resolver (gate.CheckCitationsInFiles),
// feature-scoped in the verb: it runs the SAME resolver the tree-wide
// check-citations runs — same grammar, same universes, same code-span/fence
// exemption, same grandfather skip — over just this feature's own document files
// (its ledger directory + its serving designs), scoping the resolver to the
// contract set without altering its global behavior (prd:contract-gate/NR-3).
// The tree-wide check-citations behavior for its existing callers is
// byte-unchanged (it is the whole-corpus caller of the same core).
func init() {
	Register(Check{
		Name:          "link",
		Class:         ClassUnresolved,
		SeverityClass: SeverityClassLink,
		Run:           checkLink,
	})
}

// checkLink resolves every typed citation in the feature's own documents and
// reports each dangling one, naming its file+line. The document set is the
// feature's PRD ledger `.md` files (base + amendments + the effective
// projection, where citations and Serves live) plus each live serving design's
// `.md`. Deterministic: the file list is sorted and CheckCitationsInFiles walks
// it in order, and each defect names its own file+line (DINV-5). Read-only over
// the resolved contract and the committed document files (DINV-1).
func checkLink(c *contract) Defects {
	files := c.featureDocuments()
	var defects []schema.Defect
	for _, d := range gate.CheckCitationsInFiles(c.Root, c.Cfg, files) {
		// Re-attribute the shared resolver's defect to the contract-scope `link`
		// check while preserving the offender + source location the resolver
		// named (file in d.File, line + citation in d.Reason).
		defects = append(defects, schema.Defect{
			Verb: "contract-gate", File: d.File, Op: "link",
			Reason: "in " + d.File + ", " + d.Reason,
			Remedy: d.Remedy,
		})
	}
	return defects
}

// featureDocuments is the feature's own document set the link check scopes the
// citation resolver to: every `.md` under the feature's PRD ledger directory and
// each live serving design's `.md`. Absolute paths (CheckCitationsInFiles reads
// them and derives repo-relative names); sorted for deterministic scan order.
func (c *contract) featureDocuments() []string {
	var files []string
	ledgerDir := filepath.Join(c.Root, filepath.FromSlash(c.Cfg.PRDs), c.Feature)
	if items, err := os.ReadDir(ledgerDir); err == nil {
		for _, it := range items {
			if !it.IsDir() && filepath.Ext(it.Name()) == ".md" {
				files = append(files, filepath.Join(ledgerDir, it.Name()))
			}
		}
	}
	for _, sd := range c.Serving {
		files = append(files, filepath.Join(c.Root, filepath.FromSlash(c.Cfg.TechDesigns), sd.Slug+".md"))
	}
	sort.Strings(files)
	return files
}
