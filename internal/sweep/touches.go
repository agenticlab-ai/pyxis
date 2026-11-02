// This file owns the touches-underdelivery detector (maintenance-sweep-v1
// §3.2, prd FR-9). The charter under-delivery rot class is mechanized: for
// each frozen plan's committed machine face (the plan dialect's JSON, the read
// contract), every task's Touches cell is split on the house separator and only
// path-like tokens are evaluated; a token naming no file and no directory in the
// evaluated state becomes one advisory finding citing plan and task — phrased as
// a reconciliation signal, never a breach verdict (R3). A plan waiver suppresses
// that plan's findings and surfaces its reason (the in-flight-plan control,
// prd FR-10); a non-path token is skipped, never interpreted; an unparsable
// plan face is a named in-band skip and the detector continues over the rest
// (prd UB-4). The detector is a pure function of (evaluated state, policy):
// it reads the tree and the policy and writes nothing (DINV-1). Stdlib plus the
// plan package that owns the machine-face shape.
package sweep

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// touchesPlansDir is the repo-relative home of frozen plan machine faces
// (prd §4.1: docs/plans/*/NNN-plan.json). A committed face under here is
// frozen; drafts live on open PR branches, never in the evaluated tree.
const touchesPlansDir = "docs/plans"

// touchesSep is the house separator joining surface tokens within one Touches
// cell — the ` · ` middot the plans are authored with. plangate keeps a
// middot-joined cell as a single Touches element (its splitList cuts on commas
// only), so the detector re-splits here on the middot the authors used.
const touchesSep = "·"

// DetectTouchesUnderdelivery is the touches-underdelivery detector: a pure
// function of (evaluated state, policy). root is the evaluated repository tree;
// pol carries the plan waivers. It reads every frozen plan face under
// docs/plans, evaluates each task's path-like Touches tokens against the tree,
// and returns one DetectorResult carrying the advisory findings, the applied
// plan waivers with their reasons, and — in the Fault field — any unparsable
// plan named as an in-band skip. It performs no write and consults nothing but
// the tree and the policy (DINV-1).
func DetectTouchesUnderdelivery(root string, pol Policy) DetectorResult {
	res := DetectorResult{Name: DetectorTouchesUnderdelivery}

	faces, err := touchesPlanFaces(root)
	if err != nil {
		// The plans home is unreadable — a named skip for the whole detector,
		// not a masquerade of cleanliness (DINV-3/DINV-4). An absent home is
		// the legal empty state: no plans, no findings.
		res.Fault = fmt.Sprintf("touches-underdelivery: %v", err)
		return res
	}

	// Index the plan waivers by the plan id they name, for O(1) suppression.
	waived := make(map[string]string, len(pol.PlanWaivers))
	for _, w := range pol.PlanWaivers {
		waived[w.Plan] = w.Reason
	}

	var skips []string
	for _, rel := range faces {
		doc, perr := touchesReadFace(root, rel)
		if perr != nil {
			// An unparsable plan face is a named in-band skip; the detector
			// continues over the rest (prd UB-4).
			skips = append(skips, fmt.Sprintf("%s (%v)", rel, perr))
			continue
		}

		planID := touchesPlanID(doc)
		if reason, ok := waived[planID]; ok {
			// The plan waiver suppresses this plan's findings and surfaces its
			// reason — suppression is always visible (DINV-5, prd FR-10).
			res.WaiversApplied = append(res.WaiversApplied, AppliedWaiver{
				Kind:    WaiverKindPlan,
				Subject: planID,
				Reason:  reason,
			})
			continue
		}

		for _, task := range doc.Tasks {
			for _, tok := range touchesTokens(task.Touches) {
				if !touchesPathLike(tok) {
					// Non-path tokens carry no mechanical meaning here — skipped,
					// never interpreted (§3.2, a future design's ground).
					continue
				}
				if touchesDelivered(root, tok) {
					continue
				}
				res.Findings = append(res.Findings, touchesFinding(planID, task, tok))
			}
		}
	}

	if len(skips) > 0 {
		res.Fault = "touches-underdelivery: skipped unparsable plan face(s): " + strings.Join(skips, "; ")
	}
	return res
}

// touchesPlanFaces returns the sorted repo-relative paths of every committed
// plan machine face under docs/plans. An absent plans home is the empty set
// (no error); any other read failure is returned so the caller can name it.
func touchesPlanFaces(root string) ([]string, error) {
	home := filepath.Join(root, filepath.FromSlash(touchesPlansDir))
	entries, err := os.ReadDir(home)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", touchesPlansDir, err)
	}
	var faces []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(home, e.Name())
		files, ferr := os.ReadDir(sub)
		if ferr != nil {
			return nil, fmt.Errorf("reading %s/%s: %w", touchesPlansDir, e.Name(), ferr)
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), "-plan.json") {
				continue
			}
			faces = append(faces, touchesPlansDir+"/"+e.Name()+"/"+f.Name())
		}
	}
	sort.Strings(faces)
	return faces, nil
}

// touchesReadFace reads and parses one plan machine face. It is deliberately
// strict: a syntactically malformed file or one not declaring the plan schema
// is unparsable, so a corrupt face becomes a named skip rather than silently
// contributing zero findings (prd UB-4, DINV-3).
func touchesReadFace(root, rel string) (plan.Doc, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return plan.Doc{}, fmt.Errorf("unreadable: %w", err)
	}
	var doc plan.Doc
	if err := json.Unmarshal(b, &doc); err != nil {
		return plan.Doc{}, fmt.Errorf("malformed plan JSON: %w", err)
	}
	if doc.Schema != plan.SchemaVersion {
		return plan.Doc{}, fmt.Errorf("not a %s machine face (schema_version %q)", plan.SchemaVersion, doc.Schema)
	}
	return doc, nil
}

// touchesPlanID is the canonical plan identity a finding cites and a plan
// waiver names: <feature>/<plan_version> (e.g. "maintenance-sweep/001"), the
// prefix the plan's own plan_task_id keys carry.
func touchesPlanID(doc plan.Doc) string {
	return doc.Feature + "/" + doc.Version
}

// touchesTokens flattens a task's Touches cells into individual surface tokens,
// splitting each cell on the house middot and trimming surrounding space.
func touchesTokens(cells []string) []string {
	var out []string
	for _, cell := range cells {
		for _, tok := range strings.Split(cell, touchesSep) {
			if tok = strings.TrimSpace(tok); tok != "" {
				out = append(out, tok)
			}
		}
	}
	return out
}

// touchesPathLike reports whether a token is a path — it contains a slash or a
// dot. Only path-like tokens are evaluated; a bare word like "pipeline-map"
// carries no mechanical surface and is skipped (§3.2).
func touchesPathLike(tok string) bool {
	return strings.ContainsAny(tok, "/.")
}

// touchesDelivered reports whether a path-like token names something present in
// the evaluated state: an existing file, or an existing directory (a directory
// prefix is exactly an existing directory — a file living under token/ makes
// token a directory os.Stat resolves). A missing file whose parent directory
// exists is NOT delivered — the specific surface was never shipped.
func touchesDelivered(root, tok string) bool {
	tok = strings.TrimSuffix(tok, "/")
	if tok == "" {
		return true
	}
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(tok)))
	return err == nil
}

// touchesFinding builds one advisory touches-underdelivery finding. The subject
// is the absent surface; the evidence and remedy cite plan and task and frame
// the signal as a reconciliation prompt — never a breach verdict (R3, prd FR-9).
// The four contracted fields are always present, so NewFinding never refuses;
// the construction is guarded regardless (DINV-5).
func touchesFinding(planID string, task plan.Task, tok string) Finding {
	evidence := fmt.Sprintf(
		"plan %q task %s (%q) lists this surface in its Touches, but no file or directory matches it in the evaluated state",
		planID, task.ID, task.Title)
	remedy := "advisory reconciliation signal, not a breach: ship the surface if it is still intended, " +
		"or amend the plan's Touches if it was renamed or dropped"
	f, err := NewFinding(DetectorTouchesUnderdelivery, tok, evidence, remedy)
	if err != nil {
		// Unreachable — all four fields are non-empty above. Fail closed with a
		// self-describing subject rather than dropping the signal.
		return Finding{
			SchemaVersion: FindingSchemaVersion,
			Detector:      DetectorTouchesUnderdelivery,
			Subject:       tok,
			Evidence:      evidence,
			Remedy:        remedy,
		}
	}
	return f
}
