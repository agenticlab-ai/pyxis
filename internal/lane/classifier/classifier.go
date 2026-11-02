// Package classifier is the merge lane's one hermetic predicate — the single
// verb that routes every proposal into exactly one lane (merge-lane-v1 §3.1,
// DINV-1/DINV-2; plan T2). Classification is a fixed pipeline evaluated in a
// fixed order: the policy's path-derived, whole-diff FLOOR first (any hit is
// terminal HUMAN, naming the class hit); class membership second (a floor-clear
// proposal is an AUTO candidate only when its ENTIRE diff falls inside exactly
// one ratified AUTO class's perimeter); and a configured, downgrade-only
// intelligence stage last (it may only turn AUTO→HUMAN, and its ambiguity,
// timeout, or error routes HUMAN). HUMAN is the default lane: an unloadable
// policy, an empty or misparsed diff, an out-of-perimeter hunk, or any
// unclassifiable input all land HUMAN with the reason recorded — never a third
// state, never silently AUTO (DINV-1, UB-1; incident I3's fail-closed lesson).
//
// The verb consumes only diff-derived data — the diff's paths and its
// diff-derived change classes — never author-declared metadata (DINV-2), and it
// is a pure function of (diff, policy, downgrade stage): the same pinned inputs
// reproduce the same verdict byte-for-byte (DINV-4). The acting layer, the
// verdict record, the reviewer-verdict precondition, and the check-state reads
// are downstream (plan T3/T4); this package decides the lane and pins the
// inputs the verdict depends on. Stdlib only, per the repo Go rules.
package classifier

import (
	"encoding/json"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// Lane is the binary routing outcome. There is no third state: everything not
// positively classified AUTO-eligible is HUMAN (DINV-1).
type Lane string

const (
	Auto  Lane = "AUTO"
	Human Lane = "HUMAN"
)

// Diff is the one proposal-derived input, consumed as data (DINV-9): the whole
// set of paths the proposal touches and its diff-derived change classes (e.g.
// governing-doc-edit, ledger-frozen-entry-edit). Both are computed upstream
// from the diff alone — never from author-declared metadata (DINV-2); this
// package matches them against the policy, it does not derive them.
type Diff struct {
	Paths   []string
	Classes []string
}

// Pinned is the set of inputs a verdict depends on, recorded so that replaying
// them reproduces the verdict byte-identically (DINV-4). Base+Head are the
// merge-base/head pair the diff derives from — the diff reconstructs from them,
// so a verdict replays even after the proposal merges or its base moves.
type Pinned struct {
	Base          string `json:"base"`
	Head          string `json:"head"`
	PolicyVersion string `json:"policy_version"`
	CheckState    string `json:"check_state"`
}

// Inputs bundles one evaluation's proposal-derived diff and its pinned
// revision/check-state context. The policy is passed separately because it is
// always loaded from the trusted mainline context, never from the proposal's
// tree (DINV-9).
type Inputs struct {
	Diff       Diff
	Base       string
	Head       string
	CheckState string
}

// Verdict is the classifier's typed output: exactly one lane, a recorded
// reason, the floor hits when the floor fired, the matched AUTO class when the
// lane is AUTO, and the pinned inputs the verdict depends on. Every field is
// data with a fixed order, so Record serializes deterministically (DINV-4).
type Verdict struct {
	Lane      Lane     `json:"lane"`
	Reason    string   `json:"reason"`
	FloorHits []string `json:"floor_hits,omitempty"`
	AutoClass string   `json:"auto_class,omitempty"`
	Pinned    Pinned   `json:"pinned"`
}

// Downgrader is the optional, configured intelligence stage (FR-4). It is
// consulted only for an otherwise-AUTO proposal and, by construction, can only
// downgrade AUTO→HUMAN — it never sees a HUMAN verdict and cannot upgrade one.
// Its ambiguity, timeout, or error must surface as a non-nil err (or
// downgrade=true): the classifier treats any of them as fail-closed HUMAN. It
// is deferred by default (a nil Downgrader means no stage is configured).
type Downgrader interface {
	// Downgrade reports whether the AUTO candidate should be downgraded to
	// HUMAN, with reason naming why. A non-nil err is fail-closed: the lane
	// routes HUMAN naming the stage error.
	Downgrade(Diff) (downgrade bool, reason string, err error)
}

// Classify runs the fixed floor→classes→downgrade pipeline over one proposal
// and returns exactly one lane verdict, fail-closed on every uncertain input.
// It is pure: given equal (Inputs, policy, Downgrader) it returns an equal
// Verdict, whose Record is byte-identical across runs (DINV-4).
func Classify(in Inputs, p *policy.Policy, dg Downgrader) Verdict {
	pin := Pinned{Base: in.Base, Head: in.Head, CheckState: in.CheckState}
	if p != nil {
		pin.PolicyVersion = p.SchemaVersion
	}
	human := func(reason string, hits []string) Verdict {
		return Verdict{Lane: Human, Reason: reason, FloorHits: hits, Pinned: pin}
	}

	// An unloadable policy means the lane is all-HUMAN — the same fail-closed
	// posture the loader takes on any load defect (DINV-8).
	if p == nil {
		return human("no risk policy is loaded; the lane stays all-HUMAN (DINV-8)", nil)
	}
	// The kill switch flips the whole lane HUMAN-only in one data change (A-4).
	if p.HumanOnly() {
		return human("the kill switch is engaged: the lane is HUMAN-only", nil)
	}
	// Fail closed on an empty or misparsed diff before any AUTO decision — a
	// proposal we cannot read whole is refused, never guessed (UB-1; I3).
	if len(in.Diff.Paths) == 0 {
		return human("the diff has no parsed paths; fail-closed HUMAN (UB-1)", nil)
	}

	// Floor first — evaluated before any other signal, and a hit is terminal:
	// no class membership, downgrade opinion, or author claim can route a
	// floor-hit proposal AUTO (DINV-2).
	if hits := floorHits(in.Diff, p.Floor); len(hits) > 0 {
		return human("floor hit — a hard-gated surface changed: "+strings.Join(hits, ", "), hits)
	}

	// Classes second — AUTO only when the WHOLE diff falls inside exactly one
	// ratified AUTO class's perimeter (merge-lane-v1 §3.1). Zero containing
	// classes (an out-of-perimeter hunk) or more than one (ambiguous
	// membership) both fail closed to HUMAN.
	name, matched := containingClass(in.Diff, p.AutoClasses)
	switch matched {
	case 1:
		// AUTO candidate — fall through to the intelligence stage.
	case 0:
		return human("no AUTO class perimeter contains the whole diff", nil)
	default:
		return human("the whole diff is claimed by multiple AUTO classes (ambiguous); fail-closed HUMAN", nil)
	}

	// Intelligence last and optional — downgrade-only, fail-closed (FR-4).
	if dg != nil {
		down, reason, err := dg.Downgrade(in.Diff)
		switch {
		case err != nil:
			return human("the intelligence stage failed; fail-closed HUMAN: "+err.Error(), nil)
		case down:
			return human("the intelligence stage downgraded AUTO→HUMAN: "+reason, nil)
		}
	}

	return Verdict{
		Lane:      Auto,
		Reason:    "the whole diff is contained by AUTO class " + name,
		AutoClass: name,
		Pinned:    pin,
	}
}

// floorHits returns the hard-gated surfaces this diff touches, in policy
// declaration order (deterministic): a `path:<glob>` token for every floor
// glob some diff path matches, then a `class:<name>` token for every
// hard-gated change class the diff carries. An empty result means the floor is
// clear. The result derives from the diff's paths and classes only (DINV-2).
func floorHits(d Diff, f policy.Floor) []string {
	var hits []string
	for _, glob := range f.HardGatedPaths {
		for _, path := range d.Paths {
			if policy.GlobMatch(glob, path) {
				hits = append(hits, "path:"+glob)
				break
			}
		}
	}
	have := make(map[string]bool, len(d.Classes))
	for _, c := range d.Classes {
		have[c] = true
	}
	for _, cls := range f.HardGatedClasses {
		if have[cls] {
			hits = append(hits, "class:"+cls)
		}
	}
	return hits
}

// containingClass counts the AUTO classes whose perimeter contains the WHOLE
// diff and returns the sole one's name when exactly one matches. Membership is
// all-or-nothing per class: every diff path must fall inside that class's
// perimeter (merge-lane-v1 §3.1). matched==1 is the only AUTO-eligible case.
func containingClass(d Diff, classes []policy.AutoClass) (name string, matched int) {
	for _, c := range classes {
		if perimeterContains(c, d.Paths) {
			name = c.Name
			matched++
		}
	}
	if matched != 1 {
		name = ""
	}
	return name, matched
}

// perimeterContains reports whether every path in the diff falls inside the
// class's whole-diff perimeter. A single out-of-perimeter path fails the whole
// diff; an empty path set is never contained (fail-closed).
func perimeterContains(c policy.AutoClass, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		inside := false
		for _, glob := range c.Paths {
			if policy.GlobMatch(glob, path) {
				inside = true
				break
			}
		}
		if !inside {
			return false
		}
	}
	return true
}

// Record is the verdict's byte-identical serialization — the pinned-input
// record whose replay (same base+head+policy version+check state, same
// reconstructed diff) reproduces the verdict byte-for-byte (DINV-4).
// Deterministic by construction: the struct's field order is fixed and every
// slice is built in a fixed order.
func (v Verdict) Record() []byte {
	b, err := json.Marshal(v)
	if err != nil {
		// Verdict carries only strings and string slices, so Marshal cannot
		// fail; this guards a future field that breaks that (programmer error)
		// rather than emitting a partial, non-replayable record.
		panic("classifier: verdict marshal: " + err.Error())
	}
	return b
}
