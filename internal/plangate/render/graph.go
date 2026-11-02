// Graph region — the dependency diagram (plan-viewer-v1 §3.6, DINV-4, plan T2).
// The diagram definition is generated from the folded model's tasks and edges:
// every task is one node (labeled `T<n> — <title>`), every blocked_by relation
// is one edge (dep --> task), and the three foundation levels get distinct node
// styles. The engine is the pinned vendored mermaid bundle inlined at render
// time, so the page stays self-contained with zero external references
// (DINV-3). The definition derives from the folded edge set, never from prose.
package render

import (
	"html/template"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// VendorMermaid is the repo-relative path to the pinned mermaid browser bundle —
// the same bytes the doc renderer inlines. Read, never modified (PRD NR-2).
const VendorMermaid = ".pyxis/vendor/mermaid/mermaid.min.js"

// foundationStyles maps each foundation level to a mermaid class name and a
// distinct classDef style. The three levels render visibly distinct (DINV-4);
// the fixed order keeps the definition deterministic (DINV-1).
var foundationStyles = []struct{ Level, Class, Style string }{
	{plan.FoundationRoot, "fRoot", "fill:#241a4d,stroke:#7c5cff,stroke-width:2px,color:#e6e9ef"},
	{plan.FoundationInternal, "fInternal", "fill:#123047,stroke:#5b9dff,stroke-width:2px,color:#e6e9ef"},
	{plan.FoundationLeaf, "fLeaf", "fill:#0f3a2e,stroke:#27c093,stroke-width:2px,color:#e6e9ef"},
}

// graphDef builds the mermaid `graph TD` definition from the folded model:
// each task node exactly once (plan order), each blocked_by edge exactly once
// (dep --> task), then the classDef styles and per-level class assignments.
func graphDef(doc *plan.Doc) string {
	var b strings.Builder
	b.WriteString("graph TD\n")

	// Nodes: one per task, in plan order — each task exactly once (DINV-4).
	for i := range doc.Tasks {
		t := &doc.Tasks[i]
		b.WriteString("  ")
		b.WriteString(t.ID)
		b.WriteString(`["`)
		b.WriteString(mermaidLabel(t.ID + " — " + t.Title))
		b.WriteString("\"]\n")
	}

	// Edges: one per blocked_by relation (dep --> task) — each edge exactly
	// once (DINV-4). Dedupe within a task so a repeated blocker cannot double
	// an edge.
	for i := range doc.Tasks {
		t := &doc.Tasks[i]
		seen := map[string]bool{}
		for _, dep := range t.BlockedBy {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			b.WriteString("  ")
			b.WriteString(dep)
			b.WriteString(" --> ")
			b.WriteString(t.ID)
			b.WriteString("\n")
		}
	}

	// Foundation styling: every classDef is always defined (so the three
	// styles are provably distinct), then each present level's nodes are
	// assigned to its class in plan order.
	for _, s := range foundationStyles {
		b.WriteString("  classDef ")
		b.WriteString(s.Class)
		b.WriteString(" ")
		b.WriteString(s.Style)
		b.WriteString("\n")
	}
	for _, s := range foundationStyles {
		var ids []string
		for i := range doc.Tasks {
			if doc.Tasks[i].Foundation.Level == s.Level {
				ids = append(ids, doc.Tasks[i].ID)
			}
		}
		if len(ids) == 0 {
			continue
		}
		b.WriteString("  class ")
		b.WriteString(strings.Join(ids, ","))
		b.WriteString(" ")
		b.WriteString(s.Class)
		b.WriteString("\n")
	}
	return b.String()
}

// labelBreak collapses any newline run in a node label to one space — a raw
// newline would truncate the mermaid node declaration.
var labelBreak = regexp.MustCompile(`[\r\n]+`)

// mermaidLabel makes a label safe inside a quoted mermaid node: `#` and `"` are
// the characters that break the quoted-label parser, so they become mermaid
// entity codes (`#` first, so the `#` in `#quot;` is not re-encoded). The label
// is HTML-escaped a second time by the page template — the diagram lives in a
// <pre> whose text mermaid reads — so only mermaid's metacharacters matter here.
func mermaidLabel(s string) string {
	s = labelBreak.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "#", "#35;")
	s = strings.ReplaceAll(s, `"`, "#quot;")
	return s
}

// scriptClose neutralizes the element-closing sequences the mermaid bundle
// embeds inside its own JS strings (</script>, </body>, </html>): unescaped, the
// first </script> would detonate the inline <script> element and dump the rest
// of the bundle as page text. Mirrors the doc renderer's inlineScriptSafe.
var scriptClose = regexp.MustCompile(`(?i)</(script|body|html)`)

// mermaidScript is the inlined engine: the script-safe bundle plus the init
// call that turns every <pre class="mermaid"> into an SVG on load. Returned as
// template.HTML so it renders raw (it is trusted, pinned, script-safe data).
func mermaidScript(js []byte) template.HTML {
	safe := scriptClose.ReplaceAllString(string(js), `<\/$1`)
	return template.HTML("<script>\n" + safe +
		"\n</script>\n<script>mermaid.initialize({ startOnLoad: true, theme: 'dark' });</script>\n")
}
