// Package report is the executor's normalized event stream and its two sinks —
// the one reporting surface both drivers share (docs/tech-design/implement-now.md
// §3.4, §3.9). A frontier-sweep emits one stream of typed lifecycle events
// (EVT-1..4 plus the inherited task_outcome wire and EVT-15/EVT-16:
// run_started · task_dispatched · ci_fix_attempt · task_verified · task_outcome ·
// impl_review_terminal · run_finished · review_artifact_publication_terminal ·
// hosted_review_publication_terminal · task.implementation_completed ·
// task.completed · plan.completed); a Reporter fans each
// event out, unchanged, to every configured Sink (AP-2 — identical events
// under both drivers).
//
// Two sinks are provided over that single core (never two cores, §3.9):
//
//   - OperatorSink — the attended live progress view, a structured log line per
//     event via log/slog (the operator's session view).
//   - ArgusSink — the unattended append-only JSONL journal (files-as-truth), one
//     self-describing event per line, with a fail-closed escalation surfaced the
//     moment a task escalates (a stuck/failed task, §3.9).
//
// The package is a leaf of the hexagon (AP-1): it imports only the standard
// library, so the Dispatcher and the drive-to-green loop depend on it, never the
// reverse. Events are structured, schema-versioned-by-convention structs with
// explicit json tags and additive evolution only — never a map[string]any
// contract (AP-5, AP-6).
package report

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
)

// Kind is the discriminator of a normalized executor event — the closed set of
// lifecycle moments (§3.4). It is the one field every sink switches on.
type Kind string

const (
	// Command events extend the issue-entry journal beyond the worker sweep to
	// final verification and metadata publication, including no-launch replay.
	KindCommandStarted       Kind = "command_started"
	KindCommandPhaseStarted  Kind = "command_phase_started"
	KindCommandPhaseFinished Kind = "command_phase_finished"
	KindCommandFinished      Kind = "command_finished"
	// KindRunStarted (EVT-1) opens a run: one frontier-sweep began.
	KindRunStarted Kind = "run_started"
	// KindTaskDispatched (EVT-2) — a ready task was handed to a fresh Worker on a
	// resolved backend.
	KindTaskDispatched Kind = "task_dispatched"
	// KindCIFixAttempt (EVT-3) — one bounded CI self-heal attempt began; Attempt of
	// Cap. The attempt count is bounded AND observable (prd:implement-now/FR-9).
	KindCIFixAttempt Kind = "ci_fix_attempt"
	// KindTaskVerified (EVT-4) — a Worker's verify-by-artifact resolved to a
	// verdict (green / hollow / red). Success is the verified artifact, never the
	// exit code (DINV-3).
	KindTaskVerified Kind = "task_verified"
	// KindTaskOutcome is the existing serialized implementation terminal. Its
	// discriminator remains task_outcome until the routed-outcome migration lands;
	// review data never rides this event.
	KindTaskOutcome Kind = "task_outcome"
	// KindImplReviewTerminal (EVT-15) is one independent configured-review terminal.
	KindImplReviewTerminal Kind = "impl_review_terminal"
	// KindRunFinished (EVT-16) — the run and every configured review ended; the
	// headline counts are final.
	KindRunFinished Kind = "run_finished"
	// KindHostedReviewPublicationTerminal (EVT-17) is implement-now's exact
	// reciprocal of impl-review EVT-13 for one publication moment.
	KindHostedReviewPublicationTerminal Kind = "hosted_review_publication_terminal"
	// KindReviewArtifactPublicationTerminal is impl-review EVT-13, emitted from
	// the host persistence adapter's immutable publication terminal.
	KindReviewArtifactPublicationTerminal Kind = "review_artifact_publication_terminal"
	// KindTaskImplementationCompleted is the task lifecycle's verified handoff:
	// implementation and required review are complete, while landing remains
	// outstanding.
	KindTaskImplementationCompleted Kind = "task.implementation_completed"
	// KindTaskCompleted is emitted only after the task's accepted changes are
	// proven landed and delivery is verified.
	KindTaskCompleted Kind = "task.completed"
	// KindPlanCompleted is emitted after every required task is delivered and
	// the plan-level completion conditions pass.
	KindPlanCompleted Kind = "plan.completed"

	// The local-first convergence loop (worker/prepublish.go). These events
	// describe the work a task does BEFORE its branch is pushed.

	// KindLocalGateAttempt — the CI-mirroring local gate went red in the
	// worktree and one bounded local fixer attempt began; Attempt of Cap. No
	// push and no CI run is spent on it.
	KindLocalGateAttempt Kind = "local_gate_attempt"
	// KindLocalGateTerminal records the final local CI result before review or
	// publication, including a first-pass success with no fixer attempt.
	KindLocalGateTerminal Kind = "local_gate_terminal"
	KindGateCheckProgress Kind = "gate_check_progress"
	KindRecoveryHandoff   Kind = "recovery_handoff"
	KindAgentProgress     Kind = "agent_progress"
	// KindReviewRound — one pre-publish review round over the exact local
	// revision finished with a verdict (Round, Verdict, Revision, Blocking).
	KindReviewRound Kind = "review_round"
	// KindRemediationAttempt — the review requested changes and one bounded
	// remediation sublaunch began; Attempt of Cap.
	KindRemediationAttempt Kind = "remediation_attempt"
	// KindPrePublishReviewTerminal — the pre-publish review loop's terminal for
	// one task: rounds, remediations, the last verdict/revision, whether the
	// review covers the published head (Covered) and where its artifact landed.
	KindPrePublishReviewTerminal Kind = "pre_publish_review_terminal"

	// Task session continuity (worker/session.go, tasksession).

	// KindTaskSession — one implementation-role launch's continuity decision:
	// Action started / resumed / degraded, the harness and session id, the
	// launch Purpose (implementation, remediation, fix, feedback).
	KindTaskSession Kind = "task_session"
	// KindFeedbackRound — a human-review feedback round was admitted for a task
	// with an open proposal: the proposal, the round ordinal, how many new
	// comments it answers and whether changes were requested.
	KindFeedbackRound Kind = "feedback_round"
)

const (
	ReviewStatusCompleted      = "verified"
	ReviewStatusFailed         = "failed"
	ReviewObligationVerified   = "verified"
	ReviewObligationPending    = "pending"
	ReviewObligationNotCreated = "not_created"
)

// Verdict is the closed set of verify-by-artifact results a task_verified event
// carries (§3.4 — "proposal, green, hollow"). It is never a success signal on its
// own; the terminal class is task_outcome.
const (
	// VerdictGreen — a proposal exists and every required check is green (DINV-3).
	VerdictGreen = "green"
	// VerdictHollow — a clean run that opened no proposal (DINV-3, never succeeded).
	VerdictHollow = "hollow"
	// VerdictRed — a proposal exists but is not green.
	VerdictRed = "red"
)

// Counts are the headline tallies a run_started/run_finished event carries. It is
// a pointer field on Event so it serializes only for the run-level events (and
// then shows every count, including zeros — a run that finished 0/0/0/0 is real
// information, not an omittable default).
type Counts struct {
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Blocked   int `json:"blocked"`
	Tokens    int `json:"tokens"`

	RoutingExact       int `json:"routing_exact"`
	RoutingPromoted    int `json:"routing_promoted"`
	RoutingUnsupported int `json:"routing_unsupported"`
	RoutingInvalid     int `json:"routing_invalid"`

	// These internal tallies remain available to the local report and operator
	// view, but EVT-16's closed 14-key wire schema does not include them. A
	// withheld frontier remains distinguishable from an idle one through its
	// per-task task_outcome terminals in the same durable journal (AP-16).
	Skipped int `json:"-"`

	// AwaitingProposal and ProbeUnavailable are the two pre-claim proposal
	// preflight classes, here for exactly the reason above: a sweep that
	// withheld its whole ready set — because the work was already proposed, or
	// because the preflight could not be read at all — would otherwise publish
	// a run_finished byte-identical to a run that found nothing ready. On an
	// unattended drive the journal is the only record, so a code-host outage
	// that stalled the frontier must not be indistinguishable from an idle
	// sweep there (AP-16). Neither is ever counted succeeded or failed.
	AwaitingProposal int `json:"-"`
	ProbeUnavailable int `json:"-"`
	// PrePublishReviewed counts succeeded tasks whose pre-publish review covers
	// the exact published head (no post-green review owed). Local to the report
	// and operator view; EVT-16's closed wire schema is unchanged.
	PrePublishReviewed int `json:"-"`
	// FeedbackRounds counts human-review feedback rounds this sweep admitted.
	FeedbackRounds int `json:"-"`

	ReviewCompleted            int `json:"review_completed"`
	ReviewFailed               int `json:"review_failed"`
	ReviewPending              int `json:"review_pending"`
	ReviewArtifactUnverified   int `json:"review_artifact_unverified"`
	ReviewArtifactUntrusted    int `json:"review_artifact_untrusted"`
	ReviewInfrastructureFailed int `json:"review_infrastructure_failed"`
}

// Event is one normalized event in the executor stream. It is a single closed
// struct with a Kind discriminator: every field carries the trace/correlation
// context the run needs (plan task id → tracker issue → backend → verdict →
// class), and only the fields relevant to a Kind are populated (AP-5). Evolution
// is additive: new optional fields may be added; existing ones are stable (AP-6).
type Event struct {
	RemainingMS *int64 `json:"remaining_ms,omitempty"`
	StdoutLog   string `json:"stdout_log,omitempty"`
	StderrLog   string `json:"stderr_log,omitempty"`
	Usage       *Usage `json:"usage,omitempty"`
	// ID is the stable delivery identity used by consequential handlers. A
	// lifecycle owner must supply it before durable publication.
	ID   string    `json:"id,omitempty"`
	Kind Kind      `json:"kind"`
	Time time.Time `json:"time"`
	// RunID correlates every event of one sweep. The Reporter stamps it when a
	// producer leaves it empty, so a producer never has to thread it.
	RunID string `json:"run_id,omitempty"`
	// Command timing is separate from run_finished's worker/review counts.
	// Pointers retain observed zero duration and a successful exit code of zero.
	ElapsedMS *int64 `json:"elapsed_ms,omitempty"`
	ExitCode  *int   `json:"exit_code,omitempty"`

	// Per-task correlation (task_dispatched / ci_fix_attempt / task_verified /
	// task_outcome). TaskID is the plan-task id; Issue is its tracker number.
	TaskID string `json:"task_id,omitempty"`
	Issue  int    `json:"issue,omitempty"`

	// task_dispatched: the resolved harness. Its
	// concrete model and effort use the shared fields below when locally routed.
	// The JSON name stays backend so existing journals still decode.
	Harness entity.Harness `json:"backend,omitempty"`

	// ci_fix_attempt: the current attempt and the configured cap.
	Attempt int `json:"attempt,omitempty"`
	Cap     int `json:"cap,omitempty"`

	// task_verified: the verify-by-artifact verdict (green / hollow / red).
	// review_round / pre_publish_review_terminal: the review verdict.
	Verdict string `json:"verdict,omitempty"`

	// review_round / remediation_attempt / pre_publish_review_terminal: the
	// 1-based review round, the unresolved blocking-finding count, the
	// remediation sublaunches consumed, and whether the accepted review covers
	// the published head (so no post-green review is owed).
	Round        int  `json:"round,omitempty"`
	Blocking     int  `json:"blocking,omitempty"`
	Remediations int  `json:"remediations,omitempty"`
	Covered      bool `json:"covered,omitempty"`
	// pre_publish_review_terminal: the regions remediations changed away from
	// the findings they were sent to fix (scope drift), one location each.
	ScopeDrift []string `json:"scope_drift,omitempty"`

	// task_session / feedback_round: the continuity action and session
	// identity, the launch purpose, and the feedback round's comment count.
	Action           string `json:"action,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	Purpose          string `json:"purpose,omitempty"`
	Comments         int    `json:"comments,omitempty"`
	ChangesRequested bool   `json:"changes_requested,omitempty"`

	// task_outcome: the terminal class, the fail-closed escalation flag (a stuck
	// task that exhausted its cap), and an advisory failure reason.
	Class     string `json:"class,omitempty"`
	Escalated bool   `json:"escalated,omitempty"`
	// Reason is the kind-specific failure text. Review terminals are independent
	// events, so their reason never contaminates an implementation task outcome.
	Reason string `json:"reason,omitempty"`

	// impl_review_terminal (EVT-15). These fields are never populated on the
	// implementation task_outcome event; Model and Effort are also valid on
	// task_dispatched as ordinary route telemetry.
	DriveRoundID         string `json:"drive_round_id,omitempty"`
	ReviewAttemptID      string `json:"review_attempt_id,omitempty"`
	ReviewObligationID   string `json:"review_obligation_id,omitempty"`
	ReviewProcessID      string `json:"review_process_id,omitempty"`
	PublicationAttemptID string `json:"publication_attempt_id,omitempty"`
	ReviewerPrincipal    string `json:"reviewer_principal,omitempty"`
	ProposalRef          string `json:"proposal_ref,omitempty"`
	Revision             string `json:"revision,omitempty"`
	ProposalID           string `json:"proposal_id,omitempty"`
	ReviewedRevision     string `json:"reviewed_revision,omitempty"`
	NativeProfile        string `json:"native_profile,omitempty"`
	Catalog              string `json:"catalog,omitempty"`
	Policy               string `json:"policy,omitempty"`
	Model                string `json:"model,omitempty"`
	Effort               string `json:"effort,omitempty"`
	Phase                string `json:"phase,omitempty"`
	Status               string `json:"status,omitempty"`
	ObligationStatus     string `json:"obligation_status,omitempty"`
	FailureClass         string `json:"failure_class,omitempty"`
	ArtifactRef          string `json:"artifact_ref,omitempty"`
	ArtifactProofRef     string `json:"artifact_proof_ref,omitempty"`
	NativeReceipt        string `json:"native_receipt,omitempty"`
	AttemptProofRef      string `json:"attempt_proof_ref,omitempty"`
	Medium               string `json:"medium,omitempty"`
	IdempotencyID        string `json:"idempotency_id,omitempty"`

	// run_started / run_finished: the headline tallies.
	Counts *Counts `json:"counts,omitempty"`

	// run_started: who invoked this sweep and over what scope (self-drive-wake/001
	// T8, EVT-6). Initiator distinguishes an attended operator run from a
	// wake-driven one; Scope names what the invoker was sweeping (a repo slug, a
	// plan reference, …). Both are additive and optional — an invoker that sets
	// neither renders exactly as before this property existed.
	Initiator string `json:"initiator,omitempty"`
	Scope     string `json:"scope,omitempty"`

	// Plan identifies the task's frozen plan for lifecycle events.
	Plan string `json:"plan,omitempty"`
}

// Usage distinguishes an observed zero from an interrupted, unreported window.
type Usage struct {
	Tokens         int  `json:"tokens"`
	Turns          int  `json:"turns"`
	TokensObserved bool `json:"tokens_observed"`
	TurnsObserved  bool `json:"turns_observed"`
}

// TaskImplementationCompleted builds the lifecycle transition that awaits
// merge. The caller owns the completion predicate; registration only controls
// what reacts to the verified transition.
func TaskImplementationCompleted(id, taskID string, issue int, plan, revision, proposalRef string) Event {
	return Event{ID: id, Kind: KindTaskImplementationCompleted, TaskID: taskID, Issue: issue,
		Plan: plan, Revision: revision, ProposalRef: proposalRef}
}

// TaskCompleted builds the delivered-task transition.
func TaskCompleted(id, taskID string, issue int, plan, revision string) Event {
	return Event{ID: id, Kind: KindTaskCompleted, TaskID: taskID, Issue: issue, Plan: plan, Revision: revision}
}

// PlanCompleted builds the plan transition. Feature/project completion is not
// implied; a registered parent handler may re-evaluate those lifecycles.
func PlanCompleted(id, plan, revision string) Event {
	return Event{ID: id, Kind: KindPlanCompleted, Plan: plan, Revision: revision}
}

// MarshalJSON keeps legacy event bytes additive while making EVT-15's required
// escalated boolean explicit even when false. The shared Event field remains
// omitempty for older kinds, whose wire contract did not carry false values.
func (e Event) MarshalJSON() ([]byte, error) {
	type eventWire Event
	if e.Kind == KindImplReviewTerminal || e.Kind == KindHostedReviewPublicationTerminal {
		return json.Marshal(struct {
			eventWire
			Escalated bool `json:"escalated"`
		}{eventWire: eventWire(e), Escalated: e.Escalated})
	}
	return json.Marshal(eventWire(e))
}

type HostedReviewPublicationTerminalFields struct {
	AttemptID, ObligationID, ProposalRef, Revision      string
	Status, ArtifactRef, NativeReceipt, AttemptProofRef string
	FailureClass, Reason                                string
	Escalated                                           bool
}

func HostedReviewPublicationTerminal(f HostedReviewPublicationTerminalFields) Event {
	return Event{Kind: KindHostedReviewPublicationTerminal,
		ReviewAttemptID: f.AttemptID, ReviewObligationID: f.ObligationID,
		ProposalRef: f.ProposalRef, Revision: f.Revision, Status: f.Status,
		ArtifactRef: f.ArtifactRef, NativeReceipt: f.NativeReceipt,
		AttemptProofRef: f.AttemptProofRef, FailureClass: f.FailureClass,
		Reason: f.Reason, Escalated: f.Escalated}
}

type ReviewArtifactPublicationTerminalFields struct {
	ProcessID, ObligationID, PublicationAttemptID, ProposalID, ReviewedRevision string
	Medium, IdempotencyID, Status, ArtifactRef, NativeReceipt, AttemptProofRef  string
	FailureClass, Reason                                                        string
}

func ReviewArtifactPublicationTerminal(f ReviewArtifactPublicationTerminalFields) Event {
	return Event{Kind: KindReviewArtifactPublicationTerminal,
		ReviewProcessID: f.ProcessID, ReviewObligationID: f.ObligationID,
		PublicationAttemptID: f.PublicationAttemptID, ProposalID: f.ProposalID,
		ReviewedRevision: f.ReviewedRevision, Medium: f.Medium, IdempotencyID: f.IdempotencyID,
		Status: f.Status, ArtifactRef: f.ArtifactRef, NativeReceipt: f.NativeReceipt,
		AttemptProofRef: f.AttemptProofRef, FailureClass: f.FailureClass, Reason: f.Reason}
}

// RunStarted builds the EVT-1 event opening a run, attributed to the invoker
// that started it (self-drive-wake/001 T8): initiator (e.g. "operator",
// "wake") and scope (e.g. a repo slug, a plan reference) are both optional —
// an invoker that passes "" for either renders exactly as before this
// property existed (additive, EVT-6).
func RunStarted(initiator, scope string) Event {
	return Event{Kind: KindRunStarted, Initiator: initiator, Scope: scope}
}

// RunFinished builds the EVT-16 event closing a run with its final headline
// counts. It takes the whole Counts value rather than a positional list: the
// tally spans many same-typed fields, which is exactly the transposition
// hazard when the tally spans many same-typed fields.
func RunFinished(c Counts) Event {
	return Event{Kind: KindRunFinished, Counts: &c}
}

// TaskDispatched builds the EVT-2 event: a ready task handed to a Worker on the
// resolved backend.
func TaskDispatched(taskID string, issue int, harness entity.Harness) Event {
	return Event{Kind: KindTaskDispatched, TaskID: taskID, Issue: issue, Harness: harness}
}

// CIFixAttempt builds the EVT-3 event: the start of one bounded self-heal attempt.
func CIFixAttempt(taskID string, attempt, cap int) Event {
	return Event{Kind: KindCIFixAttempt, TaskID: taskID, Attempt: attempt, Cap: cap}
}

// LocalGateAttempt builds the local_gate_attempt event: the CI-mirroring local
// gate went red in the worktree and bounded local fixer attempt Attempt of Cap
// began — before any push, so no CI run is spent on it.
func LocalGateAttempt(taskID string, attempt, cap int) Event {
	return Event{Kind: KindLocalGateAttempt, TaskID: taskID, Attempt: attempt, Cap: cap}
}

func LocalGateTerminal(taskID string, green bool, attempts, cap int, reason string) Event {
	verdict := VerdictRed
	if green {
		verdict = VerdictGreen
	}
	return Event{Kind: KindLocalGateTerminal, TaskID: taskID, Verdict: verdict, Attempt: attempts, Cap: cap, Reason: reason}
}

// ReviewRoundFields are the review_round event's inputs.
type ReviewRoundFields struct {
	TaskID       string
	Issue        int
	Round        int
	Verdict      string
	Revision     string
	ProposalRef  string
	ReviewID     string
	Blocking     int
	Model        string
	Effort       string
	Phase        string
	FailureClass string
	Reason       string
}

// ReviewRound builds the review_round event: one pre-publish review round over
// the exact local revision reached a verdict (or failed, with Reason).
func ReviewRound(f ReviewRoundFields) Event {
	return Event{Kind: KindReviewRound, TaskID: f.TaskID, Issue: f.Issue, Round: f.Round,
		Verdict: f.Verdict, Revision: f.Revision, ProposalRef: f.ProposalRef,
		ReviewProcessID: f.ReviewID, Blocking: f.Blocking, Model: f.Model, Effort: f.Effort,
		Phase: f.Phase, FailureClass: f.FailureClass, Reason: f.Reason}
}

// RemediationAttempt builds the remediation_attempt event: the pre-publish
// review requested changes and bounded remediation sublaunch Attempt of Cap
// began in the task's worktree.
func RemediationAttempt(taskID string, issue, attempt, cap int) Event {
	return Event{Kind: KindRemediationAttempt, TaskID: taskID, Issue: issue, Attempt: attempt, Cap: cap}
}

// PrePublishReviewTerminalFields are the pre_publish_review_terminal event's
// inputs.
type PrePublishReviewTerminalFields struct {
	TaskID       string
	Issue        int
	Rounds       int
	Remediations int
	Cap          int
	Verdict      string
	Revision     string
	ReviewID     string
	Covered      bool
	ArtifactRef  string
	Reason       string
	Escalated    bool
	ScopeDrift   []string
}

// PrePublishReviewTerminal builds the pre_publish_review_terminal event: the
// loop's terminal for one task. Covered=true means the accepted review binds
// the exact head the proposal carries, so no post-green review is owed for it.
func PrePublishReviewTerminal(f PrePublishReviewTerminalFields) Event {
	return Event{Kind: KindPrePublishReviewTerminal, TaskID: f.TaskID, Issue: f.Issue,
		Round: f.Rounds, Remediations: f.Remediations, Cap: f.Cap, Verdict: f.Verdict,
		Revision: f.Revision, ReviewProcessID: f.ReviewID, Covered: f.Covered,
		ArtifactRef: f.ArtifactRef, Reason: f.Reason, Escalated: f.Escalated, ScopeDrift: f.ScopeDrift}
}

// TaskSession builds the task_session event: one launch's continuity decision.
func TaskSession(taskID string, issue int, harness entity.Harness, action, sessionID, purpose, reason string) Event {
	return Event{Kind: KindTaskSession, TaskID: taskID, Issue: issue, Harness: harness, Action: action,
		SessionID: sessionID, Purpose: purpose, Reason: reason}
}

// FeedbackRound builds the feedback_round event: a human-review feedback round
// admitted for a task's open proposal.
func FeedbackRound(taskID string, issue, proposal, round, comments int, changesRequested bool) Event {
	return Event{Kind: KindFeedbackRound, TaskID: taskID, Issue: issue, Round: round,
		ProposalRef: fmt.Sprintf("#%d", proposal), Comments: comments, ChangesRequested: changesRequested}
}

// TaskVerified builds the EVT-4 event: a task's verify-by-artifact verdict.
func TaskVerified(taskID string, issue int, verdict string) Event {
	return Event{Kind: KindTaskVerified, TaskID: taskID, Issue: issue, Verdict: verdict}
}

// TaskOutcome builds the inherited task_outcome wire event: a task's terminal
// class, escalation flag, and advisory reason. It deliberately claims no
// routed_task_outcome conformance until that separate migration lands.
func TaskOutcome(taskID string, issue int, class string, escalated bool, reason string) Event {
	return Event{Kind: KindTaskOutcome, TaskID: taskID, Issue: issue, Class: class, Escalated: escalated, Reason: reason}
}

type ImplReviewTerminalFields struct {
	DriveRoundID, AttemptID, ObligationID, TaskID, ProposalRef, Revision                 string
	Harness                                                                              entity.Harness
	ReviewerPrincipal, NativeProfile, Catalog, Policy, Model, Effort                     string
	Phase, Status, ObligationStatus, FailureClass, ArtifactRef, ArtifactProofRef, Reason string
	Escalated                                                                            bool
}

func ImplReviewTerminal(f ImplReviewTerminalFields) Event {
	return Event{Kind: KindImplReviewTerminal, DriveRoundID: f.DriveRoundID,
		ReviewAttemptID: f.AttemptID, ReviewObligationID: f.ObligationID,
		TaskID: f.TaskID, ProposalRef: f.ProposalRef, Revision: f.Revision,
		Harness: f.Harness, ReviewerPrincipal: f.ReviewerPrincipal,
		NativeProfile: f.NativeProfile, Catalog: f.Catalog,
		Policy: f.Policy, Model: f.Model, Effort: f.Effort,
		Phase: f.Phase, Status: f.Status,
		ObligationStatus: f.ObligationStatus, FailureClass: f.FailureClass,
		ArtifactRef: f.ArtifactRef, ArtifactProofRef: f.ArtifactProofRef,
		Reason: f.Reason, Escalated: f.Escalated}
}

// ClassProbeUnavailable is the task_outcome class an executor emits when a
// pre-claim preflight could not be evaluated (dispatch.ClassProbeUnavailable's
// String form). The literal is restated here rather than imported because
// dispatch imports this package; the event's string Class field is the round
// trip, and dispatch's own test pins the two together.
const ClassProbeUnavailable = "probe-unavailable"

// IsEscalation reports whether an event is a fail-closed escalation the argus
// sink must surface to the human per its authority contract (§3.9): a stuck task
// that exhausted its cap (Escalated), any task that reached a failed terminal
// (a "stuck/failed" task — the two the design escalates), or a task the sweep
// withheld because a preflight was unreadable. That last one escalates because
// the sweep found ready work and could not judge whether starting it was safe:
// an unattended drive that silently withheld its whole frontier would otherwise
// be indistinguishable from an idle one (AP-16). It reads only task_outcome
// events; every other kind is progress, not an escalation. A configured review
// failure rides the same terminal event and escalates without changing Class.
func IsEscalation(e Event) bool {
	if e.Kind == KindImplReviewTerminal {
		return e.Escalated && e.Status == ReviewStatusFailed
	}
	return e.Kind == KindTaskOutcome && (e.Escalated || e.Class == "failed" || e.Class == ClassProbeUnavailable)
}

// NewRunID returns a fresh, opaque run correlation id (8 random bytes, hex). It
// is used by a driver (the cmd surface) to stamp one sweep's events; the Reporter
// carries it onto every normalized event kind so the whole run shares one
// correlation key.
func NewRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is catastrophic and vanishingly rare; fall back to a
		// time-derived id so a run still correlates rather than aborting.
		return "run-" + time.Now().UTC().Format("20060102T150405.000000000")
	}
	return "run-" + hex.EncodeToString(b[:])
}
