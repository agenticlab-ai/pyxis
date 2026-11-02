// Package attribution is the Argus attribution gate — the authorship-scoped
// protected-path merge-boundary refusal (docs/tech-design/argus-attribution-gate.md).
// It is the unbypassable merge-boundary backstop the write hook's fast layer
// cannot be (§1): a required CI check that reads each of a PR's diffs, derives
// whether the diff is Argus-authored from an in-tree commit trailer — never a
// mutable platform badge, never an ambient credential (§3.1, DINV-3) — and
// refuses the PR when an Argus-authored diff touches a protected path, using the
// shared classifier and scope resolver (internal/argus/boundary) for the
// boundary (§3.2). When authorship is underivable on a protected-path diff it
// fails closed for that protected-path subset (§3.3, DINV-2); a human-authored
// diff to the same paths is never refused (§3.4, DINV-4).
//
// It composes as a self-registered docgate gate check (§3.4, Q3) — a base-free
// contribution the docgate umbrella runs in the already-required docs-gate CI
// context, adding no hand-edited shared enumeration line (AP-27). The gate
// registration lives in internal/docgate/gate/reg_check_attribution.go and
// delegates to Evaluate; the derivation reads only the repo via `git`, so it
// runs identically at a desk and in CI with no ambient credential.
//
// This file is the pure, hermetic core: given the PR's changed files each
// already carried its derived authorship, and an author-independent scope
// resolver, it produces the refusal set. It reads no git and no clock — a
// planted directive in a commit body or a diff cannot steer its structural
// verdict (trailer presence + protected-path classification). The git
// derivation that feeds it the authorship signal lives in derive.go.
package attribution

import (
	"fmt"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Verb names this gate in every defect it emits (the standard defect record's
// verb field — schema.Defect) and is the docgate check verb the registration
// contributes, so a refusal is attributable to the attribution gate and not a
// sibling merge-boundary arm.
const Verb = "check-argus-attribution"

// TrailerKey is the stable, documented commit-trailer key the Argus drive
// appends to each commit it authors, identifying the commit as Argus-authored
// and naming the driving problem: a `Argus-Drive: <slug>` trailer in the
// commit's final trailer block (§3.1). The key is a contractual, in-tree signal
// (AP-6) — the gate reads it structurally, never guessing at free-form prose.
// The trailer only *identifies* the driving problem; it never *bounds* the
// drive's scope, which the resolver derives author-independently (§3.2.1).
const TrailerKey = "Argus-Drive"

// Author is a changed diff's derived authorship — the trailer-presence verdict
// the gate keys on. Exactly one applies to any changed file.
type Author int

const (
	// Human — the state the gate treats as positively-human: passed, never
	// refused (DINV-5, NR-4). The git derivation reaches it only when authorship
	// is POSITIVELY established — no produced Argus stamp on the diff AND the PR
	// carries the positive human-authorship attestation (§3.3, resolveState).
	// Absence of any Argus signal is no longer human by itself (the frozen
	// masquerade); an unattested absence derives Underivable, not Human. The
	// hermetic core takes this value as an already-resolved positively-human diff.
	Human Author = iota
	// Argus — the diff carries a produced, content-bound `Argus-Drive` stamp on
	// at least one introducing commit (§3.1, §3.2). Positively-Argus WINS the
	// precedence: the diff is classified against the drive's scope and refused on
	// a protected-path hit (DINV-1) regardless of any human attestation also
	// present (the both-signals collision refuses — a drive-plantable attestation
	// cannot launder it; DINV-5).
	Argus
	// Underivable — authorship cannot be positively established for the diff: an
	// unverifiable Argus trailer (present but with no derivable slug, no binding,
	// or a binding that does not re-derive — a REPLAYED trailer, §3.2), an
	// unresolvable/unreadable introducing commit, or an absent Argus signal with
	// no positive human attestation on a protected-path diff. On a protected-path
	// diff this fails closed (DINV-2); a positively-established human diff is
	// never Underivable, so the human-author path stays open.
	Underivable
)

// String names the authorship verdict for a defect reason.
func (a Author) String() string {
	switch a {
	case Human:
		return "human-authored"
	case Argus:
		return "Argus-authored"
	case Underivable:
		return "authorship-underivable"
	}
	return "unknown-authorship"
}

// Change is one of a PR's changed files with its already-derived authorship and
// — for an Argus-authored change — the driving problem slug read from the
// trailer. The core takes Change values; derive.go builds them from git.
type Change struct {
	Path   string // repo-relative changed path
	Author Author // the trailer-presence verdict for this diff
	Slug   string // the driving problem slug (Argus only; from the trailer)
}

// ScopeResolver resolves a drive's declared scope for a named problem slug from
// an author-independent source (the named problem's committed delivery ledger,
// or — on a brand-new problem's first PR — a slug-derived fixed grammar). It is
// injected so the core stays hermetic and table-testable; derive.go supplies
// the real git-backed resolver. A nil result means the scope is unresolvable
// (e.g. a malformed committed ledger), which a consumer fails closed on (AP-17).
type ScopeResolver func(slug string) *boundary.Scope

// Refusals is the gate's verdict over a PR's changed files: the set of refused
// diffs, each a schema.Defect naming the path, the boundary it hit, and a
// remedy. The result is deterministic (sorted by path). An empty result is a
// pass — the PR carries no Argus-authored (or underivable) protected-path diff.
//
// Per change:
//   - Human — never refused (DINV-4); the human-author path is open regardless
//     of the target, including a human PR to the authority contract.
//   - Argus — classified against the drive's declared scope; refused unless the
//     target is provably in-scope. A protected class (authority contract /
//     governance document / another problem's ledger / out of declared scope)
//     refuses (DINV-1); an unresolvable scope refuses fail-closed (AP-17).
//   - Underivable — refused only when the target is one of the fixed,
//     scope-independent protected classes (the authority contract or a
//     governance document): the protected-path subset the gate can classify
//     without a resolved drive (DINV-2). A non-protected-path underivable diff
//     is not refused (the fail-closed rule is scoped to the protected subset).
func Refusals(cfg config.Config, changes []Change, resolve ScopeResolver) []schema.Defect {
	var out []schema.Defect
	for _, ch := range changes {
		switch ch.Author {
		case Human:
			// The anti-segment stays served: a human diff is never refused here.
			continue
		case Argus:
			if d := argusRefusal(cfg, ch, resolve); d != nil {
				out = append(out, *d)
			}
		case Underivable:
			if d := underivableRefusal(cfg, ch); d != nil {
				out = append(out, *d)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Op < out[j].Op
	})
	return out
}

// argusRefusal classifies an Argus-authored change against its drive's declared
// scope and returns a refusal for anything not provably in-scope. The scope
// width comes from the resolver (author-independent), never from the trailer's
// problem field, so a self-named broader slug cannot reclassify an out-of-scope
// write as in-scope (§3.2.1, DINV-6).
func argusRefusal(cfg config.Config, ch Change, resolve ScopeResolver) *schema.Defect {
	d := boundary.Drive{Slug: ch.Slug, Scope: resolve(ch.Slug)}
	class := boundary.Classify(cfg, ch.Path, d)
	if class == boundary.InScope {
		return nil
	}
	def := refusal(ch.Path, ch.Author, class)
	return &def
}

// underivableRefusal fails closed on the protected-path subset the gate can
// classify with no resolved drive: the fixed, scope-independent classes (the
// authority contract and the governance documents). It classifies against an
// empty drive — with no slug and no scope, boundary.Classify fires only those
// fixed path-based classes and reports every scope-dependent target
// Unresolvable, which this layer does NOT refuse (a non-protected-path
// underivable diff passes; the fail-closed rule is scoped to the protected
// subset — DINV-2).
func underivableRefusal(cfg config.Config, ch Change) *schema.Defect {
	class := boundary.Classify(cfg, ch.Path, boundary.Drive{})
	switch class {
	case boundary.AuthorityContract, boundary.GovernanceDocument:
		def := refusal(ch.Path, ch.Author, class)
		return &def
	}
	return nil
}

// refusal builds the standard defect record (verb, file, op, reason, remedy)
// for a refused diff — the reason names the path, the boundary class it hit,
// and the authorship; the remedy is the actionable next step for that class.
func refusal(path string, author Author, class boundary.Class) schema.Defect {
	return schema.Defect{
		Verb:   Verb,
		File:   path,
		Op:     opFor(class),
		Reason: fmt.Sprintf("%s diff touches %s (%s) — the attribution gate refuses it at the merge boundary", author, path, class),
		Remedy: remedyFor(class),
	}
}

// opFor is the short, stable op token naming which boundary a refusal hit — the
// defect record's op field, a machine-stable classification key.
func opFor(class boundary.Class) string {
	switch class {
	case boundary.AuthorityContract:
		return "authority-contract"
	case boundary.GovernanceDocument:
		return "governance-document"
	case boundary.OtherProblemLedger:
		return "other-problem-ledger"
	case boundary.OutOfScope:
		return "out-of-scope"
	case boundary.Unresolvable:
		return "unresolvable-scope"
	}
	return "protected-path"
}

// remedyFor is the actionable next step for a refused boundary class, so the
// refusal reads as a boundary signal and not a transient failure.
func remedyFor(class boundary.Class) string {
	switch class {
	case boundary.AuthorityContract, boundary.GovernanceDocument, boundary.OtherProblemLedger:
		return "a human lands this protected-path change by their own PR — Argus never authors it"
	case boundary.OutOfScope:
		return "confine the drive to its declared scope, or split the out-of-scope change out and land it as a human-authored PR"
	case boundary.Unresolvable:
		return "the drive's declared scope is unresolvable (e.g. a malformed committed delivery ledger) — resolve it before the write can be judged in-scope"
	}
	return "a human lands this change by their own PR"
}
