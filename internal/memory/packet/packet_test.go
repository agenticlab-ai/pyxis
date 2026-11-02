package packet

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
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

func newTree(t *testing.T) *tree { return &tree{t: t, files: store.MapSource{}} }

func (tr *tree) clone() *tree {
	c := &tree{t: tr.t, files: store.MapSource{}, n: tr.n + 100}
	maps.Copy(c.files, tr.files)
	return c
}

func (tr *tree) add(kind, subject, family, statement string, rels ...record.Relation) *record.Record {
	tr.n++
	var e [10]byte
	e[9] = tr.n
	r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, t0.Add(time.Duration(tr.n)*time.Second), e), Kind: kind, Subject: subject,
		Family: family, Statement: statement, Intent: "intent of " + family, Rationale: "because", Issuer: "m@example.com",
		CreatedAt: t0.Add(time.Duration(tr.n) * time.Second).Format(time.RFC3339), Relations: rels}
	if kind == record.Assumption {
		r.Expires = t0.Add(time.Hour).Format(time.RFC3339)
	}
	b, err := record.Marshal(r)
	if err != nil {
		tr.t.Fatal(err)
	}
	tr.files[r.Path()] = b
	return r
}

func (tr *tree) set() *store.Set {
	s, err := store.Load(context.Background(), tr.files)
	if err != nil {
		tr.t.Fatal(err)
	}
	return s
}

func rel(t, target string) record.Relation { return record.Relation{Type: t, Target: target} }

func codes(fs []Finding) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return strings.Join(out, ",")
}

func TestCleanChangeWithCarriedDesign(t *testing.T) {
	base := newTree(t)
	fr1 := base.add(record.Promise, "prd/x", "FR-1", "the executor shall build the frontier")
	d1 := base.add(record.Decision, "design/x", "D-1", "a dispatcher over a worker", rel(record.Serves, fr1.ID))
	head := base.clone()
	learn := head.add(record.Learning, "prd/x", "", "planning found FR-1 unverifiable")
	fr1b := head.add(record.Promise, "prd/x", "FR-1", "the executor shall build the ready frontier", rel(record.Supersedes, fr1.ID))
	d1b := head.add(record.Decision, "design/x", "D-1", "a dispatcher over a worker, frontier verified on the obligation", rel(record.Supersedes, d1.ID), rel(record.Serves, fr1b.ID))
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "feature/a", BaseRef: "main", Description: "late discovery"})
	if !p.OK() || len(p.Warnings) != 0 {
		t.Fatalf("expected a clean packet: %s", p.Summary())
	}
	if len(p.Added) != 3 || p.Added[0].ID != d1b.ID || p.Added[1].ID != learn.ID || p.Added[2].ID != fr1b.ID {
		t.Fatalf("added in id order: %v", p.Added)
	}
	if len(p.Superseded) != 2 || len(p.Reopens) != 0 || len(p.Undesigned) != 0 || p.Consequence != record.Medium {
		t.Fatalf("packet: %+v", p)
	}
	body := p.Render()
	for _, want := range []string{"# late discovery", "consequence **medium**", "### Promise FR-1 on prd/x — replaces the current promise", "- **before** (since 2026-09-03): the executor shall build the frontier",
		"- **after**: the executor shall build the ready frontier", "### Decision D-1 on design/x — replaces the current decision", "- **serves**: prd/x#FR-1", "## Also carried", "**Learning** on prd/x",
		"- nothing: every decision serving a moved claim rides this change", "- nothing: every new promise is served by a decision", "- no other branch touches these families"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body lacks %q:\n%s", want, body)
		}
	}
	if !strings.Contains(p.Summary(), "check: ok") {
		t.Fatalf("summary: %s", p.Summary())
	}
}

func TestReopensUndesignedAndConcurrent(t *testing.T) {
	base := newTree(t)
	fr1 := base.add(record.Promise, "prd/x", "FR-1", "one")
	base.add(record.Decision, "design/x", "D-1", "serves one", rel(record.Serves, fr1.ID))
	head := base.clone()
	head.add(record.Promise, "prd/x", "FR-1", "one, corrected", rel(record.Supersedes, fr1.ID))
	fr2 := head.add(record.Promise, "prd/x", "FR-2", "two, brand new")
	head.add(record.Expectation, "prd/x", "DOD-1", "done when two")
	other := newTree(t)
	other.n = 50
	o1 := other.add(record.Promise, "prd/x", "FR-2", "two, from another branch")
	o2 := other.add(record.Promise, "prd/x", "FR-1", "one, also corrected elsewhere", rel(record.Supersedes, fr1.ID))
	o3 := other.add(record.Promise, "prd/x", "FR-9", "one, renumbered elsewhere", rel(record.Supersedes, fr1.ID))
	p := Compute(Input{Base: base.set(), Head: head.set(), InFlight: map[string]*store.Set{"feature/b": other.set()}, Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "feature/a", BaseRef: "main"})
	if !p.OK() {
		t.Fatalf("no blockers expected: %s", p.Summary())
	}
	if got := codes(p.Warnings); got != "reopens,undesigned,undesigned,concurrent,concurrent,concurrent" {
		t.Fatalf("warnings %s", got)
	}
	if len(p.Reopens) != 1 || p.Reopens[0].Serves != fr1.ID || len(p.Undesigned) != 2 || p.Undesigned[1].ID != fr2.ID {
		t.Fatalf("reopens %+v undesigned %v", p.Reopens, p.Undesigned)
	}
	if len(p.Concurrent) != 3 || p.Concurrent[0].Theirs.ID != o1.ID || p.Concurrent[1].Theirs.ID != o2.ID || !strings.Contains(p.Concurrent[1].What, "also changes prd/x#FR-1") ||
		p.Concurrent[2].Theirs.ID != o3.ID || !strings.Contains(p.Concurrent[2].What, "also supersedes") {
		t.Fatalf("concurrent %+v", p.Concurrent)
	}
	body := p.Render()
	for _, want := range []string{"## What it reopens", "design/x#D-1 serves " + fr1.ID, "- prd/x#FR-2", "- prd/x#DOD-1", "feature/b also changes prd/x#FR-2"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body lacks %q:\n%s", want, body)
		}
	}
	if !strings.Contains(p.Summary(), "! reopens") {
		t.Fatalf("summary: %s", p.Summary())
	}
}

func TestBlockers(t *testing.T) {
	base := newTree(t)
	fr1 := base.add(record.Promise, "prd/x", "FR-1", "one")
	fr1b := base.add(record.Promise, "prd/x", "FR-1", "one moved on main", rel(record.Supersedes, fr1.ID))
	fr3 := base.add(record.Promise, "prd/x", "FR-3", "three")
	base.add(record.Promise, "prd/x", "FR-4", "four")
	base.add(record.Promise, "prd/x", "FR-4", "four again (a conflict already on the base)")
	head := base.clone()
	stale := head.add(record.Promise, "prd/x", "FR-1", "one from a stale branch", rel(record.Supersedes, fr1.ID))
	dup := head.add(record.Promise, "prd/x", "FR-3", "three again without supersedes")
	dangling := head.add(record.Decision, "design/x", "D-9", "serves a ghost", rel(record.Serves, "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV"))
	edited := *fr3
	edited.Statement = "three, edited in place"
	b, _ := record.Marshal(&edited)
	head.files[fr3.Path()] = b
	delete(head.files, fr1b.Path())
	head.files["prd/x/junk.json"] = []byte("{")
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "feature/c", BaseRef: "main"})
	if p.OK() {
		t.Fatal("expected blockers")
	}
	got := codes(p.Blockers)
	for _, want := range []string{"record-modified", "record-deleted", "record-invalid", "dangling-target", "effective-set-moved", "conflict-at-admission"} {
		if !strings.Contains(got, want) {
			t.Fatalf("blockers %s lack %s", got, want)
		}
	}
	if codes(p.Warnings) != "conflict-on-base,undesigned" {
		t.Fatalf("warnings %s", codes(p.Warnings))
	}
	if len(p.Moved) != 1 || p.Moved[0].By.ID != stale.ID || p.Moved[0].CurrentHead == nil {
		t.Fatalf("moved %+v", p.Moved)
	}
	// The illegal deletion is reflected in the candidate tree (and separately
	// blocked as record-deleted), so only the genuine FR-3 conflict remains.
	if len(p.Conflicts) != 1 || p.Conflicts[0].Family != "FR-3" || !strings.Contains(strings.Join(p.Conflicts[0].IDs, ","), dup.ID) {
		t.Fatalf("conflicts %+v", p.Conflicts)
	}
	if len(p.Modified) != 1 || len(p.Deleted) != 1 {
		t.Fatalf("modified %v deleted %v", p.Modified, p.Deleted)
	}
	body := p.Render()
	if !strings.Contains(body, "## Blockers") || !strings.Contains(body, dangling.ID) || !strings.Contains(body, "## Already wrong on the base") {
		t.Fatalf("body:\n%s", body)
	}
	if !strings.Contains(p.Summary(), "check: blocked") || !strings.Contains(p.Summary(), "✗ record-modified") {
		t.Fatalf("summary: %s", p.Summary())
	}
}

func TestTwoHopSupersessionIsTracedToTheBase(t *testing.T) {
	ancestor := newTree(t)
	a := ancestor.add(record.Promise, "prd/x", "FR-1", "A")
	ancestor.add(record.Decision, "design/x", "D-1", "serves A", rel(record.Serves, a.ID))
	head := ancestor.clone()
	b := head.add(record.Promise, "prd/x", "FR-1", "B", rel(record.Supersedes, a.ID))
	c := head.add(record.Promise, "prd/x", "FR-1", "C", rel(record.Supersedes, b.ID))
	// against an unmoved base the chain is clean, and the packet says what it replaces and reopens
	p := Compute(Input{Base: ancestor.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() || len(p.Superseded) != 1 || p.Superseded[0].Target.ID != a.ID || p.Superseded[0].By.ID != c.ID || len(p.Reopens) != 1 || p.Reopens[0].Serves != a.ID {
		t.Fatalf("chain to the base: %s\n%+v", p.Summary(), p.Superseded)
	}
	// main withdrew A while the branch was in flight: the chain's base end has moved, and C may not resurrect the family
	moved := ancestor.clone()
	moved.n = 200
	moved.add(record.Revocation, "prd/x", "", "A withdrawn on main", rel(record.Revokes, a.ID))
	p = Compute(Input{Base: moved.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if p.OK() || codes(p.Blockers) != "effective-set-moved" || p.Moved[0].By.ID != c.ID || p.Moved[0].Target != a.ID || p.Moved[0].Status != "revoked" {
		t.Fatalf("a two-hop chain must be judged at its base end: %s", p.Summary())
	}
	// the same with a moved head instead of a revocation
	moved2 := ancestor.clone()
	moved2.n = 220
	a2 := moved2.add(record.Promise, "prd/x", "FR-1", "A, corrected on main", rel(record.Supersedes, a.ID))
	p = Compute(Input{Base: moved2.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !strings.Contains(codes(p.Blockers), "effective-set-moved") || p.Moved[0].CurrentHead == nil || p.Moved[0].CurrentHead.ID != a2.ID {
		t.Fatalf("moved head behind a chain: %s", p.Summary())
	}
}

func TestMovedToNoHeadAndConsequence(t *testing.T) {
	base := newTree(t)
	fr1 := base.add(record.Promise, "prd/x", "FR-1", "one")
	base.add(record.Revocation, "prd/x", "", "one withdrawn", rel(record.Revokes, fr1.ID))
	head := base.clone()
	head.add(record.Promise, "prd/x", "FR-1", "one, corrected on a stale branch", rel(record.Supersedes, fr1.ID))
	head.add(record.Policy, "governance/principles", "AP-1", "a new principle")
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if len(p.Moved) != 1 || p.Moved[0].CurrentHead != nil || p.Moved[0].Status != "revoked" {
		t.Fatalf("moved %+v", p.Moved)
	}
	if !strings.Contains(p.Blockers[0].What, "no active head") {
		t.Fatalf("blocker: %+v", p.Blockers)
	}
	if p.Consequence != record.High {
		t.Fatalf("a governance policy is high: %s", p.Consequence)
	}
	many := base.clone()
	for i := range 10 {
		many.add(record.Need, "prd/y", "G-"+string(rune('a'+i)), "goal")
	}
	pol := policy.Default()
	p = Compute(Input{Base: base.set(), Head: many.set(), Policy: pol, Branch: "b", BaseRef: "main"})
	if p.Consequence != record.Medium {
		t.Fatalf("ten low claims raise one tier: %s", p.Consequence)
	}
	pol.Consequence.ManyClaims = 0
	p = Compute(Input{Base: base.set(), Head: many.set(), Policy: pol, Branch: "b", BaseRef: "main"})
	if p.Consequence != record.Low {
		t.Fatalf("many_claims 0 never raises: %s", p.Consequence)
	}
	if !strings.Contains(p.Render(), "# Memory change on b") || !strings.Contains(p.Render(), "- no decisions or plans change") {
		t.Fatalf("default title: %s", p.Render())
	}
}

func TestRenderCoversEveryField(t *testing.T) {
	base := newTree(t)
	anchor := base.add(record.Need, "code/executor", "anchor", "package executor exists")
	head := base.clone()
	fr := head.add(record.Promise, "prd/x", "FR-1", "one")
	fr.FitCriterion, fr.LearnedFrom, fr.Alternatives, fr.Tags = "passes when", []string{"incident:I2"}, []string{"do nothing"}, []string{"review"}
	b, _ := record.Marshal(fr)
	head.files[fr.Path()] = b
	head.add(record.Decision, "design/x", "D-1", "decision", rel(record.Serves, fr.ID), rel(record.About, anchor.ID), rel(record.Serves, "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV"))
	q := head.add(record.Question, "prd/x", "", "which?")
	head.add(record.Answer, "prd/x", "", "this", rel(record.Answers, q.ID))
	head.add(record.Revocation, "prd/x", "", "gone", rel(record.Revokes, anchor.ID))
	head.add(record.Learning, "prd/x", "", "lesson")
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	body := p.Render()
	for _, want := range []string{"- **fit criterion**: passes when", "- **learned from**: incident:I2", "- **alternatives rejected**: do nothing", "- **tags**: review",
		"- **about**: code/executor", "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV", "· answers " + q.ID, "· revokes " + anchor.ID, "**Learning** on prd/x: lesson"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body lacks %q:\n%s", want, body)
		}
	}
	if clip("a  b   c", 100) != "a b c" || clip("abcdefgh", 5) != "abcd…" {
		t.Fatal("clip")
	}
}

func TestBranchBehindBaseIsNotDeletion(t *testing.T) {
	ancestor := newTree(t)
	fr1 := ancestor.add(record.Promise, "prd/x", "FR-1", "one")
	base := ancestor.clone()
	base.n = 20
	base.add(record.Promise, "prd/x", "FR-2", "two, landed on main after the branch was cut")
	head := ancestor.clone()
	head.n = 40
	head.add(record.Promise, "prd/x", "FR-1", "one, corrected", rel(record.Supersedes, fr1.ID))
	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() || len(p.Deleted) != 0 || len(p.Added) != 1 {
		t.Fatalf("a branch behind main must pass: %s", p.Summary())
	}
	if p.Union.Head("prd/x", "FR-2") == nil {
		t.Fatal("the union carries what landed on main")
	}
	// without an ancestor the base tip is the reference, and the missing FR-2 reads as deleted
	p = Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if p.OK() || len(p.Deleted) != 1 {
		t.Fatalf("no ancestor: %s", p.Summary())
	}
}

func TestDocumentCheckoutLineEndingsAreNotAModification(t *testing.T) {
	ancestor := newTree(t)
	doc := ancestor.doc("prd/x", "prd", "The PRD", "# The PRD\n\nNarrative.\n")
	ancestor.files[store.PolicyFile] = []byte("{\n  \"default\": \"low\"\n}\n")
	head := ancestor.clone()
	head.files[doc.Path()] = []byte(strings.ReplaceAll(string(head.files[doc.Path()]), "\n", "\r\n"))
	head.files[doc.BodyPath()] = []byte(strings.ReplaceAll(string(head.files[doc.BodyPath()]), "\n", "\r\n"))
	head.files[store.PolicyFile] = []byte(strings.ReplaceAll(string(head.files[store.PolicyFile]), "\n", "\r\n"))

	p := Compute(Input{Base: ancestor.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if len(p.Modified) != 0 || len(p.Blockers) != 0 || p.PolicyChanged {
		t.Fatalf("Git checkout line endings cannot modify immutable memory: modified %v policy changed %v blockers %+v", p.Modified, p.PolicyChanged, p.Blockers)
	}
}

func TestRecordAlreadyLandedOnBaseIsNotAddedAgain(t *testing.T) {
	ancestor := newTree(t)
	head := ancestor.clone()
	landed := head.add(record.Learning, "prd/x", "", "already landed")
	base := ancestor.clone()
	base.files[landed.Path()] = head.files[landed.Path()]

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if len(p.Added) != 0 || len(p.Blockers) != 0 || len(p.Warnings) != 0 {
		t.Fatalf("an identical record already on the base is a merge no-op: added %+v blockers %+v warnings %+v", p.Added, p.Blockers, p.Warnings)
	}
}

func TestIndependentlyAddedRecordWithDifferentBytesBlocks(t *testing.T) {
	ancestor := newTree(t)
	head := ancestor.clone()
	branchRecord := head.add(record.Learning, "prd/x", "", "branch version")
	base := ancestor.clone()
	baseRecord := *branchRecord
	baseRecord.Statement = "different base version"
	b, err := record.Marshal(&baseRecord)
	if err != nil {
		t.Fatal(err)
	}
	base.files[baseRecord.Path()] = b

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if len(p.Added) != 1 || p.Added[0].ID != branchRecord.ID || !strings.Contains(codes(p.Blockers), "record-add-collision") {
		t.Fatalf("a divergent add/add must block: added %+v blockers %+v", p.Added, p.Blockers)
	}
}

func TestIndependentlyAddedPathBlocksEvenWhenBaseRecordIsMalformed(t *testing.T) {
	ancestor := newTree(t)
	head := ancestor.clone()
	branchRecord := head.add(record.Learning, "prd/x", "", "valid branch record")
	base := ancestor.clone()
	base.files[branchRecord.Path()] = []byte("{")

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if len(p.Added) != 1 || p.Added[0].ID != branchRecord.ID || !strings.Contains(codes(p.Blockers), "record-add-collision") {
		t.Fatalf("a divergent add/add path must block even when the base copy cannot parse: added %+v blockers %+v", p.Added, p.Blockers)
	}
}

func TestThreeWayUnionDoesNotResurrectARecordDeletedOnBase(t *testing.T) {
	ancestor := newTree(t)
	fr1 := ancestor.add(record.Promise, "prd/x", "FR-1", "one")
	doc := ancestor.doc("prd/x", "prd", "The PRD", "# The PRD\n\n{{FR-1}}\n")

	// Main deleted FR-1 after the branch was cut. The stale feature tree still
	// contains it, but the only change made by the feature is the Need.
	base := ancestor.clone()
	delete(base.files, fr1.Path())
	head := ancestor.clone()
	need := head.add(record.Need, "prd/x", "G-1", "a goal")

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if got := codes(p.Blockers); got != "embed-unresolved" {
		t.Fatalf("the real merged tree has a broken embed and must block, got %q:\n%s", got, p.Summary())
	}
	if p.Union.Head("prd/x", "FR-1") != nil {
		t.Fatal("the union resurrected FR-1 from the stale feature tree")
	}
	if len(p.Added) != 1 || p.Added[0].ID != need.ID {
		t.Fatalf("only the branch's Need is added, got %v", p.Added)
	}
	if strings.Contains(p.Summary(), fr1.ID) || !strings.Contains(p.Blockers[0].What, doc.ID) {
		t.Fatalf("the packet must name the broken document, not list stale FR-1 as an addition:\n%s", p.Summary())
	}
}

func TestRevokedWithinTheBranchIsVoid(t *testing.T) {
	base := newTree(t)
	fr1 := base.add(record.Promise, "prd/x", "FR-1", "one")
	fr1b := base.add(record.Promise, "prd/x", "FR-1", "one moved on main", rel(record.Supersedes, fr1.ID))
	head := base.clone()
	stale := head.add(record.Promise, "prd/x", "FR-1", "one from a stale branch", rel(record.Supersedes, fr1.ID))
	head.add(record.Revocation, "prd/x", "", "written against a head that moved", rel(record.Revokes, stale.ID))
	head.add(record.Promise, "prd/x", "FR-1", "one, rebased on the current head", rel(record.Supersedes, fr1b.ID))
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() || len(p.Moved) != 0 || len(p.Superseded) != 1 {
		t.Fatalf("a revoked stale claim must not block: %s", p.Summary())
	}
}

func TestPolicyChangesAreVisibleAndJudgedUnderTheBase(t *testing.T) {
	base := newTree(t)
	basePolicy, _ := policy.Default().Marshal()
	base.files[store.PolicyFile] = basePolicy
	base.add(record.Promise, "prd/x", "FR-1", "one")
	head := base.clone()
	head.add(record.Policy, "governance/principles", "AP-9", "a new principle")
	lax := policy.Default()
	lax.Consequence.HighSubjectPrefixes = nil
	lax.Consequence.ByKind[record.Policy] = record.Low
	laxBytes, _ := lax.Marshal()
	head.files[store.PolicyFile] = laxBytes
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: lax, BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.PolicyChanged || p.Consequence != record.High || codes(p.Warnings) != "policy-changed" {
		t.Fatalf("a lowered policy must not lower the verdict: %+v %s", p.Warnings, p.Consequence)
	}
	if !strings.Contains(p.Render(), "## Policy change — operator sign-off") {
		t.Fatalf("body:\n%s", p.Render())
	}
	// the stricter of the two policies judges even without a policy edit
	head2 := base.clone()
	head2.add(record.Policy, "governance/principles", "AP-9", "a new principle")
	p = Compute(Input{Base: base.set(), Head: head2.set(), Policy: lax, BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if p.PolicyChanged || p.Consequence != record.High {
		t.Fatalf("the base policy still judges: %+v", p)
	}
	// deleting the policy is a change too
	head3 := base.clone()
	delete(head3.files, store.PolicyFile)
	head3.add(record.Need, "prd/x", "G-1", "goal")
	p = Compute(Input{Base: base.set(), Head: head3.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.PolicyChanged || !strings.Contains(p.Warnings[0].What, "deleted") || p.Consequence != record.High {
		t.Fatalf("deleted policy: %+v", p.Warnings)
	}
}

func TestPolicyIsJudgedThreeWay(t *testing.T) {
	ancestor := newTree(t)
	def, _ := policy.Default().Marshal()
	ancestor.files[store.PolicyFile] = def
	ancestor.add(record.Promise, "prd/x", "FR-1", "one")
	// main tightened the policy after the branch was cut; the branch never touched it
	base := ancestor.clone()
	base.n = 50
	tight := policy.Default()
	tight.Consequence.ManyClaims = 1
	tightBytes, _ := tight.Marshal()
	base.files[store.PolicyFile] = tightBytes
	head := ancestor.clone()
	head.n = 100
	head.add(record.Need, "prd/x", "G-1", "goal")
	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: tight, Branch: "b", BaseRef: "main"})
	if p.PolicyChanged || codes(p.Warnings) != "" {
		t.Fatalf("a mainline-only policy change is not the branch's: %+v", p.Warnings)
	}
	if string(p.Union.Set.Policy) != string(tightBytes) {
		t.Fatal("the merge keeps the base's policy when the branch did not change it")
	}
	if p.Consequence != record.Medium { // one Need under many_claims=1 rises from low to medium under the base's policy
		t.Fatalf("judged under the base's policy: %s", p.Consequence)
	}
	// both changed it: the branch's change is flagged and the clash named
	both := ancestor.clone()
	both.n = 150
	lax := policy.Default()
	lax.Consequence.ManyClaims = 0
	laxBytes, _ := lax.Marshal()
	both.files[store.PolicyFile] = laxBytes
	both.add(record.Need, "prd/x", "G-2", "goal")
	p = Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: both.set(), Policy: lax, BasePolicy: tight, Branch: "b", BaseRef: "main"})
	if !p.PolicyChanged || codes(p.Warnings) != "policy-changed,policy-conflict" || p.Consequence != record.High || string(p.Union.Set.Policy) != string(laxBytes) {
		t.Fatalf("both changed: %+v %s", p.Warnings, p.Consequence)
	}
}

func TestSecondAnswerBlocks(t *testing.T) {
	base := newTree(t)
	q := base.add(record.Question, "prd/x", "", "which?")
	a := base.add(record.Answer, "prd/x", "", "blue", rel(record.Answers, q.ID))
	head := base.clone()
	head.add(record.Answer, "prd/x", "", "red", rel(record.Answers, q.ID))
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "answer-conflict" {
		t.Fatalf("blockers %+v", p.Blockers)
	}
	fixed := base.clone()
	fixed.add(record.Answer, "prd/x", "", "red, corrected", rel(record.Answers, q.ID), rel(record.Supersedes, a.ID))
	p = Compute(Input{Base: base.set(), Head: fixed.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() {
		t.Fatalf("a replacing answer passes: %+v", p.Blockers)
	}
	twice := newTree(t)
	q2 := twice.add(record.Question, "prd/x", "", "which?")
	twice.add(record.Answer, "prd/x", "", "blue", rel(record.Answers, q2.ID))
	twice.add(record.Answer, "prd/x", "", "red", rel(record.Answers, q2.ID))
	head2 := twice.clone()
	head2.add(record.Need, "prd/x", "G-1", "goal")
	p = Compute(Input{Base: twice.set(), Head: head2.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() || codes(p.Warnings) != "answer-conflict-on-base" {
		t.Fatalf("conflict already on the base: %+v %+v", p.Blockers, p.Warnings)
	}
}

func TestConcurrentDuplicateRevocationBlocksRegardlessOfIDOrder(t *testing.T) {
	for _, tc := range []struct {
		name          string
		candidateTick byte
		baseTick      byte
	}{
		{name: "candidate id sorts before base", candidateTick: 10, baseTick: 200},
		{name: "candidate id sorts after base", candidateTick: 220, baseTick: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			target := ancestor.add(record.Need, "prd/x", "G-1", "withdraw once")
			head := ancestor.clone()
			head.n = tc.candidateTick
			candidate := head.add(record.Revocation, "prd/x", "", "candidate withdrawal", rel(record.Revokes, target.ID))
			base := ancestor.clone()
			base.n = tc.baseTick
			onBase := base.add(record.Revocation, "prd/x", "", "base withdrawal", rel(record.Revokes, target.ID))

			p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
			if codes(p.Blockers) != "revocation-conflict" || !strings.Contains(p.Blockers[0].What, candidate.ID) || !strings.Contains(p.Blockers[0].What, onBase.ID) {
				t.Fatalf("duplicate revocation escaped attribution: %s", p.Summary())
			}
		})
	}
}

func TestConcurrentQuestionRevocationAndAnswerBlockRegardlessOfIDOrder(t *testing.T) {
	for _, tc := range []struct {
		name          string
		candidateTick byte
		baseTick      byte
		want          string
	}{
		{name: "revocation id sorts before answer", candidateTick: 100, baseTick: 200, want: "answers-inactive-question"},
		{name: "revocation id sorts after answer", candidateTick: 220, baseTick: 20, want: "revocation-breaks-projection"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			question := ancestor.add(record.Question, "prd/x", "", "which?")
			head := ancestor.clone()
			head.n = tc.candidateTick
			revocation := head.add(record.Revocation, "prd/x", "", "withdraw question", rel(record.Revokes, question.ID))
			base := ancestor.clone()
			base.n = tc.baseTick
			answer := base.add(record.Answer, "prd/x", "", "answer on base", rel(record.Answers, question.ID))

			p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
			if codes(p.Blockers) != tc.want || !strings.Contains(p.Blockers[0].What, answer.ID) || !strings.Contains(p.Blockers[0].What, revocation.ID) {
				t.Fatalf("concurrent answer/revocation escaped attribution: %s", p.Summary())
			}
		})
	}
}

func TestConcurrentUndoAndLaterBaseClaimBlockRegardlessOfIDOrder(t *testing.T) {
	for _, tc := range []struct {
		name          string
		candidateTick byte
		baseTick      byte
		want          string
	}{
		{name: "undo id sorts before base claim", candidateTick: 100, baseTick: 200, want: "conflict-at-admission"},
		{name: "undo id sorts after base claim", candidateTick: 220, baseTick: 20, want: "revocation-breaks-projection"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			old := ancestor.add(record.Promise, "prd/x", "FR-1", "old")
			withdrawal := ancestor.add(record.Revocation, "prd/x", "", "withdraw old", rel(record.Revokes, old.ID))
			head := ancestor.clone()
			head.n = tc.candidateTick
			head.add(record.Revocation, "prd/x", "", "undo withdrawal", rel(record.Revokes, withdrawal.ID))
			base := ancestor.clone()
			base.n = tc.baseTick
			base.add(record.Promise, "prd/x", "FR-1", "new base claim")

			p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
			if codes(p.Blockers) != tc.want {
				t.Fatalf("concurrent undo/base claim escaped attribution: %s", p.Summary())
			}
		})
	}
}

func TestInactiveCandidateSupersederCannotHideClockOrderViolation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		baseTick byte
		want     string
	}{
		{name: "base withdrawal sorts before candidate", baseTick: 20, want: "supersedes-revoked-record"},
		{name: "base withdrawal sorts after candidate", baseTick: 200, want: "revokes-inactive-record"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			target := ancestor.add(record.Promise, "prd/x", "FR-1", "target")
			head := ancestor.clone()
			head.n = 100
			candidate := head.add(record.Promise, "prd/x", "FR-1", "candidate", rel(record.Supersedes, target.ID))

			base := ancestor.clone()
			base.n = tc.baseTick
			base.add(record.Revocation, "prd/x", "", "withdraw target", rel(record.Revokes, target.ID))
			base.n = 220
			base.add(record.Promise, "prd/x", "FR-1", "later base successor", rel(record.Supersedes, candidate.ID))

			p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
			if codes(p.Blockers) != tc.want+",historical-act-reinterpreted" || p.Union.Status(candidate.ID) != resolve.Superseded || len(p.Moved) != 0 {
				t.Fatalf("inactive candidate suppressed its clock-order violation: %s", p.Summary())
			}
		})
	}
}

func TestConcurrentDocumentReactivationAndBaseWithdrawalBlockRegardlessOfIDOrder(t *testing.T) {
	for _, tc := range []struct {
		name          string
		candidateTick byte
		baseTick      byte
	}{
		{name: "reactivation id sorts before base withdrawal", candidateTick: 100, baseTick: 200},
		{name: "reactivation id sorts after base withdrawal", candidateTick: 220, baseTick: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			claim := ancestor.add(record.Promise, "prd/x", "FR-1", "claim")
			old := ancestor.doc("prd/x", "prd", "old", "# Old\n\n{{FR-1}}\n")
			current := ancestor.doc("prd/x", "prd", "current", "# Current\n", rel(record.Supersedes, old.ID))
			head := ancestor.clone()
			head.n = tc.candidateTick
			head.add(record.Revocation, "prd/x", "", "reactivate old document", rel(record.Revokes, current.ID))
			base := ancestor.clone()
			base.n = tc.baseTick
			base.add(record.Revocation, "prd/x", "", "withdraw claim", rel(record.Revokes, claim.ID))

			p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
			if codes(p.Blockers) != "revocation-breaks-projection" {
				t.Fatalf("concurrent document reactivation/base withdrawal escaped attribution: %s", p.Summary())
			}
		})
	}
}

func TestUnrelatedBaseViolationCannotHideMovedSupersession(t *testing.T) {
	ancestor := newTree(t)
	target := ancestor.add(record.Promise, "prd/x", "FR-1", "withdrawn on base")

	head := ancestor.clone()
	head.n = 100
	candidate := head.add(record.Promise, "prd/x", "FR-1", "stale candidate", rel(record.Supersedes, target.ID))

	base := ancestor.clone()
	base.n = 10
	base.add(record.Promise, "prd/y", "FR-1", "unrelated invalid carrier", rel(record.Supersedes, target.ID))
	base.add(record.Revocation, "prd/x", "", "withdraw target", rel(record.Revokes, target.ID))

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "effective-set-moved" || len(p.Moved) != 1 || p.Moved[0].By.ID != candidate.ID || p.Moved[0].Target != target.ID {
		t.Fatalf("an unrelated base violation hid the stale candidate: %s", p.Summary())
	}
	if !strings.Contains(codes(p.Warnings), "supersedes-across-subjects-on-base") {
		t.Fatalf("the unrelated base violation was not retained as a warning: %s", p.Summary())
	}
}

func TestViolationOnBaseTargetCannotHideMovedSupersession(t *testing.T) {
	ancestor := newTree(t)
	unrelated := ancestor.add(record.Promise, "prd/y", "FR-9", "unrelated")
	target := ancestor.add(record.Promise, "prd/x", "FR-1", "already invalid", rel(record.Supersedes, unrelated.ID))

	head := ancestor.clone()
	head.n = 150
	candidate := head.add(record.Promise, "prd/x", "FR-2", "candidate rename", rel(record.Supersedes, target.ID))

	base := ancestor.clone()
	base.n = 200
	current := base.add(record.Promise, "prd/x", "FR-1", "current base head", rel(record.Supersedes, target.ID))

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "effective-set-moved" || len(p.Moved) != 1 || p.Moved[0].By.ID != candidate.ID || p.Moved[0].CurrentHead == nil || p.Moved[0].CurrentHead.ID != current.ID {
		t.Fatalf("a violation carried by the base target hid the stale candidate: %s", p.Summary())
	}
	if !strings.Contains(codes(p.Warnings), "supersedes-across-subjects-on-base") {
		t.Fatalf("the target's existing violation was not retained as a warning: %s", p.Summary())
	}
}

func TestUnionDanglingFromBaseDeletionAndCandidateEdgeBlocks(t *testing.T) {
	ancestor := newTree(t)
	target := ancestor.add(record.Need, "prd/x", "G-1", "shared target")

	base := ancestor.clone()
	delete(base.files, target.Path())

	head := ancestor.clone()
	candidate := head.add(record.Decision, "design/x", "D-1", "new decision depends on the target", rel(record.DependsOn, target.ID))

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "dangling-target" || !strings.Contains(p.Blockers[0].What, candidate.ID) || len(p.Warnings) != 0 {
		t.Fatalf("a candidate edge to a target deleted on the base escaped the merged view: %s", p.Summary())
	}
}

func TestReactivatingAnInactiveDanglingCarrierBlocks(t *testing.T) {
	base := newTree(t)
	missing := "Need-01ARZ3NDEKTSV4RRFFQ69G5FAV"
	old := base.add(record.Decision, "design/x", "D-1", "old dangling decision", rel(record.DependsOn, missing))
	current := base.add(record.Decision, "design/x", "D-1", "current clean decision", rel(record.Supersedes, old.ID))
	if resolve.New(base.set()).Status(old.ID) != resolve.Superseded || len(base.set().Dangling) != 1 {
		t.Fatal("base fixture must carry one inactive dangling relation")
	}

	head := base.clone()
	reactivate := head.add(record.Revocation, "design/x", "", "reactivate old decision", rel(record.Revokes, current.ID))
	p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if p.OK() {
		t.Fatalf("%s reactivated dangling carrier %s without a blocker:\n%s", reactivate.ID, old.ID, p.Summary())
	}
}

func TestReactivatingAnInactiveViolationCarrierBlocks(t *testing.T) {
	base := newTree(t)
	foreign := base.add(record.Decision, "design/y", "D-1", "foreign")
	old := base.add(record.Decision, "design/x", "D-1", "old invalid decision", rel(record.Supersedes, foreign.ID))
	current := base.add(record.Decision, "design/x", "D-1", "current valid decision", rel(record.Supersedes, old.ID))
	baseProjection := resolve.New(base.set())
	if baseProjection.Status(old.ID) != resolve.Superseded || len(baseProjection.Violations) != 1 || baseProjection.Violations[0].Record != old.ID {
		t.Fatalf("base fixture: old=%s violations=%+v", baseProjection.Status(old.ID), baseProjection.Violations)
	}

	head := base.clone()
	reactivate := head.add(record.Revocation, "design/x", "", "reactivate invalid decision", rel(record.Revokes, current.ID))
	p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if p.OK() {
		t.Fatalf("%s reactivated violation carrier %s without a blocker:\n%s", reactivate.ID, old.ID, p.Summary())
	}
}

func TestOneCandidateSupersedingMultipleMovedTargetsBlocksEachTargetRegardlessOfIDOrder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		baseTick byte
	}{
		{name: "base revocations sort before candidate", baseTick: 10},
		{name: "base revocations sort after candidate", baseTick: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			one := ancestor.add(record.Promise, "prd/x", "FR-1", "one")
			two := ancestor.add(record.Promise, "prd/x", "FR-2", "two")

			head := ancestor.clone()
			head.n = 100
			candidate := head.add(record.Promise, "prd/x", "FR-3", "one consolidated promise", rel(record.Supersedes, one.ID), rel(record.Supersedes, two.ID))

			base := ancestor.clone()
			base.n = tc.baseTick
			base.add(record.Revocation, "prd/x", "", "withdraw one", rel(record.Revokes, one.ID))
			base.add(record.Revocation, "prd/x", "", "withdraw two", rel(record.Revokes, two.ID))

			p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
			if codes(p.Blockers) != "effective-set-moved,effective-set-moved" || len(p.Moved) != 2 {
				t.Fatalf("multi-target stale candidate escaped: %s", p.Summary())
			}
			for _, moved := range p.Moved {
				if moved.By.ID != candidate.ID || moved.Status != "revoked" {
					t.Fatalf("moved attribution = %+v, want candidate %s revoked", moved, candidate.ID)
				}
			}
		})
	}
}

func TestSupersessionForkBlocksOnlyActiveValidCandidateClaims(t *testing.T) {
	t.Run("two active claims fork one base target", func(t *testing.T) {
		base := newTree(t)
		target := base.add(record.Promise, "prd/x", "FR-1", "one base target")
		head := base.clone()
		one := head.add(record.Promise, "prd/x", "FR-2", "first replacement", rel(record.Supersedes, target.ID))
		two := head.add(record.Promise, "prd/x", "FR-3", "second replacement", rel(record.Supersedes, target.ID))

		p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if codes(p.Blockers) != "supersession-fork" || !strings.Contains(p.Blockers[0].What, target.ID) || !strings.Contains(p.Blockers[0].What, one.ID) || !strings.Contains(p.Blockers[0].What, two.ID) {
			t.Fatalf("two active candidate claims replaced the same target without a fork blocker:\n%s", p.Summary())
		}
	})

	t.Run("retired candidate does not fork", func(t *testing.T) {
		base := newTree(t)
		target := base.add(record.Promise, "prd/x", "FR-1", "one base target")
		head := base.clone()
		head.add(record.Promise, "prd/x", "FR-2", "active replacement", rel(record.Supersedes, target.ID))
		retired := head.add(record.Promise, "prd/x", "FR-3", "retired replacement", rel(record.Supersedes, target.ID))
		head.add(record.Revocation, "prd/x", "", "retire the second replacement", rel(record.Revokes, retired.ID))

		p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if !p.OK() || strings.Contains(codes(p.Blockers), "supersession-fork") || p.Union.Status(retired.ID) != resolve.Revoked {
			t.Fatalf("a retired candidate must not count toward a supersession fork:\n%s", p.Summary())
		}
	})

	t.Run("invalid candidate does not fork", func(t *testing.T) {
		base := newTree(t)
		target := base.add(record.Promise, "prd/x", "FR-1", "one base target")
		head := base.clone()
		head.add(record.Promise, "prd/x", "FR-2", "valid replacement", rel(record.Supersedes, target.ID))
		head.add(record.Promise, "prd/y", "FR-3", "cross-subject replacement", rel(record.Supersedes, target.ID))

		p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if !strings.Contains(codes(p.Blockers), "supersedes-across-subjects") || strings.Contains(codes(p.Blockers), "supersession-fork") {
			t.Fatalf("an invalid candidate must not count toward a supersession fork:\n%s", p.Summary())
		}
	})
}

func TestIndependentCandidateRecordsCanPrecedeMergeAncestorHistory(t *testing.T) {
	t.Run("older relationless record remains valid", func(t *testing.T) {
		base := newTree(t)
		base.n = 50
		base.add(record.Learning, "learning/base", "", "known history")
		head := base.clone()
		head.n = 10
		backdated := head.add(record.Learning, "learning/candidate", "", "backdated candidate")

		p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if !p.OK() || p.Union.Status(backdated.ID) != resolve.Active {
			t.Fatalf("independent older record refused:\n%s", p.Summary())
		}
	})

	t.Run("candidate older than concurrent base remains valid", func(t *testing.T) {
		ancestor := newTree(t)
		ancestor.n = 10
		ancestor.add(record.Learning, "learning/ancestor", "", "known ancestor")
		head := ancestor.clone()
		head.n = 20
		head.add(record.Learning, "learning/candidate", "", "candidate issued after ancestor")
		base := ancestor.clone()
		base.n = 30
		base.add(record.Learning, "learning/concurrent", "", "concurrent mainline record")

		p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if !p.OK() || strings.Contains(codes(p.Blockers), "record-history-insertion") {
			t.Fatalf("legitimate cross-branch chronology blocked:\n%s", p.Summary())
		}
	})

	t.Run("empty history accepts first record", func(t *testing.T) {
		base := newTree(t)
		head := base.clone()
		head.n = 1
		head.add(record.Learning, "learning/first", "", "first record")
		p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if !p.OK() {
			t.Fatalf("first record blocked:\n%s", p.Summary())
		}
	})
}

func TestCandidateCannotInsertBeforeKnownHistoryAndRetroactivelyActivateARevocation(t *testing.T) {
	base := newTree(t)
	claim := base.add(record.Promise, "prd/x", "FR-1", "kept")
	old := base.doc("prd/x", "prd", "old", "# Old\n\n{{FR-1}}\n")
	base.n = 10
	revocation := base.add(record.Revocation, "prd/x", "", "withdraw", rel(record.Revokes, claim.ID))
	baseProjection := resolve.New(base.set())
	if baseProjection.Status(claim.ID) != resolve.Active || len(baseProjection.Violations) != 1 || baseProjection.Violations[0].Code != "revocation-breaks-projection" {
		t.Fatalf("base fixture: claim=%s violations=%+v", baseProjection.Status(claim.ID), baseProjection.Violations)
	}

	head := base.clone()
	head.n = 5 // newer than old, but illicitly inserted before known base history
	backdated := head.doc("prd/x", "prd", "new", "# New\n", rel(record.Supersedes, old.ID))
	p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "historical-act-reinterpreted" || p.Union.Status(claim.ID) != resolve.Revoked {
		t.Fatalf("backdated %s silently activated %s and changed %s from active to %s:\n%s", backdated.ID, revocation.ID, claim.ID, p.Union.Status(claim.ID), p.Summary())
	}
}

func TestCandidateCannotSilentlyActivateAConcurrentBaseRevocation(t *testing.T) {
	ancestor := newTree(t)
	claim := ancestor.add(record.Promise, "prd/x", "FR-1", "kept")
	old := ancestor.doc("prd/x", "prd", "old", "# Old\n\n{{FR-1}}\n")

	head := ancestor.clone()
	head.n = 5
	candidate := head.doc("prd/x", "prd", "new", "# New\n", rel(record.Supersedes, old.ID))
	base := ancestor.clone()
	base.n = 10
	revocation := base.add(record.Revocation, "prd/x", "", "withdraw", rel(record.Revokes, claim.ID))
	baseProjection := resolve.New(base.set())
	if baseProjection.Status(claim.ID) != resolve.Active || len(baseProjection.Violations) != 1 || baseProjection.Violations[0].Code != "revocation-breaks-projection" {
		t.Fatalf("base fixture: claim=%s violations=%+v", baseProjection.Status(claim.ID), baseProjection.Violations)
	}

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "historical-act-reinterpreted" {
		t.Fatalf("candidate %s silently activated concurrent %s and changed %s from active to %s:\n%s", candidate.ID, revocation.ID, claim.ID, p.Union.Status(claim.ID), p.Summary())
	}
}

func TestAddedTargetCannotTurnBaseDanglingActIntoAWarning(t *testing.T) {
	ancestor := newTree(t)
	head := ancestor.clone()
	head.n = 1
	question := head.add(record.Question, "prd/x", "", "which?")

	base := ancestor.clone()
	base.n = 10
	answer := base.add(record.Answer, "prd/y", "", "dangling on base", rel(record.Answers, question.ID))
	if len(base.set().Dangling) != 1 {
		t.Fatal("the base fixture must start with one dangling relation")
	}

	p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "answers-across-subjects" || !strings.Contains(p.Blockers[0].What, answer.ID) || len(p.Warnings) != 0 {
		t.Fatalf("an added target exposed a defect but was not attributed to the candidate: %s", p.Summary())
	}
}

func TestAddedTargetCannotSilentlyActivateHistoricalDanglingAct(t *testing.T) {
	for _, tc := range []struct {
		name        string
		carrierKind string
		family      string
		relation    string
	}{
		{name: "revocation", carrierKind: record.Revocation, relation: record.Revokes},
		{name: "supersession", carrierKind: record.Promise, family: "FR-1", relation: record.Supersedes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			head := ancestor.clone()
			head.n = 10
			target := head.add(record.Promise, "prd/x", "FR-1", "target added by the candidate")

			base := ancestor.clone()
			base.n = 20
			carrier := base.add(tc.carrierKind, "prd/x", tc.family, "historical dangling act", rel(tc.relation, target.ID))
			baseSet := base.set()
			if len(baseSet.Dangling) != 1 || baseSet.Dangling[0].From != carrier.ID || resolve.New(baseSet).Status(carrier.ID) != resolve.Active {
				t.Fatalf("base fixture: dangling=%+v carrier=%s", baseSet.Dangling, resolve.New(baseSet).Status(carrier.ID))
			}

			p := Compute(Input{Base: baseSet, Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
			if codes(p.Blockers) != "historical-act-reinterpreted" || !strings.Contains(p.Blockers[0].What, carrier.ID) || !strings.Contains(p.Blockers[0].What, target.ID) {
				t.Fatalf("candidate target %s silently activated historical %s edge on %s:\n%s", target.ID, tc.relation, carrier.ID, p.Summary())
			}
		})
	}
}

func TestAddedTargetMayRetireHistoricalDanglingCarrier(t *testing.T) {
	for _, tc := range []struct {
		name        string
		carrierKind string
		family      string
		relation    string
	}{
		{name: "revocation", carrierKind: record.Revocation, relation: record.Revokes},
		{name: "supersession", carrierKind: record.Promise, family: "FR-1", relation: record.Supersedes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			head := ancestor.clone()
			head.n = 10
			target := head.add(record.Promise, "prd/x", "FR-1", "target added by the candidate")

			base := ancestor.clone()
			base.n = 20
			carrier := base.add(tc.carrierKind, "prd/x", tc.family, "historical dangling act", rel(tc.relation, target.ID))
			if len(base.set().Dangling) != 1 {
				t.Fatal("base fixture must carry one dangling relation")
			}

			head.n = 30
			head.add(record.Revocation, "prd/x", "", "retire the dangling carrier", rel(record.Revokes, carrier.ID))
			p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
			if !p.OK() || p.Union.Status(carrier.ID) != resolve.Revoked || strings.Contains(codes(p.Blockers), "historical-act-reinterpreted") {
				t.Fatalf("explicitly retiring historical %s carrier %s must remain allowed:\n%s", tc.relation, carrier.ID, p.Summary())
			}
		})
	}
}

func TestAddedTargetCannotActivateSupersessionFromSupersededDanglingCarrier(t *testing.T) {
	ancestor := newTree(t)
	head := ancestor.clone()
	head.n = 10
	target := head.add(record.Promise, "prd/x", "FR-1", "target added by the candidate")

	base := ancestor.clone()
	base.n = 20
	carrier := base.add(record.Promise, "prd/x", "FR-2", "historical dangling superseder", rel(record.Supersedes, target.ID))
	base.n = 30
	base.add(record.Promise, "prd/x", "FR-3", "successor to the dangling carrier", rel(record.Supersedes, carrier.ID))
	baseSet := base.set()
	if len(baseSet.Dangling) != 1 || resolve.New(baseSet).Status(carrier.ID) != resolve.Superseded {
		t.Fatalf("base fixture: dangling=%+v carrier=%s", baseSet.Dangling, resolve.New(baseSet).Status(carrier.ID))
	}

	p := Compute(Input{Base: baseSet, Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "historical-act-reinterpreted" || p.Union.Status(target.ID) != resolve.Superseded || !strings.Contains(p.Blockers[0].What, carrier.ID) {
		t.Fatalf("superseded carrier %s silently activated against %s:\n%s", carrier.ID, target.ID, p.Summary())
	}
}

func TestAddedTargetOwnsConflictsExposedFromBaseDanglingActs(t *testing.T) {
	t.Run("answers", func(t *testing.T) {
		ancestor := newTree(t)
		head := ancestor.clone()
		head.n = 1
		question := head.add(record.Question, "prd/x", "", "which?")

		base := ancestor.clone()
		base.n = 10
		base.add(record.Answer, "prd/x", "", "one", rel(record.Answers, question.ID))
		base.add(record.Answer, "prd/x", "", "two", rel(record.Answers, question.ID))

		p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if codes(p.Blockers) != "answer-conflict,historical-act-reinterpreted,historical-act-reinterpreted" || len(p.Warnings) != 0 {
			t.Fatalf("candidate question did not own the conflict it exposed: %s", p.Summary())
		}
	})

	t.Run("revocations", func(t *testing.T) {
		ancestor := newTree(t)
		head := ancestor.clone()
		head.n = 1
		target := head.add(record.Need, "prd/x", "G-1", "target")

		base := ancestor.clone()
		base.n = 10
		base.add(record.Revocation, "prd/x", "", "one", rel(record.Revokes, target.ID))
		base.add(record.Revocation, "prd/x", "", "two", rel(record.Revokes, target.ID))

		p := Compute(Input{Base: base.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if codes(p.Blockers) != "revocation-conflict,historical-act-reinterpreted,historical-act-reinterpreted" || len(p.Warnings) != 0 {
			t.Fatalf("candidate target did not own the conflict it exposed: %s", p.Summary())
		}
	})
}

func TestRevocationThatWouldCorruptAProjectionBlocks(t *testing.T) {
	t.Run("answered question", func(t *testing.T) {
		base := newTree(t)
		q := base.add(record.Question, "prd/x", "", "which?")
		base.add(record.Answer, "prd/x", "", "this", rel(record.Answers, q.ID))
		head := base.clone()
		head.add(record.Revocation, "prd/x", "", "withdraw question", rel(record.Revokes, q.ID))
		p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if codes(p.Blockers) != "revocation-breaks-projection" || p.Union.Status(q.ID) != resolve.Active {
			t.Fatalf("answered-question revocation: %s", p.Summary())
		}
	})

	t.Run("reactivated family conflict", func(t *testing.T) {
		base := newTree(t)
		left := base.add(record.Promise, "prd/x", "FR-1", "left")
		right := base.add(record.Promise, "prd/x", "FR-1", "right")
		headRecord := base.add(record.Promise, "prd/x", "FR-1", "resolved", rel(record.Supersedes, left.ID), rel(record.Supersedes, right.ID))
		if conflicts := resolve.New(base.set()).Conflicts(); len(conflicts) != 0 {
			t.Fatalf("base is not clean: %+v", conflicts)
		}
		head := base.clone()
		head.add(record.Revocation, "prd/x", "", "withdraw resolution", rel(record.Revokes, headRecord.ID))
		p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
		if codes(p.Blockers) != "revocation-breaks-projection" || p.Union.Status(headRecord.ID) != resolve.Active {
			t.Fatalf("family-reactivating revocation: %s", p.Summary())
		}
	})
}

func TestAnswersToTheWrongOrDeadQuestionBlock(t *testing.T) {
	ancestor := newTree(t)
	q1 := ancestor.add(record.Question, "prd/x", "", "one?")
	q2 := ancestor.add(record.Question, "prd/x", "", "two?")
	a1 := ancestor.add(record.Answer, "prd/x", "", "one!", rel(record.Answers, q1.ID))
	// a stale branch answers q2 while main revokes it
	head := ancestor.clone()
	head.add(record.Answer, "prd/x", "", "two!", rel(record.Answers, q2.ID))
	moved := ancestor.clone()
	moved.n = 200
	moved.add(record.Revocation, "prd/x", "", "two withdrawn", rel(record.Revokes, q2.ID))
	p := Compute(Input{Base: moved.set(), Ancestor: ancestor.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "revocation-breaks-projection" {
		t.Fatalf("answering a revoked question: %s", p.Summary())
	}
	// an answer that supersedes another question's answer
	hijack := ancestor.clone()
	hijack.n = 100
	hijack.add(record.Answer, "prd/x", "", "two, hijacking one", rel(record.Answers, q2.ID), rel(record.Supersedes, a1.ID))
	p = Compute(Input{Base: ancestor.set(), Ancestor: ancestor.set(), Head: hijack.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "supersedes-across-questions" || p.Union.Answer(q1.ID) == nil {
		t.Fatalf("cross-question supersession: %s", p.Summary())
	}
}

func TestSupersessionViolationsBlock(t *testing.T) {
	base := newTree(t)
	fr1 := base.add(record.Promise, "prd/demo", "FR-1", "the demo requirement")
	fr6 := base.add(record.Promise, "prd/demo", "FR-6", "six")
	head := base.clone()
	head.add(record.Promise, "prd/other", "FR-99", "retires a stranger", rel(record.Supersedes, fr1.ID))
	head.add(record.Promise, "prd/demo", "FR-21", "six, renumbered", rel(record.Supersedes, fr6.ID))
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "supersedes-across-subjects" || p.Union.Status(fr1.ID) != "active" {
		t.Fatalf("blockers %+v; FR-1 %v", p.Blockers, p.Union.Status(fr1.ID))
	}
	if len(p.Moved) != 0 {
		t.Fatalf("an invalid edge is not a move: %+v", p.Moved)
	}
	if !strings.Contains(p.Render(), "### Promise FR-21 on prd/demo — replaces the current promise (renamed from FR-6)") {
		t.Fatalf("rename named:\n%s", p.Render())
	}
	// a violation already on the base is the owner's problem, not this proposal's
	stale := newTree(t)
	a := stale.add(record.Promise, "prd/x", "FR-1", "one")
	stale.add(record.Promise, "prd/y", "FR-1", "elsewhere", rel(record.Supersedes, a.ID))
	head2 := stale.clone()
	head2.add(record.Need, "prd/x", "G-1", "goal")
	p = Compute(Input{Base: stale.set(), Head: head2.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() || codes(p.Warnings) != "supersedes-across-subjects-on-base" {
		t.Fatalf("on-base violation: %+v %+v", p.Blockers, p.Warnings)
	}
}
func (tr *tree) doc(subject, slot, title, body string, rels ...record.Relation) *record.Record {
	r := tr.add(record.Document, subject, slot, title, rels...)
	r.BodySHA256 = record.HashBody([]byte(body))
	r.Embeds = record.ParseEmbeds([]byte(body), subject)
	b, err := record.Marshal(r)
	if err != nil {
		tr.t.Fatal(err)
	}
	tr.files[r.Path()] = b
	tr.files[r.BodyPath()] = []byte(body)
	return r
}

func TestExistingDocumentsAreRevalidated(t *testing.T) {
	base := newTree(t)
	fr1 := base.add(record.Promise, "prd/x", "FR-1", "one")
	doc := base.doc("prd/x", "prd", "The PRD", "# The PRD\n\n{{FR-1}}\n")
	// revoking the embedded claim without revising the document breaks it
	head := base.clone()
	head.add(record.Revocation, "prd/x", "", "one withdrawn", rel(record.Revokes, fr1.ID))
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "revocation-breaks-projection" || !strings.Contains(p.Blockers[0].What, "unresolved embed {{prd/x#FR-1}} in "+doc.ID) {
		t.Fatalf("revocation: %+v", p.Blockers)
	}
	// renaming it (FR-1 → FR-9) breaks it the same way
	head2 := base.clone()
	head2.add(record.Promise, "prd/x", "FR-9", "one, renumbered", rel(record.Supersedes, fr1.ID))
	p = Compute(Input{Base: base.set(), Head: head2.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "embed-unresolved" {
		t.Fatalf("rename: %+v", p.Blockers)
	}
	// revising the document in the same proposal resolves it
	head3 := base.clone()
	head3.add(record.Promise, "prd/x", "FR-9", "one, renumbered", rel(record.Supersedes, fr1.ID))
	head3.doc("prd/x", "prd", "The PRD", "# The PRD\n\n{{FR-9}}\n", rel(record.Supersedes, doc.ID))
	p = Compute(Input{Base: base.set(), Head: head3.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() {
		t.Fatalf("revised alongside: %+v", p.Blockers)
	}
	// a document already broken on the base is the owner's problem, not this proposal's
	broken := base.clone()
	broken.add(record.Revocation, "prd/x", "", "one withdrawn on main", rel(record.Revokes, fr1.ID))
	head4 := broken.clone()
	head4.add(record.Need, "prd/x", "G-1", "goal")
	p = Compute(Input{Base: broken.set(), Head: head4.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() || !strings.Contains(codes(p.Warnings), "revocation-breaks-projection-on-base") {
		t.Fatalf("broken on base: %+v %+v", p.Blockers, p.Warnings)
	}
}

func TestReactivatedBrokenDocumentBlocks(t *testing.T) {
	base := newTree(t)
	x := base.add(record.Promise, "prd/x", "FR-1", "x")
	v1 := base.doc("prd/x", "prd", "The PRD", "# v1\n\n{{FR-1}}\n")
	v2 := base.doc("prd/x", "prd", "The PRD", "# v2\n\nno markers\n", rel(record.Supersedes, v1.ID))
	base.add(record.Revocation, "prd/x", "", "x withdrawn", rel(record.Revokes, x.ID))
	// the base is fine: v2 is active and embeds nothing
	quiet := base.clone()
	quiet.add(record.Need, "prd/x", "G-1", "goal")
	p := Compute(Input{Base: base.set(), Head: quiet.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() || strings.Contains(codes(p.Warnings), "embed") {
		t.Fatalf("a superseded broken document is history: %+v %+v", p.Blockers, p.Warnings)
	}
	// revoking v2 makes v1 active again, and v1 embeds a claim that no longer exists: this proposal did that
	head := base.clone()
	head.add(record.Revocation, "prd/x", "", "v2 withdrawn", rel(record.Revokes, v2.ID))
	p = Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "revocation-breaks-projection" || !strings.Contains(p.Blockers[0].What, "unresolved embed {{prd/x#FR-1}} in "+v1.ID) {
		t.Fatalf("a reactivated broken document must block: %+v %+v", p.Blockers, p.Warnings)
	}
	// the on-base warning reaches the PR body
	broken := newTree(t)
	y := broken.add(record.Promise, "prd/y", "FR-1", "y")
	broken.doc("prd/y", "prd", "The Y PRD", "# y\n\n{{FR-1}}\n")
	broken.add(record.Revocation, "prd/y", "", "y withdrawn on main", rel(record.Revokes, y.ID))
	head2 := broken.clone()
	head2.add(record.Need, "prd/y", "G-1", "goal")
	p = Compute(Input{Base: broken.set(), Head: head2.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if !p.OK() || codes(p.Warnings) != "revocation-breaks-projection-on-base" {
		t.Fatalf("broken on base: %+v %+v", p.Blockers, p.Warnings)
	}
	if body := p.Render(); !strings.Contains(body, "## Already wrong on the base") || !strings.Contains(body, "cannot revoke "+y.ID) {
		t.Fatalf("the on-base warning is in the PR body:\n%s", body)
	}
}

func TestDocumentDiffFenceAndLineages(t *testing.T) {
	base := newTree(t)
	v1 := base.doc("prd/x", "prd", "The PRD", "# The PRD\n\nExample:\n\n```go\nfmt.Println(1)\n```\n")
	head := base.clone()
	head.doc("prd/x", "prd", "The PRD", "# The PRD\n\nExample:\n\n```go\nfmt.Println(2)\n```\n", rel(record.Supersedes, v1.ID))
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	body := p.Render()
	if !strings.Contains(body, "\n````diff\n") || !strings.Contains(body, "\n````\n") || strings.Contains(body, "\n```diff\n") {
		t.Fatalf("a fenced example inside the diff needs a longer fence:\n%s", body)
	}
	if fenceFor([]string{" a", "+b"}) != "```" || fenceFor([]string{" ``````x"}) != "```````" {
		t.Fatalf("fenceFor: %q %q", fenceFor([]string{" a"}), fenceFor([]string{" ``````x"}))
	}
	// a document reconciling two lineages: history follows the first predecessor, the diff is against it, the second is named
	two := newTree(t)
	orig := two.doc("prd/z", "prd", "Z", "# Z\n\none\n")
	left := two.doc("prd/z", "prd", "Z", "# Z\n\nleft\n", rel(record.Supersedes, orig.ID))
	right := two.doc("prd/z", "prd", "Z", "# Z\n\nright\n", rel(record.Supersedes, orig.ID))
	head2 := two.clone()
	merged := head2.doc("prd/z", "prd", "Z", "# Z\n\nleft and right\n", rel(record.Supersedes, left.ID), rel(record.Supersedes, right.ID))
	p = Compute(Input{Base: two.set(), Head: head2.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	body = p.Render()
	if !strings.Contains(body, "- **also replaces**: "+right.ID+" (reconciling more than one lineage; the diff below is against "+left.ID+")") || !strings.Contains(body, "-left\n+left and right") {
		t.Fatalf("two lineages:\n%s", body)
	}
	if chain := p.Union.Chain(merged.ID, 10); len(chain) != 3 || chain[1].ID != left.ID {
		t.Fatalf("history follows the first predecessor: %v", chain)
	}
	// An empty body is still a real predecessor, not evidence that v2 is new.
	emptyBase := newTree(t)
	empty := emptyBase.doc("prd/empty", "prd", "Empty", "")
	emptyHead := emptyBase.clone()
	emptyHead.doc("prd/empty", "prd", "Now written", "first line\n", rel(record.Supersedes, empty.ID))
	p = Compute(Input{Base: emptyBase.set(), Head: emptyHead.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	body = p.Render()
	if strings.Contains(body, "new document") || !strings.Contains(body, "- **changed**: 1 lines added, 0 removed") || !strings.Contains(body, "+first line") {
		t.Fatalf("an empty predecessor still gets a version diff:\n%s", body)
	}
	if lines := documentLines(""); len(lines) != 0 {
		t.Fatalf("an empty body has no lines: %q", lines)
	}
}

func TestDocumentsInThePacket(t *testing.T) {
	base := newTree(t)
	fr1 := base.add(record.Promise, "prd/x", "FR-1", "one")
	v1 := base.doc("prd/x", "prd", "The PRD", "# The PRD\n\nIntro.\n\n{{FR-1}}\n\nClosing.\n")
	head := base.clone()
	head.add(record.Promise, "prd/x", "FR-1", "one, corrected", rel(record.Supersedes, fr1.ID))
	v2 := head.doc("prd/x", "prd", "The PRD", "# The PRD\n\nIntro, rewritten.\n\n{{FR-1}}\n\nA new paragraph about {{FR-9}}.\n\nClosing.\n", rel(record.Supersedes, v1.ID))
	v2.Intent = "explain the correction"
	fresh := head.doc("design/x", "design", "The design", "# The design\n\nAll new.\n")
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if codes(p.Blockers) != "embed-unresolved" || !strings.Contains(p.Blockers[0].What, "{{prd/x#FR-9}}") {
		t.Fatalf("blockers %+v", p.Blockers)
	}
	body := p.Render()
	for _, want := range []string{"## Narrative delta — the documents", "### prd/x · prd \"The PRD\" — version 2", "- **embeds**: prd/x#FR-1, prd/x#FR-9",
		"- **changed**: 3 lines added, 1 removed", "```diff\n # The PRD\n \n-Intro.\n+Intro, rewritten.\n \n {{FR-1}}\n \n+A new paragraph about {{FR-9}}.\n+\n Closing.\n```",
		"### design/x · design \"The design\" — version 1", "- **new document**: 3 lines", "```diff\n+# The design\n+\n+All new.\n```"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body lacks %q:\n%s", want, body)
		}
	}
	_ = fresh
	if d := lineDiff([]string{"a", "b", "c", "d", "e", "f", "g"}, []string{"a", "b", "c", "x", "e", "f", "g"}, 1); strings.Join(d, "|") != " c|-d|+x| e" {
		t.Fatalf("diff %q", strings.Join(d, "|"))
	}
	if d := lineDiff([]string{"a", "b", "c", "d", "e", "f", "g", "h"}, []string{"a", "x", "c", "d", "e", "f", "y", "h"}, 1); strings.Join(d, "|") != " a|-b|+x| c|@@ …| f|-g|+y| h" {
		t.Fatalf("diff with a gap %q", strings.Join(d, "|"))
	}
	if d := lineDiff(nil, []string{"a"}, 0); strings.Join(d, "|") != "+a" {
		t.Fatalf("diff from nothing %q", d)
	}
}

func TestLineDiffEditPathsAndBoundedFallback(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
	}{
		{"equal", []string{"a", "b"}, []string{"a", "b"}},
		{"insert", []string{"a", "c"}, []string{"a", "b", "c"}},
		{"delete", []string{"a", "b", "c"}, []string{"a", "c"}},
		{"replace", []string{"a", "old", "z"}, []string{"a", "new", "z"}},
		{"duplicates", []string{"a", "b", "a", "c"}, []string{"a", "a", "b", "c"}},
		{"empty-left", nil, []string{"a", "b"}},
		{"empty-right", []string{"a", "b"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertLineOperations(t, tc.a, tc.b, diffOperations(tc.a, tc.b, maxLineDiffTraceInts))
		})
	}

	a := []string{"same", "old one", "old two", "tail"}
	b := []string{"same", "new one", "new two", "tail"}
	if _, ok := myersLineOperations(a, b, 1); ok {
		t.Fatal("a one-integer trace budget should force the fallback")
	}
	ops := diffOperations(a, b, 1)
	assertLineOperations(t, a, b, ops)
	if got := formatLineOperations(ops); got != " same|-old one|-old two|+new one|+new two| tail" {
		t.Fatalf("coarse fallback = %q", got)
	}

	// Completely different large inputs used to allocate an n×m integer
	// matrix. They now exhaust the fixed trace budget and take the linear path.
	largeA, largeB := make([]string, 5000), make([]string, 5000)
	for i := range largeA {
		largeA[i], largeB[i] = fmt.Sprintf("old-%d", i), fmt.Sprintf("new-%d", i)
	}
	if _, ok := myersLineOperations(largeA, largeB, maxLineDiffTraceInts); ok {
		t.Fatal("a disjoint 10,000-line edit should exceed the bounded trace")
	}
	largeOps := diffOperations(largeA, largeB, maxLineDiffTraceInts)
	if len(largeOps) != len(largeA)+len(largeB) {
		t.Fatalf("large fallback returned %d operations", len(largeOps))
	}
	assertLineOperations(t, largeA, largeB, largeOps)
}

func assertLineOperations(t *testing.T, a, b []string, ops []lineOp) {
	t.Helper()
	remaining := append([]string(nil), a...)
	var rebuilt []string
	for _, op := range ops {
		switch op.kind {
		case ' ':
			if len(remaining) == 0 || remaining[0] != op.text {
				t.Fatalf("equal operation %q does not match input %v", op.text, remaining)
			}
			rebuilt = append(rebuilt, op.text)
			remaining = remaining[1:]
		case '-':
			if len(remaining) == 0 || remaining[0] != op.text {
				t.Fatalf("delete operation %q does not match input %v", op.text, remaining)
			}
			remaining = remaining[1:]
		case '+':
			rebuilt = append(rebuilt, op.text)
		default:
			t.Fatalf("unknown operation %q", op.kind)
		}
	}
	if len(remaining) != 0 || !reflect.DeepEqual(rebuilt, b) {
		t.Fatalf("operations left %v and rebuilt %v, want %v", remaining, rebuilt, b)
	}
}

func formatLineOperations(ops []lineOp) string {
	lines := make([]string, 0, len(ops))
	for _, op := range ops {
		lines = append(lines, string(op.kind)+op.text)
	}
	return strings.Join(lines, "|")
}

func TestConcurrentWorkSkipsSupersededInFlightRecords(t *testing.T) {
	base := newTree(t)
	head := base.clone()
	oursOld := head.add(record.Promise, "prd/x", "FR-1", "our old version")
	oursCurrent := head.add(record.Promise, "prd/x", "FR-1", "our current version", rel(record.Supersedes, oursOld.ID))

	other := newTree(t)
	old := other.add(record.Promise, "prd/x", "FR-1", "their old version")
	current := other.add(record.Promise, "prd/x", "FR-1", "their current version", rel(record.Supersedes, old.ID))

	p := Compute(Input{Base: base.set(), Head: head.set(), InFlight: map[string]*store.Set{"feature/other": other.set()}, Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if len(p.Concurrent) != 1 || p.Concurrent[0].Ours.ID != oursCurrent.ID || p.Concurrent[0].Theirs.ID != current.ID {
		t.Fatalf("only the active heads on both in-flight branches are concurrent: %+v", p.Concurrent)
	}
}

func TestDocumentModeChangeBlocksAndLiteralModeIsVisible(t *testing.T) {
	base := newTree(t)
	v1 := base.doc("prd/x", "source", "Imported source", "# Native\n")
	head := base.clone()
	v2 := head.doc("prd/x", "source", "Imported source", "# Native\n", rel(record.Supersedes, v1.ID))
	v2.LiteralBody = true
	v2.Embeds = nil
	b, err := record.Marshal(v2)
	if err != nil {
		t.Fatal(err)
	}
	head.files[v2.Path()] = b
	p := Compute(Input{Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "b", BaseRef: "main"})
	if got := codes(p.Blockers); !strings.Contains(got, "document-mode-changed") {
		t.Fatalf("mode change blockers: %+v", p.Blockers)
	}
	if body := p.Render(); !strings.Contains(body, "- **mode**: literal source body") {
		t.Fatalf("literal mode is not visible in packet:\n%s", body)
	}
}
