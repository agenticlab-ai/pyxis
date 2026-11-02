package relmanifest_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// validManifest builds a schema-valid manifest for mutation in defect tests.
func validManifest() relmanifest.Manifest {
	h := strings.Repeat("ab", 32)
	return relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: "v1.2.3",
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{"docgate": {"darwin-arm64": h}},
		Payload: relmanifest.Payload{
			ArchiveSHA256: h,
			Files: []relmanifest.PayloadFile{
				{Path: ".pyxis/config.yaml", Class: relmanifest.RepoOwned},
				{Path: ".pyxis/pipeline.md", SHA256: &h, Class: relmanifest.EngineVerbatim},
				{Path: "docs/tech-design/index.json", Class: relmanifest.Generated},
			},
		},
		DepsBundleSHA256: h,
	}
}

// The committed golden parses, re-encodes to the identical bytes, and the
// re-encoded form parses back to the identical value — byte-stable both ways.
func TestGoldenManifestRoundTrip(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "golden-manifest.json"))
	if err != nil {
		t.Fatalf("golden missing (run: go test ./internal/relmanifest/ -run TestWalkDerivesGoldenManifest -update): %v", err)
	}
	m, err := relmanifest.ParseManifest(golden)
	if err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	out, err := m.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.Equal(out, golden) {
		t.Fatalf("round trip not byte-stable:\n--- golden ---\n%s\n--- re-encoded ---\n%s", golden, out)
	}
	m2, err := relmanifest.ParseManifest(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(m, m2) {
		t.Fatalf("value round trip diverged:\n%+v\n%+v", m, m2)
	}
}

func TestManifestWithoutDependencyBundle(t *testing.T) {
	m := validManifest()
	if m.SchemaVersion != "release-manifest/v2" {
		t.Fatalf("new manifests must use v2, got %q", m.SchemaVersion)
	}
	m.DepsBundleSHA256 = ""
	b, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("deps_bundle_sha256")) {
		t.Fatal("new manifest still serializes a dependency bundle")
	}
	got, err := relmanifest.ParseManifest(b)
	if err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("manifest without dependency bundle: %+v, %v", got, err)
	}
}

func TestLegacyManifestRequiresDependencyBundle(t *testing.T) {
	m := validManifest()
	m.SchemaVersion = "release-manifest/v1"
	b, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := relmanifest.ParseManifest(b)
	if err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("legacy manifest round trip: %+v, %v", got, err)
	}
	out, err := got.Encode()
	if err != nil || !bytes.Equal(out, b) {
		t.Fatalf("legacy canonical bytes changed: %v", err)
	}
	m.DepsBundleSHA256 = ""
	if _, err := m.Encode(); err == nil || !strings.Contains(err.Error(), "deps_bundle_sha256") {
		t.Fatalf("v1 accepted an omitted dependency-bundle hash: %v", err)
	}
	m.SchemaVersion = relmanifest.ManifestSchemaVersion
	b, err = m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.Replace(b, []byte("release-manifest/v2"), []byte("release-manifest/v1"), 1)
	if _, err := relmanifest.ParseManifest(b); err == nil || !strings.Contains(err.Error(), "deps_bundle_sha256") {
		t.Fatalf("v1 parser accepted an omitted dependency-bundle hash: %v", err)
	}
}

// Unknown keys refuse by name at every nesting level — the closed parse
// (DINV-2). Fixture files carry the charter's two canonical cases.
func TestParseManifestUnknownKeys(t *testing.T) {
	cases := []struct {
		fixture string
		want    string
	}{
		{"manifest-unknown-key.json", `unknown field "channel"`},
		{"manifest-unknown-nested-key.json", `unknown field "mode"`},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", c.fixture))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			_, err = relmanifest.ParseManifest(b)
			if err == nil {
				t.Fatalf("no error; want one containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err, c.want)
			}
		})
	}
}

// Schema-invariant violations refuse by name through the shared validation:
// Encode is the exported face (it validates before rendering, fail closed).
func TestManifestInvariantsRefuseByName(t *testing.T) {
	h := strings.Repeat("ab", 32)
	cases := []struct {
		name   string
		mutate func(*relmanifest.Manifest)
		want   string
	}{
		{"wrong schema_version", func(m *relmanifest.Manifest) { m.SchemaVersion = "release-manifest/v3" },
			`schema_version "release-manifest/v3"`},
		{"empty system_version", func(m *relmanifest.Manifest) { m.SystemVersion = "" },
			"system_version is empty"},
		{"empty source_commit", func(m *relmanifest.Manifest) { m.SourceCommit = "" },
			"source_commit is empty"},
		{"bad archive hash", func(m *relmanifest.Manifest) { m.Payload.ArchiveSHA256 = "zz" },
			"payload.archive_sha256"},
		{"bad deps bundle hash", func(m *relmanifest.Manifest) { m.DepsBundleSHA256 = strings.Repeat("G", 64) },
			"deps_bundle_sha256"},
		{"bad binary hash", func(m *relmanifest.Manifest) { m.Binaries["docgate"]["darwin-arm64"] = "short" },
			`binaries["docgate"]["darwin-arm64"]`},
		{"tool with no platforms", func(m *relmanifest.Manifest) { m.Binaries["pyxis"] = map[string]string{} },
			`binaries["pyxis"]: no platform entries`},
		{"empty files", func(m *relmanifest.Manifest) { m.Payload.Files = nil },
			"payload.files is empty"},
		{"unknown class", func(m *relmanifest.Manifest) { m.Payload.Files[0].Class = "engine-owned" },
			`unknown ownership class "engine-owned"`},
		{"engine-verbatim null hash", func(m *relmanifest.Manifest) { m.Payload.Files[1].SHA256 = nil },
			`payload file ".pyxis/pipeline.md": class "engine-verbatim" requires a sha256`},
		{"repo-owned with hash", func(m *relmanifest.Manifest) { m.Payload.Files[0].SHA256 = &h },
			`payload file ".pyxis/config.yaml": class "repo-owned" must carry sha256 null`},
		{"generated with hash", func(m *relmanifest.Manifest) { m.Payload.Files[2].SHA256 = &h },
			`class "generated" must carry sha256 null`},
		{"duplicate path", func(m *relmanifest.Manifest) {
			m.Payload.Files = append(m.Payload.Files, m.Payload.Files[0])
		}, `duplicate payload path ".pyxis/config.yaml"`},
		{"empty path", func(m *relmanifest.Manifest) { m.Payload.Files[0].Path = "" },
			"empty path"},
		{"absolute path", func(m *relmanifest.Manifest) { m.Payload.Files[0].Path = "/etc/passwd" },
			`payload path "/etc/passwd"`},
		{"dot-dot path", func(m *relmanifest.Manifest) { m.Payload.Files[0].Path = "../escape.md" },
			`payload path "../escape.md"`},
		{"unclean path", func(m *relmanifest.Manifest) { m.Payload.Files[0].Path = ".pyxis/./config.yaml" },
			"not a clean, relative, slash-separated path"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := validManifest()
			c.mutate(&m)
			_, err := m.Encode()
			if err == nil {
				t.Fatalf("no error; want one containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err, c.want)
			}
		})
	}
	// The unmutated manifest encodes clean — the table's baseline.
	if _, err := validManifest().Encode(); err != nil {
		t.Fatalf("valid manifest refused: %v", err)
	}
}

// Trailing content after the manifest object is a defect (one object, closed).
func TestParseManifestTrailingContent(t *testing.T) {
	m := validManifest()
	b, err := m.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, err = relmanifest.ParseManifest(append(b, []byte("{}")...))
	if err == nil || !strings.Contains(err.Error(), "trailing content") {
		t.Fatalf("trailing content not refused: %v", err)
	}
}
