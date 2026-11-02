package contextpack

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// hasDefect reports whether any defect's rendered form contains sub.
func hasDefect(defects []schema.Defect, sub string) bool {
	for _, d := range defects {
		if strings.Contains(d.String(), sub) {
			return true
		}
	}
	return false
}

// writeFile writes content under root at a repo-relative slash path, making
// parent dirs — the hermetic fixture builder (no git, no network).
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// fixtureRoot builds a tiny repo tree: two governing docs, a subject design,
// and — deliberately — an authoring-trail file no declaration can reach.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "docs/product-vision.md", "VISION north star\n")
	writeFile(t, root, "docs/architecture-principles.md", "AP-1 principle\n")
	writeFile(t, root, "docs/tech-design/sub-one.md", "DESIGN sub-one body\n")
	// The answer key a cold reviewer must never receive. No admissible kind
	// resolves to it; it exists only to prove the boundary holds (DINV-3).
	writeFile(t, root, "docs/problems/sub-one/delivery.md", "TRAIL secret disposition reasoning\n")
	return root
}

// reviewDecl is a declaration with governing + subject + an optional
// calibration the fixture root does not provide (exercises FR-7).
func reviewDecl() Declaration {
	return Declaration{
		"tech-design-review": {
			{Kind: KindGoverning, Selector: "docs/product-vision.md"},
			{Kind: KindGoverning, Selector: "docs/architecture-principles.md"},
			{Kind: KindSubject, Selector: "docs/tech-design/{subject}.md"},
			{Kind: KindCalibration, Selector: "docs/calibration/{subject}.md", Optional: true},
		},
	}
}

// --- T1: the closed declaration and the admissible-kind set ---

func TestAdmissibleKindsHaveNoTrailKind(t *testing.T) {
	// DINV-3: the boundary is structural — no authoring-trail kind is a member,
	// so a declaration has no way to name the trail.
	for _, forbidden := range []Kind{"journal", "delivery", "disposition", "orchestration", "problem", "review"} {
		if AdmissibleKinds[forbidden] {
			t.Errorf("authoring-trail kind %q is admissible — the independence boundary is broken", forbidden)
		}
	}
	if len(AdmissibleKinds) != 3 {
		t.Errorf("admissible set has %d kinds, want the three stable-context kinds", len(AdmissibleKinds))
	}
}

func TestParseDeclaration(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantDefect string // substring; "" = must parse clean
		check      func(*testing.T, Declaration)
	}{
		{
			name: "valid stage with ordered entries and optional marker",
			in: "tech-design-review:\n" +
				"  - governing docs/product-vision.md\n" +
				"  - subject docs/tech-design/{subject}.md\n" +
				"  - calibration? docs/calibration/{subject}.md\n",
			check: func(t *testing.T, d Declaration) {
				e := d["tech-design-review"]
				if len(e) != 3 {
					t.Fatalf("got %d entries, want 3", len(e))
				}
				if e[0].Kind != KindGoverning || e[1].Kind != KindSubject {
					t.Errorf("kinds/order wrong: %+v", e)
				}
				if !e[2].Optional {
					t.Errorf("calibration? should be optional")
				}
			},
		},
		{
			name:       "authoring-trail kind is rejected (DINV-3 / UB-3)",
			in:         "s:\n  - journal docs/problems/{subject}/delivery.md\n",
			wantDefect: `unknown kind "journal"`,
		},
		{
			name:       "unknown kind is rejected",
			in:         "s:\n  - manifest docs/x.md\n",
			wantDefect: `unknown kind "manifest"`,
		},
		{
			name:       "entry before a stage header",
			in:         "  - governing docs/x.md\n",
			wantDefect: "entry before any stage header",
		},
		{
			name:       "duplicate stage",
			in:         "s:\n  - governing docs/x.md\ns:\n  - governing docs/y.md\n",
			wantDefect: "duplicate stage",
		},
		{
			name:       "malformed entry (no selector)",
			in:         "s:\n  - governing\n",
			wantDefect: "not `<kind>[?] <selector>`",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, defects := ParseDeclaration([]byte(tc.in))
			if tc.wantDefect == "" {
				if len(defects) != 0 {
					t.Fatalf("unexpected defects: %v", defects)
				}
				if tc.check != nil {
					tc.check(t, d)
				}
				return
			}
			if !hasDefect(defects, tc.wantDefect) {
				t.Fatalf("want defect containing %q, got %v", tc.wantDefect, defects)
			}
		})
	}
}

func TestResolveUnknownStage(t *testing.T) {
	_, defects := reviewDecl().Resolve("no-such-stage", "sub-one")
	if !hasDefect(defects, "no declaration for stage") {
		t.Fatalf("unknown stage should fail closed, got %v", defects)
	}
}

// --- T2: the deterministic build-fresh assembler ---

func TestBuildAssemblesManifestAndVerbatimBody(t *testing.T) {
	root := fixtureRoot(t)
	pack, defects := Build(root, "tech-design-review", "sub-one", reviewDecl())
	if len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	s := string(pack)
	for _, want := range []string{
		"# context-pack/v1",
		"# stage: tech-design-review",
		"# subject: sub-one",
		"# built_from: content:sha256:",
		"# digest: sha256:",
		"docs/product-vision.md (present)",
		"docs/calibration/sub-one.md (absent)", // FR-7: optional-absent recorded
		"VISION north star",                    // verbatim body
		"DESIGN sub-one body",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("pack missing %q", want)
		}
	}
	// FR-7: no empty body section for the absent optional class.
	if strings.Contains(s, "context-pack source: calibration") {
		t.Errorf("absent optional class must not emit a body section")
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	root := fixtureRoot(t)
	a, _ := Build(root, "tech-design-review", "sub-one", reviewDecl())
	b, _ := Build(root, "tech-design-review", "sub-one", reviewDecl())
	if !bytes.Equal(a, b) {
		t.Fatalf("two builds at one source state differ (DINV-1)")
	}
}

func TestBuildFreshReflectsSourceChange(t *testing.T) {
	root := fixtureRoot(t)
	before, _ := Build(root, "tech-design-review", "sub-one", reviewDecl())
	writeFile(t, root, "docs/tech-design/sub-one.md", "DESIGN sub-one body CHANGED\n")
	after, _ := Build(root, "tech-design-review", "sub-one", reviewDecl())
	if bytes.Equal(before, after) {
		t.Fatalf("a source change must change the pack (DINV-2, FR-6)")
	}
	if !strings.Contains(string(after), "CHANGED") {
		t.Errorf("rebuild did not pick up the new content")
	}
}

func TestBuildIndependenceNoTrailContent(t *testing.T) {
	// DINV-3: the pack over a subject whose trail exists on disk contains none
	// of it — no admissible kind reaches the trail, so it cannot enter.
	root := fixtureRoot(t)
	pack, defects := Build(root, "tech-design-review", "sub-one", reviewDecl())
	if len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	if strings.Contains(string(pack), "TRAIL secret") {
		t.Fatalf("authoring-trail content leaked into the pack — DINV-3 broken")
	}
}

func TestBuildFailsClosedOnMissingRequiredSource(t *testing.T) {
	root := fixtureRoot(t)
	// A required governing source that does not resolve → defect, no artifact.
	d := Declaration{"s": {{Kind: KindGoverning, Selector: "docs/missing.md"}}}
	pack, defects := Build(root, "s", "sub-one", d)
	if pack != nil {
		t.Fatalf("a missing required source must emit no artifact (DINV-6)")
	}
	if !hasDefect(defects, "does not resolve") {
		t.Fatalf("want a named missing-source defect, got %v", defects)
	}
}

func TestBuildUnknownSubject(t *testing.T) {
	root := fixtureRoot(t)
	pack, defects := Build(root, "tech-design-review", "ghost", reviewDecl())
	if pack != nil {
		t.Fatalf("an unknown subject must emit no artifact")
	}
	if !hasDefect(defects, `subject "ghost" has no subject source`) {
		t.Fatalf("want a UB-2 subject defect, got %v", defects)
	}
}

func TestBuildEmptySubject(t *testing.T) {
	root := fixtureRoot(t)
	_, defects := Build(root, "tech-design-review", "   ", reviewDecl())
	if !hasDefect(defects, "empty subject") {
		t.Fatalf("empty subject should fail closed, got %v", defects)
	}
}

func TestBuildLeavesSourcesUnchanged(t *testing.T) {
	// DINV-5: the compiler reads, never writes — sources are byte-identical after.
	root := fixtureRoot(t)
	rel := "docs/tech-design/sub-one.md"
	before, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if _, defects := Build(root, "tech-design-review", "sub-one", reviewDecl()); len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	after, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if !bytes.Equal(before, after) {
		t.Fatalf("a source file was mutated by a build (DINV-5)")
	}
}

// --- T3: the verb entry point over the on-disk declaration ---

func TestRunLoadsDeclarationAndBuilds(t *testing.T) {
	root := fixtureRoot(t)
	writeFile(t, root, DeclarationFile,
		"tech-design-review:\n"+
			"  - governing docs/product-vision.md\n"+
			"  - subject docs/tech-design/{subject}.md\n")
	pack, defects := Run(root, "tech-design-review", "sub-one")
	if len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	if !strings.Contains(string(pack), "DESIGN sub-one body") {
		t.Errorf("run did not assemble the subject")
	}
}

func TestRunNoDeclarationFile(t *testing.T) {
	root := t.TempDir()
	_, defects := Run(root, "s", "x")
	if !hasDefect(defects, "no declaration file") {
		t.Fatalf("absent declaration should fail closed, got %v", defects)
	}
}
