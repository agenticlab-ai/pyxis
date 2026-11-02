// The contract-scope check registry — the `contract-scope` membership realized
// as its own distributed-check-registration registry (design contract-gate-v1
// §3.9; distributed-check-registration.md; AP-27/AP-23). It is the sibling of
// docgate's tree-wide `gate` registry, NOT the same one: a contract-scope check
// runs over ONE feature's already-resolved contract (§3.6/§3.9 machine-face
// data), so it carries a `Run(*contract) []Defect` shape rather than docgate's
// tree-wide `gate.Ctx`, and it declares the verdict CLASS its defects map to.
//
// Contribution is distributed to the unit and assembly is centralized and
// deterministic: each check self-registers from its own file at package init
// (`reg_check_*.go`), and the verb assembles the registered set sorted by name
// (never by init order). Adding a contract-scope check is one new file that
// edits no shared enumeration, dispatch, or help line — two check-adding tasks
// touch two disjoint files and cannot conflict (AP-23's charter surface).
package contractgate

import (
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Defects aliases the check result type into the package so a check file needs
// nothing but `package contractgate` + its init() — no per-file import to drift
// or conflict on (mirrors the gate registry's `Defects` alias).
type Defects = []schema.Defect

// Check is one contract-scope check contributed by its own file. It reads only
// the already-resolved contract (the machine-face data resolve.go assembled;
// DINV-3 — never re-parsing effective.md or a design body), returns the defects
// it found, and declares the verdict Class those defects map to on failure.
// The four fields are the whole contract: a stable Name (the deterministic sort
// key + the CheckResult id), the verdict Class its defects contribute, the
// SeverityClass the per-trigger severity map keys the check on, and the Run
// behavior. A check never mutates (DINV-1) — it computes over the contract and
// returns data.
type Check struct {
	// Name is the check's stable id — unique, the deterministic sort key, and
	// the CheckResult.Check value the verdict records.
	Name string
	// Class is the verdict class this check's defects contribute on failure
	// (e.g. ClassIncomplete for structural completeness, ClassMalformed for
	// compilability). A passing check contributes ClassClean.
	Class Class
	// SeverityClass is the check-class the per-trigger severity map keys this
	// check on (the EVT-2 taxonomy: structural·compilability·currency·coverage·
	// link; §5.7). It is DISTINCT from the verdict Class — structural-completeness
	// and inverse-coverage both contribute the `incomplete` verdict class but are
	// separate severity classes, so the map can hold one blocking and one advisory
	// (the default split). Each check declares exactly one; severityMapCoversChecks
	// asserts the two sets agree.
	SeverityClass SeverityClass
	// Run evaluates the resolved contract and returns every defect found —
	// empty when the check passes. Read-only over the contract (DINV-1/DINV-3).
	Run func(c *contract) Defects
}

// registry is written only from package init() (one Register per check file),
// so it is effectively sealed once the program runs. Reads go through the
// sorted accessor below, never the map, so the verdict's check order never
// depends on map or init-evaluation order (DINV-1 of the DCR design).
var registry = map[string]Check{}

// Register contributes a contract-scope check. Called from a check file's
// init(). Names are unique by construction (one check per file); a duplicate is
// a program bug the AP-27 registry test catches (registerCheckPanicsOnDup makes
// it a hard failure), never a silent overwrite in production.
func Register(c Check) {
	if _, dup := registry[c.Name]; dup {
		// A duplicate name means two files claim the same check — a build-time
		// programmer error, not a runtime condition to tolerate (DINV-4 of the
		// DCR design: a collision is a failure, never a silent shadow).
		panic("contractgate: duplicate contract-scope check registered: " + c.Name)
	}
	registry[c.Name] = c
}

// contractScopeChecks returns every registered contract-scope check sorted by
// name — the stable order that makes the verdict's check_results deterministic
// regardless of init-evaluation order (DINV-5; DCR DINV-1). This is the
// `contract-scope` subset the verb assembles and runs feature-scoped (§3.9).
func contractScopeChecks() []Check {
	out := make([]Check, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// checkSeverityClass maps a check name to its declared severity class — the seam
// the verb uses to look a check up in the per-trigger severity map. Read from the
// registered check, so a new check carries its own class with no shared-line edit.
func checkSeverityClass(name string) (SeverityClass, bool) {
	c, ok := registry[name]
	if !ok {
		return "", false
	}
	return c.SeverityClass, ok
}
