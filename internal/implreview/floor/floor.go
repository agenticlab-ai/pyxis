// Package floor is the deterministic mechanical-floor exclusion (FR-4,
// docs/tech-design/impl-review-v1.md §2/§3.6, plan T5): it drops any impl-review
// candidate finding that merely RESTATES a defect class codegate's deterministic
// ladder already enforces. The impl-review lenses exist to catch what the ladder
// CANNOT see (judgment); a finding the ladder already blocks on is noise, and a
// human disposing it is wasted attention. The floor filter is the deterministic
// backstop under the lenses' own instruction to not emit floor findings — a
// mechanical drop, never a judgment call, so it is scored by an exact comparator
// (the EVAL-4 basis; the full eval suite is T12).
//
// The enforced class set is read READ-ONLY from codegate (internal/verify), never
// duplicated as a guess:
//
//   - the registered CHECK IDs come from registry.AllChecks() — the authoritative,
//     self-registering source (importing internal/verify/checks runs each check's
//     init() and registers it). This package imports the checks package for that
//     registration side effect and reads the set; it writes nothing under
//     internal/verify.
//   - the finer FINDING CLASSES those checks emit (e.g. "secret", "forbidden-binary",
//     "test-deleted") are string literals inside the check files, not an exported
//     symbol. They are carried here as an explicit, documented set (findingClasses)
//     and GUARDED by a source-scan test (floor_source_test.go) that fails if a
//     `Class:` literal in internal/verify/checks drifts from this set — so the set
//     stays honest without importing or editing internal/verify.
//
// Stdlib only.
package floor

import (
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"

	// Imported for its registration side effect only: each check's init()
	// registers it with the ladder registry, so registry.AllChecks() enumerates
	// the enforced check-ID set. This package reads that set; it never mutates
	// internal/verify.
	_ "github.com/agenticlab-ai/pyxis/internal/verify/checks"
)

// parametricClasses are the execution-fault classes codegate's checks emit via the
// runner's PARAMETRIC path (`Class: string(res.Class)` in internal/verify/checks/
// support.go) rather than as a string literal — so the source-scan guard cannot see
// them and findingClasses does not mirror them. They are read from the runner's
// closed enum (read-only) so the enforced set covers codegate's whole class output,
// literal and parametric alike (closing the parametric-coverage hole).
var parametricClasses = []string{
	string(runner.ClassInfra),
	string(runner.ClassTimeout),
}

// findingClasses is the set of FINDING classes codegate's checks emit that are
// finer than a check ID (the check-ID set is read from the registry — see
// EnforcedClasses). These are the `Class:` literals in internal/verify/checks that
// are NOT equal to their check's ID; a source-scan test (floor_source_test.go)
// pins this set against the actual literals so it cannot silently drift.
//
// Read-only mirror, deliberately explicit: the literals are not exported by
// internal/verify, and this package must not edit internal/verify to export them,
// so the honest options are (a) mirror + guard, or (b) import nothing and scan at
// runtime. (a) is chosen — the set is legible here and the guard test makes drift a
// test failure, not a silent floor gap.
var findingClasses = []string{
	// forbidden_artifacts.go
	"forbidden-binary",
	"forbidden-size",
	"forbidden-path",
	// secret_scan.go
	"secret",
	// machinery_protection.go
	"machinery",
	// testdelta.go
	"test-renamed-out",
	"test-deleted",
	"skip-marker-added",
	"assertion-count-drop",
	// falsification.go
	"tautological-test",
}

// EnforcedClasses returns the full set of defect classes codegate's ladder
// enforces, as a set for O(1) membership: every registered check's ID (read from
// registry.AllChecks() — the authoritative, self-registering source) plus the
// finer finding classes those checks emit (findingClasses). A candidate whose
// restated class is in this set is a floor restatement.
//
// It is computed fresh per call from the live registry so a newly-registered check
// is covered automatically — the floor never lags codegate's enforced set.
func EnforcedClasses() map[string]bool {
	set := make(map[string]bool)
	for _, c := range registry.AllChecks() {
		if c == nil || c.ID == "" {
			continue
		}
		set[c.ID] = true
	}
	for _, cl := range findingClasses {
		set[cl] = true
	}
	for _, cl := range parametricClasses {
		set[cl] = true
	}
	return set
}

// EnforcedClassList returns the enforced set as a sorted slice — for a test or a
// debugging surface that wants a stable enumeration rather than a membership set.
func EnforcedClassList() []string {
	set := EnforcedClasses()
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// FlaggedClasses extracts the defect classes codegate ACTUALLY FLAGGED on the
// reviewed diff from its verdict — the class of every finding and every failed/
// errored check. It is the ground truth NewScreenerForVerdict screens against so
// the floor drops a candidate restating a class codegate really caught on THIS
// proposal (the true FR-4 duplicate), not merely one it could enforce. The verdict
// is the codegate verdict the review context pack already carries (§3.4); this
// reads it read-only. A nil verdict yields no flagged classes.
func FlaggedClasses(v *schema.Verdict) []string {
	if v == nil {
		return nil
	}
	seen := map[string]bool{}
	add := func(c string) {
		if c != "" {
			seen[c] = true
		}
	}
	for _, f := range v.Findings {
		add(f.Class)
	}
	for _, rung := range v.Rungs {
		if rung == nil {
			continue
		}
		for _, chk := range rung.Checks {
			if chk == nil {
				continue
			}
			// A failed or errored check's class is a defect codegate flagged; a pass
			// carries no class (support.go's passOutcome sets none), so this only
			// picks up genuine flags.
			if chk.State == "fail" || chk.State == "error" {
				add(chk.Class)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// RestatedClass extracts the defect class a candidate DECLARES it is about — the
// token after the last "/" in its RuleID (the `<lens>/<class>` identity). This is
// the deterministic key the floor comparator uses: a lens's rule identity names the
// class it judges, so a rule whose class-token IS a codegate-enforced class is a
// restatement by construction. It is a pure string op — no prose matching, no model
// call — so the floor decision is reproducible (the EVAL-4 comparator).
//
// A RuleID with NO "/" is a malformed identity (the contract is `<lens>/<class>`);
// its WHOLE string is treated as the class token so a bare enforced-class name (a
// RuleID of just "secret") is still caught as a restatement rather than silently
// escaping the floor. A trailing "/" (a rule with an empty class token) returns ""
// — a rule that names no class restates none.
func RestatedClass(candidate report.Finding) string {
	id := strings.TrimSpace(candidate.RuleID)
	if id == "" {
		return ""
	}
	i := strings.LastIndex(id, "/")
	if i < 0 {
		// No "/": a malformed identity — treat the whole bare string as the class so
		// a bare enforced-class RuleID cannot slip past the floor.
		return id
	}
	if i == len(id)-1 {
		return "" // trailing "/": empty class token.
	}
	return id[i+1:]
}

// IsFloorRestatement reports whether a candidate restates a codegate-enforced
// class — its RuleID's class-token is in the enforced set. It is the deterministic
// predicate FR-4 rests on: a true return means the candidate duplicates a defect
// the ladder already blocks on, so impl-review must not also report it.
func IsFloorRestatement(candidate report.Finding, enforced map[string]bool) bool {
	class := RestatedClass(candidate)
	if class == "" {
		return false
	}
	return enforced[class]
}

// Screener is a runtime.Screener (the same adversarial-screen seam T4 defined):
// its Survives returns false for a candidate that restates a floor class, so a
// floor restatement is KILLED before it reaches the payload — counted in the
// round's Killed total, never assembled as a finding (FR-4). It composes with T4's
// screen rather than editing it: wrap it around (or chain it with) the runtime's
// DefaultScreener via Compose, and the runtime's existing per-candidate screen loop
// drives both.
//
// It is the DETERMINISTIC class-membership comparator the design specifies for the
// floor (§3.5, "defect-class membership") — a mechanical BACKSTOP, not the sole
// FR-4 guarantee. The primary control that a lens does not restate a ladder class is
// the lens instruction (each lens is told to name its rule for the concern, never a
// codegate class, and never to author a floor finding), and the behavioral guarantee
// is verified by the N≥5 caught-every-run eval families (T12). This screen catches
// the class a candidate DECLARES (its RuleID class-token) being an enforced class;
// NewScreenerForVerdict additionally uses codegate's ACTUAL flagged classes for the
// reviewed diff (the pack already carries the verdict — §3.4), the stronger check.
//
// The enforced set is snapshotted once at construction (NewScreener) so a single
// review round screens against a stable set — a check registering mid-round would
// otherwise shift the floor under the round.
type Screener struct {
	enforced map[string]bool
}

// NewScreener builds a floor Screener snapshotting codegate's current enforced set —
// every class codegate's ladder CAN enforce (the static catalog). Use this when the
// reviewed diff's codegate verdict is not on hand; prefer NewScreenerForVerdict when
// it is.
func NewScreener() *Screener {
	return &Screener{enforced: EnforcedClasses()}
}

// NewScreenerForVerdict builds a floor Screener that screens against the classes
// codegate ACTUALLY FLAGGED on the reviewed diff (the verdict's finding + failed-
// check classes) UNION the static catalog. This is the stronger, ground-truth FR-4
// check the design's context pack enables (§3.4 puts the codegate verdict in the
// pack): a candidate restating a class codegate really flagged on THIS proposal is
// dropped as the true duplicate. flaggedClasses is derived from the verdict by
// FlaggedClasses. The static catalog stays in the set too, so a restatement of an
// enforceable-but-not-this-time class is still floored.
func NewScreenerForVerdict(flaggedClasses []string) *Screener {
	set := EnforcedClasses()
	for _, cl := range flaggedClasses {
		if cl != "" {
			set[cl] = true
		}
	}
	return &Screener{enforced: set}
}

// Compile-time assurance the floor screens ARE runtime.Screener — they plug into
// the T4 runtime seam with no edit to the runtime.
var (
	_ runtime.Screener = (*Screener)(nil)
	_ runtime.Screener = composite{}
)

// Survives implements runtime.Screener: a candidate SURVIVES the floor only if it
// does NOT restate a codegate-enforced class. lens is unused by the floor decision
// (the RuleID carries the lens; the class-token is what collides), accepted to
// satisfy the Screener signature.
func (s *Screener) Survives(_ string, candidate report.Finding) bool {
	return !IsFloorRestatement(candidate, s.enforced)
}

// Compose chains two candidate screens into one runtime.Screener that kills a
// candidate if EITHER kills it (a candidate survives only if BOTH survive it). It
// is how the floor Screener stacks on T4's DefaultScreener (or any richer T5
// judgment screen) without either editing the other: the composite is one
// runtime.Screener the runtime drives through its existing per-candidate loop —
// hand it directly to runtime.ReviewRequest.Screener.
//
// Both arguments must be non-nil (a nil screen would silently pass everything,
// defeating the floor); Compose panics on a nil argument — a programmer error, not
// a runtime condition.
func Compose(a, b runtime.Screener) runtime.Screener {
	if a == nil || b == nil {
		panic("floor.Compose: nil screener")
	}
	return composite{a: a, b: b}
}

type composite struct{ a, b runtime.Screener }

// Survives implements runtime.Screener: a candidate survives only if both component
// screens let it survive; either one killing it kills the candidate.
func (c composite) Survives(lens string, candidate report.Finding) bool {
	return c.a.Survives(lens, candidate) && c.b.Survives(lens, candidate)
}
