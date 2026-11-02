// Page composition — the render spine's page (plan-viewer-v1 §3.6, plan
// T1/T2/T3): one html/template page, inline CSS, and the five regions in
// reading order — the header, the dependency graph (a mermaid diagram drawn by
// the inlined vendored bundle), the anchored task cards, the coverage matrix
// (task references as in-page links, deferred rows distinctly marked), the
// flags panel (every recorded acceptance) — closed by a fingerprint footer. All
// plan values render as text (auto-escaped by the template engine): the page
// never interprets plan content as markup.
package render

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// Page renders the folded model into one self-contained page. mermaidJS is the
// pinned vendored bundle, inlined so the diagram draws offline (DINV-3).
// Deterministic: given one plan state and bundle, the bytes are fixed (DINV-1).
func Page(doc *plan.Doc, mermaidJS []byte) ([]byte, error) {
	v := newView(doc, mermaidJS)
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, v); err != nil {
		return nil, fmt.Errorf("execute page template: %w", err)
	}
	return buf.Bytes(), nil
}

// view is the template's data — every value projected from the folded model;
// nothing re-parses the source.
type view struct {
	Title       string
	Feature     string
	Intake      string
	Version     string
	Dialect     string
	Status      string
	Fingerprint string
	ToolVersion string
	TaskCount   int
	BandSum     int           // sum of task band midpoints (the share-computation currency)
	LargestPct  int           // largest task's share of the band sum, percent
	Graph       string        // the mermaid graph-TD definition (auto-escaped in the <pre>)
	Script      template.HTML // the inlined, script-safe mermaid bundle + init
	Cards       []cardView    // one anchored task card per task, in plan order
	Coverage    []covRow      // the coverage matrix: covered rows then deferred rows
	Flags       []flagView    // every recorded size/overlap acceptance, task order
}

func newView(doc *plan.Doc, mermaidJS []byte) view {
	sumMid, maxMid := 0, 0
	for i := range doc.Tasks {
		m := doc.Tasks[i].Band.Mid()
		sumMid += m
		if m > maxMid {
			maxMid = m
		}
	}
	pct := 0
	if sumMid > 0 {
		pct = maxMid * 100 / sumMid
	}
	return view{
		Title:       doc.Title,
		Feature:     doc.Feature,
		Intake:      doc.Intake,
		Version:     doc.Version,
		Dialect:     doc.Dialect,
		Status:      doc.Status,
		Fingerprint: doc.Fingerprint,
		ToolVersion: doc.Tool,
		TaskCount:   len(doc.Tasks),
		BandSum:     sumMid,
		LargestPct:  pct,
		Graph:       graphDef(doc),
		Script:      mermaidScript(mermaidJS),
		Cards:       cards(doc),
		Coverage:    coverage(doc),
		Flags:       flags(doc),
	}
}

// pageTmpl is parsed once at init; a parse failure is a programmer error (the
// literal is fixed).
var pageTmpl = template.Must(template.New("page").Parse(pageHTML))

const pageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="generator" content="plangate render">
<title>{{.Title}}</title>
<style>` + pageCSS + `</style>
</head>
<body>
<main class="wrap">
<h1 class="doc-title">{{.Title}}</h1>

<section class="block" id="header">
<h2 class="section-head">Plan summary</h2>
<dl class="meta">
<dt>{{if .Feature}}Feature{{else}}Intake{{end}}</dt><dd>{{if .Feature}}{{.Feature}}{{else}}{{.Intake}}{{end}}</dd>
<dt>Plan version</dt><dd>{{.Version}}</dd>
<dt>Dialect</dt><dd><code>{{.Dialect}}</code></dd>
<dt>Status</dt><dd>{{.Status}}</dd>
<dt>Totals</dt><dd>{{.TaskCount}} task(s) · band sum {{.BandSum}} · largest task {{.LargestPct}}% of the plan</dd>
</dl>
</section>

<section class="block" id="graph">
<h2 class="section-head">Dependency graph</h2>
<div class="diagram"><pre class="mermaid">{{.Graph}}</pre></div>
</section>

<section class="block" id="cards">
<h2 class="section-head">Task charters</h2>
{{range .Cards}}
<article class="card" id="{{.ID}}">
<h3 class="card-head"><a class="anchor" href="#{{.ID}}">{{.ID}}</a> — {{.Title}}</h3>
<div class="badges">{{range .Badges}}<span class="badge"><span class="badge-k">{{.Label}}</span> {{.Value}}</span> {{end}}</div>
{{with .Goal}}<p class="field"><span class="k">Goal</span>{{.}}</p>{{end}}
{{with .Current}}<p class="field"><span class="k">Current behavior</span>{{.}}</p>{{end}}
{{with .Desired}}<p class="field"><span class="k">Desired behavior</span>{{.}}</p>{{end}}
{{with .Scope}}<div class="field"><span class="k">Scope</span><ul>{{range .}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
{{with .OutOfScope}}<div class="field"><span class="k">Out of scope</span><ul>{{range .}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
{{with .Acceptance}}<div class="field"><span class="k">Acceptance criteria</span><ul>{{range .}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
{{with .KeyInterfaces}}<div class="field"><span class="k">Key interfaces</span><ul>{{range .}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
{{with .Provenance}}<div class="field"><span class="k">Provenance</span><ul>{{range .}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
{{with .Traces}}<div class="field"><span class="k">Traces</span><ul>{{range .}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
<div class="ptid">plan-task-id <code>{{.PlanTaskID}}</code></div>
</article>
{{end}}
</section>

<section class="block" id="coverage">
<h2 class="section-head">Coverage</h2>
{{if .Coverage}}
<table class="coverage">
<thead><tr><th>Requirement</th><th>Coverage</th></tr></thead>
<tbody>
{{range .Coverage}}
<tr class="cov-row{{if .Deferred}} deferred{{end}}">
<td class="req">{{.Requirement}}</td>
{{if .Deferred}}<td class="cov"><span class="defer-tag">deferred</span> {{.Reason}}</td>{{else}}<td class="cov">{{range .Tasks}}<a class="tasklink" href="#{{.}}">{{.}}</a> {{end}}</td>{{end}}
</tr>
{{end}}
</tbody>
</table>
{{else}}
<p class="empty">No coverage rows recorded.</p>
{{end}}
</section>

<section class="block" id="flags">
<h2 class="section-head">Acceptance flags</h2>
{{if .Flags}}
<ul class="flags">
{{range .Flags}}
<li class="flag"><span class="flag-kind">{{.Kind}} accepted</span> <a class="tasklink" href="#{{.Task}}">{{.Task}}</a> — {{.Reason}}</li>
{{end}}
</ul>
{{else}}
<p class="empty">No size or overlap acceptances recorded.</p>
{{end}}
</section>

<footer class="footer">
<span>plan <code>{{if .Feature}}{{.Feature}}{{else}}{{.Intake}}{{end}}</code> version <code>{{.Version}}</code></span>
<span>source fingerprint <code>{{.Fingerprint}}</code></span>
<span>rendered by <code>{{.ToolVersion}}</code></span>
</footer>
</main>
{{.Script}}</body>
</html>
`

const pageCSS = `
:root{--bg:#0f1115;--card:#171a21;--ink:#e6e9ef;--muted:#9aa3b2;--line:#262b35;
--accent:#5b9dff;}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);
font:15px/1.55 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;}
.wrap{max-width:960px;margin:0 auto;padding:32px 24px 80px;}
h1.doc-title{font-size:26px;line-height:1.25;margin:0 0 20px;}
section.block{border-top:1px solid var(--line);padding-top:20px;margin-top:24px;}
h2.section-head{font-size:19px;margin:0 0 12px;}
dl.meta{display:grid;grid-template-columns:max-content minmax(0,1fr);gap:6px 18px;margin:0;}
dl.meta dt{color:var(--muted);font-weight:600;}
dl.meta dd{margin:0;min-width:0;overflow-wrap:anywhere;}
code{background:rgba(255,255,255,.06);border-radius:5px;padding:1px 5px;font-size:.92em;
font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;}
.diagram{background:var(--card);border:1px solid var(--line);border-radius:12px;
padding:16px;margin:0;overflow-x:auto;}
.diagram pre.mermaid{margin:0;padding:0;background:none;display:flex;justify-content:center;}
.diagram pre.mermaid svg{max-width:100%;height:auto;}
.card{background:var(--card);border:1px solid var(--line);border-radius:12px;
padding:18px 20px;margin:16px 0;}
.card-head{font-size:17px;line-height:1.35;margin:0 0 12px;overflow-wrap:anywhere;}
.card-head a.anchor{color:var(--accent);text-decoration:none;
font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;}
.card-head a.anchor:hover{text-decoration:underline;}
.badges{display:flex;flex-wrap:wrap;gap:8px;margin:0 0 14px;}
.badge{background:rgba(255,255,255,.05);border:1px solid var(--line);border-radius:999px;
padding:2px 11px;font-size:12px;color:var(--muted);}
.badge .badge-k{color:var(--ink);font-weight:600;}
.card .field{margin:10px 0;}
.card p.field{overflow-wrap:anywhere;}
.card .field .k{display:block;color:var(--muted);font-weight:600;font-size:11px;
text-transform:uppercase;letter-spacing:.05em;margin-bottom:3px;}
.card .field ul{margin:0;padding-left:20px;}
.card .field li{margin:2px 0;overflow-wrap:anywhere;}
.card .ptid{margin-top:14px;color:var(--muted);font-size:12px;}
.empty{color:var(--muted);font-style:italic;margin:0;}
table.coverage{width:100%;border-collapse:collapse;font-size:14px;}
table.coverage th{text-align:left;color:var(--muted);font-weight:600;font-size:11px;
text-transform:uppercase;letter-spacing:.05em;padding:0 12px 8px;border-bottom:1px solid var(--line);}
table.coverage td{padding:9px 12px;border-bottom:1px solid var(--line);vertical-align:top;
overflow-wrap:anywhere;}
table.coverage td.req{color:var(--ink);font-weight:600;white-space:nowrap;}
tr.cov-row.deferred td.req{color:var(--muted);font-weight:400;}
tr.cov-row.deferred td.cov{color:var(--muted);}
.defer-tag{display:inline-block;background:rgba(255,255,255,.05);border:1px solid var(--line);
border-radius:999px;padding:1px 9px;font-size:11px;text-transform:uppercase;letter-spacing:.05em;
color:var(--muted);margin-right:6px;}
a.tasklink{color:var(--accent);text-decoration:none;
font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:.92em;}
a.tasklink:hover{text-decoration:underline;}
ul.flags{list-style:none;margin:0;padding:0;}
li.flag{padding:9px 0;border-bottom:1px solid var(--line);overflow-wrap:anywhere;}
li.flag:last-child{border-bottom:none;}
.flag-kind{display:inline-block;background:rgba(255,255,255,.05);border:1px solid var(--line);
border-radius:999px;padding:2px 11px;font-size:12px;color:var(--ink);font-weight:600;margin-right:4px;}
.footer{border-top:1px solid var(--line);margin-top:32px;padding-top:16px;
display:flex;flex-wrap:wrap;gap:6px 20px;color:var(--muted);font-size:12px;}
`
