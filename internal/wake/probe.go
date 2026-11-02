package wake

import (
	"context"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// verb names this package in every defect it raises, so a shared defect surface
// attributes the wake channel.
const verb = "wake"

// The closed goal-object schemes the backstop probes. A goal object is
// `<scheme>:<ref>` (the await grammar's convention — `pr:214`,
// `item:argus/epic#7`): a merge is verified by mainline ancestry, an item by
// record existence (§3.2 — "ancestry for merges, record existence for items and
// appends"; AP-15/AP-18/AP-26).
const (
	schemePR   = "pr"
	schemeItem = "item"
)

// Prober is the backstop's READ-ONLY view of the platform — the exact goal-object
// reads that decide whether an awaited completion has happened (§3.2). It carries
// no credential and performs no mutation: a probe is a read of the media the repo
// already binds (git ancestry, a tracker record), never a status feed or a
// declared state (AP-18 — behavior keys on the object's existence, not a badge).
// The git/tracker implementation is the host adapter's (per-adapter, out of
// scope for the tick); the tests substitute a stub, so the tick's dispatch and
// convergence are proven hermetically.
type Prober interface {
	// AncestorOfMainline reports whether the merged content of a `pr:<ref>` goal
	// object is an ancestor of the mainline — the exact read that completes a
	// merge await (AP-26/I2). A merged badge whose content is not an ancestor is
	// a false completion and returns false, never a lying true.
	AncestorOfMainline(ctx context.Context, ref string) (bool, error)
	// RecordExists reports whether the tracker record an `item:<ref>` goal object
	// names exists (AP-18) — the exact read that completes an item or append
	// await. Existence is the record on the tracker itself, never a platform
	// event.
	RecordExists(ctx context.Context, ref string) (bool, error)
}

// splitScheme splits a goal object into its `<scheme>:<ref>` parts on the first
// colon, so a ref may itself contain colons (a URL, a nested id). Both parts
// must be non-empty; otherwise the goal object is unschemed and ok is false.
func splitScheme(goalObject string) (scheme, ref string, ok bool) {
	s, r, found := strings.Cut(strings.TrimSpace(goalObject), ":")
	if !found || s == "" || strings.TrimSpace(r) == "" {
		return "", "", false
	}
	return s, strings.TrimSpace(r), true
}

// probeAwait performs the one exact read an await's goal-object scheme calls for
// and reports whether the completion is verified. An unrecognized or unschemed
// goal object is a NAMED defect, never a silent false completion — the backstop
// fails loud on a goal object it cannot probe rather than resting an await
// forever (the closed-grammar discipline extended to the probe). A read error is
// returned as an error (an infrastructure fault the caller surfaces), distinct
// from a clean not-yet-complete (done=false, no defect).
func probeAwait(ctx context.Context, p Prober, a argusledger.Await) (done bool, defect *schema.Defect, err error) {
	scheme, ref, ok := splitScheme(a.GoalObject)
	if !ok {
		d := unprobeableDefect(a, "the goal object carries no `<scheme>:<ref>` form")
		return false, &d, nil
	}
	switch scheme {
	case schemePR:
		done, err = p.AncestorOfMainline(ctx, ref)
	case schemeItem:
		done, err = p.RecordExists(ctx, ref)
	default:
		d := unprobeableDefect(a, "goal-object scheme "+quote(scheme)+" is not a probeable kind (expected `pr:` or `item:`)")
		return false, &d, nil
	}
	if err != nil {
		return false, nil, err
	}
	return done, nil, nil
}

// unprobeableDefect names an await the backstop cannot probe — a loud defect the
// staleness/state surfaces carry, so an unprobeable await is a fault to fix, not
// a resting state.
func unprobeableDefect(a argusledger.Await, why string) schema.Defect {
	return schema.Defect{
		Verb:   verb,
		File:   a.GoalObject,
		Op:     "probe",
		Reason: "await " + quote(a.GoalObject) + " cannot be probed: " + why,
		Remedy: "register the await with a probeable goal object (`pr:<ref>` for a merge, `item:<ref>` for a tracker item) so the backstop can verify its completion (wake-channel-v1 §3.2)",
	}
}

// quote wraps a value for a readable, unambiguous reason.
func quote(s string) string { return "'" + s + "'" }
