package port

import (
	"context"
	"errors"

	"github.com/agenticlab-ai/pyxis/internal/entity"
)

// ErrNotQuiescent means a runtime could not prove its process tree stopped.
// Hosts must preserve the workspace without checkpointing or publishing it.
var ErrNotQuiescent = errors.New("agent runtime: process tree did not stop")

// ErrRunNonRetryable means the native provider explicitly refused retrying.
// It is terminal for this invocation, rather than a recoverable turn truncation.
var ErrRunNonRetryable = errors.New("agent runtime: non-retryable run failure")

// AgentRuntime is the port onto a coding-agent CLI backend. Each production
// adapter (claude, codex, cursor, opencode) spawns a fresh non-interactive
// process per task — no session is ever reused across tasks (DINV-11, the
// charter-bleed guard). Completion is signalled by process exit; success is
// never inferred from exit alone — the Worker verifies by artifact (DINV-3,
// §3.7).
type AgentRuntime interface {
	// Launch starts a fresh agent process for one task against the given spec and
	// returns a Handle to await or kill. The brief in the spec is the only
	// instruction the agent is given (prd:implement-now/FR-19).
	Launch(ctx context.Context, spec LaunchSpec) (Handle, error)

	// Await blocks until the launched process finishes and reports its resource
	// usage. A non-nil error is an operational failure — the process could not be
	// run or awaited; a nil error means the process finished, and whether it
	// produced a usable artifact is decided separately, by verification (DINV-3).
	// Return ErrNotQuiescent when an error leaves possible live writers; other
	// returns acknowledge termination before the host may checkpoint the tree.
	Await(ctx context.Context, handle Handle) (RunResult, error)

	// Kill terminates a launched process (cancellation, or a breached bound).
	Kill(ctx context.Context, handle Handle) error

	// AuthCheck preflights install and authentication for this backend. It
	// returns a descriptive error when a credential is missing, so the caller can
	// degrade loudly without consuming an attempt (prd:implement-now/FR-16).
	AuthCheck(ctx context.Context) error
}

// OwnerGeneration is the monotonically increasing fencing generation for one
// resolved routing attempt. Generation one is bound to routing_resolved;
// subsequent generations require an exact-read ownership-transfer proof.
type OwnerGeneration uint64

// OwnershipProof is the canonical evidence presented when a generation is
// activated. Reference and Digest bind an exact-read event; a transfer also
// carries the runtime proof whose nonce the event records.
type OwnershipProof struct {
	Kind            string
	Reference       string
	Digest          string
	AttemptID       string
	Harness         entity.Harness
	OwnerGeneration OwnerGeneration
	Fence           RuntimeFenceProof
}

const (
	OwnershipProofResolution = "routing_resolved"
	OwnershipProofTransfer   = "attempt_owner_transferred"
)

// LaunchCapability is a signed, one-generation launch authority. Its fields
// are public so it can cross a process boundary; Signature makes altered or
// fabricated values fail closed without exposing the runtime's signing key.
type LaunchCapability struct {
	AttemptID       string
	Harness         entity.Harness
	OwnerGeneration OwnerGeneration
	ProofReference  string
	ProofDigest     string
	Nonce           string
	CapabilityRef   string
	Signature       string
}

// RuntimeObservationKind is the closed liveness result returned after a
// generation has been fenced.
type RuntimeObservationKind string

const (
	RuntimeNotStarted   RuntimeObservationKind = "not_started"
	RuntimeRunning      RuntimeObservationKind = "running"
	RuntimeTerminal     RuntimeObservationKind = "terminal"
	RuntimeAbsentProven RuntimeObservationKind = "absent_proven"
	RuntimeUnknown      RuntimeObservationKind = "unknown"
)

// RuntimeObservation is returned with the signed fence proof. Handle is set
// only for a process the adapter can safely reattach to; it is never evidence
// that a transfer is allowed.
type RuntimeObservation struct {
	AttemptID       string
	OwnerGeneration OwnerGeneration
	Kind            RuntimeObservationKind
	Handle          Handle
}

// RuntimeFenceProof is signed by the runtime evidence authority and binds one
// observation to one fence nonce. ProofReference is the stable digest used by
// attempt_owner_transferred; native model credentials never enter this value.
type RuntimeFenceProof struct {
	AttemptID       string
	Harness         entity.Harness
	OwnerGeneration OwnerGeneration
	Observation     RuntimeObservationKind
	Nonce           string
	ProofReference  string
	Signature       string
}

// FencedRuntime is the consequential launch port. Activate mints exactly one
// capability for a proven current generation. LaunchFenced atomically consumes
// it at the adapter's real process-start seam. FenceAndObserve revokes any
// unconsumed capability before returning its signed observation.
type FencedRuntime interface {
	AgentRuntime
	Activate(attempt string, generation OwnerGeneration, proof OwnershipProof) (LaunchCapability, error)
	LaunchFenced(context.Context, LaunchSpec, LaunchCapability) (Handle, error)
	FenceAndObserve(context.Context, string, OwnerGeneration) (RuntimeFenceProof, RuntimeObservation, error)
	AuthorizeTerminal(LaunchCapability) error
}

// LaunchPurpose identifies which host-owned instruction protocol, if any, is
// assembled around a launch brief. The zero value retains the historical
// implementation behavior for additive compatibility; independent reviewers
// explicitly select LaunchPurposeReview so a process workdir never smuggles the
// implementation commit/publish protocol into their instructions.
// Prototypes select their own local build protocol without a delivery gate.
type LaunchPurpose string

const (
	LaunchPurposeImplementation LaunchPurpose = "implementation"
	LaunchPurposeReview         LaunchPurpose = "review"
	LaunchPurposePrototype      LaunchPurpose = "prototype"
)

// LaunchSpec is the closed set of inputs a fresh agent process needs. It evolves
// additively (AP-6): new optional fields may be added; existing ones are stable.
type LaunchSpec struct {
	// Brief is the assembled charter handed to the agent (prd:implement-now/FR-19).
	Brief string

	// RecoveryDiagnostic is the bounded, redacted gate evidence from the prior
	// checkpoint. It is data for this task's continuation, never a new charter.
	RecoveryDiagnostic string

	// Purpose selects the host-owned instruction protocol. Empty and
	// LaunchPurposeImplementation both assemble the implementation delivery
	// protocol; LaunchPurposeReview keeps Brief byte-for-byte (after outer-space
	// trimming) while Workdir remains process metadata only.
	// LaunchPurposePrototype assembles the local prototype build protocol.
	Purpose LaunchPurpose

	// Workdir is the absolute path to the task's isolated worktree.
	Workdir string

	// RuntimeStateDir is a disposable, writable directory outside a read-only
	// review worktree. Review launches use it for HOME and XDG state while
	// retaining explicit access to the selected model CLI's own
	// credential store. Implementation launches leave it empty.
	RuntimeStateDir string

	// HeadBranch is the branch already checked out in the worktree — the branch
	// the supervisor commits to and publishes a proposal from. It is carried
	// into the assembled brief's delivery protocol so the agent knows which branch
	// its work belongs on (the "brief = protocol + head branch + charter" contract,
	// prd:implement-now/FR-19).
	HeadBranch string

	// Model and Effort are the resolved native route for this launch.
	Model  string
	Effort string

	// RequestDescription asks the agent to end its final message with the
	// pull request description of its change (internal/prdescription). The
	// Worker sets it on the launches whose final message it reads for one:
	// the implementation run, including a feedback round.
	RequestDescription bool

	// Feedback, when non-nil, makes this a human-review feedback round: the
	// assembled brief carries the new proposal comments as data after the
	// delivery protocol (the Worker copies it from Task.Feedback). Nil is an
	// ordinary implementation launch.
	Feedback *ReviewFeedback

	// Session, when non-nil, asks the adapter to run this launch as a
	// continuation of the task's own harness conversation (task session
	// continuity): the process is still fresh, but the model resumes the
	// context it built on earlier rounds of the SAME task — its charter, the
	// codebase it read, the findings it addressed. Nil is the historical
	// launch: a fresh conversation. A review launch never carries a Session
	// (the reviewer is blind and fresh by contract).
	Session *SessionSpec
}

// SessionSpec names the harness conversation a launch continues or starts.
// It is per task and per harness: a session is never resumed across tasks
// (the charter-bleed guard) and never for a reviewer.
type SessionSpec struct {
	// ID is the harness's session identity. On a first launch an adapter that
	// can pre-assign identities (claude's --session-id) starts the conversation
	// under this ID so it is known before the process exists; an adapter that
	// mints its own ignores it and reports the real one in RunResult.SessionID.
	ID string
	// Resume is true when ID names an existing conversation to continue.
	Resume bool
}

// RunResult is the outcome telemetry of one finished agent process, parsed from
// the CLI's structured output. Tokens/turns are usage, not a success verdict —
// success is the verified artifact (DINV-3). Incomplete reports whether the CLI
// itself signalled that the process stopped BEFORE finishing its work — it hit
// its turn cap or errored internally, rather than reaching a clean end. That
// distinction is what lets the Worker tell a recoverable truncation (respawn and
// resume) apart from a deliberate no-op (a genuine hollow): an adapter maps its
// native signal to this flag, and Reason carries the adapter-native detail
// (e.g. "error_max_turns") for logs.
type RunResult struct {
	// ReviewChildren contains results observed in native subagent tool events,
	// or in the harness's own records of its child sessions, separate from the
	// parent reviewer's untrusted aggregate claims.
	ReviewChildren *NativeReviewChildren
	Tokens         int
	Turns          int
	// ToolCalls counts native tool-use events when the adapter has a complete
	// event stream. It is unavailable for terminal-only CLI output.
	ToolCalls int
	// Presence distinguishes reported zero from unavailable CLI telemetry.
	TokensObserved    bool
	TurnsObserved     bool
	ToolCallsObserved bool
	Incomplete        bool
	NonRetryable      bool
	Reason            string
	// Output is the terminal assistant message. Implementation callers treat it
	// as telemetry only, except a pre-publish remediation's dispute block, which
	// can only stop the task for the operator (worker/prepublish.go). Review
	// callers treat it as untrusted broker input: the
	// host validates its closed artifact schema before publishing it with the
	// reviewer-service credential.
	Output string
	// SessionID is the harness conversation this process ran under, as the CLI
	// reported it — the identity a later launch of the same task resumes. Empty
	// when the adapter cannot report one (the task then gets no continuity).
	SessionID string
}

type NativeReviewChildren struct {
	Children []NativeReviewChild `json:"children"`
}

type NativeReviewChild struct {
	ContextID string `json:"context_id"`
	// InvocationID is the harness's own name for this child where the parent
	// never sees ContextID (Codex: the agent path spawn_agent returned). The
	// reviewer can retain only this name; the host joins it to ContextID.
	InvocationID string `json:"invocation_id,omitempty"`
	Completed    bool   `json:"completed"`
	Output       string `json:"output"`
}

// Handle is an opaque reference to one launched agent process. Adapters return
// their own concrete type — holding whatever process state they need — from
// Launch and receive it back in Await and Kill. The Worker treats it as an
// opaque token and never introspects it.
type Handle interface {
	// RuntimeHandle is a marker that keeps the port type-safe across adapters.
	// The Worker never calls it.
	RuntimeHandle()
}
