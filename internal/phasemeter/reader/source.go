package reader

import (
	"path/filepath"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/efficiency"
)

// SourceFor returns the deterministic artifact+git efficiency.Source
// (design §3.10, T9) for one phase — phaseID is a skill/command name
// (phase.Phase.ID: pyxis's own pipeline stages, .pyxis/pipeline.md) run
// against featureID — by this repo's own documented artifact-path
// convention (CLAUDE.md's Layout table: docs/brainstorms, docs/prds,
// docs/reviews, docs/plans).
//
// A phase kind this convention cannot resolve to an unambiguous path —
// including tech-design-review, whose review report is named after the
// *design's* slug, which is not always featureID (this repo's own
// pipeline-measurement-v2 is one instance) and so cannot be recovered from
// featureID alone — returns a bare Source (RepoRoot only). efficiency.Compute
// reads that as no signal, never a fabricated or misattributed one — the
// same "artifact absent" honesty the efficiency package already guarantees
// for a single missing path, extended here to "convention unknown."
func SourceFor(repoRoot, featureID, phaseID string) efficiency.Source {
	switch phaseID {
	case "feature-brainstorm":
		p := filepath.Join("docs/brainstorms", featureID+".md")
		return efficiency.Source{RepoRoot: repoRoot, AnchorPaths: []string{p}, LatencyPaths: []string{p}}

	case "tech-design-brainstorm":
		p := filepath.Join("docs/brainstorms", featureID+"-tech-design.md")
		return efficiency.Source{RepoRoot: repoRoot, AnchorPaths: []string{p}, LatencyPaths: []string{p}}

	case "prd-new", "prd-amend":
		dir := filepath.Join("docs/prds", featureID)
		return efficiency.Source{RepoRoot: repoRoot, AmendmentDir: dir, LatencyPaths: []string{dir}}

	case "prd-review":
		// docs/prds/<feature>/'s review reports are named "<feature>-NNN.md"
		// (prd-review-artifacts.md) — the digit-anchored glob matches only
		// that exact scheme, so a feature whose id prefixes another feature's
		// (e.g. "argus" vs "argus-boundary-enforcement", both real features
		// in this repo) can never pull in the other's reviews.
		reviews := glob(repoRoot, filepath.Join("docs/reviews", featureID+"-[0-9][0-9][0-9].md"))
		return efficiency.Source{RepoRoot: repoRoot, ReviewPaths: reviews, LatencyPaths: reviews}

	case "tech-design-new":
		p := filepath.Join("docs/tech-design", featureID+".md")
		return efficiency.Source{RepoRoot: repoRoot, LatencyPaths: []string{p}}

	case "plan-new":
		plans := glob(repoRoot, filepath.Join("docs/plans", featureID, "*-plan.md"))
		var planPath string
		if len(plans) > 0 {
			planPath = plans[len(plans)-1] // the highest-numbered — the current — plan
		}
		return efficiency.Source{RepoRoot: repoRoot, PlanPath: planPath, LatencyPaths: plans}

	default:
		return efficiency.Source{RepoRoot: repoRoot}
	}
}

// glob returns pattern's matches under repoRoot, as paths relative to
// repoRoot (efficiency.Source's own contract: "All paths are relative to
// RepoRoot"), sorted for determinism. No match, or a malformed pattern,
// returns nil — a corpus artifact absent is no signal, never an error
// (mirrors efficiency's own readOptional philosophy).
func glob(repoRoot, pattern string) []string {
	matches, _ := filepath.Glob(filepath.Join(repoRoot, pattern))
	rel := make([]string, 0, len(matches))
	for _, m := range matches {
		r, err := filepath.Rel(repoRoot, m)
		if err != nil {
			continue
		}
		rel = append(rel, r)
	}
	sort.Strings(rel)
	return rel
}
