package port

import (
	"context"
	"fmt"
	"time"
)

// ProposalStateReader is the port onto a medium's batched proposal-state read
// (docs/tech-design/state-reader-v2.md §3.1). It is the single seam a
// wake/watch consumer polls on a cadence to learn what changed — landing,
// per-context checks, and new comments — without ever inventing its own
// medium client (AP-1). Every implementation is bound by the conformance kit
// in this package (CheckReaderConformance): a new medium adapter passes the
// kit or it does not exist (AP-3).
type ProposalStateReader interface {
	// States returns one ProposalState per requested ref — always all of
	// them, in the same order as refs — never omitted, never a
	// healthy-looking zero value standing in for a failed read (DINV-1). A
	// proposal the medium cannot answer for in this batch comes back with
	// Unknown set to a typed reason; every other returned state carries a
	// genuine landing/checks/comments read. since filters the returned
	// Comments only: a surface absent from since.Positions has no lower bound
	// (every comment on that surface is returned).
	//
	// The returned error is transport-fatal only — context cancellation, or a
	// failure that prevented the call from producing any per-proposal answer
	// at all (e.g. an unreadable local budget file). A partial or
	// per-proposal medium failure is never surfaced here: it resolves to that
	// proposal's Unknown instead (DINV-2) — the error return never smuggles
	// state.
	States(ctx context.Context, refs []ProposalRef, since Cursor) ([]ProposalState, error)
}

// ProposalRef is the medium-neutral identity of one proposal: a number scoped
// to a repo. It carries only what a reader needs to ask "what is this
// proposal's state" — narrower than Proposal (which also carries the branch a
// proposal was opened from, needed only at open time).
type ProposalRef struct {
	// Repo is the medium-scoped repository the proposal belongs to (e.g. a
	// GitHub "owner/name" slug, or a Carina store root) — the same scope the
	// adapter answering this ref is already bound to.
	Repo string

	// Number is the proposal number within Repo.
	Number int
}

// String renders a ProposalRef for logs and error messages ("owner/name#12").
func (r ProposalRef) String() string {
	if r.Repo == "" {
		return fmt.Sprintf("#%d", r.Number)
	}
	return fmt.Sprintf("%s#%d", r.Repo, r.Number)
}

// Landing is a proposal's landing state as the medium reports it — a claim,
// never an assertion of mainline ancestry (DINV-3, AP-26): a consumer that
// needs to know whether a landed revision actually reached mainline verifies
// that itself, from LandedRevision, with no further read.
type Landing int

const (
	// LandingUnknown is the zero value: a Landing that was never set, or a
	// proposal the medium could not answer for, reads as unknown rather than
	// silently as open — fail-closed, mirroring CheckPending's role as
	// CodeHost's cautious zero value. Every ProposalState with a non-nil
	// Unknown carries this value (DINV-1, DINV-2).
	LandingUnknown Landing = iota
	// LandingOpen means the proposal is still open on the medium.
	LandingOpen
	// LandingLanded means the medium reports the proposal landed (merged /
	// accepted). LandedRevision on the same ProposalState carries the
	// medium-reported landed revision id — the claim a consumer verifies
	// ancestry from (DINV-3).
	LandingLanded
	// LandingClosedUnlanded means the medium reports the proposal closed
	// without landing (e.g. closed unmerged).
	LandingClosedUnlanded
)

// String renders the landing state for logs and test output.
func (l Landing) String() string {
	switch l {
	case LandingUnknown:
		return "unknown"
	case LandingOpen:
		return "open"
	case LandingLanded:
		return "landed"
	case LandingClosedUnlanded:
		return "closed-unlanded"
	default:
		return fmt.Sprintf("Landing(%d)", int(l))
	}
}

// CheckContext is one required-or-observed check's state, keyed by its
// medium-native context name (a GitHub check-run/status name, or a Carina
// verdict field name). State reuses CheckState — the same fail-closed
// pending/green/red the aggregate CodeHost.Checks reports — so exactly one
// enum expresses "is this green" end to end (AP-11): the aggregate is a
// reduction over these per-context states, never a second derivation.
type CheckContext struct {
	Name  string
	State CheckState
}

// CommentSurface names which of a medium's proposal-discussion surfaces a
// Comment came from (state-reader-v2.md §3.1), mapped per medium in the
// adapter (§3.6/§3.7).
type CommentSurface int

const (
	// SurfaceDiscussion is a top-level proposal discussion comment.
	SurfaceDiscussion CommentSurface = iota
	// SurfaceInlineReview is a comment attached to a specific reviewed line.
	SurfaceInlineReview
	// SurfaceReviewSummary is a review's own summary comment.
	SurfaceReviewSummary
)

// String renders the comment surface for logs and test output.
func (s CommentSurface) String() string {
	switch s {
	case SurfaceDiscussion:
		return "discussion"
	case SurfaceInlineReview:
		return "inline-review"
	case SurfaceReviewSummary:
		return "review-summary"
	default:
		return fmt.Sprintf("CommentSurface(%d)", int(s))
	}
}

// Comment is one comment on a proposal, from any of its discussion surfaces.
type Comment struct {
	// Untrusted forbids imported or otherwise unattributable content from
	// gaining execution authority through an operator login allowlist.
	Untrusted bool
	// NativeID is an opaque, host-qualified identity for equal-timestamp
	// pagination. Empty preserves the legacy timestamp-only reader contract.
	NativeID string
	Author   string
	// AuthorAlias is an alternate login for the same authenticated host actor.
	// Adapters derive it only from host actor metadata, never comment bodies
	// or imported author claims. Untrusted forbids using it for admission.
	AuthorAlias string `json:"author_alias,omitempty"`
	// AuthorAssociation is the medium's own author-trust classification for
	// the comment's author relative to the repository (GitHub: OWNER, MEMBER,
	// COLLABORATOR, CONTRIBUTOR, NONE, ...). Consumers that act on comments
	// (feedback rounds) gate on it; empty means the medium reported none and
	// is treated as untrusted.
	AuthorAssociation string
	CreatedAt         time.Time
	Surface           CommentSurface
	Body              string
	// ReviewState is the medium's review verdict when the comment is a review
	// summary (SurfaceReviewSummary): e.g. GitHub's APPROVED,
	// CHANGES_REQUESTED, COMMENTED, DISMISSED. Empty on other surfaces or when
	// the medium reports none.
	ReviewState string
}

// Cursor is the caller-held position States's since parameter uses to exclude
// already-seen comments: on each surface, only comments strictly newer than
// the held position are returned. The reader is stateless (DINV-7) — it never
// stores a Cursor itself. A caller derives its next position per surface from
// the newest CreatedAt among the Comments it has consumed and holds it
// between calls; a surface absent from Positions has no lower bound (every
// comment on that surface is returned).
type Cursor struct {
	Positions map[CommentSurface]time.Time
	// NativeIDs records consumed identities at the inclusive timestamp boundary.
	NativeIDs map[string]bool
}

// UnknownReason is why a proposal's state could not be read this batch — a
// first-class, typed result, never an omission or a default (DINV-1, DINV-2).
type UnknownReason int

const (
	_ UnknownReason = iota // reserved: never a populated Unknown's value (DINV-1)

	// UnknownRateLimited means the medium reported (or the shared write
	// budget inferred) a rate limit; the next scheduled call is the retry —
	// never an in-adapter retry loop.
	UnknownRateLimited
	// UnknownOffline means the medium could not be reached at all (network,
	// DNS, transport failure).
	UnknownOffline
	// UnknownParseDefect means the medium answered but its response did not
	// parse into the expected shape — never presented as an empty-healthy
	// result (the swallowed-evidence class).
	UnknownParseDefect
	// UnknownTimeout means the call did not complete within its bound.
	UnknownTimeout
)

// String renders the unknown reason for logs and test output.
func (r UnknownReason) String() string {
	switch r {
	case UnknownRateLimited:
		return "rate-limited"
	case UnknownOffline:
		return "offline"
	case UnknownParseDefect:
		return "parse-defect"
	case UnknownTimeout:
		return "timeout"
	default:
		return fmt.Sprintf("UnknownReason(%d)", int(r))
	}
}

// ProposalState is one proposal's batched read result — a closed struct
// (AP-6): every read either yields a genuine Landing/Checks/Comments answer,
// or carries a non-nil Unknown and nothing else (DINV-1, DINV-2). It never
// mixes the two: a proposal marked Unknown carries LandingUnknown, no checks
// and no comments, so a consumer branching only on Landing never mistakes a
// failed read for a healthy one.
type ProposalState struct {
	// Landing is the medium's claimed landing state (DINV-3). It is
	// LandingUnknown exactly when Unknown is non-nil.
	Landing Landing

	// LandedRevision is the medium-reported landed commit/record id.
	// Non-empty only when Landing is LandingLanded (DINV-3) — the claim a
	// consumer verifies mainline ancestry from, with no further read.
	LandedRevision string

	// Checks is the per-context check state, compared against the medium's
	// required set by the caller (or the adapter's own delegated aggregate —
	// §3.1). Empty when Unknown is non-nil.
	Checks []CheckContext

	// RequiredMissing names required contexts the medium's required set
	// declares but no run was observed for — a required-but-silent context
	// counts as not-green (DINV-4), never a vacuous pass.
	RequiredMissing []string

	// Comments are the surface-tagged comments newer than the caller's Cursor
	// (empty when Unknown is non-nil).
	Comments []Comment

	// ReviewDecision is the medium's aggregate review decision for the
	// proposal, when it reports one (GitHub: APPROVED, CHANGES_REQUESTED,
	// REVIEW_REQUIRED). Empty when unknown or not modelled by the medium.
	ReviewDecision string

	// Unknown is non-nil exactly when the medium could not answer for this
	// proposal this batch (DINV-1, DINV-2); the top-level States error is
	// reserved for transport-fatal failures that prevent any per-proposal
	// answer at all.
	Unknown *UnknownReason
}

// Review-decision / review-state vocabulary shared by the media that report
// one (GitHub's grammar; other media map onto it or leave the field empty).
const (
	ReviewChangesRequested = "CHANGES_REQUESTED"
	ReviewApproved         = "APPROVED"
)
