package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/drivetogreen"
	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/prdescription/prdescriptiontest"
)

type ciRefusalLease struct {
	err, releaseErr, releaseContextErr error
	acquired, released                 int
}

func (l *ciRefusalLease) Acquire(context.Context, port.Task) (port.Claim, error) {
	l.acquired++
	if l.err != nil {
		return nil, l.err
	}
	return ciRefusalClaim{l}, nil
}

type ciRefusalClaim struct{ lease *ciRefusalLease }

func (c ciRefusalClaim) Release(ctx context.Context) error {
	c.lease.released++
	c.lease.releaseContextErr = ctx.Err()
	return c.lease.releaseErr
}

type ciRefusalSpace struct {
	work   port.Workdir
	err    error
	opened func()
}

func (s *ciRefusalSpace) ExistingDescendant(context.Context, port.Task, string) (port.Workdir, error) {
	if s.opened != nil {
		s.opened()
	}
	return s.work, s.err
}

func TestAssociatedCIRefusalsDoNotLaunchAndReleaseClaim(t *testing.T) {
	failure := errors.New("fixture unavailable")
	for _, tc := range []struct {
		name, want         string
		acquired, released int
		active             bool
		cause              error
	}{
		{name: "native read", want: "fixture unavailable", cause: failure},
		{name: "closed proposal", want: "is closed"},
		{name: "foreign branch", want: "branch or head differs"},
		{name: "invalid revision", want: "branch or head differs"},
		{name: "claimed elsewhere", want: "claim associated", acquired: 1, active: true, cause: port.ErrClaimLost},
		{name: "claim unavailable", want: "claim associated", acquired: 1, cause: failure},
		{name: "dirty checkout", want: "fixture unavailable", acquired: 1, released: 1, cause: failure},
		{name: "cancelled checkout", want: "context canceled", acquired: 1, released: 1, cause: context.Canceled},
		{name: "missing checkpoint", want: "no retained implementer", acquired: 1, released: 1},
		{name: "corrupt checkpoint", want: "decode issue checkpoint", acquired: 1, released: 1},
		{name: "foreign contract", want: "no matching retained", acquired: 1, released: 1},
		{name: "foreign task", want: "no matching retained", acquired: 1, released: 1},
		{name: "foreign checkpoint branch", want: "no matching retained", acquired: 1, released: 1},
		{name: "unproven ancestor", want: "no matching retained", acquired: 1, released: 1},
		{name: "release failure", want: "no retained implementer", acquired: 1, released: 1},
		{name: "green without checkout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			r := &issueRun{contract: issueentry.Contract{ID: "sha256:" + strings.Repeat("c", 64)}, task: port.Task{ID: "task", Number: 17, HeadBranch: "feat/task"}}
			proposal := port.Proposal{Number: 77, Branch: dispatch.HeadBranch(r.task)}
			head := strings.Repeat("a", 40)
			host := &issueProofHost{landing: port.LandingOpen, head: head, checks: port.CheckPending}
			lease := &ciRefusalLease{}
			work := &recoveryWorkdir{dir: filepath.Join(t.TempDir(), "checkout"), branch: proposal.Branch}
			r.repo = work.dir
			space := &ciRefusalSpace{work: work}
			record := issueentry.Checkpoint{Schema: issueentry.CheckpointSchema, ContractID: r.contract.ID, TaskID: r.task.ID, Branch: proposal.Branch, Revision: head, Description: prdescriptiontest.Message}
			switch tc.name {
			case "native read":
				host.readErr = failure
			case "closed proposal":
				host.landing = port.LandingClosedUnlanded
			case "foreign branch":
				proposal.Branch = "feat/foreign"
			case "invalid revision":
				host.head = "not-a-commit"
			case "claimed elsewhere":
				lease.err = fmt.Errorf("held: %w", port.ErrClaimLost)
			case "claim unavailable":
				lease.err = failure
			case "dirty checkout":
				space.err = failure
			case "cancelled checkout":
				space.opened = cancel
				space.err = context.Canceled
			case "corrupt checkpoint":
				dir := executorruntime.TaskStateRoot(work.Dir())
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "issue-checkpoint.json"), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "foreign contract", "foreign task", "foreign checkpoint branch", "unproven ancestor":
				switch tc.name {
				case "foreign contract":
					record.ContractID = "sha256:" + strings.Repeat("f", 64)
				case "foreign task":
					record.TaskID = "another-task"
				case "foreign checkpoint branch":
					record.Branch = "feat/other"
				case "unproven ancestor":
					record.Revision = strings.Repeat("b", 40)
				}
				if err := issueentry.RetainCheckpoint(executorruntime.TaskStateRoot(work.Dir()), record); err != nil {
					t.Fatal(err)
				}
			case "release failure":
				lease.releaseErr = failure
			case "green without checkout":
				host.checks = port.CheckGreen
			}
			resolves := 0
			runtimeResolver := func(port.Task) (harness entity.Harness, rt port.AgentRuntime, err error) {
				resolves++
				return "", nil, failure
			}
			err := r.continueAssociatedCI(ctx, host, space, lease, config.Defaults(), &capturingEmitter{}, "origin", runtimeResolver, nil, time.Minute, proposal)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal=%v, want %q", err, tc.want)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("lost refusal cause: %v", err)
			}
			if resolves != 0 || work.pushes != 0 || lease.acquired != tc.acquired || lease.released != tc.released || r.active != tc.active || lease.releaseContextErr != nil {
				t.Fatalf("unexpected effects: resolve=%d push=%d acquire=%d release=%d active=%v release-context=%v", resolves, work.pushes, lease.acquired, lease.released, r.active, lease.releaseContextErr)
			}
		})
	}
}

func TestIssueRepairReservationUsesRetainedAllowance(t *testing.T) {
	if issueRepairReservation(nil) != nil {
		t.Fatal("unscoped driver received repair authority")
	}
	repo := implementReviewGitFixture(t)
	binding := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "owner/repository"}
	snapshot := port.IssueSnapshot{Number: 17, Open: true, Executable: true}
	contract, _, err := issueentry.Prepare(issueCharter(t, binding, snapshot), "github", binding.Key(), 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	store, err := issueentry.NewStore(repo, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AdmitWithRemoteRepairs(t.Context(), contract, 1); err != nil {
		t.Fatal(err)
	}
	reserve := issueRepairReservation(&issueRun{contract: contract, store: store})
	attempt, cap, err := reserve(t.Context(), "task")
	if err != nil || attempt != 1 || cap != 1 {
		t.Fatalf("reservation=%d/%d: %v", attempt, cap, err)
	}
	if _, _, err := reserve(t.Context(), "task"); !errors.Is(err, issueentry.ErrRemoteRepairExhausted) {
		t.Fatalf("exhausted authority reused: %v", err)
	}
}

func TestAssociatedCIRefusesProposalMovementDuringCheckpointReopen(t *testing.T) {
	r, repo, head, proposal := issueRecoveryFixture(t, "true")
	record := issueentry.Checkpoint{Schema: issueentry.CheckpointSchema, ContractID: r.contract.ID, TaskID: r.task.ID, Branch: proposal.Branch, Revision: head, Description: prdescriptiontest.Message}
	if err := issueentry.RetainCheckpoint(executorruntime.TaskStateRoot(repo), record); err != nil {
		t.Fatal(err)
	}
	host := &issueProofHost{head: head, after: strings.Repeat("f", 40), landing: port.LandingOpen, checks: port.CheckRed}
	work := &recoveryWorkdir{dir: repo, branch: proposal.Branch}
	space := &ciRefusalSpace{work: work}
	lease := &ciRefusalLease{}
	resolves := 0
	resolve := func(port.Task) (entity.Harness, port.AgentRuntime, error) {
		resolves++
		return "", nil, errors.New("unexpected fixer resolution")
	}
	err := r.continueAssociatedCI(t.Context(), host, space, lease, config.Defaults(), &capturingEmitter{}, "origin", resolve, nil, time.Minute, proposal)
	var moved *drivetogreen.ProposalChangedError
	if !errors.As(err, &moved) || moved.ExpectedHead != head || moved.ActualHead != host.after {
		t.Fatalf("movement during reopen was not refused against original PR head: %v", err)
	}
	if resolves != 0 || work.pushes != 0 || lease.acquired != 1 || lease.released != 1 {
		t.Fatalf("moved proposal caused side effects: resolves=%d pushes=%d claims=%+v", resolves, work.pushes, lease)
	}
}

func TestCurrentIssueReviewsPreserveHistoryAndRejectForeignState(t *testing.T) {
	r, _, head, p := issueRecoveryFixture(t, "true")
	if err := r.store.Associate(t.Context(), r.contract, p); err != nil {
		t.Fatal(err)
	}
	const repository = "owner/repository"
	ref, err := persist.ProposalRef(repository, p.Number)
	if err != nil {
		t.Fatal(err)
	}
	current := reviewobligation.State{Obligation: reviewobligation.Obligation{
		Repository: repository, TaskID: r.task.ID, TaskNumber: r.task.Number,
		DemandID: r.task.DemandID, ProposalRef: ref, Revision: head,
	}}
	old := current
	old.Obligation.Revision = strings.Repeat("b", 40)
	rows := &issueReviewRows{rows: []reviewobligation.State{old, current}}
	host := &issueProofHost{head: head}
	selected, err := r.currentReviewStates(t.Context(), host, rows, repository, p)
	if err != nil || len(selected) != 1 || selected[0].Obligation.Revision != head || len(rows.rows) != 2 {
		t.Fatalf("current selection lost history or mixed revisions: %+v %v", selected, err)
	}
	failure := errors.New("unavailable review source")
	host.headErr = failure
	if _, err := r.currentReviewStates(t.Context(), host, rows, repository, p); !errors.Is(err, failure) {
		t.Fatalf("lost head read failure: %v", err)
	}
	host.headErr = nil
	host.head = "invalid"
	if _, err := r.currentReviewStates(t.Context(), host, rows, repository, p); err == nil {
		t.Fatal("malformed current head selected a review")
	}
	host.head = head
	rows.err = failure
	if _, err := r.currentReviewStates(t.Context(), host, rows, repository, p); !errors.Is(err, failure) {
		t.Fatalf("lost review state read failure: %v", err)
	}
	rows.err = nil
	rows.rows[0].Obligation.TaskID = "foreign-task"
	if _, err := r.currentReviewStates(t.Context(), host, rows, repository, p); err == nil {
		t.Fatal("revision filtering hid foreign task evidence")
	}
}
