package techdesigns_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

// A design whose body carries concrete shape (a sql/dbml block or a mermaid
// erDiagram) fails with design-carries-shape; a design that only references
// its shape — prose, a link, an inline mention, a flowchart — passes clean.
func TestCarriesShape(t *testing.T) {
	carrying := []struct {
		name, body, wantForm string
	}{
		{"sql", goodBody + "\n### 3.10 The shape\n\n```sql\nCREATE TABLE run (id TEXT PRIMARY KEY);\n```\n", "```sql"},
		{"dbml", goodBody + "\n### 3.10 The shape\n\n```dbml\nTable run { id text [pk] }\n```\n", "```dbml"},
		{"er-body", goodBody + "\n### 3.10 The shape\n\n```mermaid\nerDiagram\n  RUN ||--o{ TASK : has\n```\n", "erDiagram"},
		{"er-fence-line", goodBody + "\n### 3.10 The shape\n\n```mermaid erDiagram\n  RUN ||--o{ TASK : has\n```\n", "erDiagram"},
		{"sql-upper", goodBody + "\n### 3.10 The shape\n\n```SQL\nCREATE TABLE t (id int);\n```\n", "```sql"},
	}
	for _, tc := range carrying {
		t.Run("carrying/"+tc.name, func(t *testing.T) {
			d := mk(t, "carry-"+tc.name, goodHeader, tc.body)
			defects := techdesigns.CarriesShape([]*techdesigns.Design{d})
			if len(defects) == 0 {
				t.Fatalf("concrete shape not flagged")
			}
			got := defects[0]
			if got.Verb != "design-carries-shape" {
				t.Errorf("verb = %q, want design-carries-shape", got.Verb)
			}
			if got.File != "carry-"+tc.name+".md" {
				t.Errorf("file = %q, want the design file", got.File)
			}
			if !strings.Contains(got.Reason, tc.wantForm) {
				t.Errorf("reason %q does not name the form %q", got.Reason, tc.wantForm)
			}
			if got.Remedy == "" {
				t.Errorf("defect carries no remedy")
			}
		})
	}

	referencing := []struct {
		name, body string
	}{
		{"prose", goodBody + "\n### 3.10 The shape\n\nThe concrete schema lives in the paired solution-shape `carry-shape`.\n"},
		{"link", goodBody + "\n### 3.10 The shape\n\nSee [the shape](../solution-shapes/foo.md) for the schema.\n"},
		{"inline-mention", goodBody + "\n### 3.10 The shape\n\nThe body is a ` ```sql ` or ` ```dbml ` block in the shape, never here.\n"},
		{"flowchart", goodBody + "\n### 3.10 A flow\n\n```mermaid\ngraph LR\n  A --> B\n```\n"},
		{"yaml", goodBody + "\n### 3.10 Config\n\n```yaml\nkey: value\n```\n"},
	}
	for _, tc := range referencing {
		t.Run("referencing/"+tc.name, func(t *testing.T) {
			d := mk(t, "ref-"+tc.name, goodHeader, tc.body)
			if defects := techdesigns.CarriesShape([]*techdesigns.Design{d}); len(defects) != 0 {
				t.Fatalf("reference-only design flagged: %v", defects)
			}
		})
	}
}
