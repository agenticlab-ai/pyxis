package ghtracker

import (
	"strings"
	"testing"
)

func TestNewGHClientBindsProductionDependencies(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())

	c := newGHClient("owner/repo")
	if c.repo != "owner/repo" || c.run == nil || c.sleep == nil || c.budget == nil || c.budgetErr != nil {
		t.Fatalf("newGHClient = %+v, want fully bound production client", c)
	}
}

func TestShellGHNamesMissingExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := shellGH(t.Context(), "api", "user"); err == nil || !strings.Contains(err.Error(), "gh api user") {
		t.Fatalf("shellGH missing executable error = %v", err)
	}
}
