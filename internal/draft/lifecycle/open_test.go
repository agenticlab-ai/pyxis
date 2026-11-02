package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
)

type fixturePending struct{}

func (fixturePending) Add(context.Context, events.Event) error      { return nil }
func (fixturePending) Read(context.Context) ([]events.Event, error) { return nil, nil }

func gitFixture(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	root, bare := filepath.Join(parent, "work"), filepath.Join(parent, "remote.git")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git(root, "init", "-q", "-b", "main")
	git(root, "config", "user.name", "Test Agent")
	git(root, "config", "user.email", "test@example.com")
	for name, body := range map[string]string{
		"prd-template.md":             "# <Feature or capability name> — PRD\n",
		"draft-tech-spec-template.md": "# <Feature> — tech spec\n",
		"draft-problems-template.md":  "# Requirement problems\n",
		"draft-events-template.md":    "# Draft events\n",
	} {
		path := filepath.Join(root, ".pyxis/templates", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(root, "add", ".pyxis/templates")
	git(root, "commit", "-qm", "fixture")
	git(parent, "init", "--bare", "-q", bare)
	git(root, "remote", "add", "origin", bare)
	git(root, "push", "-q", "-u", "origin", "main")
	return root, bare
}

func TestOpenCreatesOnlyDraftBranchAndAgentPR(t *testing.T) {
	root, bare := gitFixture(t)
	t.Setenv("PYXIS_DRAFT_TEST_TOKEN", "fixture-secret")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
	mainBefore, err := draftshell.Run(context.Background(), root, nil, "git", "rev-parse", "main")
	if err != nil {
		t.Fatal(err)
	}
	prCreates, agentPushes := 0, 0
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			if !contains(env, "GH_TOKEN=fixture-secret") {
				return nil, fmt.Errorf("gh did not use agent token")
			}
			switch strings.Join(args, " ") {
			case "api user --jq .login":
				return []byte("pyxis-agent\n"), nil
			case "api --paginate repos/{owner}/{repo}/pulls?state=open&per_page=100 --jq .[] | select(.head.repo.full_name == .base.repo.full_name) | .head.ref":
				return nil, nil
			case "repo view --json url --jq .url":
				return []byte("https://github.com/example/repo\n"), nil
			default:
				if len(args) >= 2 && args[0] == "pr" && args[1] == "create" {
					prCreates++
					if !contains(args, "--draft") || !contains(args, "draft/widget") {
						return nil, fmt.Errorf("draft PR args: %v", args)
					}
					return []byte("https://github.com/example/repo/pull/42\n"), nil
				}
				return nil, fmt.Errorf("unexpected gh command: %v", args)
			}
		}
		if name == "git" && contains(args, "push") {
			agentPushes++
			if !contains(env, "GH_TOKEN=fixture-secret") || !contains(args, "credential.helper=!gh auth git-credential") || !contains(args, "https://github.com/example/repo.git") {
				return nil, fmt.Errorf("push did not use agent credential: %v %v", args, env)
			}
			return draftshell.Run(ctx, dir, nil, "git", "push", "origin", "draft/widget:refs/heads/draft/widget")
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	got, err := Open(context.Background(), root, cfg, "widget", run)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "draft/widget" || got.PullRequest != "https://github.com/example/repo/pull/42" || prCreates != 1 || agentPushes != 1 {
		t.Fatalf("open = %+v, PRs %d pushes %d", got, prCreates, agentPushes)
	}
	mainAfter, err := draftshell.Run(context.Background(), root, nil, "git", "rev-parse", "main")
	if err != nil || !reflect.DeepEqual(mainAfter, mainBefore) {
		t.Fatalf("mainline changed: before %q after %q err %v", mainBefore, mainAfter, err)
	}
	for _, path := range []string{"prd.md", "tech-spec.md", "problems.md", "events.md", "brainstorm/.keep", "reviews/.keep"} {
		if _, err := os.Stat(filepath.Join(root, "drafts/widget", path)); err != nil {
			t.Fatalf("missing scaffold %s: %v", path, err)
		}
	}
	remote, err := draftshell.Run(context.Background(), root, nil, "git", "ls-remote", "--heads", bare, "draft/widget")
	if err != nil || len(remote) == 0 {
		t.Fatalf("draft branch not pushed: %q %v", remote, err)
	}
}

func TestOpenPublicationFailureRestoresEveryWorkingFile(t *testing.T) {
	root, bare := gitFixture(t)
	t.Setenv("PYXIS_DRAFT_TEST_TOKEN", "fixture-secret")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
	before := gitTest(t, root, "rev-parse", "HEAD")
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			switch {
			case len(args) > 1 && args[0] == "api" && args[1] == "user":
				return []byte("pyxis-agent\n"), nil
			case len(args) > 1 && args[0] == "api" && args[1] == "--paginate":
				return nil, nil
			case len(args) > 1 && args[0] == "repo":
				return []byte("https://github.com/example/repo\n"), nil
			case len(args) > 1 && args[0] == "pr" && args[1] == "create":
				return nil, fmt.Errorf("simulated publication failure")
			}
		}
		if name == "git" && contains(args, "push") && contains(args, "https://github.com/example/repo.git") {
			return draftshell.Run(ctx, dir, nil, "git", "push", "origin", "draft/widget:refs/heads/draft/widget")
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	if _, err := Open(t.Context(), root, cfg, "widget", run); err == nil || !strings.Contains(err.Error(), "opening its draft pull request failed") {
		t.Fatalf("open error = %v", err)
	}
	if branch := gitTest(t, root, "branch", "--show-current"); branch != "main" {
		t.Fatalf("failed open left branch %s checked out", branch)
	}
	if head := gitTest(t, root, "rev-parse", "HEAD"); head != before {
		t.Fatalf("failed open changed head from %s to %s", before, head)
	}
	if status := gitTest(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("failed open changed working files: %s", status)
	}
	if _, err := os.Stat(filepath.Join(root, "drafts/widget/prd.md")); !os.IsNotExist(err) {
		t.Fatalf("failed open left draft file: %v", err)
	}
	if remote := gitTest(t, root, "ls-remote", "--heads", bare, "draft/widget"); remote == "" {
		t.Fatal("published branch was lost during local rollback")
	}
	queue, err := events.OpenPending(root, "origin", "widget", 1)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := queue.Read(t.Context())
	if err != nil || len(pending) != 1 || pending[0].Name != events.DraftStepFailed || pending[0].Step != "draft-open" {
		t.Fatalf("failed open event = %+v, err = %v", pending, err)
	}
}

func TestOpenRefusesWithoutAgentBeforeAnyWrite(t *testing.T) {
	root, _ := gitFixture(t)
	called := false
	run := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		called = true
		return nil, fmt.Errorf("must not run")
	}
	_, err := Open(context.Background(), root, config.Defaults(), "widget", run)
	if err == nil || !strings.Contains(err.Error(), "agent identity is not configured") || called {
		t.Fatalf("missing identity: %v, called=%v", err, called)
	}
	if _, err := os.Stat(filepath.Join(root, "drafts/widget")); !os.IsNotExist(err) {
		t.Fatalf("draft created without identity: %v", err)
	}
}

func TestOpenRequiresProcessAdapter(t *testing.T) {
	t.Setenv("PYXIS_DRAFT_TEST_TOKEN", "fixture-secret")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
	if _, err := Open(context.Background(), t.TempDir(), cfg, "widget", nil); err == nil || !strings.Contains(err.Error(), "adapter is required") {
		t.Fatalf("missing adapter: %v", err)
	}
}

func TestOpenNamesPreflightAndPublicationFailures(t *testing.T) {
	prior := newPendingEvents
	newPendingEvents = func(string, string, string, int) (pendingEvents, error) { return fixturePending{}, nil }
	t.Cleanup(func() { newPendingEvents = prior })
	ledgerPath := "docs/prds/widget/000-base.md"
	for _, tc := range []struct {
		name, feature, token, identity, status, ledger, tags, pulls, repoURL, failAt, want string
	}{
		{name: "bad slug", feature: "Widget", token: "secret", want: "lowercase hyphenated slug"},
		{name: "missing token", feature: "widget", want: "agent identity is not available"},
		{name: "identity lookup", feature: "widget", token: "secret", failAt: "api", want: "verify configured agent identity"},
		{name: "wrong identity", feature: "widget", token: "secret", identity: "operator", want: "differs from authenticated account"},
		{name: "dirty tree", feature: "widget", token: "secret", status: "?? notes.txt", want: "working tree is not clean"},
		{name: "fetch mainline", feature: "widget", token: "secret", failAt: "fetch", want: "fetch mainline"},
		{name: "ledger inspection", feature: "widget", token: "secret", failAt: "ls-tree", want: "inspect mainline PRD ledger"},
		{name: "mainline ledger", feature: "widget", token: "secret", ledger: ledgerPath, want: "day-one rule"},
		{name: "locked feature", feature: "widget", token: "secret", tags: "abc\trefs/tags/lock/widget/v1\n", want: "draft reopen"},
		{name: "existing PR", feature: "widget", token: "secret", pulls: "draft/widget\n", want: "already has an open draft"},
		{name: "unreadable template", feature: "widget", token: "secret", failAt: "show", want: "read draft template"},
		{name: "current branch inspection", feature: "widget", token: "secret", failAt: "branch", want: "inspect current branch before opening"},
		{name: "current head inspection", feature: "widget", token: "secret", failAt: "rev-parse", want: "inspect current head before opening"},
		{name: "branch collision", feature: "widget", token: "secret", failAt: "switch", want: "create draft branch"},
		{name: "repository lookup", feature: "widget", token: "secret", failAt: "repo", want: "resolve code-host repository"},
		{name: "unsafe repository URL", feature: "widget", token: "secret", repoURL: "http://example.test/o/r", want: "must be HTTPS"},
		{name: "agent push", feature: "widget", token: "secret", failAt: "push", want: "push draft branch"},
		{name: "PR publication", feature: "widget", token: "secret", failAt: "create", want: "was pushed, but opening"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PYXIS_DRAFT_TEST_TOKEN", tc.token)
			cfg := config.Defaults()
			cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
			run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
				key := args[0]
				if name == "gh" && key == "pr" {
					key = args[1]
				}
				if name == "gh" && key == "api" && len(args) > 1 && args[1] == "--paginate" {
					key = "list"
				}
				if name == "git" && contains(args, "push") {
					key = "push"
				}
				if key == tc.failAt {
					return nil, fmt.Errorf("fixture %s failure", key)
				}
				if name == "gh" {
					switch key {
					case "api":
						if tc.identity != "" {
							return []byte(tc.identity), nil
						}
						return []byte("pyxis-agent"), nil
					case "list":
						if tc.pulls != "" {
							return []byte(tc.pulls), nil
						}
						return nil, nil
					case "repo":
						if tc.repoURL != "" {
							return []byte(tc.repoURL), nil
						}
						return []byte("https://example.test/o/r"), nil
					}
					return []byte("https://example.test/o/r/pull/1"), nil
				}
				switch key {
				case "status":
					return []byte(tc.status), nil
				case "ls-tree":
					return []byte(tc.ledger), nil
				case "ls-remote":
					return []byte(tc.tags), nil
				case "show":
					if strings.Contains(strings.Join(args, " "), "draft-events-template.md") {
						return []byte("# Draft events\n"), nil
					}
					return []byte("# <Feature>\n"), nil
				default:
					return nil, nil
				}
			}
			_, err := Open(context.Background(), root, cfg, tc.feature, run)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Open error = %v, want %q", err, tc.want)
			}
		})
	}
}

func contains(parts []string, want string) bool {
	return slices.Contains(parts, want)
}
