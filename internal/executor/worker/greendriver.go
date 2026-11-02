package worker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// GreenDriver settles a pending proposal without inference and drives an
// explicitly-red proposal toward green through the bounded CI self-heal loop
// (drivetogreen, §3.6). It is an OPTIONAL, consumer-side seam: the Worker
// declares what it needs, an implementation is injected via WithGreenDriver,
// and a Worker built without one keeps the base verify unchanged (a non-green
// proposal stays failed, exactly as before). Defining it here — not in the
// frozen port set — keeps this additive capability outside the five ports.
type GreenDriver interface {
	// Drive waits through pending check states with zero fixer attempts, then, if
	// the settled state is red, attempts to bring the proposal to green within
	// its configured attempt cap (DINV-9, AP-16). It is called only after the
	// Worker has verified the proposal is not green and re-reads that state
	// exactly before deciding whether inference is warranted.
	//
	// A nil error with Result.Green == true means the proposal reached green. A
	// non-nil error means the drive did not converge: either the attempt bound was
	// exhausted (an escalation — the task is failed and must be surfaced) or an
	// operational failure occurred. The Worker classifies any non-green drive as
	// failed (fail-closed, DINV-3) and carries the error as the escalation/context.
	Drive(ctx context.Context, task port.Task, work port.Workdir, proposal port.Proposal) (GreenDriveResult, error)

	// GateGreen reports whether work passes the local CI-mirroring gate right now,
	// WITHOUT healing — the same deterministic checks CI requires, run over the
	// workdir (DINV-4). It is the Worker's pre-publish predicate for a
	// bound-truncated run: committed WIP that passes is shippable (the agent's
	// turns ran out, not the work), so the Worker publishes it rather than
	// respawning; WIP that fails respawns from the committed base instead of
	// opening a churny PR from partial work. A non-nil error is an operational
	// gate failure (a check that could not be run — never a check legitimately
	// failing), which the Worker treats as fail-closed. Reusing the drive loop's
	// own gate keeps "local green" one predicate (DINV-4), never a second copy.
	GateGreen(ctx context.Context, work port.Workdir) (bool, error)

	// DriveLocal is the bounded LOCAL convergence loop the Worker runs BEFORE
	// anything is published: evaluate the CI-mirroring gate over the workdir;
	// when it is red, hand the gate's own (redacted) output to a fresh fixer
	// process and re-evaluate, to the configured attempt cap — never pushing,
	// never touching the code host (DINV-4, prd:implement-now/FR-8, UB-5). A nil
	// error with Result.Green == true means the tree passes the local gate and
	// is fit to review and publish. A non-nil error means the tree did not
	// converge locally: either the cap was exhausted (an escalation — the task
	// fails closed with its work preserved and nothing pushed) or an operational
	// failure occurred (a check that could not run).
	DriveLocal(ctx context.Context, task port.Task, work port.Workdir) (LocalDriveResult, error)
}

// UsageObservableGreenDriver is the optional consumer-side accounting seam.
// A driver reports each completed or interrupted fixer result after quiescence
// so the Worker can include its observed usage in the task total.
type UsageObservableGreenDriver interface {
	BindUsageObserver(func(port.RunResult))
}

// LocalDriveResult is the terminal, observable result of one local convergence
// drive: whether the gate went green and how many fixer attempts it took.
type LocalDriveResult struct {
	// Green is true when the local gate passed within the bound.
	Green bool
	// Attempts is how many local fixer attempts were consumed (observable).
	Attempts int
	// Cap is the configured attempt bound the loop honored (observable).
	Cap int
}

// GreenDriveResult is the terminal, observable result of one green-drive. It
// carries the bounded, observable attempt count the self-heal contract requires
// (prd:implement-now/FR-9 — "the attempt count is bounded and observable").
type GreenDriveResult struct {
	// Green is true when the proposal reached green within the bound — the only
	// state that lets the Worker count the task succeeded (DINV-3).
	Green bool
	// Attempts is how many self-heal attempts were consumed (observable).
	Attempts int
	// Cap is the configured attempt bound the loop honored (observable). When
	// Green is false and Attempts == Cap, the bound was exhausted and the task
	// escalates (DINV-9).
	Cap int
}

// Option configures a Worker at construction. Options are additive: New with no
// option builds the base Worker, so every existing caller compiles unchanged.
type Option func(*Worker)

// WithGreenDriver installs a GreenDriver so the Worker settles pending CI and
// drives a red proposal to green before terminal classification
// (prd:implement-now/FR-7, FR-9). Without this option the Worker's base verify is
// unchanged — a non-green proposal is classified failed as before. A nil driver
// is ignored (the base behavior), so the option never silently disables the base
// path.
func WithGreenDriver(d GreenDriver) Option {
	return func(w *Worker) {
		if d != nil {
			w.greenDriver = d
			if accounting, ok := d.(UsageObservableGreenDriver); ok {
				accounting.BindUsageObserver(w.addUsage)
			}
		}
	}
}
