package gate_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// solution-shape T4 (DINV-4, §3.8): the witness / trace / completeness
// cross-check between a design's binding markup and its paired shape. Each
// case is exercised over the real on-disk corpus paths, so the load → parse
// → check wiring is what runs — the same discipline as the T3 test.

const shapeDesignHeader = "| Field | Value |\n|---|---|\n| **Status** | Approved |\n"

// has reports whether some defect carries the given verb.
func has(defects []schema.Defect, verb string) bool {
	for _, d := range defects {
		if d.Verb == verb {
			return true
		}
	}
	return false
}

func TestCheckShapeConsistency_UnwitnessedInvariant(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()

	// A design declaring DINV-1 and DINV-2, referencing its shape; the shape
	// witnesses DINV-1 only → DINV-2 is unwitnessed.
	write("docs/tech-design/store.md",
		"# Store\n\n"+shapeDesignHeader+
			"\n## 3. The design\n\nConcrete schema lives in docs/solution-shapes/store.md.\n\n"+
			"- **DINV-1** — the run row is unique by id.\n"+
			"- **DINV-2** — every store call carries a context.\n")
	write("docs/solution-shapes/store.md",
		"# Store shape\n\n| Field | Value |\n|---|---|\n| Design | store |\n| Status | authoritative |\n\n"+
			"Witnesses DINV-1:\n```sql\nCREATE TABLE run (id TEXT PRIMARY KEY);\n```\n")

	defects := gate.CheckShapeConsistency(root, cfg)
	if !has(defects, "unwitnessed-invariant") {
		t.Fatalf("DINV-2 with no witnessing element not flagged: %v", defects)
	}
	for _, d := range defects {
		if d.Verb == "unwitnessed-invariant" {
			if !strings.Contains(d.Reason, "DINV-2") || d.File != "store.md" || d.Remedy == "" {
				t.Fatalf("unexpected unwitnessed defect: %+v", d)
			}
		}
	}
	// DINV-1 is witnessed, so it must NOT be reported.
	for _, d := range defects {
		if d.Verb == "unwitnessed-invariant" && strings.Contains(d.Reason, "DINV-1 ") {
			t.Fatalf("witnessed DINV-1 wrongly flagged: %+v", d)
		}
	}
}

func TestCheckShapeConsistency_UntracedElement(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()

	// The design declares DINV-1; the shape has two elements — one captioned
	// with DINV-1, one with no citation at all → the latter is untraced.
	write("docs/tech-design/store.md",
		"# Store\n\n"+shapeDesignHeader+
			"\n## 3. The design\n\nSee docs/solution-shapes/store.md.\n\n"+
			"- **DINV-1** — the run row is unique by id.\n")
	write("docs/solution-shapes/store.md",
		"# Store shape\n\n| Field | Value |\n|---|---|\n| Design | store |\n| Status | authoritative |\n\n"+
			"Witnesses DINV-1:\n```sql\nCREATE TABLE run (id TEXT PRIMARY KEY);\n```\n\n"+
			"An interface with no cited decision:\n```go\ntype Store interface { Get(id string) }\n```\n")

	defects := gate.CheckShapeConsistency(root, cfg)
	if !has(defects, "untraced-shape-element") {
		t.Fatalf("shape element citing no decision not flagged: %v", defects)
	}
	for _, d := range defects {
		if d.Verb == "untraced-shape-element" {
			if d.File != "store.md" || d.Remedy == "" {
				t.Fatalf("untraced defect on wrong file / no remedy: %+v", d)
			}
		}
	}
}

func TestCheckShapeConsistency_ShapeIncomplete(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()

	// A stateful design references its shape, but no shape file exists.
	write("docs/tech-design/store.md",
		"# Store\n\n"+shapeDesignHeader+
			"\n## 3. The design\n\nThe concrete schema lives in the paired shape:\n"+
			"docs/solution-shapes/store.md.\n\n- **DINV-1** — the run row is unique.\n")

	defects := gate.CheckShapeConsistency(root, cfg)
	if !has(defects, "shape-incomplete") {
		t.Fatalf("stateful design with no paired shape not flagged: %v", defects)
	}
	for _, d := range defects {
		if d.Verb == "shape-incomplete" {
			if d.File != "store.md" || !strings.Contains(d.Reason, "store") || d.Remedy == "" {
				t.Fatalf("unexpected shape-incomplete defect: %+v", d)
			}
		}
	}
}

func TestCheckShapeConsistency_PureDecisionPasses(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()

	// A pure-decision design: it decides, references no shape, creates no
	// state — it needs no shape and passes clean.
	write("docs/tech-design/policy.md",
		"# Policy\n\n"+shapeDesignHeader+
			"\n## 2. Decision\n\n**Decision:** merges require one human approval.\n\n"+
			"## 3. The design\n\nNo state, no interface — a pure ruling.\n")

	if d := gate.CheckShapeConsistency(root, cfg); len(d) != 0 {
		t.Fatalf("pure-decision design with no state flagged: %v", d)
	}
}

func TestCheckShapeConsistency_CleanPairPasses(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()

	// A design that references its shape, declares one invariant witnessed by
	// a traced element — the fully consistent pair passes clean.
	write("docs/tech-design/store.md",
		"# Store\n\n"+shapeDesignHeader+
			"\n## 3. The design\n\nSee docs/solution-shapes/store.md.\n\n"+
			"- **DINV-1** — the run row is unique by id.\n")
	write("docs/solution-shapes/store.md",
		"# Store shape\n\n| Field | Value |\n|---|---|\n| Design | store |\n| Status | authoritative |\n\n"+
			"Witnesses DINV-1:\n```sql\nCREATE TABLE run (id TEXT PRIMARY KEY);\n```\n")

	if d := gate.CheckShapeConsistency(root, cfg); len(d) != 0 {
		t.Fatalf("clean design+shape pair flagged: %v", d)
	}
}

// The real repository corpus (no shapes authored yet, no design referencing
// one) must stay clean under the new check — a guard against the gate going
// red on merge.
func TestCheckShapeConsistency_NoFalsePositivesOnClean(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()
	// A design with invariants but NO shape reference is not stateful — it
	// pairs no shape and must not be flagged (mirrors the current corpus).
	write("docs/tech-design/conventions.md",
		"# Conventions\n\n"+shapeDesignHeader+
			"\n## 3. The design\n\n- **DINV-1** — designs are immutable once merged.\n")
	if d := gate.CheckShapeConsistency(root, cfg); len(d) != 0 {
		t.Fatalf("non-stateful design (no shape reference) flagged: %v", d)
	}
}
