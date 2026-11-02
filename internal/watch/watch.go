// Package watch owns the instruction-free, in-process plan watch loop.
//
// It only derives state and schedules the caller's sweep. It never claims a
// task, opens a proposal, or writes tracker state.
package watch

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type AwaitKind string

const (
	AwaitOpenProposal      AwaitKind = "open-proposal"
	AwaitClaimedNoProposal AwaitKind = "claimed-no-proposal"
	AwaitReadyUnclaimed    AwaitKind = "ready-unclaimed"
)

type Task struct {
	ID        string
	Proposal  *port.ProposalRef
	Claimed   bool
	Closed    bool
	BlockedBy []string
	// Since is when this task entered its CURRENT wait — caller-supplied
	// (e.g. a proposal's opened-at, a claim ref's timestamp), never derived
	// here: the loop's own derivation holds no state (DINV-5) and the reader
	// port carries no per-proposal timestamp to fall back on. The zero value
	// means "not known" — the status verb renders that as an absent age
	// rather than fabricating one (status.go). The watch loop itself never
	// reads this field.
	Since time.Time
}

type Plan struct{ Tasks []Task }

type Await struct {
	TaskID string
	Kind   AwaitKind
	Ref    port.ProposalRef
	// LastActivity is the newest proposal comment (any surface, any author)
	// on an open-proposal await. It is part of the transition key, so a new
	// review comment wakes a sweep — the sweep's own pre-claim stage decides,
	// from durable evidence, whether that is a human-feedback round to run.
	LastActivity time.Time
}

type AwaitSet struct{ Awaits []Await }

func (s AwaitSet) Complete() bool { return len(s.Awaits) == 0 }

// key identifies the whole derived state — every await's task, kind and ref, in
// Derive's sorted order. It is the loop's TRANSITION edge: a change means
// durable reality moved (a proposal opened, a claim appeared, a task landed or
// closed), which is the wake the design delivers exactly once per distinct
// state (§3.1 step 3; DINV-3's coalescing).
//
// It cannot represent a transition Derive does not surface. Derive collapses a
// proposal's check and comment state into the single boolean "still open", so
// the checks-settled and new-comment edges §3.1 step 3 also lists are invisible
// here: a pending→green move leaves this key byte-identical. Detecting them
// needs that state carried onto the Await plus the reader's cursor (fetchStates
// always passes an empty one) — issue #966 owns it.
//
// On main those edges were serviced only incidentally, by an unconditional
// cadence force-sweep that ran whether or not anything had changed — the defect
// that restarted a task four times against its own open proposal (I8). Removing
// that force ends the incidental servicing too; #966 records what is therefore
// not delivered. It is not a safety gap: the executor's pre-claim preflight
// withholds every open proposal, so a missed wake cannot duplicate work.
func (s AwaitSet) key() string {
	var out string
	for _, a := range s.Awaits {
		out += a.TaskID + "\x00" + string(a.Kind) + "\x00" + a.Ref.String()
		if !a.LastActivity.IsZero() {
			// A new comment on an open proposal is a transition (the human
			// review feedback edge): it must move the key, or the sweep that
			// services it is never woken.
			out += "\x00" + a.LastActivity.UTC().Format(time.RFC3339Nano)
		}
		out += "\n"
	}
	return out
}

// readyKey identifies the ACTIONABLE frontier: the ready-but-unclaimed awaits
// alone, in Derive's sorted order. A ready-unclaimed await carries no proposal
// ref and one fixed kind, so the task IDs identify it completely.
//
// It is NOT the transition edge (that is key) — it answers a different
// question: is there work a sweep could actually start? Only that justifies
// re-invoking the executor on an UNCHANGED state, which is what DINV-9's
// cadence-paced stuck-frontier retry is for ("the ready-task trigger", §3.1
// step 1). Re-invoking on an unchanged state with nothing ready is the defect
// that restarted a task four times against its own open proposal: time passing
// is not a reason to sweep.
//
// The empty string means "nothing is actionable": the loop still wakes once on
// any genuine transition, but a cadence tick over unchanged state only polls,
// however long the non-actionable awaits persist.
func (s AwaitSet) readyKey() string {
	var out string
	for _, a := range s.Awaits {
		if a.Kind != AwaitReadyUnclaimed {
			continue
		}
		out += a.TaskID + "\x00"
	}
	return out
}

// Derive reads only proposal state needed by the plan. An unblocked,
// unclaimed task is immediately ready; a claimed task without a proposal is
// the worker-wait await. A proposal is open until its landing/checks settle.
//
// Derive is the loop's own coarse-grained view (three await kinds; a closed
// or DAG-blocked task carries no row at all). Statuses (status.go) is the
// status verb's finer-grained sibling over the SAME fetchStates/taskReady
// helpers — one state fetch and one readiness predicate feed both (AP-11;
// design:wake-watch-v2 §3.6 — "renders from the same derivation the loop
// uses").
func Derive(ctx context.Context, plan Plan, reader port.ProposalStateReader) (AwaitSet, error) {
	if reader == nil {
		return AwaitSet{}, fmt.Errorf("watch: proposal state reader is required")
	}
	states, err := fetchStates(ctx, plan, reader)
	if err != nil {
		return AwaitSet{}, err
	}
	var out []Await
	for _, t := range plan.Tasks {
		if t.Closed {
			continue
		}
		if t.Proposal == nil {
			if t.Claimed {
				out = append(out, Await{TaskID: t.ID, Kind: AwaitClaimedNoProposal})
			} else if taskReady(t, plan) {
				out = append(out, Await{TaskID: t.ID, Kind: AwaitReadyUnclaimed})
			}
			continue
		}
		st := states[*t.Proposal]
		if st.Unknown != nil || st.Landing != port.LandingLanded || !checksSettled(st) {
			out = append(out, Await{TaskID: t.ID, Kind: AwaitOpenProposal, Ref: *t.Proposal, LastActivity: latestComment(st)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskID != out[j].TaskID {
			return out[i].TaskID < out[j].TaskID
		}
		return out[i].Kind < out[j].Kind
	})
	return AwaitSet{Awaits: out}, nil
}

// latestComment is the newest comment time across a proposal's surfaces, or
// the zero time when it has none (or is unknown).
func latestComment(s port.ProposalState) time.Time {
	var latest time.Time
	for _, c := range s.Comments {
		if c.CreatedAt.After(latest) {
			latest = c.CreatedAt
		}
	}
	return latest
}

// fetchStates reads batched proposal state for every non-closed task's
// proposal — the plan's one read per derivation (AP-15), shared verbatim by
// Derive and Statuses so exactly one fetch shape exists.
func fetchStates(ctx context.Context, plan Plan, reader port.ProposalStateReader) (map[port.ProposalRef]port.ProposalState, error) {
	refs := make([]port.ProposalRef, 0)
	for _, t := range plan.Tasks {
		if t.Proposal != nil && !t.Closed {
			refs = append(refs, *t.Proposal)
		}
	}
	states := map[port.ProposalRef]port.ProposalState{}
	if len(refs) == 0 {
		return states, nil
	}
	got, err := reader.States(ctx, refs, port.Cursor{})
	if err != nil {
		return nil, fmt.Errorf("watch: derive proposal state: %w", err)
	}
	if len(got) != len(refs) {
		return nil, fmt.Errorf("watch: reader returned %d states for %d refs", len(got), len(refs))
	}
	for i, ref := range refs {
		states[ref] = got[i]
	}
	return states, nil
}

// taskReady reports whether t is unblocked: every task named in BlockedBy is
// itself closed. A task blocking on an unknown ID (not present in plan) is
// conservatively treated as still blocking — it can never resolve to ready
// from a dangling reference.
func taskReady(t Task, plan Plan) bool {
	for _, blocker := range t.BlockedBy {
		for _, b := range plan.Tasks {
			if b.ID == blocker && !b.Closed {
				return false
			}
		}
	}
	return true
}

// checksSettled reports whether every required context has both run and
// resolved — pending or required-but-missing means not yet settled. It says
// nothing about red vs green (hasRed distinguishes that for the status
// verb's finer classification); Derive only needs "still open" either way.
func checksSettled(s port.ProposalState) bool {
	return !hasPendingOrMissing(s)
}

// hasPendingOrMissing reports whether s still has a required context with no
// observed run, or an observed run that has not resolved.
func hasPendingOrMissing(s port.ProposalState) bool {
	if len(s.RequiredMissing) > 0 {
		return true
	}
	for _, c := range s.Checks {
		if c.State == port.CheckPending {
			return true
		}
	}
	return false
}

// hasRed reports whether s carries at least one failed required context.
func hasRed(s port.ProposalState) bool {
	for _, c := range s.Checks {
		if c.State == port.CheckRed {
			return true
		}
	}
	return false
}

type Summary struct {
	Awaits int
	Sweeps int
	// Polls counts derivations that deliberately invoked no sweep — the
	// frontier had nothing ready. A watch that is alive and correctly idle
	// accrues polls and no sweeps, which is what distinguishes it from a dead
	// checker (whose reads, and so whose wake_check_ran events, stop entirely).
	Polls        int
	ReviewSweeps int
	Completed    bool
}
type Reader interface {
	port.ProposalStateReader
	Cadence(context.Context) time.Duration
}
type Sweep func(context.Context) error
type Clock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}
type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Run watches until completion or cancellation. The first ready frontier is
// swept immediately. Repeated identical derived states are only swept at the
// reader's cadence, preventing a stuck frontier from hot-looping. It wires no
// push slot (see RunLive for that seam) — a convenience wrapper for static
// plans and the tests that don't exercise one.
func Run(ctx context.Context, plan Plan, reader Reader, sweep Sweep, clock Clock) (Summary, error) {
	return RunLive(ctx, constantSource(plan), reader, sweep, clock, nil)
}
