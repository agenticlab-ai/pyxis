package adopt_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
)

func rewritePlan(t *testing.T, root string, mutate func(*adopt.Plan)) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(adopt.PlanPath))
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := adopt.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&plan)
	b, err = plan.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCoverage90ApplyRedirectAndHookFailures(t *testing.T) {
	t.Run("redirect without rendered workflow", func(t *testing.T) {
		r := newReleaseWithArchive(t, adoptFiles())
		root := nonEmptyAdopterRepo(t)
		draftPlan(t, root, r)
		rewritePlan(t, root, func(plan *adopt.Plan) {
			plan.Entries = append(plan.Entries, adopt.Entry{Path: adopt.DistinctWorkflowPath(), Class: adopt.ClassGenerated})
		})
		if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err == nil || !strings.Contains(err.Error(), "classification rendered no bytes") {
			t.Fatalf("missing redirected workflow error = %v", err)
		}
	})

	t.Run("hook activation failure", func(t *testing.T) {
		r := newReleaseWithArchive(t, adoptFiles())
		root := nonEmptyAdopterRepo(t)
		draftPlan(t, root, r)
		if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
			t.Fatal(err)
		}
		if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err == nil || !strings.Contains(err.Error(), "hook chain") {
			t.Fatalf("hook activation error = %v", err)
		}
	})
}

func TestCoverage90UndoEntryFailureBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		entry adopt.Entry
		setup func(*testing.T, string, string)
	}{
		{name: "missing add", entry: adopt.Entry{Path: "coverage/missing", Class: adopt.ClassAdd}, setup: func(*testing.T, string, string) {}},
		{name: "unreadable add", entry: adopt.Entry{Path: "coverage/loop", Class: adopt.ClassAdd}, setup: func(t *testing.T, root, rel string) {
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(path, path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "engine removal", entry: adopt.Entry{Path: "coverage/engine", Class: adopt.ClassEngineVerbatim}, setup: nonemptyCoverageDir},
		{name: "generated removal", entry: adopt.Entry{Path: "coverage/generated", Class: adopt.ClassGenerated}, setup: nonemptyCoverageDir},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, r := applyFixture(t)
			tc.setup(t, root, tc.entry.Path)
			rewritePlan(t, root, func(plan *adopt.Plan) {
				plan.Entries = append(plan.Entries, tc.entry)
				if tc.entry.Class == adopt.ClassAdd {
					plan.AddedPaths = append(plan.AddedPaths, adopt.AddedPath{Path: tc.entry.Path, SHA256: strings.Repeat("a", 64)})
				}
			})
			_, err := adopt.Undo(context.Background(), applyOpts(root, r))
			switch tc.name {
			case "missing add":
				if err != nil {
					t.Fatalf("missing add undo = %v", err)
				}
			default:
				if err == nil {
					t.Fatal("undo ignored the injected path failure")
				}
			}
		})
	}
}

func nonemptyCoverageDir(t *testing.T, root, rel string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
