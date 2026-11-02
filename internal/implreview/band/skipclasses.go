package band

// The skip set — the lane AUTO classes a review may skip (FR-9's "a trivial
// AUTO-eligible class → skip"). This is DATA on the lane's OWN class NAMES, not
// a review-side risk model: a class here names a lane-ratified AUTO perimeter
// (from the risk policy's AutoClasses) whose whole-diff membership the lane
// already deemed safe AND whose content is mechanical/docs-only enough that a
// judgment review adds no signal — a rename-only, docs-only, or generated-only
// perimeter. The lane owns which diffs fall in the class (the classifier); this
// set only names WHICH already-safe classes are also review-trivial.
//
// The defaults below are the classes the brainstorm named review-trivial
// (design-brainstorm:impl-review#O — "mechanical/docs-only/rename-only"),
// expressed as the lane's conventional AUTO-class names. A policy that ratifies
// a class under a different name simply never matches (it reviews at STANDARD —
// the fail-closed side), so the set is honest to the loaded policy: adding a new
// trivial class is a one-line data change here, never a code-logic change, and
// an unknown class is never guessed-skipped.
//
// It is deliberately a SMALL, LEGIBLE literal set rather than a heuristic over
// class content: a review-side heuristic would be a second risk model (AP-11
// forbids it). The class NAME is the whole key.
var defaultSkipClasses = []string{
	"drive-doc", // a docs-only drive edit — the lane's docs perimeter; no code to judge
	"rename",    // a pure rename with no behavior change
	"generated", // a regenerated/generated-artifact perimeter — nothing hand-authored to review
}

// skipClasses is the active skip set. It is a package var (seeded from
// defaultSkipClasses) so a caller could, in a later phase, source it from the
// pyxis-config surface (issue #4) instead of the built-in default — the same
// data-as-config posture the lane's policy takes — without touching Select's
// logic. Today it is the built-in default.
var skipClasses = appendClasses(nil, defaultSkipClasses...)

// isSkipClass reports whether the given lane AUTO class name is in the skip set
// — the deterministic membership Select consults. An empty name is never a skip
// (a skip must name its class; an unnamed AUTO class reviews at standard).
func isSkipClass(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range skipClasses {
		if c == name {
			return true
		}
	}
	return false
}

// appendClasses copies class names into a fresh slice — a tiny helper so the
// package var is an owned copy (a caller re-seeding the set never aliases the
// default literal).
func appendClasses(dst []string, names ...string) []string {
	out := append([]string(nil), dst...)
	return append(out, names...)
}
