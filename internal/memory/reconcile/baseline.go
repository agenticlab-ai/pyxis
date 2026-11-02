package reconcile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/sessionlog"
)

const (
	BaselineSchema       = "memory-reconciliation-baseline/v1"
	BaselineResultSchema = "memory-reconciliation-ratchet/v1"
)

// BaselineAllowance is the exact current occurrence count and severity for one
// stable finding address. Detail and remedy are diagnostic prose rather than
// debt identity, so wording improvements do not churn the baseline.
type BaselineAllowance struct {
	Code     string `json:"code"`
	Scope    string `json:"scope"`
	Feature  string `json:"feature,omitempty"`
	Count    int    `json:"count"`
	Blocking bool   `json:"blocking"`
	Writable bool   `json:"writable"`
}

// Baseline is a checked-in debt inventory produced from one exact report. The
// content identities must stay equal to the current reconciler inputs.
type Baseline struct {
	Schema              string              `json:"schema_version"`
	Generator           string              `json:"generator"`
	SourceClosureSHA256 string              `json:"source_closure_sha256"`
	CodeInventorySHA256 string              `json:"code_inventory_sha256"`
	DesignIndexSHA256   string              `json:"design_index_sha256,omitempty"`
	MemoryClosureSHA256 string              `json:"memory_closure_sha256"`
	FindingCount        int                 `json:"finding_count"`
	Allowances          []BaselineAllowance `json:"allowances"`
}

// BaselineDelta explains one changed debt inventory. Regressions fail the
// ratchet; improvements require the checked-in baseline to be tightened in
// the same change, so resolved debt cannot silently be reintroduced later.
type BaselineDelta struct {
	Code             string `json:"code"`
	Scope            string `json:"scope"`
	Feature          string `json:"feature,omitempty"`
	Reason           string `json:"reason"`
	BaselineCount    int    `json:"baseline_count"`
	CurrentCount     int    `json:"current_count"`
	BaselineBlocking bool   `json:"baseline_blocking"`
	CurrentBlocking  bool   `json:"current_blocking"`
	BaselineWritable bool   `json:"baseline_writable"`
	CurrentWritable  bool   `json:"current_writable"`
}

// BaselineResult is the deterministic comparison between a report and its
// checked-in exact debt inventory.
type BaselineResult struct {
	Schema        string          `json:"schema_version"`
	BaselineCount int             `json:"baseline_findings"`
	CurrentCount  int             `json:"current_findings"`
	Regressions   []BaselineDelta `json:"regressions"`
	Improvements  []BaselineDelta `json:"improvements"`
}

// Blocking reports whether current reconciliation debt regresses from the
// baseline. Improvements are reported separately because the CLI requires an
// exact candidate inventory and asks the author to tighten it immediately.
func (r BaselineResult) Blocking() bool { return len(r.Regressions) > 0 }

// Exact reports whether two debt inventories have identical addresses,
// multiplicities, severities, and deterministic-repair classifications.
func (r BaselineResult) Exact() bool { return len(r.Regressions) == 0 && len(r.Improvements) == 0 }

// NewBaseline records the exact finding-address inventory from report. Duplicate
// findings at the same address become a count, with the strictest severity and
// least writable classification retained.
func NewBaseline(report Report) Baseline {
	allowances := allowancesFor(report.Findings)
	return Baseline{
		Schema: BaselineSchema, Generator: report.Generator,
		SourceClosureSHA256: report.SourceClosureSHA256,
		CodeInventorySHA256: report.CodeInventorySHA256, DesignIndexSHA256: report.DesignIndexSHA256,
		MemoryClosureSHA256: report.MemoryClosureSHA256,
		FindingCount:        len(report.Findings), Allowances: allowances,
	}
}

// MarshalBaseline emits the single canonical checked-in representation. The
// baseline must already be structurally valid; CompareBaselines separately
// enforces monotonicity against the trusted prior file.
func MarshalBaseline(baseline Baseline) ([]byte, error) {
	if err := validateBaseline(baseline); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(baseline); err != nil {
		return nil, fmt.Errorf("encode reconciliation baseline: %w", err)
	}
	return out.Bytes(), nil
}

// ParseBaseline decodes the policy closed: unknown fields, trailing JSON,
// malformed counts, duplicates, and non-canonical ordering are all
// configuration errors rather than silently widened allowances. Older
// non-empty generator identifiers remain readable so a candidate can prove a
// monotonic transition from its trusted predecessor; live identity matching
// still requires the current generator.
func ParseBaseline(body []byte) (Baseline, error) {
	var baseline Baseline
	if err := rejectDuplicateJSONKeys(body); err != nil {
		return Baseline{}, fmt.Errorf("decode reconciliation baseline: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&baseline); err != nil {
		return Baseline{}, fmt.Errorf("decode reconciliation baseline: %w", err)
	}
	if err := consumeBaselineJSONEOF(dec); err != nil {
		return Baseline{}, err
	}
	if err := validateBaseline(baseline); err != nil {
		return Baseline{}, err
	}
	return baseline, nil
}

func rejectDuplicateJSONKeys(body []byte) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	return scanJSONValueKeys(dec, "$", 0, baselineObjectKeys)
}

func scanJSONValueKeys(dec *json.Decoder, location string, depth int, objectKeys func(string) map[string]bool) error {
	if depth > 16 {
		return fmt.Errorf("JSON nesting exceeds the reconciliation baseline limit")
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, container := token.(json.Delim)
	if !container {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		allowed := objectKeys(location)
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object member at %s is not a string", location)
			}
			if seen[key] {
				return fmt.Errorf("duplicate object member %q at %s", key, location)
			}
			if allowed != nil && !allowed[key] {
				return fmt.Errorf("unknown field %q at %s", key, location)
			}
			seen[key] = true
			if err := scanJSONValueKeys(dec, location+"."+key, depth+1, objectKeys); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	case '[':
		index := 0
		for dec.More() {
			if err := scanJSONValueKeys(dec, fmt.Sprintf("%s[%d]", location, index), depth+1, objectKeys); err != nil {
				return err
			}
			index++
		}
		_, err = dec.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q at %s", delim, location)
	}
}

func baselineObjectKeys(location string) map[string]bool {
	if location == "$" {
		return map[string]bool{
			"schema_version": true, "generator": true, "source_closure_sha256": true,
			"code_inventory_sha256": true, "design_index_sha256": true, "memory_closure_sha256": true,
			"finding_count": true, "allowances": true,
		}
	}
	if strings.HasPrefix(location, "$.allowances[") && strings.HasSuffix(location, "]") {
		return map[string]bool{
			"code": true, "scope": true, "feature": true, "count": true,
			"blocking": true, "writable": true,
		}
	}
	return nil
}

func validateBaseline(baseline Baseline) error {
	if baseline.Schema != BaselineSchema {
		return fmt.Errorf("reconciliation baseline schema_version = %q, want %q", baseline.Schema, BaselineSchema)
	}
	if baseline.Generator == "" || strings.ContainsAny(baseline.Generator, "\x00\r\n") {
		return fmt.Errorf("reconciliation baseline generator must be a non-empty single-line identifier")
	}
	if !validDigest(baseline.SourceClosureSHA256) || !validDigest(baseline.CodeInventorySHA256) || (baseline.DesignIndexSHA256 != "" && !validDigest(baseline.DesignIndexSHA256)) || !validDigest(baseline.MemoryClosureSHA256) {
		return fmt.Errorf("reconciliation baseline input identities must be lowercase SHA-256 values")
	}
	if baseline.FindingCount < 0 {
		return fmt.Errorf("reconciliation baseline finding_count must be non-negative")
	}
	if baseline.Allowances == nil {
		return fmt.Errorf("reconciliation baseline allowances must be a JSON array")
	}
	total := 0
	for i, allowance := range baseline.Allowances {
		if allowance.Code == "" || allowance.Scope == "" || allowance.Count < 1 {
			return fmt.Errorf("reconciliation baseline allowance %d has an empty address or non-positive count", i)
		}
		if strings.ContainsRune(allowance.Code, '\x00') || strings.ContainsRune(allowance.Scope, '\x00') || strings.ContainsRune(allowance.Feature, '\x00') {
			return fmt.Errorf("reconciliation baseline allowance %d address contains a NUL byte", i)
		}
		if i > 0 {
			cmp := compareAllowanceAddress(baseline.Allowances[i-1], allowance)
			if cmp == 0 {
				return fmt.Errorf("reconciliation baseline repeats allowance %s at %s", allowance.Code, allowance.Scope)
			}
			if cmp > 0 {
				return fmt.Errorf("reconciliation baseline allowances are not in canonical order at %s %s", allowance.Code, allowance.Scope)
			}
		}
		if allowance.Count > baseline.FindingCount-total {
			return fmt.Errorf("reconciliation baseline allowances exceed finding_count %d", baseline.FindingCount)
		}
		total += allowance.Count
	}
	if total != baseline.FindingCount {
		return fmt.Errorf("reconciliation baseline finding_count = %d, allowances total %d", baseline.FindingCount, total)
	}
	return nil
}

func consumeBaselineJSONEOF(dec *json.Decoder) error {
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode reconciliation baseline: trailing JSON value")
		}
		return fmt.Errorf("decode reconciliation baseline: %w", err)
	}
	return nil
}

// CompareBaseline compares a live report with the checked-in debt inventory.
// The command requires this comparison to be exact. The regression/improvement
// split makes a stale inventory actionable and is also reused to prove that a
// candidate baseline only tightens its trusted predecessor.
func CompareBaseline(baseline Baseline, report Report) BaselineResult {
	return compareBaselineAllowances(baseline.FindingCount, baseline.Allowances, len(report.Findings), allowancesFor(report.Findings))
}

// BaselineIdentitiesMatch reports whether the baseline was generated from the
// same authored-source, code, design, and memory inventories as report. A Git commit
// is deliberately not stored here: putting the baseline in that commit would
// make such an identity self-referential. The four content digests are the
// stable, reproducible identity of the inputs the reconciler actually reads.
func BaselineIdentitiesMatch(baseline Baseline, report Report) bool {
	return baseline.Generator == report.Generator &&
		baseline.SourceClosureSHA256 == report.SourceClosureSHA256 &&
		baseline.CodeInventorySHA256 == report.CodeInventorySHA256 &&
		baseline.DesignIndexSHA256 == report.DesignIndexSHA256 &&
		baseline.MemoryClosureSHA256 == report.MemoryClosureSHA256
}

// CompareBaselines prevents a candidate checked-in baseline from widening a
// trusted prior baseline. It uses the same monotonic rules as a live report:
// the candidate may only remove or improve recorded debt.
func CompareBaselines(prior, candidate Baseline) BaselineResult {
	return compareBaselineAllowances(prior.FindingCount, prior.Allowances, candidate.FindingCount, candidate.Allowances)
}

// SourceRetirementRegressions prevents a candidate from making any authored
// source disappear merely to improve its finding inventory. Every prior path
// must remain selected unless an exact closeout already trusted at the
// baseline ref authorizes retirement. Candidate-authored closeouts must never
// authorize their own transition.
func SourceRetirementRegressions(priorSources, currentSources []string, trustedRetirements map[string]bool) []BaselineDelta {
	current := make(map[string]bool, len(currentSources))
	for _, source := range currentSources {
		current[source] = true
	}
	var out []BaselineDelta
	for _, source := range priorSources {
		if current[source] || trustedRetirements[source] || sessionlog.Path(source) {
			continue
		}
		before := BaselineAllowance{Code: "authored-source-retirement", Scope: source, Count: 1, Blocking: true}
		out = append(out, baselineDelta(before, BaselineAllowance{}, "authored source disappeared without a closeout proven at the trusted ref"))
	}
	sortBaselineDeltas(out)
	return out
}

func compareBaselineAllowances(baselineCount int, baseline []BaselineAllowance, currentCount int, current []BaselineAllowance) BaselineResult {
	result := BaselineResult{
		Schema: BaselineResultSchema, BaselineCount: baselineCount,
		CurrentCount: currentCount, Regressions: []BaselineDelta{},
		Improvements: []BaselineDelta{},
	}
	baseByAddress := make(map[string]BaselineAllowance, len(baseline))
	currentByAddress := make(map[string]BaselineAllowance, len(current))
	for _, allowance := range baseline {
		baseByAddress[allowanceAddress(allowance)] = allowance
	}
	for _, allowance := range current {
		currentByAddress[allowanceAddress(allowance)] = allowance
	}
	for _, allowance := range current {
		key := allowanceAddress(allowance)
		base, ok := baseByAddress[key]
		if !ok {
			if baselinePhaseAdvance(baseByAddress, currentByAddress, allowance) {
				continue
			}
			result.Regressions = append(result.Regressions, baselineDelta(BaselineAllowance{}, allowance, "new finding address"))
			continue
		}
		if allowance.Count > base.Count {
			result.Regressions = append(result.Regressions, baselineDelta(base, allowance, "finding count increased"))
		}
		if allowance.Blocking && !base.Blocking {
			result.Regressions = append(result.Regressions, baselineDelta(base, allowance, "finding became blocking"))
		}
		if base.Writable && !allowance.Writable {
			result.Regressions = append(result.Regressions, baselineDelta(base, allowance, "finding lost its deterministic repair"))
		}
		if allowance.Count < base.Count {
			result.Improvements = append(result.Improvements, baselineDelta(base, allowance, "finding count decreased"))
		}
		if base.Blocking && !allowance.Blocking {
			result.Improvements = append(result.Improvements, baselineDelta(base, allowance, "finding became advisory"))
		}
		if !base.Writable && allowance.Writable {
			result.Improvements = append(result.Improvements, baselineDelta(base, allowance, "finding gained a deterministic repair"))
		}
	}
	for _, base := range baseline {
		if _, ok := currentByAddress[allowanceAddress(base)]; !ok {
			result.Improvements = append(result.Improvements, baselineDelta(base, BaselineAllowance{}, "finding resolved"))
		}
	}
	sortBaselineDeltas(result.Regressions)
	sortBaselineDeltas(result.Improvements)
	return result
}

// baselinePhaseAdvance recognizes the exact, directional states exposed by a
// deterministic backfill or a lossless generator reclassification. Capturing
// a missing PRD claim can reveal that the now-present claim lacks an authorized
// design edge; capturing the target can likewise reveal that a design Decision
// has not yet carried its declared edge. The v2 PRD projection also gives
// authored Q rows their precise open-question state instead of the v1
// unknown-unit state.
// Those are narrower states at the same source address, not new debt.
//
// The predecessor must be gone, the address and feature must be identical,
// and multiplicity may not grow. Retaining both states, reversing the
// transition, or changing any address remains a regression.
func baselinePhaseAdvance(baseline, current map[string]BaselineAllowance, next BaselineAllowance) bool {
	previousCode := ""
	switch next.Code {
	case "unserved-prd-memory":
		previousCode = "missing-prd-memory"
	case "missing-serves-memory":
		previousCode = "missing-design-target-memory"
	case "open-prd-question":
		previousCode = "unmapped-prd-unit"
	default:
		return false
	}
	previousAddress := BaselineAllowance{Code: previousCode, Scope: next.Scope, Feature: next.Feature}
	previous, ok := baseline[allowanceAddress(previousAddress)]
	if !ok || current[allowanceAddress(previousAddress)].Code != "" {
		return false
	}
	return next.Count <= previous.Count
}

func allowancesFor(findings []Finding) []BaselineAllowance {
	byAddress := map[string]BaselineAllowance{}
	for _, finding := range findings {
		allowance := BaselineAllowance{
			Code: finding.Code, Scope: finding.Scope, Feature: finding.Feature,
			Count: 1, Blocking: finding.Blocking, Writable: finding.Writable,
		}
		key := allowanceAddress(allowance)
		if prior, ok := byAddress[key]; ok {
			prior.Count++
			prior.Blocking = prior.Blocking || allowance.Blocking
			prior.Writable = prior.Writable && allowance.Writable
			byAddress[key] = prior
			continue
		}
		byAddress[key] = allowance
	}
	out := make([]BaselineAllowance, 0, len(byAddress))
	for _, allowance := range byAddress {
		out = append(out, allowance)
	}
	sort.Slice(out, func(i, j int) bool { return compareAllowanceAddress(out[i], out[j]) < 0 })
	return out
}

func allowanceAddress(a BaselineAllowance) string {
	return a.Code + "\x00" + a.Scope + "\x00" + a.Feature
}

func compareAllowanceAddress(a, b BaselineAllowance) int {
	if a.Code != b.Code {
		if a.Code < b.Code {
			return -1
		}
		return 1
	}
	if a.Scope != b.Scope {
		if a.Scope < b.Scope {
			return -1
		}
		return 1
	}
	if a.Feature < b.Feature {
		return -1
	}
	if a.Feature > b.Feature {
		return 1
	}
	return 0
}

func baselineDelta(base, current BaselineAllowance, reason string) BaselineDelta {
	address := current
	if address.Code == "" {
		address = base
	}
	return BaselineDelta{
		Code: address.Code, Scope: address.Scope, Feature: address.Feature, Reason: reason,
		BaselineCount: base.Count, CurrentCount: current.Count,
		BaselineBlocking: base.Blocking, CurrentBlocking: current.Blocking,
		BaselineWritable: base.Writable, CurrentWritable: current.Writable,
	}
}

func sortBaselineDeltas(deltas []BaselineDelta) {
	sort.Slice(deltas, func(i, j int) bool {
		a := BaselineAllowance{Code: deltas[i].Code, Scope: deltas[i].Scope, Feature: deltas[i].Feature}
		b := BaselineAllowance{Code: deltas[j].Code, Scope: deltas[j].Scope, Feature: deltas[j].Feature}
		if cmp := compareAllowanceAddress(a, b); cmp != 0 {
			return cmp < 0
		}
		return deltas[i].Reason < deltas[j].Reason
	})
}
