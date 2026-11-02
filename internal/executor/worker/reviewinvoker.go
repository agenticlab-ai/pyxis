package worker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

// Review phases are stable audit values. A result records the last boundary
// reached so launch, await, and artifact failures remain distinguishable on the
// executor's independent impl_review_terminal event.
const (
	ReviewPhasePreflight   = "preflight"
	ReviewPhaseObligation  = "obligation"
	ReviewPhaseRouting     = "routing"
	ReviewPhaseLaunch      = "launch"
	ReviewPhaseAwait       = "await"
	ReviewPhaseArtifact    = "artifact"
	ReviewPhaseAttribution = "attribution"

	ReviewFailureArtifactUnverified = "artifact_unverified"
	ReviewFailureArtifactUntrusted  = "artifact_untrusted"
	ReviewFailureInfrastructure     = "infrastructure"
)

// ReviewCapture is the immutable handoff created at the exact green boundary.
// Revision is read from the same code-host snapshot that proves the proposal
// green; ObligationID identifies the durable append made before the Worker can
// emit its implementation terminal or release its claim.
type ReviewCapture struct {
	ProposalRef  string
	Revision     string
	ObligationID string
	Harness      entity.Harness
	Persisted    bool
	Err          error
}

// Failed reports whether the exact-green obligation could not be persisted.
func (c ReviewCapture) Failed() bool {
	return c.Err != nil || c.ProposalRef == "" || c.Revision == "" || c.ObligationID == "" || c.Harness == ""
}

// ReviewResult is the independent impl-review outcome attached to one otherwise
// green implementation task. Err never changes the Worker's implementation
// Outcome, but the Dispatcher carries it into the run summary, exit status, and
// unattended escalation stream so an absent review cannot look like a clean
// run (#1029).
type ReviewResult struct {
	DriveRoundID       string
	AttemptID          string
	ObligationID       string
	Harness            entity.Harness
	ReviewerPrincipal  string
	ProposalRef        string
	Revision           string
	NativeProfileID    string
	CatalogID          string
	PolicyIdentity     string
	Model              string
	Effort             string
	Phase              string
	Status             string
	ObligationStatus   string
	FailureClass       string
	ArtifactRef        string
	ArtifactProofRef   string
	NativeReceipt      string
	EVT13Ref           string
	EVT17Ref           string
	Tokens             int
	ImplReviewTerminal *implreview.PublicationTerminal
	Publication        *ReviewPublicationTerminal
	Err                error
}

// ReviewPublicationTerminal is implement-now EVT-17, the exact reciprocal of
// impl-review EVT-13 for one publication moment. The first four identities are
// always required; verified terminals also require the three distinct evidence
// fields.
type ReviewPublicationTerminal struct {
	ReviewAttemptID    string `json:"review_attempt_id"`
	ReviewObligationID string `json:"review_obligation_id"`
	ProposalRef        string `json:"proposal_ref"`
	Revision           string `json:"revision"`
	Status             string `json:"status"`
	ArtifactRef        string `json:"artifact_ref,omitempty"`
	NativeReceipt      string `json:"native_receipt,omitempty"`
	AttemptProofRef    string `json:"attempt_proof_ref,omitempty"`
	FailureClass       string `json:"failure_class,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Escalated          bool   `json:"escalated"`
}

// Failed reports whether the configured reviewer failed to produce and
// exact-read its proposal artifact.
func (r ReviewResult) Failed() bool {
	return r.Err != nil || r.Status != "verified" || r.ObligationStatus != "verified" ||
		r.AttemptID == "" || r.ObligationID == "" ||
		r.ArtifactRef == "" || r.ArtifactProofRef == "" || r.NativeReceipt == "" ||
		r.EVT13Ref == "" || r.EVT17Ref == "" || r.ImplReviewTerminal == nil || r.Publication == nil
}

// ReviewInvoker invokes one bounded independent reviewer at the Worker's
// genuinely-green boundary. The task classification remains orthogonal while
// ReviewResult makes the review's own success an artifact verdict rather than a
// process-exit inference and lets the run fail closed independently.
type ReviewInvoker interface {
	Invoke(ctx context.Context, task port.Task, proposal port.Proposal) ReviewResult
}

// ReviewCapturer persists the exact-green review handoff while the Worker still
// owns its implementation claim. Invocation is intentionally a separate port:
// Dispatcher owns reviewer capacity and starts the sibling only after Run has
// returned and released both implementation resources.
type ReviewCapturer interface {
	Capture(ctx context.Context, task port.Task, proposal port.Proposal) ReviewCapture
	CompleteHandoff(ctx context.Context, capture ReviewCapture) error
}

// ReviewService is the complete typed service the Dispatcher may install. A
// configured factory cannot defer a missing preflight or capture capability
// until after task spend: both are compile-time requirements beside Invoke.
type ReviewService interface {
	ReviewInvoker
	ReviewCapturer
	Preflight(context.Context) error
	PreparePending(context.Context, string) (port.Task, port.Proposal, error)
	PrepareSelected(context.Context, string, string) (port.Task, port.Proposal, error)
}

// WithReviewCapturer installs the durable exact-green handoff. A capture error
// remains orthogonal to implementation truth, but suppresses the succeeded
// terminal observer and is reported as review infrastructure.
func WithReviewCapturer(c ReviewCapturer) Option {
	return func(w *Worker) {
		if c != nil {
			w.reviewCapturer = c
		}
	}
}

// GreenVerifiedObserver is called after the exact green verdict and all
// deterministic admissions, but before impl-review starts. Dispatcher uses it
// to order task_verified ahead of the review lifecycle without making Worker
// depend on the reporting package.
type GreenVerifiedObserver func(task port.Task)

// WithGreenVerifiedObserver installs the green-verdict observer. It is a
// reporting hook only; nil is ignored and it cannot alter classification.
func WithGreenVerifiedObserver(observe GreenVerifiedObserver) Option {
	return func(w *Worker) {
		if observe != nil {
			w.greenVerified = observe
		}
	}
}

// GreenCapturedObserver is called after the durable exact-green obligation is
// committed and before Worker.Run returns (and therefore before claim release).
// It is the only review-enabled location allowed to publish the succeeded
// implementation terminal.
type GreenCapturedObserver func(task port.Task, proposal port.Proposal, capture ReviewCapture) error

func WithGreenCapturedObserver(observe GreenCapturedObserver) Option {
	return func(w *Worker) {
		if observe != nil {
			w.greenCaptured = observe
		}
	}
}

func (w *Worker) ReviewCapture() (ReviewCapture, bool) {
	if w.reviewCapture == nil {
		return ReviewCapture{}, false
	}
	return *w.reviewCapture, true
}

// Proposal returns the genuinely-green proposal observed by this Worker.
func (w *Worker) Proposal() (port.Proposal, bool) {
	if w.greenProposal == nil {
		return port.Proposal{}, false
	}
	return *w.greenProposal, true
}
