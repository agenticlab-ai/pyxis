package watch

// The status verb's derivation (design:wake-watch-v2 §3.6; plan self-drive-
// wake/001 T9): a read-only, one-pass render of what the factory is waiting
// on, for how long, and whether that wait has run past its configured
// threshold — plus the checker's own liveness, so a dead or unverifiable
// checker is loud rather than indistinguishable from a quiet repo (FR-12,
// UB-5). Statuses shares fetchStates/taskReady with Derive (watch.go) —
// exactly one state fetch and one readiness predicate feed both the loop and
// this verb (AP-11). Nothing in this file writes tracker or medium state; the
// only side effect is an attempted, best-effort append to the wake-events
// sink (BuildReport), the same local diagnostic trail the loop already
// writes to — never a "write" in the domain sense the verb's zero-writes
// invariant governs (mirrors DINV-4's own scoping for the watch verb).

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/wake"
)

// TaskState is one task's rendered state (design:wake-watch-v2 §3.6's state
// column: "landed ✓ / waiting-on-review / checks-running / checks-red /
// blocked / unknown-with-reason" — ClaimedNoProposal/ReadyUnclaimed are this
// package's necessary split of that list's implicit "waiting on the worker"
// case, matching the loop's own two claimed/ready await kinds and the
// config's separate "worker" threshold).
type TaskState string

const (
	StateLanded            TaskState = "landed"
	StateWaitingReview     TaskState = "waiting-on-review"
	StateChecksRunning     TaskState = "checks-running"
	StateChecksRed         TaskState = "checks-red"
	StateBlocked           TaskState = "blocked"
	StateClaimedNoProposal TaskState = "claimed-no-proposal"
	StateReadyUnclaimed    TaskState = "ready-unclaimed"
	StateUnknown           TaskState = "unknown"
)

// WaitKind selects which configured threshold (Thresholds.forKind) governs a
// task's wait — the design's wait-kind mapping (§3.6/§3.8): "waiting-on-
// review = open + checks green ...; checks-running/red from per-context
// states; blocked from the plan DAG" (blocked and landed carry no threshold
// at all — WaitNone).
type WaitKind string

const (
	WaitReview WaitKind = "review"
	WaitCI     WaitKind = "ci"
	WaitWorker WaitKind = "worker"
	WaitNone   WaitKind = ""
)

// TaskStatus is one row of the status verb's render.
type TaskStatus struct {
	TaskID string
	State  TaskState
	Wait   WaitKind

	// Since/Age/HasAge describe the wait's duration. HasAge is false when the
	// caller supplied no Task.Since — the render then shows an absent age
	// rather than fabricating one (never a healthy-looking zero, UB-2's
	// posture extended to missing data).
	Since  time.Time
	Age    time.Duration
	HasAge bool

	// Stale is the per-task too-long flag: HasAge && the age exceeds Wait's
	// configured threshold (design:wake-watch-v2 §3.6) — EXCEPT for an
	// unknown-offline row, where it is forced true whenever HasAge (UB-1:
	// "status rows `unknown (offline, retrying)` + stale-marked ages" —
	// never fabricate the *state*, but never hide a known age either).
	Stale bool

	// UnknownReason is non-empty exactly when State is StateUnknown — the
	// reader's typed reason verbatim (never omitted, UB-2).
	UnknownReason string
}

func (row *TaskStatus) setAge(since, now time.Time) {
	if since.IsZero() {
		return
	}
	row.Since = since
	row.Age = now.Sub(since)
	row.HasAge = true
}

// Statuses derives one TaskStatus per plan task — landed tasks included, so
// the render shows the whole DAG's progress, not only what remains pending —
// from the SAME state fetch Derive uses (AP-11). Rows are returned sorted by
// TaskID for a stable, diffable render.
func Statuses(ctx context.Context, plan Plan, reader port.ProposalStateReader, thresholds Thresholds, now time.Time) ([]TaskStatus, error) {
	if reader == nil {
		return nil, fmt.Errorf("watch: proposal state reader is required")
	}
	states, err := fetchStates(ctx, plan, reader)
	if err != nil {
		return nil, err
	}
	out := make([]TaskStatus, 0, len(plan.Tasks))
	for _, t := range plan.Tasks {
		out = append(out, statusFor(t, plan, states, thresholds, now))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TaskID < out[j].TaskID })
	return out, nil
}

func statusFor(t Task, plan Plan, states map[port.ProposalRef]port.ProposalState, thresholds Thresholds, now time.Time) TaskStatus {
	row := TaskStatus{TaskID: t.ID}
	switch {
	case t.Closed:
		row.State = StateLanded
		return row

	case t.Proposal != nil:
		st := states[*t.Proposal]
		if st.Unknown != nil {
			row.State = StateUnknown
			row.UnknownReason = st.Unknown.String()
			row.setAge(t.Since, now)
			if *st.Unknown == port.UnknownOffline {
				row.Stale = row.HasAge
			}
			return row
		}
		switch {
		case hasRed(st):
			row.State, row.Wait = StateChecksRed, WaitCI
		case hasPendingOrMissing(st):
			row.State, row.Wait = StateChecksRunning, WaitCI
		default:
			row.State, row.Wait = StateWaitingReview, WaitReview
		}

	case t.Claimed:
		row.State, row.Wait = StateClaimedNoProposal, WaitWorker

	case taskReady(t, plan):
		row.State, row.Wait = StateReadyUnclaimed, WaitWorker

	default:
		row.State = StateBlocked
		return row
	}
	row.setAge(t.Since, now)
	row.Stale = row.HasAge && wake.Stale(row.Age, thresholds.forKind(row.Wait))
	return row
}

// Liveness is the checker's own footer (design:wake-watch-v2 §3.6): the
// last-check age read from the wake-events sink (its liveness surface,
// DINV-8), the display flag at one cadence, and the higher, configurable
// defect-event bound at the configured wake.stale.checker (default 3x
// cadence) — two thresholds, one shared predicate (DINV-7).
type Liveness struct {
	// Defect is non-empty exactly when the sink cannot answer at all — an
	// absent or unwritable events file (Sink.Health's own cannot-verify-
	// liveness reasons). LastCheck/Age/DisplayStale/BoundStale are all zero
	// in that case: the footer names the defect, never a healthy-looking age
	// standing in for one that could not be read (DINV-8).
	Defect       string
	LastCheck    time.Time
	Age          time.Duration
	Cadence      time.Duration
	Bound        time.Duration
	DisplayStale bool
	BoundStale   bool
}

func liveness(health Health, cadence, bound time.Duration, now time.Time) Liveness {
	if health.Defect != "" {
		return Liveness{Defect: health.Defect, Cadence: cadence, Bound: bound}
	}
	age := now.Sub(health.LastCheck)
	return Liveness{
		LastCheck:    health.LastCheck,
		Age:          age,
		Cadence:      cadence,
		Bound:        bound,
		DisplayStale: wake.Stale(age, cadence),
		BoundStale:   wake.Stale(age, bound),
	}
}

// Report is the status verb's whole one-pass render.
type Report struct {
	Tasks         []TaskStatus
	Reviews       []ReviewWakeRow
	ReviewUnknown *ReviewUnknown
	Liveness      Liveness
}

// Degraded reports whether anything in the report warrants the operator's
// attention: any stale or unknown task row, the checker's display flag, or
// an unverifiable liveness footer. A Report with Degraded()==false is the
// "rendered clean" case; Degraded()==true is "rendered-stale" — both render
// the full table, distinguished only by exit status (§3.6).
func (r Report) Degraded() bool {
	if r.Liveness.Defect != "" || r.Liveness.DisplayStale || r.ReviewUnknown != nil || len(r.Reviews) > 0 {
		return true
	}
	for _, t := range r.Tasks {
		if t.Stale || t.State == StateUnknown {
			return true
		}
	}
	return false
}

// BuildCombinedReport adds a pure configured-review snapshot to BuildReport.
// SnapshotStatus never initializes or appends consumer recovery state.
func BuildCombinedReport(ctx context.Context, source PlanSource, reader port.ProposalStateReader, reviews ReviewStateReader, scope string, sink *Sink, thresholds Thresholds, cadence time.Duration, now time.Time) (Report, error) {
	report, err := BuildReport(ctx, source, reader, sink, thresholds, cadence, now)
	if err != nil {
		return Report{}, err
	}
	state, err := reviews.SnapshotStatus(ctx, scope)
	if err != nil {
		return Report{}, err
	}
	if state.Status == ReviewStateUnknown {
		report.ReviewUnknown = state.Unknown
		return report, nil
	}
	report.Reviews = state.Rows
	return report, nil
}

// BuildReport reads one Plan from source (the same durable-facts-each-call
// shape RunLive ticks on — DINV-5: no state survives the process) and
// derives the full report — task rows plus the checker's liveness footer —
// attempting one EventStaleFlagged append per stale finding
// (design:prd:self-drive-wake EVT-5: "the staleness predicate — one shared
// implementation, rendered by the status verb and emitted with wake-channel
// events"). The append is best-effort and never blocks or fails the render
// (DINV-8; Sink.Append already swallows its own failures). sink must be
// non-nil — the caller always owns a real sink, exactly as the watch verb
// does. A static Plan uses constantSource, mirroring Run/RunLive's own split.
func BuildReport(ctx context.Context, source PlanSource, reader port.ProposalStateReader, sink *Sink, thresholds Thresholds, cadence time.Duration, now time.Time) (Report, error) {
	plan, err := source(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("watch: status: plan source: %w", err)
	}
	statuses, err := Statuses(ctx, plan, reader, thresholds, now)
	if err != nil {
		return Report{}, err
	}
	live := liveness(sink.Health(), cadence, thresholds.CheckerBound(cadence), now)
	for _, s := range statuses {
		if !s.Stale {
			continue
		}
		sink.Append(Event{
			Kind: EventStaleFlagged, Subject: "await", TaskID: s.TaskID,
			AgeS: int(s.Age.Seconds()), BoundS: int(thresholds.forKind(s.Wait).Seconds()),
		})
	}
	if live.BoundStale {
		sink.Append(Event{
			Kind: EventStaleFlagged, Subject: "checker",
			AgeS: int(live.Age.Seconds()), BoundS: int(live.Bound.Seconds()),
		})
	}
	return Report{Tasks: statuses, Liveness: live}, nil
}
