// Task-card region tests (plan-viewer-v1 §3.6 region 3, plan T2). Each task
// renders as one anchored card carrying its complete charter — the contract the
// worker executes — so the reviewer reads it whole. Three acceptance faces:
// every charter field and badge of every fixture task appears in its card
// (A-3's per-fixture half); a maximal task with every field populated renders
// with no omission and a reflection partition over the model fails when a new
// field is added but neither rendered nor explicitly excluded (schema-drift
// guard, R2); and hostile charter prose renders inert (A-6). Touches is
// plan-internal (§4.5) and must never surface in a card.
package render

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

// stubBundle stands in for the pinned mermaid bytes on the direct-Page tests
// (maximal, escaping) that construct a Doc in memory: the card assertions do not
// depend on the diagram engine, only on the composed page.
var stubBundle = []byte("/* mermaid stub */")

func TestTaskDimensionsVisibleAndEscaped(t *testing.T) {
	doc := maximalDoc()
	r, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	r, err = r.Compile(taskrequirements.Sources(nil))
	if err != nil {
		t.Fatal(err)
	}
	doc.Tasks[0].Requirements = &r
	doc.Tasks[0].Requirements.Review.Rationale = "<script>explain()</script>"
	b, err := Page(doc, stubBundle)
	if err != nil {
		t.Fatal(err)
	}
	card := taskCard(string(b), "T1")
	for _, want := range []string{"implementation", "review", "reasoning=complex", "coupling=composed", "novelty=adapt", "verification=composed", "impact_scope=multi_surface", "architectural_role=shared_substrate", "capability=strong", "deliberation=deep", "context=large", "assurance=elevated", "&lt;script&gt;explain()&lt;/script&gt;"} {
		if !strings.Contains(card, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(card, "<script>") || strings.Contains(card, "rung") {
		t.Fatal("native card contains executable rationale or legacy route")
	}
}

// taskCard returns the substring of page from a task's anchor to the end of its
// article element — the bytes that belong to exactly that card.
func taskCard(page, id string) string {
	return section(page, `id="`+id+`"`, "</article>")
}

// Every fixture task's charter fields, badges, and traces appear in its own
// anchored card; the plan-internal Touches field never does (A-3 per-fixture).
func TestTaskCardsFullFixture(t *testing.T) {
	root, page := renderPage(t)
	doc := plan.FoldFile(root, config.Defaults(), planRel).Doc

	for i := range doc.Tasks {
		task := doc.Tasks[i]
		card := taskCard(page, task.ID)
		if card == "" {
			t.Fatalf("no anchored card for %s", task.ID)
		}

		// Charter prose + list fields: each populated value lands in the card.
		want := []string{task.Title, task.Goal, task.Current, task.Desired}
		want = append(want, task.Scope...)
		want = append(want, task.OutOfScope...)
		want = append(want, task.Acceptance...)
		want = append(want, task.KeyInterfaces...)
		want = append(want, task.Provenance...)
		want = append(want, task.Traces...)
		want = append(want, task.PlanTaskID)
		for _, s := range want {
			if s == "" {
				continue
			}
			if !strings.Contains(card, s) {
				t.Errorf("%s card missing charter value %q", task.ID, s)
			}
		}

		// Badges: mechanical/behavioral, band, foundation always; routing and
		// blocked-by when present.
		mech := "behavioral"
		if task.Mechanical {
			mech = "mechanical"
		}
		badges := []string{mech, task.Foundation.Level, task.Foundation.Reason}
		badges = append(badges, task.BlockedBy...)
		for _, b := range badges {
			if !strings.Contains(card, b) {
				t.Errorf("%s card missing badge value %q", task.ID, b)
			}
		}

		// Touches is plan-internal — never in the charter the reviewer reads.
		for _, tch := range task.Touches {
			if strings.Contains(card, tch) {
				t.Errorf("%s card leaked plan-internal Touches value %q", task.ID, tch)
			}
		}
	}
}

// Every task carries a stable in-page anchor id="T<n>", exactly once each.
func TestTaskAnchors(t *testing.T) {
	root, page := renderPage(t)
	doc := plan.FoldFile(root, config.Defaults(), planRel).Doc
	for i := range doc.Tasks {
		anchor := `id="` + doc.Tasks[i].ID + `"`
		if n := strings.Count(page, anchor); n != 1 {
			t.Errorf("anchor %s appears %d time(s), want exactly 1", anchor, n)
		}
	}
}

// maximalTask populates every charter field, badge, and the plan-internal
// Touches with a distinct sentinel so the rendered page can be audited for
// omissions (every rendered sentinel present) and leaks (Touches absent).
func maximalTask() plan.Task {
	return plan.Task{
		ID:              "T1",
		Title:           "MAXTITLE",
		Goal:            "MAXGOAL",
		Current:         "MAXCURRENT",
		Desired:         "MAXDESIRED",
		Scope:           []string{"MAXSCOPE1", "MAXSCOPE2"},
		OutOfScope:      []string{"MAXOOS1"},
		Acceptance:      []string{"MAXACC1", "MAXACC2"},
		KeyInterfaces:   []string{"MAXKIF1"},
		Provenance:      []string{"MAXPROV1"},
		Traces:          []string{"MAXTRACE1", "MAXTRACE2"},
		BlockedBy:       []string{"MAXDEP"},
		Touches:         []string{"MAXTOUCH-LEAK"},
		Band:            plan.SizeBand{Lo: 42, Hi: 99},
		Mechanical:      false,
		SizeAccepted:    "MAXSIZEACC",
		OverlapAccepted: "MAXOVLACC",

		Pattern:    "MAXPATTERN",
		PlanTaskID: "MAXPTID",
		Foundation: plan.Foundation{Level: "medium", Reason: "MAXFDNREASON"},
	}
}

func maximalDoc() *plan.Doc {
	return &plan.Doc{
		Schema:      plan.SchemaVersion,
		Tool:        plan.ToolVersion,
		Title:       "Maximal plan",
		Feature:     "maximal",
		Version:     "001",
		Dialect:     plan.DialectHarbormaster,
		Status:      "Draft",
		Fingerprint: "sha256:max",
		Tasks:       []plan.Task{maximalTask()},
	}
}

// A maximal task renders with no omission: every populated charter field and
// every badge appears; the plan-internal Touches value does not (A-3 maximal).
func TestMaximalTaskNoOmission(t *testing.T) {
	b, err := Page(maximalDoc(), stubBundle)
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	card := taskCard(page, "T1")

	rendered := []string{
		"MAXTITLE", "MAXGOAL", "MAXCURRENT", "MAXDESIRED",
		"MAXSCOPE1", "MAXSCOPE2", "MAXOOS1", "MAXACC1", "MAXACC2",
		"MAXKIF1", "MAXPROV1", "MAXTRACE1", "MAXTRACE2", "MAXDEP",
		"MAXSIZEACC", "MAXOVLACC", "MAXPATTERN", "MAXPTID",
		"MAXFDNREASON", "medium", // foundation level + reason
		"42–99", // band lo–hi
		// routing
	}
	for _, s := range rendered {
		if !strings.Contains(card, s) {
			t.Errorf("maximal card omits %q", s)
		}
	}
	if strings.Contains(page, "MAXTOUCH-LEAK") {
		t.Error("maximal card leaked plan-internal Touches value")
	}
}

// Schema-drift guard: every exported field on plan.Task is either rendered in
// the card or explicitly excluded with a reason. A field added to the model but
// wired into neither set fails here — the render decision cannot be skipped
// silently (R2).
func TestTaskFieldsPartitioned(t *testing.T) {
	// Rendered on the card (charter fields, badges, traces, identity/anchor).
	rendered := map[string]bool{
		"ID": true, "Title": true, "Goal": true, "Current": true, "Desired": true,
		"Scope": true, "OutOfScope": true, "Acceptance": true, "KeyInterfaces": true,
		"Provenance": true, "Traces": true, "BlockedBy": true, "Band": true,
		"Mechanical": true, "SizeAccepted": true, "OverlapAccepted": true,
		"Rung": true, "Requirements": true, "Pattern": true, "PlanTaskID": true, "Foundation": true,
	}
	// Deliberately not on the card, each with a reason.
	excluded := map[string]string{
		"Touches": "plan-internal overlap sequencing (§4.5) — never in the charter",
	}

	for field := range reflect.TypeFor[plan.Task]().Fields() {
		name := field.Name
		if !field.IsExported() {
			continue
		}
		if rendered[name] == (excluded[name] != "") {
			// In both sets, or in neither.
			t.Errorf("plan.Task field %q is not partitioned: render it in the card "+
				"or add it to the excluded set with a reason", name)
		}
	}
}

// Hostile markup in any charter field renders inert — escaped, never live
// (A-6). The reviewer's page cannot be turned into an attack surface by plan
// prose.
func TestTaskCardEscaping(t *testing.T) {
	const evil = `<script>alert(1)</script><img src=x onerror=alert(1)> & <b>x</b>`
	task := maximalTask()
	task.Goal = evil
	task.Title = evil
	task.Scope = []string{evil}
	task.Acceptance = []string{evil}
	task.Traces = []string{evil}
	task.KeyInterfaces = []string{evil}
	doc := maximalDoc()
	doc.Tasks = []plan.Task{task}

	b, err := Page(doc, stubBundle)
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)

	for _, live := range []string{
		"<script>alert(1)</script>", "<img src=x onerror=alert(1)>", "<b>x</b>",
	} {
		if strings.Contains(page, live) {
			t.Errorf("charter markup rendered live (not escaped): %q", live)
		}
	}
	if !strings.Contains(page, "&lt;script&gt;") || !strings.Contains(page, "&amp;") {
		t.Error("charter markup was not escaped inert")
	}
}
