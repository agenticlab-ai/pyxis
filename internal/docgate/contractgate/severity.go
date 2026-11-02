package contractgate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The severity-map policy — the closed-schema, versioned artifact that encodes,
// per TRIGGER, which contract-scope check CLASS is blocking versus advisory
// (prd:contract-gate/FR-13; design contract-gate-v1 §3.10, §3.11). It lives on
// the pyxis-config surface next to .pyxis/config.yaml, and it is DATA: changing a
// severity is a file edit, never a behavior-code change (AP-6/AP-10). The gate
// reads it to decide whether a check's defects FAIL the run (blocking) or are
// merely surfaced (advisory) — the same check subset runs at both triggers
// (AP-11), only the exit consequence differs.
//
// It parses CLOSED (matching config.Parse's discipline): an unknown key, an
// unknown trigger, an unknown check class, or an unknown severity is a
// configuration-error (UB-6, the fail-closed edge). A MISSING policy file uses
// the built-in default split (defaultSeverityMap) and never fails (UB-6). This is
// the same "absent → defaults; malformed → fail closed" contract config.Load
// carries, kept as a sibling loader so config.Load's own closed grammar is
// untouched.

// SeverityMapSchemaVersion pins the policy schema — additive-evolvable, distinct
// from the verdict and report schemas so the three evolve independently
// (AP-5/AP-6). A file must declare this exact version.
const SeverityMapSchemaVersion = "1"

// SeverityMapRelPath is the policy's home on the pyxis-config surface — a sibling
// of .pyxis/config.yaml (design §3.10: "on the pyxis-config/ surface"). Absent →
// the built-in default split.
const SeverityMapRelPath = ".pyxis/severity-map.yaml"

// Trigger is the closed set of contexts the gate runs under (EVT-1's trigger
// enum, prd:contract-gate/§5.7): the planning-entry precondition and the
// continuous contract-doc-PR surface. A verdict records which one produced it.
type Trigger string

const (
	// TriggerPlanningEntry is the hard precondition before plan-new drafts —
	// every class blocking by default (a contract entering planning must be whole).
	TriggerPlanningEntry Trigger = "planning-entry"
	// TriggerContractDocPR is the continuous advisory surface on PRD/design PRs —
	// currency/link/structural blocking, coverage advisory by default (a PRD
	// legitimately merges before its serving designs exist).
	TriggerContractDocPR Trigger = "contract-doc-pr"
)

// triggers is the closed trigger set — an unknown trigger key in a policy file is
// a configuration-error (UB-6).
var triggers = map[Trigger]bool{
	TriggerPlanningEntry: true,
	TriggerContractDocPR: true,
}

// ParseTrigger validates a --trigger flag value against the closed trigger set,
// returning the Trigger or a non-empty error string (the CLI turns it into a
// usage error). An empty value defaults to planning-entry (the hard-gate default,
// preserving pre-split behavior).
func ParseTrigger(s string) (Trigger, string) {
	if s == "" {
		return TriggerPlanningEntry, ""
	}
	t := Trigger(s)
	if !triggers[t] {
		return "", "unknown trigger " + s + " — the closed set is planning-entry, contract-doc-pr"
	}
	return t, ""
}

// SeverityClass is the check-class dimension the severity map keys on — the
// EVT-2 defect-class enum (prd:contract-gate/§5.7: currency·coverage·link·
// structural·compilability), one per contract-scope check. It is DISTINCT from a
// verdict Class: two checks (structural-completeness and inverse-coverage) both
// contribute the `incomplete` verdict class, so a per-verdict-class split could
// not express "structural blocking, coverage advisory" (the default). Keying on
// the check class — declared by each check — makes the specified split
// expressible and matches the instrumentation taxonomy exactly.
type SeverityClass string

const (
	SeverityClassStructural    SeverityClass = "structural"
	SeverityClassCompilability SeverityClass = "compilability"
	SeverityClassCurrency      SeverityClass = "currency"
	SeverityClassCoverage      SeverityClass = "coverage"
	SeverityClassLink          SeverityClass = "link"
)

// severityClasses is the closed check-class set — an unknown class key in a
// policy file is a configuration-error (UB-6). Every registered contract-scope
// check declares exactly one of these (registry.go's Check.SeverityClass), and
// severityMapCoversChecks asserts the two sets agree.
var severityClasses = map[SeverityClass]bool{
	SeverityClassStructural:    true,
	SeverityClassCompilability: true,
	SeverityClassCurrency:      true,
	SeverityClassCoverage:      true,
	SeverityClassLink:          true,
}

// Severity is the blocking-vs-advisory decision for a (trigger, class) pair — the
// whole point of the map. A blocking defect fails the run (exit 1); an advisory
// defect is surfaced (in the verdict + report) but does not fail it (exit 0).
type Severity string

const (
	// SeverityBlocking — a defect of this class fails the run at this trigger.
	SeverityBlocking Severity = "blocking"
	// SeverityAdvisory — a defect of this class is reported but does not fail the
	// run at this trigger.
	SeverityAdvisory Severity = "advisory"
)

var severities = map[Severity]bool{SeverityBlocking: true, SeverityAdvisory: true}

// SeverityMap is the parsed policy: for each trigger, each check class's severity.
// It is a total function over the closed (trigger × class) space — the parser
// seeds it from the default and overlays only the keys a file states, so an
// omitted key keeps its default rather than becoming an implicit blocker.
type SeverityMap struct {
	SchemaVersion string
	// byTrigger[trigger][class] = the severity. Fully populated for both triggers
	// and all classes after Parse/Default (a total lookup, never a nil miss).
	byTrigger map[Trigger]map[SeverityClass]Severity
}

// Severity returns the blocking/advisory decision for a check class at a trigger.
// A total lookup: the map is fully populated over the closed space, so an
// unrecognized (trigger, class) — impossible for a well-formed map — falls back
// to blocking, the safe default (never silently advisory).
func (m SeverityMap) Severity(trig Trigger, class SeverityClass) Severity {
	if byClass, ok := m.byTrigger[trig]; ok {
		if s, ok := byClass[class]; ok {
			return s
		}
	}
	return SeverityBlocking
}

// Blocking reports whether a check class's defects fail the run at a trigger.
func (m SeverityMap) Blocking(trig Trigger, class SeverityClass) bool {
	return m.Severity(trig, class) == SeverityBlocking
}

// defaultSeverityMap is the built-in split a repo with no policy file uses
// (prd:contract-gate/FR-13 default; design §3.10, brainstorm #D-4): at
// planning-entry every class is blocking (a contract entering planning must be
// whole); on a contract-doc PR currency/link/structural/compilability are
// blocking but inverse-coverage is ADVISORY — a PRD legitimately merges before
// its serving designs exist, so continuous coverage cannot block. Returned fresh
// each call so no caller can mutate the shared default.
func defaultSeverityMap() SeverityMap {
	return SeverityMap{
		SchemaVersion: SeverityMapSchemaVersion,
		byTrigger: map[Trigger]map[SeverityClass]Severity{
			TriggerPlanningEntry: {
				SeverityClassStructural:    SeverityBlocking,
				SeverityClassCompilability: SeverityBlocking,
				SeverityClassCurrency:      SeverityBlocking,
				SeverityClassCoverage:      SeverityBlocking,
				SeverityClassLink:          SeverityBlocking,
			},
			TriggerContractDocPR: {
				SeverityClassStructural:    SeverityBlocking,
				SeverityClassCompilability: SeverityBlocking,
				SeverityClassCurrency:      SeverityBlocking,
				SeverityClassCoverage:      SeverityAdvisory, // a PRD merges before its designs exist
				SeverityClassLink:          SeverityBlocking,
			},
		},
	}
}

// DefaultSeverityMap is the exported built-in split (defaultSeverityMap) — the
// policy a repo with no file uses. Callers wanting the default without a file
// (and tests pinning it) read it here.
func DefaultSeverityMap() SeverityMap { return defaultSeverityMap() }

// sevDef builds one severity-map parse defect naming the policy file — the
// fail-closed configuration-error surface (UB-6).
func sevDef(reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "contract-gate", File: SeverityMapRelPath, Op: "severity-map", Reason: reason, Remedy: remedy}
}

// LoadSeverityMap reads root/.pyxis/severity-map.yaml. An ABSENT file yields the
// built-in default split with no defects (UB-6 — a missing policy never fails). A
// present-but-unreadable or malformed file yields defects (the caller turns them
// into a configuration-error verdict, UB-6). Mirrors config.Load's contract.
func LoadSeverityMap(root string) (SeverityMap, []schema.Defect) {
	b, err := os.ReadFile(filepath.Join(root, SeverityMapRelPath))
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSeverityMap(), nil // absent → default split (UB-6)
		}
		return defaultSeverityMap(), []schema.Defect{sevDef(
			fmt.Sprintf("severity-map policy is unreadable: %v", err),
			"fix permissions or remove "+SeverityMapRelPath+" to use the built-in default split")}
	}
	return ParseSeverityMap(b)
}

// ParseSeverityMap validates the closed severity-map grammar over normalized
// bytes and returns the resolved map (default overlaid with the file's keys) plus
// any defects. The grammar is a closed YAML subset:
//
//	schema_version: "1"
//	triggers:
//	  planning-entry:
//	    coverage: advisory
//	  contract-doc-pr:
//	    currency: blocking
//	    ...
//
// Unknown top-level keys, unknown triggers, unknown check classes, unknown
// severities, or a wrong/missing schema_version are defects (UB-6). On any defect
// the returned map is the built-in default (the caller fails closed on the
// defects; the map is never trusted alongside a defect). Stdlib only.
func ParseSeverityMap(b []byte) (SeverityMap, []schema.Defect) {
	m := defaultSeverityMap()
	var defects []schema.Defect
	bad := func(n int, reason, remedy string) {
		defects = append(defects, sevDef(fmt.Sprintf("line %d: %s", n, reason), remedy))
	}

	section := ""           // the current top-level key ("triggers" | "")
	var curTrigger Trigger  // the current trigger under "triggers:"
	sawSchema := false      // schema_version seen
	sawTriggersKey := false // the "triggers:" section header seen

	for n, raw := range strings.Split(string(schema.Normalize(b)), "\n") {
		line := strings.TrimRight(raw, " ")
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		t := strings.TrimSpace(line)
		switch {
		case indent == 0 && strings.HasSuffix(t, ":") && !strings.Contains(strings.TrimSuffix(t, ":"), " "):
			// A top-level section header (e.g. "triggers:").
			section = strings.TrimSuffix(t, ":")
			curTrigger = ""
			switch section {
			case "triggers":
				sawTriggersKey = true
			default:
				bad(n+1, fmt.Sprintf("unknown top-level key %q", section),
					"the severity-map schema is closed: schema_version + triggers only (design §3.10, FR-13)")
			}
		case indent == 0 && strings.Contains(t, ":"):
			// A top-level scalar (schema_version).
			section = ""
			curTrigger = ""
			k, v, _ := strings.Cut(t, ":")
			k, v = strings.TrimSpace(k), unquoteSev(v)
			switch k {
			case "schema_version":
				sawSchema = true
				if v != SeverityMapSchemaVersion {
					bad(n+1, fmt.Sprintf("schema_version %q is not the pinned %q", v, SeverityMapSchemaVersion),
						"declare the pinned severity-map schema version")
				}
			default:
				bad(n+1, fmt.Sprintf("unknown top-level key %q", k),
					"the severity-map schema is closed: schema_version + triggers only (FR-13)")
			}
		case indent == 2 && strings.HasSuffix(t, ":") && section == "triggers":
			// A trigger header under "triggers:".
			name := Trigger(strings.TrimSuffix(t, ":"))
			if !triggers[name] {
				bad(n+1, fmt.Sprintf("unknown trigger %q", name),
					"triggers are the closed set: planning-entry, contract-doc-pr (§5.7)")
				curTrigger = ""
				continue
			}
			curTrigger = name
		case indent == 4 && strings.Contains(t, ":") && section == "triggers":
			// A class: severity mapping under a trigger.
			if curTrigger == "" {
				bad(n+1, "class severity outside a known trigger", "nest class severities under a valid trigger")
				continue
			}
			k, v, _ := strings.Cut(t, ":")
			class := SeverityClass(strings.TrimSpace(k))
			sev := Severity(unquoteSev(v))
			if !severityClasses[class] {
				bad(n+1, fmt.Sprintf("unknown check class %q", class),
					"classes are the closed set: structural, compilability, currency, coverage, link (§5.7)")
				continue
			}
			if !severities[sev] {
				bad(n+1, fmt.Sprintf("severity %q for %q is not blocking or advisory", sev, class),
					"a severity is exactly blocking or advisory (FR-13)")
				continue
			}
			m.byTrigger[curTrigger][class] = sev
		default:
			bad(n+1, "unrecognized structure",
				"the severity map is schema_version + triggers.<trigger>.<class>: <severity> only (closed subset)")
		}
	}

	if !sawSchema {
		bad(0, "no schema_version", "a severity-map policy declares schema_version: \""+SeverityMapSchemaVersion+"\"")
	}
	if !sawTriggersKey {
		bad(0, "no triggers section", "a severity-map policy declares a triggers: block (planning-entry / contract-doc-pr)")
	}
	if len(defects) > 0 {
		return defaultSeverityMap(), defects
	}
	return m, nil
}

// unquoteSev trims surrounding quotes and a trailing inline comment from a scalar
// value — the same lenient scalar handling config.Parse's unquote applies.
func unquoteSev(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.Trim(v, `"`)
}

// severityMapClasses is the check-class set the severity map keys on, sorted —
// used by tests to assert the policy space and the registered checks agree.
func severityMapClasses() []SeverityClass {
	out := make([]SeverityClass, 0, len(severityClasses))
	for c := range severityClasses {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
