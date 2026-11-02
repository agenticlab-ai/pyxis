package dispatch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

// This file is the trusted-review feedback edge of task session continuity: a
// task whose proposal is open is normally withheld from the frontier (its work
// is proposed; restarting it would only push more commits — withholdProposed),
// but when an authorized reviewer has reviewed it since the task's session last
// answered, the sweep admits one feedback round: the Worker resumes the task's
// own session, the brief carries the new comments as data, and the ordinary
// converge-and-publish path pushes the response onto the same branch.
//
// Human rounds are bounded by human actions, not by a counter: every new
// comment (any surface) or a changes-requested review wakes exactly one round,
// because the durable cursor (FeedbackLedger) advances at admission — a
// failed round is a visible failed terminal and the human's next comment is
// the retry. The engine's own comments (its account, its review artifacts,
// its progress notes) are never feedback.

// FeedbackLedger is the durable per-task human-feedback cursor
// (tasksession.Continuity implements it): the newest proposal comment a round
// already serviced and the rounds so far.
type FeedbackLedger interface {
	LastFeedback(ctx context.Context, task port.Task) (time.Time, int, error)
	RecordFeedback(ctx context.Context, task port.Task, latest time.Time) error
}

type nativeFeedbackLedger interface {
	FeedbackSeen(context.Context, port.Task) (map[string]bool, error)
	RecordFeedbackComments(context.Context, port.Task, time.Time, []port.Comment) error
}

// feedbackDetection is the installed detector: the proposal-state reader, the
// repository the refs are scoped to, the engine's own code-host login, and the
// cursor ledger.
type feedbackDetection struct {
	reader  port.ProposalStateReader
	repo    string
	login   string
	ledger  FeedbackLedger
	trusted map[string]bool
}

// trustedAssociations are the medium author-trust classes whose comments may
// wake a feedback round (GitHub semantics: repository owner, org member,
// invited collaborator). Everything else — CONTRIBUTOR, FIRST_TIMER, NONE,
// or an empty association from a medium that reports none — is untrusted:
// an arbitrary commenter or bot must not be able to launch privileged agent
// runs or inject prompt content (feedback is quoted into the brief).
var trustedAssociations = map[string]bool{"OWNER": true, "MEMBER": true, "COLLABORATOR": true}

// WithFeedbackDetection installs trusted-review feedback rounds. reader answers
// proposal state (comments on all surfaces, the review decision); repo scopes
// the proposal refs; engineLogin is the code-host account the executor itself
// writes as (its comments are never feedback); ledger is the durable cursor;
// trustedLogins are additional code-host accounts whose comments are accepted
// even without a trusted author association (an operator allowlist). The host's
// authenticated actor aliases match the same allowlist and engine identity.
// Any nil part leaves the base behavior: every open proposal is withheld.
func WithFeedbackDetection(reader port.ProposalStateReader, repo, engineLogin string, ledger FeedbackLedger, trustedLogins ...string) Option {
	return func(d *Dispatcher) {
		if reader == nil || strings.TrimSpace(repo) == "" || ledger == nil {
			return
		}
		trusted := map[string]bool{}
		for _, l := range trustedLogins {
			if l = strings.ToLower(strings.TrimSpace(l)); l != "" {
				trusted[l] = true
			}
		}
		d.feedback = &feedbackDetection{reader: reader, repo: strings.TrimSpace(repo), login: strings.TrimSpace(engineLogin), ledger: ledger, trusted: trusted}
	}
}

// WithSessionContinuity installs task session continuity on every Worker the
// sweep builds (worker.WithSessionContinuity) and projects its decisions onto
// the event stream. A nil continuity is ignored.
func WithSessionContinuity(c worker.SessionContinuity) Option {
	return func(d *Dispatcher) {
		if c != nil {
			d.sessions = c
		}
	}
}

// feedbackDue decides whether task's open proposal carries unserviced human
// review activity. It returns the round to run, or nil when the proposal is
// simply awaiting (no new human activity), and an error when the decision
// could not be made — the caller then withholds, never guesses.
func (f *feedbackDetection) feedbackDue(ctx context.Context, task port.Task, proposal int) (*port.ReviewFeedback, error) {
	ref := port.ProposalRef{Repo: f.repo, Number: proposal}
	states, err := f.reader.States(ctx, []port.ProposalRef{ref}, port.Cursor{})
	if err != nil {
		return nil, fmt.Errorf("read proposal #%d state: %w", proposal, err)
	}
	if len(states) != 1 {
		return nil, fmt.Errorf("read proposal #%d state: reader answered %d states for one ref", proposal, len(states))
	}
	st := states[0]
	if st.Unknown != nil {
		return nil, fmt.Errorf("read proposal #%d state: %s", proposal, st.Unknown)
	}
	last, rounds, err := f.ledger.LastFeedback(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("read feedback cursor for %s: %w", task.ID, err)
	}
	var human []port.Comment
	var seen map[string]bool
	if native, ok := f.ledger.(nativeFeedbackLedger); ok {
		seen, err = native.FeedbackSeen(ctx, task)
		if err != nil {
			return nil, fmt.Errorf("read feedback identities for %s: %w", task.ID, err)
		}
	}
	latest := last
	for _, c := range st.Comments {
		newAtSameTime := c.CreatedAt.Equal(last) && c.NativeID != "" && seen != nil && !seen[c.NativeID]
		if (!c.CreatedAt.After(last) && !newAtSameTime) || f.engineAuthored(c) || !f.trustedAuthor(c) || approvalOnly(c) {
			continue
		}
		human = append(human, c)
		if c.CreatedAt.After(latest) {
			latest = c.CreatedAt
		}
	}
	if len(human) == 0 {
		return nil, nil
	}
	sort.SliceStable(human, func(i, j int) bool { return human[i].CreatedAt.Before(human[j].CreatedAt) })
	return &port.ReviewFeedback{
		Proposal: proposal, Round: rounds + 1, Comments: human,
		ChangesRequested: strings.EqualFold(st.ReviewDecision, port.ReviewChangesRequested), Latest: latest,
	}, nil
}

// trustedAuthor reports whether a comment's author is authorized to wake a
// feedback round: a trusted author association from the medium, or a login on
// the operator's allowlist. Fail-closed: no association and no allowlist hit
// means the comment is recorded on the proposal but never launches a round.
func (f *feedbackDetection) trustedAuthor(c port.Comment) bool {
	if c.Untrusted {
		return false
	}
	if trustedAssociations[strings.ToUpper(strings.TrimSpace(c.AuthorAssociation))] {
		return true
	}
	if f.trusted[strings.ToLower(strings.TrimSpace(c.Author))] {
		return true
	}
	return c.AuthorAlias != "" && f.trusted[strings.ToLower(strings.TrimSpace(c.AuthorAlias))]
}

// approvalOnly reports whether a comment is a bodyless review verdict that is
// not CHANGES_REQUESTED (an empty APPROVED or COMMENTED review shell): the
// operator ruling is that approvals alone never wake a round — only comments
// (any surface, including a review summary WITH a body) and changes-requested
// verdicts do.
func approvalOnly(c port.Comment) bool {
	return c.Surface == port.SurfaceReviewSummary && strings.TrimSpace(c.Body) == "" &&
		!strings.EqualFold(strings.TrimSpace(c.ReviewState), port.ReviewChangesRequested)
}

// engineAuthored reports whether a comment is the executor's own voice — its
// code-host account, the impl-review artifact it publishes, or its progress
// notes — none of which is human feedback.
func (f *feedbackDetection) engineAuthored(c port.Comment) bool {
	if f.login != "" {
		if strings.EqualFold(strings.TrimSpace(c.Author), f.login) {
			return true
		}
		if strings.EqualFold(strings.TrimSpace(c.AuthorAlias), f.login) {
			return true
		}
	}
	body := strings.TrimSpace(c.Body)
	return strings.HasPrefix(body, "impl-review report") || strings.Contains(body, "```impl-review") || strings.HasPrefix(body, "Opened proposal #")
}

// admitFeedback is withholdProposed's decision for one open-proposal task:
// admitted with the round attached (the cursor already advanced), or withheld
// with the reason. It never panics on a nil detector: no detector means every
// open proposal is withheld, exactly as before feedback rounds existed.
func (d *Dispatcher) admitFeedback(ctx context.Context, task port.Task, proposal int) (port.Task, bool, error) {
	if d.feedback == nil {
		return task, false, nil
	}
	fb, err := d.feedback.feedbackDue(ctx, task, proposal)
	if err != nil || fb == nil {
		return task, false, err
	}
	// Advance the cursor BEFORE the round runs (see the file comment): the
	// comments are serviced by this attempt whatever its outcome. A cursor that
	// cannot be advanced withholds — admitting would risk servicing the same
	// comments on every sweep.
	if native, ok := d.feedback.ledger.(nativeFeedbackLedger); ok {
		err = native.RecordFeedbackComments(ctx, task, fb.Latest, fb.Comments)
	} else {
		err = d.feedback.ledger.RecordFeedback(ctx, task, fb.Latest)
	}
	if err != nil {
		return task, false, fmt.Errorf("advance feedback cursor for %s: %w", task.ID, err)
	}
	task.Feedback = fb
	d.emit(report.FeedbackRound(task.ID, task.Number, fb.Proposal, fb.Round, len(fb.Comments), fb.ChangesRequested))
	return task, true, nil
}

// sessionObserver projects the Worker's continuity decisions onto the stream.
func (d *Dispatcher) sessionObserver() worker.SessionObserver {
	return func(task port.Task, e worker.SessionEvent) {
		reason := ""
		if e.Err != nil {
			reason = e.Err.Error()
		}
		d.emit(report.TaskSession(task.ID, task.Number, e.Harness, e.Action, e.SessionID, e.Purpose, reason))
	}
}

var errFeedbackUndecided = errors.New("dispatch: human feedback state could not be read")
