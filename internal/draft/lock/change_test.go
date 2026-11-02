package lock

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

func TestChangedTasksNamesEditedAddedAndRemovedTasksInOrder(t *testing.T) {
	before := []byte(`{"tasks":[{"id":"T1","title":"a","goal":"g","acceptance":["x"]},{"id":"T2","title":"b","goal":"g","acceptance":["x"]},{"id":"T3","title":"c","goal":"g","acceptance":["x"]}]}`)
	after := []byte(`{"tasks":[{"id":"T1","title":"a","goal":"g","acceptance":["x"]},{"id":"T2","title":"b","goal":"changed","acceptance":["x"]},{"id":"T10","title":"d","goal":"g","acceptance":["x"]}]}`)
	got, err := changedTasks(before, after)
	if err != nil || !slices.Equal(got, []string{"T2", "T3 (removed)", "T10"}) {
		t.Fatalf("changed = %v, err = %v", got, err)
	}
	if all, err := changedTasks(nil, after); err != nil || len(all) != 3 {
		t.Fatalf("a new plan's tasks = %v, err = %v", all, err)
	}
	for _, bad := range [][2][]byte{{[]byte("{"), after}, {before, []byte("{")}} {
		if _, err := changedTasks(bad[0], bad[1]); err == nil {
			t.Fatal("malformed plan face accepted")
		}
	}
	if err := keepsLivePlan(OtherChanges{Tasks: got}); err == nil || !strings.Contains(err.Error(), "removes T3") {
		t.Fatalf("dropped task accepted: %v", err)
	}
	if err := keepsLivePlan(OtherChanges{Tasks: []string{"T2"}}); err != nil {
		t.Fatal(err)
	}
}

func TestPlanUnfitErrorNamesEachDefect(t *testing.T) {
	err := &planUnfitError{defects: []schema.Defect{{File: "a.md", Reason: "one"}, {File: "b.md", Reason: "two"}}}
	if msg := err.Error(); !strings.Contains(msg, "a.md: one; b.md: two") {
		t.Fatalf("message = %s", msg)
	}
}

func TestRebasePreparedPlanStopsAtEachFailedStep(t *testing.T) {
	boom := errors.New("boom")
	// failing is a draft whose plan the draft edited; the git step whose
	// arguments start with match fails, and "" fails none.
	failing := func(match string) func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
			joined := strings.Join(args, " ")
			switch {
			case match != "" && strings.HasPrefix(joined, match):
				return nil, boom
			case name == "git" && args[0] == "diff":
				return []byte("docs/plans/widget/001-plan.md\n"), nil
			case name == "git" && args[0] == "rev-parse":
				return []byte(strings.Repeat("a", 40) + "\n"), nil
			}
			return nil, nil
		}
	}
	edited := failing("")
	changed := func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
		return []string{"docs/plans/widget/001-plan.md"}, nil, nil
	}
	t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
	for _, tc := range []struct {
		name       string
		run        func(context.Context, string, []string, string, ...string) ([]byte, error)
		rebase     func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error)
		unfinished func(context.Context) ([]string, error)
		want       string
	}{
		{"diff", failing("diff"), changed, nil, "inspect the draft's plan changes"},
		{"unknown task state counts as in use", func(context.Context, string, []string, string, ...string) ([]byte, error) { return nil, nil }, func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
			return nil, nil, boom
		}, nil, "rebase live plan onto lock/widget/v2"},
		{"unfinished", func(context.Context, string, []string, string, ...string) ([]byte, error) { return nil, nil }, changed,
			func(context.Context) ([]string, error) { return nil, boom }, "unfinished tasks"},
		{"head", failing("rev-parse"), changed, nil, "resolve preparation commit"},
		{"rebase", edited, func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
			return nil, nil, boom
		}, nil, "rebase live plan onto lock/widget/v2"},
		{"stage", failing("add"), changed, nil, "stage rebased plan"},
		{"amend", failing("commit"), changed, nil, "add rebased plan to the preparation commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rebaseLivePlan = tc.rebase
			err := rebasePreparedPlan(t.Context(), t.TempDir(), config.Defaults(), "widget", 2, "base", tc.run, nil, tc.unfinished, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRebasePreparedPlanRefusesAnInUsePlanOnAppendOnlyTasks(t *testing.T) {
	quiet := func(context.Context, string, []string, string, ...string) ([]byte, error) { return nil, nil }
	moved := false
	rebaseLivePlan = func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
		moved = true
		return nil, nil, nil
	}
	t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
	running := func(context.Context) ([]string, error) { return []string{"T1 (item #3 open)"}, nil }
	if err := rebasePreparedPlan(t.Context(), t.TempDir(), config.Defaults(), "widget", 2, "base", quiet, nil, running, true); !errors.Is(err, errAppendOnlyTasks) || moved {
		t.Fatalf("running append-only plan: err=%v moved=%v", err, moved)
	}
	finished := func(context.Context) ([]string, error) { return nil, nil }
	if err := rebasePreparedPlan(t.Context(), t.TempDir(), config.Defaults(), "widget", 2, "base", quiet, nil, finished, true); err != nil || moved {
		t.Fatalf("finished append-only plan: err=%v moved=%v", err, moved)
	}
}

func TestRebasePreparedPlanRefusesEditingAFinishedPlan(t *testing.T) {
	edits := func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "diff" {
			return []byte("docs/plans/widget/001-plan.md\n"), nil
		}
		return nil, nil
	}
	moved := false
	rebaseLivePlan = func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
		moved = true
		return nil, nil, nil
	}
	t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
	finished := func(context.Context) ([]string, error) { return nil, nil }
	if err := rebasePreparedPlan(t.Context(), t.TempDir(), config.Defaults(), "widget", 2, "base", edits, nil, finished, false); !errors.Is(err, errFinishedPlan) || moved {
		t.Fatalf("edited finished plan: err=%v moved=%v", err, moved)
	}
	running := func(context.Context) ([]string, error) { return []string{"T1 (issue #5 open)"}, nil }
	if err := rebasePreparedPlan(t.Context(), t.TempDir(), config.Defaults(), "widget", 2, "base", edits, nil, running, false); err != nil || !moved {
		t.Fatalf("edited running plan: err=%v moved=%v", err, moved)
	}
}

func TestRebasePreparedPlanCountsOnlyPlanFacesAsEdits(t *testing.T) {
	evidence := func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "diff" {
			return []byte("docs/plans/widget/proof-run-001.md\n"), nil
		}
		return nil, nil
	}
	moved := false
	rebaseLivePlan = func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
		moved = true
		return nil, nil, nil
	}
	t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
	finished := func(context.Context) ([]string, error) { return nil, nil }
	if err := rebasePreparedPlan(t.Context(), t.TempDir(), config.Defaults(), "widget", 2, "base", evidence, nil, finished, false); err != nil || moved {
		t.Fatalf("an evidence file blocked or moved a finished plan: err=%v moved=%v", err, moved)
	}
}

func TestOtherChangesStopsAtEachFailedRead(t *testing.T) {
	boom := errors.New("boom")
	for _, tc := range []struct {
		name string
		run  func(context.Context, string, []string, string, ...string) ([]byte, error)
		want string
	}{
		{"diff", func(context.Context, string, []string, string, ...string) ([]byte, error) { return nil, boom }, "list proposal changes"},
		{"base plan", func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
			switch args[0] {
			case "diff":
				return []byte("M\tdocs/plans/widget/001-plan.json\n"), nil
			case "ls-tree":
				return []byte("docs/plans/widget/001-plan.json\n"), nil
			}
			return nil, boom
		}, "read the base plan"},
		{"proposed plan", func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
			switch {
			case args[0] == "diff":
				return []byte("not a status line\nM\tdocs/plans/widget/001-plan.json\n"), nil
			case args[0] == "ls-tree":
				return []byte("docs/plans/widget/001-plan.json\n"), nil
			case args[0] == "show" && strings.HasPrefix(args[1], "base:"):
				return []byte(`{"tasks":[]}`), nil
			}
			return nil, boom
		}, "read proposed plan"},
		{"malformed plan", func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
			switch args[0] {
			case "diff":
				return []byte("M\tdocs/plans/widget/001-plan.json\n"), nil
			case "ls-tree":
				return []byte("docs/plans/widget/001-plan.json\n"), nil
			}
			return []byte("{"), nil
		}, "compare plan"},
		{"live plans", func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
			if args[0] == "diff" {
				return []byte("A\tdocs/plans/widget/002-plan.json\n"), nil
			}
			return nil, boom
		}, "list the live plans"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := otherChanges(t.Context(), t.TempDir(), "base", "head", "widget", "docs/plans", tc.run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestOtherChangesNamesAPlanVersionChange(t *testing.T) {
	face := []byte(`{"tasks":[{"id":"T1","title":"a","goal":"g","acceptance":["x"]}]}`)
	host := func(diff, live string) func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
			switch args[0] {
			case "diff":
				return []byte(diff), nil
			case "ls-tree":
				return []byte(live), nil
			case "show":
				if file, atBase := strings.CutPrefix(args[1], "base:"); atBase && !strings.Contains(live, file+"\n") {
					return nil, errors.New("absent at base")
				}
				return face, nil
			}
			return nil, errors.New("unexpected " + args[0])
		}
	}
	for _, tc := range []struct {
		name, diff, live, want string
	}{
		{"new version", "A\tdocs/plans/widget/002-plan.json\nA\tdocs/plans/widget/002-plan.md\n", "docs/plans/widget/001-plan.json\ndocs/plans/widget/001-plan.md\n", "adds docs/plans/widget/002-plan.md beside the live plan docs/plans/widget/001-plan.md"},
		{"removed plan", "D\tdocs/plans/widget/001-plan.json\n", "", "removes docs/plans/widget/001-plan.md"},
		{"removed source only", "D\tdocs/plans/widget/001-plan.md\n", "", "removes docs/plans/widget/001-plan.md"},
		{"new source only", "A\tdocs/plans/widget/002-plan.md\n", "docs/plans/widget/001-plan.json\ndocs/plans/widget/001-plan.md\n", "adds docs/plans/widget/002-plan.md beside the live plan docs/plans/widget/001-plan.md"},
		{"source beside its own face", "A\tdocs/plans/widget/001-plan.md\n", "docs/plans/widget/001-plan.json\n", ""},
		{"a plan source turned into a gitlink", "T\tdocs/plans/widget/001-plan.md\n", "docs/plans/widget/001-plan.json\ndocs/plans/widget/001-plan.md\n", "changes the file type of docs/plans/widget/001-plan.md"},
		{"a plan face outside the plan directory", "M\tarchive/widget/001-plan.json\nD\tarchive/widget/002-plan.md\n", "docs/plans/widget/001-plan.json\n", ""},
		{"earlier plan source", "M\tdocs/plans/widget/001-plan.md\n", "docs/plans/widget/001-plan.json\ndocs/plans/widget/001-plan.md\ndocs/plans/widget/002-plan.json\ndocs/plans/widget/002-plan.md\n", "changes the earlier plan docs/plans/widget/001-plan.md; only the live plan docs/plans/widget/002-plan.md changes in place"},
		{"earlier plan face", "M\tdocs/plans/widget/001-plan.json\n", "docs/plans/widget/001-plan.json\ndocs/plans/widget/002-plan.json\n", "changes the earlier plan docs/plans/widget/001-plan.md; only the live plan docs/plans/widget/002-plan.md changes in place"},
		{"live plan in place", "M\tdocs/plans/widget/002-plan.json\nM\tdocs/plans/widget/002-plan.md\n", "docs/plans/widget/001-plan.json\ndocs/plans/widget/002-plan.json\n", ""},
		{"first plan", "A\tdocs/plans/widget/001-plan.json\n", "docs/plans/widget/README.md\n", "adds the first plan docs/plans/widget/001-plan.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other, err := otherChanges(t.Context(), t.TempDir(), "base", "head", "widget", "docs/plans", host(tc.diff, tc.live))
			if err != nil || other.Version != tc.want {
				t.Fatalf("version = %q, err = %v, want %q", other.Version, err, tc.want)
			}
			if refused := keepsLivePlan(other); (refused != nil) != (tc.want != "") || (refused != nil && !strings.Contains(refused.Error(), "keeps its version")) {
				t.Fatalf("keepsLivePlan = %v", refused)
			}
		})
	}
}
