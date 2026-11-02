package localfs

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// A fresh store surfaces the full seed vocabulary with no file on disk (the
// governed seed is code, not a materialized file — §3.5, AP-24).
func TestSeedVocabularyWithoutFile(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	sigs, defects := s.Signals(context.Background())
	if len(defects) > 0 {
		t.Fatalf("signals: %v", defects)
	}
	if len(sigs) != len(model.SeedSignals()) {
		t.Fatalf("fresh store vocabulary size = %d, want %d", len(sigs), len(model.SeedSignals()))
	}
	if _, err := os.Stat(s.vocabPath()); !os.IsNotExist(err) {
		t.Fatalf("reading the vocabulary wrote a file (write-on-read): %v", err)
	}
}

// Defining a classification signal writes only the extension to disk and merges
// it with the seed on read; the seed is never written to the file.
func TestDefineWritesExtensionOnly(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	if defects := s.DefineSignal(ctx, model.Signal{Name: "area:api", Role: model.RoleClassification, Color: "#000", Description: "the API surface"}); len(defects) > 0 {
		t.Fatalf("define: %v", defects)
	}
	raw, err := os.ReadFile(s.vocabPath())
	if err != nil {
		t.Fatalf("read vocab: %v", err)
	}
	if !strings.Contains(string(raw), "area:api") {
		t.Fatalf("extension not written: %s", raw)
	}
	if strings.Contains(string(raw), "agent") {
		t.Fatalf("seed signal leaked into the extension file: %s", raw)
	}
	// Re-defining the identical signal is a recorded no-op (no error).
	if defects := s.DefineSignal(ctx, model.Signal{Name: "area:api", Role: model.RoleClassification, Color: "#000", Description: "the API surface"}); len(defects) > 0 {
		t.Fatalf("identical re-define reported defects: %v", defects)
	}
}

func TestDefineRefusesNonClassification(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	defects := s.DefineSignal(context.Background(), model.Signal{Name: "kind:x", Role: model.RoleRouting})
	if len(defects) == 0 || !strings.Contains(defects[0].Reason, "classification") {
		t.Fatalf("non-classification def not refused: %v", defects)
	}
}

func TestDefineRefusesUnnamespacedClassification(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	defects := s.DefineSignal(context.Background(), model.Signal{Name: "loose", Role: model.RoleClassification})
	if len(defects) == 0 || !strings.Contains(defects[0].Reason, "kind:") {
		t.Fatalf("unnamespaced classification def not refused: %v", defects)
	}
}

// A malformed hand-edited vocabulary file fails closed on read, naming the file
// and the offending row (§3.13, DINV-11).
func TestVocabularyFailsClosedOnBadRow(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := renderVocab([]model.Signal{{Name: "kind:ok", Role: model.RoleClassification}})
	// Corrupt the row to a non-classification role.
	corrupt := strings.Replace(string(bad), "| kind:ok | classification |", "| kind:ok | routing |", 1)
	if err := os.WriteFile(s.vocabPath(), []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	_, defects := s.Signals(context.Background())
	if len(defects) == 0 {
		t.Fatalf("malformed vocabulary parsed clean")
	}
	d := defects[0]
	if d.File != s.vocabPath() || d.Reason == "" || d.Remedy == "" {
		t.Fatalf("vocab defect does not name file/reason/remedy: %+v", d)
	}
}

// parseVocab skips the header and separator rows and reads the data rows.
func TestParseVocabSkipsHeaderAndSeparator(t *testing.T) {
	in := renderVocab([]model.Signal{
		{Name: "kind:sec", Role: model.RoleClassification, Color: "#111", Description: "security"},
		{Name: "area:ui", Role: model.RoleClassification, Color: "#222", Description: "the ui"},
	})
	got, defects := parseVocab("signals.md", in)
	if len(defects) > 0 {
		t.Fatalf("clean vocab reported defects: %v", defects)
	}
	if len(got) != 2 || got[0].Name != "kind:sec" || got[1].Name != "area:ui" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
