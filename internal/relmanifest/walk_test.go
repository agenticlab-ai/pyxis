package relmanifest_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// update regenerates the committed golden manifest after an intentional
// schema or fixture change:
//
//	go test ./internal/relmanifest/ -run TestWalkDerivesGoldenManifest -update
var update = flag.Bool("update", false, "regenerate the committed golden manifest")

// goldenMeta is the fixed release-level input for the fixture walks —
// hermetic constants, no clock, no network.
func goldenMeta() relmanifest.Meta {
	return relmanifest.Meta{
		SystemVersion: "v1.2.3",
		SourceCommit:  "0123456789abcdef0123456789abcdef01234567",
		Binaries: map[string]map[string]string{
			"docgate": {
				"darwin-arm64": strings.Repeat("11", 32),
				"linux-amd64":  strings.Repeat("22", 32),
			},
			"pyxis": {
				"darwin-arm64": strings.Repeat("33", 32),
				"linux-amd64":  strings.Repeat("44", 32),
			},
		},
		ArchiveSHA256:    strings.Repeat("55", 32),
		DepsBundleSHA256: strings.Repeat("66", 32),
	}
}

// The walk over the real-shaped fixture tree derives exactly the committed
// golden bytes — derivation and canonical encoding pinned together.
func TestWalkDerivesGoldenManifest(t *testing.T) {
	policy := loadFixturePolicy(t)
	m, err := relmanifest.Walk(filepath.Join("testdata", "payload-real-shaped"), policy, goldenMeta())
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	out, err := m.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	goldenPath := filepath.Join("testdata", "golden-manifest.json")
	if *update {
		if err := os.WriteFile(goldenPath, out, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden missing (run with -update to regenerate): %v", err)
	}
	if !bytes.Equal(out, golden) {
		t.Fatalf("derived manifest diverges from golden:\n--- golden ---\n%s\n--- derived ---\n%s", golden, out)
	}
}

// Per-class posture over the fixture walk: first-match classes, engine-verbatim
// hashes match an independent sha256, repo-owned and generated carry null.
func TestWalkClassesAndHashes(t *testing.T) {
	policy := loadFixturePolicy(t)
	m, err := relmanifest.Walk(filepath.Join("testdata", "payload-real-shaped"), policy, goldenMeta())
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	wantClasses := map[string]relmanifest.Class{
		".github/workflows/docs-gate.yml":      relmanifest.EngineVerbatim,
		".pyxis/config.yaml":                   relmanifest.RepoOwned,
		".pyxis/stamp/origin.yaml":             relmanifest.RepoOwned,
		".pyxis/templates/prd-template.md":     relmanifest.EngineVerbatim,
		".pyxis/vendor/mermaid/mermaid.min.js": relmanifest.EngineVerbatim,
		"docs/tech-design/index.json":          relmanifest.Generated,
		"docs/tech-design/index.md":            relmanifest.Generated,
	}
	if len(m.Payload.Files) != len(wantClasses) {
		t.Fatalf("file count = %d, want %d: %+v", len(m.Payload.Files), len(wantClasses), m.Payload.Files)
	}
	for _, f := range m.Payload.Files {
		want, known := wantClasses[f.Path]
		if !known {
			t.Errorf("unexpected file %q in manifest", f.Path)
			continue
		}
		if f.Class != want {
			t.Errorf("file %q classed %q, want %q", f.Path, f.Class, want)
		}
		if (want == relmanifest.EngineVerbatim) != (f.SHA256 != nil) {
			t.Errorf("file %q (class %q): sha256 nullability wrong (got %v)", f.Path, f.Class, f.SHA256)
		}
	}
	// One engine-verbatim hash cross-checked against an independent sha256.
	b, err := os.ReadFile(filepath.Join("testdata", "payload-real-shaped", ".pyxis", "templates", "prd-template.md"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sum := sha256.Sum256(b)
	want := hex.EncodeToString(sum[:])
	for _, f := range m.Payload.Files {
		if f.Path == ".pyxis/templates/prd-template.md" && *f.SHA256 != want {
			t.Errorf("hash mismatch for %q: got %s, want %s", f.Path, *f.SHA256, want)
		}
	}
}

// A payload file no glob matches fails the walk with an *UnclassedError
// naming EXACTLY that file (DINV-6, A-11).
func TestWalkUnclassedNamesExactlyTheFile(t *testing.T) {
	policy := loadFixturePolicy(t)
	_, err := relmanifest.Walk(filepath.Join("testdata", "payload-unclassed"), policy, goldenMeta())
	if err == nil {
		t.Fatal("unclassed payload walked clean; want the DINV-6 refusal")
	}
	var unclassed *relmanifest.UnclassedError
	if !errors.As(err, &unclassed) {
		t.Fatalf("error is not an *UnclassedError: %v", err)
	}
	if len(unclassed.Paths) != 1 || unclassed.Paths[0] != "README.md" {
		t.Fatalf("unclassed set = %v, want exactly [README.md]", unclassed.Paths)
	}
	if !strings.Contains(err.Error(), "README.md") {
		t.Fatalf("error %q does not name README.md", err)
	}
}

// Two walks over the same tree produce identical values and bytes, with
// files sorted by path — the deterministic-output contract.
func TestWalkDeterministic(t *testing.T) {
	policy := loadFixturePolicy(t)
	root := filepath.Join("testdata", "payload-real-shaped")
	m1, err := relmanifest.Walk(root, policy, goldenMeta())
	if err != nil {
		t.Fatalf("walk 1: %v", err)
	}
	m2, err := relmanifest.Walk(root, policy, goldenMeta())
	if err != nil {
		t.Fatalf("walk 2: %v", err)
	}
	if !reflect.DeepEqual(m1, m2) {
		t.Fatalf("walks diverge:\n%+v\n%+v", m1, m2)
	}
	if !slices.IsSortedFunc(m1.Payload.Files, func(a, b relmanifest.PayloadFile) int {
		return strings.Compare(a.Path, b.Path)
	}) {
		t.Fatalf("files not sorted by path: %+v", m1.Payload.Files)
	}
	b1, err := m1.Encode()
	if err != nil {
		t.Fatalf("encode 1: %v", err)
	}
	b2, err := m2.Encode()
	if err != nil {
		t.Fatalf("encode 2: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatal("encodings diverge across identical walks")
	}
}

// A non-regular payload entry (here a symlink) refuses by name — the packed
// payload carries regular files only.
func TestWalkRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, ".pyxis")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.md", filepath.Join(sub, "link.md")); err != nil {
		t.Fatal(err)
	}
	policy := relmanifest.ClassPolicy{
		SchemaVersion: relmanifest.PolicySchemaVersion,
		Classes:       []relmanifest.ClassRule{{Glob: ".pyxis/**", Class: relmanifest.EngineVerbatim}},
	}
	_, err := relmanifest.Walk(dir, policy, goldenMeta())
	if err == nil || !strings.Contains(err.Error(), `".pyxis/link.md" is not a regular file`) {
		t.Fatalf("symlink not refused by name: %v", err)
	}
}

// Walk fails closed when the release-level meta cannot yield a valid
// manifest — the derived object is validated before it is returned.
func TestWalkRefusesInvalidMeta(t *testing.T) {
	policy := loadFixturePolicy(t)
	_, err := relmanifest.Walk(filepath.Join("testdata", "payload-real-shaped"), policy, relmanifest.Meta{})
	if err == nil || !strings.Contains(err.Error(), "system_version is empty") {
		t.Fatalf("empty meta not refused by name: %v", err)
	}
	if !strings.Contains(err.Error(), "derived manifest") {
		t.Fatalf("refusal lacks the derived-manifest context: %v", err)
	}
}
