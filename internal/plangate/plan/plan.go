// Package plan is plangate's core: the plan document class of
// docs/tech-design/plan-new-v1.md — a closed-grammar markdown source
// (`NNN-plan.md`, §3.1) parsed, gated (sizing, coverage, fog, cycles,
// overlap, charter durability — §3.6), and folded into a schema-versioned
// machine face (`NNN-plan.json`). The contract is docs/prds/plan-new/
// (the frozen PRD) + the design's DINV-1..9; every enumerable rule lives
// here as a verdict, never in a prompt (AP-10/AP-11).
package plan

import (
	"encoding/json"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
)

// Version pins recorded in generated artifacts.
const (
	ToolVersion           = "plangate/v1"
	SchemaVersion         = "plan/v1"
	PlanTemplateVersion   = "plan-template/v1"
	NativeSchemaVersion   = "plan/v2"
	NativeTemplateVersion = "plan-template/v2"
	// DialectHarbormaster is the one executor dialect v1 ships (design §3.9).
	// The grammar bytes it names live in the dialect module and its
	// fixtures, never in frozen prose.
	DialectHarbormaster = "harbormaster/fs-spec:v1"
	DialectNative       = "harbormaster/fs-spec:v2"
)

// Dialects is the closed set of known executor dialects.
var Dialects = map[string]bool{DialectHarbormaster: true, DialectNative: true}

// Patterns is the closed expand–contract vocabulary (design §3.6; PRD FR-8).
const (
	PatternExpand   = "expand"
	PatternMigrate  = "migrate-batch"
	PatternContract = "contract"
)

// Foundation levels, derived from the dependency graph at fold time
// (PRD FR-22; reasons keep the predecessor's stamp vocabulary verbatim so
// the dialect renders them unchanged).
const (
	FoundationRoot     = "high"
	FoundationInternal = "medium"
	FoundationLeaf     = "low"

	ReasonRoot     = "common-root"
	ReasonInternal = "derived-internal"
	ReasonLeaf     = "derived-leaf"
)

// SizeBand is the estimated added-logic-lines band; 0–0 on mechanical tasks.
type SizeBand struct {
	Lo int `json:"lo"`
	Hi int `json:"hi"`
}

// Mid is the band midpoint, the share-computation currency.
func (b SizeBand) Mid() int { return (b.Lo + b.Hi) / 2 }

// Foundation is the plan-time risk stamp (level + reason).
type Foundation struct {
	Level  string `json:"level"`
	Reason string `json:"reason"`
}

// Task is one vertically-sliced unit of the plan — the §4.5 task contract:
// charter fields (layer 2), plan-internal metadata (Touches — never
// rendered into the charter), and computed stamps (layer 3).
type Task struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Goal            string   `json:"goal"`
	Current         string   `json:"current,omitempty"`
	Desired         string   `json:"desired,omitempty"`
	Scope           []string `json:"scope,omitempty"`
	OutOfScope      []string `json:"out_of_scope,omitempty"`
	Acceptance      []string `json:"acceptance"`
	KeyInterfaces   []string `json:"key_interfaces,omitempty"`
	Provenance      []string `json:"provenance,omitempty"`
	Traces          []string `json:"traces"`
	BlockedBy       []string `json:"blocked_by,omitempty"`
	Touches         []string `json:"touches,omitempty"` // plan-internal: overlap sequencing only
	Band            SizeBand `json:"size_band"`
	Mechanical      bool     `json:"mechanical"`
	SizeAccepted    string   `json:"size_accepted,omitempty"`
	OverlapAccepted string   `json:"overlap_accepted,omitempty"`

	Requirements *taskrequirements.Spec `json:"requirements,omitempty"`
	Pattern      string                 `json:"pattern,omitempty"` // expand–contract role
	PlanTaskID   string                 `json:"plan_task_id"`      // <plan-name>/<NNN>/<Tn> — the idempotency key
	Foundation   Foundation             `json:"foundation"`        // computed at fold
}

// CoverageRow maps one governing requirement to the tasks serving it.
type CoverageRow struct {
	Requirement string   `json:"requirement"`
	Tasks       []string `json:"tasks"`
}

// DeferredRow records a non-Must requirement deliberately not planned —
// explicit, never silent (PRD FR-15's perimeter).
type DeferredRow struct {
	Requirement string `json:"requirement"`
	Reason      string `json:"reason"`
}

// Doc is one parsed plan document.
type Doc struct {
	Schema          string        `json:"schema_version"`
	Tool            string        `json:"generated_by"`
	Title           string        `json:"title"`
	Feature         string        `json:"feature,omitempty"`
	PRD             string        `json:"prd,omitempty"`
	Design          string        `json:"design,omitempty"`
	Basis           string        `json:"basis,omitempty"`
	Intake          string        `json:"intake,omitempty"`
	Version         string        `json:"plan_version"` // NNN, matches the filename
	Dialect         string        `json:"dialect"`
	EpicTitle       string        `json:"epic_title"`
	Status          string        `json:"status"`
	TemplateVersion string        `json:"template_version"`
	Tasks           []Task        `json:"tasks"`
	Coverage        []CoverageRow `json:"coverage"`
	Deferred        []DeferredRow `json:"deferred,omitempty"`
	Fingerprint     string        `json:"source_fingerprint"` // sha256 over the normalized source

	sourceFile string        // repo-relative, for defects
	pending    *PendingBasis // a lock being prepared; see FoldFileAt
}

// SourceFile is the repo-relative path the doc was parsed from.
func (d *Doc) SourceFile() string { return d.sourceFile }

// Name is the stable plan identity supplied by either its Feature or Intake row.
func (d *Doc) Name() string {
	if d.Feature != "" {
		return d.Feature
	}
	return d.Intake
}

// Task returns the task with the given ID, or nil.
func (d *Doc) Task(id string) *Task {
	for i := range d.Tasks {
		if d.Tasks[i].ID == id {
			return &d.Tasks[i]
		}
	}
	return nil
}

// JSON renders the canonical machine face — deterministic bytes: struct
// order, two-space indent, one trailing newline (verify byte-compares it).
func (d *Doc) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal plan: %w", err)
	}
	return append(b, '\n'), nil
}

// def builds this package's uniform defect (fail closed, name the fix).
func def(file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "plangate", File: file, Op: op, Reason: reason, Remedy: remedy}
}
