package resolve

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

var t0 = time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

type fixture struct {
	t     *testing.T
	files store.MapSource
	n     byte
}

func (f *fixture) add(kind, subject, family, statement string, rels ...record.Relation) *record.Record {
	f.n++
	var e [10]byte
	e[9] = f.n
	r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, t0.Add(time.Duration(f.n)*time.Second), e), Kind: kind, Subject: subject,
		Family: family, Statement: statement, Issuer: "m@example.com", CreatedAt: t0.Format(time.RFC3339), Relations: rels}
	if kind == record.Assumption {
		r.Expires = t0.Add(time.Hour).Format(time.RFC3339)
	}
	b, err := record.Marshal(r)
	if err != nil {
		f.t.Fatal(err)
	}
	f.files[r.Path()] = b
	return r
}

func (f *fixture) projection() *Projection {
	s, err := store.Load(context.Background(), f.files)
	if err != nil {
		f.t.Fatal(err)
	}
	if len(s.Problems) > 0 {
		f.t.Fatalf("fixture problems: %v", s.Problems)
	}
	return New(s)
}

func rel(t, target string) record.Relation { return record.Relation{Type: t, Target: target} }

func fixtureID(kind string, n byte) string {
	var entropy [10]byte
	entropy[9] = n
	return record.NewID(kind, t0.Add(time.Duration(n)*time.Second), entropy)
}

func TestStatusHeadsConflictsAndChains(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}}
	fr1 := f.add(record.Promise, "prd/x", "FR-1", "v1")
	fr1b := f.add(record.Promise, "prd/x", "FR-1", "v2", rel(record.Supersedes, fr1.ID))
	fr1c := f.add(record.Promise, "prd/x", "FR-1", "v3", rel(record.Supersedes, fr1b.ID))
	fr2 := f.add(record.Promise, "prd/x", "FR-2", "two")
	fr2dup := f.add(record.Promise, "prd/x", "FR-2", "two again, no supersedes")
	fr3 := f.add(record.Promise, "prd/x", "FR-3", "three")
	rev := f.add(record.Revocation, "prd/x", "", "withdrawn", rel(record.Revokes, fr3.ID))
	q := f.add(record.Question, "prd/x", "", "which colour?")
	p := f.projection()

	if p.Status(fr1.ID) != Superseded || p.Status(fr1b.ID) != Superseded || p.Status(fr1c.ID) != Active {
		t.Fatalf("chain statuses: %v %v %v", p.Status(fr1.ID), p.Status(fr1b.ID), p.Status(fr1c.ID))
	}
	if p.Status(fr3.ID) != Revoked || p.Status(rev.ID) != Active || p.Status(q.ID) != Active || p.Status("nope") != "" {
		t.Fatal("revocation statuses")
	}
	if h := p.Head("prd/x", "FR-1"); h == nil || h.ID != fr1c.ID {
		t.Fatalf("head FR-1: %v", h)
	}
	if h := p.Head("prd/x", "FR-2"); h == nil || h.ID != fr2dup.ID {
		t.Fatalf("conflicting family head is the newest: %v", h)
	}
	if p.Head("prd/x", "FR-3") != nil || p.Head("prd/nope", "FR-1") != nil {
		t.Fatal("revoked family has no head")
	}
	if got := strings.Join(p.Families("prd/x"), ","); got != "FR-1,FR-2" {
		t.Fatalf("families %q", got)
	}
	c := p.Conflicts()
	if len(c) != 1 || c[0].Family != "FR-2" || len(c[0].IDs) != 2 || c[0].IDs[0] != fr2.ID {
		t.Fatalf("conflicts %+v", c)
	}
	chain := p.Chain(fr1c.ID, 10)
	if len(chain) != 3 || chain[0].ID != fr1c.ID || chain[2].ID != fr1.ID {
		t.Fatalf("chain %v", chain)
	}
	if len(p.Chain(fr1c.ID, 2)) != 2 || len(p.Chain("nope", 5)) != 0 {
		t.Fatal("chain limits")
	}
	if s := p.Successors(fr1.ID); len(s) != 1 || s[0].ID != fr1c.ID {
		t.Fatalf("successors %v", s)
	}
	active := p.Active("prd/x")
	if len(active) != 5 { // fr1c, fr2, fr2dup, rev, q
		t.Fatalf("active %d", len(active))
	}
}

func TestRevokedSupersederRestoresThePrevious(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}}
	fr1 := f.add(record.Promise, "prd/x", "FR-1", "v1")
	fr1b := f.add(record.Promise, "prd/x", "FR-1", "v2", rel(record.Supersedes, fr1.ID))
	f.add(record.Revocation, "prd/x", "", "v2 withdrawn", rel(record.Revokes, fr1b.ID))
	p := f.projection()
	if p.Status(fr1.ID) != Active || p.Status(fr1b.ID) != Revoked {
		t.Fatalf("revoking the successor reactivates the previous: %v %v", p.Status(fr1.ID), p.Status(fr1b.ID))
	}
}

func TestDesignRelations(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}}
	fr1 := f.add(record.Promise, "prd/x", "FR-1", "v1")
	fr2 := f.add(record.Promise, "prd/x", "FR-2", "two")
	dod := f.add(record.Expectation, "prd/x", "DOD-1", "done when")
	anchor := f.add(record.Need, "code/executor", "anchor", "package executor exists")
	d1 := f.add(record.Decision, "design/x", "D-1", "serves FR-1 and DOD-1", rel(record.Serves, fr1.ID), rel(record.Serves, dod.ID), rel(record.About, anchor.ID))
	d2 := f.add(record.Decision, "design/x", "D-2", "serves FR-1 only", rel(record.Serves, fr1.ID), rel(record.DependsOn, d1.ID))
	plan := f.add(record.Plan, "plan/x", "", "task", rel(record.Serves, fr1.ID))
	learn := f.add(record.Learning, "prd/x", "", "lesson", rel(record.Refines, fr1.ID))
	fr1b := f.add(record.Promise, "prd/x", "FR-1", "v2", rel(record.Supersedes, fr1.ID))
	p := f.projection()

	deps := p.Dependents(fr1.ID)
	var got []string
	for _, d := range deps {
		got = append(got, d.Record.ID+":"+d.Type)
	}
	if strings.Join(got, ",") != d1.ID+":serves,"+d2.ID+":serves,"+learn.ID+":refines,"+plan.ID+":serves" {
		t.Fatalf("dependents %v", got)
	}
	if len(p.Dependents(d1.ID)) != 1 || p.Dependents(d1.ID)[0].Type != record.DependsOn {
		t.Fatal("depends_on dependents")
	}
	re := p.Reopened([]string{"design/x", "plan/x"})
	if len(re) != 3 || re[0].Record.ID != d1.ID || re[0].Serves != fr1.ID || re[0].Status != Superseded || re[0].Successor == nil || re[0].Successor.ID != fr1b.ID {
		t.Fatalf("reopened %+v", re)
	}
	if len(p.Reopened(nil)) != 3 {
		t.Fatal("reopened over all subjects")
	}
	und := p.Undesigned("prd/x")
	if len(und) != 2 || und[0].ID != fr2.ID || und[1].ID != fr1b.ID {
		t.Fatalf("undesigned %v", und)
	}
	if len(p.Orphaned()) != 0 {
		t.Fatal("designs that still serve a superseded claim are reopened, not orphaned")
	}
	d3 := f.add(record.Decision, "design/x", "D-3", "serves FR-2 only", rel(record.Serves, fr2.ID))
	f.add(record.Revocation, "prd/x", "", "FR-2 gone: the feature was never built", rel(record.Revokes, fr2.ID))
	p = f.projection()
	orph := p.Orphaned()
	if len(orph) != 1 || orph[0].ID != d3.ID {
		t.Fatalf("orphaned %v (d1 still serves DOD-1, d2's FR-1 has a successor)", orph)
	}
}

func TestAnswerCannotPointForwardToANewerQuestion(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}}
	questionID := fixtureID(record.Question, 2)
	answer := f.add(record.Answer, "prd/x", "", "an answer from before the question", rel(record.Answers, questionID))
	question := f.add(record.Question, "prd/x", "", "what is the answer?")
	if question.ID != questionID {
		t.Fatalf("fixture question id = %s, want %s", question.ID, questionID)
	}

	p := f.projection()
	if len(p.Violations) != 1 || p.Violations[0].Code != "relation-out-of-order" || p.Violations[0].Record != answer.ID || p.Violations[0].Target != question.ID {
		t.Fatalf("forward Answer violations = %+v", p.Violations)
	}
	if p.Answer(question.ID) != nil || len(p.ActiveAnswers(question.ID)) != 0 {
		t.Fatalf("a forward Answer edge was honoured: answer=%v active=%v", p.Answer(question.ID), p.ActiveAnswers(question.ID))
	}
	open := p.OpenQuestions([]string{"prd/x"})
	if len(open) != 1 || open[0].ID != question.ID {
		t.Fatalf("the later Question must remain open: %+v", open)
	}
}

func TestDecisionCannotServeANewerPromise(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}}
	promiseID := fixtureID(record.Promise, 2)
	decision := f.add(record.Decision, "design/x", "D-1", "a decision from before its requirement", rel(record.Serves, promiseID))
	promise := f.add(record.Promise, "prd/x", "FR-1", "the later requirement")
	if promise.ID != promiseID {
		t.Fatalf("fixture promise id = %s, want %s", promise.ID, promiseID)
	}

	p := f.projection()
	if len(p.Violations) != 1 || p.Violations[0].Code != "relation-out-of-order" || p.Violations[0].Record != decision.ID || p.Violations[0].Target != promise.ID {
		t.Fatalf("forward serves violations = %+v", p.Violations)
	}
	if got := p.Targets(decision, record.Serves); len(got) != 0 {
		t.Fatalf("forward serves targets = %v, want none", got)
	}
	if deps := p.Dependents(promise.ID); len(deps) != 0 {
		t.Fatalf("forward serves edge produced dependents: %+v", deps)
	}
	if undesigned := p.Undesigned("prd/x"); len(undesigned) != 1 || undesigned[0].ID != promise.ID {
		t.Fatalf("the later Promise must remain undesigned: %+v", undesigned)
	}
}

func TestEveryOtherPresentRelationMustPointBackward(t *testing.T) {
	for _, typ := range []string{record.DependsOn, record.About, record.Refines, record.Conflicts} {
		t.Run(typ, func(t *testing.T) {
			f := &fixture{t: t, files: store.MapSource{}}
			targetID := fixtureID(record.Need, 2)
			carrier := f.add(record.Learning, "design/x", "", "forward relation", rel(typ, targetID))
			target := f.add(record.Need, "code/x", "anchor", "later target")
			if target.ID != targetID {
				t.Fatalf("fixture target id = %s, want %s", target.ID, targetID)
			}

			p := f.projection()
			if len(p.Violations) != 1 || p.Violations[0].Code != "relation-out-of-order" || p.Violations[0].Record != carrier.ID || p.Violations[0].Target != target.ID {
				t.Fatalf("%s forward-edge violations = %+v", typ, p.Violations)
			}
			if got := p.Targets(carrier, typ); len(got) != 0 {
				t.Fatalf("%s forward targets = %v, want none", typ, got)
			}
		})
	}
}

func TestQuestionsAndAssumptions(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}}
	q1 := f.add(record.Question, "prd/x", "", "which colour?")
	q2 := f.add(record.Question, "prd/y", "", "which size?")
	a1 := f.add(record.Answer, "prd/x", "", "blue", rel(record.Answers, q1.ID))
	a1b := f.add(record.Answer, "prd/x", "", "navy, on reflection", rel(record.Answers, q1.ID), rel(record.Supersedes, a1.ID))
	old := f.add(record.Assumption, "prd/x", "A-1", "holds until")
	p := f.projection()
	if a := p.Answer(q1.ID); a == nil || a.ID != a1b.ID || len(p.ActiveAnswers(q1.ID)) != 1 || len(p.AnswerConflicts()) != 0 {
		t.Fatalf("the replacing answer stands alone: %v (first was %s)", a, a1.ID)
	}
	// revoking the replacement restores the first answer; revoking that too leaves the question open
	f.add(record.Revocation, "prd/x", "", "navy was wrong", rel(record.Revokes, a1b.ID))
	p = f.projection()
	if a := p.Answer(q1.ID); a == nil || a.ID != a1.ID {
		t.Fatalf("after revoking the correction: %v", a)
	}
	f.add(record.Revocation, "prd/x", "", "blue was wrong too", rel(record.Revokes, a1.ID))
	p = f.projection()
	if p.Answer(q1.ID) != nil || len(p.OpenQuestions([]string{"prd/x"})) != 1 {
		t.Fatal("a question whose answers are all revoked is open again")
	}
	// two answers with no supersession between them are a conflict; the newest stands meanwhile
	c1 := f.add(record.Answer, "prd/x", "", "green", rel(record.Answers, q1.ID))
	c2 := f.add(record.Answer, "prd/x", "", "red", rel(record.Answers, q1.ID))
	p = f.projection()
	if cs := p.AnswerConflicts(); len(cs) != 1 || cs[0].Question != q1.ID || len(cs[0].IDs) != 2 || cs[0].IDs[0] != c1.ID {
		t.Fatalf("answer conflicts %+v", cs)
	}
	if p.Answer(q1.ID).ID != c2.ID {
		t.Fatal("newest stands until resolved")
	}
	f = &fixture{t: t, files: store.MapSource{}}
	q1 = f.add(record.Question, "prd/x", "", "which colour?")
	q2 = f.add(record.Question, "prd/y", "", "which size?")
	f.add(record.Answer, "prd/x", "", "blue", rel(record.Answers, q1.ID))
	old = f.add(record.Assumption, "prd/x", "A-1", "holds until")
	p = f.projection()
	if p.Answer(q2.ID) != nil {
		t.Fatal("unanswered")
	}
	if o := p.OpenQuestions(nil); len(o) != 1 || o[0].ID != q2.ID {
		t.Fatalf("open %v", o)
	}
	if o := p.OpenQuestions([]string{"prd/x"}); len(o) != 0 {
		t.Fatalf("open on prd/x %v", o)
	}
	if e := p.Expired(t0.Add(2 * time.Hour)); len(e) != 1 || e[0].ID != old.ID {
		t.Fatalf("expired %v", e)
	}
	if e := p.Expired(t0); len(e) != 0 {
		t.Fatalf("not yet expired %v", e)
	}
}

func TestAnswersAreBoundToTheirQuestion(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}}
	q1 := f.add(record.Question, "prd/x", "", "one?")
	q2 := f.add(record.Question, "prd/x", "", "two?")
	a1 := f.add(record.Answer, "prd/x", "", "answer to one", rel(record.Answers, q1.ID))
	a2 := f.add(record.Answer, "prd/x", "", "answer to two, hijacking one", rel(record.Answers, q2.ID), rel(record.Supersedes, a1.ID))
	p := f.projection()
	if p.Status(a1.ID) != Active || len(p.Violations) != 1 || p.Violations[0].Code != "supersedes-across-questions" || p.Violations[0].Record != a2.ID {
		t.Fatalf("an answer to another question retires nothing: %v %+v", p.Status(a1.ID), p.Violations)
	}
	if p.Answer(q1.ID) == nil || p.Answer(q1.ID).ID != a1.ID {
		t.Fatal("q1 keeps its answer")
	}
	// withdrawing an answered question would invalidate its answer, so the
	// revocation edge is refused and the answered question stays active
	revocation := f.add(record.Revocation, "prd/x", "", "two withdrawn", rel(record.Revokes, q2.ID))
	p = f.projection()
	var codes []string
	for _, v := range p.Violations {
		codes = append(codes, v.Code)
	}
	if strings.Join(codes, ",") != "supersedes-across-questions,revocation-breaks-projection" || p.Status(q2.ID) != Active || p.Status(revocation.ID) != Active || p.Answer(q2.ID) == nil {
		t.Fatalf("answer to a revoked question: %v %v", codes, p.Answer(q2.ID))
	}
}

func TestActsStayOnSubjectAndApplyOnlyOnce(t *testing.T) {
	t.Run("answer across subjects", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		q := f.add(record.Question, "prd/x", "", "which?")
		a := f.add(record.Answer, "prd/y", "", "wrong subject", rel(record.Answers, q.ID))
		p := f.projection()
		if len(p.Violations) != 1 || p.Violations[0].Code != "answers-across-subjects" || p.Violations[0].Record != a.ID {
			t.Fatalf("cross-subject answer violations = %+v", p.Violations)
		}
		if p.Answer(q.ID) != nil || len(p.OpenQuestions([]string{"prd/x"})) != 1 {
			t.Fatal("a cross-subject answer must not answer the question")
		}
	})

	t.Run("revocation across subjects", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		claim := f.add(record.Promise, "prd/x", "FR-1", "kept")
		revocation := f.add(record.Revocation, "prd/y", "", "wrong subject", rel(record.Revokes, claim.ID))
		p := f.projection()
		if len(p.Violations) != 1 || p.Violations[0].Code != "revokes-across-subjects" || p.Violations[0].Record != revocation.ID {
			t.Fatalf("cross-subject revocation violations = %+v", p.Violations)
		}
		if p.Status(claim.ID) != Active {
			t.Fatalf("cross-subject revocation changed target status to %s", p.Status(claim.ID))
		}
	})

	t.Run("duplicate revocation", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		claim := f.add(record.Promise, "prd/x", "FR-1", "withdrawn once")
		first := f.add(record.Revocation, "prd/x", "", "first", rel(record.Revokes, claim.ID))
		second := f.add(record.Revocation, "prd/x", "", "second", rel(record.Revokes, claim.ID))
		p := f.projection()
		conflicts := p.RevocationConflicts()
		if len(p.Violations) != 0 || len(conflicts) != 1 || conflicts[0].Target != claim.ID || strings.Join(conflicts[0].IDs, ",") != first.ID+","+second.ID || p.Status(claim.ID) != Revoked {
			t.Fatalf("duplicate revocations: violations=%+v conflicts=%+v status=%s", p.Violations, conflicts, p.Status(claim.ID))
		}
	})

	t.Run("revocation of superseded record", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		old := f.add(record.Promise, "prd/x", "FR-1", "old")
		newer := f.add(record.Promise, "prd/x", "FR-1", "new", rel(record.Supersedes, old.ID))
		revocation := f.add(record.Revocation, "prd/x", "", "too late", rel(record.Revokes, old.ID))
		p := f.projection()
		if len(p.Violations) != 1 || p.Violations[0].Code != "revokes-inactive-record" || p.Violations[0].Record != revocation.ID || !strings.Contains(p.Violations[0].What, newer.ID) {
			t.Fatalf("inactive-target revocation violations = %+v", p.Violations)
		}
	})
}

func TestRevocationProjectionIsCausalAndRepairable(t *testing.T) {
	t.Run("revoked act has no effect", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		claim := f.add(record.Promise, "prd/x", "FR-1", "kept")
		first := f.add(record.Revocation, "prd/x", "", "withdrawn", rel(record.Revokes, claim.ID))
		undo := f.add(record.Revocation, "prd/x", "", "withdrawal was invalid", rel(record.Revokes, first.ID))
		p := f.projection()
		if p.Status(first.ID) != Revoked || p.Status(undo.ID) != Active || p.Status(claim.ID) != Active || len(p.Violations) != 0 {
			t.Fatalf("undo projection: claim=%s first=%s undo=%s violations=%+v", p.Status(claim.ID), p.Status(first.ID), p.Status(undo.ID), p.Violations)
		}
		third := f.add(record.Revocation, "prd/x", "", "withdrawn now", rel(record.Revokes, claim.ID))
		p = f.projection()
		if p.Status(claim.ID) != Revoked || p.Status(third.ID) != Active || len(p.RevocationConflicts()) != 0 {
			t.Fatalf("replacement revocation: claim=%s conflicts=%+v", p.Status(claim.ID), p.RevocationConflicts())
		}
	})

	t.Run("invalid cross-subject act can be revoked", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		claim := f.add(record.Promise, "prd/x", "FR-1", "kept")
		bad := f.add(record.Revocation, "prd/y", "", "wrong subject", rel(record.Revokes, claim.ID))
		f.add(record.Revocation, "prd/y", "", "remove bad act", rel(record.Revokes, bad.ID))
		p := f.projection()
		if p.Status(bad.ID) != Revoked || p.Status(claim.ID) != Active || len(p.Violations) != 0 {
			t.Fatalf("repair projection: claim=%s bad=%s violations=%+v", p.Status(claim.ID), p.Status(bad.ID), p.Violations)
		}
	})

	t.Run("later superseder is blamed after revocation", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		claim := f.add(record.Promise, "prd/x", "FR-1", "withdrawn")
		revocation := f.add(record.Revocation, "prd/x", "", "withdraw", rel(record.Revokes, claim.ID))
		successor := f.add(record.Promise, "prd/x", "FR-1", "stale successor", rel(record.Supersedes, claim.ID))
		p := f.projection()
		if p.Status(claim.ID) != Revoked || p.Status(successor.ID) != Active || len(p.Violations) != 1 || p.Violations[0].Code != "supersedes-revoked-record" || p.Violations[0].Record != successor.ID || strings.Join(p.Violations[0].Related, ",") != revocation.ID {
			t.Fatalf("revocation/supersession order: statuses=%s/%s violations=%+v", p.Status(claim.ID), p.Status(successor.ID), p.Violations)
		}
	})

	t.Run("forward revocation is ignored and cannot form a cycle", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		forward := f.add(record.Revocation, "prd/x", "", "invalid forward act", rel(record.Revokes, "Revocation-01ARZ3NDEKTSV4RRFFQ69G5FAV"))
		later := f.add(record.Revocation, "prd/x", "", "later act", rel(record.Revokes, forward.ID))
		forward.Relations = []record.Relation{rel(record.Revokes, later.ID)}
		b, err := record.Marshal(forward)
		if err != nil {
			t.Fatal(err)
		}
		f.files[forward.Path()] = b
		p := f.projection()
		if p.Status(later.ID) != Active || p.Status(forward.ID) != Revoked || len(p.Violations) != 0 {
			t.Fatalf("later valid revocation repairs forward act: statuses=%s/%s violations=%+v", p.Status(forward.ID), p.Status(later.ID), p.Violations)
		}
	})

	t.Run("dangling target gets no status", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		missing := "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV"
		f.add(record.Revocation, "prd/x", "", "missing", rel(record.Revokes, missing))
		set, err := store.Load(context.Background(), f.files)
		if err != nil {
			t.Fatal(err)
		}
		p := New(set)
		if len(set.Dangling) != 1 || p.Status(missing) != "" {
			t.Fatalf("dangling=%+v status=%q", set.Dangling, p.Status(missing))
		}
	})
}

func TestRevocationCanReduceExistingConflictsOneMemberAtATime(t *testing.T) {
	t.Run("family conflict", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		first := f.add(record.Promise, "prd/x", "FR-1", "one")
		f.add(record.Promise, "prd/x", "FR-1", "two")
		f.add(record.Promise, "prd/x", "FR-1", "three")
		remove := f.add(record.Revocation, "prd/x", "", "remove one", rel(record.Revokes, first.ID))
		p := f.projection()
		if p.Status(remove.ID) != Active || p.Status(first.ID) != Revoked || len(p.Conflicts()) != 1 || len(p.Conflicts()[0].IDs) != 2 || len(p.Violations) != 0 {
			t.Fatalf("partial family repair: statuses=%s/%s conflicts=%+v violations=%+v", p.Status(remove.ID), p.Status(first.ID), p.Conflicts(), p.Violations)
		}
	})

	t.Run("answer conflict", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		question := f.add(record.Question, "prd/x", "", "which?")
		first := f.add(record.Answer, "prd/x", "", "one", rel(record.Answers, question.ID))
		f.add(record.Answer, "prd/x", "", "two", rel(record.Answers, question.ID))
		f.add(record.Answer, "prd/x", "", "three", rel(record.Answers, question.ID))
		remove := f.add(record.Revocation, "prd/x", "", "remove one", rel(record.Revokes, first.ID))
		p := f.projection()
		if p.Status(remove.ID) != Active || p.Status(first.ID) != Revoked || len(p.AnswerConflicts()) != 1 || len(p.AnswerConflicts()[0].IDs) != 2 || len(p.Violations) != 0 {
			t.Fatalf("partial answer repair: statuses=%s/%s conflicts=%+v violations=%+v", p.Status(remove.ID), p.Status(first.ID), p.AnswerConflicts(), p.Violations)
		}
	})

	t.Run("revocation conflict", func(t *testing.T) {
		f := &fixture{t: t, files: store.MapSource{}}
		target := f.add(record.Promise, "prd/x", "FR-1", "withdrawn")
		first := f.add(record.Revocation, "prd/x", "", "one", rel(record.Revokes, target.ID))
		f.add(record.Revocation, "prd/x", "", "two", rel(record.Revokes, target.ID))
		f.add(record.Revocation, "prd/x", "", "three", rel(record.Revokes, target.ID))
		remove := f.add(record.Revocation, "prd/x", "", "remove one", rel(record.Revokes, first.ID))
		p := f.projection()
		if p.Status(remove.ID) != Active || p.Status(first.ID) != Revoked || len(p.RevocationConflicts()) != 1 || len(p.RevocationConflicts()[0].IDs) != 2 || len(p.Violations) != 0 {
			t.Fatalf("partial revocation repair: statuses=%s/%s conflicts=%+v violations=%+v", p.Status(remove.ID), p.Status(first.ID), p.RevocationConflicts(), p.Violations)
		}
	})
}

func TestSupersessionMustStayOnSubjectAndPointBackwards(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}}
	fr1 := f.add(record.Promise, "prd/demo", "FR-1", "the demo requirement")
	stranger := f.add(record.Promise, "prd/other", "FR-99", "retires a stranger", rel(record.Supersedes, fr1.ID))
	p := f.projection()
	if p.Status(fr1.ID) != Active || len(p.Violations) != 1 || p.Violations[0].Code != "supersedes-across-subjects" || p.Violations[0].Record != stranger.ID {
		t.Fatalf("cross-subject supersession must not retire: %v %+v", p.Status(fr1.ID), p.Violations)
	}
	if h := p.Head("prd/demo", "FR-1"); h == nil || h.ID != fr1.ID {
		t.Fatal("the original requirement keeps its place in its PRD")
	}
	// a two-record cycle: the edge pointing forward in time is the invalid one, so truth survives
	g := &fixture{t: t, files: store.MapSource{}}
	older := g.add(record.Promise, "prd/x", "FR-1", "older")
	newer := g.add(record.Promise, "prd/x", "FR-1", "newer", rel(record.Supersedes, older.ID))
	older.Relations = []record.Relation{rel(record.Supersedes, newer.ID)}
	b, _ := record.Marshal(older)
	g.files[older.Path()] = b
	p = g.projection()
	if p.Status(newer.ID) != Active || p.Status(older.ID) != Superseded || len(p.Violations) != 1 || p.Violations[0].Code != "supersedes-out-of-order" {
		t.Fatalf("cycle: newer %v older %v violations %+v", p.Status(newer.ID), p.Status(older.ID), p.Violations)
	}
	if c := p.Conflicts(); len(c) != 0 {
		t.Fatalf("no conflict remains once the invalid edge is ignored: %+v", c)
	}
	if SupersedesViolation(newer, nil) != nil {
		t.Fatal("a missing target is dangling, not a violation")
	}
	// a family rename on the same subject is a plain supersession
	h := &fixture{t: t, files: store.MapSource{}}
	fr6 := h.add(record.Promise, "prd/x", "FR-6", "six")
	fr21 := h.add(record.Promise, "prd/x", "FR-21", "six, renumbered", rel(record.Supersedes, fr6.ID))
	p = h.projection()
	if p.Status(fr6.ID) != Superseded || p.Status(fr21.ID) != Active || len(p.Violations) != 0 {
		t.Fatalf("rename: %v %v %+v", p.Status(fr6.ID), p.Status(fr21.ID), p.Violations)
	}
}

func TestDocumentSupersessionKeepsItsInterpretationMode(t *testing.T) {
	older := &record.Record{ID: "Document-01K47QTR40YFHX57HPYYGK9K5Q", Kind: record.Document, Subject: "prd/x"}
	newer := &record.Record{ID: "Document-01K47QTR41YFHX57HPYYGK9K5R", Kind: record.Document, Subject: "prd/x", LiteralBody: true}
	v := SupersedesViolation(newer, older)
	if v == nil || v.Code != "document-mode-changed" || !strings.Contains(v.What, "literal_body=false to literal_body=true") {
		t.Fatalf("native to literal: %+v", v)
	}
	older.LiteralBody, newer.LiteralBody = true, false
	v = SupersedesViolation(newer, older)
	if v == nil || v.Code != "document-mode-changed" || !strings.Contains(v.What, "literal_body=true to literal_body=false") {
		t.Fatalf("literal to native: %+v", v)
	}
	newer.LiteralBody = true
	if v := SupersedesViolation(newer, older); v != nil {
		t.Fatalf("stable literal mode rejected: %+v", v)
	}
}
