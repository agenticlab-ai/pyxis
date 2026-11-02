package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func lockedSeed(t *testing.T) LockedContract {
	t.Helper()
	prd, err := os.ReadFile("../compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	tech, err := os.ReadFile("../compile/testdata/techspec/working.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := compile.Compile("widget", "main", map[string][]byte{"prd.md": prd, "tech-spec.md": tech})
	if err != nil {
		t.Fatal(err)
	}
	return LockedContract{Version: 1, PRD: prd, TechSpec: tech, Digest: result.Digest}
}

func TestReopenSeedsExactLockedTextOnALocalBranch(t *testing.T) {
	root, bare := gitFixture(t)
	gitTest(t, root, "tag", "-a", "lock/widget/v1", "-m", "fixture lock")
	gitTest(t, root, "push", "origin", "refs/tags/lock/widget/v1")
	seed := lockedSeed(t)
	queue, err := events.OpenPending(root, "origin", "widget", 2)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := events.New(events.DraftStepFailed, "widget", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	failed.Step, failed.ErrorClass = "draft-reopen", "transport"
	if err := queue.Add(t.Context(), failed); err != nil {
		t.Fatal(err)
	}
	reader := func(_ context.Context, _, remote, feature string, version int, _ RunCommand) (LockedContract, error) {
		if remote != "origin" || feature != "widget" || version != 1 {
			t.Fatalf("lock request %s %s v%d", remote, feature, version)
		}
		return seed, nil
	}
	t.Setenv("PYXIS_DRAFT_TEST_TOKEN", "fixture-secret")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			if !contains(env, "GH_TOKEN=fixture-secret") {
				return nil, fmt.Errorf("agent credential missing")
			}
			switch {
			case len(args) > 1 && args[0] == "api" && args[1] == "user":
				return []byte("pyxis-agent\n"), nil
			case len(args) > 1 && args[0] == "api" && args[1] == "--paginate":
				return nil, nil
			}
			return nil, fmt.Errorf("a reopen publishes nothing; unexpected gh command %v", args)
		}
		if name == "git" && contains(args, "push") {
			return nil, fmt.Errorf("a reopen publishes nothing; unexpected push %v", args)
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	result, err := ReopenWithCause(t.Context(), root, cfg, "widget", "dependency-change", "issue:42", reader, run)
	if err != nil || result.Branch != "draft/widget-v2" || result.Version != 2 || result.Digest != seed.Digest || result.PullRequest != "" || len(result.Unfinished) != 0 {
		t.Fatalf("reopen=%+v error=%v", result, err)
	}
	if branch := gitTest(t, root, "branch", "--show-current"); branch != "draft/widget-v2" {
		t.Fatalf("reopen left the checkout on %s", branch)
	}
	for name, want := range map[string][]byte{"prd.md": seed.PRD, "tech-spec.md": seed.TechSpec, "problems.md": {}} {
		got, err := os.ReadFile(filepath.Join(root, "drafts/widget", name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("seed %s changed: %v", name, err)
		}
	}
	eventBody, err := os.ReadFile(filepath.Join(root, "drafts/widget/events.md"))
	if err != nil {
		t.Fatal(err)
	}
	eventLog, err := events.Parse(eventBody)
	if err != nil || len(eventLog) != 3 || eventLog[0].ID != failed.ID || eventLog[1].Name != events.DraftOpened || eventLog[1].Origin != "reopen" || eventLog[1].BasisVersion == nil || *eventLog[1].BasisVersion != 1 || eventLog[2].Name != events.DraftReopened || eventLog[2].FromVersion != 1 || eventLog[2].Cause != "dependency-change" || eventLog[2].CauseRef != "issue:42" {
		t.Fatalf("reopen event log = %+v, err = %v", eventLog, err)
	}
	changed := gitTest(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD^", "HEAD")
	for line := range strings.SplitSeq(changed, "\n") {
		if !strings.HasPrefix(line, "drafts/widget/") {
			t.Fatalf("non-seed first commit path %s", line)
		}
	}
	if got := gitTest(t, root, "ls-remote", "--heads", bare, "draft/widget-v2"); got != "" {
		t.Fatalf("reopen published its branch before preparation: %s", got)
	}
}

func TestReopenRefusesWithoutAgentSetupBeforeWriting(t *testing.T) {
	root, _ := gitFixture(t)
	called := false
	reader := func(context.Context, string, string, string, int, RunCommand) (LockedContract, error) {
		called = true
		return LockedContract{}, nil
	}
	if _, err := Reopen(t.Context(), root, config.Defaults(), "widget", reader, nil); err == nil {
		t.Fatal("missing process adapter accepted")
	}
	if _, err := ReopenWithCause(t.Context(), root, config.Defaults(), "widget", "unknown", "issue:42", reader, func(context.Context, string, []string, string, ...string) ([]byte, error) { return nil, nil }); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("unknown reopen cause = %v", err)
	}
	if _, err := ReopenWithCause(t.Context(), root, config.Defaults(), "widget", "dependency-change", "", reader, func(context.Context, string, []string, string, ...string) ([]byte, error) { return nil, nil }); err == nil || !strings.Contains(err.Error(), "evidence reference") {
		t.Fatalf("missing reopen cause reference = %v", err)
	}
	run := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		called = true
		return nil, fmt.Errorf("unexpected process call")
	}
	if _, err := Reopen(t.Context(), root, config.Defaults(), "widget", reader, run); err == nil || !strings.Contains(err.Error(), "draft.agent_login") || called {
		t.Fatalf("setup refusal=%v called=%v", err, called)
	}
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_ABSENT_TOKEN"
	t.Setenv(cfg.Draft.AgentTokenEnv, "")
	if _, err := Reopen(t.Context(), root, cfg, "widget", reader, run); err == nil || !strings.Contains(err.Error(), "agent identity is not available") || called {
		t.Fatalf("missing token refusal=%v called=%v", err, called)
	}
}

func TestReopenKeepsUnfinishedPlanTasksRunning(t *testing.T) {
	root := mergedPlanFixture(t)
	gitTest(t, root, "tag", "-a", "lock/widget/v1", "-m", "fixture lock")
	gitTest(t, root, "push", "origin", "refs/tags/lock/widget/v1")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
	t.Setenv(cfg.Draft.AgentTokenEnv, "fixture-secret")
	seed := lockedSeed(t)
	reader := func(context.Context, string, string, string, int, RunCommand) (LockedContract, error) {
		return seed, nil
	}
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			switch {
			case len(args) > 1 && args[0] == "api" && args[1] == "user":
				return []byte("pyxis-agent"), nil
			case len(args) > 1 && args[0] == "api" && args[1] == "--paginate":
				return nil, nil
			case len(args) > 1 && args[0] == "repo" && args[1] == "view":
				return []byte("example/repo"), nil
			case len(args) > 1 && args[0] == "api" && args[1] == "--method":
				return []byte(`[{"number":17,"body":"<!-- pyxis:plan-task widget/001/T1 -->","state":"closed"}]`), nil
			}
			return nil, fmt.Errorf("unexpected gh command %v", args)
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	result, err := Reopen(t.Context(), root, cfg, "widget", reader, run)
	if err != nil || result.Version != 2 || len(result.Unfinished) != 1 || result.Unfinished[0] != "T2 (not projected)" {
		t.Fatalf("reopen=%+v error=%v", result, err)
	}
	if branch := gitTest(t, root, "branch", "--show-current"); branch != "draft/widget-v2" {
		t.Fatalf("reopen left the checkout on %s", branch)
	}
}

// trackerItems is a configured task tracker holding the given work items.
type trackerItems []model.WorkItem

func (l trackerItems) List(context.Context, model.Query) ([]model.WorkItem, []schema.Defect) {
	return l, nil
}

func TestReopenTrackedListsTasksFromTheConfiguredTracker(t *testing.T) {
	root := mergedPlanFixture(t)
	gitTest(t, root, "tag", "-a", "lock/widget/v1", "-m", "fixture lock")
	gitTest(t, root, "push", "origin", "refs/tags/lock/widget/v1")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
	t.Setenv(cfg.Draft.AgentTokenEnv, "fixture-secret")
	seed := lockedSeed(t)
	reader := func(context.Context, string, string, string, int, RunCommand) (LockedContract, error) {
		return seed, nil
	}
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			switch {
			case len(args) > 1 && args[0] == "api" && args[1] == "user":
				return []byte("pyxis-agent"), nil
			case len(args) > 1 && args[0] == "api" && args[1] == "--paginate":
				return nil, nil
			}
			return nil, fmt.Errorf("the code host's issues were read: %v", args)
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	tasks := trackerItems{
		{ID: 3, State: model.StateClosed, Body: "<!-- pyxis:plan-task widget/001/T1 -->"},
		{ID: 4, State: model.StateOpen, Body: "<!-- pyxis:plan-task widget/001/T2 -->"},
	}
	result, err := ReopenTracked(t.Context(), root, cfg, "widget", "other", "", reader, run, tasks)
	if err != nil || len(result.Unfinished) != 1 || result.Unfinished[0] != "T2 (item #4 open)" {
		t.Fatalf("reopen=%+v error=%v", result, err)
	}
}

func TestReopenNamesPreflightAndSeedFailures(t *testing.T) {
	prior := newPendingEvents
	newPendingEvents = func(string, string, string, int) (pendingEvents, error) { return fixturePending{}, nil }
	t.Cleanup(func() { newPendingEvents = prior })
	seed := lockedSeed(t)
	for _, tc := range []struct{ name, fail, want string }{
		{"identity lookup", "identity", "verify configured agent identity"},
		{"wrong identity", "wrong identity", "differs from authenticated"},
		{"dirty tree", "dirty", "working tree is not clean"},
		{"fetch", "fetch", "fetch current mainline"},
		{"inspect lock", "tags error", "inspect binding lock"},
		{"no lock", "no tag", "has no binding lock"},
		{"open draft", "open PR", "already has an open draft"},
		{"plan tasks", "plan tasks", "inspect unfinished plan tasks"},
		{"read lock", "read lock", "read binding lock"},
		{"incomplete text", "incomplete", "incomplete stored text"},
		{"wrong digest", "digest", "does not reproduce"},
		{"pending event read", "pending", "read pending draft events"},
		{"remote branch inspection", "remote branch", "inspect remote draft branch"},
		{"stale remote branch", "stale branch", "left from an earlier draft"},
		{"current branch inspection", "branch", "inspect current branch before reopening"},
		{"current head inspection", "rev-parse", "inspect current head before reopening"},
		{"branch collision", "switch", "create reopen branch"},
		{"stage", "add", "stage reopen seed"},
		{"commit", "commit", "commit reopen seed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.fail == "pending" {
				newPendingEvents = func(string, string, string, int) (pendingEvents, error) {
					return rejectingPending{readErr: errors.New("remote unavailable")}, nil
				}
				defer func() {
					newPendingEvents = func(string, string, string, int) (pendingEvents, error) { return fixturePending{}, nil }
				}()
			}
			root := t.TempDir()
			cfg := config.Defaults()
			cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
			t.Setenv(cfg.Draft.AgentTokenEnv, "fixture-secret")
			reader := func(context.Context, string, string, string, int, RunCommand) (LockedContract, error) {
				if tc.fail == "read lock" {
					return LockedContract{}, errors.New("missing record")
				}
				if tc.fail == "incomplete" {
					return LockedContract{Version: 1}, nil
				}
				if tc.fail == "digest" {
					wrong := seed
					wrong.Digest = "sha256:wrong"
					return wrong, nil
				}
				return seed, nil
			}
			run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				if (tc.fail == "identity" && name == "gh" && strings.HasPrefix(joined, "api user")) || (tc.fail == "fetch" && strings.HasPrefix(joined, "fetch ")) || (tc.fail == "tags error" && strings.HasPrefix(joined, "ls-remote --tags")) || (tc.fail == "branch" && joined == "branch --show-current") || (tc.fail == "rev-parse" && joined == "rev-parse HEAD") || (tc.fail == "switch" && strings.HasPrefix(joined, "switch -c")) || (tc.fail == "add" && strings.HasPrefix(joined, "add --")) || (tc.fail == "commit" && strings.HasPrefix(joined, "commit -m")) || (tc.fail == "plan tasks" && strings.HasPrefix(joined, "ls-tree")) || (tc.fail == "remote branch" && strings.HasPrefix(joined, "ls-remote origin refs/heads/")) {
					return nil, errors.New("simulated failure")
				}
				switch {
				case name == "gh" && strings.HasPrefix(joined, "api user"):
					if tc.fail == "wrong identity" {
						return []byte("another-account"), nil
					}
					return []byte("pyxis-agent"), nil
				case name == "git" && strings.HasPrefix(joined, "status"):
					if tc.fail == "dirty" {
						return []byte("?? notes.txt"), nil
					}
					return nil, nil
				case name == "git" && strings.HasPrefix(joined, "ls-remote --tags"):
					if tc.fail == "no tag" {
						return nil, nil
					}
					return []byte(strings.Repeat("a", 40) + "\trefs/tags/lock/widget/v1"), nil
				case name == "git" && joined == "ls-remote origin refs/heads/draft/widget-v2" && tc.fail == "stale branch":
					return []byte(strings.Repeat("b", 40) + "\trefs/heads/draft/widget-v2\n"), nil
				case name == "gh" && strings.HasPrefix(joined, "api --paginate"):
					if tc.fail == "open PR" {
						return []byte("draft/widget-v2"), nil
					}
					return nil, nil
				default:
					return nil, nil
				}
			}
			if _, err := Reopen(t.Context(), root, cfg, "widget", reader, run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}
