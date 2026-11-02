package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec/taskspectest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

const (
	e2eRepository = "owner/repository"
	e2eLabel      = "automated-implementation"
	e2eIssue      = 42
	e2ePR         = 77
	e2eHead       = "feat/t1-add-e2e-fixture-42"
)

// TestEndToEnd_ImplementNowDeliversOnce exercises the real command composition
// root over production dispatcher, worker, Git workspace, claim, code-host and
// tracker adapters. Only the two true external systems are substituted: the
// agent CLI and GitHub CLI are strict helper processes, while the Git remote is
// a real local bare repository.
func TestEndToEnd_ImplementNowDeliversOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper-process E2E currently targets POSIX Git environments")
	}
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement})
	if err := os.MkdirAll(filepath.Join(fixture.repo, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.repo, ".pyxis", "config.yaml"), []byte(harnessconfigtest.YAML+"executor:\n  logging:\n    directory: logs[1]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exit := run(fixture.args(), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("first run exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", exit, stdout.String(), stderr.String())
	}
	for _, want := range []string{"succeeded: 1", "failed:    0", "blocked:   0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("first summary missing %q:\n%s", want, stdout.String())
		}
	}

	calls := readE2ECalls(t, fixture.stateDir)
	launches := callsFor(calls, "codex", "exec")
	if len(launches) != 1 {
		t.Fatalf("codex implementation launches = %d, want exactly 1\nall calls: %+v", len(launches), calls)
	}
	launch := launches[0]
	assertImplementationLaunchDir(t, fixture.root, fixture.repo, launch.Dir)
	for _, want := range []string{"exec", "--json", "-m", "gpt-5.6", `model_reasoning_effort="medium"`, "--dangerously-bypass-approvals-and-sandbox", "-"} {
		if !containsArg(launch.Args, want) {
			t.Errorf("codex launch args missing %q: %q", want, launch.Args)
		}
	}
	for _, forbidden := range []string{"resume", "--last", "--ephemeral"} {
		if containsArg(launch.Args, forbidden) {
			t.Errorf("implementation launch reused or made a session ephemeral via %q: %q", forbidden, launch.Args)
		}
	}
	for _, want := range []string{fixture.cfg.TaskBody, e2eHead, "Go supervisor owns all Git mutations", "Do NOT stage, commit"} {
		if !strings.Contains(launch.Stdin, want) {
			t.Errorf("assembled brief missing %q:\n%s", want, launch.Stdin)
		}
	}

	if got := fixture.gitBare("show", e2eHead+":e2e-output.txt"); got != "implemented by the stub harness" {
		t.Errorf("published artifact = %q, want stub output", got)
	}
	if _, err := os.Stat(filepath.Join(fixture.repo, "e2e-output.txt")); !os.IsNotExist(err) {
		t.Errorf("implementation artifact escaped the isolated worktree into fixture repo: err=%v", err)
	}
	mainSHA := fixture.gitBare("rev-parse", "main")
	headSHA := fixture.gitBare("rev-parse", e2eHead)
	if mainSHA == headSHA {
		t.Fatal("implementation branch did not advance beyond main")
	}
	if fixture.gitBareTry("show-ref", "--verify", "refs/claim/42") == nil {
		t.Fatal("claim ref survived the task terminal")
	}

	creates := githubCalls(calls, "POST", "repos/"+e2eRepository+"/pulls")
	if len(creates) != 1 {
		t.Fatalf("draft PR creates = %d, want 1", len(creates))
	}
	create := creates[0]
	for key, want := range map[string]string{
		"head": e2eHead,
		"base": "main",
	} {
		if got := formValue(create.Args, key); got != want {
			t.Errorf("created PR %s = %q, want %q", key, got, want)
		}
	}
	if got := formValue(create.Args, "draft"); got != "true" {
		t.Errorf("created PR draft = %q, want true", got)
	}
	if got := formValue(create.Args, "title"); got != "chore: Add e2e fixture" {
		t.Errorf("created PR title = %q, want conventional title without identity prefix", got)
	}
	if body := formValue(create.Args, "body"); !strings.Contains(body, "Closes #42") || !strings.Contains(body, "never merges") {
		t.Errorf("created PR body lacks task provenance or never-merge boundary:\n%s", body)
	}
	checkPath := fmt.Sprintf("repos/%s/commits/%s/check-runs?filter=latest&per_page=100", e2eRepository, headSHA)
	if got := len(githubCalls(calls, "GET", checkPath)); got != 1 {
		t.Errorf("exact-head check reads = %d, want 1 for %s", got, headSHA)
	}
	comments := githubCalls(calls, "POST", fmt.Sprintf("repos/%s/issues/%d/comments", e2eRepository, e2eIssue))
	if len(comments) != 1 || !strings.Contains(formValue(comments[0].Args, "body"), "Opened proposal #77") {
		t.Errorf("progress comments = %+v, want one proposal link", comments)
	}

	assertJournalLifecycle(t, fixture.journal)
	logDir := filepath.Join(fixture.repo, "logs[1]")
	entries, err := os.ReadDir(logDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("execution logs = %v, %v; want one feature log", entries, err)
	}
	logPath := filepath.Join(logDir, entries[0].Name())
	if !strings.Contains(stdout.String(), "implement-now: execution log "+logPath) {
		t.Fatalf("execution log path missing from stdout: %s", stdout.String())
	}
	firstLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"implementation started with codex", "local CI is green after 0 recovery", "proposal remote CI is green", "finished succeeded"} {
		if !strings.Contains(string(firstLog), want) {
			t.Errorf("execution log missing %q:\n%s", want, firstLog)
		}
	}

	// The tracker still reports the issue open, exactly like a re-invocation
	// before the proposal is merged. The pre-claim proposal probe must withhold
	// it without another model launch or duplicate PR.
	stdout.Reset()
	stderr.Reset()
	secondExit := run(fixture.args(), &stdout, &stderr)
	if secondExit != 0 {
		t.Fatalf("second run exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", secondExit, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "awaiting-proposal: 1") || !strings.Contains(stdout.String(), "AWAITING-PROPOSAL T1 (#42)") {
		t.Errorf("second run did not report healthy duplicate withholding:\n%s", stdout.String())
	}
	after := readE2ECalls(t, fixture.stateDir)
	if got := len(callsFor(after, "codex", "exec")); got != 1 {
		t.Errorf("codex launches after reinvocation = %d, want still 1", got)
	}
	if got := len(githubCalls(after, "POST", "repos/"+e2eRepository+"/pulls")); got != 1 {
		t.Errorf("draft PR creates after reinvocation = %d, want still 1", got)
	}
	if got := fixture.gitBare("rev-parse", e2eHead); got != headSHA {
		t.Errorf("implementation head moved on reinvocation: %s -> %s", headSHA, got)
	}
	entries, err = os.ReadDir(logDir)
	if err != nil || len(entries) != 1 || filepath.Join(logDir, entries[0].Name()) != logPath {
		t.Fatalf("reinvocation did not reuse %q: %v, %v", logPath, entries, err)
	}
	secondLog, err := os.ReadFile(logPath)
	if err != nil || !bytes.HasPrefix(secondLog, firstLog) || len(secondLog) <= len(firstLog) {
		t.Fatalf("reinvocation did not append to the original log: %q, %v", secondLog, err)
	}
	if got := strings.Count(string(secondLog), "implementation started with codex"); got != 1 {
		t.Errorf("execution log reports %d implementations, want one", got)
	}
}

// TestEndToEnd_MachineLocalConfigOverridesTrackedConfig proves precedence at
// the command boundary, not only in the config packages. The tracked config
// selects Claude, for which this fixture intentionally installs no helper; the
// ignored local layer selects Codex. A missing or incorrectly ordered overlay
// therefore fails before a worker can complete.
func TestEndToEnd_MachineLocalConfigOverridesTrackedConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper-process E2E currently targets POSIX Git environments")
	}
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement})
	configDir := filepath.Join(fixture.repo, ".pyxis")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.repo, ".gitignore"), []byte(".pyxis/config.local.yaml\n.pyxis/logs/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("executor:\n  routing:\n    default: claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture.gitAt(fixture.repo, "add", ".gitignore", ".pyxis/config.yaml")
	fixture.gitAt(fixture.repo, "commit", "-m", "chore: configure tracked backend")
	fixture.gitAt(fixture.repo, "push", "origin", "main")
	if err := os.WriteFile(filepath.Join(configDir, "config.local.yaml"), []byte("executor:\n  routing:\n    default: codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exit := run(fixture.args(), &stdout, &stderr); exit != 0 {
		t.Fatalf("run exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", exit, stdout.String(), stderr.String())
	}
	launches := callsFor(readE2ECalls(t, fixture.stateDir), "codex", "exec")
	if len(launches) != 1 {
		t.Fatalf("codex launches = %d, want 1 from the local override", len(launches))
	}
	if got := fixture.gitAt(fixture.repo, "status", "--short"); got != "" {
		t.Fatalf("machine-local config was not ignored; git status:\n%s", got)
	}
}

// A clean agent-process exit is telemetry, not evidence of delivery. This
// scenario makes the fake harness exit zero after a valid completed-turn event
// while deliberately producing no commit. The production pipeline must report
// the task as hollow/failed and must not publish a branch or open a PR.
func TestEndToEnd_ImplementNowRejectsExitOnlySuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper-process E2E currently targets POSIX Git environments")
	}
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessHollow})

	var stdout, stderr bytes.Buffer
	exit := run(fixture.args(), &stdout, &stderr)
	if exit != 1 {
		t.Fatalf("hollow run exit = %d, want 1\nstdout:\n%s\nstderr:\n%s", exit, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "succeeded: 0") || !strings.Contains(stdout.String(), "failed:    1") {
		t.Errorf("hollow summary did not fail the task:\n%s", stdout.String())
	}
	calls := readE2ECalls(t, fixture.stateDir)
	if got := len(callsFor(calls, "codex", "exec")); got != 1 {
		t.Errorf("hollow harness launches = %d, want 1", got)
	}
	if got := len(githubCalls(calls, "POST", "repos/"+e2eRepository+"/pulls")); got != 0 {
		t.Errorf("hollow run created %d PRs, want 0", got)
	}
	if err := fixture.gitBareTry("show-ref", "--verify", "refs/heads/"+e2eHead); err == nil {
		t.Fatal("hollow run published an implementation branch")
	}
	if err := fixture.gitBareTry("show-ref", "--verify", "refs/claim/42"); err == nil {
		t.Fatal("claim ref survived the hollow terminal")
	}
	events := readJournalEvents(t, fixture.journal)
	if !hasJournalEvent(events, "task_verified", "hollow", "") {
		t.Errorf("journal lacks task_verified hollow event: %+v", events)
	}
	if !hasJournalEvent(events, "task_outcome", "", "failed") {
		t.Errorf("journal lacks failed task_outcome: %+v", events)
	}
}

func TestRunTrackerSelectionBoundaries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper-process E2E currently targets POSIX Git environments")
	}
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement})

	for _, tc := range []struct {
		name  string
		extra []string
		want  string
	}{
		{name: "unknown tracker", extra: []string{"--tracker", "unknown"}, want: "unknown --tracker"},
		{name: "carina", extra: []string{"--tracker", "carina", "--carina-bin", filepath.Join(fixture.root, "missing-carina")}, want: "sweep"},
		{name: "plan-scoped carina", extra: []string{"--tracker", "carina", "--plan-ref", "feature/001", "--carina-bin", filepath.Join(fixture.root, "missing-carina")}, want: "sweep"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(append(fixture.args(), tc.extra...), &stdout, &stderr)
			if code != 2 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("run = %d stdout=%q stderr=%q, want operational failure containing %q", code, stdout.String(), stderr.String(), tc.want)
			}
		})
	}
}

// With executor.review configured, the default composition runs the
// local-first loop: the reviewer is launched over the exact LOCAL revision —
// before any push, in an independent read-only clone with no pull request to
// look at — and only the accepted revision is published, once. The review
// artifact then lands on the opened proposal, the accepted review covers the
// published head, and no post-green obligation review runs.
func TestEndToEnd_ImplementNowReviewsBeforePublish(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper-process E2E currently targets POSIX Git environments")
	}
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement, Review: true})

	var stdout, stderr bytes.Buffer
	exit := run(fixture.args(), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("review run exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", exit, stdout.String(), stderr.String())
	}
	for _, want := range []string{"succeeded: 1", "failed:    0", "pre-publish-reviewed: 1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("summary missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "review-completed:") || strings.Contains(stdout.String(), "review-failed:") {
		t.Errorf("summary reports a post-green review for a head the pre-publish review covers:\n%s", stdout.String())
	}

	calls := readE2ECalls(t, fixture.stateDir)
	implementationLaunches := codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")
	reviewLaunches := codexLaunchesWithArg(calls, "danger-full-access")
	if len(implementationLaunches) != 1 || len(reviewLaunches) != 1 {
		t.Fatalf("codex launches = implementation %d review %d, want one each\nall calls: %+v", len(implementationLaunches), len(reviewLaunches), calls)
	}
	assertImplementationLaunchDir(t, fixture.root, fixture.repo, implementationLaunches[0].Dir)
	reviewLaunch := reviewLaunches[0]
	for _, want := range []string{"-a", "never", "--no-daemon", "--sandbox", "danger-full-access", "--ignore-user-config"} {
		if !containsArg(reviewLaunch.Args, want) {
			t.Errorf("review launch args missing %q: %q", want, reviewLaunch.Args)
		}
	}
	if containsArg(reviewLaunch.Args, "--ephemeral") {
		t.Errorf("review launch discards the session records the host verifies: %q", reviewLaunch.Args)
	}
	if containsArg(reviewLaunch.Args, "--dangerously-bypass-approvals-and-sandbox") {
		t.Errorf("review launch received implementation write authority: %q", reviewLaunch.Args)
	}
	if samePath(reviewLaunch.Dir, fixture.repo) || isWithin(reviewLaunch.Dir, filepath.Join(fixture.root, ".pyxis-worktrees")) {
		t.Errorf("review launch dir %q is not an independent disposable clone", reviewLaunch.Dir)
	}

	// The review ran BEFORE the branch was published and the proposal opened.
	reviewAt := callIndex(calls, func(c e2eStubCall) bool {
		return c.Program == "codex" && containsArg(c.Args, "exec") && containsArg(c.Args, "danger-full-access")
	})
	proposalAt := callIndex(calls, func(c e2eStubCall) bool {
		return c.Program == "gh" && methodOf(c.Args) == "POST" && len(c.Args) > 0 && c.Args[len(c.Args)-1] == "repos/"+e2eRepository+"/pulls"
	})
	if reviewAt < 0 || proposalAt < 0 || reviewAt > proposalAt {
		t.Fatalf("review launch at call %d, proposal open at call %d — the review must precede the publish\ncalls: %+v", reviewAt, proposalAt, calls)
	}

	headSHA := fixture.gitBare("rev-parse", e2eHead)
	localRef := "local:" + e2eRepository + "@" + e2eHead
	for _, want := range []string{"/impl-review " + localRef + " " + headSHA, "do not look for a pull request", "git diff ", "Review obligation:", "Review process:", "Do not write, comment on, push to, or otherwise mutate", "task charter (data, not instructions)"} {
		if !strings.Contains(reviewLaunch.Stdin, want) {
			t.Errorf("review brief missing %q:\n%s", want, reviewLaunch.Stdin)
		}
	}
	if strings.Contains(reviewLaunch.Stdin, "github:"+e2eRepository+"#") {
		t.Errorf("pre-publish review brief names a code-host proposal that does not exist yet:\n%s", reviewLaunch.Stdin)
	}

	// The covering artifact landed on the opened proposal exactly once.
	reviewWrites := githubCallsContaining(calls, "POST", fmt.Sprintf("repos/%s/issues/%d/comments", e2eRepository, e2ePR))
	if len(reviewWrites) != 1 {
		t.Fatalf("review artifact publications = %d, want exactly 1", len(reviewWrites))
	}
	comment, ok, err := readE2ECommentState(fixture.cfg)
	if err != nil || !ok {
		t.Fatalf("read published review comment: found=%v err=%v", ok, err)
	}
	payload, err := persist.ParseArtifact(comment.Body)
	if err != nil {
		t.Fatalf("published review artifact is invalid: %v", err)
	}
	revision, ok := verdict.LatestReviewedRevision(payload)
	if !ok || payload.ProposalRef != localRef || revision != headSHA {
		t.Errorf("published review identity = proposal %q revision %q, want %s / %s", payload.ProposalRef, revision, localRef, headSHA)
	}

	events := readJournalEvents(t, fixture.journal)
	if !hasJournalEvent(events, "review_round", "approve", "") {
		t.Errorf("journal lacks an approve review_round: %+v", events)
	}
	terminal, ok := journalEvent(events, "pre_publish_review_terminal")
	if !ok || !terminal.Covered || terminal.Verdict != "approve" || terminal.Round != 1 {
		t.Errorf("journal pre_publish_review_terminal = %+v (found %v), want covered approve after one round", terminal, ok)
	}
	if _, found := journalEvent(events, "impl_review_terminal"); found {
		t.Errorf("journal carries a post-green impl_review_terminal for a covered head: %+v", events)
	}
	if !hasJournalEvent(events, "task_verified", "green", "") || !hasJournalEvent(events, "task_outcome", "", "succeeded") {
		t.Errorf("journal lacks the green implementation terminals: %+v", events)
	}
}

// With `pre_publish: false`, configuring executor.review drives the post-green
// review half of the production composition root exactly as before the
// local-first loop: the exact-revision read-only reviewer, durable obligation
// and process stores, host-owned GitHub publication, exact receipt read-back,
// paired terminals, and run-level review accounting all stay real. The helper
// processes substitute only Codex and GitHub at their command boundaries.
func TestEndToEnd_ImplementNowPublishesConfiguredReviewPostGreen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper-process E2E currently targets POSIX Git environments")
	}
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement, Review: true, ReviewPostGreenOnly: true})

	var stdout, stderr bytes.Buffer
	exit := run(fixture.args(), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("review run exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", exit, stdout.String(), stderr.String())
	}
	for _, want := range []string{"succeeded: 1", "failed:    0", "review-completed: 1", "review-failed:    0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("review summary missing %q:\n%s", want, stdout.String())
		}
	}

	calls := readE2ECalls(t, fixture.stateDir)
	implementationLaunches := codexLaunchesWithArg(calls, "--dangerously-bypass-approvals-and-sandbox")
	reviewLaunches := codexLaunchesWithArg(calls, "danger-full-access")
	if len(implementationLaunches) != 1 || len(reviewLaunches) != 1 {
		t.Fatalf("codex launches = implementation %d review %d, want one each\nall calls: %+v", len(implementationLaunches), len(reviewLaunches), calls)
	}
	assertImplementationLaunchDir(t, fixture.root, fixture.repo, implementationLaunches[0].Dir)
	reviewLaunch := reviewLaunches[0]
	for _, want := range []string{"-a", "never", "--no-daemon", "--sandbox", "danger-full-access", "--ignore-user-config"} {
		if !containsArg(reviewLaunch.Args, want) {
			t.Errorf("review launch args missing %q: %q", want, reviewLaunch.Args)
		}
	}
	if containsArg(reviewLaunch.Args, "--ephemeral") {
		t.Errorf("review launch discards the session records the host verifies: %q", reviewLaunch.Args)
	}
	if containsArg(reviewLaunch.Args, "--dangerously-bypass-approvals-and-sandbox") {
		t.Errorf("review launch received implementation write authority: %q", reviewLaunch.Args)
	}
	if samePath(reviewLaunch.Dir, fixture.repo) || isWithin(reviewLaunch.Dir, filepath.Join(fixture.root, ".pyxis-worktrees")) {
		t.Errorf("review launch dir %q is not an independent disposable clone", reviewLaunch.Dir)
	}
	headSHA := fixture.gitBare("rev-parse", e2eHead)
	for _, want := range []string{"/impl-review github:" + e2eRepository + "#77 " + headSHA, "Review obligation:", "Review process:", "Do not write, comment on, push to, or otherwise mutate"} {
		if !strings.Contains(reviewLaunch.Stdin, want) {
			t.Errorf("review brief missing %q:\n%s", want, reviewLaunch.Stdin)
		}
	}

	reviewWrites := githubCallsContaining(calls, "POST", fmt.Sprintf("repos/%s/issues/%d/comments", e2eRepository, e2ePR))
	if len(reviewWrites) != 1 {
		t.Fatalf("review artifact publications = %d, want exactly 1", len(reviewWrites))
	}
	comment, ok, err := readE2ECommentState(fixture.cfg)
	if err != nil || !ok {
		t.Fatalf("read published review comment: found=%v err=%v", ok, err)
	}
	payload, err := persist.ParseArtifact(comment.Body)
	if err != nil {
		t.Fatalf("published review artifact is invalid: %v", err)
	}
	revision, ok := verdict.LatestReviewedRevision(payload)
	if !ok || payload.ProposalRef != "github:"+e2eRepository+"#77" || revision != headSHA {
		t.Errorf("published review identity = proposal %q revision %q, want github:%s#77 / %s", payload.ProposalRef, revision, e2eRepository, headSHA)
	}
	reviewReads := githubCallsContaining(calls, "GET", fmt.Sprintf("repos/%s/issues/comments/%d", e2eRepository, e2eReviewComment))
	if len(reviewReads) == 0 {
		t.Fatal("review publication was not authenticated by exact native-receipt read-back")
	}

	events := readJournalEvents(t, fixture.journal)
	if !hasJournalStatus(events, "review_artifact_publication_terminal", "verified") {
		t.Errorf("journal lacks verified review artifact publication terminal: %+v", events)
	}
	if !hasJournalStatus(events, "impl_review_terminal", "verified") {
		t.Errorf("journal lacks verified implement-now review terminal: %+v", events)
	}
}

type implementNowE2EFixture struct {
	t        *testing.T
	root     string
	repo     string
	origin   string
	stateDir string
	journal  string
	config   string
	cfg      e2eStubConfig
}

type e2eFixtureOptions struct {
	HarnessMode e2eHarnessMode
	Review      bool
	// ReviewPostGreenOnly writes `pre_publish: false` so the configured review
	// runs only after remote green (the pre-loop composition), instead of the
	// default local-first loop.
	ReviewPostGreenOnly       bool
	ReviewAttemptsSourceWrite bool
}

func newImplementNowE2EFixture(t *testing.T, opts e2eFixtureOptions) *implementNowE2EFixture {
	t.Helper()
	if opts.HarnessMode == "" {
		opts.HarnessMode = e2eHarnessImplement
	}
	root := t.TempDir()
	f := &implementNowE2EFixture{
		t: t, root: root,
		repo:     filepath.Join(root, "repo"),
		origin:   filepath.Join(root, "origin.git"),
		stateDir: filepath.Join(root, "stub-state"),
		journal:  filepath.Join(root, "repo", ".pyxis/logs/executor/events.jsonl"),
		config:   filepath.Join(root, "stub-config.json"),
	}
	f.cfg = e2eStubConfig{
		StateDir:                  f.stateDir,
		Origin:                    f.origin,
		Repository:                e2eRepository,
		Label:                     e2eLabel,
		IssueNumber:               e2eIssue,
		PRNumber:                  e2ePR,
		TaskTitle:                 "[T1] Add e2e fixture",
		TaskBody:                  "Create e2e-output.txt containing the requested implementation output.",
		Capsule:                   taskspectest.Marker(t, "T1"),
		HarnessMode:               opts.HarnessMode,
		ReviewAttemptsSourceWrite: opts.ReviewAttemptsSourceWrite,
	}
	if err := os.MkdirAll(f.stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f.cfg.CallEndpoint = startE2ECallRecorder(t, f.cfg)
	writeJSONFile(t, f.config, f.cfg)

	home := filepath.Join(root, "home")
	for _, dir := range []string{home, filepath.Join(root, "state"), filepath.Join(root, "gh-config"), filepath.Join(root, "codex-home")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	globalGitConfig := filepath.Join(root, "gitconfig")
	// Local transports may run receive-pack's automatic maintenance after a
	// push. Keep the hermetic origin quiescent so TempDir cleanup never races a
	// detached git maintenance process under the race runtime.
	const testGitConfig = "[gc]\n\tauto = 0\n\tautoDetach = false\n[maintenance]\n\tauto = false\n"
	if err := os.WriteFile(globalGitConfig, []byte(testGitConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(e2eHelperModeEnv, "1")
	t.Setenv(e2eStubConfigEnv, f.config)
	t.Setenv("PATH", e2eHelperBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// The race runtime sleeps for one second when every helper subprocess exits.
	// These short-lived stubs do not need that flush grace period; disabling it
	// removes pure test-harness latency without changing the parent race run.
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("GH_CONFIG_DIR", filepath.Join(root, "gh-config"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex-home"))
	t.Setenv("OPENAI_API_KEY", "e2e-model-auth-only")
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		unsetEnvForTest(t, key)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", globalGitConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", globalGitConfig)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		unsetEnvForTest(t, key)
	}
	seedGitHubBudgetCutover(t)

	f.gitAt(root, "init", "--bare", "--initial-branch=main", f.origin)
	f.gitAt(root, "init", "--initial-branch=main", f.repo)
	// Production Git calls sanitize global configuration from the environment.
	// Persist this fixture's maintenance policy in both repositories so local
	// receive-pack cannot detach work that outlives the test.
	for _, repo := range []string{f.origin, f.repo} {
		f.gitAt(repo, "config", "gc.auto", "0")
		f.gitAt(repo, "config", "maintenance.auto", "false")
	}
	f.gitAt(f.repo, "config", "user.name", "Pyxis E2E")
	f.gitAt(f.repo, "config", "user.email", "pyxis-e2e@example.invalid")
	if err := os.WriteFile(filepath.Join(f.repo, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.repo, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(f.repo, ".pyxis", "e2e-stub-config.json"), f.cfg)
	if err := os.WriteFile(filepath.Join(f.repo, ".gitignore"), []byte("/.pyxis/logs/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedPaths := []string{"README.md", ".gitignore", ".pyxis/e2e-stub-config.json"}
	if opts.Review {
		writeReviewFixtureConfig(t, f.repo, f.cfg, opts.ReviewPostGreenOnly)
		seedPaths = append(seedPaths, ".pyxis/config.yaml", ".pyxis/skills/impl-review/calibration.md")
	}
	f.gitAt(f.repo, append([]string{"add"}, seedPaths...)...)
	f.gitAt(f.repo, "commit", "-m", "chore: seed fixture")
	f.gitAt(f.repo, "remote", "add", "origin", f.origin)
	f.gitAt(f.repo, "push", "-u", "origin", "main")
	return f
}

// The production content budget deliberately waits a full GitHub window on a
// machine's first-ever writer. This fixture represents an already-cut-over
// machine so the test can exercise writes immediately; the budget ledgers stay
// real and isolated under the temporary XDG state root.
func seedGitHubBudgetCutover(t *testing.T) {
	t.Helper()
	path, err := budget.MarkerPath("github.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":1,"host":"github.com"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeReviewFixtureConfig(t *testing.T, repo string, cfg e2eStubConfig, postGreenOnly bool) {
	t.Helper()
	calibrationDir := filepath.Join(repo, ".pyxis", "skills", "impl-review")
	if err := os.MkdirAll(calibrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	executorConfig := "executor:\n  review:\n    backend: codex\n    budget_minutes: 1\n"
	if postGreenOnly {
		executorConfig += "    pre_publish: false\n"
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte(executorConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(repo, ".pyxis", "e2e-stub-config.json"), cfg)
	const calibration = `# impl-review-calibration/v1 — e2e fixture

| Row | Lens | Default severity | Escalation condition |
|---|---|---|---|
| CAL-E2E | correctness-critic | blocking | A correctness defect reaches the artifact. |
`
	if err := os.WriteFile(filepath.Join(calibrationDir, "calibration.md"), []byte(calibration), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *implementNowE2EFixture) args() []string {
	return []string{
		"--repo", f.repo,
		"--repo-slug", e2eRepository,
		"--remote", "origin",
		"--base", "main",
		"--label", e2eLabel,
		"--ambient-backend", "codex",
		"--worker-budget", "30s",
		"--unattended",
		"--journal", f.journal,
	}
}

func (f *implementNowE2EFixture) gitAt(dir string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git -C %s %s: %s: %v", dir, strings.Join(args, " "), out, err)
	}
	return strings.TrimSpace(string(out))
}

func (f *implementNowE2EFixture) gitBare(args ...string) string {
	f.t.Helper()
	return f.gitAt(f.root, append([]string{"--git-dir", f.origin}, args...)...)
}

func (f *implementNowE2EFixture) gitBareTry(args ...string) error {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"--git-dir", f.origin}, args...)...)
	cmd.Dir = f.root
	return cmd.Run()
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func callsFor(calls []e2eStubCall, program, exactArg string) []e2eStubCall {
	var out []e2eStubCall
	for _, call := range calls {
		if call.Program == program && containsArg(call.Args, exactArg) {
			out = append(out, call)
		}
	}
	return out
}

func codexLaunchesWithArg(calls []e2eStubCall, exactArg string) []e2eStubCall {
	var out []e2eStubCall
	for _, call := range calls {
		if call.Program == "codex" && containsArg(call.Args, "exec") && containsArg(call.Args, exactArg) {
			out = append(out, call)
		}
	}
	return out
}

func githubCalls(calls []e2eStubCall, method, path string) []e2eStubCall {
	var out []e2eStubCall
	for _, call := range calls {
		if call.Program == "gh" && len(call.Args) > 0 && call.Args[len(call.Args)-1] == path && methodOf(call.Args) == method {
			out = append(out, call)
		}
	}
	return out
}

func githubCallsContaining(calls []e2eStubCall, method, path string) []e2eStubCall {
	var out []e2eStubCall
	for _, call := range calls {
		if call.Program == "gh" && methodOf(call.Args) == method && containsArg(call.Args, path) {
			out = append(out, call)
		}
	}
	return out
}

func assertImplementationLaunchDir(t *testing.T, workspaceParent, repo, launchDir string) {
	t.Helper()
	worktrees := filepath.Join(workspaceParent, ".pyxis-worktrees")
	if samePath(launchDir, repo) || !isWithin(launchDir, worktrees) || samePath(launchDir, worktrees) {
		t.Errorf("implementation launch dir = %q, want a child of %q distinct from the fixture repo", launchDir, worktrees)
	}
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func isWithin(path, root string) bool {
	pathAbs, pathErr := filepath.Abs(path)
	rootAbs, rootErr := filepath.Abs(root)
	if pathErr != nil || rootErr != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func assertJournalLifecycle(t *testing.T, path string) {
	t.Helper()
	events := readJournalEvents(t, path)
	var kinds []string
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	want := []string{"run_started", "task_dispatched", "task_verified", "task_outcome", "run_finished"}
	position := 0
	counts := make(map[string]int, len(want))
	for _, kind := range kinds {
		counts[kind]++
		if position < len(want) && kind == want[position] {
			position++
		}
	}
	if position != len(want) {
		t.Errorf("journal kinds = %v, want ordered lifecycle %v", kinds, want)
	}
	for _, kind := range want {
		if counts[kind] != 1 {
			t.Errorf("journal kind %s count = %d, want exactly 1", kind, counts[kind])
		}
	}
}

type e2eJournalEvent struct {
	Kind    string `json:"kind"`
	Verdict string `json:"verdict"`
	Class   string `json:"class"`
	Status  string `json:"status"`
	Covered bool   `json:"covered"`
	Round   int    `json:"round"`
}

// journalEvent returns the first event of kind, if any.
func journalEvent(events []e2eJournalEvent, kind string) (e2eJournalEvent, bool) {
	for _, event := range events {
		if event.Kind == kind {
			return event, true
		}
	}
	return e2eJournalEvent{}, false
}

// callIndex returns the position of the first recorded stub call matching pred,
// or -1 — the harness records calls in the order the production code made them.
func callIndex(calls []e2eStubCall, pred func(e2eStubCall) bool) int {
	for i, call := range calls {
		if pred(call) {
			return i
		}
	}
	return -1
}

func readJournalEvents(t *testing.T, path string) []e2eJournalEvent {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close journal: %v", err)
		}
	}()
	var events []e2eJournalEvent
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), e2eMaxJSONLLine)
	for s.Scan() {
		var event e2eJournalEvent
		if err := json.Unmarshal(s.Bytes(), &event); err != nil {
			t.Fatalf("decode journal line: %v\n%s", err, s.Text())
		}
		events = append(events, event)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func hasJournalEvent(events []e2eJournalEvent, kind, verdict, class string) bool {
	for _, event := range events {
		if event.Kind == kind && event.Verdict == verdict && event.Class == class {
			return true
		}
	}
	return false
}

func hasJournalStatus(events []e2eJournalEvent, kind, status string) bool {
	for _, event := range events {
		if event.Kind == kind && event.Status == status {
			return true
		}
	}
	return false
}
