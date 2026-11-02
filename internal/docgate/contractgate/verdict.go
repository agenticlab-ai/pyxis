// Package contractgate hosts the mechanical stage-6 contract gate — the
// `docgate contract-gate <feature>` verb (design contract-gate-v1 §3.1). It
// resolves a feature's assembled contract read-only from docgate's committed
// machine faces (the PRD source map + the live serving designs), runs the
// contract-scope check subset, and returns one closed, versioned Verdict.
//
// This is the verb SHELL (plan task T2): the check subset is EMPTY here, so a
// well-formed contract yields a `clean` verdict and the individual checks land
// in later tasks. The package is read-only by construction — it writes no file,
// runs no git, makes no network call, invokes no model (DINV-1, DINV-5); a
// clean pass materializes nothing (DINV-6). Its single parser is the fold /
// design index it reads; it never re-parses effective.md or a design body for
// substance (DINV-3).
package contractgate

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// SchemaVersion pins the verdict schema. Additive-evolvable: the reserved
// semantic slot (DINV-4) and the semantic verdict classes are declared here as
// KNOWN keys the LLM follow-up (#419) fills additively, so #419 lands with no
// v1 schema break (design §3.7; AP-5/AP-6).
const SchemaVersion = "1"

// Class is the verdict's overall outcome — a closed v1 enum. The mechanical
// classes are fully specified in v1; the semantic classes (ClassAmbiguous,
// ClassContradictory) are declared but NEVER produced by v1 code (the semantic
// layer is not evaluated here, DINV-4). A clean verdict is "mechanically
// clean", never "executable" (design §3.7, prd:contract-gate/FR-7).
type Class string

const (
	// ClassClean — every contract-scope check passed. In T2 the check set is
	// empty, so a resolvable contract is clean. Mechanically clean only.
	ClassClean Class = "clean"
	// ClassIncomplete — a structural or coverage check found a gap (a Must with
	// no fit criterion / verify method, an uncovered requirement). Later tasks.
	ClassIncomplete Class = "incomplete"
	// ClassMalformed — a compilability check found a malformed join. Later tasks.
	ClassMalformed Class = "malformed"
	// ClassStale — a currency check found a superseded serving input. Later.
	ClassStale Class = "stale"
	// ClassUnresolved — a link check found a dangling citation. Later tasks.
	ClassUnresolved Class = "unresolved"
	// ClassError — an input machine face was present but unparseable/malformed;
	// the gate fails closed rather than reporting clean (design §3.11).
	ClassError Class = "error"
	// ClassConfigurationError — a required input is missing/unreadable or the
	// contract is not merged (no such feature ledger); zero checks run, fail
	// closed (design §3.11, prd:contract-gate/UB-1..4).
	ClassConfigurationError Class = "configuration-error"

	// ClassAmbiguous / ClassContradictory are the RESERVED semantic classes
	// (DINV-4): declared for #419, never produced by v1 mechanical code. Their
	// presence in the enum keeps #419 an additive evolution.
	ClassAmbiguous     Class = "ambiguous"
	ClassContradictory Class = "contradictory"
)

// mechanicalClasses is the set a v1 verdict may actually carry. The semantic
// classes are excluded — a v1 verdict never reports them (DINV-4).
var mechanicalClasses = map[Class]bool{
	ClassClean: true, ClassIncomplete: true, ClassMalformed: true,
	ClassStale: true, ClassUnresolved: true, ClassError: true,
	ClassConfigurationError: true,
}

// CheckResult is one contract-scope check's outcome, aggregated into the
// verdict (design §3.7). Empty in T2 (the check set is empty); the later
// check tasks append one per check.
type CheckResult struct {
	// Check is the check's stable id (e.g. "structural-completeness").
	Check string `json:"check"`
	// Class is the verdict class this check contributes on failure, or
	// ClassClean when it passed.
	Class Class `json:"class"`
	// Passed is true when the check found no defect.
	Passed bool `json:"passed"`
	// Severity is the blocking-vs-advisory disposition of this check's defects at
	// the active trigger, read from the per-trigger severity map (FR-13; §3.10). A
	// failing check whose severity is advisory reports its defects but does NOT
	// fail the run; a blocking one does. Empty on a passed check (nothing to
	// disposition) and on the "resolve" pseudo-result (a fail-closed edge is not a
	// check subject to the severity split — it always fails closed).
	Severity Severity `json:"severity,omitempty"`
	// Defects names each offender this check red (check id, offender, unblock);
	// the dual-face defect report (task T5) is assembled from these.
	Defects []schema.Defect `json:"defects,omitempty"`
}

// Blocking reports whether this check red AND its defects are blocking at the
// active trigger — the results that actually fail the run (exit 1). A failed but
// advisory check is surfaced, not blocking.
func (r CheckResult) Blocking() bool {
	return !r.Passed && r.Severity == SeverityBlocking
}

// Advisory reports whether this check red but its defects are advisory at the
// active trigger — surfaced in the verdict/report, non-blocking (exit 0).
func (r CheckResult) Advisory() bool {
	return !r.Passed && r.Severity == SeverityAdvisory
}

// InputIdentity fingerprints one resolved input the verdict evaluated (design
// §3.7, AP-13): the source map's content fingerprint, and each live serving
// design's identity. A verdict is thus never true about the wrong contract
// state — a disputed verdict is reproducible by re-running against the same
// fingerprints (design §3.4).
type InputIdentity struct {
	// Kind is the input's class: "prd-source-map" | "serving-design".
	Kind string `json:"kind"`
	// Ref is the input's stable reference: the PRD path for the source map, the
	// design slug for a serving design.
	Ref string `json:"ref"`
	// Fingerprint is the input's content identity: the source map's
	// content_fingerprint, or a design's content hash. Empty when the input
	// carries no fingerprint of its own (recorded honestly, never faked).
	Fingerprint string `json:"fingerprint,omitempty"`
}

// Semantic is the RESERVED-not-evaluated slot (DINV-4). v1 never populates its
// internals: Evaluated is always false and Classes/Notes stay empty, so a
// mechanically-clean pass is never reported as the full "executable" verdict.
// The LLM follow-up (#419) fills this additively — a known key that never trips
// closed-schema unknown-key rejection (design §3.7, AP-5/AP-6).
type Semantic struct {
	// Evaluated reports whether the semantic layer ran. ALWAYS false in v1.
	Evaluated bool `json:"evaluated"`
	// Note records why the slot is empty — the honest "not evaluated in v1"
	// marker a consumer reads instead of inferring executability.
	Note string `json:"note"`
	// Classes / Findings are #419's additive extension points, empty in v1.
	Classes  []Class         `json:"classes,omitempty"`
	Findings []schema.Defect `json:"findings,omitempty"`
}

// notEvaluatedNote is the fixed marker every v1 verdict carries in its reserved
// semantic slot. Deterministic (DINV-5): no clock, no run-specific text.
const notEvaluatedNote = "not evaluated in v1 — the semantic layer (issue #419) fills this slot additively; a clean class is mechanically clean, never executable"

// Verdict is the verb's one closed, schema-versioned output (design §3.7). It
// records the overall class, each check's result (empty in T2), the identity of
// every input it evaluated, and the reserved semantic slot. Serialized as data
// downstream reads verbatim (AP-21); additive-evolvable via new tagged fields.
type Verdict struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedBy   string `json:"generated_by"`
	ToolVersion   string `json:"tool_version"`
	// Feature is the feature slug the verdict is scoped to.
	Feature string `json:"feature"`
	// Trigger is the context the gate ran under — planning-entry or
	// contract-doc-pr (EVT-1's trigger, §5.7). It selects the severity split the
	// verdict was judged against, so a verdict names the policy lens that produced
	// its blocking/advisory dispositions and its exit code.
	Trigger Trigger `json:"trigger"`
	// Class is the overall outcome — a mechanical class in v1 (DINV-4). It is the
	// most-severe FAILING check's class (blocking or advisory alike): a verdict
	// can carry a non-clean class yet still be non-blocking when its only failed
	// checks are advisory at this trigger. Read Blocked() for the exit-code
	// outcome; Class names WHAT is wrong, Blocked() whether it stops the run.
	Class Class `json:"class"`
	// Blocking is whether any failed check is blocking at the active trigger — the
	// authoritative exit-code driver (true ⇒ exit 1). Recorded in the verdict so a
	// downstream reader sees the split decision as data, not just an exit code.
	Blocking bool `json:"blocking"`
	// CheckResults is the per-check result slice — empty in T2 (empty check set).
	CheckResults []CheckResult `json:"check_results"`
	// Evaluated is the identity of every resolved input (fingerprints + refs).
	Evaluated []InputIdentity `json:"evaluated"`
	// Semantic is the reserved-not-evaluated slot (DINV-4).
	Semantic Semantic `json:"semantic"`
	// Reason is a short, human-readable summary of a non-clean class (empty on
	// clean). It routes attention; the check results and defect report carry the
	// authoritative detail.
	Reason string `json:"reason,omitempty"`
}

// newVerdict seeds a verdict with the fixed schema header and the reserved,
// not-evaluated semantic slot (DINV-4) — the one place the semantic marker is
// set, so no path can accidentally report it evaluated. The trigger defaults to
// planning-entry (the hard-gate default, preserving T5's every-class-blocking
// behavior); the verb sets it from its --trigger flag.
func newVerdict(feature string) Verdict {
	return Verdict{
		SchemaVersion: SchemaVersion,
		GeneratedBy:   "docgate",
		ToolVersion:   schema.ToolVersion,
		Feature:       feature,
		Trigger:       TriggerPlanningEntry,
		CheckResults:  []CheckResult{},
		Evaluated:     []InputIdentity{},
		Semantic:      Semantic{Evaluated: false, Note: notEvaluatedNote},
	}
}

// Clean reports whether the verdict found NO defect at all — every check passed
// (or a fail-closed resolution edge is not a "clean" state). Distinct from
// Blocked(): an advisory-only bounce is NOT clean (it carries defects) yet is
// non-blocking. A clean verdict emits no report (DINV-6).
func (v Verdict) Clean() bool { return v.Class == ClassClean }

// Blocked reports whether the run FAILS (exit 1) — true when any failed check is
// blocking at the active trigger, or the verdict is a fail-closed edge
// (configuration-error / error). An advisory-only bounce is not blocked (exit 0):
// its defects are surfaced but do not stop the run (FR-9; §3.10). This is the
// authoritative exit-code driver, distinct from Clean().
func (v Verdict) Blocked() bool { return v.Blocking }

// hasAdvisory reports whether the verdict carries any failed-but-advisory check —
// defects surfaced without blocking. Used to phrase the human summary honestly.
func (v Verdict) hasAdvisory() bool {
	for _, r := range v.CheckResults {
		if r.Advisory() {
			return true
		}
	}
	return false
}

// Summary is a one-line human rendering for stderr (design §3.1: a human
// summary alongside the structured verdict). It never carries run-specific
// noise (DINV-5): only the feature, class, input count, and any reason.
func (v Verdict) Summary() string {
	s := "contract-gate " + v.Feature + " [" + string(v.Trigger) + "]: " + string(v.Class)
	switch {
	case v.Class == ClassClean:
		s += " (mechanically clean — the semantic layer is not evaluated in v1)"
	case !v.Blocking && v.hasAdvisory():
		// A non-clean verdict that does not stop the run: its defects are advisory
		// at this trigger (FR-9). Say so, so a reader never mistakes exit 0 for no
		// defects.
		s += " (advisory only — defects surfaced, non-blocking at this trigger)"
	}
	// Count only real check runs — a fail-closed resolution defect is recorded
	// as a "resolve" pseudo-result but is NOT a check (design §3.11: zero checks
	// run on a bad input), so the human line reports it honestly.
	checks, advisory := 0, 0
	for _, r := range v.CheckResults {
		if r.Check != "resolve" {
			checks++
		}
		if r.Advisory() {
			advisory++
		}
	}
	s += fmt.Sprintf(" — %d input(s) evaluated, %d check(s) run", len(v.Evaluated), checks)
	if advisory > 0 {
		s += fmt.Sprintf(", %d advisory defect(s)", advisory)
	}
	if v.Reason != "" {
		s += "\n  " + v.Reason
	}
	return s
}

// Validate re-checks the verdict's closed shape as a self-guard (design §3.11
// "the verdict validates closed"): a known schema version, a mechanical class
// (never a reserved semantic class, DINV-4), and the semantic slot present and
// marked not-evaluated. It returns a defect list — empty when the verdict is
// well-formed. Deterministic and side-effect-free.
func (v Verdict) Validate() []schema.Defect {
	var defects []schema.Defect
	bad := func(reason, remedy string) {
		defects = append(defects, schema.Defect{
			Verb: "contract-gate", File: "(verdict)", Op: "validate",
			Reason: reason, Remedy: remedy,
		})
	}
	if v.SchemaVersion != SchemaVersion {
		bad("verdict schema_version "+v.SchemaVersion+" is not the pinned "+SchemaVersion,
			"emit the pinned verdict schema version")
	}
	if !triggers[v.Trigger] {
		bad("verdict trigger "+string(v.Trigger)+" is not a known trigger",
			"a verdict names the closed trigger it ran under: planning-entry or contract-doc-pr (§5.7)")
	}
	if !mechanicalClasses[v.Class] {
		bad("verdict class "+string(v.Class)+" is not a v1 mechanical class",
			"a v1 verdict carries a mechanical class only; the semantic classes are reserved-not-evaluated (DINV-4)")
	}
	if v.Semantic.Evaluated {
		bad("semantic slot reports evaluated in v1",
			"the semantic layer is not evaluated in v1 — the slot is reserved for issue #419 (DINV-4)")
	}
	if v.Semantic.Note == "" {
		bad("semantic slot carries no not-evaluated marker",
			"the reserved semantic slot names why it is empty (DINV-4)")
	}
	return defects
}
