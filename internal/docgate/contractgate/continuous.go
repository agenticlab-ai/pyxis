package contractgate

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

// The continuous contract-doc-PR surface (prd:contract-gate/FR-9; design
// contract-gate-v1 §3.10): on a PR that touches contract documents — a feature's
// PRD ledger or a technical design — run the contract-scope subset over EVERY
// affected feature's contract under the contract-doc-pr trigger, so drift
// surfaces at introduction (currency/link/structural blocking, inverse-coverage
// advisory — a PRD legitimately merges before its serving designs exist). It is
// the SAME gate the planning-entry precondition runs (AP-11), differing only in
// the trigger's severity split; the CI context (`.github/workflows/`) invokes it.
//
// The affected-feature derivation maps the changed-file set to the feature(s)
// whose contract those files belong to:
//   - a changed PRD-ledger file `<prds>/<feature>/...`      → feature <feature>
//   - a changed technical design `<tech_designs>/<slug>.md` → every feature it
//     SERVES (its `Serves prd:<feature>/...` targets), because a design change
//     can invalidate the currency/coverage of the features it serves.
//
// A design maps to a feature via its resolved Serves — read from the SAME design
// graph the gate resolves (techdesigns.LiveGraph over gate.LoadDesigns), never a
// second Serves parser (DINV-3, NR-2). The derivation is read-only.

// Continuous runs the contract-scope gate under the contract-doc-pr trigger over
// every feature affected by changedFiles, returning one ContinuousResult. It
// loads config + the severity map once (fail-closed on a malformed config or
// severity-map, UB-6) and the design graph once (to map changed designs to
// served features), then gates each affected feature. Read-only on every path
// (DINV-1): only file reads and pure computation.
//
// changedFiles are repo-relative paths (forward-slash), e.g. from a CI
// `git diff --name-only <base>...<head>`. A path outside the contract corpora
// (not a PRD ledger file, not a design) contributes no affected feature — the
// context is a no-op for a PR that touches no contract doc.
func Continuous(root string, changedFiles []string) ContinuousResult {
	cfg, cdef := config.Load(root)
	if len(cdef) > 0 {
		return ContinuousResult{ConfigError: "the pyxis config does not parse: " + cdef[0].Reason}
	}
	sev, sdef := LoadSeverityMap(root)
	if len(sdef) > 0 {
		return ContinuousResult{ConfigError: "the severity-map policy does not parse: " + sdef[0].Reason}
	}

	features := affectedFeatures(root, cfg, changedFiles)
	res := ContinuousResult{Trigger: TriggerContractDocPR, Features: features}
	for _, f := range features {
		v := runResolved(root, f, TriggerContractDocPR, cfg, sev)
		res.Verdicts = append(res.Verdicts, v)
		if v.Blocked() {
			res.Blocking = true
		}
	}
	return res
}

// ContinuousResult is the closed outcome of a continuous run over a PR's changed
// files: the trigger it applied, the affected features it derived, one verdict per
// feature, and whether ANY of them blocked (the exit-code driver). A malformed
// config or severity-map short-circuits into ConfigError (the whole context fails
// closed, UB-6) — no per-feature verdicts are produced in that case.
type ContinuousResult struct {
	// Trigger is always contract-doc-pr for a continuous run (the surface's split).
	Trigger Trigger `json:"trigger"`
	// Features are the affected feature slugs the changed files derived, sorted.
	Features []string `json:"features"`
	// Verdicts is one verdict per affected feature, in Features order.
	Verdicts []Verdict `json:"verdicts"`
	// Blocking is true iff any feature's verdict blocked — the context fails (exit
	// 1) iff this is true. An advisory-only bounce (e.g. an uncovered Must) leaves
	// it false: the defect is surfaced but the PR is not blocked (FR-9).
	Blocking bool `json:"blocking"`
	// ConfigError names a fail-closed config/severity-map defect (UB-6); non-empty
	// means the whole context failed closed with no per-feature verdicts.
	ConfigError string `json:"config_error,omitempty"`
}

// Failed reports whether the continuous context should fail the PR (exit 1): a
// fail-closed config/severity-map error, or any blocking per-feature verdict. An
// empty affected-feature set (a PR touching no contract doc) is a clean pass.
func (r ContinuousResult) Failed() bool {
	return r.ConfigError != "" || r.Blocking
}

// affectedFeatures derives the sorted, de-duplicated feature set the changed
// files belong to: a changed PRD-ledger file names its feature directly; a
// changed design names every feature it serves. Read-only.
func affectedFeatures(root string, cfg config.Config, changedFiles []string) []string {
	prdsRoot := filepath.ToSlash(filepath.Clean(cfg.PRDs))
	designRoot := filepath.ToSlash(filepath.Clean(cfg.TechDesigns))

	set := map[string]bool{}
	var changedDesignSlugs []string
	for _, raw := range changedFiles {
		p := filepath.ToSlash(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		p = filepath.ToSlash(filepath.Clean(p))
		// A PRD-ledger file: <prds>/<feature>/<...>. The immediate child directory
		// under the PRD root is the feature slug.
		if rest, ok := underDir(p, prdsRoot); ok {
			if feature, _, hasChild := strings.Cut(rest, "/"); hasChild && feature != "" {
				set[feature] = true
			}
			continue
		}
		// A technical design: <tech_designs>/<slug>.md (never the generated index).
		if rest, ok := underDir(p, designRoot); ok {
			if !strings.Contains(rest, "/") && strings.HasSuffix(rest, ".md") && !strings.HasPrefix(rest, "index.") {
				changedDesignSlugs = append(changedDesignSlugs, strings.TrimSuffix(rest, ".md"))
			}
		}
	}

	// Map each changed design slug to the feature(s) it serves, via the SAME
	// resolved design graph the gate uses (never a second Serves parser). A design
	// that fails to parse or is absent simply contributes no feature here — the
	// per-feature gate run (or lint-tech-designs) is where a broken design reds.
	if len(changedDesignSlugs) > 0 {
		for feature := range servedFeaturesOf(root, cfg, changedDesignSlugs) {
			set[feature] = true
		}
	}

	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// underDir reports whether path p is inside directory dir, returning the portion
// of p relative to dir. Exact-dir and sibling-prefix matches (e.g. "docs/prdsX")
// are rejected — only a true child counts.
func underDir(p, dir string) (rest string, ok bool) {
	if dir == "." || dir == "" {
		return p, true
	}
	if !strings.HasPrefix(p, dir+"/") {
		return "", false
	}
	return strings.TrimPrefix(p, dir+"/"), true
}

// servedFeaturesOf returns the set of feature slugs served by any of the given
// design slugs, read from the resolved live design graph (techdesigns.LiveGraph
// over gate.LoadDesigns — the SAME resolution the gate performs). Each design's
// resolved Serves refs are `prd:<feature>/<anchor>`; the feature is the segment
// between `prd:` and the first `/`. Read-only; a graph-resolution defect yields an
// empty set (the per-feature gate is the fail-closed authority, not this map).
func servedFeaturesOf(root string, cfg config.Config, slugs []string) map[string]bool {
	want := map[string]bool{}
	for _, s := range slugs {
		want[s] = true
	}
	features := map[string]bool{}

	designs, ddef := gate.LoadDesigns(root, cfg)
	if len(ddef) > 0 {
		// A design directory that cannot be read at all leaves the design→feature
		// map empty; the PR's own PRD-ledger changes (if any) still gate.
		return features
	}
	entries, _, gdef := techdesigns.LiveGraph(designs, designEnv(root, cfg))
	if len(gdef) > 0 {
		return features
	}
	for slug, e := range entries {
		if !want[slug] {
			continue
		}
		for _, ref := range e.Serves {
			if f, ok := featureOfServesRef(ref); ok {
				features[f] = true
			}
		}
	}
	return features
}

// featureOfServesRef extracts the feature slug from a `prd:<feature>/<anchor>`
// Serves ref. A non-prd ref (doc:, design:, corpus:) serves no PRD feature and
// yields ok=false.
func featureOfServesRef(ref string) (string, bool) {
	rest, ok := strings.CutPrefix(ref, "prd:")
	if !ok {
		return "", false
	}
	feature, _, hasAnchor := strings.Cut(rest, "/")
	if !hasAnchor || feature == "" {
		return "", false
	}
	return feature, true
}
