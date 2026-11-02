// Package writehook is the write-time protected-path deny for Argus drives
// (docs/tech-design/argus-write-hook.md §3.1): one canonical deny decision,
// authored once on the vendor-neutral agent surface, that every supported
// agent CLI's own pre-write hook consumes through a per-CLI adapter. It takes
// a pending write's target and the drive's declared scope, reuses the shared
// boundary predicate to classify the target, and returns allow for an in-scope
// target or a deny naming the path, the boundary it hit, and a remedy — before
// any bytes are written.
//
// The deny logic and the protected-path knowledge live here once; each CLI's
// wiring is a pointer/adapter (adapter_<cli>.go + a .pyxis/writehook config),
// never a content copy (DINV-5, AP-25). This is the *fast* layer, not the
// unbypassable one: a desk-side deny cannot stop an out-of-band write (a raw
// shell redirect, a non-honoring CLI) — the merge-boundary layers (the
// authority-contract structure arm and the attribution gate) carry that
// guarantee (§2, §3.4). Fail closed on ambiguity: an unresolvable declared
// scope and a hook-runtime error both deny (§3.3, AP-17).
package writehook

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// Decision is the canonical verdict for one pending write: allow, or a deny
// carrying the three-part {path, boundary, remedy} reason NFR-4 mandates
// (DINV-1). Policy records which version-pinned protected-path data produced
// the verdict (AP-6).
type Decision struct {
	Allow    bool   // true iff the target is in the drive's declared scope
	Path     string // the pending write's target (normalized by the classifier)
	Boundary string // the boundary the target hit — the class name, or "" when allowed
	Remedy   string // the actionable next step for that boundary — "" when allowed
	Policy   string // the protected-path data version the verdict was made against
}

// Reason renders the three-part {path, boundary, remedy} deny reason surfaced
// to the drive as a boundary signal (never a transient failure). Every deny
// across every per-CLI adapter reuses this one shape so the fixtures can assert
// all three elements (A-1, A-4). An allow has no reason.
func (d Decision) Reason() string {
	if d.Allow {
		return ""
	}
	return fmt.Sprintf("write denied: %s — boundary: %s; remedy: %s", d.Path, d.Boundary, d.Remedy)
}

// remedies maps each denying boundary class to its actionable remedy — the
// third element of the reason. Authored once here beside the decision; a CLI
// adapter never re-authors a remedy.
var remedies = map[boundary.Class]string{
	boundary.AuthorityContract:  "the authority contract is human-changed policy — a human lands this change by their own PR",
	boundary.GovernanceDocument: "the governing layer is append-only and human-authored — a human lands this change by their own PR",
	boundary.OtherProblemLedger: "write only your own problem's ledger — another problem's ledger is landed by that problem's own drive",
	boundary.OutOfScope:         "widen the drive's declared scope to include this path, or a human lands this change by their own PR",
	boundary.Unresolvable:       "resolve the drive's declared scope (start a drive with a resolvable declared scope) before writing",
}

// remedyFor returns the remedy for a denying class, or a fail-closed generic
// remedy for any class not in the table — an unrecognized class denies (never
// silently allows), honoring AP-17 on the safe side.
func remedyFor(c boundary.Class) string {
	if r, ok := remedies[c]; ok {
		return r
	}
	return "this write is outside the drive's boundary — a human lands a legitimate change by their own PR"
}

// Decide is the canonical deny decision. It classifies target against the
// protected-path set and drive's declared scope (the shared predicate) and
// returns allow iff the class is in-scope; every other class — the four
// protected-path classes and the fail-closed unresolvable signal — denies with
// the three-part reason (§3.1, DINV-1, DINV-2). It is synchronous (a write is
// allowed or denied whole) and local (no network, no new identity): the input
// is the config (for the target-facing doc roots the classification is keyed
// on), the target path, and the resolved scope, nothing else.
func Decide(cfg config.Config, target string, drive boundary.Drive) Decision {
	class := boundary.Classify(cfg, target, drive)
	if class == boundary.InScope {
		return Decision{Allow: true, Path: target, Boundary: class.String(), Policy: boundary.DataVersion}
	}
	return Decision{
		Allow:    false,
		Path:     target,
		Boundary: class.String(),
		Remedy:   remedyFor(class),
		Policy:   boundary.DataVersion,
	}
}

// HookErrorBoundary names the boundary a hook-runtime deny reports — distinct
// from a classified protected-path hit and from the unresolvable-input case.
const HookErrorBoundary = "hook runtime error"

// DenyForHookError is the fail-closed decision for a hook process error, crash,
// or timeout at evaluation (§3.3, DINV-3, UB-6): a wedged or crashing hook on a
// self-constraining surface is a plausible bypass, so a runtime error denies
// with the same three-part reason, never fails open. target is the best-effort
// pending path (or a placeholder when even that could not be read).
func DenyForHookError(target string, cause error) Decision {
	if target == "" {
		target = "(unresolved target — hook errored before the path could be read)"
	}
	remedy := "fix or restart the hook, then retry the write"
	if cause != nil {
		remedy = fmt.Sprintf("%s (%v)", remedy, cause)
	}
	return Decision{
		Allow:    false,
		Path:     target,
		Boundary: HookErrorBoundary,
		Remedy:   remedy,
		Policy:   boundary.DataVersion,
	}
}
