package techdesigns_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

func mk(t *testing.T, slug, header, body string) *techdesigns.Design {
	t.Helper()
	d, defects := techdesigns.Parse(slug, []byte("# "+slug+"\n\n| Field | Value |\n|---|---|\n"+header+"\n"+body))
	if d == nil {
		t.Fatalf("parse %s: %v", slug, defects)
	}
	return d
}

const goodHeader = `| **Status** | Approved |
| **Owner** | @o |
| **Approver(s)** | @o |
| **Date** | 2026-07-07 |
| **Relevant-when** | testing |
| **Scope** | fixture |
| **Tracking** | — |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | — |
| **Security/privacy impact** | none — fixture |
| **Template version** | tech-design-template/v1 |`

const goodBody = `
## 3. The design

### 3.4 Observability

none — fixture.

### 3.5 Security & privacy

none — fixture.
`

func env() techdesigns.Env {
	return techdesigns.Env{
		Grandfathered: map[string]bool{},
		PRDUnits:      map[string]map[string]string{"docgate": {"FR-1": "live", "FR-2": "retired", "WR-1": "live"}},
		DocExists:     func(p string) bool { return p == "AGENTS.md" },
	}
}

func wantDefect(t *testing.T, defects []schema.Defect, want string) {
	t.Helper()
	for _, d := range defects {
		if strings.Contains(d.Reason, want) {
			return
		}
	}
	t.Errorf("no defect containing %q (got %v)", want, defects)
}

func TestLint(t *testing.T) {
	good := mk(t, "good", goodHeader, goodBody)
	if d := techdesigns.Lint([]*techdesigns.Design{good}, env()); len(d) != 0 {
		t.Fatalf("conforming design flagged: %v", d)
	}
	cases := []struct {
		name, header, body, want string
	}{
		{"missing-field", strings.Replace(goodHeader, "| **Scope** | fixture |\n", "", 1), goodBody, `missing header field "Scope"`},
		{"unknown-field", goodHeader + "\n| **Mood** | fine |", goodBody, `unknown header field "Mood"`},
		{"range-shorthand", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | prd:docgate/FR-1..FR-8 |", 1), goodBody, "range shorthand"},
		{"bad-grammar", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | requirements:FR-1 |", 1), goodBody, "does not match the typed grammar"},
		{"dangling-prd", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | prd:docgate/FR-99 |", 1), goodBody, "not in the ledger's source map"},
		{"dangling-doc", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | doc:nope.md |", 1), goodBody, "path does not exist"},
		{"no-observability", goodHeader, strings.Replace(goodBody, "Observability", "Operability", 1), "no Observability subsection"},
		{"no-security", goodHeader, strings.Replace(goodBody, "Security & privacy", "Safety", 1), "no Security & privacy subsection"},
		{"security-pair", strings.Replace(goodHeader, "| **Security/privacy impact** | none — fixture |", "| **Security/privacy impact** | new auth surface |", 1), goodBody, "disagree on `none`"},
		{"dangling-supersedes", strings.Replace(goodHeader, "| **Supersedes** | — |", "| **Supersedes** | ghost |", 1), goodBody, "does not resolve to a design"},
		{"precedence-wr-only", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | prd:docgate/WR-1 · doc:AGENTS.md |", 1), goodBody, "does not satisfy PRD-precedes-design (incident I1)"},
	}
	for _, c := range cases {
		d := mk(t, "x", c.header, c.body)
		wantDefect(t, techdesigns.Lint([]*techdesigns.Design{d}, env()), c.want)
	}
	// Precedence: an FR anchor beside the stub → green; precedence-grandfathered WR-only → green.
	e2 := env()
	e2.PRDUnits["docgate"]["WR-1"] = "live"
	okd := mk(t, "okd", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | prd:docgate/WR-1 · prd:docgate/FR-1 |", 1), goodBody)
	if d := techdesigns.Lint([]*techdesigns.Design{okd}, e2); len(d) != 0 {
		t.Fatalf("FR-anchored design flagged: %v", d)
	}
	e2.PrecedenceGrandfathered = map[string]bool{"wr": true}
	wr := mk(t, "wr", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | prd:docgate/WR-1 |", 1), goodBody)
	if d := techdesigns.Lint([]*techdesigns.Design{wr}, e2); len(d) != 0 {
		t.Fatalf("precedence-grandfathered design flagged: %v", d)
	}
	// Grandfathered: same defective doc, zero defects.
	bad := mk(t, "old", strings.Replace(goodHeader, "| **Scope** | fixture |\n", "", 1), "")
	e := env()
	e.Grandfathered["old"] = true
	if d := techdesigns.Lint([]*techdesigns.Design{bad}, e); len(d) != 0 {
		t.Fatalf("grandfathered design flagged: %v", d)
	}
}

func TestIndex(t *testing.T) {
	a := mk(t, "a", goodHeader, goodBody)
	b := mk(t, "b", strings.Replace(strings.Replace(goodHeader,
		"| **Supersedes** | — |", "| **Supersedes** | a |", 1),
		"| **Serves** | — |", "| **Serves** | prd:docgate/FR-1 |", 1), goodBody)
	c := mk(t, "c", strings.Replace(strings.Replace(goodHeader,
		"| **Builds-on** | — |", "| **Builds-on** | a |", 1),
		"| **Serves** | — |", "| **Serves** | prd:docgate/FR-2 · design:a |", 1), goodBody)
	md, js, defects := techdesigns.GenIndex([]*techdesigns.Design{a, b, c}, env())
	if len(defects) != 0 {
		t.Fatalf("graph defects: %v", defects)
	}
	s := string(js)
	for _, want := range []string{
		`"a": {`, `"status": "superseded"`, `"superseded_by": "b"`,
		`"stale-builds-on"`, // c builds on superseded a
		`"stale-serves"`,    // c serves retired FR-2 and superseded design a
		`"prd:docgate/FR-1": [`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("index.json missing %q", want)
		}
	}
	if !strings.Contains(string(md), "| `a` | superseded → b |") {
		t.Errorf("index.md liveness row wrong:\n%s", md)
	}
	// Determinism.
	md2, js2, _ := techdesigns.GenIndex([]*techdesigns.Design{a, b, c}, env())
	if string(md) != string(md2) || string(js) != string(js2) {
		t.Fatal("index generation is not deterministic")
	}
	// Two live successors of one target → defect.
	b2 := mk(t, "b2", strings.Replace(goodHeader, "| **Supersedes** | — |", "| **Supersedes** | a |", 1), goodBody)
	_, _, d2 := techdesigns.GenIndex([]*techdesigns.Design{a, b, b2}, env())
	wantDefect(t, d2, "one live successor")
	// Cycle → defect.
	x := mk(t, "x", strings.Replace(goodHeader, "| **Supersedes** | — |", "| **Supersedes** | y |", 1), goodBody)
	y := mk(t, "y", strings.Replace(goodHeader, "| **Supersedes** | — |", "| **Supersedes** | x |", 1), goodBody)
	_, _, d3 := techdesigns.GenIndex([]*techdesigns.Design{x, y}, env())
	wantDefect(t, d3, "cycle")
}

func TestCoverageOnRead(t *testing.T) {
	// design a serves the live unit FR-1.
	a := mk(t, "a", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | prd:docgate/FR-1 |", 1), goodBody)

	// FR-28: gen-index no longer embodies coverage — no `coverage` key, no
	// `## Coverage` section, and schema_version is bumped to 2.
	md, js, defects := techdesigns.GenIndex([]*techdesigns.Design{a}, env())
	if len(defects) != 0 {
		t.Fatalf("index defects: %v", defects)
	}
	if strings.Contains(string(js), `"coverage"`) {
		t.Error("index.json still carries the coverage key")
	}
	if strings.Contains(string(md), "## Coverage") {
		t.Error("index.md still carries the coverage section")
	}
	if !strings.Contains(string(js), `"schema_version": "2"`) {
		t.Errorf("schema_version not bumped to 2:\n%s", js)
	}

	// Coverage is computed on read: universe = live ledger units (FR-1, WR-1;
	// FR-2 is retired), FR-1 served by a, WR-1 unserved.
	cmd, cjs, cdef := techdesigns.GenCoverage([]*techdesigns.Design{a}, env())
	if len(cdef) != 0 {
		t.Fatalf("coverage defects: %v", cdef)
	}
	if !strings.Contains(string(cjs), `"universe_size": 2`) {
		t.Errorf("coverage universe wrong:\n%s", cjs)
	}
	if !strings.Contains(string(cjs), `"prd:docgate/FR-1"`) {
		t.Error("coverage did not record FR-1 as served")
	}
	if !strings.Contains(string(cmd), "WR-1") {
		t.Errorf("coverage md missing the unserved WR-1:\n%s", cmd)
	}

	// DINV-1: adding an UNSERVED ledger unit leaves index.* byte-identical...
	e2 := env()
	e2.PRDUnits["docgate"]["FR-99"] = "live"
	md2, js2, _ := techdesigns.GenIndex([]*techdesigns.Design{a}, e2)
	if string(md) != string(md2) || string(js) != string(js2) {
		t.Error("adding an unserved ledger unit changed index.* — DINV-1 violated")
	}
	// ...but coverage-on-read picks it up (as unserved).
	_, cjs2, _ := techdesigns.GenCoverage([]*techdesigns.Design{a}, e2)
	if !strings.Contains(string(cjs2), "FR-99") {
		t.Error("coverage did not pick up the newly-added unit")
	}
}
