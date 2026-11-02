package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	localapproval "github.com/agenticlab-ai/pyxis/internal/adapters/approval/local"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	carinatracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/carina"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/packet"
	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/plangate/precheck"
	"github.com/agenticlab-ai/pyxis/internal/plangate/project"
	"github.com/agenticlab-ai/pyxis/internal/plangate/tracker"
)

type world struct {
	t      *testing.T
	root   string
	remote string
	base   string
	cfg    config.Config
	host   *host
}

type host struct {
	openPR     string
	request    string
	prNumber   int
	taskIssue  int
	taskClosed bool
	failName   string
	failMatch  string
}

func (h *host) run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	if name == h.failName && strings.Contains(strings.Join(args, " "), h.failMatch) {
		return nil, fmt.Errorf("injected %s failure", h.failMatch)
	}
	if name == "gh" {
		switch {
		case slices.Equal(args, []string{"api", "user", "--jq", ".login"}):
			return []byte("agent\n"), nil
		case len(args) >= 2 && args[0] == "api" && args[1] == "--paginate":
			if h.openPR != "" {
				return []byte(h.openPR + "\n"), nil
			}
			return nil, nil
		case slices.Equal(args, []string{"repo", "view", "--json", "url", "--jq", ".url"}):
			return []byte("https://github.com/example/repo\n"), nil
		case slices.Equal(args, []string{"repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"}):
			return []byte("example/repo\n"), nil
		case len(args) >= 3 && slices.Equal(args[:3], []string{"api", "--method", "GET"}):
			if h.taskIssue == 0 {
				return []byte("[]"), nil
			}
			state := "open"
			if h.taskClosed {
				state = "closed"
			}
			body, err := json.Marshal([]map[string]any{{"number": h.taskIssue, "body": dialect.PlanTaskMarker("widget/001/T1"), "state": state}})
			return body, err
		case len(args) >= 2 && args[0] == "pr" && args[1] == "create":
			for i := range args {
				if args[i] == "--head" && i+1 < len(args) {
					h.openPR = args[i+1]
				}
				// Only a prepared lock opens its pull request ready for review,
				// with the approval request as its body; a draft opens as a draft.
				if args[i] == "--body" && i+1 < len(args) && !slices.Contains(args, "--draft") {
					h.request = args[i+1]
				}
			}
			h.prNumber++
			return fmt.Appendf(nil, "https://github.com/example/repo/pull/%d\n", h.prNumber), nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "view":
			return fmt.Appendf(nil, "%d\n", h.prNumber), nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "edit":
			for i := range args {
				if args[i] == "--body" && i+1 < len(args) {
					h.request = args[i+1]
				}
			}
			return nil, nil
		case len(args) >= 2 && args[0] == "pr" && args[1] == "ready":
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected gh invocation %v", args)
	}
	if name == "go" {
		if len(args) >= 3 && slices.Equal(args[:3], []string{"run", "./cmd/memory-gate", "--base"}) {
			return []byte("memory-gate: pass\n"), nil
		}
		return nil, fmt.Errorf("unexpected go invocation %v", args)
	}
	if name == "git" {
		local := append([]string(nil), args...)
		for i, arg := range local {
			if arg == "https://github.com/example/repo.git" {
				local[i] = "origin"
			}
		}
		return draftshell.Run(ctx, dir, env, name, local...)
	}
	return nil, fmt.Errorf("unexpected process %s %v", name, args)
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func approvalFixture(t *testing.T, pull int, head string, reviews []approval.Review) localapproval.Reader {
	t.Helper()
	type row struct {
		Number  int               `json:"number"`
		Head    string            `json:"head"`
		Reviews []approval.Review `json:"reviews"`
	}
	data, err := json.Marshal(struct {
		Schema       string `json:"schema"`
		PullRequests []row  `json:"pull_requests"`
	}{Schema: "draft-approval-fixture/v1", PullRequests: []row{{Number: pull, Head: head, Reviews: reviews}}})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "approval.json")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return localapproval.Reader{Path: file, Operator: "mdarabi"}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newWorld(t *testing.T) *world {
	t.Helper()
	parent := t.TempDir()
	remote := filepath.Join(parent, "remote.git")
	seed := filepath.Join(parent, "seed")
	root := filepath.Join(parent, "fresh")
	git(t, parent, "init", "--bare", "-q", remote)
	git(t, parent, "init", "-q", "-b", "main", seed)
	git(t, seed, "config", "user.name", "Fixture Agent")
	git(t, seed, "config", "user.email", "agent@example.test")
	git(t, seed, "remote", "add", "origin", remote)
	for _, name := range []string{"prd-template.md", "draft-tech-spec-template.md", "draft-problems-template.md", "draft-events-template.md"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "..", ".pyxis", "templates", name))
		if err != nil {
			t.Fatal(err)
		}
		write(t, seed, ".pyxis/templates/"+name, string(body))
	}
	write(t, seed, "README.md", "fixture mainline\n")
	git(t, seed, "add", "--", ".pyxis/templates", "README.md")
	git(t, seed, "commit", "-qm", "fixture mainline")
	git(t, seed, "push", "-q", "-u", "origin", "main")
	git(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	git(t, parent, "clone", "-q", "--no-local", remote, root)
	git(t, root, "config", "user.name", "Fixture Agent")
	git(t, root, "config", "user.email", "agent@example.test")
	cfg := config.Defaults()
	cfg.Draft.AgentLogin, cfg.Draft.AgentTokenEnv = "agent", "PYXIS_DRAFT_E2E_TOKEN"
	t.Setenv(cfg.Draft.AgentTokenEnv, "fixture-token")
	return &world{t: t, root: root, remote: remote, base: git(t, root, "rev-parse", "HEAD"), cfg: cfg, host: &host{prNumber: 16}}
}

func (w *world) commitDraft(message, branch string) {
	w.t.Helper()
	git(w.t, w.root, "add", "--", "drafts/widget")
	git(w.t, w.root, "commit", "-qm", message)
	git(w.t, w.root, "push", "-q", "origin", branch)
}

func readyPRD(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/working-prd.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	body = strings.Replace(body, "| Q-1 | Who owns retries? | PM | lock | open | |", "| Q-1 | Who owns retries? | PM | lock | resolved | The PM owns retries. |", 1)
	body = strings.Replace(body, "## 5. System requirements", "## 1. Summary\n\n## 2. Problem space\n\n## 3. Goals, metrics, and guardrails\n\n## 4. Solution space\n\n## 5. System requirements", 1)
	body = strings.Replace(body, "| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", "### 5.8 Definition of Done\n\n| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", 1)
	body = strings.Replace(body, "## 7. Learning", "## 6. Delivery\n\n## 7. Learning", 1)
	return body + "\n## 8. Contract readiness\n"
}

func buildCarina(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "carina")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", bin, "./cmd/carina")
	cmd.Dir = filepath.Join("..", "..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build Carina: %v\n%s", err, out)
	}
	return bin
}

func review(part, digest string) string {
	return "# " + part + " review\n\n**Review subject:** " + part + "\n**Draft digest:** " + digest + "\n"
}

func TestFreshCloneDraftThroughReopen(t *testing.T) {
	w := newWorld(t)
	opened, err := lifecycle.Open(t.Context(), w.root, w.cfg, "widget", w.host.run)
	if err != nil || opened.Branch != "draft/widget" || opened.PullRequest != "https://github.com/example/repo/pull/17" {
		t.Fatalf("open: %+v %v", opened, err)
	}
	if git(t, w.root, "rev-parse", "origin/main") != w.base {
		t.Fatal("opening changed mainline")
	}
	prd := readyPRD(t)
	tech := "# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n"
	write(t, w.root, "drafts/widget/prd.md", prd)
	write(t, w.root, "drafts/widget/tech-spec.md", tech)
	write(t, w.root, "drafts/widget/problems.md", "# Problems\n")
	write(t, w.root, "drafts/widget/brainstorm/001.md", "# First brainstorm\n\nOpening a widget needs a queue.\n")
	digest := compile.ContractDigest([]byte(prd), []byte(tech))
	write(t, w.root, "drafts/widget/reviews/001.md", review("prd", digest))
	write(t, w.root, "drafts/widget/reviews/002.md", review("tech-spec", digest))
	w.commitDraft("draft round one", opened.Branch)
	// ABUSE-7: an open draft has no planning or executor basis.
	if basis, defects := precheck.Run(t.Context(), w.root, "widget", w.host.run); basis != "" || len(defects) == 0 || !strings.Contains(defects[0].Reason, "no binding locked version") || !strings.Contains(defects[0].Remedy, "approve and merge") {
		t.Fatalf("open draft admitted planning: basis=%q defects=%+v", basis, defects)
	}

	tech = strings.Replace(tech, "Open and queue the widget.", "Open and durably queue the widget.", 1)
	write(t, w.root, "drafts/widget/tech-spec.md", tech)
	write(t, w.root, "drafts/widget/brainstorm/002.md", "# Second brainstorm\n\nThe queue survives retries.\n")
	digest = compile.ContractDigest([]byte(prd), []byte(tech))
	write(t, w.root, "drafts/widget/reviews/003.md", review("prd", digest))
	write(t, w.root, "drafts/widget/reviews/004.md", review("tech-spec", digest))
	w.commitDraft("draft round two: design revision", opened.Branch)

	prd = strings.Replace(prd, "The widget shall open.", "The widget shall open durably.", 1)
	write(t, w.root, "drafts/widget/prd.md", prd)
	write(t, w.root, "drafts/widget/brainstorm/003.md", "# Third brainstorm\n\nThe PRD reflects durability.\n")
	w.commitDraft("draft round three: PRD after design", opened.Branch)
	// ABUSE-3: both reviews became stale after the PRD revision.
	before := git(t, w.root, "rev-parse", "HEAD")
	refused, err := lock.Prepare(t.Context(), w.root, w.cfg, "widget", w.host.run)
	if err == nil || !strings.Contains(err.Error(), "readiness refused") || len(refused.Verdict.Unmet) == 0 {
		t.Fatalf("expected refused lock: %+v %v", refused, err)
	}
	foundStale := false
	for _, unmet := range refused.Verdict.Unmet {
		if unmet.Condition == "FR-16" && strings.HasPrefix(unmet.Item, "reviews/") && strings.Contains(unmet.Remedy, "re-run") {
			foundStale = true
		}
	}
	if !foundStale {
		t.Fatalf("stale-review refusal lacked condition, item, or remedy: %+v", refused.Verdict.Unmet)
	}
	if git(t, w.root, "rev-parse", "HEAD") != before || git(t, w.root, "status", "--porcelain") != "" {
		t.Fatal("refusal changed the working draft")
	}

	digest = compile.ContractDigest([]byte(prd), []byte(tech))
	write(t, w.root, "drafts/widget/reviews/005.md", review("prd", digest))
	write(t, w.root, "drafts/widget/reviews/006.md", review("tech-spec", digest))
	w.commitDraft("refresh reviews for exact contract", opened.Branch)
	// ABUSE-1: a silent rewrite stays visible as unruled in the lock request.
	unruled, err := lock.Prepare(t.Context(), w.root, w.cfg, "widget", w.host.run)
	if err != nil || !unruled.Verdict.Ready || !strings.Contains(w.host.request, "**unruled**") {
		t.Fatalf("silent rewrite not marked in approval request: %+v %v\nrequest: %s", unruled, err, w.host.request)
	}
	if _, err := lock.Round(t.Context(), w.root, w.cfg, "widget", w.host.run); err != nil {
		t.Fatalf("restore draft round: %v", err)
	}
	if restored, err := os.ReadFile(filepath.Join(w.root, "drafts/widget/prd.md")); err != nil || string(restored) != prd {
		t.Fatalf("restored round changed PRD: %v", err)
	}
	write(t, w.root, "drafts/widget/problems.md", "# Problems\n\n### P-1 — FR-1 — Durable opening\n\n**Problem:** Opening must survive retries.\n**Ruling:** operator · Retries cannot lose an opening. · Require durable opening.\n")
	w.commitDraft("rule durability", opened.Branch)
	prepared, err := lock.Prepare(t.Context(), w.root, w.cfg, "widget", w.host.run)
	if err != nil || !prepared.Verdict.Ready || !strings.Contains(w.host.request, "P-1") {
		t.Fatalf("prepare: %+v %v\nrequest: %s", prepared, err, w.host.request)
	}
	git(t, w.root, "switch", "main")
	git(t, w.root, "merge", "--squash", opened.Branch)
	git(t, w.root, "commit", "-qm", "lock widget v1")
	git(t, w.root, "push", "-q", "origin", "main")
	w.host.openPR = ""
	merge := git(t, w.root, "rev-parse", "HEAD")
	request := lock.RecordRequest{Feature: "widget", Remote: "origin", ExpectedVersion: 1, MergeCommit: merge, PullRequest: 17, ApprovedHead: prepared.Head, ReadinessDigest: prepared.Verdict.DraftDigest, ProposalsSinceOpened: 4}
	// ABUSE-2 and ABUSE-8: neither an agent review nor an agent's claim to
	// speak for the operator can supply the exact-head operator approval.
	forged := approvalFixture(t, 17, prepared.Head, []approval.Review{{ID: 1, Author: "agent", State: "APPROVED", CommitID: prepared.Head, Body: "operator approved"}, {ID: 2, Author: "mdarabi", State: "APPROVED", CommitID: before}})
	if _, err := lock.RecordMerge(t.Context(), w.root, request, forged, w.host.run); !errors.Is(err, approval.ErrNoApproval) || !strings.Contains(err.Error(), "widget") || !strings.Contains(err.Error(), prepared.Head) || !strings.Contains(err.Error(), "request operator approval") {
		t.Fatalf("agent and stale approvals authorized lock: %v", err)
	}
	if tags := git(t, w.root, "tag", "--list", "lock/widget/v1"); tags != "" {
		t.Fatalf("unapproved lock tag exists: %s", tags)
	}
	operator := approvalFixture(t, 17, prepared.Head, []approval.Review{{ID: 3, Author: "mdarabi", State: "APPROVED", CommitID: prepared.Head, Body: "reviewed exact contract"}})
	locked, err := lock.RecordMerge(t.Context(), w.root, request, operator, w.host.run)
	if err != nil || locked.Lock == nil || locked.Lock.Version != 1 {
		t.Fatalf("record lock: %+v %v", locked, err)
	}
	clone := filepath.Join(t.TempDir(), "new-clone")
	git(t, filepath.Dir(clone), "clone", "-q", "--no-local", w.remote, clone)
	read, err := lock.ReadLock(t.Context(), clone, "widget", 1, w.host.run)
	if err != nil || read.MergeCommit != merge || read.ApprovedHead != prepared.Head {
		t.Fatalf("fresh clone lock: %+v %v", read, err)
	}
	attack := filepath.Join(t.TempDir(), "in-place-edit")
	// ABUSE-5: a proposal cannot mutate a locked memory record in place.
	git(t, filepath.Dir(attack), "clone", "-q", "--no-local", w.remote, attack)
	attackRepo, err := gitsrc.Open(t.Context(), attack)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := store.Load(t.Context(), gitsrc.TreeSource{Repo: attackRepo, Ref: "refs/tags/lock/widget/v1", Prefix: store.Dir})
	if err != nil {
		t.Fatal(err)
	}
	row := resolve.New(bound).Head("prd/widget", "FR-1")
	if row == nil {
		t.Fatal("locked FR-1 missing")
	}
	modified := *row
	modified.Statement = "The widget shall silently weaken after lock."
	encoded, err := record.Marshal(&modified)
	if err != nil {
		t.Fatal(err)
	}
	write(t, attack, filepath.ToSlash(filepath.Join(store.Dir, row.Path())), string(encoded))
	changed, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(attack, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	check := packet.Compute(packet.Input{Base: bound, Head: changed, Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "other/proposal", BaseRef: "lock/widget/v1"})
	blockedEdit := false
	for _, blocker := range check.Blockers {
		blockedEdit = blockedEdit || blocker.Code == "record-modified" && strings.Contains(blocker.What, "pyxis draft reopen <feature>")
	}
	if check.OK() || !blockedEdit {
		t.Fatalf("in-place edit escaped lock or lacked reopen remedy: %+v", check.Blockers)
	}
	planBody, err := os.ReadFile("testdata/widget-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	planPath := "docs/plans/widget/001-plan.md"
	write(t, clone, planPath, string(planBody))
	if folded := plan.FoldFile(clone, w.cfg, planPath); len(folded.Defects) != 0 || len(folded.JSON) == 0 {
		t.Fatalf("locked plan fold: %+v", folded.Defects)
	}
	if _, err := os.Stat(filepath.Join(clone, "docs/prds/widget")); !os.IsNotExist(err) {
		t.Fatalf("draft-born plan needed a PRD ledger: %v", err)
	}
	git(t, clone, "config", "user.name", "Fixture Agent")
	git(t, clone, "config", "user.email", "agent@example.test")
	git(t, clone, "add", "--", planPath)
	git(t, clone, "commit", "-qm", "plan widget from lock v1")
	git(t, clone, "push", "-q", "origin", "main")
	bin := buildCarina(t)
	itemsRoot := t.TempDir()
	projected, defects := project.Project(clone, w.cfg, planPath, tracker.NewCarina(itemsRoot, bin), dialect.Harbormaster(), project.Options{
		Log: io.Discard,
		MergedBytes: func(rel string) ([]byte, error) {
			return w.host.run(t.Context(), clone, nil, "git", "show", "origin/main:"+rel)
		},
	})
	if len(defects) != 0 || projected == nil || len(projected.Items) != 1 || projected.Routing != "applied" {
		t.Fatalf("plan projection: %+v %v", projected, defects)
	}
	executorTracker, err := carinatracker.NewForPlan(itemsRoot, "widget/001", carinatracker.WithBinary(bin), carinatracker.WithRepository("example/repo"))
	if err != nil {
		t.Fatal(err)
	}
	frontier, err := executorTracker.Frontier(t.Context())
	if err != nil || len(frontier) != 1 || frontier[0].ID != "T1" || frontier[0].Number != projected.Items[0].Issue {
		t.Fatalf("executor pickup: %+v %v", frontier, err)
	}
	w.host.taskIssue = projected.Items[0].Issue
	// An open task no longer blocks a reopen: it keeps running, and the
	// reopened draft's one pull request may change its charter in the plan.
	reopened, err := lifecycle.ReopenWithCause(t.Context(), clone, w.cfg, "widget", "new-need", "issue:42", lock.LoadContract, w.host.run)
	if err != nil || reopened.Version != 2 || reopened.Branch != "draft/widget-v2" || reopened.PullRequest != "" || len(reopened.Unfinished) != 1 || !strings.HasPrefix(reopened.Unfinished[0], "T1 (issue #") {
		t.Fatalf("reopen: %+v %v", reopened, err)
	}
	if remote := git(t, clone, "ls-remote", "origin", "refs/heads/"+reopened.Branch); remote != "" {
		t.Fatalf("reopen published its branch before preparation: %s", remote)
	}
	closeTask := exec.CommandContext(t.Context(), bin, "close", "--root", itemsRoot, strconv.Itoa(projected.Items[0].Issue), "--reason", "completed")
	if out, err := closeTask.CombinedOutput(); err != nil {
		t.Fatalf("complete projected task: %v\n%s", err, out)
	}
	w.host.taskClosed = true
	if remaining, err := executorTracker.Frontier(t.Context()); err != nil || len(remaining) != 0 {
		t.Fatalf("completed task remains in executor frontier: %+v %v", remaining, err)
	}
	seedPRD, err := os.ReadFile(filepath.Join(clone, "drafts/widget/prd.md"))
	if err != nil || string(seedPRD) != prd {
		t.Fatalf("reopen did not seed binding text: %v", err)
	}
	revised := strings.Replace(prd, "| NFR-1 | Performance | Open quickly. | Under 1 ms. | Test |\n", "", 1)
	if revised == prd {
		t.Fatal("fixture did not retire NFR-1")
	}
	write(t, clone, "drafts/widget/prd.md", revised)
	write(t, clone, "drafts/widget/problems.md", "# Problems\n\n### P-2 — NFR-1 — Retire speed target\n\n**Problem:** Durable opening makes the old speed target impossible.\n**Ruling:** operator · Durability takes precedence over the old latency target. · Retire NFR-1.\n")
	write(t, clone, "drafts/widget/brainstorm/004.md", "# Reopen brainstorm\n\nDurability replaces the latency target.\n")
	reopenDigest := compile.ContractDigest([]byte(revised), []byte(tech))
	write(t, clone, "drafts/widget/reviews/007.md", review("prd", reopenDigest))
	write(t, clone, "drafts/widget/reviews/008.md", review("tech-spec", reopenDigest))
	git(t, clone, "add", "--", "drafts/widget")
	git(t, clone, "commit", "-qm", "retire latency target in reopened draft")
	// The finished plan stays history: preparation leaves it alone, publishes
	// the draft, and opens its one pull request with the approval request.
	second, err := lock.Prepare(t.Context(), clone, w.cfg, "widget", w.host.run)
	if err != nil || !second.Verdict.Ready || second.PullRequest != 18 || !strings.Contains(w.host.request, "NFR-1") {
		t.Fatalf("prepare reopened lock: %+v %v\nrequest: %s", second, err, w.host.request)
	}
	git(t, clone, "switch", "main")
	git(t, clone, "merge", "--squash", reopened.Branch)
	git(t, clone, "commit", "-qm", "lock widget v2")
	git(t, clone, "push", "-q", "origin", "main")
	w.host.openPR = ""
	merge2 := git(t, clone, "rev-parse", "HEAD")
	secondOperator := approvalFixture(t, 18, second.Head, []approval.Review{{ID: 4, Author: "mdarabi", State: "APPROVED", CommitID: second.Head}})
	secondLock, err := lock.RecordMerge(t.Context(), clone, lock.RecordRequest{Feature: "widget", Remote: "origin", ExpectedVersion: 2, MergeCommit: merge2, PullRequest: 18, ApprovedHead: second.Head, ReadinessDigest: second.Verdict.DraftDigest, ProposalsSinceOpened: 1}, secondOperator, w.host.run)
	if err != nil || secondLock.Lock == nil || secondLock.Lock.Version != 2 {
		t.Fatalf("record reopened lock: %+v %v", secondLock, err)
	}
	finalClone := filepath.Join(t.TempDir(), "final-clone")
	git(t, filepath.Dir(finalClone), "clone", "-q", "--no-local", w.remote, finalClone)
	oldContract, err := lock.LoadContract(t.Context(), finalClone, "origin", "widget", 1, w.host.run)
	if err != nil || !strings.Contains(string(oldContract.PRD), "NFR-1") {
		t.Fatalf("binding v1 was lost: %+v %v", oldContract, err)
	}
	newContract, err := lock.LoadContract(t.Context(), finalClone, "origin", "widget", 2, w.host.run)
	if err != nil || strings.Contains(string(newContract.PRD), "| NFR-1 |") || string(newContract.TechSpec) != string(oldContract.TechSpec) {
		t.Fatalf("difference lock did not retain unchanged tech spec and retire NFR-1: %+v %v", newContract, err)
	}
}

func readyWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	if _, err := lifecycle.Open(t.Context(), w.root, w.cfg, "widget", w.host.run); err != nil {
		t.Fatal(err)
	}
	prd := readyPRD(t)
	tech := "# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n"
	write(t, w.root, "drafts/widget/prd.md", prd)
	write(t, w.root, "drafts/widget/tech-spec.md", tech)
	write(t, w.root, "drafts/widget/problems.md", "# Problems\n")
	digest := compile.ContractDigest([]byte(prd), []byte(tech))
	write(t, w.root, "drafts/widget/reviews/001.md", review("prd", digest))
	write(t, w.root, "drafts/widget/reviews/002.md", review("tech-spec", digest))
	w.commitDraft("ready draft", "draft/widget")
	return w
}

func preparedMergeWorld(t *testing.T) (*world, lock.RecordRequest) {
	t.Helper()
	w := readyWorld(t)
	prepared, err := lock.Prepare(t.Context(), w.root, w.cfg, "widget", w.host.run)
	if err != nil {
		t.Fatal(err)
	}
	git(t, w.root, "switch", "main")
	git(t, w.root, "merge", "--squash", "draft/widget")
	git(t, w.root, "commit", "-qm", "lock widget v1")
	git(t, w.root, "push", "-q", "origin", "main")
	w.host.openPR = ""
	return w, lock.RecordRequest{Feature: "widget", Remote: "origin", ExpectedVersion: 1,
		MergeCommit: git(t, w.root, "rev-parse", "HEAD"), PullRequest: 17,
		ApprovedHead: prepared.Head, ReadinessDigest: prepared.Verdict.DraftDigest, ProposalsSinceOpened: 1}
}

func treeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	base := filepath.Join(root, "drafts", "widget")
	err := filepath.WalkDir(base, func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && file == base {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(name)] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDraftStepFailuresPreserveWorkingFiles(t *testing.T) {
	t.Run("open publication", func(t *testing.T) {
		w := newWorld(t)
		w.host.failName, w.host.failMatch = "gh", "pr create"
		before := treeBytes(t, w.root)
		_, err := lifecycle.Open(t.Context(), w.root, w.cfg, "widget", w.host.run)
		if err == nil || !strings.Contains(err.Error(), "opening its draft pull request failed") || !strings.Contains(err.Error(), "injected pr create failure") {
			t.Fatalf("open failure did not name step and cause: %v", err)
		}
		if after := treeBytes(t, w.root); !reflect.DeepEqual(after, before) || git(t, w.root, "rev-parse", "HEAD") != w.base {
			t.Fatalf("open failure changed files or head: %+v", after)
		}
	})
	for _, tc := range []struct {
		name, process, match, step string
	}{
		{"worktree", "git", "worktree add", "create preparation worktree"},
		{"memory gate", "go", "./cmd/memory-gate", "memory gate"},
		{"publication", "git", "push", "push prepared lock"},
		{"review request", "gh", "pr edit", "approval request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := readyWorld(t)
			before, head := treeBytes(t, w.root), git(t, w.root, "rev-parse", "HEAD")
			w.host.failName, w.host.failMatch = tc.process, tc.match
			_, err := lock.Prepare(t.Context(), w.root, w.cfg, "widget", w.host.run)
			if err == nil || !strings.Contains(err.Error(), tc.step) || !strings.Contains(err.Error(), "injected") {
				t.Fatalf("step failure did not name %s: %v", tc.step, err)
			}
			if after := treeBytes(t, w.root); !reflect.DeepEqual(after, before) || git(t, w.root, "rev-parse", "HEAD") != head || git(t, w.root, "status", "--porcelain") != "" {
				t.Fatalf("%s failure changed working draft", tc.name)
			}
		})
	}
	t.Run("resume round", func(t *testing.T) {
		w := readyWorld(t)
		if _, err := lock.Prepare(t.Context(), w.root, w.cfg, "widget", w.host.run); err != nil {
			t.Fatal(err)
		}
		before, head := treeBytes(t, w.root), git(t, w.root, "rev-parse", "HEAD")
		w.host.failName, w.host.failMatch = "git", "revert --no-edit"
		if _, err := lock.Round(t.Context(), w.root, w.cfg, "widget", w.host.run); err == nil || !strings.Contains(err.Error(), "revert lock preparation") {
			t.Fatalf("round failure did not name step: %v", err)
		}
		if after := treeBytes(t, w.root); !reflect.DeepEqual(after, before) || git(t, w.root, "rev-parse", "HEAD") != head {
			t.Fatal("failed round changed prepared branch")
		}
	})
	t.Run("reopen seed", func(t *testing.T) {
		w, request := preparedMergeWorld(t)
		operator := approvalFixture(t, 17, request.ApprovedHead, []approval.Review{{ID: 1, Author: "mdarabi", State: "APPROVED", CommitID: request.ApprovedHead}})
		if _, err := lock.RecordMerge(t.Context(), w.root, request, operator, w.host.run); err != nil {
			t.Fatal(err)
		}
		before, head := treeBytes(t, w.root), git(t, w.root, "rev-parse", "HEAD")
		w.host.failName, w.host.failMatch = "git", "commit -m docs(draft): reopen"
		_, err := lifecycle.Reopen(t.Context(), w.root, w.cfg, "widget", lock.LoadContract, w.host.run)
		if err == nil || !strings.Contains(err.Error(), "commit reopen seed") || !strings.Contains(err.Error(), "injected commit -m docs(draft): reopen failure") {
			t.Fatalf("reopen failure did not name step and cause: %v", err)
		}
		if after := treeBytes(t, w.root); !reflect.DeepEqual(after, before) || git(t, w.root, "rev-parse", "HEAD") != head || git(t, w.root, "status", "--porcelain") != "" {
			t.Fatal("failed reopen changed binding working tree")
		}
	})
	t.Run("lock record publication", func(t *testing.T) {
		w, request := preparedMergeWorld(t)
		operator := approvalFixture(t, 17, request.ApprovedHead, []approval.Review{{ID: 1, Author: "mdarabi", State: "APPROVED", CommitID: request.ApprovedHead}})
		before, head := treeBytes(t, w.root), git(t, w.root, "rev-parse", "HEAD")
		w.host.failName, w.host.failMatch = "git", "push origin refs/tags/lock/"
		_, err := lock.RecordMerge(t.Context(), w.root, request, operator, w.host.run)
		if err == nil || !strings.Contains(err.Error(), "publish lock tag") || !strings.Contains(err.Error(), "injected") {
			t.Fatalf("lock record failure did not name step and cause: %v", err)
		}
		if after := treeBytes(t, w.root); !reflect.DeepEqual(after, before) || git(t, w.root, "rev-parse", "HEAD") != head || git(t, w.root, "status", "--porcelain") != "" {
			t.Fatal("failed lock record changed merged working tree")
		}
	})
}

func TestDraftAbuseKeepsTrustedLedgerAuthority(t *testing.T) {
	// ABUSE-6: a document-pipeline feature cannot claim the draft path.
	t.Run("existing feature cannot open draft", func(t *testing.T) {
		w := newWorld(t)
		write(t, w.root, "docs/prds/widget/000-base.md", "# Existing widget contract\n")
		git(t, w.root, "add", "--", "docs/prds/widget/000-base.md")
		git(t, w.root, "commit", "-qm", "existing document contract")
		git(t, w.root, "push", "-q", "origin", "main")
		_, err := lifecycle.Open(t.Context(), w.root, w.cfg, "widget", w.host.run)
		if err == nil || !strings.Contains(err.Error(), "widget has a PRD ledger") || !strings.Contains(err.Error(), "document pipeline") {
			t.Fatalf("existing feature claimed draft override: %v", err)
		}
		if files := treeBytes(t, w.root); len(files) != 0 {
			t.Fatalf("refused existing feature still opened files: %+v", files)
		}
	})
	// ABUSE-4: candidate changes cannot choose a weaker requirement-link rule.
	t.Run("candidate cannot remove trusted ledger authority", func(t *testing.T) {
		root := t.TempDir()
		at := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
		claim := &record.Record{Schema: record.Schema, ID: record.NewID(record.Promise, at, [10]byte{1}), Kind: record.Promise, Subject: "prd/widget", Family: "FR-1", Statement: "Open widgets", Issuer: "fixture", CreatedAt: at.Format(time.RFC3339)}
		decision := &record.Record{Schema: record.Schema, ID: record.NewID(record.Decision, at.Add(time.Second), [10]byte{2}), Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: "Implement opening", Issuer: "fixture", CreatedAt: at.Add(time.Second).Format(time.RFC3339), Relations: []record.Relation{{Type: record.Serves, Target: claim.ID}}}
		for _, r := range []*record.Record{claim, decision} {
			if _, err := store.Write(filepath.Join(root, store.Dir), r); err != nil {
				t.Fatal(err)
			}
		}
		set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
		if err != nil {
			t.Fatal(err)
		}
		for name, snap := range map[string]reconcile.Snapshot{
			"candidate adds ledger":    {PRDFeatures: []string{"widget"}},
			"candidate removes ledger": {TrustedPRDFeatures: map[string]bool{"widget": true}},
		} {
			t.Run(name, func(t *testing.T) {
				report := reconcile.Analyze(snap, set, resolve.New(set), reconcile.Options{})
				blocked := false
				for _, finding := range report.Findings {
					blocked = blocked || finding.Code == "unauthorized-prd-serves" && finding.Blocking
				}
				if !blocked {
					t.Fatalf("candidate-selected native authority admitted: %+v", report.Findings)
				}
			})
		}
	})
}
