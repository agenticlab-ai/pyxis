// Package schema holds the closed registries docgate parses against:
// ID namespaces, table column schemas, the amendment envelope, evidence
// classes, and the op grammar — all derived from the three templates and
// pinned to their versions (docs/tech-design/docgate-v1.md §3.1, §3.3).
// Unknown anything is a defect, never a lenient parse (DINV-2).
package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ToolVersion is recorded in generated artifacts and never hashed into the
// content fingerprint (design §3.6).
const ToolVersion = "docgate/v1"

// Template version pins this build parses against.
const (
	PRDTemplateVersion       = "prd-template/v1"
	PRDTemplateV2            = "prd-template/v2"
	AmendmentTemplateVersion = "amendment-template/v1"
	AmendmentTemplateV2      = "amendment-template/v2"
)

// Argus ledger class template versions this build parses against
// (docs/tech-design/argus-ledger-lint.md §3.4; argus-artifacts §3.1/§3.2).
// The parse is closed: a ledger whose declared version is not pinned here is
// rejected, never parsed leniently against a guessed shape (DINV-2).
const (
	ProblemLedgerVersion  = "problem-ledger/v1"
	DeliveryLedgerVersion = "delivery-ledger/v1"
)

// LedgerAnchorRe matches an Argus ledger entry heading — `### A-<n> — <gist>`
// (a problem ledger's authority entry) or `### E-<n> — <gist>` (a delivery
// ledger's journal entry) — capturing the family letter and dense number.
var LedgerAnchorRe = regexp.MustCompile(`^###\s+([AE])-(\d+)\b`)

// LedgerRegionRe matches a top-level `## <title>` region heading — the
// section-heading delimiter both ledger classes are partitioned by.
var LedgerRegionRe = regexp.MustCompile(`^##\s+(.+?)\s*$`)

// LedgerClass is the version-pinned parse shape of one Argus ledger class:
// which template version it declares, the entry-anchor family letter, the
// `## ` region whose body carries the entries, and — for the delivery
// ledger — its regenerable derived-state region (empty for the problem
// ledger, which has none). Closed-parse data (docgate-v2 "class rules as
// registries in schema").
type LedgerClass struct {
	Name            string // problem-ledger | delivery-ledger
	TemplateVersion string // the pinned version the class declares
	AnchorFamily    string // A | E — the entry-anchor letter
	EntryRegion     string // the `## ` region heading whose body holds entries
	DerivedRegion   string // the regenerable `## ` region, or "" if the class has none
}

// LedgerClasses is the closed registry of the two Argus ledger classes the
// arms read (the authority contract carries no A-n/E-n anchors and is out of
// scope — argus-ledger-lint §3.1). Keyed by class name.
var LedgerClasses = map[string]LedgerClass{
	"problem-ledger": {
		Name:            "problem-ledger",
		TemplateVersion: ProblemLedgerVersion,
		AnchorFamily:    "A",
		EntryRegion:     "Authority entries",
		DerivedRegion:   "",
	},
	"delivery-ledger": {
		Name:            "delivery-ledger",
		TemplateVersion: DeliveryLedgerVersion,
		AnchorFamily:    "E",
		EntryRegion:     "Journal",
		DerivedRegion:   "Derived state",
	},
}

// Defect is the one failure type everywhere: fail closed, name the fix.
type Defect struct {
	Verb   string `json:"verb"`
	File   string `json:"file"`
	Op     string `json:"op"`
	Reason string `json:"reason"`
	Remedy string `json:"remedy"`
}

func (d Defect) String() string {
	return fmt.Sprintf("%s: %s: %s — %s (remedy: %s)", d.Verb, d.File, d.Op, d.Reason, d.Remedy)
}

// Prefixes is the global per-PRD ID namespace (prd-template/v1 Appendix A).
var Prefixes = []string{"ABUSE", "DOD", "EVAL", "EVT", "FR", "NFR", "NG", "NR", "UB", "WR", "F", "G", "J", "M", "Q", "R"}

// idPattern matches a unit ID at the start of a table cell, longest prefix
// first (FR before F, NR/NG before N-anything ambiguity — order matters).
var idPattern = regexp.MustCompile(`^(ABUSE|DOD|EVAL|EVT|FR|NFR|NG|NR|UB|WR|G|J|M|Q|R|F)-?(\d+)\b`)

// ParseID extracts a unit ID token like "FR-3", "Q1", "M1" from a cell,
// returning the canonical ID, its prefix, and ok.
func ParseID(cell string) (id, prefix string, ok bool) {
	m := idPattern.FindStringSubmatch(strings.TrimSpace(stripEmphasis(cell)))
	if m == nil {
		return "", "", false
	}
	sep := ""
	if strings.Contains(m[0], "-") {
		sep = "-"
	}
	return m[1] + sep + m[2], m[1], true
}

func stripEmphasis(s string) string {
	return strings.TrimLeft(s, "*_ ")
}

// TableSchemas maps an ID prefix to the exact column header row its table
// carries in prd-template/v1. Order and names must match exactly (UB-2).
var TableSchemas = map[string][]string{
	"FR":    {"ID", "Requirement (EARS)", "Rationale", "Priority", "Risk", "Fit criterion", "Verify", "Traces to"},
	"NR":    {"ID", "Existing behavior protected", "Why protected", "Risk", "Fit criterion", "Verify", "Traces to"},
	"UB":    {"ID", "If… (trigger)", "Then the system shall…", "User sees", "Priority", "Risk", "Fit criterion", "Verify", "Traces to"},
	"ABUSE": {"ID", "Actor and intent", "Vector", "Impact", "Severity", "Mitigated by", "Verify"},
	"NFR":   {"ID", "Area", "Requirement", "Fit criterion", "Verify"},
	"EVT":   {"ID", "Event name", "Fires when", "Properties (typed)", "Source", "Feeds"},
	"DOD":   {"ID", "Done criterion", "Source", "Verify", "Required evidence", "Blocking?"},
	"EVAL":  {"ID", "Behavior", "Eval set / harness", "Threshold", "Negative examples"},
	"NG":    {"ID", "Non-goal", "Why refused"},
	"WR":    {"ID", "Waiting room", "Revisit when / intended version"},
	"Q":     {"#", "Question", "Owner", "Needed by", "Status"},
	"R":     {"#", "Risk / assumption / rabbit hole", "Likelihood", "Impact", "Mitigation or test", "Owner"},
	"G":     {"#", "We believe that…", "Will result in…", "Measured by…"},
	"J":     {"#", "When… (situation)", "I want to… (motivation)", "So I can… (outcome)", "Today's workaround", "Evidence"},
	"M":     {"Metric", "Goal", "Signal", "Baseline", "Target (at checkpoint)", "Feeds from"},
}

// BaseHeaderFields is the closed field set of a prd-template/v1 base header.
var BaseHeaderFields = map[string]bool{
	"In one line": true, "Relevant-when": true, "Status": true, "Tier": true,
	"Owner": true, "Approver(s)": true, "Contributors": true, "Informed": true,
	"Date": true, "Appetite": true, "Target release": true, "Tracking": true,
	"Artifacts": true, "Template version": true, "Design coverage": true,
}

// EnvelopeFields is the closed, exact amendment envelope (amendment-template/v1).
// Every supported version REQUIRES these seven; later versions add optional fields.
var EnvelopeFields = []string{"Amends", "Status", "Owner", "Date", "Learned-from", "Blast radius", "Template version"}

// EnvelopeOptionalFields maps a declared template version to the fields it
// permits beyond the required seven (FR-24: validation keys off the
// DECLARED version — merged v1 amendments are grandfathered forever).
var EnvelopeOptionalFields = map[string][]string{
	AmendmentTemplateVersion: {},
	AmendmentTemplateV2:      {"Tracking"},
}

// Incident registry schema (incident-entry-template/v1) — one definition
// consumed by the lint (FR-25) and the writer (prd:incident-machinery FR-1..3).
var (
	// IncidentIDRe matches an entry head: **I<n> (YYYY-MM-DD) [tags]**.
	IncidentIDRe = regexp.MustCompile(`^\*\*I(\d+) \((\d{4}-\d{2}-\d{2})\) \[([^\]]+)\]\*\*`)
	// IncidentTagRe is one kebab-case tag token.
	IncidentTagRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// IncidentFields are the four named fields, each a `- **<Field>:**`-anchored line.
var IncidentFields = []string{"Decision", "Broke", "Lesson", "Links"}

// CorpusIDRe extracts baseline requirement IDs from the flat placeholder
// corpus (the eleven families; FR-20/FR-26 share this one extractor).
var CorpusIDRe = regexp.MustCompile(`\b(SCAF|PLAN|IMPL|GATE|FLEET|IDEN|GOV|KNOW|OBS|BENCH|VNEXT)-[0-9]+\b`)

// EvidenceClasses is the closed Learned-from vocabulary.
var EvidenceClasses = map[string]bool{
	"incident": true, "review-thread": true, "telemetry-query": true,
	"build-result": true, "user-research": true, "support-ticket": true,
	"operator-decision": true, "dependency-change": true, "policy-change": true,
}

// Op verbs (amendment-template/v1 grammar).
const (
	OpAdd       = "add"
	OpSupersede = "supersede"
	OpRetire    = "retire"
	OpAnswer    = "answer"
	OpRescope   = "rescope"
)

// Blast radius tiers, ordered.
var TierRank = map[string]int{"additive": 0, "corrective": 1, "contract-changing": 2}

// Normalize applies the design §3.6 rules: CRLF/CR → LF, strip trailing
// spaces and tabs per line, exactly one final newline.
func Normalize(b []byte) []byte {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	s = strings.Join(lines, "\n")
	s = strings.TrimRight(s, "\n") + "\n"
	return []byte(s)
}

// Fingerprint is sha256 over the normalized replayed entries joined with a
// \x00 separator between entries (single entry: no separator).
func Fingerprint(normalized [][]byte) string {
	h := sha256.New()
	for i, b := range normalized {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write(b)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// SortIDs orders unit IDs stably: by prefix (alphabetical) then number.
func SortIDs(ids []string) {
	sort.Slice(ids, func(a, b int) bool {
		ia, pa, _ := ParseID(ids[a])
		ib, pb, _ := ParseID(ids[b])
		if pa != pb {
			return pa < pb
		}
		var na, nb int
		if _, err := fmt.Sscanf(strings.TrimPrefix(strings.TrimPrefix(ia, pa), "-"), "%d", &na); err != nil {
			na = 0 // Preserve the historical zero rank for unparseable IDs.
		}
		if _, err := fmt.Sscanf(strings.TrimPrefix(strings.TrimPrefix(ib, pb), "-"), "%d", &nb); err != nil {
			nb = 0
		}
		return na < nb
	})
}
