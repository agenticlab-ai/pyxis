package ghtracker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
)

func TestSelectedCredentialsReachTrackerAndPlanScope(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n[ \"$GH_HOST\" = github.com ] && [ \"$GH_TOKEN\" = task-actor ] || exit 2\nprintf task-actor\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TEST_SELECTED_TASK", "task-actor")
	t.Setenv("GH_TOKEN", "ambient")
	t.Setenv("GH_HOST", "wrong.example")
	for _, b := range []*budget.Budget{nil, {}} {
		tracker := NewGHWithTokenEnv("o/tasks", "ready", "TEST_SELECTED_TASK", b, "github.com")
		c := tracker.gh.(*ghClient)
		if c.budget == nil || c.budgetErr != nil || (b != nil && c.budget != b) {
			t.Fatalf("selected task budget was not retained: %v", c.budgetErr)
		}
		if out, err := c.run(t.Context(), "api", "user"); err != nil || string(out) != "task-actor" {
			t.Fatalf("selected tracker request %s %v", out, err)
		}
	}
	plan, err := NewGHForPlanWithTokenEnv("o/tasks", "ready", "feature/001", "TEST_SELECTED_TASK", &budget.Budget{}, "github.com")
	if err != nil || plan.planFeature != "feature" || plan.planVersion != "001" {
		t.Fatalf("selected plan scope %v", err)
	}
	if _, err := NewGHForPlanWithTokenEnv("o/tasks", "ready", "invalid", "TEST_SELECTED_TASK", &budget.Budget{}); err == nil {
		t.Fatal("invalid plan scope accepted")
	}
	for _, b := range []*budget.Budget{nil, {}} {
		if c := clientWithTokenEnv("o/tasks", "", b); b != nil && c.budget != b {
			t.Fatal("legacy tracker budget changed")
		}
	}
}
