package view

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
		Family: family, Statement: statement, Issuer: "m@example.com", CreatedAt: t0.Format(time.RFC3339), Relations: rels}
	if kind == record.Assumption {
		r.Expires = t0.Add(time.Hour).Format(time.RFC3339)
	}
	return tr.put(r)
}

func (tr *tree) put(r *record.Record) *record.Record {
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

func mustRender(t *testing.T, p *resolve.Projection, subject, slot, source string) string {
	t.Helper()
	got, err := Render(p, subject, slot, source)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestPRDView(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	fr10 := tr.add(record.Promise, "prd/x", "FR-10", "ten")
	fr2 := tr.add(record.Promise, "prd/x", "FR-2", "two")
	fr2.Intent, fr2.Rationale, fr2.FitCriterion, fr2.LearnedFrom, fr2.Alternatives, fr2.Tags = "i", "r", "f", []string{"l"}, []string{"a"}, []string{"t"}
	tr.put(fr2)
	fr2b := tr.add(record.Promise, "prd/x", "FR-2", "two, corrected", rel(record.Supersedes, fr2.ID))
	dod := tr.add(record.Expectation, "prd/x", "DOD-1", "done")
	tr.add(record.Need, "prd/x", "G-1", "goal")
	tr.add(record.Decision, "design/x", "D-1", "serves ten", rel(record.Serves, fr10.ID))
	as := tr.add(record.Assumption, "prd/x", "A-1", "assumed")
	tr.add(record.Policy, "prd/x", "P-1", "rule")
	q := tr.add(record.Question, "prd/x", "", "which?")
	tr.add(record.Answer, "prd/x", "", "this", rel(record.Answers, q.ID))
	tr.add(record.Question, "prd/x", "", "and this?")
	tr.add(record.Learning, "prd/x", "", "lesson")
	rv := tr.add(record.Promise, "prd/x", "FR-3", "three")
	tr.add(record.Revocation, "prd/x", "", "gone", rel(record.Revokes, rv.ID))
	p := tr.projection()
	md := PRD(p, "prd/x", map[string]bool{fr2b.ID: true, dod.ID: true})
	for _, want := range []string{"# prd/x", "history: 1 superseded, 1 revoked", "**Changed since the reference**: 2 records new here", "## Needs", "### G-1", "## Promises",
		"### FR-2 **(replaced)**", "two, corrected", "· 1 earlier version(s)", "### FR-10", " · served by design/x#D-1", " · **undesigned**", "## Expectations", "### DOD-1 **(new)**",
		"## Assumptions", "- expires: 2026-09-03", "## Policies", "### P-1", "## Questions", "- **open** (", "and this?", "which? → **this** (m@example.com, 2026-09-03)", "## Learnings", "- 2026-09-03: lesson"} {
		if !strings.Contains(md, want) {
			t.Fatalf("PRD lacks %q:\n%s", want, md)
		}
	}
	if strings.Index(md, "### FR-2") > strings.Index(md, "### FR-10") {
		t.Fatal("families sort numerically")
	}
	if strings.Contains(md, "three") {
		t.Fatal("revoked claims are not shown")
	}
	_ = as
	if !strings.Contains(PRD(p, "prd/x", nil), "11 active records") || strings.Contains(PRD(p, "prd/x", nil), "Changed since") {
		t.Fatalf("plain view:\n%s", PRD(p, "prd/x", nil))
	}
	if !strings.Contains(PRD(p, "prd/x", map[string]bool{"nope": true}), "(nothing)") {
		t.Fatal("reference with no changes")
	}
}

func TestDesignWhyDeliveryStatus(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	fr1 := tr.add(record.Promise, "prd/x", "FR-1", "one")
	fr1.Provenance = record.Provenance{Source: "docs/prds/x/000-base.md", PR: 12}
	tr.put(fr1)
	fr2 := tr.add(record.Promise, "prd/x", "FR-2", "two")
	anchor := tr.add(record.Need, "code/executor", "anchor", "package executor exists")
	base := tr.add(record.Need, "design/base", "anchor", "base design")
	d1 := tr.add(record.Decision, "design/x", "D-1", "serves one and two", rel(record.Serves, fr1.ID), rel(record.Serves, fr2.ID), rel(record.About, anchor.ID), rel(record.DependsOn, base.ID))
	d1.Intent = "keep it simple"
	tr.put(d1)
	d2 := tr.add(record.Decision, "design/x", "D-2", "serves a ghost", rel(record.Serves, "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV"))
	fr1b := tr.add(record.Promise, "prd/x", "FR-1", "one, corrected", rel(record.Supersedes, fr1.ID))
	fr1b.Intent, fr1b.Rationale = "verifiable", "planning found it unverifiable"
	tr.put(fr1b)
	fr3 := tr.add(record.Promise, "prd/x", "FR-3", "three, undesigned")
	tr.add(record.Need, "design/x", "anchor", "design x exists")
	dead := tr.add(record.Promise, "prd/x", "FR-4", "four")
	d3 := tr.add(record.Decision, "design/x", "D-3", "serves four only", rel(record.Serves, dead.ID))
	tr.add(record.Revocation, "prd/x", "", "four was never built", rel(record.Revokes, dead.ID))
	tr.add(record.Question, "prd/x", "", "open?")
	old := tr.add(record.Assumption, "prd/x", "A-1", "assumed")
	dup := tr.add(record.Promise, "prd/x", "FR-3", "three again")
	tr.files["prd/x/bad.json"] = []byte("{")
	p := tr.projection()

	design := Design(p, "design/x")
	for _, want := range []string{"# design/x", "## Anchors", "- anchor: design x exists", "## Decisions", "### D-1", "- intent: keep it simple", "- serves prd/x#FR-1: one **(reopened: the claim is superseded; the head is now " + fr1b.ID + ")**",
		"- serves prd/x#FR-2: two\n", "- about code/executor", "- depends on design/base#anchor", "- serves Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV **(missing)**", "### D-3"} {
		if !strings.Contains(design, want) {
			t.Fatalf("design lacks %q:\n%s", want, design)
		}
	}
	if !strings.Contains(Design(p, "design/none"), "- no active decisions") {
		t.Fatal("empty design")
	}
	why := Why(p, fr1b.ID)
	for _, want := range []string{"# why prd/x#FR-1 says what it says", "## now · active · 2026-09-03 · " + fr1b.ID, "- intent: verifiable", "- why: planning found it unverifiable",
		"## v-1 · superseded · 2026-09-03 · " + fr1.ID, "- from: docs/prds/x/000-base.md (PR #12)"} {
		if !strings.Contains(why, want) {
			t.Fatalf("why lacks %q:\n%s", want, why)
		}
	}
	if w := Why(p, fr2.ID); !strings.Contains(w, "## Rests on it now") || !strings.Contains(w, "- serves design/x#D-1 (Decision)") {
		t.Fatalf("why of a served claim lists what rests on it:\n%s", w)
	}
	if !strings.Contains(Why(p, "nope"), "no such record") {
		t.Fatal("why of a missing id")
	}
	delivery := Delivery(p, "prd/x")
	// FR-1's head is the corrected claim, which D-1 does not serve yet (it serves the old one): undesigned until re-pointed
	for _, want := range []string{"# delivery — prd/x", "| FR-1 |  | **undesigned** |", "| FR-2 | design/x#D-1 | designed |", "| FR-3 |  | **undesigned** |", "1 of 4 claims designed; 3 undesigned"} {
		if !strings.Contains(delivery, want) {
			t.Fatalf("delivery lacks %q:\n%s", want, delivery)
		}
	}
	_ = fr3
	status := Status(p, t0.Add(2*time.Hour))
	for _, want := range []string{"# memory status", "| prd/x |", "## Needs a human", "- invalid file prd/x/bad.json", "- dangling: " + d2.ID + " serves Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV", "- conflict: prd/x#FR-3 has 2 active claims",
		"- reopened: design/x#D-1 serves " + fr1.ID, "- orphaned: design/x#D-3 serves nothing that is still wanted", "- open question on prd/x since 2026-09-03: open?", "- expired assumption prd/x#A-1"} {
		if !strings.Contains(status, want) {
			t.Fatalf("status lacks %q:\n%s", want, status)
		}
	}
	_, _, _ = d3, old, dup
	bad := &tree{t: t, files: store.MapSource{}}
	one := bad.add(record.Promise, "prd/x", "FR-1", "one")
	bad.add(record.Promise, "prd/y", "FR-1", "elsewhere", rel(record.Supersedes, one.ID))
	qq := bad.add(record.Question, "prd/x", "", "which?")
	bad.add(record.Answer, "prd/x", "", "a", rel(record.Answers, qq.ID))
	bad.add(record.Answer, "prd/x", "", "b", rel(record.Answers, qq.ID))
	withdrawn := bad.add(record.Promise, "prd/x", "FR-4", "withdrawn twice")
	bad.add(record.Revocation, "prd/x", "", "first withdrawal", rel(record.Revokes, withdrawn.ID))
	bad.add(record.Revocation, "prd/x", "", "second withdrawal", rel(record.Revokes, withdrawn.ID))
	if s := Status(bad.projection(), t0); !strings.Contains(s, "- supersedes-across-subjects: ") || !strings.Contains(s, "- answer-conflict: "+qq.ID+" has 2 active answers") || !strings.Contains(s, "- revocation-conflict: "+withdrawn.ID+" has 2 active revocations") {
		t.Fatalf("status lists violations and act conflicts:\n%s", s)
	}
	clean := &tree{t: t, files: store.MapSource{}}
	clean.add(record.Promise, "prd/x", "FR-1", "one")
	if s := Status(clean.projection(), t0); !strings.Contains(s, "- nothing") {
		t.Fatalf("clean status:\n%s", s)
	}
	if clip("abcdef", 4) != "abc…" || plural(record.Policy) != "Policies" || plural(record.Need) != "Needs" {
		t.Fatal("helpers")
	}
	if pfx, n := splitFamily("FR-12"); pfx != "FR" || n != 12 {
		t.Fatal("splitFamily")
	}
	if pfx, n := splitFamily("anchor"); pfx != "anchor" || n != 0 {
		t.Fatal("splitFamily plain")
	}
	if pfx, n := splitFamily("§6.x"); pfx != "§6.x" || n != 0 {
		t.Fatal("splitFamily non-numeric suffix")
	}
}

func (tr *tree) doc(subject, slot, title, body string, n byte, rels ...record.Relation) *record.Record {
	r := tr.add(record.Document, subject, slot, title, rels...)
	r.BodySHA256 = record.HashBody([]byte(body))
	r.Embeds = record.ParseEmbeds([]byte(body), subject)
	tr.put(r)
	tr.files[r.BodyPath()] = []byte(body)
	return r
}

func TestRenderDocument(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	fr1 := tr.add(record.Promise, "prd/x", "FR-1", "one PR per slice")
	fr1.Intent, fr1.Rationale, fr1.FitCriterion = "small reviews", "large PRs were skimmed", "three slices give three PRs"
	tr.put(fr1)
	fr2 := tr.add(record.Promise, "prd/x", "FR-2", "retarget on merge")
	tr.add(record.Need, "prd/x", "G-1", "cheap review")
	tr.add(record.Decision, "design/x", "D-1", "a stack manifest", rel(record.Serves, fr1.ID))
	q := tr.add(record.Question, "prd/x", "", "which colour?")
	tr.add(record.Answer, "prd/x", "", "blue", rel(record.Answers, q.ID))
	tr.add(record.Learning, "prd/x", "", "lesson")
	v1 := tr.doc("prd/x", "prd", "Stacked PRs", "# Stacked PRs\n\nWe want {{FR-1}} because reviewers skim.\n\n{{FR-1}}\n\nAlso {{prd/x#FR-9}} and {{design/x#D-1}}.\n", 0)
	v2 := tr.doc("prd/x", "prd", "Stacked PRs", "# Stacked PRs\n\nRewritten: {{FR-1}}.\n\n{{ FR-2 }}\n", 0, rel(record.Supersedes, v1.ID))
	v2.Intent = "tighten the narrative"
	tr.put(v2)
	p := tr.projection()
	if got := mustRender(t, p, "prd/x", "prd", "the working tree"); true {
		for _, want := range []string{"> **prd/x · prd** · version 2 · since 2026-09-03 by m@example.com · rendered from the working tree",
			"Rewritten: **FR-1** (one PR per slice).", "> **FR-2** · retarget on merge\n>\n> _since 2026-09-03 · **undesigned**_",
			"## Claims this document does not cover", "- **G-1** (Need): cheap review", "## Questions", "which colour? → **blue**", "## Learnings", "lesson",
			"## Document history", "- v2 · 2026-09-03 · m@example.com · " + v2.ID + " · tighten the narrative", "- v1 · 2026-09-03 · m@example.com · " + v1.ID} {
			if !strings.Contains(got, want) {
				t.Fatalf("render lacks %q:\n%s", want, got)
			}
		}
		if strings.Contains(got, "FR-9") || strings.Contains(got, "{{") {
			t.Fatalf("v2 embeds only FR-1 and FR-2 and every marker expands:\n%s", got)
		}
	}
	// Revoking v2 would reactivate v1 with an unresolved embed. The projection
	// refuses that act, leaves v2 current, and makes the reason visible.
	revocation := tr.add(record.Revocation, "prd/x", "", "withdrawn", rel(record.Revokes, v2.ID))
	p = tr.projection()
	got := mustRender(t, p, "prd/x", "prd", "x")
	if p.Status(v2.ID) != resolve.Active || len(p.Violations) != 1 || p.Violations[0].Record != revocation.ID || p.Violations[0].Code != "revocation-breaks-projection" {
		t.Fatalf("unsafe revocation projection: status=%s violations=%+v", p.Status(v2.ID), p.Violations)
	}
	if !strings.Contains(got, "version 2") || strings.Contains(got, "version 1 · since") || strings.Contains(got, "FR-9") {
		t.Fatalf("an unsafe revocation changed the rendered head:\n%s", got)
	}
	if _, err := Render(p, "prd/x", "design", "x"); err == nil || !strings.Contains(err.Error(), "has no active design document") {
		t.Fatalf("missing design document: %v", err)
	}
	if _, err := Render(p, "prd/none", "prd", "x"); err == nil || !strings.Contains(err.Error(), "no active prd document") {
		t.Fatal("missing document message")
	}
	// a marker broken across lines is not a marker: it stays raw and the claim it meant is appended as uncovered
	tr.doc("prd/x", "prd", "Stacked PRs", "# Stacked PRs\n\n{{\nFR-1\n}}\n", 0, rel(record.Supersedes, tr.projection().Head("prd/x", "prd").ID))
	p = tr.projection()
	raw := mustRender(t, p, "prd/x", "prd", "x")
	if !strings.Contains(raw, "{{\nFR-1\n}}") || !strings.Contains(raw, "## Claims this document does not cover") || !strings.Contains(raw, "- **FR-1** (Promise): one PR per slice") {
		t.Fatalf("multiline marker:\n%s", raw)
	}
	// a design document that leaves out a decision, an assumption or a policy still shows them
	tr.add(record.Decision, "design/x", "D-2", "a second decision the design narrative forgot")
	tr.add(record.Assumption, "design/x", "A-1", "assumed for the design")
	tr.add(record.Policy, "design/x", "P-1", "a rule on the design")
	tr.doc("design/x", "design", "The design", "# The design\n\n{{D-1}}\n", 0)
	p = tr.projection()
	design := mustRender(t, p, "design/x", "design", "x")
	for _, want := range []string{"## Claims this document does not cover", "- **D-2** (Decision): a second decision the design narrative forgot", "- **A-1** (Assumption): assumed for the design", "- **P-1** (Policy): a rule on the design"} {
		if !strings.Contains(design, want) {
			t.Fatalf("design render lacks %q:\n%s", want, design)
		}
	}
	if strings.Contains(design, "- **D-1**") {
		t.Fatal("an embedded decision is covered, not appended")
	}
	if !strings.Contains(PRD(p, "prd/x", nil), "Narrative: `prd` document \"Stacked PRs\" (version 3) — render it with `memory render prd/x --slot prd`") {
		t.Fatalf("PRD view points at the document:\n%s", PRD(p, "prd/x", nil))
	}
	// Markers in hidden comments and code are examples, not coverage. They stay
	// byte-for-byte in the narrative and the claim remains in the uncovered list.
	tr.doc("prd/inert", "prd", "Inert markers", "# Examples\n\n<!-- {{FR-1}} -->\n\n`{{FR-1}}`\n\n```text\n{{FR-1}}\n```\n", 0)
	inertClaim := tr.add(record.Promise, "prd/inert", "FR-1", "must remain visible")
	p = tr.projection()
	inert := mustRender(t, p, "prd/inert", "prd", "x")
	for _, want := range []string{"<!-- {{FR-1}} -->", "`{{FR-1}}`", "```text\n{{FR-1}}\n```", "- **FR-1** (Promise): must remain visible"} {
		if !strings.Contains(inert, want) {
			t.Fatalf("inert marker render lacks %q:\n%s", want, inert)
		}
	}
	// Needs can be served too. Their block embeds and the ordinary PRD view
	// name the active decisions that serve them without calling them undesigned.
	need := tr.add(record.Need, "prd/need", "G-1", "make reviews cheap")
	tr.add(record.Decision, "design/need", "D-1", "use small slices", rel(record.Serves, need.ID))
	tr.doc("prd/need", "prd", "Review goal", "# Review goal\n\n{{G-1}}\n", 0)
	p = tr.projection()
	needRender := mustRender(t, p, "prd/need", "prd", "x")
	if !strings.Contains(needRender, "> **G-1** · make reviews cheap\n>\n> _since 2026-09-03 · served by design/need#D-1_") || strings.Contains(needRender, "undesigned") {
		t.Fatalf("embedded Need omits its serving decision:\n%s", needRender)
	}
	if needView := PRD(p, "prd/need", nil); !strings.Contains(needView, "served by design/need#D-1") {
		t.Fatalf("PRD Need omits its serving decision:\n%s", needView)
	}
	_ = inertClaim
	if atoi("x") != 0 || atoi("12") != 12 {
		t.Fatal("atoi")
	}
	_ = fr2
}

func TestRenderRefusesAStaleFallbackWhenANewerDocumentIsInvalid(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	v1 := tr.doc("prd/x", "prd", "v1", "# Version one\n", 0)
	v2 := tr.doc("prd/x", "prd", "v2", "# Version two\n", 0, rel(record.Supersedes, v1.ID))
	tr.files[v2.BodyPath()] = []byte("# tampered\n")
	tr.doc("prd/x", "design", "design", "# Design\n", 0)
	tr.doc("prd/y", "prd", "other", "# Other\n", 0)
	p := tr.projection()

	got, err := Render(p, "prd/x", "prd", "the working tree")
	if err == nil || got != "" || !strings.Contains(err.Error(), "cannot render prd/x#prd") || !strings.Contains(err.Error(), v2.BodyPath()) || !strings.Contains(err.Error(), "does not match body_sha256") {
		t.Fatalf("broken current document must fail closed, got %q, err %v", got, err)
	}
	if strings.Contains(got, "Version one") {
		t.Fatal("an invalid v2 must never reactivate v1 in the rendered output")
	}
	if got := mustRender(t, p, "prd/x", "design", "the working tree"); !strings.Contains(got, "# Design") {
		t.Fatalf("another slot on the same subject must remain readable:\n%s", got)
	}
	if got := mustRender(t, p, "prd/y", "prd", "the working tree"); !strings.Contains(got, "# Other") {
		t.Fatalf("another subject must remain readable:\n%s", got)
	}

	// A subject overview advertises every active Document slot, so its
	// wildcard validation must catch a stale fallback outside the namespace's
	// conventional slot as well.
	other := &tree{t: t, files: store.MapSource{}}
	other.doc("prd/x", "prd", "healthy prd", "# Healthy PRD\n", 0)
	designV1 := other.doc("prd/x", "design", "design v1", "# Design one\n", 0)
	designV2 := other.doc("prd/x", "design", "design v2", "# Design two\n", 0, rel(record.Supersedes, designV1.ID))
	other.files[designV2.BodyPath()] = []byte("# tampered design\n")
	otherProjection := other.projection()
	if err := ValidateDocumentFiles(otherProjection, "prd/x", "prd"); err != nil {
		t.Fatalf("the healthy PRD slot must remain renderable: %v", err)
	}
	if err := ValidateDocumentFiles(otherProjection, "prd/x", ""); err == nil || !strings.Contains(err.Error(), designV2.BodyPath()) {
		t.Fatalf("the subject overview must reject a stale secondary slot: %v", err)
	}
}
