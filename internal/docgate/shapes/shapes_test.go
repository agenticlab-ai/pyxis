package shapes_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/shapes"
)

const wellFormed = `# solution-shape: widget-store

| Field | Value |
|---|---|
| Design | widget-store |
| Status | authoritative |

## Schema

` + "```sql" + `
CREATE TABLE widget (id UUID PRIMARY KEY, tenant UUID NOT NULL);
` + "```" + `

## Interfaces

` + "```go" + `
type Store interface { Get(ctx context.Context, id string) (Widget, error) }
` + "```" + `
`

// A1: a well-formed shape parses to its typed value with slug, status, and
// body populated.
func TestParseWellFormed(t *testing.T) {
	s, defects := shapes.Parse("widget-store", []byte(wellFormed))
	if len(defects) != 0 {
		t.Fatalf("well-formed shape flagged: %v", defects)
	}
	if s == nil {
		t.Fatal("well-formed shape parsed to nil value")
	}
	if s.Slug != "widget-store" || s.Design != "widget-store" || s.Status != shapes.StatusAuthoritative {
		t.Fatalf("typed value wrong: %+v", s)
	}
	if !strings.Contains(s.Body, "CREATE TABLE widget") || !strings.Contains(s.Body, "type Store interface") {
		t.Fatalf("body not populated with the concrete content: %q", s.Body)
	}
}

func TestParseSupersededByCode(t *testing.T) {
	doc := strings.Replace(wellFormed, "authoritative", "superseded-by-code", 1)
	s, defects := shapes.Parse("widget-store", []byte(doc))
	if len(defects) != 0 || s == nil {
		t.Fatalf("valid superseded-by-code shape flagged: %v", defects)
	}
	if s.Status != shapes.StatusSupersededByCode {
		t.Fatalf("status wrong: %q", s.Status)
	}
}

// A2: a shape with an unknown status yields the named defect and no value.
func TestParseUnknownStatus(t *testing.T) {
	doc := strings.Replace(wellFormed, "authoritative", "provisional", 1)
	s, defects := shapes.Parse("widget-store", []byte(doc))
	if s != nil {
		t.Fatalf("unknown status still produced a value: %+v", s)
	}
	if !hasDefect(defects, "status", "unknown status") {
		t.Fatalf("expected named unknown-status defect, got: %v", defects)
	}
}

// Malformed-shape defects: missing/unknown header field, non-native body.
func TestParseMalformed(t *testing.T) {
	cases := map[string]struct{ doc, op, reason string }{
		"no-h1": {
			doc:    "no title here\n\n| Field | Value |\n|---|---|\n| Design | x |\n| Status | authoritative |\n",
			op:     "parse",
			reason: "H1",
		},
		"no-header-table": {
			doc:    "# t\n\njust prose, no table\n",
			op:     "header",
			reason: "no header table",
		},
		"missing-field": {
			doc:    "# t\n\n| Field | Value |\n|---|---|\n| Design | x |\n\n```sql\nx\n```\n",
			op:     "header",
			reason: `missing header field "Status"`,
		},
		"unknown-field": {
			doc:    "# t\n\n| Field | Value |\n|---|---|\n| Design | x |\n| Status | authoritative |\n| Owner | y |\n\n```sql\nx\n```\n",
			op:     "header",
			reason: `unknown header field "Owner"`,
		},
		"prose-body": {
			doc:    "# t\n\n| Field | Value |\n|---|---|\n| Design | x |\n| Status | authoritative |\n\njust prose, no schema\n",
			op:     "body",
			reason: "native-form",
		},
		"empty-body": {
			doc:    "# t\n\n| Field | Value |\n|---|---|\n| Design | x |\n| Status | authoritative |\n",
			op:     "body",
			reason: "empty body",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s, defects := shapes.Parse("x", []byte(tc.doc))
			if s != nil {
				t.Fatalf("malformed shape produced a value: %+v", s)
			}
			if !hasDefect(defects, tc.op, tc.reason) {
				t.Fatalf("want %s/%q defect, got: %v", tc.op, tc.reason, defects)
			}
		})
	}
}

// A3 (pairing): a shape whose paired design is absent yields the named defect;
// a present design resolves cleanly.
func TestResolvePairing(t *testing.T) {
	s, defects := shapes.Parse("widget-store", []byte(wellFormed))
	if s == nil {
		t.Fatalf("setup parse failed: %v", defects)
	}

	present := func(slug string) bool { return slug == "widget-store" }
	if d := shapes.ResolvePairing(s, present); len(d) != 0 {
		t.Fatalf("paired shape flagged: %v", d)
	}

	absent := func(string) bool { return false }
	d := shapes.ResolvePairing(s, absent)
	if !hasDefect(d, "pairing", "paired design") {
		t.Fatalf("expected named unpaired defect, got: %v", d)
	}
}

// LooksLikeShape recognizes a shape by its header signature (the paired-design
// `Design` field), and rejects a technical design — the detector that catches a
// shape misfiled into the design corpus (#135, DINV-3).
func TestLooksLikeShape(t *testing.T) {
	// A well-formed shape is recognized.
	if !shapes.LooksLikeShape([]byte(wellFormed)) {
		t.Fatal("well-formed shape not recognized by its header signature")
	}
	// A malformed shape (empty body, unknown status) still LOOKS like a shape —
	// the `Design` field is present; malformedness is Parse's defect, misfiling
	// is the detector's concern.
	malformed := "# s\n\n| Field | Value |\n|---|---|\n| Design | widget-store |\n| Status | draft |\n"
	if !shapes.LooksLikeShape([]byte(malformed)) {
		t.Fatal("malformed-but-shaped document not recognized")
	}
	// A technical design (no `Design` header field) is NOT a shape.
	design := "# A design\n\n| Field | Value |\n|---|---|\n" +
		"| **Status** | Approved |\n| **Owner** | @x |\n| **Serves** | prd:f/FR-1 |\n" +
		"| **Template version** | tech-design-template/v1 |\n\n## 2. Decision\n\n**Decision:** x.\n"
	if shapes.LooksLikeShape([]byte(design)) {
		t.Fatal("a technical design was mistaken for a shape")
	}
	// A document with no header table, or no H1, is not a shape.
	if shapes.LooksLikeShape([]byte("# title\n\njust prose, no header table\n")) {
		t.Fatal("a header-less document was mistaken for a shape")
	}
	if shapes.LooksLikeShape([]byte("no h1\n\n| Field | Value |\n|---|---|\n| Design | x |\n")) {
		t.Fatal("a document without an H1 was treated as a shape")
	}
}

// hasDefect reports whether some defect carries the given op and a reason
// substring — the "named defect" assertion.
func hasDefect(defects []schema.Defect, op, reason string) bool {
	for _, d := range defects {
		if d.Op == op && strings.Contains(d.Reason, reason) {
			return true
		}
	}
	return false
}
