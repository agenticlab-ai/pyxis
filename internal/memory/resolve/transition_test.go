package resolve

import (
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestHistoricalTransitionRefusesMissingInputs(t *testing.T) {
	p := New(&store.Set{})
	for _, pair := range [][2]*Projection{{nil, p}, {p, nil}, {&Projection{}, p}, {p, &Projection{}}} {
		got := HistoricalTransitionViolations(pair[0], pair[1])
		if len(got) != 1 || got[0].Code != "historical-input-missing" {
			t.Fatalf("missing input: %+v", got)
		}
	}
}

func TestHistoricalTransitionUsesHonoredEdgesAndStableOrdering(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}, n: 20}
	target := fixtureID(record.Question, 1)
	first := f.add(record.Answer, "prd/x", "", "first dangling answer", rel(record.Answers, target))
	second := f.add(record.Answer, "prd/x", "", "second dangling answer", rel(record.Answers, target))
	before := f.projection()
	f.n = 0
	f.add(record.Question, "prd/x", "", "target")
	after := f.projection()
	got := HistoricalTransitionViolations(before, after)
	if len(got) != 2 || got[0].Record != first.ID || got[1].Record != second.ID || len(after.AnswerConflicts()) != 1 {
		t.Fatalf("historical activation beside another defect: %+v", got)
	}
	for range 5 {
		if next := HistoricalTransitionViolations(before, f.projection()); !reflect.DeepEqual(got, next) {
			t.Fatal("map iteration changed transition findings")
		}
	}
	if next := HistoricalTransitionViolations(after, after); len(next) != 0 {
		t.Fatal("identical history changed meaning")
	}
}

func TestHistoricalTransitionDoesNotMistakeAnotherRejectionForActivation(t *testing.T) {
	f := &fixture{t: t, files: store.MapSource{}, n: 20}
	f.add(record.Answer, "prd/x", "", "dangling answer", rel(record.Answers, fixtureID(record.Question, 1)))
	before := f.projection()
	f.n = 0
	f.add(record.Question, "prd/y", "", "wrong subject target")
	after := f.projection()
	if len(after.Violations) != 1 || after.Violations[0].Code != "answers-across-subjects" {
		t.Fatalf("fixture did not change the rejection reason: %+v", after.Violations)
	}
	if got := HistoricalTransitionViolations(before, after); len(got) != 0 {
		t.Fatalf("still-rejected edge reported as effective: %+v", got)
	}
}
