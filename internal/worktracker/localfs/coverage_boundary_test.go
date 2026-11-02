package localfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func TestWriteHelpersFailClosedAtFilesystemBoundaries(t *testing.T) {
	root := t.TempDir()
	s := New(root, fixedClock())
	missing := filepath.Join(root, "missing")

	if _, _, err := s.createTemp(missing); err == nil {
		t.Fatal("createTemp accepted an absent directory")
	}
	if err := syncDir(missing); err == nil {
		t.Fatal("syncDir accepted an absent directory")
	}
	if _, err := s.reserveID(1); err == nil {
		t.Fatal("reserveID accepted an absent items directory")
	}
	if max, err := s.maxID(); err != nil || max != 0 {
		t.Fatalf("maxID on an absent store = %d, %v", max, err)
	}
	s.sweepTemps() // An absent items directory is an intentional no-op.

	if processAlive(0) {
		t.Fatal("non-positive pid reported alive")
	}
	if _, ok := idFromName("0.md"); ok {
		t.Fatal("non-positive item id parsed")
	}

	target := filepath.Join(root, "target-directory")
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.atomicReplace(target, []byte("replacement"), nil); err == nil {
		t.Fatal("atomicReplace overwrote a directory")
	}
}

func TestWriteHelpersHandleCollisionsAndDirectoryEntries(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	if err := os.MkdirAll(filepath.Join(s.itemsDir(), "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(itemPath(s.itemsDir(), 1), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := s.reserveID(1); err != nil || got != 2 {
		t.Fatalf("reserve after collision = %d, %v", got, err)
	}
	if max, err := s.maxID(); err != nil || max != 2 {
		t.Fatalf("maxID with directory entry = %d, %v", max, err)
	}
}

func TestChildrenFiltersAndQuarantinesBoundaryRows(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	ctx := context.Background()
	parent, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "parent"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
	if _, defects := s.SetParent(ctx, child.ID, parent.ID); len(defects) > 0 {
		t.Fatalf("set parent: %v", defects)
	}

	invalid := []worktracker.ChildFilter{
		{State: model.State("unknown")},
		{Role: model.Role("unknown")},
	}
	for _, filter := range invalid {
		if got, defects := s.Children(ctx, parent.ID, filter, worktracker.Display); got != nil || len(defects) != 1 {
			t.Fatalf("invalid filter %+v = %v, %v", filter, got, defects)
		}
	}

	filters := []worktracker.ChildFilter{
		{State: model.StateClosed},
		{Signal: "missing"},
		{Role: model.RoleClassification},
	}
	for _, filter := range filters {
		if got, defects := s.Children(ctx, parent.ID, filter, worktracker.Display); len(got) != 0 || len(defects) != 0 {
			t.Fatalf("non-matching filter %+v = %v, %v", filter, got, defects)
		}
	}

	if err := os.WriteFile(s.markerPath(strconv.Itoa(parent.ID), 999), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, defects := s.Children(ctx, parent.ID, worktracker.ChildFilter{}, worktracker.Display); len(got) != 1 || len(defects) != 0 {
		t.Fatalf("quarantined missing child = %v, %v", got, defects)
	}
}

func TestHierarchyPropagatesIndexAndVocabularyDefects(t *testing.T) {
	ctx := context.Background()
	s := New(t.TempDir(), fixedClock())
	if err := os.MkdirAll(filepath.Dir(s.parentDir("1")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.parentDir("1"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, defects := s.Rollup(ctx, 1, worktracker.Display); len(defects) != 1 {
		t.Fatalf("rollup index defect = %v", defects)
	}

	s = New(t.TempDir(), fixedClock())
	parent, _ := s.Create(ctx, model.CreateSpec{Type: model.TypeEpic, Title: "parent"})
	child, _ := s.Create(ctx, model.CreateSpec{Title: "child"})
	if _, defects := s.SetParent(ctx, child.ID, parent.ID); len(defects) > 0 {
		t.Fatalf("set parent: %v", defects)
	}
	raw := renderVocab([]model.Signal{{Name: "kind:test", Role: model.RoleClassification}})
	corrupt := strings.Replace(string(raw), "| kind:test | classification |", "| kind:test | routing |", 1)
	if err := os.WriteFile(s.vocabPath(), []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, defects := s.Children(ctx, parent.ID, worktracker.ChildFilter{Role: model.RoleClassification}, worktracker.Display); got != nil || len(defects) == 0 {
		t.Fatalf("vocabulary defect = %v, %v", got, defects)
	}
}
