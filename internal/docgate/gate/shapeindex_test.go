package gate_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// #135 (DINV-3): a solution-shape reaching the technical-design corpus is a
// defect — there gen-index would admit it into the design index. The verb
// flags a misfiled shape and passes a real design, over the on-disk corpus.
func TestCheckShapeIndexedAsDesign(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()

	// A real design in the corpus → no false positive.
	write("docs/tech-design/real-design.md",
		"# Real\n\n| Field | Value |\n|---|---|\n| **Status** | Approved |\n\n## 2. Decision\n\n**Decision:** x.\n")
	if d := gate.CheckShapeIndexedAsDesign(root, cfg); len(d) != 0 {
		t.Fatalf("a real design flagged as a misfiled shape: %v", d)
	}

	// A solution-shape dropped into the design corpus → shape-indexed-as-design.
	write("docs/tech-design/widget-store.md",
		"# solution-shape: widget-store\n\n| Field | Value |\n|---|---|\n"+
			"| Design | real-design |\n| Status | authoritative |\n\n"+
			"```sql\nCREATE TABLE widget (id UUID PRIMARY KEY);\n```\n")
	defects := gate.CheckShapeIndexedAsDesign(root, cfg)
	if len(defects) == 0 {
		t.Fatal("a shape sitting in the design corpus was not flagged")
	}
	got := defects[0]
	if got.Verb != "shape-indexed-as-design" || got.File != "docs/tech-design/widget-store.md" {
		t.Fatalf("unexpected defect: %+v", got)
	}
	if got.Remedy == "" {
		t.Fatal("defect carries no remedy")
	}
}

// #135 acceptance: index generation is blind to the shapes root — a shape in
// its proper home never appears in the index, and every existing design's
// index entry is byte-identical before and after the shape exists.
func TestGenIndexBlindToShapesRoot(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()

	write("docs/tech-design/real-design.md",
		"# Real\n\n| Field | Value |\n|---|---|\n| **Status** | Approved |\n\n## 2. Decision\n\n**Decision:** x.\n")

	mdBefore, jsBefore, def1 := gate.GenIndexBytes(root, cfg)
	if len(def1) != 0 {
		t.Fatalf("index defects before: %v", def1)
	}

	// Drop a shape into its proper home — distinct from & outside the design
	// corpus (config guarantees the disjointness).
	write("docs/solution-shapes/real-design.md",
		"# SHAPE-MARKER-DO-NOT-INDEX\n\n| Field | Value |\n|---|---|\n"+
			"| Design | real-design |\n| Status | authoritative |\n\n"+
			"```sql\nCREATE TABLE t (id INT);\n```\n")

	mdAfter, jsAfter, def2 := gate.GenIndexBytes(root, cfg)
	if len(def2) != 0 {
		t.Fatalf("index defects after: %v", def2)
	}
	if !bytes.Equal(mdBefore, mdAfter) || !bytes.Equal(jsBefore, jsAfter) {
		t.Fatal("the shape's presence changed the index — generation is not blind to the shapes root")
	}
	if strings.Contains(string(mdAfter), "SHAPE-MARKER") || strings.Contains(string(jsAfter), "SHAPE-MARKER") {
		t.Fatal("a solution-shape leaked into the generated technical-design index")
	}
}
