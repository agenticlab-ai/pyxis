package reader

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

func TestCoverage90EmptySpendAndDeterministicModelSort(t *testing.T) {
	empty := record.Record{PhaseID: "empty"}
	if got := markUnpriced(empty); got.Spend != nil {
		t.Fatalf("markUnpriced(empty) spend = %#v", got.Spend)
	}
	view := toView(record.Record{Spend: map[string]record.ModelSpend{
		"z-model": {},
		"a-model": {},
	}}, record.Efficiency{})
	if len(view.Models) != 2 || view.Models[0].Model != "a-model" {
		t.Fatalf("sorted models = %+v", view.Models)
	}
}
