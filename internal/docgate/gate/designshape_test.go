package gate_test

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// solution-shape T3 (DINV-1): the gate verb flags a design whose body carries
// concrete shape and passes one that only references it — over the real
// on-disk corpus path, so the load → parse → check wiring is exercised.
func TestCheckDesignCarriesShape(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()

	const header = "| Field | Value |\n|---|---|\n| **Status** | Approved |\n"

	// A design carrying a concrete schema block → design-carries-shape.
	write("docs/tech-design/carrier.md",
		"# Carrier\n\n"+header+"\n## 3. The design\n\n```sql\nCREATE TABLE run (id TEXT PRIMARY KEY);\n```\n")
	defects := gate.CheckDesignCarriesShape(root, cfg)
	if len(defects) == 0 {
		t.Fatalf("design carrying a sql schema not flagged")
	}
	if defects[0].Verb != "design-carries-shape" || defects[0].File != "carrier.md" {
		t.Fatalf("unexpected defect: %+v", defects[0])
	}
	if defects[0].Remedy == "" {
		t.Fatalf("defect carries no remedy")
	}

	// Replace the block with a reference → clean.
	write("docs/tech-design/carrier.md",
		"# Carrier\n\n"+header+"\n## 3. The design\n\nThe schema lives in the paired solution-shape; see it there.\n")
	if d := gate.CheckDesignCarriesShape(root, cfg); len(d) != 0 {
		t.Fatalf("reference-only design flagged: %v", d)
	}
}
