package seed

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// workspaceWith builds a bare workspace containing the given marker files.
func workspaceWith(t *testing.T, markers ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, m := range markers {
		if err := os.WriteFile(filepath.Join(dir, m), []byte("marker\n"), 0o644); err != nil {
			t.Fatalf("write marker %s: %v", m, err)
		}
	}
	return dir
}

func manifestAbs(dir string) string {
	return filepath.Join(dir, filepath.FromSlash(invoke.ManifestRelPath))
}

// The go stack ships real ecosystem defaults: the template spells them out
// verbatim (FR-20 — "verb commands filled from the ecosystem defaults where
// real defaults exist") and the harness's own parser consumes the file
// identically to a hand-authored one.
func TestSeedWritesTheGoDefaults(t *testing.T) {
	dir := workspaceWith(t, "go.mod")

	res, ref := Seed(dir, ecosystem.New())
	if ref != nil {
		t.Fatalf("refused: %s", ref.Message)
	}
	if res.Stack != "go" || res.Placeholder {
		t.Fatalf("result = %+v, want stack go with real defaults", res)
	}

	m, _, cerr := invoke.LoadManifest(dir)
	if cerr != nil {
		t.Fatalf("the harness cannot consume the seeded manifest: %s", cerr.Message)
	}
	if m.Stack != "go" {
		t.Fatalf("stack = %q, want go", m.Stack)
	}
	want := ecosystem.New().ResolveVerbs("go", nil)
	if len(m.Verbs) != len(want) {
		t.Fatalf("verbs = %d, want the %d ecosystem defaults", len(m.Verbs), len(want))
	}
	// Full-struct comparison, not just commands: a template whose verbs
	// dropped the env pass-throughs would still closed-parse but break at
	// runtime (manifest verbs OVERRIDE defaults, and the strip-by-default
	// runner leaves `go build` without HOME).
	for name, wv := range want {
		gv, ok := m.Verbs[name]
		if !ok {
			t.Fatalf("verb %q missing from the template", name)
		}
		if !reflect.DeepEqual(*gv, *wv) {
			t.Errorf("verb %q = %+v, want the ecosystem default %+v", name, *gv, *wv)
		}
	}
}

// A symlink at the manifest path — even a dangling one — is an existing
// operator-placed entry: UB-10 refuses, and nothing is ever written through
// the link.
func TestSeedRefusesASymlinkAtTheManifestPath(t *testing.T) {
	dir := workspaceWith(t, "go.mod")
	if err := os.MkdirAll(filepath.Dir(manifestAbs(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("dangling-target", manifestAbs(dir)); err != nil {
		t.Fatal(err)
	}

	res, ref := Seed(dir, ecosystem.New())
	if res != nil || ref == nil {
		t.Fatalf("res = %+v, ref = %+v — want the UB-10 refusal", res, ref)
	}
	if ref.Code != invoke.ExitConfigErr {
		t.Errorf("code = %d, want %d", ref.Code, invoke.ExitConfigErr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".pyxis", "dangling-target")); !os.IsNotExist(err) {
		t.Error("seed wrote through the symlink")
	}
}

// Stacks without real defaults (node, python) get the four ladder verbs as
// named to-be-edited placeholders — self-naming commands and a pre-filled
// pass-through env, closed-parsing like any hand-authored manifest.
func TestSeedWritesNamedPlaceholders(t *testing.T) {
	cases := []struct {
		stack  string
		marker string
	}{
		{"node", "package.json"},
		{"python", "pyproject.toml"},
	}
	for _, tc := range cases {
		t.Run(tc.stack, func(t *testing.T) {
			dir := workspaceWith(t, tc.marker)

			res, ref := Seed(dir, ecosystem.New())
			if ref != nil {
				t.Fatalf("refused: %s", ref.Message)
			}
			if res.Stack != tc.stack || !res.Placeholder {
				t.Fatalf("result = %+v, want stack %s with placeholders", res, tc.stack)
			}

			m, _, cerr := invoke.LoadManifest(dir)
			if cerr != nil {
				t.Fatalf("the harness cannot consume the seeded manifest: %s", cerr.Message)
			}
			for _, name := range ladderVerbNames {
				v, ok := m.Verbs[name]
				if !ok {
					t.Fatalf("ladder verb %q missing from the placeholder set", name)
				}
				if !strings.Contains(v.Command, "placeholder") || !strings.Contains(v.Command, name) {
					t.Errorf("verb %q command %q does not name itself as a placeholder for that verb", name, v.Command)
				}
				for _, env := range []string{"PATH", "HOME"} {
					if e, ok := v.Env[env]; !ok || e.Mode != schema.EnvPass {
						t.Errorf("verb %q env %s = %+v, want a pass-through entry", name, env, v.Env)
					}
				}
			}
		})
	}
}

// Several markers of ONE stack are not ambiguity — python declares three
// marker filenames and any subset of them still detects python.
func TestSeedSameStackMultipleMarkersIsUnambiguous(t *testing.T) {
	dir := workspaceWith(t, "requirements.txt", "setup.py")

	res, ref := Seed(dir, ecosystem.New())
	if ref != nil {
		t.Fatalf("refused: %s", ref.Message)
	}
	if res.Stack != "python" {
		t.Fatalf("stack = %q, want python", res.Stack)
	}
}

// UB-10: an existing manifest is operator-owned — refuse by name, and the
// file stays byte-identical.
func TestSeedRefusesAnExistingManifest(t *testing.T) {
	dir := workspaceWith(t, "go.mod")
	original := []byte(`{"version":"1.0.0","stack":"go","verbs":{}}` + "\n")
	if err := os.MkdirAll(filepath.Dir(manifestAbs(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestAbs(dir), original, 0o644); err != nil {
		t.Fatal(err)
	}

	res, ref := Seed(dir, ecosystem.New())
	if res != nil || ref == nil {
		t.Fatalf("res = %+v, ref = %+v — want a refusal", res, ref)
	}
	if ref.Code != invoke.ExitConfigErr {
		t.Errorf("code = %d, want %d", ref.Code, invoke.ExitConfigErr)
	}
	if !strings.Contains(ref.Message, invoke.ManifestRelPath) {
		t.Errorf("refusal does not cite the manifest path: %s", ref.Message)
	}
	after, err := os.ReadFile(manifestAbs(dir))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Errorf("the operator-owned manifest was modified:\n%s", after)
	}
}

// UB-11, empty tree: write nothing — not even the parent directory — and
// carry the hand-authoring remedy (the manifest path and the declared-stack
// requirement).
func TestSeedRefusesWithNoMarkers(t *testing.T) {
	dir := t.TempDir()

	res, ref := Seed(dir, ecosystem.New())
	if res != nil || ref == nil {
		t.Fatalf("res = %+v, ref = %+v — want a refusal", res, ref)
	}
	if ref.Code != invoke.ExitConfigErr {
		t.Errorf("code = %d, want %d", ref.Code, invoke.ExitConfigErr)
	}
	for _, want := range []string{invoke.ManifestRelPath, "stack", "go.mod", "package.json"} {
		if !strings.Contains(ref.Message, want) {
			t.Errorf("refusal lacks %q: %s", want, ref.Message)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".pyxis")); !os.IsNotExist(err) {
		t.Error("a refusal must write nothing, but .pyxis/ was created")
	}
}

// UB-11, mixed markers: never guess between stacks — name each detected
// stack with its evidence and write nothing.
func TestSeedRefusesAmbiguousMarkers(t *testing.T) {
	dir := workspaceWith(t, "go.mod", "package.json")

	res, ref := Seed(dir, ecosystem.New())
	if res != nil || ref == nil {
		t.Fatalf("res = %+v, ref = %+v — want a refusal", res, ref)
	}
	if ref.Code != invoke.ExitConfigErr {
		t.Errorf("code = %d, want %d", ref.Code, invoke.ExitConfigErr)
	}
	for _, want := range []string{"go", "node", "go.mod", "package.json", invoke.ManifestRelPath} {
		if !strings.Contains(ref.Message, want) {
			t.Errorf("refusal lacks %q: %s", want, ref.Message)
		}
	}
	if _, err := os.Stat(manifestAbs(dir)); !os.IsNotExist(err) {
		t.Error("a refusal must write nothing, but the manifest exists")
	}
}

func TestSeedReportsManifestDirectoryFailure(t *testing.T) {
	dir := workspaceWith(t, "go.mod")
	if err := os.WriteFile(filepath.Join(dir, ".pyxis"), []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, ref := Seed(dir, ecosystem.New())
	if res != nil || ref == nil || ref.Code != invoke.ExitRungErr {
		t.Fatalf("Seed with occupied manifest parent = (%+v, %+v), want operational refusal", res, ref)
	}
}

func TestWriteExclusiveReportsOpenFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "manifest.json")
	if err := writeExclusive(path, []byte("x")); err == nil {
		t.Fatal("writeExclusive with missing parent: want open error")
	}
}
