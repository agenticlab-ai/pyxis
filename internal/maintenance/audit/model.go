// Package audit owns the closed maintenance lifecycle journal contract.
package audit

import "time"

const (
	SchemaVersion    = "pyxis-maintenance-log/v1"
	MaxRecordBytes   = 32 * 1024
	MaxSummaryBytes  = 4 * 1024
	MaxReferences    = 32
	MaxIdentityBytes = 1024
)

type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type EventName string

const (
	EventInstallation EventName = "maintenance_installation_result"
	EventClaim        EventName = "maintenance_claim_result"
	EventRun          EventName = "maintenance_run_result"
	EventLifecycle    EventName = "maintenance_lifecycle"
	EventTail         EventName = "journal_tail_incomplete"
)

type Phase string

const (
	PhaseAdmission     Phase = "admission"
	PhaseInstallation  Phase = "installation"
	PhaseConfiguration Phase = "configuration"
	PhaseRouting       Phase = "routing"
	PhaseClaim         Phase = "claim"
	PhaseWorkspace     Phase = "workspace"
	PhaseAgent         Phase = "agent"
	PhaseVerification  Phase = "verification"
	PhasePublication   Phase = "publication"
	PhaseTerminal      Phase = "terminal"
	PhaseCleanup       Phase = "cleanup"
	PhaseAudit         Phase = "audit"
)

type Outcome string

const (
	OutcomeSucceeded  Outcome = "succeeded"
	OutcomeNoChange   Outcome = "no_change"
	OutcomeFailed     Outcome = "failed"
	OutcomeTimedOut   Outcome = "timed_out"
	OutcomeIncomplete Outcome = "incomplete"
	OutcomeSkipped    Outcome = "skipped"
	OutcomeRefused    Outcome = "refused"
	OutcomeWon        Outcome = "won"
	OutcomeLost       Outcome = "lost"
	OutcomeUnproved   Outcome = "unproved"
)

type Identity struct {
	TraceID      string `json:"trace_id"`
	InvocationID string `json:"invocation_id"`
}

type Route struct {
	Harness       string `json:"harness,omitempty"`
	Profile       string `json:"profile,omitempty"`
	Model         string `json:"model,omitempty"`
	Effort        string `json:"effort,omitempty"`
	Policy        string `json:"policy,omitempty"`
	Qualification string `json:"qualification,omitempty"`
}

type Claim struct {
	Owner     string `json:"owner,omitempty"`
	Ref       string `json:"ref,omitempty"`
	RecordOID string `json:"record_oid,omitempty"`
}

type Delivery struct {
	Branch string `json:"branch,omitempty"`
	Head   string `json:"head,omitempty"`
	PRURL  string `json:"pr_url,omitempty"`
}

type TargetStatus string

const (
	TargetMeasured    TargetStatus = "measured"
	TargetNotVerified TargetStatus = "not_verified"
)

type TargetVerification struct {
	Status     TargetStatus `json:"status"`
	DurationMS *int64       `json:"duration_ms,omitempty"`
	TargetMS   *int64       `json:"target_ms,omitempty"`
}

type Reference struct {
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Digest   string `json:"digest,omitempty"`
}

type Usage struct {
	Tokens    *int `json:"tokens,omitempty"`
	Turns     *int `json:"turns,omitempty"`
	ToolCalls *int `json:"tool_calls,omitempty"`
}

// Event is one independently decodable lifecycle record. Writer-owned fields
// are replaced at append time; optional knowledge remains absent until known.
type Event struct {
	SchemaVersion string    `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`
	Level         Level     `json:"level"`
	Event         EventName `json:"event"`
	Phase         Phase     `json:"phase"`
	TraceID       string    `json:"trace_id"`
	InvocationID  string    `json:"invocation_id"`
	HostID        string    `json:"host_id"`
	RepositoryID  string    `json:"repository_id"`

	JobID                  string              `json:"job_id,omitempty"`
	JobDate                string              `json:"job_date,omitempty"`
	Timezone               string              `json:"timezone,omitempty"`
	InstallationGeneration string              `json:"installation_generation,omitempty"`
	ConfigCommit           string              `json:"config_commit,omitempty"`
	Route                  *Route              `json:"route,omitempty"`
	Launch                 *Route              `json:"launch,omitempty"`
	Claim                  *Claim              `json:"claim,omitempty"`
	Usage                  *Usage              `json:"usage,omitempty"`
	Outcome                Outcome             `json:"outcome,omitempty"`
	Changed                *bool               `json:"changed,omitempty"`
	OwnedEntries           *int                `json:"owned_entries,omitempty"`
	ElapsedMS              *int64              `json:"elapsed_ms,omitempty"`
	ExitCode               *int                `json:"exit_code,omitempty"`
	CauseCode              string              `json:"cause_code,omitempty"`
	Cause                  string              `json:"cause,omitempty"`
	CauseTruncated         bool                `json:"cause_truncated,omitempty"`
	Delivery               *Delivery           `json:"delivery,omitempty"`
	Summary                string              `json:"summary,omitempty"`
	SummaryTruncated       bool                `json:"summary_truncated,omitempty"`
	References             []Reference         `json:"artifact_references,omitempty"`
	ReferencesTruncated    bool                `json:"references_truncated,omitempty"`
	TargetVerification     *TargetVerification `json:"target_verification,omitempty"`
	HistoricalTailBytes    *int64              `json:"historical_tail_bytes,omitempty"`
}

type Durability uint8

const (
	Buffered Durability = iota
	Consequential
)

type Result struct {
	HistoricalTailIncomplete bool
}
