package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := draftshell.Run(t.Context(), root, nil, "git", args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func mergedPlanFixture(t *testing.T) string {
	t.Helper()
	root, _ := gitFixture(t)
	body, err := os.ReadFile("testdata/reopen/001-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(root, "docs/plans/widget/001-plan.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "--", "docs/plans/widget/001-plan.md")
	gitTest(t, root, "commit", "-m", "merge widget plan")
	gitTest(t, root, "push", "origin", "main")
	return root
}

func planIssues(t *testing.T, issues []struct {
	Number int    `json:"number"`
	Body   string `json:"body"`
	State  string `json:"state"`
}) RunCommand {
	t.Helper()
	data, err := json.Marshal(issues)
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			if len(args) > 0 && args[0] == "repo" {
				return []byte("example/repo\n"), nil
			}
			if len(args) > 0 && args[0] == "api" {
				return data, nil
			}
			return nil, fmt.Errorf("unexpected gh command %v", args)
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
}

func TestUnfinishedTasksCountsUnprojectedAndOpenTasks(t *testing.T) {
	root := mergedPlanFixture(t)
	cfg := config.Defaults()
	type issue = struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
		State  string `json:"state"`
	}
	t1 := issue{Number: 17, Body: "<!-- pyxis:plan-task widget/001/T1 -->", State: "closed"}
	t2 := issue{Number: 18, Body: "<!-- pyxis:plan-task widget/001/T2 -->", State: "open"}
	got, err := UnfinishedTasks(t.Context(), root, cfg, "widget", planIssues(t, []issue{t1}))
	if err != nil || len(got) != 1 || got[0] != "T2 (not projected)" {
		t.Fatalf("unprojected=%v error=%v", got, err)
	}
	got, err = UnfinishedTasks(t.Context(), root, cfg, "widget", planIssues(t, []issue{t1, t2}))
	if err != nil || len(got) != 1 || !strings.Contains(got[0], "issue #18 open") {
		t.Fatalf("open=%v error=%v", got, err)
	}
	t2.State = "closed"
	got, err = UnfinishedTasks(t.Context(), root, cfg, "widget", planIssues(t, []issue{t1, t2}))
	if err != nil || len(got) != 0 {
		t.Fatalf("completed=%v error=%v", got, err)
	}
	t2.Number = 19
	if _, err := UnfinishedTasks(t.Context(), root, cfg, "widget", planIssues(t, []issue{t1, t2, {Number: 18, Body: t2.Body, State: "closed"}})); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate issue error=%v", err)
	}
}

func TestUnfinishedTasksAllowsFeatureWithoutPlan(t *testing.T) {
	root, _ := gitFixture(t)
	got, err := UnfinishedTasks(t.Context(), root, config.Defaults(), "widget", planIssues(t, nil))
	if err != nil || len(got) != 0 {
		t.Fatalf("no plan=%v error=%v", got, err)
	}
}

func TestUnfinishedTasksRejectsUnverifiablePlanOrTracker(t *testing.T) {
	root := mergedPlanFixture(t)
	cfg := config.Defaults()
	for _, tc := range []struct{ name, fail, want string }{
		{"plan listing", "ls-tree", "list merged plans"},
		{"plan read", "show", "read current plan"},
		{"invalid plan", "invalid plan", "current plan"},
		{"repository lookup", "repo", "resolve tracker repository"},
		{"invalid repository", "bad repo", "tracker repository"},
		{"issue list", "api", "list tracker issues"},
		{"malformed issue list", "malformed issues", "decode tracker issues"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				if (tc.fail == "ls-tree" && name == "git" && strings.HasPrefix(joined, "ls-tree")) || (tc.fail == "show" && name == "git" && strings.HasPrefix(joined, "show")) || (tc.fail == "repo" && name == "gh" && args[0] == "repo") || (tc.fail == "api" && name == "gh" && args[0] == "api") {
					return nil, errors.New("unavailable")
				}
				if tc.fail == "invalid plan" && name == "git" && args[0] == "show" {
					return []byte("invalid plan"), nil
				}
				if name == "gh" && args[0] == "repo" {
					if tc.fail == "bad repo" {
						return []byte("bad repo"), nil
					}
					return []byte("example/repo"), nil
				}
				if name == "gh" && args[0] == "api" {
					if tc.fail == "malformed issues" {
						return []byte("{"), nil
					}
					return []byte("[]"), nil
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			if _, err := UnfinishedTasks(t.Context(), root, cfg, "widget", run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	if _, err := UnfinishedTasks(t.Context(), root, cfg, "Bad", planIssues(t, nil)); err == nil {
		t.Fatal("invalid feature accepted")
	}
}

func TestUnfinishedTasksReadsNextIssuePage(t *testing.T) {
	root := mergedPlanFixture(t)
	var first []map[string]any
	for i := range 100 {
		first = append(first, map[string]any{"number": i + 100, "body": "other issue", "state": "open"})
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON := []byte(`[{"number":17,"body":"<!-- pyxis:plan-task widget/001/T1 -->","state":"closed"},{"number":18,"body":"<!-- pyxis:plan-task widget/001/T2 -->","state":"closed"}]`)
	pages := 0
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" && args[0] == "repo" {
			return []byte("example/repo"), nil
		}
		if name == "gh" && args[0] == "api" {
			pages++
			if strings.HasSuffix(args[len(args)-1], "page=1") {
				return firstJSON, nil
			}
			return secondJSON, nil
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	got, err := UnfinishedTasks(t.Context(), root, config.Defaults(), "widget", run)
	if err != nil || len(got) != 0 || pages != 2 {
		t.Fatalf("paged tasks=%v pages=%d error=%v", got, pages, err)
	}
}
