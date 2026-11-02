// Package reconcile compares the repository's authored sources and production
// code with the immutable records in memory. The compiler is deliberately
// offline: callers hand it one pinned git snapshot and it performs no network
// or inference work.
package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

const (
	ReportSchema   = "memory-reconciliation/v1"
	Generator      = "memory-reconcile/v7"
	CloseoutSchema = "memory-closeout/v1"
)

// Options are the configurable repository paths needed by the compiler.
type Options struct {
	ProductVision           string
	ArchitecturePrinciples  string
	Incidents               string
	PRDs                    string
	TechDesigns             string
	Plans                   string
	Brainstorms             string
	Problems                string
	Reviews                 string
	Feedback                string
	ExtraSources            []string
	LegacySessionLogs       bool // trusted-baseline reproduction only: include v1 session-log sources
	LegacyCodeInventory     bool // trusted-baseline reproduction only: use the pre-zsh v1/v2 inventory
	LegacyContractCoverage  bool // trusted-baseline reproduction only: classify incomplete coverage as v3 debt
	LegacySourceRetirement  bool // trusted-baseline reproduction only: preserve the pre-v5 source obligation rules
	LegacyNativeServes      bool // trusted-base v5 or earlier: native links have not yet gained authority
	LegacyNativeEvaluator   bool // historical v5 replay reports its original evaluator identity
	LegacyCodeCoverage      bool // trusted v1-v6 replay only: mandatory anchors and per-unit semantic coverage
	Grandfathered           []string
	PrecedenceGrandfathered []string
}

// File is one regular file from the pinned source tree.
type File struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
	Body   []byte `json:"-"`
}

// PRDUnit is one row after the PRD ledger compiler has replayed every
// amendment. A mapped unit has an exact memory kind and field projection;
// intentionally contextual rows remain mapped=false and are dispositioned by
// Analyze rather than guessed into a record kind.
type PRDUnit struct {
	ID           string  `json:"id"`
	Prefix       string  `json:"prefix"`
	Status       string  `json:"status"`
	IntroducedBy string  `json:"introduced_by"`
	Supersedes   *string `json:"supersedes"`
	SupersededBy *string `json:"superseded_by"`
	Kind         string  `json:"kind,omitempty"`
	Statement    string  `json:"statement,omitempty"`
	Intent       string  `json:"intent,omitempty"`
	Rationale    string  `json:"rationale,omitempty"`
	FitCriterion string  `json:"fit_criterion,omitempty"`
	Priority     string  `json:"priority,omitempty"`
	Consequence  string  `json:"consequence,omitempty"`
	SourceSHA256 string  `json:"source_sha256"`
	// LegacySourceSHA256s use the original PRD-unit digest schema emitted by
	// generator v1 and the session-log-retirement v2 for this exact semantic row
	// at any replay prefix retained by the current ledger. That bounded history
	// lets readers recognize an earlier record without accepting an arbitrary
	// digest. New records use the PRD-unit v2 digest schema under generator v3.
	LegacySourceSHA256s []string `json:"legacy_source_sha256s,omitempty"`
	Columns             []string `json:"columns"`
	Cells               []string `json:"cells"`
	Mapped              bool     `json:"mapped"`
}

// PRD is one fully replayed feature ledger.
type PRD struct {
	Feature           string    `json:"feature"`
	Source            string    `json:"source"`
	AsOf              string    `json:"as_of"`
	Fingerprint       string    `json:"content_fingerprint"`
	SourceMapSHA256   string    `json:"source_map_sha256"`
	RequirementsFirst bool      `json:"requirements_first,omitempty"`
	Units             []PRDUnit `json:"units"`
}

// Design is the liveness and declared relationship face of one technical
// design, read from docgate's generated index at the same pinned commit.
type Design struct {
	Slug             string   `json:"slug"`
	Source           string   `json:"source"`
	SHA256           string   `json:"sha256"`
	Status           string   `json:"status"`
	CurrentSuccessor string   `json:"current_successor,omitempty"`
	BuildsOn         []string `json:"builds_on"`
	Serves           []string `json:"serves"`
	Flags            []string `json:"flags"`
}

// Snapshot is every deterministic input to one reconciliation pass.
type Snapshot struct {
	SourceRetirements   map[string]SourceRetirementProof `json:"-"`
	Commit              string                           `json:"source_commit"`
	SourceClosureSHA256 string                           `json:"source_closure_sha256"`
	DesignIndexSHA256   string                           `json:"design_index_sha256,omitempty"`
	CodeInventorySHA256 string                           `json:"code_inventory_sha256"`
	Files               map[string]File                  `json:"-"`
	ExistingPaths       map[string]bool                  `json:"-"`
	PRDFeatures         []string                         `json:"-"`
	TrustedPRDFeatures  map[string]bool                  `json:"-"` // ledger features at the trusted base
	PRDSourceMapUnits   map[string]map[string]string     `json:"-"`
	CompilerFindings    []Finding                        `json:"-"`
	SourceOptions       Options                          `json:"-"`
	AuthoredSources     []string                         `json:"authored_sources"`
	Code                []codeinventory.Unit             `json:"code"`
	PRDs                []PRD                            `json:"prds"`
	Designs             []Design                         `json:"designs"`
	ContractHistory     *ContractHistory                 `json:"-"`
}

// Finding is one exact gap. Code is stable for automation; Scope names the
// source address or code path a human can repair.
type Finding struct {
	Code     string `json:"code"`
	Scope    string `json:"scope"`
	Feature  string `json:"feature,omitempty"`
	Detail   string `json:"detail"`
	Remedy   string `json:"remedy"`
	Writable bool   `json:"writable"`
	Blocking bool   `json:"blocking"`
}

// Draft is a record without issuance fields. The CLI fills the immutable id,
// issuer and timestamp under the repository-wide memory write lock.
type Draft struct {
	Kind         string
	Subject      string
	Family       string
	Statement    string
	Intent       string
	Rationale    string
	FitCriterion string
	Tags         []string
	Consequence  string
	Relations    []record.Relation
	Provenance   record.Provenance
	LiteralBody  bool
	Body         []byte
}

// Action is a safe append the deterministic compiler can perform without
// inventing product meaning.
type Action struct {
	Code       string `json:"code"`
	Scope      string `json:"scope"`
	BodySHA256 string `json:"body_sha256,omitempty"`
	Draft      Draft  `json:"-"`
}

// Counts summarizes a report without hiding its exact findings/actions.
type Counts struct {
	Sources    int `json:"sources"`
	Code       int `json:"code_units"`
	PRDUnits   int `json:"prd_units"`
	Designs    int `json:"designs"`
	Findings   int `json:"findings"`
	Blocking   int `json:"blocking"`
	Writable   int `json:"writable"`
	Actions    int `json:"actions"`
	Completion int `json:"completion"`
}

// Report is the canonical result of one pass.
type Report struct {
	Schema                 string              `json:"schema_version"`
	Generator              string              `json:"generator"`
	SourceCommit           string              `json:"source_commit"`
	SourceClosureSHA256    string              `json:"source_closure_sha256"`
	CodeInventorySHA256    string              `json:"code_inventory_sha256"`
	DesignIndexSHA256      string              `json:"design_index_sha256,omitempty"`
	MemoryClosureSHA256    string              `json:"memory_closure_sha256"`
	Counts                 Counts              `json:"counts"`
	Findings               []Finding           `json:"findings"`
	Actions                []Action            `json:"actions"`
	DeferredDesignFeatures []string            `json:"deferred_design_features,omitempty"`
	Completion             *ContractCompletion `json:"completion,omitempty"`
}

// Blocking reports whether the pass found any integrity failure.
func (r Report) Blocking() bool { return r.Counts.Blocking > 0 }

func sortReport(r *Report) {
	sort.Strings(r.DeferredDesignFeatures)
	sort.Slice(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		return a.Detail < b.Detail
	})
	sort.Slice(r.Actions, func(i, j int) bool {
		if r.Actions[i].Code != r.Actions[j].Code {
			return r.Actions[i].Code < r.Actions[j].Code
		}
		return r.Actions[i].Scope < r.Actions[j].Scope
	})
}

func digestJSON(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
