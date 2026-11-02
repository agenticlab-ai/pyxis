package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// coverage — slice-scoped per-plan coverage (design:plangate-slice-coverage,
// prd:plan-new/FR-26). A plan's fold validates its OWN matrix — every claim is
// live and mapped to a task, every task is mapped, Deferred rows are legal —
// but never demands every live feature Must (DINV-1); feature-level Must
// completeness is the union across the feature's plans (FeatureCoverage,
// DINV-2), never a per-plan fold block (DINV-3).
func init() { RegisterDoc(DocCheck{Name: "coverage", Run: checkCoverage}) }

func checkCoverage(e *Env, bad Bad) {
	d := e.Doc
	gov := e.Gov
	if d.Intake != "" {
		if len(d.Coverage) != 0 || len(d.Deferred) != 0 {
			bad("coverage", "non-feature intake has no governing PRD for coverage rows", "omit Coverage and Deferred rows; cite intent and decision in each task's Traces")
		}
		return
	}
	if gov.live == nil {
		return // governing surface unloadable — already a defect
	}
	covered := map[string]bool{}
	deferred := map[string]bool{}
	inCoverage := map[string]bool{}
	for _, row := range d.Coverage {
		m := prdRefRe.FindStringSubmatch(row.Requirement)
		if m == nil {
			bad("coverage", fmt.Sprintf("coverage requirement %q is not a prd:<feature>/<ID> reference", row.Requirement), "the matrix maps the governing PRD's units to tasks (PRD FR-15)")
			continue
		}
		if m[1] != d.Feature {
			bad("coverage", fmt.Sprintf("coverage row %q cites a foreign PRD", row.Requirement), "the matrix covers the plan's own governing PRD; foreign refs belong in task Traces")
			continue
		}
		if covered[row.Requirement] {
			bad("coverage", fmt.Sprintf("duplicate coverage row %q", row.Requirement), "one row per requirement")
		}
		covered[row.Requirement] = true
		if !gov.live[m[2]] {
			bad("coverage", fmt.Sprintf("coverage requirement %q is not a live unit of %s", row.Requirement, d.PRD), "cite live units of the governing projection (PRD FR-2)")
		}
		if len(row.Tasks) == 0 {
			bad("coverage", fmt.Sprintf("requirement %q maps to no task", row.Requirement), "an unmapped requirement is fog — all or nothing (PRD FR-16)")
		}
		for _, id := range row.Tasks {
			if d.Task(id) == nil {
				bad("coverage", fmt.Sprintf("coverage row %q names unknown task %q", row.Requirement, id), "coverage cites plan task IDs")
			}
			inCoverage[id] = true
		}
	}
	// Deferred rows are legal decisions: never a Must, always with a reason.
	for _, row := range d.Deferred {
		m := prdRefRe.FindStringSubmatch(row.Requirement)
		if m == nil || m[1] != d.Feature {
			bad("deferred", fmt.Sprintf("deferred requirement %q is not a unit of the governing PRD", row.Requirement), "deferral rows cite prd:<feature>/<ID>")
			continue
		}
		if gov.must[m[2]] {
			bad("deferred", fmt.Sprintf("Must requirement %q deferred", row.Requirement), "Must rows cannot defer — feature-level completeness is checked over the union of the feature's plans (PRD FR-26)")
		}
		if !gov.live[m[2]] {
			bad("deferred", fmt.Sprintf("deferred requirement %q is not a live unit at the plan's governing version", row.Requirement), "defer only requirements active at the plan's Basis")
		}
		if deferred[row.Requirement] || covered[row.Requirement] {
			bad("deferred", fmt.Sprintf("requirement %q is repeated or both covered and deferred", row.Requirement), "give each requirement one coverage or deferral decision")
		}
		deferred[row.Requirement] = true
		if strings.TrimSpace(row.Reason) == "" {
			bad("deferred", fmt.Sprintf("deferral of %q carries no reason", row.Requirement), "deferrals are recorded decisions")
		}
	}
	if e.Locked != nil {
		var missing []string
		for id := range e.Locked.requirements {
			ref := "prd:" + d.Feature + "/" + id
			if !covered[ref] && !deferred[ref] {
				missing = append(missing, id)
			}
		}
		sort.Strings(missing)
		for _, id := range missing {
			bad("coverage", fmt.Sprintf("locked requirement %s is neither covered nor deferred", id), "map it to a task or defer it with a reason (feature-draft FR-15)")
		}
	}
	// DINV-1: coverage is checked against THIS plan's matrix (its slice), never
	// against every live feature Must — that is FeatureCoverage's job (DINV-2).
	// A rogue task no requirement asked for is still a defect.
	for i := range d.Tasks {
		if !inCoverage[d.Tasks[i].ID] {
			bad(d.Tasks[i].ID, "task appears in no coverage row", "a task no requirement asked for is rogue — map it or remove it (PRD FR-15)")
		}
	}
}

// FeatureCoverage unions the coverage matrices of every plan under a feature
// and reports each live Must FR/UB that no plan claims (DINV-2, prd:plan-new/
// FR-26). It is feature-scoped and independent of any single plan's fold
// (DINV-3), so a slice-plan or a frozen base plan folds green while this
// surfaces the feature-level gaps.
func FeatureCoverage(root string, cfg config.Config, feature string) []schema.Defect {
	prdDir := filepath.ToSlash(filepath.Join(cfg.PRDs, feature))
	planDir := filepath.ToSlash(filepath.Join(cfg.Plans, feature))
	files, err := Files(root, planDir)
	if err != nil {
		return []schema.Defect{def(planDir, "coverage", fmt.Sprintf("cannot enumerate the feature's plans: %v", err), "the feature's plans live under the plans root")}
	}
	if len(files) > 0 {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(files[0])))
		if err != nil {
			return []schema.Defect{def(files[0], "coverage", fmt.Sprintf("cannot read plan: %v", err), "repair the plan source")}
		}
		first, parsed := Parse(files[0], body)
		if len(parsed) != 0 {
			return parsed
		}
		if first != nil && strings.HasPrefix(first.PRD, "prd/") {
			// Draft-born plans each bind a possibly different lock. Their fold
			// checks every requirement at that Basis, so a union against today's
			// feature state would invalidate older, already delivered plans.
			var defects []schema.Defect
			for _, rel := range files {
				defects = append(defects, FoldFile(root, cfg, rel).Defects...)
			}
			return defects
		}
	}
	gov, gdef := loadGoverning(root, prdDir, planDir)
	if len(gdef) > 0 {
		return gdef
	}
	claimed := map[string]bool{}
	for _, rel := range files {
		res := FoldFile(root, cfg, rel)
		for _, row := range res.Doc.Coverage {
			if m := prdRefRe.FindStringSubmatch(row.Requirement); m != nil && m[1] == feature {
				claimed[m[2]] = true // the UNION of every plan's slice
			}
		}
	}
	var missing []string
	for id := range gov.must {
		if !claimed[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return []schema.Defect{def(planDir, "coverage",
			fmt.Sprintf("feature Must requirements covered by no plan: %s", strings.Join(missing, ", ")),
			"add a plan whose matrix covers each — feature-level completeness is the union across the feature's plans (PRD FR-26)")}
	}
	return nil
}
