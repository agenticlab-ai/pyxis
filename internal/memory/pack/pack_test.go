package pack

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

var t0 = time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

type tree struct {
	t     *testing.T
	files store.MapSource
	n     byte
}

func (tr *tree) add(kind, subject, family, statement string, rels ...record.Relation) *record.Record {
	tr.n++
	var e [10]byte
	e[9] = tr.n
	r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, t0.Add(time.Duration(tr.n)*time.Second), e), Kind: kind, Subject: subject,
		Family: family, Statement: statement, Intent: "intent " + family, Rationale: "why " + family, FitCriterion: "fit " + family, Issuer: "m@example.com",
		CreatedAt: t0.Format(time.RFC3339), Relations: rels}
	b, err := record.Marshal(r)
	if err != nil {
		tr.t.Fatal(err)
	}
	tr.files[r.Path()] = b
	return r
}

func (tr *tree) projection() *resolve.Projection {
	s, err := store.Load(context.Background(), tr.files)
	if err != nil {
		tr.t.Fatal(err)
	}
	return resolve.New(s)
}

func rel(t, target string) record.Relation { return record.Relation{Type: t, Target: target} }

func fixture(t *testing.T) (*tree, map[string]*record.Record) {
	tr := &tree{t: t, files: store.MapSource{}}
	ids := map[string]*record.Record{}
	ids["fr1"] = tr.add(record.Promise, "prd/x", "FR-1", "one")
	ids["fr2"] = tr.add(record.Promise, "prd/x", "FR-2", "two")
	ids["dod"] = tr.add(record.Expectation, "prd/x", "DOD-1", "done when one and two")
	ids["goal"] = tr.add(record.Need, "prd/x", "G-1", "the goal")
	ids["anchor"] = tr.add(record.Need, "design/base", "anchor", "the base design exists")
	ids["d1"] = tr.add(record.Decision, "design/x", "D-1", "serve one", rel(record.Serves, ids["fr1"].ID), rel(record.DependsOn, ids["anchor"].ID))
	ids["pol"] = tr.add(record.Policy, "governance/principles", "AP-1", "every executable artifact is testable")
	ids["q1"] = tr.add(record.Question, "prd/x", "", "which colour?")
	ids["a1"] = tr.add(record.Answer, "prd/x", "", "blue", rel(record.Answers, ids["q1"].ID))
	ids["q2"] = tr.add(record.Question, "prd/x", "", "which size?")
	ids["learn"] = tr.add(record.Learning, "prd/x", "", "we learned")
	ids["other"] = tr.add(record.Promise, "prd/y", "FR-1", "unrelated subject")
	return tr, ids
}

func TestCompileCollectsEverySection(t *testing.T) {
	tr, ids := fixture(t)
	fr2b := tr.add(record.Promise, "prd/x", "FR-2", "two, corrected", rel(record.Supersedes, ids["fr2"].ID))
	stale := tr.add(record.Decision, "design/x", "D-2", "serves the old two", rel(record.Serves, ids["fr2"].ID))
	p := tr.projection()
	other := &tree{t: t, files: store.MapSource{}, n: 50}
	inflight := other.add(record.Promise, "prd/x", "FR-1", "one from another branch")
	other.add(record.Promise, "prd/z", "FR-9", "elsewhere")
	os, err := store.Load(context.Background(), other.files)
	if err != nil {
		t.Fatal(err)
	}
	pk, err := Compile(p, map[string]*store.Set{"feature/b": os}, Scope{Subjects: []string{"prd/x"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Requirements) != 4 || pk.Requirements[0].ID != ids["dod"].ID { // DOD, G-1, FR-1, FR-2b in id order
		t.Fatalf("requirements %+v", pk.Requirements)
	}
	if len(pk.Decisions) != 2 { // d1 and the anchor it depends on; the stale decision serves the superseded FR-2, so it is a reopen, not a serving decision
		t.Fatalf("decisions %+v", pk.Decisions)
	}
	if len(pk.Policies) != 1 || pk.Policies[0].ID != ids["pol"].ID {
		t.Fatalf("policies %+v", pk.Policies)
	}
	if len(pk.Open) != 1 || pk.Open[0].ID != ids["q2"].ID || len(pk.Answered) != 1 || !strings.Contains(pk.Answered[0].Note, "blue") {
		t.Fatalf("questions %+v %+v", pk.Open, pk.Answered)
	}
	if len(pk.Reopened) != 1 || pk.Reopened[0].ID != stale.ID || !strings.Contains(pk.Reopened[0].Note, fr2b.ID) {
		t.Fatalf("reopened %+v", pk.Reopened)
	}
	if len(pk.Concurrent) != 1 || pk.Concurrent[0].ID != inflight.ID || pk.Concurrent[0].Note != "in flight on feature/b" {
		t.Fatalf("concurrent %+v", pk.Concurrent)
	}
	if len(pk.Learnings) != 1 || len(pk.AdjacentNotLoaded) != 0 || len(pk.Omissions) != 0 || pk.Used != record.Tokens(pk.Render()) {
		t.Fatalf("pack %+v", pk)
	}
	md := pk.Render()
	for _, want := range []string{"# Task pack — prd/x", "## Requirements (the task)", "- **Promise FR-1** (prd/x, ", "  - intent: intent FR-1", "  - fit criterion: fit FR-1",
		"## Decisions that serve them", "  - serves: prd/x#FR-1", "## Policies that bind", "## Open questions — ask before assuming", "which size?",
		"## Already answered — do not ask again", "answer (m@example.com, 2026-09-03): blue", "## Reopened — decisions whose claim moved", "the head is now " + fr2b.ID,
		"## Concurrent — other branches on these families", "in flight on feature/b", "## Learnings", "we learned", "- nothing: the pack is complete for its scope"} {
		if !strings.Contains(md, want) {
			t.Fatalf("render lacks %q:\n%s", want, md)
		}
	}
}

func TestADesignSubjectBringsItsDecisions(t *testing.T) {
	tr, ids := fixture(t)
	p := tr.projection()
	pk, err := Compile(p, nil, Scope{Subjects: []string{"design/x"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Requirements) != 0 || len(pk.Decisions) != 2 || pk.Decisions[0].ID != ids["d1"].ID || pk.Decisions[1].ID != ids["anchor"].ID {
		t.Fatalf("a design in scope loads its decisions: %+v %+v", pk.Requirements, pk.Decisions)
	}
	if !strings.Contains(pk.Render(), "- **Decision D-1** (design/x, "+ids["d1"].ID+")") {
		t.Fatalf("render:\n%s", pk.Render())
	}
	pk, _ = Compile(p, nil, Scope{Subjects: []string{"design/x"}, Families: []string{"D-9"}}, 100000, nil)
	if len(pk.Decisions) != 0 || strings.Join(pk.AdjacentNotLoaded, ",") != "design/x#D-1" {
		t.Fatalf("family scoping applies to decisions too: %+v %v", pk.Decisions, pk.AdjacentNotLoaded)
	}
}

func TestLiteralDocumentBringsItsNarrativeWithoutClaimMarkers(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	body := []byte("# Legacy contract\n\n## Template\n\nKeep `${{ github.sha }}` and `{{package_name}}` byte-for-byte.\n")
	d := tr.add(record.Document, "prd/legacy", "prd", "Legacy contract")
	d.LiteralBody = true
	d.BodySHA256 = record.HashBody(body)
	b, err := record.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	tr.files[d.Path()] = b
	tr.files[d.BodyPath()] = body

	pk, err := Compile(tr.projection(), nil, Scope{Subjects: []string{"prd/legacy"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Narrative) != 1 || !strings.Contains(pk.Render(), "Keep `${{ github.sha }}` and `{{package_name}}` byte-for-byte.") {
		t.Fatalf("literal source narrative is missing from the pack: %+v\n%s", pk.Narrative, pk.Render())
	}
	if len(pk.Omissions) != 0 {
		t.Fatalf("an ample budget must carry the complete imported narrative: %v", pk.Omissions)
	}
}

func TestScopeFamiliesAndBudget(t *testing.T) {
	tr, ids := fixture(t)
	p := tr.projection()
	pk, err := Compile(p, nil, Scope{Subjects: []string{"prd/x"}, Families: []string{"FR-1"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Requirements) != 1 || pk.Requirements[0].ID != ids["fr1"].ID {
		t.Fatalf("requirements %+v", pk.Requirements)
	}
	if strings.Join(pk.AdjacentNotLoaded, ",") != "prd/x#DOD-1,prd/x#FR-2,prd/x#G-1" {
		t.Fatalf("adjacent %v", pk.AdjacentNotLoaded)
	}
	if len(pk.Decisions) != 2 {
		t.Fatalf("decisions %+v", pk.Decisions)
	}
	pk, _ = Compile(p, nil, Scope{Subjects: []string{"prd/x"}, Families: []string{"prd/x#FR-2"}}, 100000, nil)
	if len(pk.Requirements) != 1 || pk.Requirements[0].ID != ids["fr2"].ID || len(pk.Decisions) != 0 {
		t.Fatalf("subject#family scope %+v", pk.Requirements)
	}
	if !strings.Contains(pk.Render(), "- no decision serves these claims: they are undesigned") || !strings.Contains(pk.Render(), "adjacent, not requested: prd/x#FR-1") {
		t.Fatalf("render:\n%s", pk.Render())
	}
	// the budget bounds the delivered rendering, metadata included; the lowest priority goes first
	// the header prints the budget, so the reference is measured with a budget of the same width as the tight one
	full, err := Compile(p, nil, Scope{Subjects: []string{"prd/x"}}, 999, nil)
	if err != nil || full.Used != record.Tokens(full.Render()) || len(full.Omissions) != 0 {
		t.Fatalf("full %+v %v", full, err)
	}
	tight, err := Compile(p, nil, Scope{Subjects: []string{"prd/x"}}, full.Used-1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tight.Used > full.Used-1 || tight.Used != record.Tokens(tight.Render()) || len(tight.Learnings) != 1 || len(tight.Requirements) != 4 ||
		!strings.Contains(strings.Join(tight.Omissions, ";"), "answered questions: 1 of 1 omitted for budget") {
		t.Fatalf("one over budget drops the lowest priority first: used %d of %d, %+v", tight.Used, full.Used-1, tight.Omissions)
	}
	if !strings.Contains(tight.Render(), "- omitted: answered questions: 1 of 1 omitted for budget") {
		t.Fatalf("render:\n%s", tight.Render())
	}
	minimal, err := Compile(p, nil, Scope{Subjects: []string{"prd/x"}}, 350, nil)
	if err != nil || minimal.Used > 350 || len(minimal.Requirements) == 4 || len(minimal.Policies) != 1 {
		t.Fatalf("a tight budget cuts requirements too, and still fits: %+v %v", minimal, err)
	}
	if _, err := Compile(p, nil, Scope{Subjects: []string{"prd/x"}}, 5, nil); err == nil || !strings.Contains(err.Error(), "too small") {
		t.Fatalf("an infeasible budget is an error: %v", err)
	}
	for _, b := range []int{0, -1} {
		if _, err := Compile(p, nil, Scope{Subjects: []string{"prd/x"}}, b, nil); err == nil || !strings.Contains(err.Error(), "positive") {
			t.Fatalf("budget %d: %v", b, err)
		}
	}
	// the measure is the caller's: a JSON consumer budgets the JSON
	counted, err := Compile(p, nil, Scope{Subjects: []string{"prd/x"}}, 1, func(*Pack) int { return 1 })
	if err != nil || counted.Used != 1 || len(counted.Omissions) != 0 {
		t.Fatalf("custom measure: %+v %v", counted, err)
	}
	empty, err := Compile(p, nil, Scope{Subjects: []string{"prd/none"}}, 100000, nil)
	if err != nil || len(empty.Requirements) != 0 || !strings.Contains(empty.Render(), "- no active claims in scope") {
		t.Fatalf("empty scope %+v %v", empty, err)
	}
}

func TestNarrativeSections(t *testing.T) {
	tr, ids := fixture(t)
	body := "Preamble mentioning {{FR-1}}.\n\n# Goals\n\nNothing embedded here.\n\n## Frontier\n\nThe frontier serves {{FR-1}} and {{prd/x#DOD-1}}.\n\n## Other\n\nAbout {{FR-2}} only.\n"
	d := tr.add(record.Document, "prd/x", "prd", "The PRD")
	d.BodySHA256, d.Embeds = record.HashBody([]byte(body)), record.ParseEmbeds([]byte(body), "prd/x")
	b, _ := record.Marshal(d)
	tr.files[d.Path()], tr.files[d.BodyPath()] = b, []byte(body)
	p := tr.projection()
	pk, err := Compile(p, nil, Scope{Subjects: []string{"prd/x"}, Families: []string{"FR-1"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Narrative) != 2 || pk.Narrative[0].Statement != "The PRD › preamble" || pk.Narrative[1].Statement != "The PRD › Frontier" || !strings.Contains(pk.Narrative[1].Note, "{{prd/x#DOD-1}}") {
		t.Fatalf("narrative %+v", pk.Narrative)
	}
	for _, r := range pk.Requirements {
		if r.Kind == record.Document {
			t.Fatal("documents are narrative, not requirements")
		}
	}
	if !strings.Contains(pk.Render(), "## Narrative around the task") || !strings.Contains(pk.Render(), "- **Document prd** (prd/x, "+d.ID+"): The PRD › Frontier") {
		t.Fatalf("render:\n%s", pk.Render())
	}
	// a document on another subject narrating an in-scope claim, and a marker that sits in a heading
	other := tr.add(record.Document, "design/x", "design", "The design")
	obody := "# Design\n\n## {{prd/x#FR-1}} in the executor\n\nHow the frontier honours it.\n\n## Unrelated\n\nNothing here.\n"
	other.BodySHA256, other.Embeds = record.HashBody([]byte(obody)), record.ParseEmbeds([]byte(obody), "design/x")
	ob, _ := record.Marshal(other)
	tr.files[other.Path()], tr.files[other.BodyPath()] = ob, []byte(obody)
	p = tr.projection()
	pk, err = Compile(p, nil, Scope{Subjects: []string{"prd/x"}, Families: []string{"FR-1"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, it := range pk.Narrative {
		titles = append(titles, it.Statement)
	}
	if strings.Join(titles, "|") != "The PRD › preamble|The PRD › Frontier|The design › {{prd/x#FR-1}} in the executor" {
		t.Fatalf("narrative across subjects and headings: %v", titles)
	}
	if s := docSections("no headings at all\n"); len(s) != 1 || s[0].title != "preamble" {
		t.Fatalf("sections %+v", s)
	}
	if s := docSections("# A\n\n# B\ntext\n"); len(s) != 1 || s[0].title != "B" {
		t.Fatalf("empty sections are dropped: %+v", s)
	}
	fenced := "# Runtime\n\nThis serves {{FR-1}}.\n\n````markdown\n## example\n```sh\n# setup comment\necho ready\n```\n````\n\nAfter the example.\n#not-a-heading\n"
	if s := docSections(fenced); len(s) != 1 || s[0].title != "Runtime" || !strings.Contains(s[0].text, "# setup comment") || !strings.Contains(s[0].text, "After the example") || !strings.Contains(s[0].text, "#not-a-heading") {
		t.Fatalf("fenced examples and non-headings stay in their section: %+v", s)
	}
	if s := docSections("   ### Indented ###\ntext\n"); len(s) != 1 || s[0].title != "Indented" {
		t.Fatalf("valid ATX heading: %+v", s)
	}
	_ = ids
}

func TestDesignScopeIncludesDecisionAndNarrative(t *testing.T) {
	tr, ids := fixture(t)
	body := "# Design\n\n## Export path\n\nThe implementation follows {{D-1}}.\n"
	d := tr.add(record.Document, "design/x", "design", "The design")
	d.BodySHA256, d.Embeds = record.HashBody([]byte(body)), record.ParseEmbeds([]byte(body), "design/x")
	b, err := record.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	tr.files[d.Path()], tr.files[d.BodyPath()] = b, []byte(body)

	p := tr.projection()
	pk, err := Compile(p, nil, Scope{Subjects: []string{"design/x"}, Families: []string{"D-1"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Requirements) != 0 {
		t.Fatalf("a scoped Decision is not a requirement: %+v", pk.Requirements)
	}
	if len(pk.Decisions) != 2 || pk.Decisions[0].ID != ids["d1"].ID || pk.Decisions[1].ID != ids["anchor"].ID {
		t.Fatalf("decision and its dependency: %+v", pk.Decisions)
	}
	if len(pk.Narrative) != 1 || pk.Narrative[0].ID != d.ID || pk.Narrative[0].Statement != "The design › Export path" {
		t.Fatalf("decision narrative: %+v", pk.Narrative)
	}
	md := pk.Render()
	if !strings.Contains(md, "**Decision D-1**") || !strings.Contains(md, "The design › Export path") {
		t.Fatalf("design pack rendered as empty:\n%s", md)
	}

	// If the requirement and its design are both scoped, the Decision is direct
	// and derived through serves, but must still appear exactly once.
	pk, err = Compile(p, nil, Scope{Subjects: []string{"prd/x", "design/x"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, it := range pk.Decisions {
		if it.ID == ids["d1"].ID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("direct and serving Decision appears %d times: %+v", count, pk.Decisions)
	}
}

func TestEveryDirectDecisionExpandsItsOwnDependencies(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	leaf := tr.add(record.Need, "design/base", "anchor", "dependency of the second decision")
	first := tr.add(record.Decision, "design/x", "D-1", "first decision")
	second := tr.add(record.Decision, "design/x", "D-2", "second decision", rel(record.DependsOn, leaf.ID))
	first.Relations = []record.Relation{rel(record.DependsOn, second.ID)}
	b, err := record.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	tr.files[first.Path()] = b

	pk, err := Compile(tr.projection(), nil, Scope{Subjects: []string{"design/x"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, decision := range pk.Decisions {
		seen[decision.ID] = true
	}
	if len(pk.Decisions) != 3 || !seen[first.ID] || !seen[second.ID] || !seen[leaf.ID] {
		t.Fatalf("each scoped decision needs its one-depth dependency closure: %+v", pk.Decisions)
	}
}

func TestConcurrentIncludesOnlyActiveBranchHeads(t *testing.T) {
	tr, _ := fixture(t)
	branch := &tree{t: t, files: store.MapSource{}, n: 50}
	v1 := branch.add(record.Promise, "prd/x", "FR-1", "old branch wording")
	v2 := branch.add(record.Promise, "prd/x", "FR-1", "current branch wording", rel(record.Supersedes, v1.ID))
	bs, err := store.Load(context.Background(), branch.files)
	if err != nil {
		t.Fatal(err)
	}

	pk, err := Compile(tr.projection(), map[string]*store.Set{"feature/b": bs}, Scope{Subjects: []string{"prd/x"}, Families: []string{"FR-1"}}, 100000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Concurrent) != 1 || pk.Concurrent[0].ID != v2.ID {
		t.Fatalf("concurrent records must be active branch heads: %+v", pk.Concurrent)
	}
}
