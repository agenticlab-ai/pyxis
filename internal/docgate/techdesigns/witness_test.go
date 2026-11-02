package techdesigns_test

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/shapes"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

func TestShapeConsistencyRequiresWitnessAndTrace(t *testing.T) {
	design := &techdesigns.Design{
		Slug: "widget",
		Doc: &ledger.Doc{Lines: []string{
			"**DINV-1** — widget state remains durable.",
			"The state lives in shape:widget.",
			"```markdown",
			"**DINV-2** and shape:missing appear only in an example.",
			"```",
			"See ../solution-shapes/widget.md for the same shape.",
		}},
	}
	paired := &shapes.Shape{
		Slug:   "widget",
		Design: "widget",
		Body:   "DINV-1 stores widget state\n\n```sql\nCREATE TABLE widget (id TEXT);\n```\n",
	}
	if got := techdesigns.ShapeConsistency([]*techdesigns.Design{nil, design}, []*shapes.Shape{nil, paired}, "docs/solution-shapes"); len(got) != 0 {
		t.Fatalf("paired and traced shape refused: %v", got)
	}

	missing := techdesigns.ShapeConsistency([]*techdesigns.Design{design}, nil, "docs/solution-shapes")
	if len(missing) != 1 || missing[0].Verb != "shape-incomplete" {
		t.Fatalf("missing shape defects = %v; want shape-incomplete", missing)
	}

	paired.Body = "The fence body mentions DINV-1, but its caption does not.\n\nUntraced element\n```sql\nCREATE TABLE widget (id TEXT);\n```\n"
	untraced := techdesigns.ShapeConsistency([]*techdesigns.Design{design}, []*shapes.Shape{paired}, "docs/solution-shapes")
	if len(untraced) != 2 || untraced[0].Verb != "unwitnessed-invariant" || untraced[1].Verb != "untraced-shape-element" {
		t.Fatalf("untraced shape defects = %v; want witness and trace refusals", untraced)
	}
}
