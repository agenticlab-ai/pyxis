package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/prdescription/prdescriptiontest"
)

// failedTransportAttempt reproduces the shape of issue #1587: the
// implementation committed a described checkpoint, the first local gate failed
// on dependency transport, the one repair changed nothing, and the no-progress
// guard ended the attempt with no proposal. The worktree also carries a plan
// whose Basis names a lock tag no clone holds yet.
func failedTransportAttempt(t *testing.T) (f *implementNowE2EFixture, args []string, checkpoint, transport string) {
	t.Helper()
	f = directIssueFixture(t)
	transport = filepath.Join(f.stateDir, "transport-unavailable")
	appendDirectIssueConfig(t, f, "  lint-dependencies:\n    command: test ! -e \""+transport+"\"\n")
	plan := filepath.Join(f.repo, "docs", "plans", "widget", "001-plan.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	header := "# Widget — plan 001\n\n| Field | Value |\n|---|---|\n| **Feature** | widget |\n| **PRD** | prd/widget |\n| **Design** | design/widget |\n| **Basis** | lock/widget/v1 |\n\n## Tasks\n"
	if err := os.WriteFile(plan, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	f.gitAt(f.repo, "add", "docs/plans/widget/001-plan.md")
	f.gitAt(f.repo, "commit", "-m", "docs: cite a contract lock")
	f.gitAt(f.repo, "push", "origin", "main")

	var out, diagnostic bytes.Buffer
	number := strconv.Itoa(e2eIssue)
	if code := run(append(f.args(), "--inspect-issue", number), &out, &diagnostic); code != 0 {
		t.Fatalf("inspect %d: %s", code, &diagnostic)
	}
	args = append(f.args(), "--issue", number, "--issue-input", issueInputFile(t, f, out.Bytes()))

	// Recovery never admits: before the first invocation there is nothing to
	// recover, and nothing is launched.
	out.Reset()
	diagnostic.Reset()
	if code := run(append(append([]string{}, args...), "--recover-checkpoint", strings.Repeat("a", 40)), &out, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "requires an admitted contract") {
		t.Fatalf("unadmitted recovery: %d %s %s", code, &out, &diagnostic)
	}
	if len(codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec")) != 0 {
		t.Fatal("unadmitted recovery launched a model")
	}

	if err := os.WriteFile(transport, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diagnostic.Reset()
	if code := run(args, &out, &diagnostic); code != 1 {
		t.Fatalf("transport-failed attempt exit %d: %s %s", code, &out, &diagnostic)
	}
	calls := readE2ECalls(t, f.stateDir)
	implementation := codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")
	if len(implementation) != 2 || len(codexLaunchesWithArg(calls, "danger-full-access")) != 0 || !strings.Contains(out.String(), "made no progress") {
		t.Fatalf("want one implementation and one no-progress repair: %d launches\n%s", len(implementation), &out)
	}
	if _, published, err := readE2EPRState(f.cfg); err != nil || published {
		t.Fatalf("failed attempt published: %v %v", published, err)
	}
	checkpoint = f.gitAt(implementation[0].Dir, "rev-parse", "HEAD")
	want := "result=no_proposal phase=implementation checkpoint=" + checkpoint + " worker_active=false proposal=none"
	if !strings.Contains(diagnostic.String(), want) || !strings.Contains(diagnostic.String(), "--recover-checkpoint "+checkpoint) {
		t.Fatalf("terminal does not name the stopped attempt and its recovery:\n%s", &diagnostic)
	}
	return f, args, checkpoint, transport
}

func recoverIssue(t *testing.T, args []string, revision string, wantCode int, want ...string) string {
	t.Helper()
	var out, diagnostic bytes.Buffer
	code := run(append(append([]string{}, args...), "--recover-checkpoint", revision), &out, &diagnostic)
	if code != wantCode {
		t.Fatalf("recovery exit %d, want %d\nstdout: %s\nstderr: %s", code, wantCode, &out, &diagnostic)
	}
	for _, s := range want {
		if !strings.Contains(out.String()+diagnostic.String(), s) {
			t.Fatalf("recovery output lacks %q\nstdout: %s\nstderr: %s", s, &out, &diagnostic)
		}
	}
	return out.String()
}

func TestDirectIssueRecoversTransportFailedCheckpointWithoutInference(t *testing.T) {
	f, args, checkpoint, transport := failedTransportAttempt(t)
	launches := len(codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec"))
	worktree := codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "--dangerously-bypass-approvals-and-sandbox")[0].Dir
	unpublished := func(stage string) {
		t.Helper()
		calls := readE2ECalls(t, f.stateDir)
		if got := len(codexLaunchesWithArg(calls, "exec")); got != launches {
			t.Fatalf("%s launched %d model processes", stage, got-launches)
		}
		if _, published, err := readE2EPRState(f.cfg); err != nil || published {
			t.Fatalf("%s published a proposal: %v", stage, err)
		}
		if err := f.gitBareTry("rev-parse", "--verify", "--quiet", "refs/heads/"+f.gitAt(worktree, "branch", "--show-current")); err == nil {
			t.Fatalf("%s pushed the task branch", stage)
		}
	}

	// A changed intake is a different contract and never reaches recovery.
	changed := filepath.Join(f.root, "changed-input.json")
	input, err := os.ReadFile(args[len(args)-1])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changed, bytes.Replace(input, []byte("Produce e2e-output.txt"), []byte("Produce something else"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	changedArgs := append(append([]string{}, args[:len(args)-1]...), changed)
	recoverIssue(t, changedArgs, checkpoint, 2, "admit issue", "different admitted contract")

	// The required lock exists nowhere: refused before the gate runs, with a
	// setup action, and no tag is fabricated.
	recoverIssue(t, args, checkpoint, 1, "required contract ref refs/tags/lock/widget/v1", "phase=recovery_contract_refs", "worker_active=false proposal=none", "--recover-checkpoint "+checkpoint)
	if _, err := localGit(t.Context(), f.repo, "rev-parse", "--verify", "--quiet", "refs/tags/lock/widget/v1"); err == nil {
		t.Fatal("recovery created the missing lock tag")
	}
	unpublished("missing contract ref")

	// The canonical tag is fetched by its exact name; the transport is still
	// down, so the complete gate is red and nothing is published.
	main := f.gitBare("rev-parse", "main")
	f.gitBare("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "tag", "-a", "-m", "lock", "lock/widget/v1", main)
	recoverIssue(t, args, checkpoint, 1, "phase=recovery_verification", "local verification failed", "--recover-checkpoint "+checkpoint)
	if tag, err := localGit(t.Context(), f.repo, "rev-parse", "--verify", "lock/widget/v1^{commit}"); err != nil || strings.TrimSpace(string(tag)) != main {
		t.Fatalf("canonical contract ref not fetched: %s %v", tag, err)
	}
	unpublished("red gate")
	if err := os.Remove(transport); err != nil {
		t.Fatal(err)
	}

	// The checkout must be exactly the named, clean checkpoint, and the
	// implementer's description must have been retained for it.
	stray := filepath.Join(worktree, "stray.txt")
	if err := os.WriteFile(stray, []byte("operator scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recoverIssue(t, args, checkpoint, 1, "phase=recovery_checkout", "uncommitted")
	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}
	recoverIssue(t, args, main, 1, "phase=recovery_checkout", "not the expected checkpoint")
	record := filepath.Join(runtime.TaskStateRoot(worktree), "issue-checkpoint.json")
	if err := os.Rename(record, record+".moved"); err != nil {
		t.Fatal(err)
	}
	recoverIssue(t, args, checkpoint, 1, "phase=recovery_description", "no implementer description was retained")
	if err := os.Rename(record+".moved", record); err != nil {
		t.Fatal(err)
	}
	unpublished("refused checkouts")

	// The authorized recovery: one draft proposal at the exact checkpoint, its
	// association and local proof, the original description, and no model.
	stdout := recoverIssue(t, args, checkpoint, 1, "recovered checkpoint "+checkpoint+" as PR #77 without launching a model",
		"result=review_pending phase=review checkpoint="+checkpoint+" worker_active=false proposal=#77")
	calls := readE2ECalls(t, f.stateDir)
	pr, published, err := readE2EPRState(f.cfg)
	if err != nil || !published || f.gitBare("rev-parse", "refs/heads/"+pr.Head) != checkpoint || len(codexLaunchesWithArg(calls, "exec")) != launches {
		t.Fatalf("recovered proposal %+v %v at %d launches: %s", pr, err, len(codexLaunchesWithArg(calls, "exec")), stdout)
	}
	if posts := githubCallsContaining(calls, "POST", "repos/"+f.cfg.Repository+"/pulls"); len(posts) != 1 {
		t.Fatalf("pull request creations=%d", len(posts))
	}
	if !strings.Contains(pr.Body, "- Implement the task.") || !strings.Contains(pr.Body, `"terminal": "review_pending"`) || strings.Contains(pr.Body, `"role": "implementation"`) {
		t.Fatalf("recovered body lost the implementer description or relabeled the attempt:\n%s", pr.Body)
	}
	store, err := issueentry.NewStore(f.repo, "origin")
	if err != nil {
		t.Fatal(err)
	}
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: f.cfg.Repository}
	contract, _, err := store.Load(t.Context(), "github", binding.Key(), e2eIssue)
	if err != nil {
		t.Fatal(err)
	}
	p, associated, err := store.Proposal(t.Context(), contract)
	if err != nil || !associated || p.Number != 77 {
		t.Fatalf("association %+v %v %v", p, associated, err)
	}
	if local, err := store.LocalVerified(t.Context(), contract, p, checkpoint); err != nil || !local {
		t.Fatalf("exact local proof %v %v", local, err)
	}

	// A restart reuses the admitted identity and publishes nothing again.
	recoverIssue(t, args, checkpoint, 1, "already has proposal #77; checkpoint recovery published nothing", "result=review_pending")
	if posts := githubCallsContaining(readE2ECalls(t, f.stateDir), "POST", "repos/"+f.cfg.Repository+"/pulls"); len(posts) != 1 {
		t.Fatalf("restart created another proposal: %d", len(posts))
	}

	// The ordinary command finishes with the independent review only.
	var out, diagnostic bytes.Buffer
	if code := run(args, &out, &diagnostic); code != 0 || !strings.Contains(out.String(), "verified_pr #77") || !strings.Contains(out.String(), "result=verified_pr phase=complete") {
		t.Fatalf("review after recovery %d: %s %s", code, &out, &diagnostic)
	}
	final := readE2ECalls(t, f.stateDir)
	if len(codexLaunchesWithArg(final, "--dangerously-bypass-approvals-and-sandbox")) != 2 || len(codexLaunchesWithArg(final, "danger-full-access")) != 1 {
		t.Fatal("completion launched anything but the independent review")
	}
}

func TestRecoveredIssueContinuationSettlesAndRepairsBeforeFreshReview(t *testing.T) {
	f, args, checkpoint, transport := failedTransportAttempt(t)
	f.gitBare("tag", "lock/widget/v1", f.gitBare("rev-parse", "main"))
	if err := os.Remove(transport); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.stateDir, "checks-pending-once"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	before := len(codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec"))
	recoverIssue(t, args, checkpoint, 1, "without launching a model", "verification_pending")
	if got := len(codexLaunchesWithArg(readE2ECalls(t, f.stateDir), "exec")); got != before {
		t.Fatalf("checkpoint recovery launched %d model processes", got-before)
	}
	for _, marker := range []string{"checks-pending-once", "checks-red"} {
		if err := os.WriteFile(filepath.Join(f.stateDir, marker), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, diagnostic bytes.Buffer
	if code := run(args, &out, &diagnostic); code != 0 || !strings.Contains(out.String(), "verified_pr #77") {
		t.Fatalf("CI continuation %d: %s %s", code, &out, &diagnostic)
	}
	calls := readE2ECalls(t, f.stateDir)
	implementation := codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")
	reviews := codexLaunchesWithArg(calls, "danger-full-access")
	if len(implementation) != 3 || len(reviews) != 1 || !strings.Contains(implementation[2].Stdin, "fixture remote CI failure") {
		t.Fatalf("launches after continuation: implementation=%d review=%d", len(implementation), len(reviews))
	}
	pr, ok, err := readE2EPRState(f.cfg)
	if err != nil || !ok {
		t.Fatal(err)
	}
	repaired := f.gitBare("rev-parse", pr.Head)
	if repaired == checkpoint || f.gitBare("show", repaired+":ci-repaired.txt") != "repaired by the CI-only fixer" {
		t.Fatalf("repair was not published at a fresh head: %s", repaired)
	}
	comment, ok, err := readE2ECommentState(f.cfg)
	if err != nil || !ok || !strings.Contains(comment.Body, repaired) {
		t.Fatalf("fresh review does not bind repaired head %s: %+v %v", repaired, comment, err)
	}
	if f.gitBareTry("show-ref", "--verify", "refs/claim/"+strconv.Itoa(e2eIssue)) == nil {
		t.Fatal("CI continuation left the task claim held")
	}
	store, err := issueentry.NewStore(f.repo, "origin")
	if err != nil {
		t.Fatal(err)
	}
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: f.cfg.Repository}
	contract, _, err := store.Load(t.Context(), "github", binding.Key(), e2eIssue)
	if err != nil {
		t.Fatal(err)
	}
	proposal, _, err := store.Proposal(t.Context(), contract)
	if err != nil {
		t.Fatal(err)
	}
	if local, err := store.LocalVerified(t.Context(), contract, proposal, repaired); err != nil || !local {
		t.Fatalf("repaired head lacks genuine local proof: %v %v", local, err)
	}
}

func TestRecoveredIssueContinuationResumesUnpublishedRepairCheckpoint(t *testing.T) {
	f, args, checkpoint, transport := failedTransportAttempt(t)
	f.gitBare("tag", "lock/widget/v1", f.gitBare("rev-parse", "main"))
	if err := os.Remove(transport); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.stateDir, "checks-pending-once"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	recoverIssue(t, args, checkpoint, 1, "without launching a model", "verification_pending")
	store, err := issueentry.NewStore(f.repo, "origin")
	if err != nil {
		t.Fatal(err)
	}
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: f.cfg.Repository}
	contract, _, err := store.Load(t.Context(), "github", binding.Key(), e2eIssue)
	if err != nil {
		t.Fatal(err)
	}
	spent, err := store.ReserveRemoteRepair(t.Context(), contract)
	if err != nil || spent.Attempt != 1 || spent.Cap != 3 {
		t.Fatalf("first repair reservation: %+v %v", spent, err)
	}
	before := readE2ECalls(t, f.stateDir)
	launches := codexLaunchesWithArg(before, "--dangerously-bypass-approvals-and-sandbox")
	workdir := launches[0].Dir
	// This is the state a quiescent interrupted fixer leaves: its attempt is
	// spent and its edits are checkpointed, but the PR still names the old head.
	if err := os.WriteFile(filepath.Join(workdir, "preserved-ci-progress.txt"), []byte("checkpointed before interruption\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.gitAt(workdir, "add", "preserved-ci-progress.txt")
	f.gitAt(workdir, "commit", "-m", "fix: preserve interrupted CI repair")
	unpublished := f.gitAt(workdir, "rev-parse", "HEAD")
	pr, exists, err := readE2EPRState(f.cfg)
	if err != nil || !exists || f.gitBare("rev-parse", pr.Head) != checkpoint || unpublished == checkpoint {
		t.Fatalf("fixture did not retain an unpublished repair: %+v %v", pr, err)
	}
	if err := os.WriteFile(filepath.Join(f.stateDir, "checks-red"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := run(args, &out, &diagnostic); code != 0 || !strings.Contains(out.String(), "verified_pr #77") {
		t.Fatalf("resume checkpointed CI repair %d: %s %s", code, &out, &diagnostic)
	}
	calls := readE2ECalls(t, f.stateDir)
	implementation := codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")
	if len(implementation) != len(launches)+1 || implementation[len(launches)].Dir != workdir || !strings.Contains(implementation[len(launches)].Stdin, "fixture remote CI failure") {
		t.Fatalf("restart did not run one CI-only fixer in the retained checkout: %d launches", len(implementation))
	}
	if posts := githubCallsContaining(calls, "POST", "repos/"+f.cfg.Repository+"/pulls"); len(posts) != 1 {
		t.Fatalf("restart created another proposal: %d", len(posts))
	}
	repaired := f.gitBare("rev-parse", pr.Head)
	if f.gitAt(workdir, "merge-base", unpublished, repaired) != unpublished || f.gitBare("show", repaired+":preserved-ci-progress.txt") != "checkpointed before interruption" {
		t.Fatal("published repair discarded the interrupted checkpoint")
	}
	proposal, _, err := store.Proposal(t.Context(), contract)
	if err != nil {
		t.Fatal(err)
	}
	if local, err := store.LocalVerified(t.Context(), contract, proposal, repaired); err != nil || !local {
		t.Fatalf("resumed repair lacks exact local proof: %v %v", local, err)
	}
	remaining, err := store.ReserveRemoteRepair(t.Context(), contract)
	if err != nil || remaining.Attempt != 3 || remaining.Cap != spent.Cap {
		t.Fatalf("restart reset or overspent repair allowance: %+v %v", remaining, err)
	}
	if _, err := store.ReserveRemoteRepair(t.Context(), contract); !errors.Is(err, issueentry.ErrRemoteRepairExhausted) {
		t.Fatalf("exhausted repair allowance was reused: %v", err)
	}
}

// A later CI repair must not let the old head's completed review suppress
// the fresh review owed by the newly published, locally verified revision.
func TestRecoveredIssueCIRepairRequiresFreshReviewAfterPriorReviewedHead(t *testing.T) {
	f, args, checkpoint, transport := failedTransportAttempt(t)
	f.gitBare("tag", "lock/widget/v1", f.gitBare("rev-parse", "main"))
	if err := os.Remove(transport); err != nil {
		t.Fatal(err)
	}
	recoverIssue(t, args, checkpoint, 1, "without launching a model", "review_pending")
	var out, diagnostic bytes.Buffer
	if code := run(args, &out, &diagnostic); code != 0 {
		t.Fatalf("first review: %d %s %s", code, &out, &diagnostic)
	}
	before := readE2ECalls(t, f.stateDir)
	if len(codexLaunchesWithArg(before, "danger-full-access")) != 1 {
		t.Fatal("first head lacks its own review")
	}
	for _, marker := range []string{"checks-pending-once", "checks-red"} {
		if err := os.WriteFile(filepath.Join(f.stateDir, marker), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()
	diagnostic.Reset()
	if code := run(args, &out, &diagnostic); code != 0 || !strings.Contains(out.String(), "verified_pr #77") {
		t.Fatalf("repair after prior review: %d %s %s", code, &out, &diagnostic)
	}
	calls := readE2ECalls(t, f.stateDir)
	if implementations, reviews := len(codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")), len(codexLaunchesWithArg(calls, "danger-full-access")); implementations != 3 || reviews != 2 {
		t.Fatalf("expected original implementation/repair, one CI-only fixer and two revision-specific reviews; implementation=%d review=%d", implementations, reviews)
	}
	pr, ok, err := readE2EPRState(f.cfg)
	if err != nil || !ok {
		t.Fatalf("repaired proposal: %+v %v", pr, err)
	}
	repaired := f.gitBare("rev-parse", pr.Head)
	comment, ok, err := readE2ECommentState(f.cfg)
	if err != nil || !ok || repaired == checkpoint || !strings.Contains(comment.Body, repaired) {
		t.Fatalf("review was not refreshed for repaired head %s: %+v %v", repaired, comment, err)
	}
}

// A recovery interrupted between remote creation and local association is
// adopted by the existing reconciliation; no second proposal is created.
func TestDirectIssueRecoveryAdoptsAnInterruptedPublication(t *testing.T) {
	f, args, checkpoint, transport := failedTransportAttempt(t)
	f.gitBare("tag", "lock/widget/v1", f.gitBare("rev-parse", "main"))
	if err := os.Remove(transport); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(f.stateDir, "proposal-receipt-unavailable")
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	recoverIssue(t, args, checkpoint, 1, "phase=recovery_publication", "--recover-checkpoint "+checkpoint)
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	recoverIssue(t, args, checkpoint, 1, "already has proposal #77; checkpoint recovery published nothing", "result=review_pending phase=review checkpoint="+checkpoint)
	calls := readE2ECalls(t, f.stateDir)
	if posts := githubCallsContaining(calls, "POST", "repos/"+f.cfg.Repository+"/pulls"); len(posts) != 1 || len(codexLaunchesWithArg(calls, "exec")) != 2 {
		t.Fatalf("pull request creations=%d model launches=%d", len(posts), len(codexLaunchesWithArg(calls, "exec")))
	}
}

type recoveryLease struct {
	err                error
	acquired, released int
}

func (l *recoveryLease) Acquire(context.Context, port.Task) (port.Claim, error) {
	if l.err != nil {
		return nil, l.err
	}
	l.acquired++
	return recoveryClaim{l}, nil
}

type recoveryClaim struct{ lease *recoveryLease }

func (c recoveryClaim) Release(context.Context) error {
	c.lease.released++
	return nil
}

type recoverySpace struct {
	work   *recoveryWorkdir
	opened func()
}

func (s *recoverySpace) Existing(context.Context, port.Task, string) (port.Workdir, error) {
	if s.opened != nil {
		s.opened()
	}
	return s.work, nil
}

type recoveryWorkdir struct {
	dir, branch string
	nothing     bool
	pushes      int
}

func (w *recoveryWorkdir) Dir() string                              { return w.dir }
func (w *recoveryWorkdir) Branch() string                           { return w.branch }
func (w *recoveryWorkdir) Checkpoint(context.Context) (bool, error) { return true, nil }
func (w *recoveryWorkdir) Cleanup(context.Context) error            { return nil }
func (w *recoveryWorkdir) ChangeSize(context.Context) (changesize.Size, error) {
	return changesize.Size{}, nil
}
func (w *recoveryWorkdir) Push(context.Context) (bool, error) {
	w.pushes++
	return !w.nothing, nil
}

// Every stop before publication names its phase and leaves no push, proposal,
// association, or held claim behind.
func TestCheckpointRecoveryStopsBeforePublication(t *testing.T) {
	for _, tc := range []struct {
		name, phase string
		active      bool
		setup       func(*checkpointRecovery, *issueentry.Checkpoint, context.CancelFunc)
	}{
		{"claim held", "claim", true, func(rc *checkpointRecovery, _ *issueentry.Checkpoint, _ context.CancelFunc) {
			rc.lease.(*recoveryLease).err = fmt.Errorf("acquire: %w", port.ErrClaimLost)
		}},
		{"claim unavailable", "claim", false, func(rc *checkpointRecovery, _ *issueentry.Checkpoint, _ context.CancelFunc) {
			rc.lease.(*recoveryLease).err = errors.New("claim remote unavailable")
		}},
		{"foreign description", "description", false, func(_ *checkpointRecovery, c *issueentry.Checkpoint, _ context.CancelFunc) {
			c.ContractID = "sha256:" + strings.Repeat("f", 64)
		}},
		{"uncovered checkpoint", "description", false, func(_ *checkpointRecovery, c *issueentry.Checkpoint, _ context.CancelFunc) {
			c.Revision = strings.Repeat("b", 40)
		}},
		{"cancelled verification", "verification", false, func(rc *checkpointRecovery, _ *issueentry.Checkpoint, cancel context.CancelFunc) {
			rc.space.(*recoverySpace).opened = cancel
		}},
		{"nothing beyond mainline", "publication", false, func(rc *checkpointRecovery, _ *issueentry.Checkpoint, _ context.CancelFunc) {
			rc.space.(*recoverySpace).work.nothing = true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, repo, revision, proposal := issueRecoveryFixture(t, "git rev-parse HEAD")
			description, _ := prdescription.FromAgent(prdescriptiontest.Message)
			record := issueentry.Checkpoint{Schema: issueentry.CheckpointSchema, ContractID: r.contract.ID, TaskID: r.task.ID, Branch: proposal.Branch, Revision: revision, Description: description.Render()}
			lease := &recoveryLease{}
			work := &recoveryWorkdir{dir: repo, branch: proposal.Branch}
			rc := checkpointRecovery{host: &issueProofHost{}, space: &recoverySpace{work: work}, lease: lease, remote: "origin", revision: revision}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			tc.setup(&rc, &record, cancel)
			if err := issueentry.RetainCheckpoint(runtime.TaskStateRoot(repo), record); err != nil {
				t.Fatal(err)
			}
			_, stop := r.publishCheckpoint(ctx, rc)
			if stop == nil || stop.phase != tc.phase || stop.active != tc.active || stop.next == "" {
				t.Fatalf("stop %+v, want phase %s active %v", stop, tc.phase, tc.active)
			}
			if work.pushes != 0 && tc.phase != "publication" {
				t.Fatal("an unverified checkpoint was pushed")
			}
			if lease.released != lease.acquired {
				t.Fatalf("claims acquired %d released %d", lease.acquired, lease.released)
			}
			if _, associated, err := r.store.Proposal(t.Context(), r.contract); err != nil || associated {
				t.Fatalf("stopped recovery associated a proposal: %v %v", associated, err)
			}
		})
	}
}

func TestCheckpointRecoveryOperandsRefuseBeforeIO(t *testing.T) {
	for _, args := range [][]string{
		{"--recover-checkpoint", strings.Repeat("a", 40)},
		{"--inspect-issue", "17", "--recover-checkpoint", strings.Repeat("a", 40)},
		{"--issue", "17", "--issue-input", "some.json", "--recover-checkpoint", "HEAD"},
		{"--issue", "17", "--issue-input", "some.json", "--recover-checkpoint", strings.Repeat("A", 40)},
	} {
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != 2 || !strings.Contains(err.String(), "--recover-checkpoint requires") {
			t.Fatalf("args=%v exit=%d %s", args, code, err.String())
		}
	}
}

func TestIssueCheckpointRetentionKeepsOnlyTheSelectedCleanCheckpoint(t *testing.T) {
	r, repo, revision, proposal := issueRecoveryFixture(t, "git rev-parse HEAD")
	description, _ := prdescription.FromAgent(prdescriptiontest.Message)
	work := &recoveryWorkdir{dir: repo, branch: proposal.Branch}
	r.retainCheckpoint(t.Context(), port.Task{ID: "unselected"}, work, description)
	if _, ok, err := issueentry.LoadCheckpoint(runtime.TaskStateRoot(repo)); ok || err != nil || r.described != "" {
		t.Fatalf("unselected task retained: %v %v", ok, err)
	}
	dirty := filepath.Join(repo, "uncommitted.txt")
	if err := os.WriteFile(dirty, []byte("work\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.retainCheckpoint(t.Context(), r.task, work, description)
	if _, ok, err := issueentry.LoadCheckpoint(runtime.TaskStateRoot(repo)); ok || err != nil || r.described != "" {
		t.Fatalf("uncommitted work retained as a checkpoint: %v %v", ok, err)
	}
	if err := os.Remove(dirty); err != nil {
		t.Fatal(err)
	}
	r.retainCheckpoint(t.Context(), r.task, work, description)
	retained, ok, err := issueentry.LoadCheckpoint(runtime.TaskStateRoot(repo))
	if err != nil || !ok || retained.Revision != revision || retained.ContractID != r.contract.ID || retained.Branch != proposal.Branch || r.described != revision {
		t.Fatalf("selected checkpoint: %+v %v %v", retained, ok, err)
	}
}

func TestRequiredContractRefsAreFetchedNeverFabricated(t *testing.T) {
	repo := implementReviewGitFixture(t)
	remote := strings.TrimSpace(runRoutingGit(t, repo, "remote", "get-url", "origin"))
	if err := requireContractRefs(t.Context(), repo, "origin"); err != nil {
		t.Fatalf("a checkout without plans requires no ref: %v", err)
	}
	plan := filepath.Join(repo, "docs", "plans", "widget", "001-plan.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	header := "| Field | Value |\n|---|---|\n| **Feature** | widget |\n| **PRD** | prd/widget |\n| **Design** | design/widget |\n| **Basis** | lock/widget/v1 |\n"
	if err := os.WriteFile(plan, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireContractRefs(t.Context(), repo, "origin"); err == nil || !strings.Contains(err.Error(), "refs/tags/lock/widget/v1 is missing") {
		t.Fatalf("missing lock accepted: %v", err)
	}
	// A canonical name that does not denote a commit is still refused.
	blob := strings.TrimSpace(runRoutingGit(t, repo, "hash-object", "-w", "README"))
	runRoutingGit(t, repo, "push", "-q", "origin", blob+":refs/tags/lock/widget/v1")
	if err := requireContractRefs(t.Context(), repo, "origin"); err == nil || !strings.Contains(err.Error(), "does not name a commit") {
		t.Fatalf("non-commit lock accepted: %v", err)
	}
	runRoutingGit(t, repo, "tag", "-d", "lock/widget/v1")
	runRoutingGit(t, remote, "tag", "-f", "lock/widget/v1", "main")
	if err := requireContractRefs(t.Context(), repo, "origin"); err != nil {
		t.Fatalf("canonical lock not fetched: %v", err)
	}
	if err := requireContractRefs(t.Context(), repo, "origin"); err != nil {
		t.Fatalf("present lock refetched or refused: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("unknown-docgate-key: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireContractRefs(t.Context(), repo, "origin"); err == nil || !strings.Contains(err.Error(), "read plan configuration") {
		t.Fatalf("invalid plan configuration accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(repo, ".pyxis", "config.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(plan), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(plan), 0o755) }) // let TempDir cleanup remove it
	if err := requireContractRefs(t.Context(), repo, "origin"); err == nil && os.Getuid() != 0 {
		t.Fatal("unreadable plans accepted")
	}
}
