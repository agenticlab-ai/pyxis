package reconcile

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestSourceDecisionProposalRejectsInventedMeaningAndAssociations(t *testing.T) {
	t.Parallel()
	snapshot, _, file, document, _, block := designMemoryFixture(t)
	document.LiteralBody = true
	snapshot.Code = nil
	draft, ok := designDecisionDraft(snapshot, block, document)
	if !ok {
		t.Fatal("native source-only projection unavailable")
	}
	r := &record.Record{ID: "Decision-fixture", Kind: draft.Kind, Subject: draft.Subject, Family: draft.Family, Statement: draft.Statement, Tags: draft.Tags, Relations: draft.Relations, Provenance: draft.Provenance}
	if err := ValidateSourceDecisionProposal(r, document, file.Body); err != nil {
		t.Fatal("native exact source-only projection refused", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*record.Record)
	}{
		{"invented statement", func(r *record.Record) { r.Statement = "Invented meaning." }},
		{"invented rationale", func(r *record.Record) { r.Rationale = "Authored semantic claim." }},
		{"unknown block", func(r *record.Record) { r.Family = "decision.9.1" }},
		{"other subject", func(r *record.Record) { r.Subject = "design/other-v1" }},
		{"code association", func(r *record.Record) { r.Tags = append(append([]string(nil), r.Tags...), "code:internal/widget") }},
		{"different source note", func(r *record.Record) { r.Provenance.Note = "Not an exact native capture." }},
		{"different dependency", func(r *record.Record) {
			r.Relations = []record.Relation{{Type: record.DependsOn, Target: "Document-other"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := *r
			tc.mutate(&changed)
			if err := ValidateSourceDecisionProposal(&changed, document, file.Body); err == nil {
				t.Fatal("non-native companion admitted")
			}
		})
	}
	if err := ValidateSourceDecisionProposal(r, document, []byte("different captured bytes")); err == nil {
		t.Fatal("corrupt captured body admitted")
	}
	if err := ValidateSourceDecisionProposal(r, nil, file.Body); err == nil {
		t.Fatal("missing selected source admitted")
	}
	if err := ValidateSourceDecisionProposal(nil, document, file.Body); err == nil {
		t.Fatal("missing companion admitted")
	}
	other := *document
	other.Subject = "problem/widget-v1"
	if err := ValidateSourceDecisionProposal(r, &other, file.Body); err == nil {
		t.Fatal("journal treated as a design")
	}
}
