// Flags-panel tests (plan-viewer-v1 §3.6 region 5, plan T3). The panel must
// aggregate exactly the machine face's recorded acceptances — each with its
// task and reason — and nothing else (A-5 / PRD FR-5). "Exactly" is asserted
// both ways: every recorded acceptance appears, and the panel's entry count
// equals the recorded count so no phantom flag is invented and a task with no
// acceptance contributes none.
package render

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// flagsRegion returns the bytes of the flags section only.
func flagsRegion(page string) string {
	return section(page, `id="flags"`, "</section>")
}

// flagsDoc carries a mix of tasks: size-only, overlap-only, both, and neither —
// so the panel's exact membership (and non-membership) is provable.
func flagsDoc() *plan.Doc {
	return &plan.Doc{
		Schema:      plan.SchemaVersion,
		Tool:        plan.ToolVersion,
		Title:       "Flags plan",
		Feature:     "flags",
		Version:     "001",
		Dialect:     plan.DialectHarbormaster,
		Status:      "Draft",
		Fingerprint: "sha256:flags",
		Tasks: []plan.Task{
			{ID: "T1", Title: "size only", SizeAccepted: "SZ-ONE"},
			{ID: "T2", Title: "overlap only", OverlapAccepted: "OV-TWO"},
			{ID: "T3", Title: "both", SizeAccepted: "SZ-THREE", OverlapAccepted: "OV-THREE"},
			{ID: "T4", Title: "neither"},
		},
	}
}

// The panel lists exactly the recorded acceptances, each with its task and
// reason; a task with no acceptance contributes none, and the entry count
// equals the recorded count (A-5).
func TestFlagsExactMembership(t *testing.T) {
	doc := flagsDoc()
	b, err := Page(doc, stubBundle)
	if err != nil {
		t.Fatal(err)
	}
	region := flagsRegion(string(b))
	if region == "" {
		t.Fatal("no flags region rendered")
	}

	// Independently enumerate what the machine face records (size before
	// overlap within a task), the oracle the panel must match exactly.
	type want struct{ task, kind, reason string }
	var expect []want
	for i := range doc.Tasks {
		tk := doc.Tasks[i]
		if tk.SizeAccepted != "" {
			expect = append(expect, want{tk.ID, "size", tk.SizeAccepted})
		}
		if tk.OverlapAccepted != "" {
			expect = append(expect, want{tk.ID, "overlap", tk.OverlapAccepted})
		}
	}

	// Every recorded acceptance appears in one <li> that names its kind, links
	// to its card, and carries its reason.
	for _, w := range expect {
		li := ""
		for _, chunk := range strings.Split(region, `<li class="flag">`)[1:] {
			body := chunk[:strings.Index(chunk, "</li>")]
			if strings.Contains(body, w.reason) {
				li = body
				break
			}
		}
		if li == "" {
			t.Errorf("flag for %s (%s) missing reason %q", w.task, w.kind, w.reason)
			continue
		}
		if !strings.Contains(li, `href="#`+w.task+`"`) {
			t.Errorf("flag %q does not link to task card %s", w.reason, w.task)
		}
		if !strings.Contains(li, w.kind+" accepted") {
			t.Errorf("flag %q does not name its kind %q", w.reason, w.kind)
		}
	}

	// Exactly that many entries — no phantom flag, and the acceptance-free task
	// contributes nothing.
	if n := strings.Count(region, `<li class="flag">`); n != len(expect) {
		t.Errorf("flags panel has %d entries, want exactly %d", n, len(expect))
	}
	if strings.Contains(region, `href="#T4"`) {
		t.Error("acceptance-free task T4 leaked into the flags panel")
	}
}

// A plan with no recorded acceptances renders an explicit empty panel, not a
// missing region and not a phantom entry — the empty case of "exactly" (A-5).
func TestFlagsEmptyWhenNoneRecorded(t *testing.T) {
	// The green fixture carries no size/overlap acceptances.
	_, page := renderPage(t)
	region := flagsRegion(page)
	if region == "" {
		t.Fatal("flags region absent even when empty; it must state the empty case")
	}
	if strings.Contains(region, `<li class="flag">`) {
		t.Errorf("flags panel invented an entry with none recorded:\n%s", region)
	}
	if !strings.Contains(region, "No size or overlap acceptances") {
		t.Errorf("empty flags panel gives no empty-state note:\n%s", region)
	}
}
