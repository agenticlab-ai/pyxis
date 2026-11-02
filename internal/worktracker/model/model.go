// Package model is the medium-neutral work-item domain: the typed WorkItem,
// its closed value vocabularies, the append-log entry, and the create input —
// pure data with no IO (docs/tech-design/local-work-tracker.md §3.7). The
// on-disk grammar (localfs) and the CLI (cmd/carina) both project this model;
// keeping it free of file-isms is what lets the future GitHub adapter import
// the port, not Carina.
//
// The engine-recognized enum fields (type, state, close_reason, priority) draw
// their values from the closed seed vocabulary (§3.14, DINV-11): a value
// outside the set is a named defect at parse time, never a lenient guess. Only
// the classification axis (kind:/area: signals) is operand-extensible, and that
// lives with the signal vocabulary (a later task), not here.
package model

import "time"

// SchemaVersion tags the machine (JSON) projection so a consumer can pin the
// shape it reads (prd:local-work-tracker/FR-15; AP-6 — cross-boundary data is
// version-pinned). Additive evolution only.
const SchemaVersion = "worktracker.work-item/v1"

// Type is the hierarchy role — the item's place in the Feature→Milestone→Epic→
// Task DAG. Closed set (§3.14); defaults to Task on create.
type Type string

const (
	TypeFeature   Type = "feature"
	TypeMilestone Type = "milestone"
	TypeEpic      Type = "epic"
	TypeTask      Type = "task"
)

// Types is the closed hierarchy-role set, in canonical (broadest-first) order.
var Types = []Type{TypeFeature, TypeMilestone, TypeEpic, TypeTask}

// State is the core open/closed state. The rich lifecycle (triaged, ready, in
// progress…) is projected via signals + records + predicates, never this field
// (§3.14).
type State string

const (
	StateOpen   State = "open"
	StateClosed State = "closed"
)

// States is the closed state set.
var States = []State{StateOpen, StateClosed}

// CloseReason records why a closed item closed (done vs won't-do). Optional:
// the empty value is legal on an open item. Closed set (§3.14).
type CloseReason string

const (
	CloseReasonNone CloseReason = ""
	CloseCompleted  CloseReason = "completed"
	CloseRejected   CloseReason = "rejected"
)

// CloseReasons is the closed set of non-empty close reasons.
var CloseReasons = []CloseReason{CloseCompleted, CloseRejected}

// Priority is the optional, advisory user priority — carried for round-trip
// fidelity, not an engine scheduling input in v1 (§3.14, Q6). Closed set.
type Priority string

const (
	PriorityUnset  Priority = ""
	PriorityNone   Priority = "none"
	PriorityUrgent Priority = "urgent"
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// Priorities is the closed set of non-empty priorities.
var Priorities = []Priority{PriorityNone, PriorityUrgent, PriorityHigh, PriorityMedium, PriorityLow}

// LogKind is the append-log entry type (§3.14). Transitions are the first kind
// written (T2); comments and decision-records join in a later task. The kind is
// part of the closed grammar the log region parses against, so a line whose
// kind is out of set is a named defect, never a lenient parse (DINV-11).
type LogKind string

const (
	LogComment        LogKind = "comment"
	LogDecisionRecord LogKind = "decision-record"
	LogTransition     LogKind = "transition"
)

// LogKinds is the closed set of append-log entry kinds (§3.14, DINV-11).
var LogKinds = []LogKind{LogComment, LogDecisionRecord, LogTransition}

// ValidLogKind reports whether k is in the closed log-kind set.
func ValidLogKind(k LogKind) bool { return in(LogKinds, k) }

// LogEntry is one append-only log record. The log is empty in the first task;
// this is the shape later comment/record/transition behaviors append into.
type LogEntry struct {
	Kind      LogKind
	Author    string
	At        time.Time
	Namespace string // decision-record namespace, else ""
	Payload   string
}

// WorkItem is one work item — the closed-schema header, the free-markdown body,
// and the append-only log. It is the sole authoritative record of the item
// (DINV-1); the localfs adapter renders exactly this and parses back to it.
type WorkItem struct {
	ID          int
	Type        Type
	State       State
	Title       string
	Signals     []string
	Assignees   []string
	Parent      string   // owning item id as text, or "" when unparented
	BlockedBy   []string // dependency ids, incl. the opaque owner/repo#id form
	Created     time.Time
	Updated     time.Time
	CloseReason CloseReason // "" unless closed with a reason
	Priority    Priority    // "" when unset
	Body        string
	Log         []LogEntry
}

// CreateSpec is the input to Create — the fields a caller supplies; everything
// else (id, state, timestamps) the store assigns. Type defaults to Task when
// unset. The first task's create surface is deliberately small; later tasks add
// signals, parents, and dependencies as their own verbs.
type CreateSpec struct {
	Type     Type
	Title    string
	Body     string
	Priority Priority
}

// ValidType reports whether t is in the closed hierarchy-role set.
func ValidType(t Type) bool { return in(Types, t) }

// ValidState reports whether s is in the closed state set.
func ValidState(s State) bool { return in(States, s) }

// ValidCloseReason reports whether r is empty or in the closed set.
func ValidCloseReason(r CloseReason) bool { return r == CloseReasonNone || in(CloseReasons, r) }

// ValidPriority reports whether p is empty or in the closed set.
func ValidPriority(p Priority) bool { return p == PriorityUnset || in(Priorities, p) }

func in[T comparable](set []T, v T) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}
