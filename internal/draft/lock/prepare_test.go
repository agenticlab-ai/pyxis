package lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func gitFixture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func fixtureFile(t *testing.T, root, name string, body []byte) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readyPRD(t *testing.T) []byte {
	t.Helper()
	prd, err := os.ReadFile("../compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(prd)
	body = strings.Replace(body, "| Q-1 | Who owns retries? | PM | lock | open | |", "| Q-1 | Who owns retries? | PM | lock | resolved | The PM owns retries. |", 1)
	body = strings.Replace(body, "## 5. System requirements", "## 1. Summary\n\n## 2. Problem space\n\n## 3. Goals, metrics, and guardrails\n\n## 4. Solution space\n\n## 5. System requirements", 1)
	body = strings.Replace(body, "| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", "### 5.8 Definition of Done\n\n| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", 1)
	body = strings.Replace(body, "## 7. Learning", "## 6. Delivery\n\n## 7. Learning", 1)
	return []byte(body + "\n## 8. Contract readiness\n")
}

type prepareHost struct {
	body        string
	pushes      int
	reviews     int
	created     int
	taskState   string // the live plan's T1 issue state; "" when unprojected
	failPush    bool
	failHistory bool
}

func (h *prepareHost) run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	switch name {
	case "gh":
		switch {
		case slices.Equal(args, []string{"api", "user", "--jq", ".login"}):
			return []byte("agent\n"), nil
		case slices.Equal(args, []string{"repo", "view", "--json", "url", "--jq", ".url"}):
			return []byte("https://github.com/example/repo\n"), nil
		case slices.Equal(args, []string{"repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"}):
			return []byte("example/repo\n"), nil
		case len(args) >= 3 && slices.Equal(args[:3], []string{"api", "--method", "GET"}):
			if h.taskState == "" {
				return []byte("[]"), nil
			}
			return fmt.Appendf(nil, `[{"number":5,"body":"<!-- pyxis:plan-task widget/001/T1 -->","state":%q}]`, h.taskState), nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "view":
			return []byte("17\n"), nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "edit":
			for i := range args {
				if args[i] == "--body" && i+1 < len(args) {
					h.body = args[i+1]
				}
				if args[i] == "--add-reviewer" {
					h.reviews++
				}
			}
			return nil, nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "ready":
			return nil, nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "create":
			for i := range args {
				if args[i] == "--body" && i+1 < len(args) {
					h.body = args[i+1]
				}
				if args[i] == "--reviewer" {
					h.reviews++
				}
			}
			h.created++
			return []byte("https://github.com/example/repo/pull/23\n"), nil
		default:
			return nil, fmt.Errorf("unexpected gh command %v", args)
		}
	case "go":
		if slices.Equal(args, []string{"run", "./cmd/memory-gate", "--base", strings.TrimPrefix(args[len(args)-1], "--base=")}) {
			return []byte("memory-gate: pass\n"), nil
		}
		return nil, fmt.Errorf("unexpected go command %v", args)
	case "git":
		if h.failHistory && len(args) > 2 && slices.Equal(args[:3], []string{"rev-list", "--reverse", "--first-parent"}) {
			return nil, errors.New("simulated unreadable branch history")
		}
		for i := range args {
			if args[i] == "push" && i+2 < len(args) && strings.HasPrefix(args[i+2], "https://") {
				if h.failPush {
					return nil, errors.New("simulated publication failure")
				}
				h.pushes++
				local := append([]string(nil), args...)
				local[i+2] = "origin"
				return draftshell.Run(ctx, dir, env, name, local...)
			}
		}
	}
	return draftshell.Run(ctx, dir, env, name, args...)
}

func prepareFixture(t *testing.T, reviews bool) (string, config.Config, *prepareHost) {
	t.Helper()
	parent := t.TempDir()
	bare := filepath.Join(parent, "remote.git")
	work := filepath.Join(parent, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, parent, "init", "--bare", bare)
	gitFixture(t, bare, "config", "gc.auto", "0")
	gitFixture(t, bare, "config", "maintenance.auto", "false")
	gitFixture(t, work, "init", "-b", "main")
	gitFixture(t, work, "config", "gc.auto", "0")
	gitFixture(t, work, "config", "maintenance.auto", "false")
	gitFixture(t, work, "config", "user.name", "Draft Agent")
	gitFixture(t, work, "config", "user.email", "agent@example.test")
	gitFixture(t, work, "remote", "add", "origin", bare)
	fixtureFile(t, work, "README.md", []byte("base\n"))
	gitFixture(t, work, "add", "--", "README.md")
	gitFixture(t, work, "commit", "-m", "base")
	gitFixture(t, work, "push", "-u", "origin", "main")
	gitFixture(t, work, "switch", "-c", "draft/widget")
	prd := readyPRD(t)
	tech := []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n")
	files := map[string][]byte{"prd.md": prd, "tech-spec.md": tech}
	draft, err := compile.Compile("widget", "base", files)
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, work, "drafts/widget/prd.md", prd)
	fixtureFile(t, work, "drafts/widget/tech-spec.md", tech)
	if reviews {
		fixtureFile(t, work, "drafts/widget/reviews/001.md", []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** "+draft.Digest+"\n"))
		fixtureFile(t, work, "drafts/widget/reviews/002.md", []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** "+draft.Digest+"\n"))
	}
	gitFixture(t, work, "add", "--", "drafts/widget")
	gitFixture(t, work, "commit", "-m", "open draft")
	gitFixture(t, work, "push", "-u", "origin", "draft/widget")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin = "agent"
	cfg.Draft.AgentTokenEnv = "PYXIS_DRAFT_TEST_TOKEN"
	cfg.Approval.OperatorLogin = "configured-operator"
	t.Setenv(cfg.Draft.AgentTokenEnv, "test-token")
	return work, cfg, &prepareHost{}
}

// The prepared draft's review request names the repository's configured
// operator, and a repository that named none opens its pull request and requests
// nobody, whether or not the branch was already published.
func TestPrepareRequestsOnlyTheConfiguredOperator(t *testing.T) {
	for _, tc := range []struct {
		name, operator string
		published      bool
	}{
		{"configured, new pull request", "configured-operator", false},
		{"configured, existing pull request", "configured-operator", true},
		{"unconfigured, new pull request", "", false},
		{"unconfigured, existing pull request", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			cfg.Approval.OperatorLogin = tc.operator
			if !tc.published {
				gitFixture(t, work, "push", "-q", "origin", "--delete", "draft/widget")
			}
			var reviewers []string
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "gh" {
					for i, arg := range args {
						if (arg == "--reviewer" || arg == "--add-reviewer") && i+1 < len(args) {
							reviewers = append(reviewers, args[i+1])
						}
					}
				}
				return host.run(ctx, dir, env, name, args...)
			}
			result, err := Prepare(t.Context(), work, cfg, "widget", run)
			if err != nil {
				t.Fatal(err)
			}
			if result.PullRequest == 0 {
				t.Fatalf("result=%+v, want an opened pull request", result)
			}
			want := []string{tc.operator}
			if tc.operator == "" {
				want = nil
			}
			if !slices.Equal(reviewers, want) {
				t.Fatalf("reviewers requested = %v, want %v", reviewers, want)
			}
		})
	}
}

func TestPrepareLockCommitsOnlyRecordsAndRemovesWorkingFiles(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	oldHead := gitFixture(t, work, "rev-parse", "HEAD")
	result, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Verdict.Ready || result.PullRequest != 17 || result.Head == oldHead || host.pushes != 1 || host.reviews != 1 {
		t.Fatalf("result=%+v pushes=%d reviews=%d", result, host.pushes, host.reviews)
	}
	if head := gitFixture(t, work, "rev-parse", "HEAD"); head != result.Head {
		t.Fatalf("local head %s, prepared %s", head, result.Head)
	}
	if body := gitFixture(t, work, "status", "--porcelain"); body != "" {
		t.Fatalf("dirty prepared tree: %s", body)
	}
	if _, err := os.Stat(filepath.Join(work, "drafts/widget/prd.md")); !os.IsNotExist(err) {
		t.Fatalf("working PRD remains: %v", err)
	}
	changed := gitFixture(t, work, "diff", "--no-renames", "--name-status", oldHead, result.Head)
	for line := range strings.SplitSeq(changed, "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "A\tmemory/") && !strings.HasPrefix(line, "D\tdrafts/widget/") {
			t.Fatalf("unexpected prepared change %q", line)
		}
	}
	if !strings.Contains(host.body, "## 0.") || !strings.Contains(host.body, "memory/prd/widget/") || !strings.Contains(host.body, "memory/design/widget/") {
		t.Fatalf("incomplete approval request: %s", host.body)
	}
}

func TestPrepareRefusalKeepsOriginalDraft(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reviews  bool
		failPush bool
		want     string
	}{
		{"readiness", false, false, "readiness refused"},
		{"publication", true, true, "push prepared lock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, tc.reviews)
			host.failPush = tc.failPush
			oldHead := gitFixture(t, work, "rev-parse", "HEAD")
			original, err := os.ReadFile(filepath.Join(work, "drafts/widget/prd.md"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			after, err := os.ReadFile(filepath.Join(work, "drafts/widget/prd.md"))
			if err != nil || string(after) != string(original) || gitFixture(t, work, "rev-parse", "HEAD") != oldHead {
				t.Fatalf("failed preparation changed draft: read=%v head=%s", err, gitFixture(t, work, "rev-parse", "HEAD"))
			}
			if got := gitFixture(t, work, "status", "--porcelain"); got != "" {
				t.Fatalf("failed preparation dirtied draft: %s", got)
			}
		})
	}
}

func TestPrepareRetainsOpenedAndPendingEventsInMemory(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	prd, err := os.ReadFile(filepath.Join(work, "drafts/widget/prd.md"))
	if err != nil {
		t.Fatal(err)
	}
	tech, err := os.ReadFile(filepath.Join(work, "drafts/widget/tech-spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := events.New(events.DraftOpened, "widget", compile.ContractDigest(prd, tech), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	opened.Origin = "new"
	log, _, err := events.Append(nil, opened)
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, work, "drafts/widget/events.md", log)
	gitFixture(t, work, "add", "--", "drafts/widget/events.md")
	gitFixture(t, work, "commit", "-m", "record draft opening")
	gitFixture(t, work, "push", "origin", "draft/widget")
	queue, err := events.OpenPending(work, "origin", "widget", 1)
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
	if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(work, "memory/feature/widget/*.md"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("event Document paths = %v, err = %v", paths, err)
	}
	retained, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	got, err := events.Parse(retained)
	if err != nil || len(got) != 2 || got[0].ID != opened.ID || got[1].ID != failed.ID {
		t.Fatalf("retained lock events = %+v, err = %v", got, err)
	}
}

func TestRefusedLockEventTravelsToFreshClone(t *testing.T) {
	work, cfg, host := prepareFixture(t, false)
	before := gitFixture(t, work, "status", "--porcelain")
	verdict, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err == nil || !strings.Contains(err.Error(), "readiness refused") || len(verdict.Verdict.Unmet) == 0 || gitFixture(t, work, "status", "--porcelain") != before {
		t.Fatalf("refusal=%+v error=%v", verdict, err)
	}
	queue, err := events.OpenPending(work, "origin", "widget", 1)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := queue.Read(t.Context())
	if err != nil || len(pending) != 1 || pending[0].Name != events.LockRefused || pending[0].VersionDigest != verdict.Verdict.DraftDigest || len(pending[0].UnmetConditions) != len(verdict.Verdict.Unmet) {
		t.Fatalf("pending refusal=%+v err=%v", pending, err)
	}
	clone := filepath.Join(t.TempDir(), "fresh")
	gitFixture(t, work, "clone", "-b", "draft/widget", gitFixture(t, work, "remote", "get-url", "origin"), clone)
	gitFixture(t, clone, "config", "user.name", "Draft Agent")
	gitFixture(t, clone, "config", "user.email", "agent@example.test")
	prd, err := os.ReadFile(filepath.Join(clone, "drafts/widget/prd.md"))
	if err != nil {
		t.Fatal(err)
	}
	tech, err := os.ReadFile(filepath.Join(clone, "drafts/widget/tech-spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	digest := compile.ContractDigest(prd, tech)
	fixtureFile(t, clone, "drafts/widget/reviews/001.md", []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** "+digest+"\n"))
	fixtureFile(t, clone, "drafts/widget/reviews/002.md", []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** "+digest+"\n"))
	gitFixture(t, clone, "add", "--", "drafts/widget/reviews/001.md", "drafts/widget/reviews/002.md")
	gitFixture(t, clone, "commit", "-m", "finish reviews")
	gitFixture(t, clone, "push", "origin", "draft/widget")
	if _, err := Prepare(t.Context(), clone, cfg, "widget", host.run); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(clone, "memory/feature/widget/*.md"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("event Document paths = %v, err = %v", paths, err)
	}
	body, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	retained, err := events.Parse(body)
	if err != nil || len(retained) != 1 || retained[0].ID != pending[0].ID {
		t.Fatalf("fresh clone events=%+v err=%v", retained, err)
	}
}

func TestRepeatedPreparationRetainsOneRuledChange(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	prdPath := filepath.Join(work, "drafts/widget/prd.md")
	prd, err := os.ReadFile(prdPath)
	if err != nil {
		t.Fatal(err)
	}
	revised := []byte(strings.Replace(string(prd), "The widget shall open.", "The widget shall queue the open action.", 1))
	fixtureFile(t, work, "drafts/widget/prd.md", revised)
	fixtureFile(t, work, "drafts/widget/problems.md", []byte("# Problems\n\n### P-1 — FR-1 — Queue semantics\n**Problem:** Opening while offline needs a queue.\n**Ruling:** operator · Preserve the request until online. · Queue the open action.\n"))
	tech, err := os.ReadFile(filepath.Join(work, "drafts/widget/tech-spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	digest := compile.ContractDigest(revised, tech)
	fixtureFile(t, work, "drafts/widget/reviews/001.md", []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** "+digest+"\n"))
	fixtureFile(t, work, "drafts/widget/reviews/002.md", []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** "+digest+"\n"))
	gitFixture(t, work, "add", "--", "drafts/widget/prd.md", "drafts/widget/problems.md", "drafts/widget/reviews/001.md", "drafts/widget/reviews/002.md")
	gitFixture(t, work, "commit", "-m", "rule queued opening")
	gitFixture(t, work, "push", "origin", "draft/widget")
	host.failPush = true
	if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err == nil || !strings.Contains(err.Error(), "push prepared lock") {
		t.Fatalf("first preparation error = %v", err)
	}
	host.failPush = false
	if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(work, "memory/feature/widget/*.md"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("event Document paths = %v, err = %v", paths, err)
	}
	body, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	retained, err := events.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	changed := 0
	for _, event := range retained {
		if event.Name == events.RequirementChanged {
			changed++
			if event.Requirement != "FR-1" || event.RulingAuthor != "operator" || event.Problem != "P-1" || !event.ReasonPresent || !strings.HasPrefix(event.Key, "ruling:") {
				t.Fatalf("incomplete change event: %+v", event)
			}
		}
	}
	if changed != 1 {
		t.Fatalf("repeated preparation retained %d change events: %+v", changed, retained)
	}
}

func TestPrepareRebasesAndJudgesCurrentMainline(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	gitFixture(t, work, "switch", "main")
	fixtureFile(t, work, "README.md", []byte("base changed on mainline\n"))
	gitFixture(t, work, "add", "--", "README.md")
	gitFixture(t, work, "commit", "-m", "advance mainline")
	gitFixture(t, work, "push", "origin", "main")
	mainline := gitFixture(t, work, "rev-parse", "HEAD")
	gitFixture(t, work, "switch", "draft/widget")
	prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Verdict.Mainline != mainline {
		t.Fatalf("judged mainline %s, want %s", prepared.Verdict.Mainline, mainline)
	}
	gitFixture(t, work, "merge-base", "--is-ancestor", mainline, prepared.Head)
}

func TestPrepareRefreshesDriveBindingAndLeavesGeneratedCommitUnstamped(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	bin := filepath.Join(work, ".pyxis", "bin", "argus-stamp")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, work, ".git/info/exclude", []byte(".pyxis/bin/\n"))
	build := exec.Command("go", "build", "-o", bin, "./cmd/argus-stamp")
	build.Dir = filepath.Join("..", "..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build argus-stamp: %v\n%s", err, out)
	}
	hooks := filepath.Join(t.TempDir(), "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	// An installed hook resolves its tool from this worktree's ignored cache.
	// A linked worktree must reuse the original cache during rebase.
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte("#!/bin/sh\nroot=$(git rev-parse --show-toplevel)\nexec \"$root/.pyxis/bin/argus-stamp\" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "config", "core.hooksPath", hooks)
	gitFixture(t, work, "reset", "--soft", "HEAD^")
	t.Setenv("ARGUS_DRIVE_SLUG", "widget")
	gitFixture(t, work, "commit", "-C", "ORIG_HEAD")
	oldDrive := gitFixture(t, work, "rev-parse", "HEAD")
	gitFixture(t, work, "push", "--force-with-lease", "origin", "draft/widget")
	t.Setenv("ARGUS_DRIVE_SLUG", "")
	gitFixture(t, work, "switch", "main")
	fixtureFile(t, work, "base.txt", []byte("mainline advanced\n"))
	gitFixture(t, work, "add", "--", "base.txt")
	gitFixture(t, work, "commit", "-m", "advance mainline")
	gitFixture(t, work, "push", "origin", "main")
	base := gitFixture(t, work, "rev-parse", "HEAD")
	gitFixture(t, work, "switch", "draft/widget")
	// The caller's drive context must not stamp Pyxis's generated commit.
	t.Setenv("ARGUS_DRIVE_SLUG", "widget")
	checkedPreparedTree := false
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "go" && len(args) == 4 && args[1] == "./cmd/memory-gate" {
			repo, err := gitsrc.Open(ctx, dir)
			if err != nil {
				return nil, err
			}
			dirty, err := repo.DirtyPathNames(ctx, []string{"."})
			if err != nil {
				return nil, err
			}
			if len(dirty) != 0 {
				return nil, fmt.Errorf("prepared tree is dirty: %v", dirty)
			}
			checkedPreparedTree = true
		}
		return host.run(ctx, dir, env, name, args...)
	}
	prepared, err := Prepare(t.Context(), work, cfg, "widget", run)
	if err != nil {
		t.Fatal(err)
	}
	if !checkedPreparedTree {
		t.Fatal("preparation skipped the native dirty-path check")
	}
	if body := gitFixture(t, work, "log", "-1", "--format=%B"); strings.Contains(body, "Argus-Drive:") || strings.Contains(body, "Argus-Bind:") {
		t.Fatalf("generated lock commit inherited drive stamp: %q", body)
	}
	rebasedDrive := gitFixture(t, work, "rev-parse", "HEAD^")
	if rebasedDrive == oldDrive {
		t.Fatal("drive commit was not rebased")
	}
	wantBind := attribution.ComputeBinding(gitFixture(t, work, "rev-parse", rebasedDrive+"^{tree}"), []string{base})
	if body := gitFixture(t, work, "log", "-1", "--format=%B", rebasedDrive); !strings.Contains(body, "Argus-Drive: widget") || !strings.Contains(body, "Argus-Bind: "+wantBind) {
		t.Fatalf("rebased draft commit has stale or missing binding: %q", body)
	}
	if prepared.Head != gitFixture(t, work, "rev-parse", "HEAD") {
		t.Fatalf("prepared head %s differs from worktree", prepared.Head)
	}
}

func TestPrepareRejectsStaleSourceStampBeforeRebase(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	gitFixture(t, work, "commit", "--amend", "-m", "open draft\n\nArgus-Drive: widget\nArgus-Bind: "+strings.Repeat("0", 64))
	staleHead := gitFixture(t, work, "rev-parse", "HEAD")
	gitFixture(t, work, "push", "--force-with-lease", "origin", "draft/widget")
	if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err == nil || !strings.Contains(err.Error(), "stale or replayed Argus binding") {
		t.Fatalf("prepare error = %v, want stale source stamp refusal", err)
	}
	if got := gitFixture(t, work, "rev-parse", "HEAD"); got != staleHead {
		t.Fatalf("rejected preparation moved draft head to %s, want %s", got, staleHead)
	}
}

func TestPrepareRejectsMergeSourceBeforeRebase(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	gitFixture(t, work, "switch", "-c", "draft/side")
	fixtureFile(t, work, "drafts/widget/side.md", []byte("side round\n"))
	gitFixture(t, work, "add", "--", "drafts/widget/side.md")
	gitFixture(t, work, "commit", "-m", "side round")
	gitFixture(t, work, "switch", "draft/widget")
	fixtureFile(t, work, "drafts/widget/main.md", []byte("main round\n"))
	gitFixture(t, work, "add", "--", "drafts/widget/main.md")
	gitFixture(t, work, "commit", "-m", "main round")
	gitFixture(t, work, "merge", "--no-ff", "draft/side", "-m", "merge draft rounds")
	mergeHead := gitFixture(t, work, "rev-parse", "HEAD")
	gitFixture(t, work, "push", "origin", "draft/widget")
	if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err == nil || !strings.Contains(err.Error(), "single-parent source commit") {
		t.Fatalf("prepare error = %v, want merge-source refusal", err)
	}
	if got := gitFixture(t, work, "rev-parse", "HEAD"); got != mergeHead {
		t.Fatalf("rejected preparation moved draft head to %s, want %s", got, mergeHead)
	}
	if _, err := os.Stat(filepath.Join(work, "drafts/widget/side.md")); err != nil {
		t.Fatalf("rejected preparation lost side branch content: %v", err)
	}
	if host.pushes != 0 {
		t.Fatalf("rejected preparation published %d push(es)", host.pushes)
	}
}

func TestVerifySourceStampsRefusesUnreadableHistory(t *testing.T) {
	for _, tc := range []struct {
		name, failAt, want string
	}{
		{"fork unavailable", "merge-base", "find draft fork"},
		{"log unavailable", "log", "read source commits"},
		{"malformed commit record", "malformed", "malformed source commit metadata"},
		{"merge commit", "merge", "single-parent source commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
				if name != "git" {
					return nil, fmt.Errorf("unexpected command %s", name)
				}
				switch args[0] {
				case "merge-base":
					if tc.failAt == "merge-base" {
						return nil, errors.New("history unavailable")
					}
					return []byte("fork\n"), nil
				case "log":
					if tc.failAt == "log" {
						return nil, errors.New("commit log unavailable")
					}
					if tc.failAt == "merge" {
						return []byte("merge\x1ftree\x1fparent-a parent-b\x1funstamped\x1e"), nil
					}
					return []byte("malformed\x1e"), nil
				}
				return nil, fmt.Errorf("unexpected git command %v", args)
			}
			if err := verifySourceStamps(t.Context(), t.TempDir(), "base", "head", run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("verify error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSharePreparationCacheKeepsInstalledDirectoryIgnore(t *testing.T) {
	t.Parallel()
	root, tmp := t.TempDir(), t.TempDir()
	fixtureFile(t, root, ".pyxis/bin/argus-stamp", []byte("verified cached stamper\n"))
	fixtureFile(t, root, ".pyxis/bin/releases/pin.json", []byte("verified manifest\n"))
	gitFixture(t, tmp, "init", "-b", "main")
	gitFixture(t, tmp, "config", "user.name", "Preparation Test")
	gitFixture(t, tmp, "config", "user.email", "preparation@example.test")
	gitFixture(t, tmp, "config", "core.hooksPath", "/dev/null")
	fixtureFile(t, tmp, ".gitignore", []byte(".pyxis/bin/\n"))
	gitFixture(t, tmp, "add", "--", ".gitignore")
	gitFixture(t, tmp, "commit", "-m", "installed directory ignore")
	if err := sharePreparationCache(root, tmp); err != nil {
		t.Fatal(err)
	}
	repo, err := gitsrc.Open(t.Context(), tmp)
	if err != nil {
		t.Fatal(err)
	}
	if dirty, err := repo.DirtyPathNames(t.Context(), []string{"."}); err != nil || len(dirty) != 0 {
		t.Fatalf("shared cache dirties the prepared tree: paths %v, error %v", dirty, err)
	}
	for _, path := range []string{"argus-stamp", "releases/pin.json"} {
		want, err := os.ReadFile(filepath.Join(root, ".pyxis", "bin", path))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(tmp, ".pyxis", "bin", path))
		if err != nil || string(got) != string(want) {
			t.Fatalf("shared %s = %q, %v; want %q", path, got, err, want)
		}
	}
	fixtureFile(t, tmp, "unexpected.txt", []byte("must remain dirty\n"))
	if dirty, err := repo.DirtyPathNames(t.Context(), []string{"."}); err != nil || !slices.Equal(dirty, []string{"unexpected.txt"}) {
		t.Fatalf("unrelated untracked file escaped the predicate: paths %v, error %v", dirty, err)
	}
}

func TestSharePreparationCacheRefusesUnsafePaths(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		setup      func(t *testing.T, root, tmp string)
	}{
		{"source is a file", "not a directory", func(t *testing.T, root, _ string) {
			fixtureFile(t, root, ".pyxis/bin", []byte("not a cache"))
		}},
		{"source cannot be inspected", "too many levels of symbolic links", func(t *testing.T, root, _ string) {
			if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("bin", filepath.Join(root, ".pyxis", "bin")); err != nil {
				t.Fatal(err)
			}
		}},
		{"source entries cannot be read", "permission denied", func(t *testing.T, root, _ string) {
			fixtureFile(t, root, ".pyxis/bin/argus-stamp", []byte("retained cached stamper"))
			cache := filepath.Join(root, ".pyxis", "bin")
			if err := os.Chmod(cache, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(cache, 0o755); err != nil {
					t.Error(err)
				}
			})
			if _, err := os.ReadDir(cache); err == nil {
				t.Skip("filesystem permissions permit reading a mode-000 cache")
			}
		}},
		{"destination cannot be inspected", "permission denied", func(t *testing.T, root, tmp string) {
			if err := os.MkdirAll(filepath.Join(root, ".pyxis", "bin"), 0o755); err != nil {
				t.Fatal(err)
			}
			parent := filepath.Join(tmp, ".pyxis")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(parent, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(parent, 0o755); err != nil {
					t.Error(err)
				}
			})
			if _, err := os.Lstat(filepath.Join(parent, "bin")); errors.Is(err, os.ErrNotExist) {
				t.Skip("filesystem permissions permit searching a mode-000 parent")
			}
		}},
		{"destination exists", "already exists", func(t *testing.T, root, tmp string) {
			if err := os.MkdirAll(filepath.Join(root, ".pyxis", "bin"), 0o755); err != nil {
				t.Fatal(err)
			}
			fixtureFile(t, tmp, ".pyxis/bin", []byte("occupied"))
		}},
		{"destination parent is a file", "not a directory", func(t *testing.T, root, tmp string) {
			if err := os.MkdirAll(filepath.Join(root, ".pyxis", "bin"), 0o755); err != nil {
				t.Fatal(err)
			}
			fixtureFile(t, tmp, ".pyxis", []byte("occupied"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, tmp := t.TempDir(), t.TempDir()
			tc.setup(t, root, tmp)
			if err := sharePreparationCache(root, tmp); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("share cache error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestPrepareNamedStepFailuresPreserveLocalDraft(t *testing.T) {
	for _, tc := range []struct {
		name, command, want string
	}{
		{"identity", "gh api user", "verify agent identity"},
		{"branch", "git branch --show-current", "inspect draft branch"},
		{"status", "git status --porcelain", "inspect draft working files"},
		{"draft head", "git rev-parse HEAD", "resolve draft head"},
		{"fetch", "git fetch origin main", "fetch current mainline"},
		{"mainline", "git rev-parse origin/main", "resolve current mainline"},
		{"remote", "git ls-remote origin refs/heads/draft/widget", "inspect remote draft branch"},
		{"worktree", "git worktree add --detach", "create preparation worktree"},
		{"rebase", "git rebase", "rebase draft"},
		{"remove", "git rm -r", "remove prepared working files"},
		{"stage", "git add -- memory/", "stage compiled records"},
		{"commit", "git commit -m docs(draft): prepare lock widget", "commit prepared lock"},
		{"gate", "go run ./cmd/memory-gate", "memory gate at prepared tree"},
		{"repo", "gh repo view", "resolve code-host repository"},
		{"pr", "gh pr view", "find draft pull request"},
		{"body", "gh pr edit 17 --body", "publish approval request"},
		{"ready", "gh pr ready 17", "mark prepared PR"},
		{"reviewer", "gh pr edit 17 --add-reviewer", "request operator review"},
		{"reset", "git reset --hard", "align prepared working tree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			oldHead := gitFixture(t, work, "rev-parse", "HEAD")
			before, err := os.ReadFile(filepath.Join(work, "drafts/widget/prd.md"))
			if err != nil {
				t.Fatal(err)
			}
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if strings.Contains(name+" "+strings.Join(args, " "), tc.command) {
					return nil, errors.New("simulated step failure")
				}
				return host.run(ctx, dir, env, name, args...)
			}
			if _, err := Prepare(t.Context(), work, cfg, "widget", run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
			after, err := os.ReadFile(filepath.Join(work, "drafts/widget/prd.md"))
			if err != nil || string(after) != string(before) || gitFixture(t, work, "rev-parse", "HEAD") != oldHead {
				t.Fatalf("failed %s changed local draft: read=%v", tc.name, err)
			}
		})
	}
}

func TestPrepareRejectsMalformedEventSourcesWithoutChangingDraft(t *testing.T) {
	for _, tc := range []string{"working log", "pending ref"} {
		t.Run(tc, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			if tc == "working log" {
				fixtureFile(t, work, "drafts/widget/events.md", []byte("malformed\n"))
				gitFixture(t, work, "add", "--", "drafts/widget/events.md")
				gitFixture(t, work, "commit", "-m", "corrupt draft events")
				gitFixture(t, work, "push", "origin", "draft/widget")
			} else {
				store, err := gitrefstore.New(work, "origin", "refs/pyxis/draft-pending/widget/v1")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.Update(t.Context(), "events.md", "corrupt pending events", func([]byte) ([]byte, bool, error) { return []byte("malformed\n"), true, nil }); err != nil {
					t.Fatal(err)
				}
			}
			before := gitFixture(t, work, "rev-parse", "HEAD")
			_, err := Prepare(t.Context(), work, cfg, "widget", host.run)
			want := "read pending draft events"
			if tc == "working log" {
				want = "compile rebased draft"
			}
			if err == nil || !strings.Contains(err.Error(), want) || gitFixture(t, work, "rev-parse", "HEAD") != before || gitFixture(t, work, "status", "--porcelain") != "" {
				t.Fatalf("malformed %s: error=%v", tc, err)
			}
		})
	}
}

func TestPreparePreflightRefusals(t *testing.T) {
	if _, err := Prepare(t.Context(), "", config.Defaults(), "Bad/feature", nil); err == nil || !strings.Contains(err.Error(), "valid feature") {
		t.Fatalf("invalid feature: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*testing.T, string, *config.Config, *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error)
		want string
	}{
		{"missing identity", func(_ *testing.T, _ string, cfg *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			cfg.Draft.AgentLogin = ""
			return host.run
		}, "configure the agent identity"},
		{"missing token", func(t *testing.T, _ string, cfg *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			t.Setenv(cfg.Draft.AgentTokenEnv, "")
			return host.run
		}, "is not set"},
		{"wrong agent", func(_ *testing.T, _ string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "gh" && len(args) > 1 && args[1] == "user" {
					return []byte("somebody-else"), nil
				}
				return host.run(ctx, dir, env, name, args...)
			}
		}, "differs from configured draft agent"},
		{"wrong branch", func(t *testing.T, work string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			gitFixture(t, work, "switch", "main")
			return host.run
		}, "not main"},
		{"dirty", func(t *testing.T, work string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			fixtureFile(t, work, "untracked.txt", []byte("dirty"))
			return host.run
		}, "commit or move working changes"},
		{"remote absent", func(_ *testing.T, _ string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) > 0 && args[0] == "ls-remote" {
					return nil, nil
				}
				return host.run(ctx, dir, env, name, args...)
			}
		}, "publish prepared lock"},
		{"remote moved", func(_ *testing.T, _ string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) > 0 && args[0] == "ls-remote" {
					return []byte(strings.Repeat("f", 40) + "\trefs/heads/draft/widget\n"), nil
				}
				return host.run(ctx, dir, env, name, args...)
			}
		}, "moved"},
		{"bad repository URL", func(_ *testing.T, _ string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "gh" && len(args) > 1 && args[1] == "view" && args[0] == "repo" {
					return []byte("https://token@github.com/example/repo"), nil
				}
				return host.run(ctx, dir, env, name, args...)
			}
		}, "without credentials"},
		{"bad PR number", func(_ *testing.T, _ string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "view" {
					return []byte("unknown"), nil
				}
				return host.run(ctx, dir, env, name, args...)
			}
		}, "invalid draft pull request number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			run := tc.edit(t, work, &cfg, host)
			oldHead := gitFixture(t, work, "rev-parse", "HEAD")
			if _, err := Prepare(t.Context(), work, cfg, "widget", run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
			if head := gitFixture(t, work, "rev-parse", "HEAD"); head != oldHead {
				t.Fatalf("refusal moved branch to %s", head)
			}
		})
	}
}

func TestPrepareFailureReportsPendingRefErrors(t *testing.T) {
	cause := errors.New("step broke")
	cfg := config.Defaults()
	cfg.Remote = ""
	if err := recordPrepareFailure(t.Context(), t.TempDir(), cfg, "widget", 1, "", cause); !errors.Is(err, cause) || !strings.Contains(err.Error(), "queue preparation failure") {
		t.Fatalf("invalid pending ref error = %v", err)
	}
	cfg.Remote = "origin"
	if err := recordPrepareFailure(t.Context(), t.TempDir(), cfg, "widget", 1, "", cause); !errors.Is(err, cause) || !strings.Contains(err.Error(), "append pending draft event") {
		t.Fatalf("unavailable pending ref error = %v", err)
	}
}

func TestRoundRestoresExactWorkingFilesAndDigest(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	original, err := os.ReadFile(filepath.Join(work, "drafts/widget/prd.md"))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := Round(t.Context(), work, cfg, "widget", host.run)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Head == prepared.Head || resumed.Digest != prepared.Verdict.DraftDigest || host.pushes != 2 {
		t.Fatalf("prepared=%+v resumed=%+v pushes=%d", prepared, resumed, host.pushes)
	}
	after, err := os.ReadFile(filepath.Join(work, "drafts/widget/prd.md"))
	if err != nil || string(after) != string(original) {
		t.Fatalf("round did not restore exact PRD: %v", err)
	}
	if got := gitFixture(t, work, "status", "--porcelain"); got != "" {
		t.Fatalf("round left dirty tree: %s", got)
	}
	if remote := gitFixture(t, work, "ls-remote", "origin", "refs/heads/draft/widget"); !strings.HasPrefix(remote, resumed.Head) {
		t.Fatalf("remote branch = %q, want %s", remote, resumed.Head)
	}
}

func TestRoundRequiresPreparationAtHead(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	oldHead := gitFixture(t, work, "rev-parse", "HEAD")
	if _, err := Round(t.Context(), work, cfg, "widget", host.run); err == nil || !strings.Contains(err.Error(), "latest commit is not") {
		t.Fatalf("error=%v", err)
	}
	if got := gitFixture(t, work, "rev-parse", "HEAD"); got != oldHead {
		t.Fatalf("refusal moved branch to %s", got)
	}
}

func TestRoundPreflightRefusals(t *testing.T) {
	if _, err := Round(t.Context(), "", config.Defaults(), "Bad/feature", nil); err == nil || !strings.Contains(err.Error(), "valid feature") {
		t.Fatalf("invalid feature: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*testing.T, string, *config.Config, *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error)
		want string
	}{
		{"wrong branch", func(t *testing.T, work string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			gitFixture(t, work, "switch", "main")
			return host.run
		}, "not main"},
		{"dirty", func(t *testing.T, work string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			fixtureFile(t, work, "untracked.txt", []byte("dirty"))
			return host.run
		}, "clean working tree"},
		{"missing digest", func(t *testing.T, work string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			gitFixture(t, work, "commit", "--allow-empty", "-m", "docs(draft): prepare lock widget")
			return host.run
		}, "no draft digest"},
		{"remote moved", func(_ *testing.T, _ string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) > 0 && args[0] == "ls-remote" {
					return []byte(strings.Repeat("f", 40) + "\trefs/heads/draft/widget\n"), nil
				}
				return host.run(ctx, dir, env, name, args...)
			}
		}, "remote draft branch moved"},
		{"missing identity", func(_ *testing.T, _ string, cfg *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			cfg.Draft.AgentLogin = ""
			return host.run
		}, "configure the agent identity"},
		{"missing token", func(t *testing.T, _ string, cfg *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			t.Setenv(cfg.Draft.AgentTokenEnv, "")
			return host.run
		}, "is not set"},
		{"wrong agent", func(_ *testing.T, _ string, _ *config.Config, host *prepareHost) func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "gh" && len(args) > 1 && args[0] == "api" && args[1] == "user" {
					return []byte("somebody-else"), nil
				}
				return host.run(ctx, dir, env, name, args...)
			}
		}, "differs from configured draft agent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			if tc.name == "remote moved" || tc.name == "missing identity" || tc.name == "missing token" || tc.name == "wrong agent" {
				if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err != nil {
					t.Fatal(err)
				}
			}
			run := tc.edit(t, work, &cfg, host)
			if _, err := Round(t.Context(), work, cfg, "widget", run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestRoundRejectsRestoredDigestMismatch(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "commit", "--amend", "-m", "docs(draft): prepare lock widget", "-m", "Draft-Digest: sha256:"+strings.Repeat("f", 64))
	gitFixture(t, work, "push", "--force-with-lease", "origin", "HEAD:refs/heads/draft/widget")
	if _, err := Round(t.Context(), work, cfg, "widget", host.run); err == nil || !strings.Contains(err.Error(), "differs from prepared") {
		t.Fatalf("error=%v", err)
	}
}

func TestRoundNamedFailuresKeepPreparedBranch(t *testing.T) {
	for _, tc := range []struct{ name, command, want string }{
		{"branch", "git branch --show-current", "inspect current branch"},
		{"status", "git status --porcelain", "inspect working tree"},
		{"message", "git show -s --format=%B HEAD", "read preparation commit"},
		{"remote", "git ls-remote origin refs/heads/draft/widget", "inspect remote draft branch"},
		{"identity", "gh api user", "verify agent identity"},
		{"worktree", "git worktree add --detach", "create round worktree"},
		{"revert", "git revert --no-edit HEAD", "revert lock preparation"},
		{"merge-base", "git merge-base HEAD", "resolve draft mainline"},
		{"repo", "gh repo view", "resolve code-host repository"},
		{"push", "push --force-with-lease", "push restored round"},
		{"pr", "gh pr view", "find draft pull request"},
		{"ready", "gh pr ready --undo", "return PR #17 to drafting"},
		{"body", "gh pr edit 17 --body", "update draft PR #17"},
		{"reset", "git reset --hard", "align restored working tree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
			if err != nil {
				t.Fatal(err)
			}
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if strings.Contains(name+" "+strings.Join(args, " "), tc.command) {
					return nil, errors.New("simulated round failure")
				}
				return host.run(ctx, dir, env, name, args...)
			}
			if _, err := Round(t.Context(), work, cfg, "widget", run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
			if head := gitFixture(t, work, "rev-parse", "HEAD"); head != prepared.Head {
				t.Fatalf("failed round moved local branch to %s", head)
			}
			if _, err := os.Stat(filepath.Join(work, "drafts/widget/prd.md")); !os.IsNotExist(err) {
				t.Fatalf("failed round restored local working files: %v", err)
			}
		})
	}
}

func TestPreparePublishesAnUnpublishedDraftWithItsOnePullRequest(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	gitFixture(t, work, "push", "-q", "origin", "--delete", "draft/widget")
	result, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err != nil {
		t.Fatal(err)
	}
	if result.PullRequest != 23 || host.created != 1 || host.pushes != 1 || host.reviews != 1 {
		t.Fatalf("result=%+v created=%d pushes=%d reviews=%d", result, host.created, host.pushes, host.reviews)
	}
	if remote := gitFixture(t, work, "ls-remote", "origin", "refs/heads/draft/widget"); !strings.HasPrefix(remote, result.Head+"\t") {
		t.Fatalf("remote branch %q, prepared %s", remote, result.Head)
	}
	if !strings.Contains(host.body, "# Lock request — widget") || !strings.Contains(host.body, "## Also in this pull request") {
		t.Fatalf("pull request opened without the approval request: %s", host.body)
	}
}

func TestPrepareWithdrawsAFirstPublicationWhosePullRequestFails(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	gitFixture(t, work, "push", "-q", "origin", "--delete", "draft/widget")
	oldHead := gitFixture(t, work, "rev-parse", "HEAD")
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return nil, errors.New("simulated host outage")
		}
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "list" {
			return []byte("[]\n"), nil // the host reports no pull request
		}
		return host.run(ctx, dir, env, name, args...)
	}
	if _, err := Prepare(t.Context(), work, cfg, "widget", run); err == nil || !strings.Contains(err.Error(), "branch was withdrawn") {
		t.Fatalf("error = %v", err)
	}
	if remote := gitFixture(t, work, "ls-remote", "origin", "refs/heads/draft/widget"); remote != "" || gitFixture(t, work, "rev-parse", "HEAD") != oldHead {
		t.Fatalf("failed publication left remote %q, local moved=%v", remote, gitFixture(t, work, "rev-parse", "HEAD") != oldHead)
	}
	result, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err != nil || result.PullRequest != 23 {
		t.Fatalf("retry after a withdrawn publication: result=%+v err=%v", result, err)
	}
}

func TestPrepareKeepsAFirstPullRequestWhoseReviewerFailed(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	gitFixture(t, work, "push", "-q", "origin", "--delete", "draft/widget")
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" && len(args) > 1 && args[0] == "pr" && (args[1] == "create" || (args[1] == "edit" && slices.Contains(args, "--add-reviewer"))) {
			return nil, errors.New("pull request update failed")
		}
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "list" {
			return []byte(`[{"number":17,"headRefName":"draft/widget","baseRefName":"main","isCrossRepository":false}]`), nil
		}
		return host.run(ctx, dir, env, name, args...)
	}
	result, err := Prepare(t.Context(), work, cfg, "widget", run)
	if err == nil || !strings.Contains(err.Error(), "request it by hand") || result.PullRequest != 17 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if remote := gitFixture(t, work, "ls-remote", "origin", "refs/heads/draft/widget"); !strings.HasPrefix(remote, result.Head+"\t") || gitFixture(t, work, "rev-parse", "HEAD") != result.Head {
		t.Fatalf("an opened pull request was not kept with its local branch: remote %q head %s", remote, result.Head)
	}
}

// draftEditsPlan changes the live plan in place on the draft branch: the
// plan is merged on mainline first, and the draft edits it.
func draftEditsPlan(t *testing.T, work string) {
	t.Helper()
	mainlinePlan(t, work)
	gitFixture(t, work, "rebase", "-q", "main")
	body, err := os.ReadFile(filepath.Join(work, "docs/plans/widget/001-plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, work, "docs/plans/widget/001-plan.md", append(body, []byte("\n<!-- changed with the contract -->\n")...))
	gitFixture(t, work, "add", "--", "docs/plans/widget/001-plan.md")
	gitFixture(t, work, "commit", "-q", "-m", "change plan widget with its contract")
	gitFixture(t, work, "push", "-q", "-f", "origin", "draft/widget")
}

// mainlinePlan merges the plan source on mainline, as an earlier plan PR did.
func mainlinePlan(t *testing.T, work string) {
	t.Helper()
	body, err := os.ReadFile("../e2e/testdata/widget-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "switch", "-q", "main")
	fixtureFile(t, work, "docs/plans/widget/001-plan.md", body)
	gitFixture(t, work, "add", "--", "docs/plans/widget/001-plan.md")
	gitFixture(t, work, "commit", "-q", "-m", "merge plan 001")
	gitFixture(t, work, "push", "-q", "origin", "main")
	gitFixture(t, work, "switch", "-q", "draft/widget")
}

func TestPrepareMovesALivePlanOnlyWhileItIsInUse(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string, *prepareHost)
		moved bool
	}{
		{"no plan", func(*testing.T, string, *prepareHost) {}, false},
		{"finished plan left alone", func(t *testing.T, work string, h *prepareHost) { mainlinePlan(t, work); h.taskState = "closed" }, false},
		{"unfinished plan", func(t *testing.T, work string, h *prepareHost) { mainlinePlan(t, work); h.taskState = "open" }, true},
		{"unprojected plan", func(t *testing.T, work string, _ *prepareHost) { mainlinePlan(t, work) }, true},
		{"plan the draft edits", func(t *testing.T, work string, _ *prepareHost) { draftEditsPlan(t, work) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			tc.setup(t, work, host)
			moved := false
			rebaseLivePlan = func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
				moved = true
				return nil, nil, nil
			}
			t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
			if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err != nil {
				t.Fatal(err)
			}
			if moved != tc.moved {
				t.Fatalf("plan moved = %v, want %v", moved, tc.moved)
			}
		})
	}
}

// listedTasks is a configured task tracker holding the given work items.
type listedTasks []model.WorkItem

func (l listedTasks) List(context.Context, model.Query) ([]model.WorkItem, []schema.Defect) {
	return l, nil
}

func TestPrepareTrackedReadsTaskStateFromTheConfiguredTracker(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state model.State
		moved bool
	}{
		{"finished on the tracker", model.StateClosed, false},
		{"running on the tracker", model.StateOpen, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			mainlinePlan(t, work)
			host.taskState = "open" // a stale GitHub copy that must not be read
			if tc.state == model.StateOpen {
				host.taskState = "closed"
			}
			moved := false
			rebaseLivePlan = func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
				moved = true
				return nil, nil, nil
			}
			t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
			tasks := listedTasks{{ID: 9, State: tc.state, Body: "<!-- pyxis:plan-task widget/001/T1 -->"}}
			if _, err := PrepareTracked(t.Context(), work, cfg, "widget", host.run, tasks); err != nil {
				t.Fatal(err)
			}
			if moved != tc.moved {
				t.Fatalf("plan moved = %v, want %v", moved, tc.moved)
			}
		})
	}
}

// appendOnlyTasks is a configured task tracker, like Carina, whose task
// comments cannot be rewritten.
type appendOnlyTasks struct{ listedTasks }

func (appendOnlyTasks) AppendOnly() bool { return true }

func TestPrepareRefusesEditingAFinishedPlan(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	draftEditsPlan(t, work)
	host.taskState = "closed"
	oldHead := gitFixture(t, work, "rev-parse", "HEAD")
	_, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err == nil || !strings.Contains(err.Error(), "finished plan stays history") || host.pushes != 0 || gitFixture(t, work, "rev-parse", "HEAD") != oldHead {
		t.Fatalf("edited finished plan prepared: err=%v pushes=%d", err, host.pushes)
	}
}

func TestPrepareRefusesMovingARunningPlanOnAppendOnlyTasks(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	mainlinePlan(t, work)
	oldHead := gitFixture(t, work, "rev-parse", "HEAD")
	tasks := appendOnlyTasks{listedTasks{{ID: 9, State: model.StateOpen, Body: "<!-- pyxis:plan-task widget/001/T1 -->"}}}
	_, err := PrepareTracked(t.Context(), work, cfg, "widget", host.run, tasks)
	if err == nil || !strings.Contains(err.Error(), "cannot change") || host.pushes != 0 || gitFixture(t, work, "rev-parse", "HEAD") != oldHead {
		t.Fatalf("running Carina plan moved: err=%v pushes=%d", err, host.pushes)
	}
	tasks.listedTasks[0].State = model.StateClosed
	if _, err := PrepareTracked(t.Context(), work, cfg, "widget", host.run, tasks); err != nil {
		t.Fatalf("a finished Carina plan blocked preparation: %v", err)
	}
}

func TestPrepareCarriesTheRebasedPlanInThePreparationCommit(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	draftEditsPlan(t, work)
	var pending plan.PendingBasis
	rebaseLivePlan = func(root string, _ config.Config, feature string, p plan.PendingBasis) ([]string, []schema.Defect, error) {
		pending = p
		fixtureFile(t, root, "docs/plans/widget/001-plan.md", []byte("# plan on "+p.Basis+"\n"))
		fixtureFile(t, root, "docs/plans/widget/001-plan.json", []byte(`{"tasks":[{"id":"T1","title":"Open the widget","goal":"g","acceptance":["a"]}]}`))
		return []string{"docs/plans/widget/001-plan.md", "docs/plans/widget/001-plan.json"}, nil, nil
	}
	t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
	result, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Basis != "lock/widget/v1" || len(pending.Commit) != 40 {
		t.Fatalf("plan rebased onto %+v", pending)
	}
	if subject := gitFixture(t, work, "log", "-1", "--format=%s%n%b", result.Head); !strings.Contains(subject, "docs(draft): prepare lock widget") || !strings.Contains(subject, "Draft-Digest: ") {
		t.Fatalf("plan is not part of the preparation commit: %s", subject)
	}
	files := gitFixture(t, work, "show", "--name-only", "--format=", result.Head)
	if !strings.Contains(files, "docs/plans/widget/001-plan.md") || !strings.Contains(files, "memory/design/widget/") {
		t.Fatalf("preparation commit files: %s", files)
	}
	if !strings.Contains(host.body, "changed tasks: T1") || !strings.Contains(host.body, "A docs/plans/widget/001-plan.json") {
		t.Fatalf("approval request hides the plan change: %s", host.body)
	}
}

func TestPrepareRefusesAPlanThatNoLongerFits(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	draftEditsPlan(t, work)
	rebaseLivePlan = func(string, config.Config, string, plan.PendingBasis) ([]string, []schema.Defect, error) {
		return nil, []schema.Defect{{File: "docs/plans/widget/001-plan.md", Reason: `design decision or anchor "D-2" not found`}}, nil
	}
	t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
	oldHead := gitFixture(t, work, "rev-parse", "HEAD")
	_, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err == nil || !strings.Contains(err.Error(), "live plan does not fit") || !strings.Contains(err.Error(), "D-2") {
		t.Fatalf("error = %v", err)
	}
	if host.pushes != 0 || gitFixture(t, work, "rev-parse", "HEAD") != oldHead {
		t.Fatalf("refused preparation published or moved the draft: pushes=%d", host.pushes)
	}
	queue, err := events.OpenPending(work, "origin", "widget", 1)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := queue.Read(t.Context())
	if err != nil || len(pending) != 1 || pending[0].Name != events.DraftStepFailed {
		t.Fatalf("refusal not queued: %+v err=%v", pending, err)
	}
}

func TestPrepareRefusesAPlanChangeThatDropsATask(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	gitFixture(t, work, "switch", "-q", "main")
	fixtureFile(t, work, "docs/plans/widget/001-plan.json", []byte(`{"tasks":[{"id":"T1","title":"a","goal":"g","acceptance":["a"]},{"id":"T2","title":"b","goal":"g","acceptance":["a"]}]}`))
	gitFixture(t, work, "add", "--", "docs/plans/widget/001-plan.json")
	gitFixture(t, work, "commit", "-q", "-m", "merge plan 001")
	gitFixture(t, work, "push", "-q", "origin", "main")
	gitFixture(t, work, "switch", "-q", "draft/widget")
	draftEditsPlan(t, work)
	rebaseLivePlan = func(root string, _ config.Config, _ string, _ plan.PendingBasis) ([]string, []schema.Defect, error) {
		fixtureFile(t, root, "docs/plans/widget/001-plan.json", []byte(`{"tasks":[{"id":"T1","title":"a","goal":"g","acceptance":["a"]}]}`))
		return []string{"docs/plans/widget/001-plan.json"}, nil, nil
	}
	t.Cleanup(func() { rebaseLivePlan = plan.RebaseLive })
	_, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err == nil || !strings.Contains(err.Error(), "removes T2") || host.pushes != 0 {
		t.Fatalf("dropped task accepted: err=%v pushes=%d", err, host.pushes)
	}
}
