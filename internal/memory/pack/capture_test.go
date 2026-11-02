package pack

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestPackOmitsDerivedRecordsWithoutHidingTheirConsumers(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	anchor := tr.add(record.Need, "code/widget", "anchor", "Production code unit internal/widget exists.")
	anchor.Tags = []string{"reconcile:anchor"}
	learning := tr.add(record.Learning, "learning/widget", "", "Package widget implements widgets.")
	learning.Intent = "Preserve the implemented responsibility of internal/widget in durable software memory."
	learning.Rationale = "Source-code audit found production code without a dedicated semantic memory record."
	learning.Tags = []string{"reconcile:code-semantics"}
	for _, r := range []*record.Record{anchor, learning} {
		b, err := record.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		tr.files[r.Path()] = b
	}
	d := tr.add(record.Decision, "design/widget", "D-1", "Use bounded queues because consumers can stall.", rel(record.DependsOn, anchor.ID))
	l := tr.add(record.Learning, "learning/widget", "", "A slow consumer exhausted memory; bound its queue.")
	pk, err := Compile(tr.projection(), nil, Scope{Subjects: []string{"code/widget", "design/widget", "learning/widget"}}, 8000, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{anchor.ID, learning.ID} {
		if strings.Contains(pk.Render(), hidden) {
			t.Fatalf("derived record consumed task context: %s", hidden)
		}
	}
	for _, visible := range []string{d.ID, l.ID} {
		if !strings.Contains(pk.Render(), visible) {
			t.Fatalf("useful knowledge was hidden: %s", visible)
		}
	}
}
