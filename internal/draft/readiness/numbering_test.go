package readiness

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestReopenedDraftNumberingRefusesReusedHistoryAndRetiredRows(t *testing.T) {
	root := filepath.Join(t.TempDir(), store.Dir)
	at := time.Now().Add(-time.Hour).UTC()
	index := 0
	add := func(kind, subject, family string) *record.Record {
		index++
		var entropy [10]byte
		entropy[9] = byte(index)
		when := at.Add(time.Duration(index) * time.Millisecond)
		r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, when, entropy), Kind: kind,
			Subject: subject, Family: family, Statement: family + " old", Issuer: "agent:test", CreatedAt: when.Format(time.RFC3339)}
		if kind == record.Document {
			r.LiteralBody = true
			if _, err := store.WriteDocument(root, r, []byte("# historical\n")); err != nil {
				t.Fatal(err)
			}
		} else if _, err := store.Write(root, r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	add(record.Document, "brainstorm/widget", "record-002")
	add(record.Document, "review/widget", "report-003")
	add(record.Question, "prd/widget", "P-5")
	add(record.Promise, "prd/widget", "FR-1")
	retiredRequirement := add(record.Promise, "prd/widget", "FR-9")
	retiredDecision := add(record.Decision, "design/widget", "D-4")
	for _, old := range []*record.Record{retiredRequirement, retiredDecision} {
		index++
		var entropy [10]byte
		entropy[9] = byte(index)
		when := at.Add(time.Duration(index) * time.Millisecond)
		revoke := &record.Record{Schema: record.Schema, ID: record.NewID(record.Revocation, when, entropy), Kind: record.Revocation,
			Subject: old.Subject, Statement: "Retire " + old.Family, Issuer: "agent:test", CreatedAt: when.Format(time.RFC3339),
			Relations: []record.Relation{{Type: record.Revokes, Target: old.ID}}}
		if _, err := store.Write(root, revoke); err != nil {
			t.Fatal(err)
		}
	}
	set, err := store.Load(t.Context(), store.DirSource{Root: root})
	if err != nil || len(set.Problems) > 0 {
		t.Fatalf("binding memory: %v %+v", err, set.Problems)
	}
	binding := resolve.New(set)
	result := compile.Result{Feature: "widget", Problems: []compile.Problem{{ID: "P-5"}}, Records: []compile.Record{
		{Draft: reconcile.Draft{Kind: record.Document, Subject: "brainstorm/widget", Family: "record-002"}},
		{Draft: reconcile.Draft{Kind: record.Document, Subject: "review/widget", Family: "report-003"}},
		{Draft: reconcile.Draft{Kind: record.Promise, Subject: "prd/widget", Family: "FR-1"}},
		{Draft: reconcile.Draft{Kind: record.Promise, Subject: "prd/widget", Family: "FR-9"}},
		{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-4"}},
	}}
	unmet := checkNumbering(result, binding)
	if len(unmet) != 5 {
		t.Fatalf("reused numbers and IDs: %+v", unmet)
	}
	for _, want := range []string{"brainstorm/002.md", "brainstorm/003.md", "reviews/003.md", "reviews/004.md", "problems.md#P-5", "P-6", "prd/widget#FR-9", "design/widget#D-4"} {
		found := false
		for _, item := range unmet {
			found = found || strings.Contains(item.Item, want) || strings.Contains(item.Remedy, want)
		}
		if !found {
			t.Fatalf("numbering refusal lacks %q: %+v", want, unmet)
		}
	}
	if extra := checkNumbering(result, nil); len(extra) != 0 {
		t.Fatalf("first version has no binding history: %+v", extra)
	}
}
