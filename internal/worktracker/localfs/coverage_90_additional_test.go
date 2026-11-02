package localfs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func TestCoverage90GrammarRefusals(t *testing.T) {
	var reasons []string
	bad := func(_, reason, _ string) { reasons = append(reasons, reason) }
	parseLog([]string{
		"not an entry",
		"- comment · too few",
		"- comment · author · not-a-time · none · payload",
	}, 0, bad)
	if len(reasons) != 3 {
		t.Fatalf("log parse reasons = %v", reasons)
	}
	if _, defects := Parse("show", "fixture.md", []byte("---\nid: 1\n")); len(defects) == 0 {
		t.Fatal("unterminated header parsed clean")
	}
	brokenHeader := strings.Join([]string{
		"---",
		"",
		"not a pair",
		"id: 1",
		"id: 2",
		"---",
		"body without a log marker",
	}, "\n")
	if _, defects := Parse("show", "fixture.md", []byte(brokenHeader)); len(defects) < 2 {
		t.Fatalf("broken header defects = %v", defects)
	}
}

func TestCoverage90StoreReadAndValidationRefusals(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	if _, defects := s.Get(t.Context(), 0); len(defects) == 0 {
		t.Fatal("non-positive id resolved")
	}
	if _, defects := s.Create(t.Context(), model.CreateSpec{Title: "bad enums", Type: model.Type("bogus"), Priority: model.Priority("bogus")}); len(defects) < 2 {
		t.Fatalf("bad create enums defects = %v", defects)
	}
	w, defects := s.Create(t.Context(), model.CreateSpec{Title: "mismatched header"})
	if len(defects) > 0 {
		t.Fatal(defects)
	}
	raw, err := os.ReadFile(itemPath(s.itemsDir(), w.ID))
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "id: 1", "id: 2", 1))
	if err := os.WriteFile(itemPath(s.itemsDir(), w.ID), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, defects := s.Get(t.Context(), w.ID); len(defects) == 0 || !strings.Contains(defects[0].Reason, "does not match") {
		t.Fatalf("mismatched header defects = %v", defects)
	}
}

func TestCoverage90DependencyAndQueryDirectoryBoundaries(t *testing.T) {
	ctx := context.Background()
	s := New(t.TempDir(), fixedClock())
	if _, defects := s.RemoveBlockedBy(ctx, 999, "1"); len(defects) == 0 {
		t.Fatal("remove dependency from missing item succeeded")
	}
	if got, defects := s.Blocking(ctx, 1); len(got) != 0 || len(defects) != 0 {
		t.Fatalf("blocking on absent store = %v, %v", got, defects)
	}
	if got, defects := s.List(ctx, model.Query{}); len(got) != 0 || len(defects) != 0 {
		t.Fatalf("list on absent store = %v, %v", got, defects)
	}

	blocked := New(t.TempDir(), fixedClock())
	if err := os.WriteFile(blocked.itemsDir(), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, defects := blocked.Blocking(ctx, 1); len(defects) == 0 {
		t.Fatal("blocking scan through a file succeeded")
	}
	if _, defects := blocked.Search(ctx, "needle"); len(defects) == 0 {
		t.Fatal("search scan through a file succeeded")
	}
}

func TestCoverage90DirtyIndexBoundaries(t *testing.T) {
	if !New(t.TempDir(), fixedClock()).dirty() {
		t.Fatal("store without a freshness stamp reported clean")
	}
	s := New(t.TempDir(), fixedClock())
	if err := os.MkdirAll(s.revDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.stampPath(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if s.dirty() {
		t.Fatal("store with a stamp and no items reported dirty")
	}
	if err := os.MkdirAll(filepath.Join(s.itemsDir(), "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(s.stampPath(), future, future); err != nil {
		t.Fatal(err)
	}
	if s.dirty() {
		t.Fatal("directory-only item tree reported dirty")
	}
}

func TestCoverage90SignalFilesystemAndValidationBoundaries(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	if defects := s.DefineSignal(t.Context(), model.Signal{Role: model.RoleClassification}); len(defects) == 0 {
		t.Fatal("empty signal name succeeded")
	}
	if defects := s.DefineSignal(t.Context(), model.Signal{Name: "kind:test", Role: model.Role("bogus")}); len(defects) == 0 {
		t.Fatal("invalid signal role succeeded")
	}
	if err := os.MkdirAll(s.vocabPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, defects := s.readExtensions(); len(defects) == 0 {
		t.Fatal("directory vocabulary read cleanly")
	}

	rootFile := filepath.Join(t.TempDir(), "root-file")
	if err := os.WriteFile(rootFile, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if defects := New(rootFile, fixedClock()).writeVocab("signal", nil); len(defects) == 0 {
		t.Fatal("vocabulary write through file root succeeded")
	}
}

func TestCoverage90LockAndMaxIDOpenFailures(t *testing.T) {
	if New(t.TempDir(), nil).now == nil {
		t.Fatal("nil clock was not defaulted")
	}
	s := New(t.TempDir(), fixedClock())
	if err := os.MkdirAll(filepath.Join(s.lockDir(), "1.lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.withItemLock(1, func() error { return nil }); err == nil {
		t.Fatal("directory lock file opened")
	}
	if err := os.WriteFile(s.itemsDir(), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.maxID(); err == nil {
		t.Fatal("maxID scanned through a file")
	}
	gitignoreRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(gitignoreRoot, ".gitignore"), []byte("existing-rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	New(gitignoreRoot, fixedClock()).writeRevGitignore()
	body, err := os.ReadFile(filepath.Join(gitignoreRoot, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "existing-rule\n") || !strings.Contains(string(body), "rev/") {
		t.Fatalf("gitignore append = %q", body)
	}
}
