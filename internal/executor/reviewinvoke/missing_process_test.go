package reviewinvoke

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

type missingProcessCoordinator struct {
	fakeProcessCoordinator
	missing []implreview.AdmittedProcess
	err     error
}

func (p *missingProcessCoordinator) MissingProcesses(context.Context, implreview.ReviewSubject) ([]implreview.AdmittedProcess, error) {
	return p.missing, p.err
}

func missingAttemptFixture(t *testing.T) (*Invoker, *fakeRuntime, implreview.AdmittedProcess, reviewobligation.AttemptLease) {
	t.Helper()
	rt := &fakeRuntime{}
	verifier := &fakeVerifier{}
	i := lifecycleInvoker(t, rt, verifier, time.Minute)
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "repo")
	runGit(t, root, "init", "--bare", "-q", remote)
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-q")
	writeFile(t, filepath.Join(repo, "README"), "fixture\n")
	runGit(t, repo, "add", "README")
	runGit(t, repo, "commit", "-qm", "fixture")
	runGit(t, repo, "remote", "add", "origin", remote)
	gitStore, err := reviewobligation.NewGitStore(repo, "origin", "refs/pyxis/test-review-state")
	if err != nil {
		t.Fatal(err)
	}
	i.store = gitStore
	i.terminalVerifier, err = NewStoreTerminalVerifier(gitStore, verifier)
	if err != nil {
		t.Fatal(err)
	}
	captureForInvoke(i, port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	o, found, err := i.store.Get(t.Context(), "review-obligation-test")
	if err != nil || !found {
		t.Fatalf("obligation: found=%v err=%v", found, err)
	}
	process := implreview.AdmittedProcess{ReviewProcessID: "process-original", Subject: implreview.ReviewSubject{
		ReviewObligationID: o.ID, ProposalID: o.ProposalRef, ReviewedRevision: o.Revision,
	}}
	i.processes = &missingProcessCoordinator{missing: []implreview.AdmittedProcess{process}}
	lease, err := i.store.Begin(t.Context(), reviewobligation.Attempt{SchemaVersion: reviewobligation.AttemptSchema,
		ObligationID: o.ID, AttemptID: process.ReviewProcessID, RunID: "run-original", AttemptProofRef: "proof-original"})
	if err != nil {
		t.Fatal(err)
	}
	return i, rt, process, lease
}

func TestInvokeRepairsMissingAttemptTerminalBeforeFreshReview(t *testing.T) {
	i, rt, process, lease := missingAttemptFixture(t)
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	// The canonical process completed without a handoff, but writing the
	// attempt's failed terminal was interrupted. No live owner holds its fence.
	result := i.Invoke(t.Context(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err != nil || result.Status != "verified" || rt.launches != 1 || result.AttemptID == process.ReviewProcessID {
		t.Fatalf("fresh recovery=%+v launches=%d", result, rt.launches)
	}
	terminal, found, err := i.store.Terminal(t.Context(), process.Subject.ReviewObligationID, process.ReviewProcessID)
	if err != nil || !found || terminal.Status != "failed" || terminal.FailureClass != worker.ReviewFailureInfrastructure || terminal.ArtifactRef != "" {
		t.Fatalf("original terminal=%+v found=%v err=%v", terminal, found, err)
	}
}

type missingTerminalWriteFailure struct{ reviewobligation.Store }

func (s missingTerminalWriteFailure) AppendTerminal(context.Context, reviewobligation.Terminal) error {
	return errors.New("terminal write unavailable")
}

type missingStatePublication struct{ reviewobligation.Store }

func (s missingStatePublication) State(ctx context.Context, id string) (reviewobligation.State, bool, error) {
	state, found, err := s.Store.State(ctx, id)
	state.Pair = &reviewobligation.PublicationPair{AttemptID: "process-original"}
	return state, found, err
}

func TestMissingProcessRecoveryPreservesOwnershipAndPublicationFences(t *testing.T) {
	tests := []struct {
		name      string
		holdFence bool
		configure func(*Invoker, *missingProcessCoordinator)
		want      string
	}{
		{name: "live process has no terminal", configure: func(_ *Invoker, p *missingProcessCoordinator) { p.missing = nil }, want: "attempt already active"},
		{name: "original owner holds fence", holdFence: true, want: "attempt already active"},
		{name: "different revision", configure: func(_ *Invoker, p *missingProcessCoordinator) { p.missing[0].Subject.ReviewedRevision = "other" }, want: "mismatches obligation"},
		{name: "missing process identity", configure: func(_ *Invoker, p *missingProcessCoordinator) { p.missing[0].ReviewProcessID = "" }, want: "mismatches obligation"},
		{name: "canonical evidence unavailable", configure: func(_ *Invoker, p *missingProcessCoordinator) { p.err = errors.New("process evidence unavailable") }, want: "process evidence unavailable"},
		{name: "process never admitted an attempt", configure: func(_ *Invoker, p *missingProcessCoordinator) { p.missing[0].ReviewProcessID = "unadmitted" }, want: "attempt already active"},
		{name: "terminal persistence unavailable", configure: func(i *Invoker, _ *missingProcessCoordinator) { i.store = missingTerminalWriteFailure{i.store} }, want: "terminal write unavailable"},
		{name: "publication evidence appeared", configure: func(i *Invoker, _ *missingProcessCoordinator) { i.store = missingStatePublication{i.store} }, want: "acquired publication evidence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i, rt, process, lease := missingAttemptFixture(t)
			if tt.holdFence {
				defer func() {
					if err := lease.Close(); err != nil {
						t.Error(err)
					}
				}()
			} else if err := lease.Close(); err != nil {
				t.Fatal(err)
			}
			if tt.configure != nil {
				tt.configure(i, i.processes.(*missingProcessCoordinator))
			}
			result := i.Invoke(t.Context(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
			if result.Err == nil || !strings.Contains(result.Err.Error(), tt.want) || result.Status != "failed" || rt.launches != 0 {
				t.Fatalf("refusal=%+v launches=%d", result, rt.launches)
			}
			if terminal, found, err := i.store.Terminal(t.Context(), process.Subject.ReviewObligationID, process.ReviewProcessID); err != nil || found {
				t.Fatalf("original outcome changed: %+v found=%v err=%v", terminal, found, err)
			}
		})
	}
}

func TestMissingProcessRecoveryPreservesSettledFailure(t *testing.T) {
	i, rt, process, lease := missingAttemptFixture(t)
	original := reviewobligation.Terminal{SchemaVersion: reviewobligation.TerminalSchema, ObligationID: process.Subject.ReviewObligationID,
		AttemptID: process.ReviewProcessID, Status: "failed", FailureClass: worker.ReviewFailureArtifactUntrusted}
	if err := i.store.AppendTerminal(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	result := i.Invoke(t.Context(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err != nil || result.Status != "verified" || rt.launches != 1 {
		t.Fatalf("retry=%+v launches=%d", result, rt.launches)
	}
	terminal, found, err := i.store.Terminal(t.Context(), process.Subject.ReviewObligationID, process.ReviewProcessID)
	if err != nil || !found || terminal != original {
		t.Fatalf("settled failure changed: %+v found=%v err=%v", terminal, found, err)
	}
}
