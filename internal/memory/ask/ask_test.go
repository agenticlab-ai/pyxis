package ask

import (
	"context"
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

func (tr *tree) add(kind, subject, statement string, rels ...record.Relation) *record.Record {
	tr.n++
	var e [10]byte
	e[9] = tr.n
	r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, t0.Add(time.Duration(tr.n)*time.Second), e), Kind: kind, Subject: subject,
		Statement: statement, Issuer: "m@example.com", CreatedAt: t0.Format(time.RFC3339), Relations: rels}
	b, err := record.Marshal(r)
	if err != nil {
		tr.t.Fatal(err)
	}
	tr.files[r.Path()] = b
	return r
}

func TestFind(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	q1 := tr.add(record.Question, "prd/x", "Should the reviewer share the operator's parent clone or get an isolated workspace?")
	a1 := tr.add(record.Answer, "prd/x", "isolated workspace, always", record.Relation{Type: record.Answers, Target: q1.ID})
	q2 := tr.add(record.Question, "prd/x", "Should the reviewer share the operator's parent clone?")
	q3 := tr.add(record.Question, "prd/y", "Which colour should the badge be?")
	s, err := store.Load(context.Background(), tr.files)
	if err != nil {
		t.Fatal(err)
	}
	p := resolve.New(s)

	m := Find(p, "prd/x", "Does the reviewer share the operator's parent clone or use an isolated workspace?", 0.6)
	if m == nil || m.Question.ID != q1.ID || m.Answer == nil || m.Answer.ID != a1.ID {
		t.Fatalf("expected the answered question: %+v", m)
	}
	if m2 := Find(p, "prd/x", "Should the reviewer share the parent clone?", 0.6); m2 == nil || m2.Question.ID != q2.ID || m2.Answer != nil {
		t.Fatalf("expected the closer open question: %+v", m2)
	}
	if Find(p, "prd/x", "Which colour should the badge be?", 0.6) != nil {
		t.Fatal("subject scoping")
	}
	if m3 := Find(p, "", "Which colour should the badge be?", 0.6); m3 == nil || m3.Question.ID != q3.ID {
		t.Fatalf("all subjects: %+v", m3)
	}
	if Find(p, "prd/x", "How many retries?", 0.6) != nil || Find(p, "prd/x", "", 0.6) != nil || Find(p, "prd/x", "the a an", 0.6) != nil {
		t.Fatal("no match")
	}
	if jaccard(map[string]bool{}, map[string]bool{"a": true}) != 0 {
		t.Fatal("jaccard of empty")
	}
	// at equal similarity an answered question wins
	tr2 := &tree{t: t, files: store.MapSource{}}
	open := tr2.add(record.Question, "prd/x", "retry budget per task?")
	answered := tr2.add(record.Question, "prd/x", "retry budget per task?")
	tr2.add(record.Answer, "prd/x", "three", record.Relation{Type: record.Answers, Target: answered.ID})
	s2, _ := store.Load(context.Background(), tr2.files)
	if m := Find(resolve.New(s2), "prd/x", "retry budget per task?", 0.9); m == nil || m.Question.ID != answered.ID {
		t.Fatalf("answered beats open (%s): %+v", open.ID, m)
	}
}
