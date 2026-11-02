package relmanifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// loadFixturePolicy parses testdata/policy.yaml — the fixture mirror of the
// authored class policy.
func loadFixturePolicy(t *testing.T) relmanifest.ClassPolicy {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "policy.yaml"))
	if err != nil {
		t.Fatalf("read fixture policy: %v", err)
	}
	p, err := relmanifest.ParseClassPolicy(b)
	if err != nil {
		t.Fatalf("parse fixture policy: %v", err)
	}
	return p
}

// The shape example's rows parse in order with their classes and version.
func TestParseClassPolicyAuthoredShape(t *testing.T) {
	p := loadFixturePolicy(t)
	if p.SchemaVersion != relmanifest.PolicySchemaVersion {
		t.Fatalf("schema version = %q, want %q", p.SchemaVersion, relmanifest.PolicySchemaVersion)
	}
	want := []relmanifest.ClassRule{
		{Glob: ".pyxis/config.yaml", Class: relmanifest.RepoOwned},
		{Glob: ".pyxis/stamp/**", Class: relmanifest.RepoOwned},
		{Glob: ".pyxis/vendor/**", Class: relmanifest.EngineVerbatim},
		{Glob: ".pyxis/**", Class: relmanifest.EngineVerbatim},
		{Glob: ".github/workflows/docs-gate.yml", Class: relmanifest.EngineVerbatim},
		{Glob: "docs/tech-design/index.*", Class: relmanifest.Generated},
	}
	if len(p.Classes) != len(want) {
		t.Fatalf("rule count = %d, want %d (%+v)", len(p.Classes), len(want), p.Classes)
	}
	for i, w := range want {
		if p.Classes[i] != w {
			t.Errorf("rule %d = %+v, want %+v (order is first-match-wins contract)", i, p.Classes[i], w)
		}
	}
}

// Every closed-parse violation refuses by name: unknown keys (both levels),
// unknown classes, missing/wrong schema pieces, malformed structure and
// globs. Fixture-file rows cover the charter's named unknown-key/class
// defects; inline rows cover the rest of the grammar.
func TestParseClassPolicyDefects(t *testing.T) {
	valid := "schema_version: class-policy/v1\nclasses:\n  - { glob: \"a.md\", class: generated }\n"
	if _, err := relmanifest.ParseClassPolicy([]byte(valid)); err != nil {
		t.Fatalf("valid minimal policy refused: %v", err)
	}
	cases := []struct {
		name    string
		src     string
		fixture string
		want    string
	}{
		{name: "unknown entry key", fixture: "policy-unknown-key.yaml", want: `unknown key "owner"`},
		{name: "unknown ownership class", fixture: "policy-unknown-class.yaml", want: `unknown ownership class "engine-owned"`},
		{name: "unknown top-level key",
			src:  "schema_version: class-policy/v1\nscope: all\nclasses:\n  - { glob: \"a\", class: generated }\n",
			want: `unknown top-level key "scope"`},
		{name: "missing schema_version",
			src:  "classes:\n  - { glob: \"a\", class: generated }\n",
			want: "missing schema_version"},
		{name: "wrong schema_version",
			src:  "schema_version: class-policy/v2\nclasses:\n  - { glob: \"a\", class: generated }\n",
			want: `schema_version "class-policy/v2"`},
		{name: "missing classes",
			src:  "schema_version: class-policy/v1\n",
			want: "missing classes"},
		{name: "duplicate top-level key",
			src:  "schema_version: class-policy/v1\nschema_version: class-policy/v1\nclasses:\n  - { glob: \"a\", class: generated }\n",
			want: `duplicate top-level key "schema_version"`},
		{name: "list item outside classes",
			src:  "  - { glob: \"a\", class: generated }\nschema_version: class-policy/v1\nclasses:\n  - { glob: \"b\", class: generated }\n",
			want: "list item outside classes"},
		{name: "scalar under classes",
			src:  "schema_version: class-policy/v1\nclasses:\n  foo: bar\n",
			want: "unrecognized structure"},
		{name: "entry not a flow map",
			src:  "schema_version: class-policy/v1\nclasses:\n  - a\n",
			want: "not a `{ glob: …, class: … }` flow map"},
		{name: "unterminated flow map",
			src:  "schema_version: class-policy/v1\nclasses:\n  - { glob: \"a\", class: generated\n",
			want: "unterminated flow map"},
		{name: "trailing garbage after entry",
			src:  "schema_version: class-policy/v1\nclasses:\n  - { glob: \"a\", class: generated } x\n",
			want: "trailing content"},
		{name: "duplicate key in entry",
			src:  "schema_version: class-policy/v1\nclasses:\n  - { glob: \"a\", glob: \"b\", class: generated }\n",
			want: `duplicate key "glob"`},
		{name: "missing glob",
			src:  "schema_version: class-policy/v1\nclasses:\n  - { class: generated }\n",
			want: "missing glob"},
		{name: "missing class",
			src:  "schema_version: class-policy/v1\nclasses:\n  - { glob: \"a\" }\n",
			want: `class entry "a" missing class`},
		{name: "bad glob pattern",
			src:  "schema_version: class-policy/v1\nclasses:\n  - { glob: \"a[\", class: generated }\n",
			want: "bad pattern segment"},
		{name: "absolute glob",
			src:  "schema_version: class-policy/v1\nclasses:\n  - { glob: \"/a\", class: generated }\n",
			want: "absolute pattern"},
		{name: "empty glob segment",
			src:  "schema_version: class-policy/v1\nclasses:\n  - { glob: \"a//b\", class: generated }\n",
			want: "empty path segment"},
		{name: "classes with inline value",
			src:  "schema_version: class-policy/v1\nclasses: []\n",
			want: "classes takes no inline value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := []byte(c.src)
			if c.fixture != "" {
				b, err := os.ReadFile(filepath.Join("testdata", c.fixture))
				if err != nil {
					t.Fatalf("read fixture: %v", err)
				}
				src = b
			}
			_, err := relmanifest.ParseClassPolicy(src)
			if err == nil {
				t.Fatalf("no error; want one containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err, c.want)
			}
		})
	}
}

// First match wins: the specific repo-owned rows beat the engine-verbatim
// catch-all they precede, and an unmatched path reports ok=false.
func TestClassOfFirstMatchWins(t *testing.T) {
	p := loadFixturePolicy(t)
	cases := []struct {
		path string
		want relmanifest.Class
		ok   bool
	}{
		{".pyxis/config.yaml", relmanifest.RepoOwned, true},
		{".pyxis/stamp/origin.yaml", relmanifest.RepoOwned, true},
		{".pyxis/vendor/mermaid/mermaid.min.js", relmanifest.EngineVerbatim, true},
		{".pyxis/templates/prd-template.md", relmanifest.EngineVerbatim, true},
		{".github/workflows/docs-gate.yml", relmanifest.EngineVerbatim, true},
		{"docs/tech-design/index.json", relmanifest.Generated, true},
		{"docs/tech-design/index.md", relmanifest.Generated, true},
		{"README.md", "", false},
		{".github/workflows/other.yml", "", false},
	}
	for _, c := range cases {
		got, ok, err := p.ClassOf(c.path)
		if err != nil {
			t.Fatalf("ClassOf(%q): %v", c.path, err)
		}
		if ok != c.ok || got != c.want {
			t.Errorf("ClassOf(%q) = (%q, %v), want (%q, %v)", c.path, got, ok, c.want, c.ok)
		}
	}
}

// The doublestar semantics pinned: `**` matches zero or more whole segments;
// `*` and `?` never cross a slash.
func TestGlobSemantics(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{".pyxis/**", ".pyxis/config.yaml", true},
		{".pyxis/**", ".pyxis/a/b/c.md", true},
		{".pyxis/**", ".pyxis", true}, // ** matches zero segments
		{".pyxis/**", "docs/x.md", false},
		{".pyxis/*", ".pyxis/config.yaml", true},
		{".pyxis/*", ".pyxis/a/b.md", false}, // * never crosses /
		{"docs/tech-design/index.*", "docs/tech-design/index.json", true},
		{"docs/tech-design/index.*", "docs/tech-design/index.md", true},
		{"docs/tech-design/index.*", "docs/tech-design/sub/index.json", false},
		{"a/**/z.md", "a/z.md", true},
		{"a/**/z.md", "a/b/z.md", true},
		{"a/**/z.md", "a/b/c/z.md", true},
		{"a/**/z.md", "a/b/c/q.md", false},
		{"**/mermaid.min.js", ".pyxis/vendor/mermaid/mermaid.min.js", true},
		{".github/workflows/docs-gate.yml", ".github/workflows/docs-gate.yml", true},
		{".github/workflows/docs-gate.yml", ".github/workflows/other.yml", false},
	}
	for _, c := range cases {
		p := relmanifest.ClassPolicy{Classes: []relmanifest.ClassRule{{Glob: c.glob, Class: relmanifest.EngineVerbatim}}}
		_, ok, err := p.ClassOf(c.path)
		if err != nil {
			t.Fatalf("glob %q vs %q: %v", c.glob, c.path, err)
		}
		if ok != c.want {
			t.Errorf("glob %q vs %q = %v, want %v", c.glob, c.path, ok, c.want)
		}
	}
}
