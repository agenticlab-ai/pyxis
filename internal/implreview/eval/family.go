// Package eval is the impl-review eval suite and its execution protocol — the
// permanent regression harness that pins the reviewer's behavior on controlled
// ground (docs/tech-design/impl-review-v1.md §3.5, plan T12; FR-4, FR-10, UB-3,
// M3, DOD-2).
//
// The design constraint that shapes this whole package is HERMETIC CI: the gate
// is `go test ./...`, which runs with NO network and NO LLM. So the suite
// is split in two, and this package is the split's Go half:
//
//   - The eval HARNESS (Go, hermetic, fully tested — harness.go): it runs a
//     family N times (N read from the versioned provenance, >= 5), passes ONLY if
//     the family is caught on EVERY run (an intermittent miss is a DEFECT, never
//     a silent rerun-until-green), scores the floor-exclusion (EVAL-4) and
//     injection-invariance (EVAL-5) legs by a DETERMINISTIC COMPARATOR
//     (comparator.go — defect-class membership; verdict equality — never a judge
//     model), enforces the compactness bound on payload size over the goldens
//     (compactness.go), and reads the band token/latency budgets as versioned
//     data (budget.go). The harness is driven by an injected LensRunner
//     (runtime.LensRunner, the T4 seam) so the N-run / caught-every-run /
//     comparator / compactness logic is fully exercised HERMETICALLY with a
//     deterministic FAKE runner (family_runner.go).
//
//   - The eight FAMILIES as fixture DATA — .pyxis/skills/impl-review/fixtures/:
//     the eight EVAL-1..8 families as JSON (this file's Family schema), the band
//     budgets (budgets.json), and the decode-pinned run provenance
//     (provenance.json). Families grow ONLY by reviewed data PR; a removal or a
//     threshold relaxation is a reviewed decision (§3.5) — this package NEVER
//     hard-codes a family, it LOADS them, exactly as the calibration table is
//     data (calibration.md).
//
//   - The LIVE-LLM run (actually running the real lenses N times against a real
//     backend) is GATED behind the `eval` build tag (live.go / live_test.go) and
//     an env-var skip, so it does NOT run in the hermetic `go test ./...`
//     (which must stay green) but IS the documented real eval path (DOD-2's
//     "permanent CI" = the harness in CI + the gated live suite). See live.go's
//     doc for how to run it.
//
// This package imports the T4 runtime seam (runtime.LensRunner), the report
// schema, the persist artifact composer (for compactness), and the band + floor
// packages (for the truncation and floor-exclusion legs' deterministic scoring).
// It edits none of them. Stdlib + those packages only, per the repo Go rules.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FamilySchema is the one fixture shape this build parses. A family fixture
// declaring any other schema is rejected — the parse is CLOSED (AP-6), never a
// lenient read.
const FamilySchema = "impl-review-eval-family/v1"

// The eight family kinds — the closed set of family shapes EVAL-1..8 take. The
// kind selects how the harness runs and scores a family (harness.go): a
// seeded-catch family asserts its expected finding is caught on every run; a
// floor-exclusion / injection-invariance / truncation family is scored by a
// deterministic comparator; a nit-golden family bounds findings-per-golden.
const (
	// KindSeededCatch — a single-band seeded defect the reviewer must catch on
	// every run (EVAL-1, EVAL-2).
	KindSeededCatch = "seeded-catch"
	// KindSeededCatchBandSensitive — a seeded defect whose catch depends on the
	// band: the deep-band leg must catch it (blocking), a standard-band leg may
	// record it uncaught (EVAL-3, EVAL-7). Both legs are required.
	KindSeededCatchBandSensitive = "seeded-catch-band-sensitive"
	// KindFloorExclusion — the floor-exclusion family: zero findings whose class
	// is a codegate-enforced defect class, scored by the defect-class-membership
	// comparator (EVAL-4).
	KindFloorExclusion = "floor-exclusion"
	// KindInjectionInvariance — the injection family: the verdict is identical to
	// the directive-free baseline and the directive surfaces as a candidate,
	// scored by the verdict-equality comparator (EVAL-5).
	KindInjectionInvariance = "injection-invariance"
	// KindNitGolden — the nit-discipline goldens: <= max_findings_p50 findings per
	// golden at p50 across the golden set (EVAL-6); also the compactness leg's
	// golden set.
	KindNitGolden = "nit-golden"
	// KindTruncationResistance — the oversize family: zero approve verdicts over a
	// diff exceeding the band budget, a size-escalation recorded (EVAL-8).
	KindTruncationResistance = "truncation-resistance"
)

// ExpectedCatch is the finding a CORRECT reviewer produces for a seeded-catch
// leg — the ground-truth catch the harness asserts is present on every run. Its
// DefectClass is the deterministic key the harness matches on (a real run's
// finding is a match iff its class-token equals this DefectClass), so the catch
// assertion is a mechanical membership check, not prose comparison.
type ExpectedCatch struct {
	// DefectClass is the class token the caught finding declares (the token after
	// the last "/" in RuleID) — the deterministic match key.
	DefectClass string `json:"defect_class"`
	// RuleID is the lens+rule identity of the expected finding.
	RuleID string `json:"rule_id"`
	// CalRow is the calibration row the expected finding cites.
	CalRow string `json:"cal_row"`
	// Severity is the expected finding's severity (blocking / advisory).
	Severity string `json:"severity"`
	// Message is the expected finding's message (the seeded finding the fake
	// runner emits, and the human-readable statement of the defect).
	Message string `json:"message"`
}

// FloorCandidate is a candidate finding a floor-exclusion leg seeds that MUST be
// excluded — it restates a codegate-enforced class (EVAL-4). The harness feeds
// these to the reviewer and the defect-class-membership comparator asserts NONE
// survives into the payload.
type FloorCandidate struct {
	RuleID         string `json:"rule_id"`
	CalRow         string `json:"cal_row"`
	Severity       string `json:"severity"`
	DefectClass    string `json:"defect_class"`
	Message        string `json:"message"`
	ReasonExcluded string `json:"reason_excluded"`
}

// AllowedFinding is a judgment finding a floor-exclusion leg seeds that SHOULD
// survive — a genuine defect the ladder cannot see, whose class is NOT enforced.
// It proves the floor drops restatements without dropping real findings.
type AllowedFinding struct {
	RuleID      string `json:"rule_id"`
	CalRow      string `json:"cal_row"`
	Severity    string `json:"severity"`
	DefectClass string `json:"defect_class"`
	Message     string `json:"message"`
}

// DirectiveSurfaced is the finding an injection leg expects when the reviewer
// surfaces a planted directive as a candidate (UB-3) rather than obeying it.
type DirectiveSurfaced struct {
	DefectClass string `json:"defect_class"`
	RuleID      string `json:"rule_id"`
	CalRow      string `json:"cal_row"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
}

// ExpectedOutcome is the truncation leg's expected outcome (EVAL-8): a
// size-escalation recorded, the proposal routed HUMAN, and the approve-class
// verdicts forbidden.
type ExpectedOutcome struct {
	SizeEscalation    bool     `json:"size_escalation"`
	RouteHuman        bool     `json:"route_human"`
	ForbiddenVerdicts []string `json:"forbidden_verdicts"`
}

// Golden is one clean, legitimate proposal in a nit-golden family (EVAL-6): the
// diff a disciplined reviewer draws at most a few findings on. Its Findings are
// the seeded findings a correct reviewer produces; the p50 bound is over the set.
type Golden struct {
	Band     string    `json:"band"`
	Charter  string    `json:"charter"`
	Diff     string    `json:"diff"`
	Findings []Finding `json:"findings"`
}

// Finding is a seeded finding within a golden (the fields the harness needs to
// build a payload for the compactness leg and to count for the nit leg). It is
// NOT report.Finding — it is the fixture's compact form; toReportFinding builds
// the report finding from it (harness.go).
type Finding struct {
	RuleID      string `json:"rule_id"`
	CalRow      string `json:"cal_row"`
	Severity    string `json:"severity"`
	DefectClass string `json:"defect_class"`
	Message     string `json:"message"`
}

// Leg is one band-leg of a family. A family has one or more legs; a
// band-sensitive family (EVAL-3/EVAL-7) has a deep leg and a standard leg. Which
// fields are populated depends on the family Kind (validated by validateFamily).
type Leg struct {
	// Band is the leg's review depth (skip / standard / deep) — the band the
	// harness runs this leg under.
	Band string `json:"band"`
	// MustCatch is whether this leg's defect MUST be caught. For a band-sensitive
	// family a standard leg may set MustCatch=false to record the same defect as
	// uncaught (both legs required — §5.9).
	MustCatch bool `json:"must_catch"`
	// ShipsBlocking is set on a deep leg whose caught finding must ship BLOCKING
	// (EVAL-7's deep leg, FR-10).
	ShipsBlocking bool `json:"ships_blocking,omitempty"`
	// Note is the recorded human-readable basis for a leg (e.g. WHY a standard
	// leg records a miss); informative.
	Note string `json:"note,omitempty"`

	// Charter is the task charter the leg reviews under.
	Charter string `json:"charter"`
	// Diff is the seeded diff the leg reviews (a seeded-catch / floor / golden
	// leg). Empty on an injection leg (which carries baseline_diff/injected_diff)
	// and a truncation leg (which carries diff_bytes).
	Diff string `json:"diff,omitempty"`

	// --- seeded-catch fields ---
	ExpectedCatch *ExpectedCatch `json:"expected_catch,omitempty"`

	// --- floor-exclusion fields ---
	FloorCandidates []FloorCandidate `json:"floor_candidates,omitempty"`
	AllowedFindings []AllowedFinding `json:"allowed_findings,omitempty"`

	// --- injection-invariance fields ---
	BaselineDiff      string             `json:"baseline_diff,omitempty"`
	InjectedDiff      string             `json:"injected_diff,omitempty"`
	BaselineVerdict   string             `json:"baseline_verdict,omitempty"`
	DirectiveSurfaced *DirectiveSurfaced `json:"directive_surfaced,omitempty"`

	// --- truncation-resistance fields ---
	DiffBytes       int              `json:"diff_bytes,omitempty"`
	DiffFillerNote  string           `json:"diff_filler_note,omitempty"`
	ExpectedOutcome *ExpectedOutcome `json:"expected_outcome,omitempty"`
}

// Family is one EVAL-n family fixture (§3.5, §5.9). It is versioned data loaded
// from .pyxis/skills/impl-review/fixtures/families/; the harness runs it under
// the protocol its Kind selects.
type Family struct {
	// Schema pins the fixture shape; MUST equal FamilySchema.
	Schema string `json:"schema"`
	// ID is the family id (EVAL-1..8).
	ID string `json:"id"`
	// Kind selects the run + scoring protocol (one of the Kind* constants).
	Kind string `json:"kind"`
	// Lens is the lens the family exercises ("*" for a cross-lens family like the
	// floor, injection, nit, or truncation families).
	Lens string `json:"lens"`
	// Title is the family's one-line behavior statement (from §5.9).
	Title string `json:"title"`
	// Threshold is the family's pass threshold (from §5.9), recorded for the
	// reader; the harness enforces the machine-checkable part per Kind.
	Threshold string `json:"threshold"`
	// Comparator names the deterministic comparator a comparator-scored family
	// uses (defect-class-membership / verdict-equality / no-approve-over-oversize);
	// empty for a catch/golden family.
	Comparator string `json:"comparator,omitempty"`
	// MaxFindingsP50 is the nit family's per-golden p50 bound (EVAL-6).
	MaxFindingsP50 int `json:"max_findings_p50,omitempty"`
	// NegativeExample is the §5.9 negative example the family guards against —
	// recorded so a reader sees what a naive reviewer would get wrong.
	NegativeExample string `json:"negative_example"`

	// Legs are the family's band-legs (seeded-catch, floor, injection,
	// truncation families).
	Legs []Leg `json:"legs,omitempty"`
	// Goldens are the nit family's clean proposals (EVAL-6).
	Goldens []Golden `json:"goldens,omitempty"`
}

// LoadFamily reads and parses one family fixture at path, CLOSED (an unknown key
// is an error, AP-6), and validates its Kind-specific shape. A malformed family
// is a data defect surfaced here, never a lenient read that silently drops a
// field.
func LoadFamily(path string) (*Family, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("eval: read family %s: %w", path, err)
	}
	var f Family
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("eval: parse family %s: %w", path, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("eval: parse family %s: trailing data after the fixture", path)
	}
	if err := validateFamily(&f); err != nil {
		return nil, fmt.Errorf("eval: family %s: %w", path, err)
	}
	return &f, nil
}

// LoadFamilies reads every family fixture under dir (the canonical dir is
// .pyxis/skills/impl-review/fixtures/families), sorted by file name for a stable
// order, and returns them. It requires at least one family (a suite with no
// families is a misconfiguration) and rejects a duplicate family id.
func LoadFamilies(dir string) ([]*Family, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("eval: read families dir %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	out := make([]*Family, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		f, err := LoadFamily(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if seen[f.ID] {
			return nil, fmt.Errorf("eval: duplicate family id %q (file %s)", f.ID, name)
		}
		seen[f.ID] = true
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("eval: no family fixtures found in %s", dir)
	}
	return out, nil
}

// validateFamily enforces the closed, Kind-specific shape of a family — a data
// defect (a catch family with no expected_catch, an injection family with no
// baseline verdict, …) is an error, never a family that runs vacuously and
// passes. This is the fixture-integrity gate the hermetic test relies on.
func validateFamily(f *Family) error {
	if f.Schema != FamilySchema {
		return fmt.Errorf("wrong schema %q (want %q)", f.Schema, FamilySchema)
	}
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("empty family id")
	}
	switch f.Kind {
	case KindSeededCatch, KindSeededCatchBandSensitive:
		if len(f.Legs) == 0 {
			return fmt.Errorf("%s family has no legs", f.Kind)
		}
		anyMustCatch := false
		for i := range f.Legs {
			lg := &f.Legs[i]
			if lg.Band == "" {
				return fmt.Errorf("leg %d has no band", i)
			}
			if lg.MustCatch {
				anyMustCatch = true
				if lg.ExpectedCatch == nil {
					return fmt.Errorf("leg %d must_catch but has no expected_catch", i)
				}
				if strings.TrimSpace(lg.ExpectedCatch.DefectClass) == "" {
					return fmt.Errorf("leg %d expected_catch has no defect_class", i)
				}
				if lg.Diff == "" {
					return fmt.Errorf("leg %d must_catch but has no diff", i)
				}
			}
		}
		if !anyMustCatch {
			return fmt.Errorf("%s family %q has no must_catch leg", f.Kind, f.ID)
		}
	case KindFloorExclusion:
		if f.Comparator != ComparatorDefectClassMembership {
			return fmt.Errorf("floor-exclusion family must use the %q comparator, has %q", ComparatorDefectClassMembership, f.Comparator)
		}
		if len(f.Legs) == 0 {
			return fmt.Errorf("floor-exclusion family has no legs")
		}
		for i := range f.Legs {
			if len(f.Legs[i].FloorCandidates) == 0 {
				return fmt.Errorf("floor-exclusion leg %d seeds no floor_candidates to exclude", i)
			}
		}
	case KindInjectionInvariance:
		if f.Comparator != ComparatorVerdictEquality {
			return fmt.Errorf("injection family must use the %q comparator, has %q", ComparatorVerdictEquality, f.Comparator)
		}
		if len(f.Legs) == 0 {
			return fmt.Errorf("injection family has no legs")
		}
		for i := range f.Legs {
			lg := &f.Legs[i]
			if lg.BaselineDiff == "" || lg.InjectedDiff == "" {
				return fmt.Errorf("injection leg %d needs both baseline_diff and injected_diff", i)
			}
			if lg.BaselineVerdict == "" {
				return fmt.Errorf("injection leg %d has no baseline_verdict", i)
			}
			if lg.DirectiveSurfaced == nil {
				return fmt.Errorf("injection leg %d does not name the directive it must surface", i)
			}
		}
	case KindNitGolden:
		if f.MaxFindingsP50 <= 0 {
			return fmt.Errorf("nit family has no positive max_findings_p50")
		}
		if len(f.Goldens) == 0 {
			return fmt.Errorf("nit family has no goldens")
		}
	case KindTruncationResistance:
		if f.Comparator != ComparatorNoApproveOverOversize {
			return fmt.Errorf("truncation family must use the %q comparator, has %q", ComparatorNoApproveOverOversize, f.Comparator)
		}
		if len(f.Legs) == 0 {
			return fmt.Errorf("truncation family has no legs")
		}
		for i := range f.Legs {
			lg := &f.Legs[i]
			if lg.DiffBytes <= 0 {
				return fmt.Errorf("truncation leg %d has no positive diff_bytes", i)
			}
			if lg.ExpectedOutcome == nil {
				return fmt.Errorf("truncation leg %d has no expected_outcome", i)
			}
			if len(lg.ExpectedOutcome.ForbiddenVerdicts) == 0 {
				return fmt.Errorf("truncation leg %d names no forbidden_verdicts", i)
			}
		}
	default:
		return fmt.Errorf("unknown family kind %q", f.Kind)
	}
	return nil
}
