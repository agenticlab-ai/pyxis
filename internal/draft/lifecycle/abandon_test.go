package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func abandonmentFixture(t *testing.T) (string, string, config.Config) {
	t.Helper()
	root, bare := gitFixture(t)
	gitTest(t, root, "switch", "-c", "draft/widget")
	path := filepath.Join(root, "drafts/widget/prd.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("working draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "--", "drafts/widget/prd.md")
	gitTest(t, root, "commit", "-m", "open widget draft")
	gitTest(t, root, "push", "-u", "origin", "draft/widget")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
	cfg.Approval.OperatorLogin = "configured-operator"
	t.Setenv(cfg.Draft.AgentTokenEnv, "fixture-secret")
	return root, bare, cfg
}

type abandonmentHost struct {
	t                               *testing.T
	ready, reviewer, edited, merged bool
	policySynced, gated             bool
	autoDelete                      bool
	failAt                          string
	identity                        string
	pulls                           string
	prData                          string
	repoSlug                        string
	repoURL                         string
	policyOutput                    string
	policyPath                      string
	newBranch                       string // an unpublished draft's branch, pushed for the first time
	openPR                          string // what `gh pr list` reports open on that branch
	created                         bool
	createErr                       error
}

func (h *abandonmentHost) run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	if h.failAt != "" && strings.Contains(name+" "+strings.Join(args, " "), h.failAt) {
		return nil, errors.New("simulated host failure")
	}
	if name == "go" {
		switch {
		case contains(args, "--baseline-sync"):
			h.policySynced = true
			if h.policyOutput != "" {
				return []byte(h.policyOutput), nil
			}
			head := gitTest(h.t, dir, "rev-parse", "HEAD")
			if h.policyPath != "" {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, h.policyPath)), 0o755); err != nil {
					h.t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, h.policyPath), []byte("updated policy\n"), 0o644); err != nil {
					h.t.Fatal(err)
				}
				return fmt.Appendf(nil, `{"schema_version":"memory-policy-sync/v1","status":"complete","head":%q,"written":[%q]}`, head, h.policyPath), nil
			}
			return fmt.Appendf(nil, `{"schema_version":"memory-policy-sync/v1","status":"complete","head":%q,"written":[]}`, head), nil
		case contains(args, "./cmd/memory-gate"):
			h.gated = true
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected go command %v", args)
		}
	}
	if name == "gh" {
		if !contains(env, "GH_TOKEN=fixture-secret") {
			return nil, errors.New("agent credential missing")
		}
		switch {
		case len(args) > 1 && args[0] == "api" && args[1] == "user":
			if h.identity != "" {
				return []byte(h.identity), nil
			}
			return []byte("pyxis-agent\n"), nil
		case len(args) > 1 && args[0] == "api" && args[1] == "--paginate":
			if h.pulls != "" {
				return []byte(h.pulls), nil
			}
			return []byte("draft/widget\n"), nil
		case len(args) > 2 && args[0] == "api" && args[1] == "repos/example/repo":
			if h.autoDelete {
				return []byte("true\n"), nil
			}
			return []byte("false\n"), nil
		case len(args) > 1 && args[0] == "repo" && contains(args, "nameWithOwner"):
			if h.repoSlug != "" {
				return []byte(h.repoSlug), nil
			}
			return []byte("example/repo\n"), nil
		case len(args) > 1 && args[0] == "repo" && contains(args, "url"):
			if h.repoURL != "" {
				return []byte(h.repoURL), nil
			}
			return []byte("https://github.com/example/repo\n"), nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "view":
			if h.prData != "" {
				return []byte(h.prData), nil
			}
			return []byte(`{"number":42,"isDraft":true,"headRefName":"draft/widget"}`), nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "edit":
			if contains(args, "--body") {
				h.edited = true
			}
			if contains(args, "--add-reviewer") && contains(args, "configured-operator") {
				h.reviewer = true
			}
			return nil, nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "ready":
			h.ready = true
			return nil, nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "list":
			if h.openPR == "" {
				return []byte("[]\n"), nil
			}
			return []byte(h.openPR), nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "create":
			if h.createErr != nil {
				return nil, h.createErr
			}
			h.created = true
			h.reviewer = contains(args, "--reviewer") && contains(args, "configured-operator")
			return []byte("https://github.com/example/repo/pull/51\n"), nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "merge":
			h.merged = true
			return nil, errors.New("agent cannot merge")
		default:
			return nil, fmt.Errorf("unexpected gh command %v", args)
		}
	}
	if name == "git" && contains(args, "https://github.com/example/repo.git") {
		var leased bool
		if h.newBranch != "" {
			leased = slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "--force-with-lease=refs/heads/"+h.newBranch+":") })
		} else {
			leased = contains(args, "--force-with-lease=refs/heads/draft/widget:"+gitTest(h.t, dir, "rev-parse", "draft/widget"))
		}
		if !contains(env, "GH_TOKEN=fixture-secret") || !leased {
			return nil, fmt.Errorf("push lacked agent lease: %v", args)
		}
		local := append([]string(nil), args...)
		for i := range local {
			if local[i] == "https://github.com/example/repo.git" {
				local[i] = "origin"
			}
		}
		return draftshell.Run(ctx, dir, nil, "git", local...)
	}
	return draftshell.Run(ctx, dir, env, name, args...)
}

func TestAbandonReplacesPRWithOnlyReasonLearning(t *testing.T) {
	root, bare, cfg := abandonmentFixture(t)
	mainBefore := gitTest(t, root, "rev-parse", "origin/main")
	host := &abandonmentHost{t: t, autoDelete: true}
	result, err := Abandon(t.Context(), root, cfg, "widget", "The requirement was withdrawn.", host.run)
	if err != nil || result.Branch != "draft/widget" || result.PullRequest != 42 || !host.ready || !host.reviewer || !host.edited || !host.policySynced || !host.gated || host.merged {
		t.Fatalf("abandon=%+v host=%+v error=%v", result, host, err)
	}
	if gitTest(t, root, "rev-parse", "origin/main") != mainBefore {
		t.Fatal("abandonment changed mainline")
	}
	if gitTest(t, root, "rev-parse", "HEAD^") != mainBefore {
		t.Fatal("abandonment proposal did not start at mainline")
	}
	paths := strings.Split(gitTest(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD^", "HEAD"), "\n")
	if len(paths) != 1 || paths[0] != result.RecordPath || !strings.HasPrefix(paths[0], "memory/learning/draft-abandonment-widget-v1/") {
		t.Fatalf("proposal paths=%v", paths)
	}
	body, err := os.ReadFile(filepath.Join(root, result.RecordPath))
	if err != nil {
		t.Fatal(err)
	}
	var r record.Record
	if err := json.Unmarshal(body, &r); err != nil || r.Kind != record.Learning || r.Rationale != "The requirement was withdrawn." || r.Provenance.PR != 42 || r.Provenance.Source != "draft-abandonment:widget:v1" {
		t.Fatalf("reason record=%+v error=%v", r, err)
	}
	if remote := gitTest(t, root, "ls-remote", "--heads", bare, "draft/widget"); !strings.HasPrefix(remote, result.Head) {
		t.Fatalf("remote branch=%s", remote)
	}
}

func TestAbandonCarriesOpenedPendingAndRuledChangeEvents(t *testing.T) {
	root, _, cfg := abandonmentFixture(t)
	gitTest(t, root, "reset", "--hard", "origin/main")
	prd, err := os.ReadFile("../compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	first := []byte(strings.Replace(string(prd), "The widget shall open.", "The widget should open.", 1))
	tech := []byte("# Widget tech spec\n")
	write := func(name string, body []byte) {
		t.Helper()
		path := filepath.Join(root, "drafts/widget", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opened, err := events.New(events.DraftOpened, "widget", compile.ContractDigest(first, tech), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	opened.Origin = "new"
	eventBody, _, err := events.Append(nil, opened)
	if err != nil {
		t.Fatal(err)
	}
	write("prd.md", first)
	write("tech-spec.md", tech)
	write("events.md", eventBody)
	gitTest(t, root, "add", "--", "drafts/widget")
	gitTest(t, root, "commit", "-m", "open complete draft")
	write("prd.md", prd)
	write("problems.md", []byte("# Problems\n\n### P-1 — FR-1 — Clarify wording\n**Problem:** The opening verb was unclear.\n**Ruling:** operator · Name the opening action. · Use the shall wording.\n"))
	gitTest(t, root, "add", "--", "drafts/widget/prd.md", "drafts/widget/problems.md")
	gitTest(t, root, "commit", "-m", "rule requirement")
	gitTest(t, root, "push", "--force-with-lease", "origin", "draft/widget")
	queue, err := events.OpenPending(root, "origin", "widget", 1)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := events.New(events.DraftStepFailed, "widget", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	failed.Step, failed.ErrorClass = "review", "transport"
	if err := queue.Add(t.Context(), failed); err != nil {
		t.Fatal(err)
	}
	host := &abandonmentHost{t: t, autoDelete: true}
	result, err := Abandon(t.Context(), root, cfg, "widget", "Withdrawn after review.", host.run)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(filepath.Join(root, result.RecordPath))
	if err != nil {
		t.Fatal(err)
	}
	var learning record.Record
	if err := json.Unmarshal(bytes, &learning); err != nil {
		t.Fatal(err)
	}
	_, log, ok := strings.Cut(learning.Rationale, "Draft events:\n")
	if !ok || !strings.HasPrefix(learning.Rationale, "Withdrawn after review.\n\n") {
		t.Fatalf("abandonment rationale = %q", learning.Rationale)
	}
	retained, err := events.Parse([]byte(log))
	if err != nil || len(retained) != 3 || retained[0].Name != events.DraftOpened || retained[1].Name != events.DraftStepFailed || retained[2].Name != events.RequirementChanged || retained[2].Key == "" || retained[2].RulingAuthor != "operator" {
		t.Fatalf("retained events = %+v, err = %v", retained, err)
	}
}

func TestAbandonRequiresReasonAgentAndBranchDeletionSetting(t *testing.T) {
	root, _, cfg := abandonmentFixture(t)
	host := &abandonmentHost{t: t, autoDelete: false}
	if _, err := Abandon(t.Context(), root, cfg, "widget", "", host.run); err == nil {
		t.Fatal("blank reason accepted")
	}
	withoutAgent := cfg
	withoutAgent.Draft.AgentLogin = ""
	if _, err := Abandon(t.Context(), root, withoutAgent, "widget", "withdrawn", host.run); err == nil || !strings.Contains(err.Error(), "agent identity") {
		t.Fatalf("agent guard=%v", err)
	}
	if _, err := Abandon(t.Context(), root, cfg, "widget", "withdrawn", host.run); err == nil || !strings.Contains(err.Error(), "automatic head-branch deletion") {
		t.Fatalf("branch deletion guard=%v", err)
	}
	if path := gitTest(t, root, "ls-tree", "-r", "--name-only", "HEAD", "--", "memory/learning"); path != "" {
		t.Fatalf("wrote Learning before guard: %s", path)
	}
}

func TestAbandonRefusesUnverifiedPolicySyncBeforePublish(t *testing.T) {
	root, bare, cfg := abandonmentFixture(t)
	host := &abandonmentHost{t: t, autoDelete: true, policyOutput: `{"schema_version":"memory-policy-sync/v1","status":"complete","head":"wrong","written":[]}`}
	if _, err := Abandon(t.Context(), root, cfg, "widget", "withdrawn", host.run); err == nil || !strings.Contains(err.Error(), "incomplete result") {
		t.Fatalf("policy result guard=%v", err)
	}
	if host.gated || host.edited || host.ready {
		t.Fatalf("abandonment published after invalid policy result: %+v", host)
	}
	if remote := gitTest(t, root, "ls-remote", "--heads", bare, "draft/widget"); !strings.HasPrefix(remote, gitTest(t, root, "rev-parse", "HEAD")) {
		t.Fatalf("remote branch changed: %s", remote)
	}
}

func TestAbandonIncludesRequiredPolicyChangeAlongsideOneLearning(t *testing.T) {
	root, _, cfg := abandonmentFixture(t)
	path := ".github/memory-reconciliation-baseline.json"
	host := &abandonmentHost{t: t, autoDelete: true, policyPath: path}
	result, err := Abandon(t.Context(), root, cfg, "widget", "withdrawn", host.run)
	if err != nil || !host.gated || !host.ready {
		t.Fatalf("abandon=%+v host=%+v error=%v", result, host, err)
	}
	paths := strings.Split(gitTest(t, root, "diff", "--name-only", "origin/main", "HEAD"), "\n")
	if len(paths) != 2 || paths[0] != path || paths[1] != result.RecordPath {
		t.Fatalf("abandonment proposal paths=%v", paths)
	}
	if gitTest(t, root, "show", "--format=", "--name-only", "HEAD") != path {
		t.Fatal("policy was not committed after the Learning")
	}
}

func TestAbandonRefusesUnexpectedOrUncommittedPolicyChange(t *testing.T) {
	for _, tc := range []struct{ path, failAt, want string }{
		{"other/file", "", "unexpected path"},
		{".github/memory-reconciliation-baseline.json", "git add -- .github/memory-reconciliation-baseline.json", "stage abandonment memory policy"},
		{".github/memory-reconciliation-baseline.json", "git commit -m chore(memory)", "commit abandonment memory policy"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			root, _, cfg := abandonmentFixture(t)
			host := &abandonmentHost{t: t, autoDelete: true, policyPath: tc.path, failAt: tc.failAt}
			if _, err := Abandon(t.Context(), root, cfg, "widget", "withdrawn", host.run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("policy guard=%v, want %q", err, tc.want)
			}
			if host.gated || host.edited || host.ready {
				t.Fatalf("abandonment published after policy failure: %+v", host)
			}
		})
	}
}

func TestAbandonNamesFailuresBeforeAndAfterRecordPublication(t *testing.T) {
	for _, tc := range []struct {
		name, failAt, identity, pulls, prData, repoSlug, repoURL, want string
		wrongBranch                                                    bool
	}{
		{name: "identity API", failAt: "gh api user", want: "verify configured agent identity"},
		{name: "wrong identity", identity: "operator", want: "differs from authenticated"},
		{name: "working tree", failAt: "git status --porcelain", want: "inspect draft working tree"},
		{name: "fetch", failAt: "git fetch origin main", want: "fetch mainline"},
		{name: "inspect draft", failAt: "git ls-remote --tags", want: "inspect draft for abandonment"},
		{name: "no draft", pulls: "other/branch", want: "no open feature-draft"},
		{name: "wrong branch", wrongBranch: true, want: "not draft/widget"},
		{name: "branch inspection", failAt: "git branch --show-current", want: "inspect draft branch"},
		{name: "remote inspection", failAt: "git ls-remote origin refs/heads/draft/widget", want: "inspect remote draft branch"},
		{name: "PR lookup", failAt: "gh pr view", want: "find draft pull request"},
		{name: "PR mismatch", prData: `{"number":42,"isDraft":true,"headRefName":"draft/other"}`, want: "invalid draft pull request"},
		{name: "repo lookup", failAt: "gh repo view --json nameWithOwner", want: "resolve code-host repository"},
		{name: "bad repository", repoSlug: "bad repo", want: "is invalid"},
		{name: "push URL lookup", failAt: "gh repo view --json url", want: "resolve code-host push URL"},
		{name: "unsafe push URL", repoURL: "http://example.test/repo", want: "must be HTTPS"},
		{name: "worktree", failAt: "git worktree add", want: "create abandonment worktree"},
		{name: "stage Learning", failAt: "git add --", want: "stage abandonment Learning"},
		{name: "commit Learning", failAt: "git commit -m", want: "commit abandonment Learning"},
		{name: "policy sync", failAt: "go run ./cmd/memory reconcile --baseline-sync", want: "synchronize abandonment memory policy"},
		{name: "memory gate", failAt: "go run ./cmd/memory-gate", want: "memory gate at abandonment tree"},
		{name: "push Learning", failAt: "--force-with-lease", want: "publish abandonment Learning"},
		{name: "edit PR", failAt: "gh pr edit 42 --body", want: "publish abandonment explanation"},
		{name: "ready PR", failAt: "gh pr ready", want: "mark abandonment PR ready"},
		{name: "request review", failAt: "gh pr edit 42 --add-reviewer", want: "request operator review"},
		{name: "align local", failAt: "git reset --hard", want: "align abandoned local branch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _, cfg := abandonmentFixture(t)
			if tc.wrongBranch {
				gitTest(t, root, "switch", "main")
			}
			host := &abandonmentHost{t: t, autoDelete: true, failAt: tc.failAt, identity: tc.identity, pulls: tc.pulls, prData: tc.prData, repoSlug: tc.repoSlug, repoURL: tc.repoURL}
			if _, err := Abandon(t.Context(), root, cfg, "widget", "withdrawn", host.run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

// reopenedLocally leaves widget locked at v1 with a v2 draft that was never
// published: reopen keeps a draft local until its preparation.
func reopenedLocally(t *testing.T) (string, string, config.Config) {
	t.Helper()
	root, bare := gitFixture(t)
	gitTest(t, root, "tag", "lock/widget/v1")
	gitTest(t, root, "push", "-q", "origin", "lock/widget/v1")
	gitTest(t, root, "switch", "-c", "draft/widget-v2")
	path := filepath.Join(root, "drafts/widget/prd.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("reopened draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "--", "drafts/widget/prd.md")
	gitTest(t, root, "commit", "-m", "reopen widget v2")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "pyxis-agent", "PYXIS_DRAFT_TEST_TOKEN"
	cfg.Approval.OperatorLogin = "configured-operator"
	t.Setenv(cfg.Draft.AgentTokenEnv, "fixture-secret")
	return root, bare, cfg
}

// A repository with no configured operator still publishes its abandonment and
// requests nobody, on the new pull request and on the existing one.
func TestAbandonWithNoConfiguredOperatorRequestsNobody(t *testing.T) {
	t.Run("new pull request", func(t *testing.T) {
		root, _, cfg := reopenedLocally(t)
		cfg.Approval.OperatorLogin = ""
		host := &abandonmentHost{t: t, autoDelete: true, pulls: "other/branch\n", newBranch: "draft/widget-v2"}
		if result, err := Abandon(t.Context(), root, cfg, "widget", "No contract change was needed.", host.run); err != nil || !host.created || host.reviewer {
			t.Fatalf("abandon=%+v host=%+v error=%v", result, host, err)
		}
	})
	t.Run("existing pull request", func(t *testing.T) {
		root, _, cfg := abandonmentFixture(t)
		cfg.Approval.OperatorLogin = ""
		host := &abandonmentHost{t: t, autoDelete: true}
		if result, err := Abandon(t.Context(), root, cfg, "widget", "The requirement was withdrawn.", host.run); err != nil || !host.edited || host.reviewer {
			t.Fatalf("abandon=%+v host=%+v error=%v", result, host, err)
		}
	})
}

func TestAbandonPublishesAnUnpublishedReopenWithItsReason(t *testing.T) {
	root, bare, cfg := reopenedLocally(t)
	host := &abandonmentHost{t: t, autoDelete: true, pulls: "other/branch\n", newBranch: "draft/widget-v2"}
	result, err := Abandon(t.Context(), root, cfg, "widget", "No contract change was needed.", host.run)
	if err != nil || result.Branch != "draft/widget-v2" || result.PullRequest != 51 || !host.created || !host.reviewer || host.edited || host.ready {
		t.Fatalf("abandon=%+v host=%+v error=%v", result, host, err)
	}
	body, err := os.ReadFile(filepath.Join(root, result.RecordPath))
	if err != nil {
		t.Fatal(err)
	}
	var r record.Record
	if err := json.Unmarshal(body, &r); err != nil || r.Subject != "learning/draft-abandonment-widget-v2" || r.Provenance.PR != 0 {
		t.Fatalf("reason record=%+v error=%v", r, err)
	}
	if remote := gitTest(t, root, "ls-remote", "--heads", bare, "draft/widget-v2"); !strings.HasPrefix(remote, result.Head) {
		t.Fatalf("remote branch=%q", remote)
	}
}

func TestAbandonWithdrawsAnUnpublishedReopenWhosePullRequestFails(t *testing.T) {
	root, bare, cfg := reopenedLocally(t)
	draftHead := gitTest(t, root, "rev-parse", "HEAD")
	host := &abandonmentHost{t: t, autoDelete: true, pulls: "other/branch\n", newBranch: "draft/widget-v2", createErr: errors.New("host outage")}
	if _, err := Abandon(t.Context(), root, cfg, "widget", "No contract change was needed.", host.run); err == nil || !strings.Contains(err.Error(), "branch was withdrawn") {
		t.Fatalf("error = %v", err)
	}
	if remote := gitTest(t, root, "ls-remote", "--heads", bare, "draft/widget-v2"); remote != "" || gitTest(t, root, "rev-parse", "HEAD") != draftHead {
		t.Fatalf("failed abandonment left remote %q or moved the local draft", remote)
	}
}

func TestAbandonRefusesAnUnpublishedReopenWithAStrayBranch(t *testing.T) {
	root, _, cfg := reopenedLocally(t)
	gitTest(t, root, "push", "-q", "origin", "draft/widget-v2")
	host := &abandonmentHost{t: t, autoDelete: true, pulls: "other/branch\n", newBranch: "draft/widget-v2"}
	if _, err := Abandon(t.Context(), root, cfg, "widget", "No contract change was needed.", host.run); err == nil || !strings.Contains(err.Error(), "has no open pull request") {
		t.Fatalf("error = %v", err)
	}
}

func TestAbandonKeepsAnOpenedPullRequestWhoseReviewerFailed(t *testing.T) {
	root, bare, cfg := reopenedLocally(t)
	host := &abandonmentHost{t: t, autoDelete: true, pulls: "other/branch\n", newBranch: "draft/widget-v2", createErr: errors.New("pull request update failed"), openPR: `[{"number":51,"headRefName":"draft/widget-v2","baseRefName":"main","isCrossRepository":false}]`, failAt: "gh pr edit 51 --add-reviewer"}
	result, err := Abandon(t.Context(), root, cfg, "widget", "No contract change was needed.", host.run)
	if err == nil || !strings.Contains(err.Error(), "request it by hand") || result.PullRequest != 51 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if remote := gitTest(t, root, "ls-remote", "--heads", bare, "draft/widget-v2"); !strings.HasPrefix(remote, result.Head) || gitTest(t, root, "rev-parse", "HEAD") != result.Head {
		t.Fatalf("an opened pull request was not kept with its local branch: remote %q", remote)
	}
}
