// Hermetic gate-suite tests (design A-1, A-8, A-9's package half): a
// fixture repo is built in a temp dir — governing projection, design doc,
// corpus file, plan source — and each defect class is one mutation of the
// golden plan asserted against its named verdict. No network, no clock.
package plan

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

var update = flag.Bool("update", false, "rewrite testdata/golden-001-plan.json from the fixture fold")

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

const goldenPlan = `# Widget capability — plan 001

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
**Desired behavior:** A typed widget model validates closed and rejects an unknown field with a defect naming that field.

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

// writeFixture builds the hermetic repo and returns its root.
func writeFixture(t *testing.T, planSource string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"docs/prds/widget/effective.source.json":  governingSourceJSON,
		"docs/prds/widget/effective.md":           governingEffectiveMD,
		"docs/prds/scaffolding-and-onboarding.md": corpusDoc,
		"docs/tech-design/widget-design.md":       designDoc,
		"docs/plans/widget/001-plan.md":           planSource,
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
	return root
}

func foldFixture(t *testing.T, planSource string) Result {
	t.Helper()
	root := writeFixture(t, planSource)
	return FoldFile(root, config.Defaults(), "docs/plans/widget/001-plan.md")
}

func TestGoldenPlanFolds(t *testing.T) {
	res := foldFixture(t, goldenPlan)
	for _, d := range res.Defects {
		t.Errorf("unexpected defect: %s", d)
	}
	if res.Doc == nil || len(res.Doc.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %+v", res.Doc)
	}
	// Foundation derivation (FR-22): T1 is the common root; T2/T3 leaves.
	for i, want := range []Foundation{
		{FoundationRoot, ReasonRoot}, {FoundationLeaf, ReasonLeaf}, {FoundationLeaf, ReasonLeaf},
	} {
		if got := res.Doc.Tasks[i].Foundation; got != want {
			t.Errorf("task %s foundation = %+v, want %+v", res.Doc.Tasks[i].ID, got, want)
		}
	}
	if got, want := res.Doc.Tasks[0].PlanTaskID, "widget/001/T1"; got != want {
		t.Errorf("plan-task-id = %q, want %q", got, want)
	}

	golden := filepath.Join("testdata", "golden-001-plan.json")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, res.JSON, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("golden missing (run: go test ./internal/plangate/... -run TestGoldenPlanFolds -update): %v", err)
	}
	if !bytes.Equal(res.JSON, want) {
		t.Error("fold output differs from the committed golden (NFR-1 determinism)")
	}
	// Determinism across re-runs (A-8).
	again := foldFixture(t, goldenPlan)
	if !bytes.Equal(res.JSON, again.JSON) {
		t.Error("two folds of one source differ")
	}
}

func TestDraftLockHeaderParsesWithoutChangingFeatureTaskIdentity(t *testing.T) {
	source := strings.Replace(goldenPlan, "| **PRD** | docs/prds/widget |", "| **PRD** | prd/widget |", 1)
	source = strings.Replace(source, "| **Design** | docs/tech-design/widget-design.md |", "| **Design** | design/widget |\n| **Basis** | lock/widget/v2 |", 1)
	doc, defects := Parse("docs/plans/widget/001-plan.md", []byte(source))
	if len(defects) != 0 || doc == nil || doc.Basis != "lock/widget/v2" || doc.PRD != "prd/widget" || doc.Design != "design/widget" || doc.Tasks[0].PlanTaskID != "widget/001/T1" {
		t.Fatalf("draft header doc=%+v defects=%v", doc, defects)
	}
	withoutBasis := strings.Replace(source, "| **Basis** | lock/widget/v2 |\n", "", 1)
	res := foldFixture(t, withoutBasis)
	found := false
	for _, defect := range res.Defects {
		found = found || strings.Contains(defect.Reason, "lock Basis")
	}
	if !found {
		t.Fatalf("draft-born plan without lock Basis was accepted: %v", res.Defects)
	}
}

func TestNonFeatureIntakeFoldsWithoutFeatureOnlyFields(t *testing.T) {
	body, err := os.ReadFile("testdata/header/intake-001-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	root := writeFixture(t, goldenPlan)
	rel := "docs/plans/maintenance/001-plan.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	res := FoldFile(root, config.Defaults(), rel)
	if len(res.Defects) != 0 || res.Doc == nil {
		t.Fatalf("intake fold defects=%v", res.Defects)
	}
	if res.Doc.Intake != "maintenance" || res.Doc.Feature != "" || res.Doc.PRD != "" || res.Doc.Design != "" || res.Doc.Basis != "" || res.Doc.Tasks[0].PlanTaskID != "maintenance/001/T1" {
		t.Fatalf("intake model=%+v", res.Doc)
	}
	for _, field := range []string{`"feature"`, `"prd"`, `"design"`, `"basis"`} {
		if bytes.Contains(res.JSON, []byte(field)) {
			t.Fatalf("intake machine face contains %s", field)
		}
	}
	if _, defects := Parse(rel, []byte(strings.Replace(string(body), "| **Intake** | maintenance |", "| **Intake** | maintenance |\n| **Feature** | widget |", 1))); len(defects) == 0 {
		t.Fatal("intake mixed with feature fields")
	}
	withCoverage := string(body) + "\n## Coverage\n\n| Requirement | Tasks |\n|---|---|\n| prd:widget/FR-1 | T1 |\n"
	if err := os.WriteFile(path, []byte(withCoverage), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed := FoldFile(root, config.Defaults(), rel); len(changed.Defects) == 0 || !strings.Contains(changed.Defects[0].Reason, "no governing PRD") {
		t.Fatalf("non-feature coverage was accepted: %v", changed.Defects)
	}
}

func TestVerify(t *testing.T) {
	root := writeFixture(t, goldenPlan)
	cfg := config.Defaults()
	rel := "docs/plans/widget/001-plan.md"
	jsRel := JSONPath(rel)

	if d := Verify(root, cfg, rel); len(d) == 0 || !strings.Contains(d[0].Reason, "missing") {
		t.Fatalf("expected missing-face defect, got %v", d)
	}
	res := FoldFile(root, cfg, rel)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(jsRel)), res.JSON, 0o644); err != nil {
		t.Fatal(err)
	}
	if d := Verify(root, cfg, rel); len(d) != 0 {
		t.Fatalf("expected clean verify, got %v", d)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(jsRel)), append(res.JSON, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := Verify(root, cfg, rel); len(d) == 0 || !strings.Contains(d[0].Reason, "differs") {
		t.Fatalf("expected divergence defect, got %v", d)
	}
}

// mutate applies old→new to the golden plan, failing if nothing changed.
func mutate(t *testing.T, old, new string) string {
	t.Helper()
	if !strings.Contains(goldenPlan, old) {
		t.Fatalf("mutation anchor %q not in the golden plan", old)
	}
	return strings.Replace(goldenPlan, old, new, 1)
}

// TestFeatureCoverage exercises the union completeness check (DINV-2/DINV-3,
// A-3/A-4): a full plan is feature-complete; a slice-plan that omits a Must
// folds green per-plan (the table case above) yet is named at the feature level.
func TestFeatureCoverage(t *testing.T) {
	if defs := FeatureCoverage(writeFixture(t, goldenPlan), config.Defaults(), "widget"); len(defs) != 0 {
		t.Fatalf("full plan should be feature-complete, got %v", defs)
	}
	sliced := strings.Replace(goldenPlan, "| prd:widget/UB-1 | T2 |\n", "", 1)
	defs := FeatureCoverage(writeFixture(t, sliced), config.Defaults(), "widget")
	if len(defs) == 0 || !strings.Contains(defs[0].Reason, "UB-1") {
		t.Fatalf("a Must omitted from every plan's slice should be a named feature-level defect, got %v", defs)
	}
}

func TestGateDefects(t *testing.T) {
	cases := []struct {
		name string
		plan string
		want string // substring of at least one defect
	}{
		{"unknown header row", mutate(t, "| **Status** | Draft |", "| **Status** | Draft |\n| **Mood** | great |"), "unknown header row"},
		{"missing header row", mutate(t, "| **Dialect** | harbormaster/fs-spec:v1 |\n", ""), `missing header row "Dialect"`},
		{"unknown dialect", mutate(t, "harbormaster/fs-spec:v1", "carrier-pigeon/v0"), "unknown dialect"},
		{"template mismatch", mutate(t, "plan-template/v1", "plan-template/v9"), "unsupported"},
		{"feature/prd mismatch", mutate(t, "| **Feature** | widget |", "| **Feature** | gadget |"), "does not match the PRD directory"},
		{"sparse task ids", mutate(t, "### T2 — Serve widget lookups", "### T5 — Serve widget lookups"), "dense and ascending"},
		{"unknown task field", mutate(t, "| **Routing** | rung=2 |", "| **Routing** | rung=2 |\n| **Vibe** | good |"), "unknown task field"},
		{"bad size band", mutate(t, "| **Size band** | 60–120 |", "| **Size band** | roughly 100 |"), "not `<lo>–<hi>`"},
		{"acceptance without checkbox", mutate(t, "- [ ] a lookup for a stored widget returns it", "- a lookup for a stored widget returns it"), "without a checkbox"},
		{"unknown region label", mutate(t, "**Goal:** The widget model type exists with closed validation.", "**Vibes:** immaculate.\n**Goal:** The widget model type exists with closed validation."), "unknown region label"},
		{"unknown blocked-by", mutate(t, "| **Blocked by** | T1 |\n| **Size band** | 80–160 |", "| **Blocked by** | T9 |\n| **Size band** | 80–160 |"), `blocked-by target "T9"`},
		{"cycle", mutate(t, "| **Blocked by** | — |", "| **Blocked by** | T2 |"), "dependency cycle"},
		{"over acceptance cap", mutate(t, "- [ ] a lookup miss names the widget it did not find",
			"- [ ] a lookup miss names the widget it did not find\n- [ ] criterion three holds\n- [ ] criterion four holds\n- [ ] criterion five holds"), "acceptance-criteria count"},
		{"size accepted suppresses", mutate(t, mutAnchorT2SizeAccepted,
			"| **Blocked by** | T1 |\n| **Size band** | 80–460 |\n| **Mechanical** | no |\n| **Size accepted** | atomic behavior; splitting breaks the contract |"), ""},
		{"band over cap", mutate(t, "| **Size band** | 80–160 |", "| **Size band** | 80–460 |"), "size-band upper bound"},
		{"mechanical nonzero band", mutate(t, "| **Size band** | 0 |", "| **Size band** | 10–20 |"), "non-zero size band"},
		{"mechanical without provenance", mutate(t, "**Provenance:**\n- upstream tag v1.2.3 with its published checksum recorded beside the bundle\n\n", ""), "without provenance"},
		{"behavior task with provenance", mutate(t, "**Scope:**\n- the widget model type and its closed validation", "**Provenance:**\n- upstream tag\n\n**Scope:**\n- the widget model type and its closed validation"), "carrying provenance"},
		{"path in charter", mutate(t, "- the lookup behavior over stored widgets", "- rewrite the parser in internal/widget/serve/parse.go"), "file path or line number"},
		{"line ref in charter", mutate(t, "**Desired behavior:** A lookup returns the widget when present and a named miss when absent, never an empty success.", "**Desired behavior:** Change the handler near line 42 to return a named miss."), "file path or line number"},
		{"vague acceptance", mutate(t, "- [ ] a lookup for a stored widget returns it", "- [ ] lookups feel fast and intuitive"), "banned vagueness"},
		{"missing traces", mutate(t, "**Traces:** prd:widget/FR-1 · design:widget-design#3.1\n", "\n"), "no trace references"},
		{"dangling prd trace", mutate(t, "prd:widget/FR-1 ·", "prd:widget/FR-9 ·"), "does not resolve to a live unit"},
		{"dangling design anchor", mutate(t, "design:widget-design#3.1", "design:widget-design#9.9"), "not found"},
		{"unknown ref form", mutate(t, "design:widget-design#3.1", "spec:widget#1"), "unknown trace reference form"},
		{"Must omitted from slice folds green (DINV-1)", mutate(t, "| prd:widget/UB-1 | T2 |\n", ""), ""},
		{"optional omitted from slice folds green (DINV-1)", mutate(t, "| prd:widget/FR-3 | summaries have no consumer yet; next plan |\n", ""), ""},
		{"must deferred", mutate(t, "| prd:widget/FR-3 | summaries have no consumer yet; next plan |", "| prd:widget/FR-3 | summaries have no consumer yet; next plan |\n| prd:widget/UB-1 | later |"), `Must requirement "prd:widget/UB-1" deferred`},
		{"rogue task", mutate(t, "| prd:widget/FR-2 | T2, T3 |", "| prd:widget/FR-2 | T2 |"), "appears in no coverage row"},
		{"duplicate coverage", mutate(t, "| prd:widget/UB-1 | T2 |", "| prd:widget/UB-1 | T2 |\n| prd:widget/FR-1 | T1 |"), "duplicate coverage row"},
		{"foreign coverage", mutate(t, "| prd:widget/UB-1 | T2 |", "| prd:widget/UB-1 | T2 |\n| prd:gadget/FR-1 | T1 |"), "foreign PRD"},
		{"unknown coverage task", mutate(t, "| prd:widget/UB-1 | T2 |", "| prd:widget/UB-1 | T2, T9 |"), `names unknown task "T9"`},
		{"parallel overlap", mutate(t, mutAnchorT3Head, strings.Replace(mutAnchorT3Head, "| **Blocked by** | T1 |", "| **Blocked by** | — |", 1)+""), ""},
		{"pyxis path", mutate(t, "- persistence of widgets", "- the .pyxis/config.yaml surface"), "bucket-1 operand content"},
		{"pattern shape", mutate(t, "| **Pattern** | — |\n| **Touches** | internal/widget/serve |", "| **Pattern** | migrate-batch |\n| **Touches** | internal/widget/serve |"), "expand–contract shape"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := foldFixture(t, tc.plan)
			if tc.want == "" {
				for _, d := range res.Defects {
					t.Errorf("expected clean fold, got: %s", d)
				}
				return
			}
			for _, d := range res.Defects {
				if strings.Contains(d.Reason, tc.want) || strings.Contains(d.Remedy, tc.want) {
					return
				}
			}
			t.Errorf("no defect contains %q; got %v", tc.want, res.Defects)
		})
	}
}

// The share gate arms at three behavior tasks; T3 becomes a behavior task
// holding well over half the plan's estimated size.
func TestShareGate(t *testing.T) {
	behaviorT3 := `### T3 — Export widget summaries

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/widget/export |

**Goal:** Summaries export per widget.
**Current behavior:** No export path exists.
**Desired behavior:** A summary export lists each widget once with its lookup count.

**Scope:**
- the summary export behavior

**Out of scope:**
- export scheduling

**Acceptance criteria:**
- [ ] an export lists each stored widget exactly once

**Key interfaces:**
- ` + "`Export`" + ` — the summary surface

**Traces:** prd:widget/FR-2
`
	before, _, _ := strings.Cut(goldenPlan, "### T3")
	_, coverage, _ := strings.Cut(goldenPlan, "## Coverage")
	threeBehavior := before + behaviorT3 + "\n## Coverage" + coverage
	// Shrink the siblings so T3 dominates: T1 30–50, T2 30–50.
	threeBehavior = strings.Replace(threeBehavior, "| **Size band** | 60–120 |", "| **Size band** | 30–50 |", 1)
	threeBehavior = strings.Replace(threeBehavior, "| **Size band** | 80–160 |", "| **Size band** | 30–50 |", 1)
	res := foldFixture(t, threeBehavior)
	found := false
	for _, d := range res.Defects {
		found = found || strings.Contains(d.Reason, "of the plan's estimated size")
	}
	if !found {
		t.Errorf("expected a share defect, got %v", res.Defects)
	}
}

// Overlap needs its own arrangement: T2 and T3 unordered on one surface.
func TestOverlapSequencing(t *testing.T) {
	unordered := strings.Replace(goldenPlan, mutAnchorT3Head,
		strings.Replace(strings.Replace(mutAnchorT3Head, "| **Blocked by** | T1 |", "| **Blocked by** | — |", 1),
			"| **Touches** | vendor/widget |", "| **Touches** | internal/widget/serve |", 1), 1)
	res := foldFixture(t, unordered)
	found := false
	for _, d := range res.Defects {
		found = found || strings.Contains(d.Reason, "parallel siblings")
	}
	if !found {
		t.Errorf("expected a parallel-siblings overlap defect, got %v", res.Defects)
	}

	accepted := strings.Replace(unordered, "| **Overlap accepted** | — |\n| **Routing** | — |\n| **Pattern** | — |\n| **Touches** | internal/widget/serve |",
		"| **Overlap accepted** | append-only file; concurrent edits merge cleanly |\n| **Routing** | — |\n| **Pattern** | — |\n| **Touches** | internal/widget/serve |", 1)
	res = foldFixture(t, accepted)
	for _, d := range res.Defects {
		if strings.Contains(d.Reason, "parallel siblings") {
			t.Errorf("overlap acceptance did not suppress: %s", d)
		}
	}
}

// Anchors used by mutations (kept as constants so a fixture edit that
// breaks them fails loudly in mutate()).
const (
	mutAnchorT2SizeAccepted = "| **Blocked by** | T1 |\n| **Size band** | 80–160 |\n| **Mechanical** | no |\n| **Size accepted** | — |"
	mutAnchorT3Head         = "| **Blocked by** | T1 |\n| **Size band** | 0 |\n| **Mechanical** | yes |\n| **Size accepted** | — |\n| **Overlap accepted** | — |\n| **Routing** | — |\n| **Pattern** | — |\n| **Touches** | vendor/widget |"
)
