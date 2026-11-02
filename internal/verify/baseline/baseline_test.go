package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParse_Valid(t *testing.T) {
	data := []byte(`{
		"version": "1.0.0",
		"identities": ["pkg::TestA", "pkg::TestB"],
		"provenance": {
			"captured_at": "2026-07-18T00:00:00Z",
			"captured_by": "operator",
			"codegate_version": "1.0.0-dev",
			"verb_output_hash": "deadbeef"
		}
	}`)

	b, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(b.Identities) != 2 {
		t.Errorf("Identities = %v, want 2 entries", b.Identities)
	}
	if b.Provenance.CapturedBy != "operator" {
		t.Errorf("CapturedBy = %q, want %q", b.Provenance.CapturedBy, "operator")
	}
}

// TestParse_EmptyIdentities proves a fully-green capture (zero pre-existing
// failures) is a legitimate, parseable baseline — not a malformed one.
func TestParse_EmptyIdentities(t *testing.T) {
	b, err := Parse([]byte(`{"version":"1.0.0","identities":[],"provenance":{"captured_at":"2026-07-18T00:00:00Z","codegate_version":"1.0.0-dev","verb_output_hash":"x"}}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(b.Identities) != 0 {
		t.Errorf("Identities = %v, want empty", b.Identities)
	}
}

// TestParse_Malformed is UB-5's core case: a baseline that is syntactically
// valid JSON but does not conform to the schema must fail closed, exactly
// like invalid JSON or a missing file does one layer up (invoke's bindings
// resolution).
func TestParse_Malformed(t *testing.T) {
	tests := map[string]string{
		"not json at all":       `not json at all`,
		"empty object":          `{}`,
		"unknown field":         `{"version":"1.0.0","identities":[],"provenance":{"captured_at":"2026-07-18T00:00:00Z","codegate_version":"x","verb_output_hash":"x"},"extra":"field"}`,
		"identities wrong type": `{"version":"1.0.0","identities":"not-an-array","provenance":{}}`,
		"truncated":             `{"version":"1.0.0","identities":[`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(data)); err == nil {
				t.Errorf("Parse(%q) returned no error, want a malformed-baseline error", data)
			}
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json")); err == nil {
		t.Error("Load of a missing file returned no error")
	}
}

func TestLoad_MalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load malformed file: want parse error")
	}
}

func TestLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	captured := Capture([]string{"pkg::TestB", "pkg::TestA"}, "1.0.0-dev", "operator", "outputhash", time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC))
	if err := captured.WriteAtomicToFile(path); err != nil {
		t.Fatalf("WriteAtomicToFile: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Identities) != 2 {
		t.Fatalf("Identities = %v, want 2 entries", loaded.Identities)
	}
	if loaded.Provenance.CapturedBy != "operator" || loaded.Provenance.CodegateVersion != "1.0.0-dev" || loaded.Provenance.VerbOutputHash != "outputhash" {
		t.Errorf("Provenance = %+v, want it to round-trip", loaded.Provenance)
	}
	if !loaded.Provenance.CapturedAt.Equal(captured.Provenance.CapturedAt) {
		t.Errorf("CapturedAt = %v, want %v", loaded.Provenance.CapturedAt, captured.Provenance.CapturedAt)
	}
}

// TestCapture_NilIdentitiesSerializeAsEmptyArray proves a fully-green
// capture never round-trips through JSON `null` for identities — `null` and
// `[]` are easy to conflate downstream, and this package always writes the
// unambiguous shape.
func TestCapture_NilIdentitiesSerializeAsEmptyArray(t *testing.T) {
	b := Capture(nil, "1.0.0-dev", "operator", "hash", time.Now().UTC())
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(raw["identities"]) != "[]" {
		t.Errorf("identities field = %s, want []", raw["identities"])
	}
}

// TestWriteAtomicToFile_NoPartialFile mirrors schema.TestAtomicWriteNoPartialFile:
// after a successful write, only the final baseline file remains in the
// directory — the temp file used for the atomic rename is always cleaned up.
func TestWriteAtomicToFile_NoPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	b := Capture([]string{"pkg::TestA"}, "1.0.0-dev", "operator", "hash", time.Now().UTC())
	if err := b.WriteAtomicToFile(path); err != nil {
		t.Fatalf("WriteAtomicToFile: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "baseline.json" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("directory contents = %v, want exactly [baseline.json]", names)
	}
}

// TestWriteAtomicToFile_Overwrite proves a second capture cleanly replaces
// the first (the real capture-baseline re-run scenario), never leaving a
// stray temp file or corrupting the target on the rename.
func TestWriteAtomicToFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	first := Capture([]string{"pkg::TestA"}, "1.0.0-dev", "op", "h1", time.Now().UTC())
	if err := first.WriteAtomicToFile(path); err != nil {
		t.Fatalf("first WriteAtomicToFile: %v", err)
	}
	second := Capture([]string{"pkg::TestA", "pkg::TestB"}, "1.0.0-dev", "op", "h2", time.Now().UTC())
	if err := second.WriteAtomicToFile(path); err != nil {
		t.Fatalf("second WriteAtomicToFile: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Identities) != 2 {
		t.Errorf("Identities = %v, want the second capture's 2 entries", loaded.Identities)
	}
}

func TestWriteAtomicToFileReportsDirectoryAndRenameFailures(t *testing.T) {
	b := Capture(nil, "dev", "operator", "hash", time.Now().UTC())

	t.Run("parent is file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "parent")
		if err := os.WriteFile(parent, []byte("occupied"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := b.WriteAtomicToFile(filepath.Join(parent, "baseline.json")); err == nil {
			t.Fatal("WriteAtomicToFile with file parent: want error")
		}
	})

	t.Run("target is directory", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "baseline.json")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "occupied"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := b.WriteAtomicToFile(target); err == nil {
			t.Fatal("WriteAtomicToFile over nonempty directory: want rename error")
		}
	})
}
