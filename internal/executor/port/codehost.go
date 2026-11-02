package port

import (
	"context"
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

// ErrNoProposal is returned by CodeHost.OpenProposal when a clean agent run left
// nothing to propose — no commits or diff on the head branch. It is not a
// failure: the Worker maps it to a hollow outcome, because a clean exit with no
// artifact is hollow, never succeeded (DINV-3). Callers match it with errors.Is.
var ErrNoProposal = errors.New("port: nothing to propose")

// CodeHost is the port onto the code host's proposal and check surface. A
// production adapter opens a pull request from the task's head branch and reads
// its required-check states through the checks API — an exact, fresh read, never
// a status rollup (AP-13, AP-15). The executor never merges (DINV-6): it opens
// proposals, keeps their descriptions current, and reads their state, nothing
// more.
type CodeHost interface {
	// OpenProposal opens a proposal from the task's head branch — the work in the
	// given Workdir — described by the pull request description template filled
	// for that revision (internal/prdescription). An already open proposal for
	// the head is returned as it is. It returns ErrNoProposal when the run
	// produced nothing to propose, which the Worker classifies as hollow
	// (DINV-3).
	OpenProposal(ctx context.Context, task Task, work Workdir, description prdescription.Description) (Proposal, error)

	// DescribeProposal replaces the description of the proposal open from the
	// task's head branch, keeping the rest of its body, and reports whether one
	// is open. The Worker calls it before it pushes a revision onto an open
	// proposal, so the checks that run on that revision read its description.
	DescribeProposal(ctx context.Context, task Task, work Workdir, description prdescription.Description) (bool, error)

	// Checks reads the aggregate required-check state of a proposal by exact,
	// fresh read.
	Checks(ctx context.Context, proposal Proposal) (CheckState, error)
}

// ProposalProbe answers, before any claim, which of a sweep's candidate tasks
// already carry an open proposal — and whether that proposal is red.
//
// It exists because the claim lease cannot fill this role. A Worker releases
// its claim on every terminal path, success included, while the task's tracker
// item stays open until its proposal merges. In that window the task reads as
// open ∧ unblocked ∧ unclaimed — indistinguishable, to a frontier walk, from
// work never started — so any scheduler that invokes a sweep then will start a
// second worker on already-proposed work. This port is the executor's own
// evidence that it must not (AP-17: a dispatch act requires positive evidence).
//
// The port takes the whole candidate set in one call per sweep, so an adapter
// is free to answer from a single listing where its medium allows; the caller
// never issues a per-task port round-trip.
type ProposalProbe interface {
	// OpenProposals reports the open proposal for each given task that has one,
	// keyed by Task.ID. A task absent from the map has no open proposal. A
	// landed or closed proposal is never reported.
	OpenProposals(ctx context.Context, tasks []Task) (map[string]OpenProposal, error)
}

// OpenProposal is one probed proposal. It carries the reference and nothing
// else — deliberately no check state: existence alone decides, because check
// colour is not recovery evidence. A red proposal is as likely a Worker that
// exhausted its bounded self-heal as one that died, and resuming either on
// colour alone retries without bound across sweeps. Resuming an open proposal
// belongs to an explicit recovery authorization, not to this probe.
type OpenProposal struct {
	// Number is the proposal (pull request) number.
	Number int
}

// Proposal identifies an opened proposal (a pull request).
type Proposal struct {
	// Number is the proposal (pull request) number.
	Number int

	// Branch is the head branch the proposal was opened from.
	Branch string
}

// CheckState is the aggregate state of a proposal's required checks.
type CheckState int

const (
	// CheckPending is the zero value: required checks have not resolved yet.
	// Anything other than CheckGreen is treated as not-green (fail-closed).
	CheckPending CheckState = iota
	// CheckGreen means every required check passed — the only state that admits a
	// succeeded outcome (DINV-3, AP-13).
	CheckGreen
	// CheckRed means at least one required check failed.
	CheckRed
)

// String renders the check state for logs and test output.
func (s CheckState) String() string {
	switch s {
	case CheckPending:
		return "pending"
	case CheckGreen:
		return "green"
	case CheckRed:
		return "red"
	default:
		return fmt.Sprintf("CheckState(%d)", int(s))
	}
}
