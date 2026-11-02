package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// The fresh-instance identity (§3.4, FR-2/NFR-3, DINV-5).
//
// The reviewer runs as a fresh top-level session that is a SIBLING of the
// implementing worker — never a child of it. A child would inherit the
// implementer's context and defeat FR-2: a compromised implementing agent
// could see or steer the review from inside. The four lens subagents are
// children of the REVIEWER root, and each round records a context_id derived
// from that reviewer root plus a provenance-check that the reviewer context is
// distinct from the implementing session's root.
//
// This file carries that identity in Go: a reviewer root is an opaque session
// identity (the runtime supplies the real one; a test supplies a distinct
// string). ContextID derives the round's fresh-instance id from it
// deterministically, and CheckFreshInstance is the would-fail-if-inherited leg
// A-6 asserts — it FAILS when the reviewer context equals or derives from the
// implementing session's.

// ErrInheritedContext is the sentinel A-6's would-fail-if-inherited leg asserts:
// the reviewer context resolves to (or derives from) the implementing session's
// own context, so the review is not independent of a possibly-compromised
// implementer (FR-2/NFR-3/DINV-5). A caller branches on it with errors.Is.
var ErrInheritedContext = errors.New("impl-review runtime: reviewer context inherited from the implementing session (not a sibling spawn)")

// ErrEmptyRoot is returned when a session-root identity is empty — a missing
// root cannot be provenance-checked, so the runtime fails closed rather than
// certify independence over nothing.
var ErrEmptyRoot = errors.New("impl-review runtime: empty session root")

// contextIDPrefix marks a derived reviewer context_id so the derivation is
// self-describing in the payload (the round's context_id, §3.1). It also lets
// CheckFreshInstance recognize a context_id that was DERIVED from the
// implementing root (the "derives-from" leg of DINV-5), not just one that
// equals it.
const contextIDPrefix = "implreview-ctx:"

// ContextID derives the round's fresh-instance context_id from the reviewer
// root (§3.1's context_id, §3.4's "each context_id derives from the reviewer
// root"). It is deterministic — the same reviewer root always yields the same
// context_id — with no clock or randomness, so a round's provenance is
// reproducible. A caller records the returned id on report.Round.ContextID.
//
// The id is prefixed and content-addressed over the root so it is
// recognizable (CheckFreshInstance can tell a reviewer-root-derived id from an
// implementer-root-derived one) without leaking the raw root.
func ContextID(reviewerRoot string) (string, error) {
	root := strings.TrimSpace(reviewerRoot)
	if root == "" {
		return "", fmt.Errorf("%w: reviewer root", ErrEmptyRoot)
	}
	return contextIDPrefix + digestRoot(root), nil
}

// digestRoot content-addresses a session root to a short, stable hex digest —
// the derivation both ContextID and CheckFreshInstance share so a context_id
// derived from a given root is recognizable by re-deriving from that root.
func digestRoot(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])
}

// CheckFreshInstance is the provenance check A-6's would-fail-if-inherited leg
// exercises (§3.4, DINV-5, FR-2/NFR-3). It FAILS with ErrInheritedContext when
// the reviewer context equals or DERIVES FROM the implementing session's root —
// the two legs of "not a sibling spawn":
//
//   - equals: the reviewer root IS the implementing root (a child session
//     reuses the parent's session identity), or
//   - derives-from: the reviewer root's derived context_id equals the id that
//     would derive from the implementing root (the reviewer context was seeded
//     from the implementer's).
//
// Both empty roots fail closed (ErrEmptyRoot): a missing root cannot certify
// independence. When the reviewer root is a genuinely distinct session — a true
// sibling spawn — CheckFreshInstance returns nil.
func CheckFreshInstance(reviewerRoot, implementerRoot string) error {
	rr := strings.TrimSpace(reviewerRoot)
	ir := strings.TrimSpace(implementerRoot)
	if rr == "" {
		return fmt.Errorf("%w: reviewer root", ErrEmptyRoot)
	}
	if ir == "" {
		return fmt.Errorf("%w: implementer root", ErrEmptyRoot)
	}

	// Leg 1 — equals: the reviewer root IS the implementing root.
	if rr == ir {
		return fmt.Errorf("%w: reviewer root equals the implementing session root", ErrInheritedContext)
	}
	// Leg 2 — derives-from: the reviewer context_id would equal the implementer
	// context_id — the reviewer context was seeded from the implementer's, so it
	// is not a fresh instance even though the raw roots differ.
	if digestRoot(rr) == digestRoot(ir) {
		return fmt.Errorf("%w: reviewer context derives from the implementing session's", ErrInheritedContext)
	}
	return nil
}

// Provenance builds the round's fresh-instance report.Provenance (§3.1) from
// the reviewer root and the model/prompts/calibration versions this round ran
// under. It is a convenience the runtime uses when assembling a round so the
// context_id and provenance are recorded together, consistently.
func Provenance(reviewerRoot string, p report.Provenance) (contextID string, prov report.Provenance, err error) {
	id, err := ContextID(reviewerRoot)
	if err != nil {
		return "", report.Provenance{}, err
	}
	return id, p, nil
}
