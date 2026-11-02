package checks

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// MachineryProtectionCheckID is this check's registry and finding CheckID.
const MachineryProtectionCheckID = "machinery-protection"

// AcquisitionPinRelPath is the operand acquisition config's fixed,
// workspace-relative location (design docs/tech-design/repo-layout-and-
// system-footprint.md §3.6) — the file carrying the pinned-artifact
// sections (today `docgate:`, additively `codegate:` per design
// codegate-v1 §3.1). codegate protects the whole file as a codegate-known
// constant rather than parsing its YAML — the design review's disposition
// (docs/reviews/codegate-v1.md FND-11): a downgraded pin is gate-gaming
// regardless of which section moved.
const AcquisitionPinRelPath = ".pyxis/config.yaml"

func init() {
	register(schema.Check{
		ID:                MachineryProtectionCheckID,
		Rung:              "0",
		AuthorIndependent: true,
		InputSet:          []string{"diff"},
	}, adaptEnvCheck(MachineryProtectionCheckID, runMachineryProtection))
}

// MachineryPaths derives rung 0's protected-path set — never a hand-kept
// list (design codegate-v1 §3.7): the manifest's own fixed location, its
// declared baseline path (when the manifest declares one), the
// acquisition-pin file, and the verdict artifact's path (belt-and-braces for
// DINV-2 — design §3.6). A nil manifest still yields the three
// constant-derived paths.
func MachineryPaths(manifest *schema.Manifest) map[string]bool {
	paths := map[string]bool{
		invoke.ManifestRelPath: true,
		AcquisitionPinRelPath:  true,
		invoke.VerdictRelPath:  true,
	}
	if manifest != nil {
		if bp := strings.TrimSpace(manifest.BaselinePath); bp != "" {
			// Normalize to the workspace-relative forward-slash form git reports
			// in TouchedPaths(), so however the manifest spells the path —
			// "./docs/baseline.json", a Windows "docs\baseline.json", or an
			// absolute "/docs/baseline.json" (which resolveBaselineHash still
			// reads workspace-relative) — it exact-matches and cannot read
			// correctly yet silently escape protection.
			paths[normalizeRepoPath(bp)] = true
		}
	}
	return paths
}

// normalizeRepoPath folds a path to the workspace-relative, forward-slash form
// git reports in a diff: Windows backslashes become '/', "./" and ".." are
// cleaned, and a leading '/' is stripped. Applied to both the protected set and
// the touched-path lookup so membership matches regardless of spelling.
func normalizeRepoPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "/")
	return p
}

// runMachineryProtection reds when the proposal diff touches any path in
// MachineryPaths — added, modified, deleted, or renamed either into or out
// of it (DiffSet.TouchedPaths covers all four; a rename either direction is
// gate-gaming). "The implementer must not adjudicate its own gate" (basis
// FR-8's rationale): this check is what makes that structural.
func runMachineryProtection(ctx context.Context, env *Env) (*schema.CheckResult, []schema.Finding, error) {
	start := time.Now()
	if env.Diff == nil {
		return nil, nil, fmt.Errorf("%s: no diff set provided", MachineryProtectionCheckID)
	}

	protected := MachineryPaths(env.Manifest)

	var findings []schema.Finding
	for _, p := range env.Diff.TouchedPaths() {
		if !protected[normalizeRepoPath(p)] {
			continue
		}
		findings = append(findings, schema.Finding{
			CheckID: MachineryProtectionCheckID,
			Class:   "machinery",
			Rung:    "0",
			Path:    p,
			Message: "the proposal diff touches verification machinery",
		})
	}

	result := &schema.CheckResult{
		CheckID:  MachineryProtectionCheckID,
		State:    "pass",
		Duration: schema.DurationMS(time.Since(start)),
	}
	if len(findings) > 0 {
		result.State = "fail"
		result.Message = fmt.Sprintf("%d machinery path(s) touched by the proposal diff", len(findings))
	}
	return result, findings, nil
}
