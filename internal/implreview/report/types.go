// Package report is the closed, versioned impl-review-report/v1 payload type
// and its validation — the machine-native record one impl-review carries
// (docs/tech-design/impl-review-v1.md §3.1, §3.7). One payload holds a whole
// review: the finding set (grammar-compatible with codegate's Finding on the
// overlapping wire keys, NOT a Go import of it — DINV-6/NR-1), the
// attribution-bound dispositions, the append-only per-round record with each
// round's reviewed revision, and the envelope's band + downgrade-only verdict.
//
// The payload is the single source of truth every consumer reads: the harvest,
// the human rendering (a pure function of the payload — DINV-2), and any
// downstream reader (AP-21). It parses CLOSED — an unknown JSON key is an
// error, never a lenient decode (AP-6; Unmarshal below). The validation realizes
// DINV-1/2/3/7 (plus the decline-reason and at-least-one-round rules) with a
// named sentinel per rule so a caller can tell which rule failed with errors.Is.
//
// This package imports NO codegate struct: the grammar compatibility is a
// reflection assertion in the test (grammar_compat_test.go), which pins the
// overlapping keys against codegate's current Finding without coupling the two
// layouts. Stdlib only, per the repo Go rules.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// SchemaVersion is the one payload shape this build parses and emits. A report
// declaring any other version is invalid, never validated leniently. Additive
// evolution stays within v1; a breaking change is v2 by supersession (§3.1).
const SchemaVersion = "impl-review-report/v1"

// Finding is impl-review's OWN finding type. Its overlapping fields are
// byte-compatible with codegate's schema.Finding grammar — the same JSON keys
// with the same meaning (`path`, `line`, `message`, `log_ref`) — so the host's
// fixer parses codegate and impl-review findings as ONE grammar family (NR-1,
// DINV-6). The compatibility is a wire contract asserted by a reflection test,
// NOT a struct import: codegate's `check_id`/`class`/`rung` are deliberately
// absent (impl-review has no rungs, and `rule_id`/`lens` are its own identity),
// so there is no dead field and zero codegate change by construction.
//
// The review-only fields carry the identity, severity, calibration citation,
// acceptance, and disposition the reviewer contract needs; `rule_id` is its OWN
// field and never an overload of codegate's `check_id`.
type Finding struct {
	// ID is the dense FND-n anchor within a report (cited
	// `impl-review:<proposal-ref>#FND-3`).
	ID string `json:"id"`

	// --- codegate-grammar-compatible fields (byte-identical keys) ---
	// Path is the file the finding is against (a repo-relative path).
	Path string `json:"path,omitempty"`
	// Line is the 1-based line within Path, or 0 when the finding is file-level.
	Line int `json:"line,omitempty"`
	// Message is the human-readable finding text.
	Message string `json:"message"`
	// LogRef points at supporting evidence (a log/artifact reference).
	LogRef string `json:"log_ref,omitempty"`

	// --- review-only fields ---
	// RuleID is the lens+rule identity — impl-review's own field, never an
	// overload of codegate's check_id.
	RuleID string `json:"rule_id"`
	// Lens is the emitting lens (one of the four Lens* values).
	Lens string `json:"lens"`
	// Severity is blocking or advisory; it MUST equal the severity its CalRow
	// maps to (DINV-3), verified by Validate against a CalLookup.
	Severity string `json:"severity"`
	// CalRow is the calibration-row citation (FR-12) — required on every
	// finding (DINV-3).
	CalRow string `json:"cal_row"`
	// Acceptance is the acceptance the finding is anchored to.
	Acceptance string `json:"acceptance"`
	// Disposition is the recorded worker/operator disposition of the finding,
	// absent until one is made.
	Disposition *Disposition `json:"disposition,omitempty"`
}

// Lens enum — the four fresh-context lenses (§3.1). A finding's Lens is one of
// these.
const (
	LensSpecFidelity      = "spec-fidelity"
	LensDesignConformance = "design-conformance"
	LensSecurityAbuse     = "security-abuse"
	LensCorrectnessCritic = "correctness-critic"
)

// Severity enum — the two finding severities (§3.1). Advisory-only findings
// never block; a blocking finding drives request-changes.
const (
	SeverityBlocking = "blocking"
	SeverityAdvisory = "advisory"
)

// Disposition is a recorded disposition of a finding (attribution, FR-16). The
// Attribution field is the MEDIUM author of a separate disposition artifact
// (§3.2) — for an operator disposition it is the operator's own PR review /
// decision-record author, never a worker-writable claim; this package carries
// it as an opaque string and the harvest (T11) resolves it against the operator
// identity. Reason is mandatory when Verdict is VerdictDeclined (§3.1).
type Disposition struct {
	// Actor is who dispositioned — worker or operator.
	Actor string `json:"actor"`
	// Verdict is the disposition outcome — applied/declined (worker) or
	// accepted/rejected (operator).
	Verdict string `json:"verdict"`
	// Reason is mandatory on a declined disposition; Validate rejects a
	// reason-less decline.
	Reason string `json:"reason,omitempty"`
	// Attribution is the medium author of the separate disposition artifact
	// (opaque here — resolved by the harvest). Empty for an inline worker
	// disposition whose author is the payload itself.
	Attribution string `json:"attribution,omitempty"`
}

// Disposition Actor enum.
const (
	ActorWorker   = "worker"
	ActorOperator = "operator"
)

// Disposition Verdict enum. applied/declined are the worker verdicts;
// accepted/rejected are the operator verdicts.
const (
	VerdictApplied  = "applied"
	VerdictDeclined = "declined"
	VerdictAccepted = "accepted"
	VerdictRejected = "rejected"
)

// Provenance is a round's fresh-instance identity (§3.1, FR-2/NFR-3): the model
// it ran, and the prompts + calibration versions it evaluated under.
type Provenance struct {
	Model          string `json:"model"`
	PromptsVersion string `json:"prompts_version"`
	CalVersion     string `json:"cal_version"`
}

// Round is one evaluation of the proposal — the append unit of the two-round
// model (§3.1). Each round binds its OWN reviewed revision (the head sha THAT
// round evaluated), because a fix between rounds is a new sha; the live
// verdict's staleness is judged against the latest round's ReviewedRevision
// (DINV-7). A round records its fresh-instance context + provenance, its
// finding set, and the EVT candidate/killed/findings counts.
//
// A Round is append-only: a caller records a new round, never mutates a prior
// one's ReviewedRevision (plan T1 acceptance #4). This package validates a
// report's structure; the append-only enforcement over a prior persisted state
// is ValidateAppend below.
type Round struct {
	// ReviewedRevision is the head sha this round evaluated (DINV-7) — required
	// on every round.
	ReviewedRevision string `json:"reviewed_revision"`
	// ContextID is the reviewer-root-derived fresh-instance context id (§3.4).
	ContextID string `json:"context_id"`
	// Provenance is the round's model + prompts + calibration versions.
	Provenance Provenance `json:"provenance"`
	// Findings is the round's finding set.
	Findings []Finding `json:"findings"`
	// FindingsCount / Candidates / Killed are the round's EVT counts
	// (findings/candidates/killed — §3.1).
	FindingsCount int `json:"findings_count"`
	Candidates    int `json:"candidates"`
	Killed        int `json:"killed"`
}

// Band enum — the review depth, read from the lane's risk class (FR-9).
const (
	BandSkip     = "skip"
	BandStandard = "standard"
	BandDeep     = "deep"
)

// Verdict enum — the live, downgrade-only verdict set (FR-7, §3.1). No value
// grants the auto lane; the set only forces a human or bounces to the worker.
const (
	VerdictApprove               = "approve"
	VerdictApproveWithAdvisories = "approve-with-advisories"
	VerdictRequestChanges        = "request-changes"
	VerdictEscalate              = "escalate"
)

// EscalationBrief is the single-conflict decision brief, present iff the live
// verdict is VerdictEscalate (FR-17, §3.1): the one conflict, the options with
// their risks, and the decision needed — addressed to the human.
type EscalationBrief struct {
	Conflict       string   `json:"conflict"`
	Options        []string `json:"options"`
	DecisionNeeded string   `json:"decision_needed"`
}

// Report is the one impl-review-report/v1 payload per proposal per review
// (DINV-1). It carries the envelope — the schema version, the proposal
// reference, the review id, the band, and the live downgrade-only verdict — and
// the append-only Rounds. EscalationBrief is present iff Verdict is
// VerdictEscalate.
type Report struct {
	// SchemaVersion pins the payload shape; MUST equal SchemaVersion.
	SchemaVersion string `json:"schema_version"`
	// ProposalRef identifies the proposal reviewed (its medium reference).
	ProposalRef string `json:"proposal_ref"`
	// ReviewID identifies this review of that proposal; (ProposalRef, ReviewID)
	// is unique across reports (DINV-1, ValidateUnique).
	ReviewID string `json:"review_id"`
	// Band is the review depth (skip/standard/deep).
	Band string `json:"band"`
	// Verdict is the live downgrade-only verdict.
	Verdict string `json:"verdict"`
	// Rounds is the append-only per-round record; at least one is required.
	Rounds []Round `json:"rounds"`
	// EscalationBrief is present iff Verdict is VerdictEscalate.
	EscalationBrief *EscalationBrief `json:"escalation_brief,omitempty"`
}

// Unmarshal parses an impl-review-report/v1 payload from JSON, failing CLOSED on
// an unknown key (AP-6, DINV-2's closed-parse basis). It does NOT validate the
// payload's integrity rules — call Validate for DINV-1/2/3/7 — it only rejects a
// structurally-foreign document. Every producer and reader routes through this
// one parser so a payload reviewed on either medium recovers identically (§3.2).
func Unmarshal(data []byte) (*Report, error) {
	var r Report
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("impl-review-report parse: %w", err)
	}
	// A closed parse rejects unknown keys; a trailing second JSON value is a
	// malformed payload the decoder would otherwise ignore.
	if dec.More() {
		return nil, fmt.Errorf("impl-review-report parse: trailing data after the payload")
	}
	return &r, nil
}

// Marshal serializes the report to indented JSON — the on-disk/on-wire form the
// adapters (T2) persist. It is the inverse of Unmarshal on a valid payload.
func (r *Report) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("impl-review-report marshal: %w", err)
	}
	return data, nil
}
