package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Validate enforces the design-bound invariants a well-formed policy must
// satisfy before any reader trusts it (merge-lane-v1 §3.1, §3.6): the floor
// covers the self-unlock minimum (paths AND classes), and no AUTO class
// claims the lane's build inputs in its perimeter. A policy failing either
// refuses to load — the two-step ratify-then-plant path is closed at the
// validator, under every policy state (DINV-6). Validate is pure and runs
// after Parse over a structurally-sound Policy; Load composes both plus the
// ratified-lineage read.
func Validate(p *Policy) []schema.Defect {
	var defects []schema.Defect

	// self-unlock minimum: every design-bound glob/class must be in the floor.
	// Ratification may WIDEN the floor, never shrink below this set.
	have := set(p.Floor.HardGatedPaths)
	for _, glob := range SelfUnlockMinPaths {
		if !have[glob] {
			defects = append(defects, def("validate",
				fmt.Sprintf("floor omits self-unlock-minimum path %q", glob),
				"the floor must cover the whole self-unlock minimum (merge-lane-v1 §3.1); widen it, never shrink below the minimum (DINV-6)"))
		}
	}
	haveClass := set(p.Floor.HardGatedClasses)
	for _, cls := range SelfUnlockMinClasses {
		if !haveClass[cls] {
			defects = append(defects, def("validate",
				fmt.Sprintf("floor omits self-unlock-minimum class %q", cls),
				"the floor must hard-gate the governing and frozen-ledger classes (merge-lane-v1 §3.1, DINV-6)"))
		}
	}

	// build-input claim: no AUTO class may cover go.mod / go.sum. A class
	// perimeter and the build inputs may not overlap (merge-lane-v1 §3.6).
	for _, c := range p.AutoClasses {
		for _, glob := range c.Paths {
			for _, bi := range BuildInputs {
				if globMatch(glob, bi) {
					defects = append(defects, def("validate",
						fmt.Sprintf("AUTO class %q claims the lane build input %q via perimeter glob %q", c.Name, bi, glob),
						"remove the build inputs from every AUTO class perimeter — they are the lane's trust base and stay floor-gated (merge-lane-v1 §3.6, DINV-6)"))
				}
			}
		}
	}

	return defects
}

func set(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}

// GlobMatch reports whether a whole-diff path glob admits the given path,
// using the lane's one glob semantics (`**` spans `/`, `*` spans a run of
// non-`/`). It is the single perimeter/floor matcher every reader shares: the
// classifier (plan T2) matches both the floor's hard-gated paths and each AUTO
// class's whole-diff perimeter through it, so the glob semantics never drift
// between the validator's build-input check and the classifier (AP-11).
func GlobMatch(pattern, name string) bool { return globMatch(pattern, name) }

// globMatch reports whether a whole-diff path glob would admit the given
// path. `**` spans any characters including `/`; `*` spans any run of
// non-`/` characters; every other rune is literal. Used only to decide
// whether an AUTO class perimeter reaches a build input — deterministic,
// no filesystem.
func globMatch(pattern, name string) bool {
	re, err := regexp.Compile("^" + globToRegexp(pattern) + "$")
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

func globToRegexp(pattern string) string {
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	return b.String()
}
