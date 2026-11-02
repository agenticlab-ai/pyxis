// This file owns the maintenance sweep's report contract (maintenance-sweep-v1
// §3.3): the closed, versioned machine-face schema the pass emits — one report
// per invocation carrying the evaluated-state pin, the outcome, and one result
// per detector in fixed order, each with its findings, applied waivers, and
// in-band fault. Every finding carries the four contracted fields (detector,
// subject, evidence, remedy — DINV-5); a finding missing any refuses both
// construction and marshalling. Marshalling is byte-deterministic for equal
// input: detectors in fixed order, subjects lexicographic, no wall-clock field
// anywhere on the machine face (DINV-2). Stdlib only.
package sweep

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ReportSchemaVersion and FindingSchemaVersion pin the two closed machine-face
// schemas (maintenance-sweep-v1 §3.3); evolution is additive.
const (
	ReportSchemaVersion  = "maintenance-report/v1"
	FindingSchemaVersion = "finding/v1"
)

// Detector names — the fixed, closed set, in the canonical emit order the
// machine face is sorted into (maintenance-sweep-v1 §3.2, §3.3). No detector
// outside this set runs (A-1).
const (
	DetectorUnwiredPackages      = "unwired-packages"
	DetectorTouchesUnderdelivery = "touches-underdelivery"
	DetectorRollupStaleness      = "rollup-staleness"
)

// DetectorOrder is the fixed detector order the report is emitted in — the
// determinism ground (DINV-2). A result whose name is outside this set sorts
// after the known detectors, by name.
var DetectorOrder = []string{
	DetectorUnwiredPackages,
	DetectorTouchesUnderdelivery,
	DetectorRollupStaleness,
}

// Outcome values — the pinned ranking findings > fault > clean (§3.3).
const (
	OutcomeClean    = "clean"
	OutcomeFindings = "findings"
	OutcomeFault    = "fault"
)

// Waiver kinds surfaced on the machine face (§3.3, DINV-5).
const (
	WaiverKindPackage = "package"
	WaiverKindPlan    = "plan"
)

// Finding is one typed advisory signal — the four PRD-contracted fields plus
// its schema pin (prd FR-1, DINV-5). Construct it through NewFinding so the
// four-field contract is enforced at the source; Report.Marshal re-checks it.
type Finding struct {
	SchemaVersion string `json:"schema_version"`
	Detector      string `json:"detector"`
	Subject       string `json:"subject"`
	Evidence      string `json:"evidence"`
	Remedy        string `json:"remedy"`
}

// NewFinding builds a finding, refusing construction unless all four contracted
// fields are present (DINV-5). detector must be one of the closed set.
func NewFinding(detector, subject, evidence, remedy string) (Finding, error) {
	f := Finding{
		SchemaVersion: FindingSchemaVersion,
		Detector:      detector,
		Subject:       subject,
		Evidence:      evidence,
		Remedy:        remedy,
	}
	if err := f.validate(); err != nil {
		return Finding{}, err
	}
	return f, nil
}

// validate reports the first way a finding breaches its four-field contract.
func (f Finding) validate() error {
	var missing []string
	if strings.TrimSpace(f.Detector) == "" {
		missing = append(missing, "detector")
	}
	if strings.TrimSpace(f.Subject) == "" {
		missing = append(missing, "subject")
	}
	if strings.TrimSpace(f.Evidence) == "" {
		missing = append(missing, "evidence")
	}
	if strings.TrimSpace(f.Remedy) == "" {
		missing = append(missing, "remedy")
	}
	if len(missing) > 0 {
		return fmt.Errorf("finding missing contracted field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// AppliedWaiver is one suppression surfaced with its recorded reason — the
// visibility contract (DINV-5, prd FR-10): suppression is never silent.
type AppliedWaiver struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
}

// DetectorResult is one detector's contribution to the report: its findings,
// the waivers it applied, and its in-band fault (empty when it ran clean — a
// fault is named here, never masked, DINV-4).
type DetectorResult struct {
	Name           string          `json:"name"`
	Findings       []Finding       `json:"findings"`
	WaiversApplied []AppliedWaiver `json:"waivers_applied"`
	Fault          string          `json:"fault"`
}

// EvaluatedState pins the exact repository state the pass evaluated (§3.3): the
// HEAD commit, and a dirty flag when the working tree was not clean — a
// non-clean tree never certifies silently.
type EvaluatedState struct {
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

// Report is the one machine-face object per pass (maintenance-report/v1). No
// field carries wall-clock content — the report is a regenerable projection of
// state, byte-identical for identical (evaluated state, policy) (DINV-2).
//
// Faults carries the pass-level faults that precede or bypass the detectors —
// invocation misparse, an unreadable repository state, or a malformed policy
// (incl. a reasonless waiver). A detector's own fault stays inside its
// DetectorResult; Faults is exactly the faults with no detector to name them
// (§3.3, DINV-3). It participates in the pinned ranking identically to a
// detector fault: zero findings with any fault (pass-level or detector-level) →
// fault; findings win over either.
type Report struct {
	SchemaVersion  string           `json:"schema_version"`
	EvaluatedState EvaluatedState   `json:"evaluated_state"`
	Outcome        string           `json:"outcome"`
	Detectors      []DetectorResult `json:"detectors"`
	Faults         []string         `json:"faults"`
}

// NewReport assembles a report for the given evaluated state and detector
// results, deriving the outcome by the pinned ranking. The results are copied
// into canonical order (fixed detectors, lexicographic subjects) so the value
// marshals deterministically regardless of construction order. Pass-level
// faults (§3.3) may be supplied variadically — a pass that short-circuits before
// the detectors (misparse, unreadable state, malformed policy) passes its named
// faults here and no detector results.
func NewReport(state EvaluatedState, detectors []DetectorResult, faults ...string) Report {
	r := Report{
		SchemaVersion:  ReportSchemaVersion,
		EvaluatedState: state,
		Detectors:      canonicalDetectors(detectors),
		Faults:         canonicalFaults(faults),
	}
	r.Outcome = r.deriveOutcome()
	return r
}

// deriveOutcome ranks the pass: any finding → findings; else any fault
// (pass-level or detector-level) → fault; else clean (§3.3). The exit-code
// mapping mirrors this ranking (owned at T5).
func (r Report) deriveOutcome() string {
	hasFinding, hasFault := false, len(r.Faults) > 0
	for _, d := range r.Detectors {
		if len(d.Findings) > 0 {
			hasFinding = true
		}
		if d.Fault != "" {
			hasFault = true
		}
	}
	switch {
	case hasFinding:
		return OutcomeFindings
	case hasFault:
		return OutcomeFault
	default:
		return OutcomeClean
	}
}

// ExitCode maps an outcome to the pinned exit ranking (§3.3, shape's CLI
// contract): clean → 0, findings → 1, fault → 2. It is the outcome field's
// mirror — the exit code and the machine-face outcome are the same ranking read
// two ways (DINV-4).
func ExitCode(outcome string) int {
	switch outcome {
	case OutcomeFindings:
		return 1
	case OutcomeFault:
		return 2
	default:
		return 0
	}
}

// ExitCode returns the report's own exit code by the pinned ranking.
func (r Report) ExitCode() int { return ExitCode(r.Outcome) }

// Marshal renders the machine face as deterministic JSON. It re-validates every
// finding's four-field contract (a breach refuses to marshal, DINV-5) and emits
// detectors in fixed order with lexicographically ordered subjects and waivers
// (DINV-2). Two calls on an equal report value yield byte-identical output.
func (r Report) Marshal() ([]byte, error) {
	canon := Report{
		SchemaVersion:  r.SchemaVersion,
		EvaluatedState: r.EvaluatedState,
		Outcome:        r.Outcome,
		Detectors:      canonicalDetectors(r.Detectors),
		Faults:         canonicalFaults(r.Faults),
	}
	if canon.SchemaVersion == "" {
		canon.SchemaVersion = ReportSchemaVersion
	}
	for _, d := range canon.Detectors {
		for _, f := range d.Findings {
			if err := f.validate(); err != nil {
				return nil, fmt.Errorf("detector %q: %w", d.Name, err)
			}
		}
	}
	return json.Marshal(canon)
}

// canonicalFaults returns a non-nil copy of the pass-level faults so an
// absent-fault report marshals its faults as [] not null — the stable JSON
// shape (DINV-2). Insertion order is preserved: the pass produces its faults
// deterministically (policy defects in file order, one usage/state fault), so
// order is already a pure function of the input — no re-sort needed.
func canonicalFaults(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// canonicalDetectors returns a copy of the results in fixed detector order with
// each detector's findings and waivers sorted lexicographically by subject —
// the determinism the machine face contracts (DINV-2). Nil slices are
// normalized to empty so the JSON shape is stable.
func canonicalDetectors(in []DetectorResult) []DetectorResult {
	out := make([]DetectorResult, len(in))
	copy(out, in)

	rank := func(name string) int {
		for i, d := range DetectorOrder {
			if d == name {
				return i
			}
		}
		return len(DetectorOrder)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := rank(out[i].Name), rank(out[j].Name)
		if ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})

	// make() never returns nil, so an empty detector marshals as [] not null —
	// the stable JSON shape (DINV-2).
	for i := range out {
		findings := make([]Finding, len(out[i].Findings))
		copy(findings, out[i].Findings)
		sort.SliceStable(findings, func(a, b int) bool { return findings[a].Subject < findings[b].Subject })
		out[i].Findings = findings

		waivers := make([]AppliedWaiver, len(out[i].WaiversApplied))
		copy(waivers, out[i].WaiversApplied)
		sort.SliceStable(waivers, func(a, b int) bool { return waivers[a].Subject < waivers[b].Subject })
		out[i].WaiversApplied = waivers
	}
	return out
}
