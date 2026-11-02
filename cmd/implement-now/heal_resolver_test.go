package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
)

type healWorkdir struct {
	dir    string
	branch string
}

func (w *healWorkdir) Dir() string                            { return w.dir }
func (w *healWorkdir) Branch() string                         { return w.branch }
func (*healWorkdir) Checkpoint(context.Context) (bool, error) { return false, nil }
func (*healWorkdir) Push(context.Context) (bool, error)       { return false, nil }
func (*healWorkdir) Cleanup(context.Context) error            { return nil }

func TestGitPusherPublishesConfiguredBranchAndReturnsExactHead(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", "-q", remote)
	repo := t.TempDir()
	runGitCommand(t, "", "-C", repo, "init", "-q", "-b", "task-branch")
	runGitCommand(t, "", "-C", repo, "config", "user.name", "test")
	runGitCommand(t, "", "-C", repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "work.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, "", "-C", repo, "add", "work.txt")
	runGitCommand(t, "", "-C", repo, "commit", "-qm", "work")
	runGitCommand(t, "", "-C", repo, "remote", "add", "upstream", remote)
	w := &healWorkdir{dir: repo, branch: "task-branch"}
	hooks := filepath.Join(repo, "hooks")
	if err := os.Mkdir(hooks, 0o700); err != nil {
		t.Fatal(err)
	}
	// Git's hook integration requires an executable entry point. A failing
	// hook must never run during supervisor publication after the confined gate.
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte("#!/bin/sh\nexit 97\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, repo, "config", "core.hooksPath", hooks)
	runGitCommand(t, repo, "add", "hooks/pre-push")
	runGitCommand(t, repo, "commit", "-qm", "repository hook fixture")
	head, err := (gitPusher{remote: "upstream"}).Push(t.Context(), w)
	if err != nil || len(head) != 40 {
		t.Fatalf("Push head=%q err=%v", head, err)
	}
	remoteHead := strings.TrimSpace(runGitCommand(t, "", "--git-dir", remote, "rev-parse", "refs/heads/task-branch"))
	if remoteHead != head {
		t.Fatalf("remote head=%q want=%q", remoteHead, head)
	}
	// Local verification is published only after the exact branch is present at
	// the remote; a status write failure must not be reported as a green push.
	statusFailure := errors.New("verification service unavailable")
	for _, callbackErr := range []error{nil, statusFailure} {
		called := false
		published, err := (gitPusher{remote: "upstream", afterPush: func(_ context.Context, got port.Workdir, sha string) error {
			called = true
			if got != w || sha != remoteHead {
				t.Fatalf("verification publication work=%v sha=%q", got, sha)
			}
			if actual := strings.TrimSpace(runGitCommand(t, "", "--git-dir", remote, "rev-parse", "refs/heads/task-branch")); actual != sha {
				t.Fatalf("verification preceded remote publication: %q != %q", actual, sha)
			}
			return callbackErr
		}}).Push(t.Context(), w)
		if !called || !errors.Is(err, callbackErr) || (callbackErr == nil && published != head) || (callbackErr != nil && published != "") {
			t.Fatalf("publication called=%v head=%q err=%v", called, published, err)
		}
	}
	// An unborn HEAD makes the old index dirty; refuse publication before any
	// remote write, rather than guessing which revision was verified.
	runGitCommand(t, "", "-C", repo, "symbolic-ref", "HEAD", "refs/heads/missing")
	if _, err := (gitPusher{remote: "upstream"}).Push(t.Context(), w); err == nil || !strings.Contains(err.Error(), "uncheckpointed changes") {
		t.Fatalf("Push with unreadable HEAD error = %v", err)
	}
	if logs, err := (emptyLogFetcher{}).FetchFailedLogs(t.Context(), port.Proposal{}); err != nil || logs != "" {
		t.Fatalf("empty logs=%q err=%v", logs, err)
	}
	checks := codeGateChecks()
	if len(checks) != 1 || checks[0].Name != "code-gate" {
		t.Fatalf("gate checks=%#v", checks)
	}
}

func TestGitPusherFailsClosedBeforeAndAtGitBoundary(t *testing.T) {
	valid := &healWorkdir{dir: t.TempDir(), branch: "task"}
	for name, tc := range map[string]struct {
		p gitPusher
		w port.Workdir
	}{
		"nil workdir": {p: gitPusher{remote: "origin"}},
		"no remote":   {p: gitPusher{}, w: valid},
		"no branch":   {p: gitPusher{remote: "origin"}, w: &healWorkdir{dir: valid.dir}},
		"git failure": {p: gitPusher{remote: "missing"}, w: valid},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tc.p.Push(t.Context(), tc.w); err == nil {
				t.Fatal("invalid push boundary was accepted")
			}
		})
	}
}

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

type resolverStore struct {
	states      []reviewobligation.State
	openErr     error
	completeErr error
	completed   string
}

func (*resolverStore) Preflight(context.Context) error                           { return nil }
func (*resolverStore) Append(context.Context, reviewobligation.Obligation) error { return nil }
func (*resolverStore) Get(context.Context, string) (reviewobligation.Obligation, bool, error) {
	return reviewobligation.Obligation{}, false, nil
}
func (*resolverStore) State(context.Context, string) (reviewobligation.State, bool, error) {
	return reviewobligation.State{}, false, nil
}
func (s *resolverStore) Open(context.Context, string) ([]reviewobligation.State, error) {
	return append([]reviewobligation.State(nil), s.states...), s.openErr
}
func (s *resolverStore) CompleteHandoff(_ context.Context, id string) error {
	s.completed = id
	return s.completeErr
}
func (*resolverStore) Begin(context.Context, reviewobligation.Attempt) (reviewobligation.AttemptLease, error) {
	return nil, nil
}
func (*resolverStore) Resume(context.Context, string, string) (reviewobligation.AttemptLease, error) {
	return nil, nil
}
func (*resolverStore) Attempt(context.Context, string, string) (reviewobligation.Attempt, string, bool, error) {
	return reviewobligation.Attempt{}, "", false, nil
}
func (*resolverStore) Terminal(context.Context, string, string) (reviewobligation.Terminal, bool, error) {
	return reviewobligation.Terminal{}, false, nil
}
func (*resolverStore) RecordPair(context.Context, reviewobligation.PublicationPair) error { return nil }
func (*resolverStore) AppendTerminal(context.Context, reviewobligation.Terminal) error    { return nil }
func (*resolverStore) Verified(context.Context, string) (bool, error)                     { return false, nil }

func TestReviewHandoffRepairerProjectsPendingAndPairRepairStates(t *testing.T) {
	obligation := reviewobligation.Obligation{
		ID: "review-1", TaskID: "T1", TaskNumber: 17, Harness: "claude",
		ProposalRef: "github:owner/repo#7", Revision: "abc", DemandID: "demand-1",
		DemandProfile: modelRoutingTask().DemandProfile,
	}
	store := &resolverStore{states: []reviewobligation.State{
		{Obligation: obligation, Status: "handoff_pending"},
		{Obligation: obligation, Status: "verified"},
	}}
	repairer := reviewHandoffRepairer{store: store, scope: "feature/001", repository: "owner/repo"}
	pending, err := repairer.Pending(t.Context())
	if err != nil || len(pending) != 1 || pending[0].ObligationID != obligation.ID || pending[0].Task.Number != 17 {
		t.Fatalf("Pending=%#v err=%v", pending, err)
	}
	if err := repairer.Complete(t.Context(), obligation.ID); err != nil || store.completed != obligation.ID {
		t.Fatalf("Complete id=%q err=%v", store.completed, err)
	}

	pair := reviewobligation.PublicationPair{ObligationID: obligation.ID}
	store.states = []reviewobligation.State{
		{Obligation: obligation, Status: "pair_verified", Pair: &pair},
		{Obligation: obligation, Status: "pair_verified"},
		{Obligation: obligation, Status: "pending"},
	}
	results, err := repairer.RepairPairs(t.Context())
	if err != nil || len(results) != 2 {
		t.Fatalf("RepairPairs=%#v err=%v", results, err)
	}
	for _, result := range results {
		if result.Result.Err == nil || result.TaskID != obligation.TaskID {
			t.Fatalf("fail-closed pair result=%#v", result)
		}
	}
}

func TestReviewHandoffRepairerSurfacesStoreAndIdentityFailures(t *testing.T) {
	want := errors.New("store unavailable")
	store := &resolverStore{openErr: want, completeErr: want}
	repairer := reviewHandoffRepairer{store: store}
	if _, err := repairer.Pending(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Pending error=%v", err)
	}
	if _, err := repairer.RepairPairs(t.Context()); !errors.Is(err, want) {
		t.Fatalf("RepairPairs error=%v", err)
	}
	if err := repairer.Complete(t.Context(), "review-1"); !errors.Is(err, want) {
		t.Fatalf("Complete error=%v", err)
	}
	store.openErr = nil
	store.states = []reviewobligation.State{{Status: "handoff_pending", Obligation: reviewobligation.Obligation{ID: "review-1", TaskID: "T1"}}}
	if _, err := repairer.Pending(t.Context()); err == nil || !strings.Contains(err.Error(), "tracker task number") {
		t.Fatalf("identity error=%v", err)
	}
}

func (*healWorkdir) ChangeSize(context.Context) (changesize.Size, error) {
	return changesize.Size{}, nil
}
