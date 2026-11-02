// The proof pass (plan-viewer-v1 §6 A-1/A-3/A-8, plan T5): the viewer proven
// whole. Three standing surfaces beyond the per-region tests —
//
//   - a byte-deterministic end-to-end golden over a maximal fixture plan: one
//     plan source exercising every region (all three foundation levels and the
//     edge set, every charter field and badge, covered and deferred coverage
//     rows, size and overlap acceptance flags, hostile markup) is folded and
//     rendered, two renders are byte-identical, and the bytes match the
//     committed golden — so any silent drift in the composed page fails here
//     (R2). The golden inlines a small deterministic stub for the diagram
//     engine so it stays legible and does not churn on a mermaid re-vendor
//     (NR-2); the real 3.5 MB bundle's self-containment is proven separately.
//   - the asset audit (zero external references) and the write audit (exactly
//     one file created, none modified) run over the full maximal page with the
//     real inlined engine (A-1/DINV-3, DINV-1).
//   - a real plan in the repository (docs/plans/plan-viewer) rendered from its
//     own committed bytes to a self-contained page (FR-1/FR-6) — the live
//     demonstration made into a standing, hermetic fixture.
package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// update regenerates the committed golden after an intentional page change:
//
//	go test ./internal/plangate/render/ -run TestDeterministicGolden -update
var update = flag.Bool("update", false, "regenerate the committed render golden")

// goldenStubBundle stands in for the pinned mermaid bytes in the golden so the
// committed page stays small and legible and does not churn when the real
// bundle is re-vendored (NR-2). It is inlined verbatim, so it also proves the
// script-safe inlining path over deterministic input.
const goldenStubBundle = "/* mermaid stub — the real 3.5 MB bundle is inlined at runtime */\n"

// maximalPlan is one green plan source that exercises every region of the page:
// all three foundation levels (T1 root, T2 internal, T3/T4/T5 leaves) and a
// multi-edge DAG; every behavior-charter field and badge (T1) and the
// mechanical shape (T5); covered coverage rows mapping multiple tasks plus a
// deferred Should row; a size acceptance and two overlap acceptances feeding the
// flags panel; and hostile markup in the title. Adding a rendered field to the
// model without wiring the template changes these bytes and fails the golden.
const maximalPlan = `# Widget capability & <b>maximal</b> plan 001

| Field | Value |
|---|---|
| **Feature** | widget |
| **PRD** | docs/prds/widget |
| **Design** | docs/tech-design/widget-design.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Widget capability v1 |
| **Status** | Draft |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Define the widget model

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=2 |
| **Pattern** | — |
| **Touches** | internal/widget/model |

**Goal:** The widget model type exists with closed validation.
**Current behavior:** No widget model exists; callers hold untyped maps.
**Desired behavior:** A typed widget model validates closed and rejects an unknown field with a named defect.

**Scope:**
- the widget model type and its closed validation
- the named defect returned for an unknown field

**Out of scope:**
- persistence of widgets

**Acceptance criteria:**
- [ ] validating a widget with an unknown field yields a defect naming the field
- [ ] validating a well-formed widget returns no defect

**Key interfaces:**
- ` + "`Widget`" + ` — the model type callers construct

**Traces:** prd:widget/FR-1 · design:widget-design#3.1

### T2 — Serve widget lookups

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 80–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | shares the serve surface with the reindex batch; both writers run under one lock |
| **Pattern** | — |
| **Touches** | internal/widget/serve |

**Goal:** Lookups return the stored widget or a named miss.
**Current behavior:** No lookup surface exists.
**Desired behavior:** A lookup returns the widget when present and a named miss when absent, never an empty success.

**Scope:**
- the lookup behavior over stored widgets
- the named-miss result

**Out of scope:**
- caching of lookup results

**Acceptance criteria:**
- [ ] a lookup for a stored widget returns it
- [ ] a lookup miss names the widget it did not find

**Key interfaces:**
- ` + "`Lookup`" + ` — the read surface

**Traces:** prd:widget/FR-2 · corpus:SCAF-1

### T3 — Query the widget index

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 100–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Pattern** | — |
| **Touches** | internal/widget/query |

**Goal:** A miss reports the queried key so the caller can distinguish absence from error.
**Current behavior:** A miss returns an empty result indistinguishable from an error.
**Desired behavior:** A query miss names the key it did not find and reports absence as a first-class outcome.

**Scope:**
- the query behavior over the index
- the named-miss outcome distinct from an error

**Out of scope:**
- ranking of query results

**Acceptance criteria:**
- [ ] a query miss names the key it did not find
- [ ] a query hit returns the stored widget

**Key interfaces:**
- ` + "`Query`" + ` — the index read surface

**Traces:** prd:widget/UB-1

### T4 — Reindex widgets in one batch

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 200–400 |
| **Mechanical** | no |
| **Size accepted** | the reindex is one indivisible transaction over the whole store and cannot be split without leaving a half-built index |
| **Overlap accepted** | shares the serve surface with the lookup task; both writers run under one lock |
| **Pattern** | — |
| **Touches** | internal/widget/serve |

**Goal:** The whole widget store rebuilds its index atomically.
**Current behavior:** No reindex exists; a corrupt index has no repair path.
**Desired behavior:** A reindex rebuilds the index in one transaction and either fully replaces the old index or leaves it untouched on failure.

**Scope:**
- the atomic reindex over the whole store
- the all-or-nothing replacement on failure

**Out of scope:**
- incremental reindexing

**Acceptance criteria:**
- [ ] a completed reindex fully replaces the prior index
- [ ] a reindex that fails partway leaves the prior index untouched

**Key interfaces:**
- ` + "`Reindex`" + ` — the atomic rebuild surface

**Traces:** prd:widget/FR-2 · design:widget-design#3.1

### T5 — Vendor the widget schema bundle

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Pattern** | — |
| **Touches** | vendor/widget |

**Goal:** The pinned upstream widget schema bundle lands unchanged.

**Provenance:**
- upstream tag v1.2.3 with its published checksum recorded beside the bundle

**Acceptance criteria:**
- [ ] the recorded checksum matches the vendored bundle

**Traces:** prd:widget/FR-2

## Coverage

| Requirement | Tasks |
|---|---|
| prd:widget/FR-1 | T1 |
| prd:widget/FR-2 | T2, T4, T5 |
| prd:widget/UB-1 | T3 |

## Deferred

| Requirement | Reason |
|---|---|
| prd:widget/FR-3 | summaries have no consumer yet; next plan |
`

// renderMaximal folds and renders the maximal fixture with the given bundle,
// returning the page bytes.
func renderMaximal(t *testing.T, bundle []byte) (root, page string) {
	t.Helper()
	root = writeFixtureBundle(t, maximalPlan, true, bundle)
	r := Render(root, config.Defaults(), "docs/plans/widget", "", "")
	if len(r.Defects) > 0 {
		t.Fatalf("maximal plan refused: %v", r.Defects)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(r.OutPath)))
	if err != nil {
		t.Fatal(err)
	}
	return root, string(b)
}

// The end-to-end golden: the maximal fixture renders byte-deterministically —
// two renders of one plan state are identical — and the bytes match the
// committed golden (A-1/A-3). The stub engine keeps the golden legible.
func TestDeterministicGolden(t *testing.T) {
	_, page := renderMaximal(t, []byte(goldenStubBundle))

	// Two renders of one plan state are byte-identical (DINV-1).
	_, again := renderMaximal(t, []byte(goldenStubBundle))
	if page != again {
		t.Fatal("two renders of one plan state differ — render is not deterministic")
	}

	golden := filepath.Join("testdata", "maximal-plan.golden.html")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(page), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("golden missing (run: go test ./internal/plangate/render/ -run TestDeterministicGolden -update): %v", err)
	}
	if page != string(want) {
		t.Error("rendered page differs from the committed golden " +
			"(run -update after an intentional page change)")
	}
}

// The asset and write audits over the full maximal page with the real inlined
// engine: zero external references (A-1/DINV-3) and exactly one file created,
// none modified (DINV-1) — the standing audits, now over the whole page.
func TestMaximalPageAudits(t *testing.T) {
	// Write audit: exactly one file created, none modified, across the render.
	root := writeFixture(t, maximalPlan, true)
	before := snapshot(t, root)
	r := Render(root, config.Defaults(), "docs/plans/widget", "", "")
	if len(r.Defects) > 0 {
		t.Fatalf("maximal plan refused: %v", r.Defects)
	}
	created, modified := diff(before, snapshot(t, root))
	if len(created) != 1 || created[0] != r.OutPath {
		t.Errorf("write audit: created %v, want exactly [%s]", created, r.OutPath)
	}
	if len(modified) != 0 {
		t.Errorf("write audit: modified %v, want none", modified)
	}

	// Asset audit: the full page opens offline with zero external references.
	page, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(r.OutPath)))
	if err != nil {
		t.Fatal(err)
	}
	assertSelfContained(t, string(page))
}

// assertSelfContained runs the zero-external-reference audit over a full page:
// inline styling, only inline scripts, the bundle's embedded element closers
// neutralized, and no network-causing reference in the HTML/CSS shell (the
// inlined engine's JS string literals are not asset references, so scripts are
// stripped before the shell audit). Shared by the green- and maximal-fixture
// asset audits so the standing audit has one definition.
func assertSelfContained(t *testing.T, page string) {
	t.Helper()
	if !strings.Contains(page, "<style>") {
		t.Error("styling is not inline")
	}
	if externalScriptRe.MatchString(page) {
		t.Error("page carries an external <script src=…>")
	}
	if n := strings.Count(page, "<script"); n != 2 {
		t.Errorf("expected exactly 2 inline <script> tags (bundle + init), got %d", n)
	}
	if n := strings.Count(page, "</script>"); n != 2 {
		t.Errorf("expected exactly 2 </script> closers, got %d (bundle broke out?)", n)
	}
	if n := strings.Count(page, "</body>"); n != 1 {
		t.Errorf("expected exactly one </body>, got %d (bundle broke out?)", n)
	}
	if n := strings.Count(page, "</html>"); n != 1 {
		t.Errorf("expected exactly one </html>, got %d (bundle broke out?)", n)
	}
	shell := stripScriptBodies(page)
	for _, bad := range []string{
		`src="http`, `src='http`, `src="//`, `href="http`, `href='http`,
		`<link`, `url(http`, `url(//`, `@import`,
	} {
		if strings.Contains(shell, bad) {
			t.Errorf("page shell carries a network-causing reference %q", bad)
		}
	}
}

// A real plan in the repository renders to a self-contained page (FR-1/FR-6):
// the live demonstration made into a hermetic fixture. The real plan-viewer
// plan's own committed bytes — source, machine face, its governing PRD
// projection and design, and the real mermaid bundle — are copied into a temp
// root and rendered; the page must open offline. Guards the demo path against
// regression without writing into the working tree.
func TestRealRepoPlanRenders(t *testing.T) {
	repo := repoRoot(t)
	root := t.TempDir()
	for _, rel := range []string{
		"docs/prds/plan-viewer/effective.source.json",
		"docs/prds/plan-viewer/effective.md",
		"docs/tech-design/plan-viewer-v1.md",
		"docs/plans/plan-viewer/001-plan.md",
		"docs/plans/plan-viewer/001-plan.json",
		VendorMermaid,
	} {
		copyFile(t, filepath.Join(repo, filepath.FromSlash(rel)),
			filepath.Join(root, filepath.FromSlash(rel)))
	}
	if err := os.MkdirAll(filepath.Join(root, ScratchDir), 0o755); err != nil {
		t.Fatal(err)
	}

	r := Render(root, config.Defaults(), "docs/plans/plan-viewer", "", "")
	if len(r.Defects) > 0 {
		t.Fatalf("the real plan-viewer plan refused to render: %v", r.Defects)
	}
	page, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(r.OutPath)))
	if err != nil {
		t.Fatal(err)
	}
	assertSelfContained(t, string(page))
	// The page carries the real plan's identity, so the fixture cannot pass on
	// an empty or wrong render.
	if !strings.Contains(string(page), "plan-viewer") {
		t.Error("the rendered real plan does not carry its feature identity")
	}
}

// TestMaximalPlanFoldsGreen fails loudly with the fold defects if the maximal
// fixture ever stops folding green — so a golden regeneration cannot silently
// bake a red plan's absent page.
func TestMaximalPlanFoldsGreen(t *testing.T) {
	root := writeFixtureBundle(t, maximalPlan, false, []byte(goldenStubBundle))
	res := plan.FoldFile(root, config.Defaults(), planRel)
	for _, d := range res.Defects {
		t.Errorf("maximal fixture defect: %s", d)
	}
	if res.Doc == nil || len(res.Doc.Tasks) != 5 {
		t.Fatalf("expected 5 tasks in the maximal fixture, got %+v", res.Doc)
	}
}

// repoRoot walks up from the test's working directory to the module root (the
// directory holding go.mod), so a test can read the real repository's files.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root (go.mod) not found above the test working directory")
		}
		dir = parent
	}
}

// copyFile copies src to dst, creating parent directories.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
