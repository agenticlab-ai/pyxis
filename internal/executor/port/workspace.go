package port

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
)

// Workspace is the port onto per-task isolated working directories. A production
// adapter creates a git worktree per task, branched off freshly-fetched
// mainline, mechanically forbidding any mutation of the parent checkout
// (prd:implement-now/FR-4, DINV-1, DINV-2); the hermetic tests use a temp-dir
// fake.
type Workspace interface {
	// Prepare provisions an isolated working directory for the task, branched off
	// freshly-fetched mainline (never an unmerged blocker's branch — DINV-2). The
	// returned Workdir is the Worker's only writable surface.
	Prepare(ctx context.Context, task Task) (Workdir, error)
}

// Workdir is one task's isolated working directory.
type Workdir interface {
	// Dir is the absolute path to the working directory — the workdir handed to
	// the agent runtime and read by the code host when opening a proposal.
	Dir() string

	// Branch is the worktree's branch name — the head a proposal is opened from.
	Branch() string

	// Checkpoint commits any uncommitted changes in the working directory,
	// without publishing anything, and reports whether the branch carries any work
	// beyond the base it was cut from. The Go supervisor calls this after the
	// agent and its children stop, before checking or publishing a revision, and
	// when preserving interrupted WIP. It is the sole commit owner. A clean tree
	// is a no-op; hasWork is not a verified-success predicate.
	Checkpoint(ctx context.Context) (hasWork bool, err error)

	// Push publishes the worktree's branch to its remote so a proposal can be
	// opened from it — the deterministic publish step after Checkpoint
	// (prd:implement-now/FR-10, §3.1). It must reject uncheckpointed changes rather
	// than commit during publication. It reports
	// whether anything was published: a worktree whose HEAD is no further along
	// than the base it was branched from has nothing to propose, so Push makes no
	// remote change and returns published=false (the Worker maps that to a hollow
	// outcome — DINV-3), avoiding a junk remote branch and a remote round-trip.
	// It never merges (DINV-6): it only publishes the head so a proposal can open.
	Push(ctx context.Context) (published bool, err error)

	// ChangeSize measures the committed change on the branch against the
	// mainline it was cut from or last integrated: the Lines of code figure of
	// the proposal's description (internal/changesize).
	ChangeSize(ctx context.Context) (changesize.Size, error)

	// Cleanup removes the working directory. Reaping is the janitor's job (on
	// merged-and-closed or age); a Worker does not reap its own workdir, since
	// the proposal's branch must outlive the run until it lands.
	Cleanup(ctx context.Context) error
}
