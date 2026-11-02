// Graph-region tests (plan-viewer-v1 §3.6, DINV-4, plan T2 acceptance 1 & 2):
// the diagram definition is derived from the folded edge set — each task node
// exactly once, each blocked_by edge exactly once — and the foundation levels
// present render with distinct styles. Pure over the folded model; the
// self-containment half (acceptance 3) lives in render_test.go.
package render

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// foldFixture folds the green fixture and returns its model.
func foldFixture(t *testing.T) *plan.Doc {
	t.Helper()
	root := writeFixture(t, greenPlan, true)
	res := plan.FoldFile(root, config.Defaults(), planRel)
	if len(res.Defects) > 0 || res.Doc == nil {
		t.Fatalf("fixture did not fold green: %v", res.Defects)
	}
	return res.Doc
}

// Each task node appears exactly once and each blocked_by edge exactly once,
// with no edge the model does not carry (T2 acceptance 1 / DINV-4).
func TestGraphNodesAndEdgesOnce(t *testing.T) {
	doc := foldFixture(t)
	def := graphDef(doc)

	for _, tk := range doc.Tasks {
		if n := strings.Count(def, tk.ID+`["`); n != 1 {
			t.Errorf("node %s declared %d times, want exactly 1", tk.ID, n)
		}
	}

	wantEdges := map[string]bool{}
	for _, tk := range doc.Tasks {
		for _, dep := range tk.BlockedBy {
			wantEdges[dep+" --> "+tk.ID] = true
		}
	}
	if len(wantEdges) == 0 {
		t.Fatal("fixture has no edges to assert on")
	}
	for edge := range wantEdges {
		if n := strings.Count(def, edge); n != 1 {
			t.Errorf("edge %q appears %d times, want exactly 1", edge, n)
		}
	}
	// No extra edges: the definition's arrow count equals the model's.
	if got := strings.Count(def, " --> "); got != len(wantEdges) {
		t.Errorf("definition has %d edges, model has %d", got, len(wantEdges))
	}
}

// The three foundation classes carry three distinct styles, and every task with
// a known level is assigned to its level's class — so the levels present render
// distinctly (T2 acceptance 2 / DINV-4). The fixture must exercise ≥2 levels.
func TestGraphFoundationStylesDistinct(t *testing.T) {
	doc := foldFixture(t)
	def := graphDef(doc)

	seen := map[string]bool{}
	for _, s := range foundationStyles {
		if !strings.Contains(def, "classDef "+s.Class+" "+s.Style) {
			t.Errorf("missing distinct classDef for level %q", s.Level)
		}
		if seen[s.Style] {
			t.Errorf("foundation style for %q is not distinct", s.Level)
		}
		seen[s.Style] = true
	}

	levels := map[string][]string{}
	for _, tk := range doc.Tasks {
		levels[tk.Foundation.Level] = append(levels[tk.Foundation.Level], tk.ID)
	}
	exercised := 0
	for _, s := range foundationStyles {
		ids := levels[s.Level]
		if len(ids) == 0 {
			continue
		}
		exercised++
		if !strings.Contains(def, "class "+strings.Join(ids, ",")+" "+s.Class) {
			t.Errorf("level %q nodes %v not assigned to class %s", s.Level, ids, s.Class)
		}
	}
	if exercised < 2 {
		t.Fatalf("fixture exercises %d foundation level(s); need ≥2 to prove distinct rendering", exercised)
	}
}

// The graph region lands in the page: the mermaid container with every task
// node, drawn by the inlined engine (page-level integration for T2).
func TestGraphSectionInPage(t *testing.T) {
	root, page := renderPage(t)
	doc := plan.FoldFile(root, config.Defaults(), planRel).Doc

	graph := section(page, `id="graph"`, "</section>")
	if !strings.Contains(graph, `class="mermaid"`) {
		t.Fatal("graph section missing the mermaid diagram container")
	}
	for _, tk := range doc.Tasks {
		if !strings.Contains(graph, tk.ID+"[") {
			t.Errorf("graph section missing node %s", tk.ID)
		}
	}
	if !strings.Contains(page, "mermaid.initialize(") {
		t.Error("page missing the mermaid init call that draws the diagram")
	}
}

// mermaidLabel neutralizes the characters that break a quoted mermaid node so a
// hostile or punctuation-bearing title cannot corrupt the diagram (defensive;
// A-6's escaping is proven end-to-end at T3).
func TestMermaidLabelSafe(t *testing.T) {
	got := mermaidLabel(`a "b" #c` + "\n" + "d")
	for _, bad := range []string{`"`, "#c", "\n"} {
		if strings.Contains(got, bad) {
			t.Errorf("mermaidLabel left %q unescaped: %q", bad, got)
		}
	}
	if !strings.Contains(got, "#quot;") || !strings.Contains(got, "#35;") {
		t.Errorf("mermaidLabel did not emit entity codes: %q", got)
	}
}
