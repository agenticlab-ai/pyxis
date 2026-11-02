package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/github"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/watch"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

type pyxisReviewStore struct {
	reviewobligation.Store
	states []reviewobligation.State
	err    error
}

func (s pyxisReviewStore) Open(context.Context, string) ([]reviewobligation.State, error) {
	return s.states, s.err
}

func TestGitHubWatchReadersParseAndNameFailures(t *testing.T) {
	t.Run("closed issues", func(t *testing.T) {
		installPyxisGHHelper(t, `[{"number":7},{"number":9}]`, "", 0)
		got, err := closedIssues(t.Context(), t.TempDir(), "owner/repo", "ready")
		if err != nil || !got[7] || !got[9] || len(got) != 2 {
			t.Fatalf("closed issues = %v, %v", got, err)
		}
	})
	t.Run("open proposals", func(t *testing.T) {
		installPyxisGHHelper(t, `[{"number":21,"headRefName":"feat/work-17"},{"number":22,"headRefName":"not-a-task"}]`, "", 0)
		got, err := openProposalsByIssue(t.Context(), t.TempDir(), "owner/repo")
		if err != nil || got[17] != 21 || len(got) != 1 {
			t.Fatalf("open proposals = %v, %v", got, err)
		}
	})
	t.Run("malformed response", func(t *testing.T) {
		installPyxisGHHelper(t, `{`, "", 0)
		if _, err := closedIssues(t.Context(), t.TempDir(), "owner/repo", "ready"); err == nil || !strings.Contains(err.Error(), "parse closed issues") {
			t.Fatalf("closed issue parse error = %v", err)
		}
		if _, err := openProposalsByIssue(t.Context(), t.TempDir(), "owner/repo"); err == nil || !strings.Contains(err.Error(), "parse open proposals") {
			t.Fatalf("proposal parse error = %v", err)
		}
	})
	t.Run("transport failure", func(t *testing.T) {
		installPyxisGHHelper(t, "", "permission denied", 1)
		if _, err := runGH(t.Context(), t.TempDir(), "issue", "list"); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("runGH error = %v", err)
		}
		if _, err := closedIssues(t.Context(), t.TempDir(), "owner/repo", "ready"); err == nil || !strings.Contains(err.Error(), "list closed issues") {
			t.Fatalf("closedIssues error = %v", err)
		}
		if _, err := openProposalsByIssue(t.Context(), t.TempDir(), "owner/repo"); err == nil || !strings.Contains(err.Error(), "list open proposals") {
			t.Fatalf("openProposals error = %v", err)
		}
	})
}

func TestClaimedIssuesReadsOnlyPositiveNumericClaimRefs(t *testing.T) {
	repo := t.TempDir()
	runPyxisGit(t, repo, "init", "-q")
	runPyxisGit(t, repo, "config", "user.name", "Pyxis Test")
	runPyxisGit(t, repo, "config", "user.email", "pyxis@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runPyxisGit(t, repo, "add", "README")
	runPyxisGit(t, repo, "commit", "-qm", "base")
	runPyxisGit(t, repo, "update-ref", "refs/claim/17", "HEAD")
	runPyxisGit(t, repo, "update-ref", "refs/claim/not-a-number", "HEAD")
	got, err := claimedIssues(t.Context(), repo, ".")
	if err != nil || !got[17] || len(got) != 1 {
		t.Fatalf("claimed issues = %v, %v", got, err)
	}
	if _, err := claimedIssues(t.Context(), repo, "missing"); err == nil || !strings.Contains(err.Error(), "list claims") {
		t.Fatalf("missing remote error = %v", err)
	}
}

func TestPrintChecksAndWiringOperationsRenderEveryShape(t *testing.T) {
	checks := preflight.Report{
		{Check: "one", Status: preflight.StatusPass},
		{Check: "two", Status: preflight.StatusAdvise, Warning: "heads up"},
		{Check: "three", Status: preflight.StatusRefuse, Reason: "blocked", Conflicts: []string{"a", "b"}, Remedy: "repair it", Route: "pyxis adopt"},
	}
	var out bytes.Buffer
	printChecks(&out, checks)
	for _, want := range []string{"check one: pass", "check two: advise", "heads up", "conflicts: a, b", "remedy: repair it", "pyxis adopt"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("check output missing %q: %s", want, out.String())
		}
	}
	out.Reset()
	printWiringOps(&out, []wire.Op{
		{Name: wire.OpPushWorkflow, Transport: wire.TransportConverge, Condition: "ci_mode=both"},
		{Name: wire.OpCreateRepo, Transport: wire.TransportGH, Condition: "repo=create", Slug: "owner/repo"},
	})
	if strings.Count(out.String(), "wiring:") < 3 || !strings.Contains(out.String(), "probe:") || !strings.Contains(out.String(), "create-repo") {
		t.Fatalf("wiring output = %q", out.String())
	}
}

func TestGitHubPlanSourceCombinesTrackerProposalAndClaimState(t *testing.T) {
	installPyxisGHHelper(t, `[{"number":7,"headRefName":"feat/work-17"},{"number":9,"headRefName":"feat/work-9"}]`, "", 0)
	restore := listClaims
	defer func() { listClaims = restore }()
	listClaims = func(context.Context, string, string) (map[int]bool, error) {
		return map[int]bool{17: true}, nil
	}

	source := ghPlanSource(t.TempDir(), "owner/repo", "ready", "origin", planMap{
		planRef:     "feature/001",
		issueByTask: map[string]int{"T1": 17, "T2": 9},
		blockedBy:   map[string][]string{"T1": {}, "T2": {"T1"}},
	})
	plan, err := source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]watch.Task{}
	for _, task := range plan.Tasks {
		byID[task.ID] = task
	}
	if got := byID["T1"]; got.Closed || !got.Claimed || got.Proposal == nil || got.Proposal.Number != 7 {
		t.Fatalf("T1 = %+v, want open claimed task with proposal 7", got)
	}
	if got := byID["T2"]; !got.Closed || got.Claimed || got.Proposal == nil || got.Proposal.Number != 9 || len(got.BlockedBy) != 1 {
		t.Fatalf("T2 = %+v, want closed dependent task with proposal 9", got)
	}
}

func TestCarinaCadenceAdapterDeclaresAdviceCadence(t *testing.T) {
	if got := (carinaCadenceAdapter{}).Cadence(t.Context()); got <= 0 {
		t.Fatalf("cadence = %s, want positive advice", got)
	}
}

func TestGitHubCadenceAdapterDeclaresAdviceCadence(t *testing.T) {
	adapter := cadenceAdapter{StateReader: ghcodehost.NewStateReader("owner/repo")}
	if got := adapter.Cadence(t.Context()); got <= 0 {
		t.Fatalf("cadence = %s, want positive advice", got)
	}
}

func TestWatchBoundaryHelpers(t *testing.T) {
	t.Run("review provider maps and reports store states", func(t *testing.T) {
		adapter := reviewProviderAdapter{store: pyxisReviewStore{states: []reviewobligation.State{{
			Obligation: reviewobligation.Obligation{
				ID: "obligation-1", TaskID: "T1", ProposalRef: "github:o/r#7",
				Revision: "abc123", Harness: "codex",
			},
			Status: "failed", AttemptCount: 2, LastFailureClass: "timeout",
		}}}}
		got, err := adapter.Open(t.Context(), "feature/001")
		if err != nil || len(got) != 1 || got[0].ID != "obligation-1" || got[0].AttemptCount != 2 {
			t.Fatalf("Open = %+v, %v", got, err)
		}

		wantErr := errors.New("store unavailable")
		adapter = reviewProviderAdapter{store: pyxisReviewStore{err: wantErr}}
		if _, err := adapter.Open(t.Context(), "feature/001"); !errors.Is(err, wantErr) {
			t.Fatalf("Open error = %v, want %v", err, wantErr)
		}
	})

	t.Run("subprocess wrappers name command failures", func(t *testing.T) {
		args := sweepArgs{override: "/usr/bin/false", repo: t.TempDir(), repoSlug: "o/r", label: "ready", scope: "feature/001", trackerKind: "gh"}
		if err := subprocessSweep(args, &bytes.Buffer{}, &bytes.Buffer{})(t.Context()); err == nil || !strings.Contains(err.Error(), "sweep") {
			t.Fatalf("subprocessSweep error = %v", err)
		}
		if err := subprocessReviewSweep(args, &bytes.Buffer{}, &bytes.Buffer{})(t.Context(), watch.ReviewRecoveryRequest{DriveRoundID: "round-1"}); err == nil || !strings.Contains(err.Error(), "review-only sweep") {
			t.Fatalf("subprocessReviewSweep error = %v", err)
		}
	})

	t.Run("default command and JSON faces", func(t *testing.T) {
		cmd, err := implementNowCommand(t.Context(), "", []string{"--repo", "fixture"})
		if err != nil || cmd == nil || len(cmd.Args) < 3 {
			t.Fatalf("implementNowCommand = %+v, %v", cmd, err)
		}
		if got := string(findJSON([]byte(`{"ok":true}`))); got != `{"ok":true}` {
			t.Fatalf("object-first findJSON = %q", got)
		}
		if got := string(findJSON([]byte("plain output"))); got != "plain output" {
			t.Fatalf("fallback findJSON = %q", got)
		}
	})
}

func installPyxisGHHelper(t *testing.T, stdout, stderr string, exit int) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	gh := filepath.Join(dir, "gh")
	if err := os.Symlink(executable, gh); err != nil {
		t.Skipf("cannot install Go test helper as gh: %v", err)
	}
	t.Setenv("PYXIS_TEST_GH_HELPER", "1")
	t.Setenv("PYXIS_TEST_GH_STDOUT", stdout)
	t.Setenv("PYXIS_TEST_GH_STDERR", stderr)
	t.Setenv("PYXIS_TEST_GH_EXIT", string(rune('0'+exit)))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func runPyxisGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
