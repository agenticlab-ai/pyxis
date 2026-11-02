package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

func issueRecoveryFixture(t *testing.T, command string) (*issueRun, string, string, port.Proposal) {
	t.Helper()
	repo := implementReviewGitFixture(t)
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis/config.yaml"), []byte("code-gate:\n  required:\n    command: "+command+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runRoutingGit(t, repo, "add", ".pyxis/config.yaml")
	runRoutingGit(t, repo, "commit", "-qm", "test: recovery local verification")
	runRoutingGit(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
	revision, err := localGit(t.Context(), repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "owner/repository"}
	snapshot := port.IssueSnapshot{Number: 17, Open: true, Executable: true}
	path := filepath.Join(t.TempDir(), "input.json")
	writeJSONFile(t, path, issueCharter(t, binding, snapshot))
	now := func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
	r, _, err := admitIssue(t.Context(), &issueReadFixture{snapshot: snapshot}, binding, 17, path, repo, "origin", now(), now)
	if err != nil {
		t.Fatal(err)
	}
	return r, repo, strings.TrimSpace(string(revision)), port.Proposal{Number: 77, Branch: dispatch.HeadBranch(r.task)}
}

type issueRecoveryHost struct {
	*issueProofHost
	open      map[string]port.OpenProposal
	probeErr  error
	headAfter string
	headReads int
	inspected func()
}

func (h *issueRecoveryHost) OpenProposals(_ context.Context, tasks []port.Task) (map[string]port.OpenProposal, error) {
	if len(tasks) != 1 || tasks[0].HeadBranch != dispatch.HeadBranch(tasks[0]) {
		return nil, errors.New("recovery did not probe the exact selected task branch")
	}
	return h.open, h.probeErr
}

func (h *issueRecoveryHost) ProposalHead(ctx context.Context, p port.Proposal) (string, error) {
	h.headReads++
	if h.headReads > 1 && h.headAfter != "" {
		return h.headAfter, nil
	}
	return h.issueProofHost.ProposalHead(ctx, p)
}

func (h *issueRecoveryHost) InspectProposal(ctx context.Context, p port.Proposal) (port.Landing, string, port.CheckState, error) {
	if h.inspected != nil {
		h.inspected()
	}
	return h.issueProofHost.InspectProposal(ctx, p)
}

func TestIssueRecoveryReconcilesOnlyLocallyVerifiedNativeProposal(t *testing.T) {
	for _, fault := range []string{"", "local CI", "no native proposal", "probe denied", "unselected proposal", "multiple proposals", "invalid number", "head denied", "invalid revision", "local gate failed", "head moved", "inspection denied", "closed proposal", "local status denied", "association denied"} {
		t.Run(fault, func(t *testing.T) {
			command := "git rev-parse origin/HEAD"
			if fault == "local gate failed" {
				command = "false"
			}
			r, repo, revision, proposal := issueRecoveryFixture(t, command)
			host := &issueRecoveryHost{issueProofHost: &issueProofHost{head: revision, landing: port.LandingOpen, checks: port.CheckGreen}, open: map[string]port.OpenProposal{r.task.ID: {Number: proposal.Number}}}
			var boundary executionHost = host
			switch fault {
			case "no native proposal":
				host.open = nil
			case "probe denied":
				host.probeErr = errors.New("denied")
			case "unselected proposal":
				host.open = map[string]port.OpenProposal{"other": {Number: 78}}
			case "multiple proposals":
				host.open["other"] = port.OpenProposal{Number: 78}
			case "invalid number":
				host.open[r.task.ID] = port.OpenProposal{}
			case "head denied":
				host.headErr = errors.New("denied")
			case "invalid revision":
				host.head = "--upload-pack=bad"
			case "head moved":
				host.headAfter = strings.Repeat("a", 40)
			case "inspection denied":
				host.readErr = errors.New("denied")
			case "closed proposal":
				host.landing = port.LandingClosedUnlanded
			case "local CI", "local status denied":
				status := &localStatusFake{head: revision}
				if fault == "local status denied" {
					status.statusErr = errors.New("denied")
				}
				boundary = &localVerifiedHost{executionHost: host, status: status, mode: "local"}
			case "association denied":
				host.inspected = func() { runRoutingGit(t, repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing")) }
			}
			p, associated, err := r.reconcileProposal(t.Context(), boundary, repo, "origin")
			wantError := fault != "" && fault != "local CI" && fault != "no native proposal"
			if (err != nil) != wantError {
				t.Fatalf("reconciliation: %+v %v %v", p, associated, err)
			}
			if wantError || fault == "no native proposal" {
				if associated {
					t.Fatal("failed recovery retained an association")
				}
				return
			}
			if !associated || p != proposal {
				t.Fatalf("lost exact recovered proposal: %+v %v", p, associated)
			}
			if local, err := r.store.LocalVerified(t.Context(), r.contract, p, revision); err != nil || !local {
				t.Fatalf("recovery did not retain exact local proof: %v", err)
			}
			// Once retained, native discovery is unnecessary even during an outage.
			host.probeErr = errors.New("denied")
			if again, ok, err := r.reconcileProposal(t.Context(), host, repo, "origin"); err != nil || !ok || again != p {
				t.Fatalf("retained recovery replay: %+v %v", again, err)
			}
		})
	}
}

func TestIssueRecoveryCheckoutFailsClosedAtGitBoundaries(t *testing.T) {
	_, repo, revision, _ := issueRecoveryFixture(t, "test -f .pyxis/config.yaml")
	for failedOperation := 1; failedOperation <= 6; failedOperation++ {
		t.Run(string(rune('0'+failedOperation)), func(t *testing.T) {
			calls := 0
			git := func(ctx context.Context, dir string, args ...string) ([]byte, error) {
				calls++
				if calls == failedOperation {
					return nil, errors.New("git unavailable")
				}
				return localGit(ctx, dir, args...)
			}
			if err := verifyPublishedLocal(t.Context(), repo, "origin", revision, git); err == nil || calls != failedOperation {
				t.Fatalf("partial recovery checkout certified: %v after %d calls", err, calls)
			}
		})
	}
	// A scratch-directory outage must fail before any native/Git mutation.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if err := verifyPublishedLocal(t.Context(), repo, "origin", revision, localGit); err == nil {
		t.Fatal("missing recovery scratch directory accepted")
	}
}

func TestIssueMissingReviewRequiresExactRetainedLocalProof(t *testing.T) {
	r, _, revision, p := issueRecoveryFixture(t, "test -f .pyxis/config.yaml")
	if err := r.store.AssociateVerified(t.Context(), r.contract, p, revision); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"", "native denied", "moved head", "invalid revision"} {
		t.Run(fault, func(t *testing.T) {
			host := &issueProofHost{head: revision, landing: port.LandingOpen, checks: port.CheckGreen}
			switch fault {
			case "native denied":
				host.readErr = errors.New("denied")
			case "moved head":
				host.head = strings.Repeat("a", 40)
			case "invalid revision":
				host.head = "invalid"
			}
			inspect := issueVerifiedInspector{executionHost: host, session: r}
			_, got, _, err := inspect.InspectProposal(t.Context(), p)
			if (err != nil) != (fault != "") || (fault == "" && got != revision) {
				t.Fatalf("missing handoff inspection: %s %v", got, err)
			}
		})
	}
}

type issueRecoveryReviewer struct {
	worker.ReviewService
	preflightErr error
	capture      worker.ReviewCapture
	task         port.Task
	proposal     port.Proposal
}

func (r *issueRecoveryReviewer) Preflight(context.Context) error { return r.preflightErr }
func (r *issueRecoveryReviewer) Capture(_ context.Context, task port.Task, p port.Proposal) worker.ReviewCapture {
	r.task, r.proposal = task, p
	return r.capture
}

func TestIssueMissingReviewUsesSelectedServiceWithoutImplementation(t *testing.T) {
	for _, fault := range []string{"", "inherited", "native", "no factory", "no resolver", "resolver denied", "factory denied", "missing service", "preflight denied", "capture denied", "incomplete capture", "unpersisted capture"} {
		t.Run(fault, func(t *testing.T) {
			task := port.Task{ID: "issue", Number: 17}
			p := port.Proposal{Number: 77, Branch: "feat/issue"}
			r := &issueRun{task: task}
			reviewer := &issueRecoveryReviewer{capture: worker.ReviewCapture{ProposalRef: "github:owner/repository#77", Revision: strings.Repeat("a", 40), ObligationID: "obligation", Harness: entity.Codex, Persisted: true}}
			harness := entity.Codex
			resolved := 0
			var resolve dispatch.RuntimeResolver = func(observed port.Task) (entity.Harness, port.AgentRuntime, error) {
				resolved++
				if observed.ID != task.ID || fault == "resolver denied" {
					return "", nil, errors.New("denied")
				}
				return entity.Codex, nil, nil
			}
			var factory dispatch.ReviewInvokerFactory = func(selected entity.Harness) (worker.ReviewService, error) {
				if selected != entity.Codex || fault == "factory denied" {
					return nil, errors.New("denied")
				}
				if fault == "missing service" {
					return nil, nil
				}
				return reviewer, nil
			}
			switch fault {
			case "inherited", "resolver denied", "no resolver":
				harness = ""
				if fault == "no resolver" {
					resolve = nil
				}
			case "native":
				harness = entity.Claude
				r.task.NativeRouting = &taskrouting.Spec{Review: taskrouting.Route{Harness: entity.Codex}}
			case "no factory":
				factory = nil
			case "preflight denied":
				reviewer.preflightErr = errors.New("denied")
			case "capture denied":
				reviewer.capture.Err = errors.New("denied")
			case "incomplete capture":
				reviewer.capture.Revision = ""
			case "unpersisted capture":
				reviewer.capture.Persisted = false
			}
			err := r.captureMissingReview(t.Context(), p, harness, resolve, factory)
			wantError := fault != "" && fault != "inherited" && fault != "native"
			if (err != nil) != wantError {
				t.Fatalf("review recovery: %v", err)
			}
			wantResolved := 0
			if fault == "inherited" {
				wantResolved = 1
			}
			if !wantError && (reviewer.task.ID != task.ID || reviewer.proposal != p || resolved != wantResolved) {
				t.Fatal("recovery changed the selected task, proposal or harness")
			}
		})
	}
}
