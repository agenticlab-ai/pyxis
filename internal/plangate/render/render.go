// Package render is plangate's read-side viewer (plan-viewer-v1). This is the
// render spine (plan T1): it folds a plan in memory and, when green, composes
// one self-contained HTML page skeleton — a header region (feature, version,
// dialect, status, totals) and a fingerprint footer, inline-styled and
// template-escaped — written to the review loop's gitignored scratch. It
// writes exactly one file and nothing else (DINV-1); a fold-red plan, a stale
// or missing machine face, a bad version, or a missing directory each refuse
// with the named quadruple and no page (DINV-2). The page renders the
// schema-versioned contract, never a re-parse of prose (AP-6). Read-only: no
// tracker, no merged-plan check — drafts on open PRs are the primary input.
// The page carries the header, footer, and the dependency-graph region — a
// mermaid diagram drawn by the pinned vendored bundle inlined at render time
// (a missing bundle refuses); the task-card, coverage, and flags regions
// extend it in later tasks.
package render

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// ScratchDir is the gitignored home for standalone plan previews; the
// default page can be opened directly in a browser.
const ScratchDir = ".pyxis/scratch"

// versionRe matches a dense plan version token (NNN) for --version selection.
var versionRe = regexp.MustCompile(`^\d+$`)

// def builds this package's uniform defect (fail closed, name the fix).
func def(file, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "plangate", File: file, Op: "render", Reason: reason, Remedy: remedy}
}

// Result is one render's outcome: the page's repo-relative path once written,
// or the defects that refused it. OutPath is non-empty iff a file was written.
type Result struct {
	OutPath string
	Defects []schema.Defect
}

// Render folds the named (default: highest) plan version in memory and, if it
// is green and its committed machine face is current, writes one self-contained
// page. Any of those failing refuses with a named defect and writes nothing
// (UB-1/UB-2/FR-7). out overrides the default
// .pyxis/scratch/plan-<feature>-<version>.html location.
func Render(root string, cfg config.Config, dir, version, out string) Result {
	rel, defects := selectVersion(root, dir, version)
	if len(defects) > 0 {
		return Result{Defects: defects}
	}

	// Fold in memory: fold-red yields no page, the defects printed as `fold`
	// prints them (FR-7/DINV-2). A page's existence therefore certifies a
	// green fold at render time (AP-13).
	res := plan.FoldFile(root, cfg, rel)
	if len(res.Defects) > 0 {
		return Result{Defects: res.Defects}
	}

	// The committed machine face must match the recomputed fold: a stale or
	// missing face refuses with the re-fold remedy (UB-2), so the reviewer is
	// never shown a page built from a source whose committed twin has drifted.
	jsRel := plan.JSONPath(rel)
	committed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(jsRel)))
	if err != nil {
		return Result{Defects: []schema.Defect{def(jsRel,
			"committed machine face missing", "run `plangate fold` and commit both faces before rendering")}}
	}
	if !bytes.Equal(committed, res.JSON) {
		return Result{Defects: []schema.Defect{def(jsRel,
			"committed machine face differs from the recomputed fold",
			"never hand-edit the .json: re-run `plangate fold` and recommit, then render")}}
	}

	// The diagram engine is the pinned vendored mermaid bundle, inlined so the
	// page draws its graph offline (DINV-3). A missing bundle fails closed with
	// the re-vendor remedy — never a silently graphless page (R4).
	mermaidJS, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(VendorMermaid)))
	if err != nil {
		return Result{Defects: []schema.Defect{def(VendorMermaid,
			"pinned mermaid bundle unreadable — the page inlines it to draw the dependency graph",
			"restore the vendored bundle with `bash .pyxis/vendor/mermaid/refresh.sh`")}}
	}

	page, err := Page(res.Doc, mermaidJS)
	if err != nil {
		return Result{Defects: []schema.Defect{def(rel, err.Error(),
			"report this — a green fold must always render")}}
	}

	outRel := out
	if outRel == "" {
		outRel = DefaultOut(res.Doc)
	}
	outAbs := filepath.Join(root, filepath.FromSlash(outRel))
	if err := os.MkdirAll(filepath.Dir(outAbs), 0o755); err != nil {
		return Result{Defects: []schema.Defect{def(outRel, err.Error(),
			"ensure the output directory is writable")}}
	}
	if err := os.WriteFile(outAbs, page, 0o644); err != nil {
		return Result{Defects: []schema.Defect{def(outRel, err.Error(),
			"ensure the output directory is writable")}}
	}
	return Result{OutPath: outRel}
}

// DefaultOut is the scratch page path for a folded plan (bucket 3).
func DefaultOut(doc *plan.Doc) string {
	return ScratchDir + "/plan-" + doc.Name() + "-" + doc.Version + ".html"
}

// selectVersion resolves dir + optional NNN to the plan source's repo-relative
// path. A missing/empty directory or an unknown version refuses with the named
// quadruple and no page (UB-1).
func selectVersion(root, dir, version string) (string, []schema.Defect) {
	files, err := plan.Files(root, dir)
	if err != nil {
		return "", []schema.Defect{def(dir, "plan directory unreadable",
			"point render at an existing plan directory (e.g. docs/plans/<feature>)")}
	}
	if len(files) == 0 {
		return "", []schema.Defect{def(dir, "no NNN-plan.md sources in the plan directory",
			"fold a plan into this directory first, or point render at a populated one")}
	}
	if version == "" {
		return files[len(files)-1], nil // highest is the default
	}
	if !versionRe.MatchString(version) {
		return "", []schema.Defect{def(dir, fmt.Sprintf("version %q is not a dense NNN token", version),
			"pass --version as a zero-padded number (e.g. --version 001)")}
	}
	want := dir + "/" + version + "-plan.md"
	for _, f := range files {
		if f == want {
			return f, nil
		}
	}
	return "", []schema.Defect{def(want, fmt.Sprintf("plan version %q does not exist in %s", version, dir),
		"render an existing version, or omit --version to render the highest")}
}
