// Package policy is the merge lane's risk-policy data class and the one
// shared validator every reader loads it through (merge-lane-v1 §3.1, §3.6;
// plan T1). The policy is versioned, closed-schema data at the repo's config
// surface: floor classes, the AUTO-eligible class set as data-defined
// whole-diff predicates, thresholds, the activation block, and the kill
// switch. No proposal can widen it — the self-unlock minimum is design
// content (the constants below), validator-enforced, and a policy whose
// floor fails to cover it, whose AUTO code class claims the lane's build
// inputs, that misparses, that carries an unknown key, that is unratified,
// or that is absent refuses to load; on any such defect the lane treats
// every proposal HUMAN while the validator names the defect (DINV-6,
// DINV-8). Stdlib only, per the repo Go rules; the parse is closed, never a
// lenient guess.
package policy

import "github.com/agenticlab-ai/pyxis/internal/docgate/schema"

// SchemaVersion is the one policy shape this build parses against. A policy
// declaring any other version is a defect, never parsed leniently (DINV-8).
const SchemaVersion = "risk-policy/v1"

// verb names this validator in every defect it emits (schema.Defect.Verb).
const verb = "lane-policy"

// SelfUnlockMinPaths is the self-unlock minimum as PATHS — design content
// (merge-lane-v1 §3.1), not merely policy content. The floor of any policy
// that loads MUST cover every glob here; ratification may WIDEN the floor
// beyond it, never shrink below it (DINV-6). The set is the lane's whole
// trust base: the control plane, the governing documents, the risk policy +
// activation home, the operating manual in both faces, the agent-control
// surface with its vendor-dir pointers, the lane's OWN code, and the lane's
// OWN build inputs. Kept verbatim so the floor stays auditable — a policy
// that dropped one of these globs (the hollow-floor case) refuses to load.
//
// Cross-feature exclusion — the governing-doc globs here are NOT config-driven
// (config-driven-portability-v1 DINV-1 lifts target-facing doc-path resolution;
// this is a different surface the T6 coherence inspection allowlists). This set
// is merge-lane-v1's security floor: an immovable, human-auditable allowlist a
// separately-ratified policy file must cover by EXACT-STRING membership
// (validate.go), never a place the engine resolves "where the vision lives" to
// check it. Deriving these entries from config would (1) break the "kept
// verbatim so the floor stays auditable" property the design demands — the
// audited trust base would vary by config; (2) desynchronize the exact-string
// coverage check from the operator-ratified policy file, which spells its floor
// globs as literals, so a non-default config would spuriously fail the
// floor-coverage validation or — worse — hand an attacker a second lever to
// SHRINK the minimum below the ratified floor (config change → self-unlock),
// the very ABUSE-2 path DINV-6 exists to close. Config-driven-portability must
// not unilaterally reshape another feature's security floor; a governing-doc
// relocation here is a merge-lane-v1 design change (its DINV-6), not a config
// read. The three governing-doc entries below stay LITERAL by that design.
var SelfUnlockMinPaths = []string{
	".github/workflows/**", // control plane
	".githooks/**",
	"docs/product-vision.md", // governing documents
	"docs/architecture-principles.md",
	"docs/observability/incidents.md",
	"docs/policy/**", // the risk policy + activation state
	"AGENTS.md",      // the operating manual — the merge convention's home
	"CLAUDE.md",      // …and its symlinked face
	".pyxis/**",      // the agent-control surface
	".claude/**",     // vendor-dir pointers into it
	".codex/**",
	"cmd/lane/**",      // the lane's OWN code (classifier, validator, acting glue, review arm)
	"internal/lane/**", //
	"go.mod",           // the lane's OWN build inputs — a substitution poisons the next mainline build
	"go.sum",           //
}

// SelfUnlockMinClasses is the self-unlock minimum as diff-derived CLASSES:
// the governing-document and frozen-ledger-entry protections. A policy whose
// floor omits either refuses to load (DINV-6).
var SelfUnlockMinClasses = []string{"governing-doc-edit", "ledger-frozen-entry-edit"}

// BuildInputs are the lane's own module manifests. No AUTO class may claim
// these in its perimeter — they are the lane's trust base and a class that
// covered them would open the two-step ratify-then-plant path (merge-lane-v1
// §3.1/§3.6, DINV-6); the validator refuses such a policy.
var BuildInputs = []string{"go.mod", "go.sum"}

// Floor is the policy's hard gate — evaluated first, any hit terminal HUMAN
// (DINV-2). Both arms are diff-derived: paths matched against the whole diff,
// classes never author-declared.
type Floor struct {
	HardGatedPaths   []string
	HardGatedClasses []string
}

// AutoClass is one AUTO-eligible class: a data-defined deterministic
// predicate — a name, a whole-diff path perimeter, and its precondition
// flags. Membership is the ENTIRE diff inside ONE class's perimeter
// (merge-lane-v1 §3.1); the classifier (T2) consumes it, not this package.
type AutoClass struct {
	Name                    string
	Paths                   []string
	RequiresReviewerVerdict bool
}

// Thresholds are the shadow-window bar the agreement report reads.
type Thresholds struct {
	ShadowWindowMinProposals int
	AgreementBar             float64
}

// Activation is the operator's recorded activation block — read as
// preconditions, never performed by this feature (FR-9). enforce:true with
// the backstop ref absent resolves shadow plus a named activation-incomplete
// defect at every state read (owned downstream at T4).
type Activation struct {
	Enforce           bool
	BackstopChangeRef string
}

// Policy is the parsed, validated risk policy — the closed shape a reader
// receives from Load. Every field is data; retuning is a policy PR under the
// operator trailer, never a code change.
type Policy struct {
	SchemaVersion string
	RatifiedBy    string
	Floor         Floor
	AutoClasses   []AutoClass
	Thresholds    Thresholds
	Activation    Activation
	KillSwitch    bool
}

// HumanOnly reports whether the kill switch has flipped the lane to
// HUMAN-only — the drill-tested single data change (merge-lane-v1 §3.7,
// A-4). The full state resolution (killed · halted · enforcing · shadow) is
// owned downstream (T4/T5); this is the kill arm the reader exposes today.
func (p *Policy) HumanOnly() bool { return p.KillSwitch }

func def(op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: relFile, Op: op, Reason: reason, Remedy: remedy}
}

// relFile is the defect File slot — overwritten by Load with the resolved
// policy path; the pure Parse/Validate path names the schema generically.
const relFile = "risk-policy"
