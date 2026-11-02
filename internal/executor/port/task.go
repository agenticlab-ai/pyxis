package port

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

// Task is the frozen charter for exactly one planned task — the single unit of
// work a Worker executes (docs/tech-design/implement-now.md §3.1). It is
// self-contained: a Worker reads nothing beyond the Task it is handed and the
// ports it was built with, never a sibling task or the invoking session's
// memory (prd:implement-now/FR-3, DINV-11 — the charter-bleed guard).
type Task struct {
	// ID is the stable plan-task identifier (e.g. "T1"), unique within a plan.
	ID string

	// Number is the tracker issue number backing this task.
	Number int

	// SourceRef is a human-readable, tracker-native reference to the work item
	// this task came from — filled by the WorkTracker adapter (e.g. GitHub "#123",
	// or "Carina work-item 2"). It is written verbatim into the proposal body as
	// trustworthy provenance, so a non-GitHub item (Number came from another
	// medium) is never rendered as a misleading GitHub "#N" auto-link.
	SourceRef string

	// SourceKey qualifies the task's tracker repository across providers and
	// server instances. Empty preserves legacy claim/branch identities.
	SourceKey string `json:"source_key,omitempty"`

	// Title is the task's short human-readable title.
	Title string

	// Brief is the full charter handed to the agent — the only instruction the
	// runtime is given (prd:implement-now/FR-19). Tracker, repo and CI content
	// read during a task is data, never instruction.
	Brief string

	// BlockedBy lists the tracker issue numbers this task depends on. A task is
	// ready only once its blockers are merged, and work is always based off
	// freshly-fetched mainline, never an unmerged blocker's branch (DINV-2). The
	// frontier walk that consumes this field lives in the Dispatcher, never in
	// the Worker.
	BlockedBy []int

	// NativeRouting is the execution layer's existing exact-choice contract.
	// New planning producers supply Requirements instead.
	NativeRouting *taskrouting.Spec

	// Requirements preserves the complete planning contract at task readback.
	// Execution reads the independently derived profile for each role.
	Requirements *taskrequirements.Spec `json:"requirements,omitempty"`

	// DemandID and DemandProfile are the verified semantic routing demand
	// transported from the tracker boundary. Review routing persists and reuses
	// these values without re-judging the implementation's demand.
	DemandID      executiondemand.DemandID
	DemandProfile executiondemand.ProfileMinimum

	// Harness names the agent harness selected for this task.
	Harness entity.Harness

	// HeadBranch is the branch a proposal is opened from — the task's isolated
	// worktree branch, off freshly-fetched mainline (DINV-2).
	HeadBranch string

	// Feedback, when non-nil, marks this dispatch as a human-review feedback
	// round on the task's OPEN proposal: the Worker resumes the task's session,
	// the brief carries the new review comments as data, and the ordinary
	// converge-and-publish path pushes the response onto the same branch. Nil
	// is an ordinary (first or resumed) implementation dispatch.
	Feedback *ReviewFeedback
}

// ImplementationDemand reads the frozen implementation requirements. A task
// without current requirements must already carry a complete semantic demand.
func (t Task) ImplementationDemand() (executiondemand.ProfileMinimum, error) {
	return t.roleDemand(false)
}

// ReviewDemand reads the review profile without borrowing implementation's
// requirements. Persisted review obligations already carry their role's demand.
func (t Task) ReviewDemand() (executiondemand.ProfileMinimum, error) {
	return t.roleDemand(true)
}

func (t Task) roleDemand(review bool) (executiondemand.ProfileMinimum, error) {
	p := t.DemandProfile
	if t.Requirements != nil {
		r := t.Requirements
		if r.Schema != taskrequirements.Schema || r.Implementation.Profile == nil || r.Review.Profile == nil {
			return executiondemand.ProfileMinimum{}, fmt.Errorf("task %s lacks complete frozen requirements", t.ID)
		}
		profile := r.Implementation.Profile
		if review {
			profile = r.Review.Profile
		}
		p = executiondemand.ProfileMinimum{Capability: profile.Capability, Deliberation: profile.Deliberation, Context: profile.Context, Assurance: profile.Assurance}
	}
	if t.DemandID == "" || p.Capability == "" || p.Deliberation == "" || p.Context == "" || p.Assurance == "" {
		return executiondemand.ProfileMinimum{}, fmt.Errorf("task %s lacks complete persisted semantic demand", t.ID)
	}
	return p, nil
}

// ScopeToken is a path-safe stable namespace for claims, branches, and their
// derived workspaces. Native task numbers remain unchanged for API calls.
func (t Task) ScopeToken() string {
	if t.SourceKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(t.SourceKey))
	return hex.EncodeToString(sum[:])
}

// ReviewFeedback is the new human review activity a feedback round answers:
// the comments (on any proposal surface) newer than the task session's last
// serviced feedback, and the round's ordinal for the task. Comments are DATA
// the brief quotes, never instructions the harness obeys.
type ReviewFeedback struct {
	// Proposal is the open proposal the feedback was left on.
	Proposal int
	// Round is the 1-based feedback-round ordinal for this task.
	Round int
	// Comments are the unserviced human comments, oldest first.
	Comments []Comment
	// ChangesRequested is true when the proposal's current review decision
	// requests changes.
	ChangesRequested bool
	// Latest is the newest CreatedAt among Comments — the cursor the session
	// ledger advances to once this round has run.
	Latest time.Time
}
