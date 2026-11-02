package shapes_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/shapes"
)

// Lifecycle T1 (§3.6, DINV-3): the flip is a one-way move over the closed enum.
func TestTransition(t *testing.T) {
	cases := map[string]struct {
		from, to string
		wantOp   string // "" = clean
	}{
		"flip-authoritative-to-superseded": {shapes.StatusAuthoritative, shapes.StatusSupersededByCode, ""},
		"idempotent-authoritative":         {shapes.StatusAuthoritative, shapes.StatusAuthoritative, ""},
		"idempotent-superseded":            {shapes.StatusSupersededByCode, shapes.StatusSupersededByCode, ""},
		"reverse-flip-refused":             {shapes.StatusSupersededByCode, shapes.StatusAuthoritative, "lifecycle"},
		"unknown-prior":                    {"provisional", shapes.StatusAuthoritative, "status"},
		"unknown-target":                   {shapes.StatusAuthoritative, "retired", "status"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := shapes.Transition("widget-store.md", tc.from, tc.to)
			if tc.wantOp == "" {
				if len(d) != 0 {
					t.Fatalf("legal transition %s→%s flagged: %v", tc.from, tc.to, d)
				}
				return
			}
			if len(d) == 0 {
				t.Fatalf("illegal transition %s→%s not flagged", tc.from, tc.to)
			}
			if d[0].Op != tc.wantOp {
				t.Fatalf("want op %q, got %+v", tc.wantOp, d[0])
			}
			if d[0].File != "widget-store.md" || d[0].Remedy == "" {
				t.Fatalf("defect not fully populated: %+v", d[0])
			}
		})
	}
}

// Lifecycle T2 (§3.4, DINV-3, R4): an authoritative shape whose implementing
// code exists is stale; superseded-by-code and no-code shapes are clean.
func TestStaleShape(t *testing.T) {
	auth := &shapes.Shape{Slug: "widget-store", Status: shapes.StatusAuthoritative}
	// Code exists + still authoritative → stale-shape.
	d := shapes.StaleShape(auth, true)
	if len(d) == 0 || d[0].Verb != "stale-shape" || d[0].File != "widget-store.md" {
		t.Fatalf("authoritative shape with code not flagged stale: %v", d)
	}
	if d[0].Remedy == "" {
		t.Fatalf("stale-shape defect carries no remedy")
	}
	// No code yet → clean.
	if d := shapes.StaleShape(auth, false); len(d) != 0 {
		t.Fatalf("authoritative shape without code flagged: %v", d)
	}
	// Already superseded-by-code → clean even with code.
	done := &shapes.Shape{Slug: "widget-store", Status: shapes.StatusSupersededByCode}
	if d := shapes.StaleShape(done, true); len(d) != 0 {
		t.Fatalf("superseded-by-code shape flagged stale: %v", d)
	}
	// nil is a no-op.
	if d := shapes.StaleShape(nil, true); len(d) != 0 {
		t.Fatalf("nil shape produced a defect: %v", d)
	}
}

// Lifecycle T3 (DINV-5, FR-6): a change confined to non-binding content is not
// a Decision change; a change to the binding markup (**Decision:**/DINV-n/A-n)
// is — the discrimination that keeps a shape-confined change apart from a
// supersession-only Decision change.
func TestBindingMarkupChanged(t *testing.T) {
	const design = "# D\n\n## 2. Decision\n\n**Decision:** the run store is keyed by tenant.\n\n" +
		"### 3.3 Invariants\n\n- **DINV-1** — every run carries a tenant. *(FR-1)*\n\n" +
		"Some non-binding prose describing the rationale.\n"

	// Editing only non-binding prose → not a binding-markup change.
	proseEdit := design + "\nMore rationale added later, purely descriptive.\n"
	if shapes.BindingMarkupChanged([]byte(design), []byte(proseEdit)) {
		t.Fatalf("a prose-only edit was classified as a binding-markup change")
	}

	// Altering the Decision statement → a binding-markup change.
	decisionEdit := strings.Replace(design, "keyed by tenant", "keyed by region", 1)
	if !shapes.BindingMarkupChanged([]byte(design), []byte(decisionEdit)) {
		t.Fatalf("an altered **Decision:** was not classified as a binding-markup change")
	}

	// Altering a DINV-n invariant → a binding-markup change.
	dinvEdit := strings.Replace(design, "every run carries a tenant", "every run carries a region", 1)
	if !shapes.BindingMarkupChanged([]byte(design), []byte(dinvEdit)) {
		t.Fatalf("an altered DINV-n was not classified as a binding-markup change")
	}

	// A DINV token inside a code fence is illustrative, not binding.
	fenced := design + "\n```\nDINV-9 mentioned in an example block\n```\n"
	fencedEdit := design + "\n```\nDINV-9 reworded in an example block\n```\n"
	if shapes.BindingMarkupChanged([]byte(fenced), []byte(fencedEdit)) {
		t.Fatalf("an edit inside a code fence was classified as a binding-markup change")
	}
}
