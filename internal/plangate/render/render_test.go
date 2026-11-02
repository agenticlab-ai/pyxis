// Hermetic render-spine tests (plan-viewer-v1 §3.1/§3.6, plan T1's acceptance):
// a temp repo holds the governing projection, design, corpus, a green plan
// source, and its committed machine face. Each acceptance is asserted over the
// rendered bytes or the refusal; no network, no clock.
package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

const planRel = "docs/plans/widget/001-plan.md"

func TestIntakeViewerNamesThePlanWithoutEmptyFeatureField(t *testing.T) {
	body, err := os.ReadFile("../plan/testdata/header/intake-001-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	doc, defects := plan.Parse("docs/plans/maintenance/001-plan.md", body)
	if len(defects) != 0 {
		t.Fatalf("parse intake: %v", defects)
	}
	page, err := Page(doc, []byte("window.mermaid = {};"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "<dt>Intake</dt><dd>maintenance</dd>") || strings.Contains(string(page), "<dt>Feature</dt><dd></dd>") || !strings.HasSuffix(DefaultOut(doc), "plan-maintenance-001.html") {
		t.Fatal("intake viewer lost its plan identity")
	}
}

// The retained plan viewer owns the browser bundle's pin contract.
func TestVendoredMermaidPin(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	versions, err := os.ReadFile(filepath.Join(root, ".pyxis", "scripts", "versions.env"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^MERMAID_VERSION=(\S+)$`).FindSubmatch(versions)
	if len(match) != 2 {
		t.Fatal("missing Mermaid version pin")
	}
	b, err := os.ReadFile(filepath.Join(root, ".pyxis", "vendor", "mermaid", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.Version != string(match[1]) || !strings.Contains(string(repoBundle(t)), pkg.Version) {
		t.Fatalf("Mermaid package/bundle do not match version pin %s", match[1])
	}
}

const governingSourceJSON = `{
  "schema_version": "prd-effective-source/v1",
  "units": {
    "FR-1": {"status": "live"},
    "FR-2": {"status": "live"},
    "FR-3": {"status": "live"},
    "UB-1": {"status": "live"},
    "NG-1": {"status": "live"}
  }
}`

const governingEffectiveMD = `# Widget — PRD (projection)

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | The widget shall exist. | why | Must | low | fit | Test | J1 |
| FR-2 | The widget shall serve lookups. | why | Must | med | fit | Test | J1 |
| FR-3 | The widget should export summaries. | why | Should | low | fit | Test | J1 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If a lookup misses | say so | a named miss | Must | low | fit | Test | FR-2 |
`

const designDoc = `# Widget design

### 3.1 The model

body

- **DINV-1** — widgets validate closed.
`

const corpusDoc = "# Scaffolding placeholder\n\nSCAF-1 is the baseline scaffold requirement.\n"

// greenPlan folds green under every gate. Its H1 title carries hostile markup
// so the skeleton's template escaping is provable (A-6's escaping half at T1).
const greenPlan = `# Widget & <b>capability</b> <img src=x onerror=alert(1)> — plan 001

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

**Out of scope:**
- persistence of widgets

**Acceptance criteria:**
- [ ] validating a widget with an unknown field yields a defect naming the field

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
| **Overlap accepted** | — |
| **Routing** | — |
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

**Traces:** prd:widget/FR-2 · prd:widget/UB-1 · corpus:SCAF-1

### T3 — Vendor the widget schema bundle

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 0 |
| **Mechanical** | yes |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
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
| prd:widget/FR-2 | T2, T3 |
| prd:widget/UB-1 | T2 |

## Deferred

| Requirement | Reason |
|---|---|
| prd:widget/FR-3 | summaries have no consumer yet; next plan |
`

// writeFixture builds the hermetic repo and returns its root. withFace commits
// the machine face beside the source. The page inlines the repo's real pinned
// mermaid bundle, so self-containment is proven against the true engine bytes.
func writeFixture(t *testing.T, planSource string, withFace bool) string {
	t.Helper()
	return writeFixtureBundle(t, planSource, withFace, repoBundle(t))
}

// writeFixtureBundle is writeFixture with the inlined diagram engine injected —
// the golden suite passes a small deterministic stub so the committed golden
// stays legible and does not churn on a mermaid re-vendor (NR-2), while the
// self-containment audits keep using the real 3.5 MB bundle.
func writeFixtureBundle(t *testing.T, planSource string, withFace bool, bundle []byte) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"docs/prds/widget/effective.source.json":  governingSourceJSON,
		"docs/prds/widget/effective.md":           governingEffectiveMD,
		"docs/prds/scaffolding-and-onboarding.md": corpusDoc,
		"docs/tech-design/widget-design.md":       designDoc,
		planRel:                                   planSource,
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The scratch dir pre-exists so a default render creates exactly one file.
	if err := os.MkdirAll(filepath.Join(root, ScratchDir), 0o755); err != nil {
		t.Fatal(err)
	}
	// The mermaid bundle the page inlines; render fails closed without it.
	bp := filepath.Join(root, filepath.FromSlash(VendorMermaid))
	if err := os.MkdirAll(filepath.Dir(bp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bp, bundle, 0o644); err != nil {
		t.Fatal(err)
	}
	if withFace {
		res := plan.FoldFile(root, config.Defaults(), planRel)
		if len(res.Defects) > 0 {
			t.Fatalf("fixture plan does not fold green: %v", res.Defects)
		}
		jsRel := plan.JSONPath(planRel)
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(jsRel)), res.JSON, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestGreenPlanFoldsGreen(t *testing.T) {
	root := writeFixture(t, greenPlan, false)
	res := plan.FoldFile(root, config.Defaults(), planRel)
	for _, d := range res.Defects {
		t.Errorf("unexpected defect: %s", d)
	}
	if res.Doc == nil || len(res.Doc.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %+v", res.Doc)
	}
}

// renderPage renders the green fixture at the default location and returns
// (root, page string).
func renderPage(t *testing.T) (string, string) {
	t.Helper()
	root := writeFixture(t, greenPlan, true)
	r := Render(root, config.Defaults(), "docs/plans/widget", "", "")
	if len(r.Defects) > 0 {
		t.Fatalf("green plan refused: %v", r.Defects)
	}
	if r.OutPath != ScratchDir+"/plan-widget-001.html" {
		t.Fatalf("default out path = %q", r.OutPath)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(r.OutPath)))
	if err != nil {
		t.Fatal(err)
	}
	return root, string(b)
}

// The header region carries feature, version, dialect, status, and totals; the
// footer carries the source fingerprint and the tool version (T1 acceptance 1).
func TestHeaderAndFooter(t *testing.T) {
	root, page := renderPage(t)
	res := plan.FoldFile(root, config.Defaults(), planRel)
	doc := res.Doc

	header := section(page, `id="header"`, "</section>")
	for _, want := range []string{doc.Feature, doc.Version, doc.Dialect, doc.Status,
		"3 task(s)", "band sum", "largest task"} {
		if !strings.Contains(header, want) {
			t.Errorf("header missing %q", want)
		}
	}

	footer := page[strings.Index(page, `class="footer"`):]
	if doc.Fingerprint == "" || !strings.Contains(footer, doc.Fingerprint) {
		t.Errorf("footer does not carry the source fingerprint %q", doc.Fingerprint)
	}
	if !strings.Contains(footer, plan.ToolVersion) {
		t.Errorf("footer does not carry the tool version %q", plan.ToolVersion)
	}
}

// Self-containment (A-1/DINV-3, T2 acceptance 3): the page opens offline with
// zero external references. Styling is inline; the only scripts are inline (the
// pinned mermaid bundle and its init — no src attribute); the bundle's embedded
// element closers are neutralized (exactly one real </body>/</html>); and an
// asset audit of the HTML/CSS shell — the inlined engine's JS string literals
// are not asset references — finds nothing network-causing.
func TestSelfContainedWithBundle(t *testing.T) {
	_, page := renderPage(t)
	assertSelfContained(t, page)
}

// externalScriptRe matches a <script> that pulls an external resource — a src
// attribute. The page's two scripts are attribute-less (inline), so this must
// never match.
var externalScriptRe = regexp.MustCompile(`(?is)<script[^>]*\ssrc\s*=`)

// scriptBodyRe spans one attribute-less <script>…</script>; the bundle's own
// closers are neutralized so only the two real script bodies match.
var scriptBodyRe = regexp.MustCompile(`(?is)<script>.*?</script>`)

// stripScriptBodies empties every <script> so an asset audit sees the HTML/CSS
// shell, not the trusted inlined engine's JS string literals.
func stripScriptBodies(page string) string {
	return scriptBodyRe.ReplaceAllString(page, "<script></script>")
}

// repoBundle reads the repo's real pinned mermaid bundle, located by walking up
// from the test's working directory to the module's vendor tree.
func repoBundle(t *testing.T) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, filepath.FromSlash(VendorMermaid))
		if b, err := os.ReadFile(p); err == nil {
			return b
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("vendored mermaid bundle %s not found; run `bash .pyxis/vendor/mermaid/refresh.sh`", VendorMermaid)
		}
		dir = parent
	}
}

// Template escaping: markup in a rendered value is inert (auto-escaped), never
// live (the skeleton's escaping half of A-6).
func TestTemplateEscaping(t *testing.T) {
	_, page := renderPage(t)
	if strings.Contains(page, "<img src=x onerror=alert(1)>") || strings.Contains(page, "<b>capability</b>") {
		t.Error("title markup rendered live (not escaped)")
	}
	if !strings.Contains(page, "&lt;img src=x onerror=alert(1)&gt;") || !strings.Contains(page, "&amp;") {
		t.Error("title markup was not escaped inert")
	}
}

// A filesystem audit across a render shows exactly one file created and none
// modified (T1 acceptance 4 / DINV-1).
func TestFilesystemAuditOneFile(t *testing.T) {
	root := writeFixture(t, greenPlan, true)
	before := snapshot(t, root)

	r := Render(root, config.Defaults(), "docs/plans/widget", "", "")
	if len(r.Defects) > 0 {
		t.Fatalf("green plan refused: %v", r.Defects)
	}
	created, modified := diff(before, snapshot(t, root))
	if len(created) != 1 || created[0] != r.OutPath {
		t.Errorf("filesystem audit: created %v, want exactly [%s]", created, r.OutPath)
	}
	if len(modified) != 0 {
		t.Errorf("filesystem audit: modified %v, want none", modified)
	}
}

// A fold-red plan yields no page and every fold defect; a missing directory,
// unknown version, and stale machine face each refuse with the named quadruple
// and zero writes (T1 acceptance 2 & 3 / FR-7/UB-1/UB-2/DINV-2).
func TestRefusals(t *testing.T) {
	cfg := config.Defaults()

	t.Run("fold-red: no page, defects surfaced", func(t *testing.T) {
		red := strings.Replace(greenPlan, "| **Dialect** | harbormaster/fs-spec:v1 |\n", "", 1)
		root := writeFixture(t, greenPlan, true)
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(planRel)), []byte(red), 0o644); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, root)
		r := Render(root, cfg, "docs/plans/widget", "", "")
		if len(r.Defects) == 0 || r.OutPath != "" {
			t.Fatalf("fold-red should refuse with defects and no page; got %+v", r)
		}
		found := false
		for _, d := range r.Defects {
			if strings.Contains(d.Reason, "Dialect") {
				found = true
			}
			assertQuadruple(t, d.String())
		}
		if !found {
			t.Errorf("fold defects not surfaced: %v", r.Defects)
		}
		assertNoWrites(t, root, before)
	})

	t.Run("missing directory", func(t *testing.T) {
		root := writeFixture(t, greenPlan, true)
		before := snapshot(t, root)
		r := Render(root, cfg, "docs/plans/absent", "", "")
		requireRefusal(t, r)
		assertNoWrites(t, root, before)
	})

	t.Run("unknown version", func(t *testing.T) {
		root := writeFixture(t, greenPlan, true)
		before := snapshot(t, root)
		r := Render(root, cfg, "docs/plans/widget", "007", "")
		requireRefusal(t, r)
		if !strings.Contains(r.Defects[0].Reason, "007") {
			t.Errorf("version defect should name the version: %v", r.Defects)
		}
		assertNoWrites(t, root, before)
	})

	t.Run("stale machine face", func(t *testing.T) {
		root := writeFixture(t, greenPlan, true)
		jsRel := plan.JSONPath(planRel)
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(jsRel)), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, root)
		r := Render(root, cfg, "docs/plans/widget", "", "")
		requireRefusal(t, r)
		if !strings.Contains(r.Defects[0].Reason, "differs") {
			t.Errorf("stale-face defect should name the divergence: %v", r.Defects)
		}
		assertNoWrites(t, root, before)
	})

	t.Run("missing machine face", func(t *testing.T) {
		root := writeFixture(t, greenPlan, false) // source only, no committed face
		before := snapshot(t, root)
		r := Render(root, cfg, "docs/plans/widget", "", "")
		requireRefusal(t, r)
		if !strings.Contains(r.Defects[0].Reason, "missing") {
			t.Errorf("missing-face defect should name the absence: %v", r.Defects)
		}
		assertNoWrites(t, root, before)
	})

	t.Run("missing mermaid bundle: no page, re-vendor remedy", func(t *testing.T) {
		root := writeFixture(t, greenPlan, true)
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(VendorMermaid))); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, root)
		r := Render(root, cfg, "docs/plans/widget", "", "")
		requireRefusal(t, r)
		if !strings.Contains(r.Defects[0].File, "mermaid") ||
			!strings.Contains(r.Defects[0].Remedy, "refresh.sh") {
			t.Errorf("missing-bundle defect should name the bundle and the re-vendor remedy: %v", r.Defects)
		}
		assertNoWrites(t, root, before)
	})
}

// --out overrides the default location.
func TestOutOverride(t *testing.T) {
	root := writeFixture(t, greenPlan, true)
	out := ScratchDir + "/custom.html"
	r := Render(root, config.Defaults(), "docs/plans/widget", "", out)
	if len(r.Defects) > 0 || r.OutPath != out {
		t.Fatalf("--out not honored: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(out))); err != nil {
		t.Errorf("page not written to --out: %v", err)
	}
}

// section returns the substring from the marker to the next end token.
func section(page, start, end string) string {
	i := strings.Index(page, start)
	if i < 0 {
		return ""
	}
	j := strings.Index(page[i:], end)
	if j < 0 {
		return page[i:]
	}
	return page[i : i+j]
}

func requireRefusal(t *testing.T, r Result) {
	t.Helper()
	if len(r.Defects) == 0 || r.OutPath != "" {
		t.Fatalf("expected a refusal with no page; got %+v", r)
	}
	assertQuadruple(t, r.Defects[0].String())
}

func assertQuadruple(t *testing.T, s string) {
	t.Helper()
	// The named quadruple: verb, file, op, reason, and remedy all present.
	if !strings.Contains(s, "plangate:") || !strings.Contains(s, "remedy:") {
		t.Errorf("defect is not the named quadruple: %q", s)
	}
}

func assertNoWrites(t *testing.T, root string, before map[string]fileStat) {
	t.Helper()
	created, modified := diff(before, snapshot(t, root))
	if len(created) != 0 || len(modified) != 0 {
		t.Errorf("refusal wrote to disk: created %v modified %v", created, modified)
	}
}

type fileStat struct {
	size    int64
	modNano int64
}

// snapshot records every regular file's size+mtime under root (repo-relative).
func snapshot(t *testing.T, root string) map[string]fileStat {
	t.Helper()
	out := map[string]fileStat{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = fileStat{size: info.Size(), modNano: info.ModTime().UnixNano()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// diff returns files present only in after (created) and files whose size or
// mtime changed (modified).
func diff(before, after map[string]fileStat) (created, modified []string) {
	for p, a := range after {
		b, ok := before[p]
		if !ok {
			created = append(created, p)
			continue
		}
		if b != a {
			modified = append(modified, p)
		}
	}
	return created, modified
}
