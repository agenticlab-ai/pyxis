package gate_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

const shapeAuthoritative = `# solution-shape: widget-store

| Field | Value |
|---|---|
| Design | widget-store |
| Status | authoritative |

## Schema

` + "```sql" + `
CREATE TABLE widget (id UUID PRIMARY KEY, tenant UUID NOT NULL);
` + "```" + `
`

const designWithShape = `# widget-store — technical design

| Field | Value |
|---|---|
| **Status** | Approved |

## 2. Decision

**Decision:** the widget store is keyed by tenant.

## 3. The design

### 3.3 Invariants

- **DINV-1** — every widget carries a tenant. *(FR-1)*

The schema lives in the paired solution-shape; see it there.
`

// #136 A-4 / A-5 over the real git+disk wiring: the flip is one-way, a
// shape-confined change leaves the paired design untouched, and a binding-markup
// change to the paired design is refused as a shape change.
func TestCheckShapeLifecycle(t *testing.T) {
	root, run, write := fixtureRepo(t)
	cfg := config.Defaults()

	write("docs/tech-design/widget-store.md", designWithShape)
	write("docs/solution-shapes/widget-store.md", shapeAuthoritative)
	run("add", ".")
	run("commit", "-qm", "base: design + authoritative shape")

	// A shape-confined change (re-approval) leaves the frozen design untouched.
	write("docs/solution-shapes/widget-store.md",
		shapeAuthoritative+"\n## Interfaces\n\n```go\ntype Store interface{ Get() }\n```\n")
	run("add", ".")
	run("commit", "-qm", "shape re-approval: add interfaces")
	if d := gate.CheckShapeLifecycle(root, "HEAD~1", cfg); len(d) != 0 {
		t.Fatalf("shape-confined change flagged: %v", d)
	}
	// The frozen design is genuinely untouched — check-immutable stays clean.
	if d := gate.CheckImmutable(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("shape-confined change disturbed the frozen design: %v", d)
	}

	// The legal flip authoritative → superseded-by-code passes.
	write("docs/solution-shapes/widget-store.md",
		strings.Replace(shapeAuthoritative, "authoritative", "superseded-by-code", 1))
	run("add", ".")
	run("commit", "-qm", "flip: superseded-by-code")
	if d := gate.CheckShapeLifecycle(root, "HEAD~1", cfg); len(d) != 0 {
		t.Fatalf("legal flip flagged: %v", d)
	}

	// The reverse flip is refused (one-way lifecycle).
	write("docs/solution-shapes/widget-store.md", shapeAuthoritative)
	run("add", ".")
	run("commit", "-qm", "illegal reverse flip")
	wantDefect(t, gate.CheckShapeLifecycle(root, "HEAD~1", cfg), "regressed superseded-by-code")

	// A binding-markup change to the paired design is refused as a shape change.
	write("docs/tech-design/widget-store.md",
		strings.Replace(designWithShape, "keyed by tenant", "keyed by region", 1))
	write("docs/solution-shapes/widget-store.md", shapeAuthoritative)
	run("add", ".")
	run("commit", "-qm", "smuggle a Decision change")
	wantDefect(t, gate.CheckShapeLifecycle(root, "HEAD~1", cfg), "Decision change, not a shape change")
}

func TestCheckShapeLifecycleTreatsMissingHistoryAsNewShape(t *testing.T) {
	root, _, write := fixtureRepo(t)
	write("docs/solution-shapes/widget-store.md", shapeAuthoritative)
	if defects := gate.CheckShapeLifecycle(root, "missing-ref", config.Defaults()); len(defects) != 0 {
		t.Fatalf("new shape with unavailable history was flagged: %v", defects)
	}
}
