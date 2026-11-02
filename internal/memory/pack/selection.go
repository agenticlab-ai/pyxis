package pack

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/agenticlab-ai/pyxis/internal/memory/association"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

func validateScope(scope Scope) error {
	for _, code := range scope.Code {
		if code == "." || path.IsAbs(code) || path.Clean(code) != code || code == ".." || strings.HasPrefix(code, "../") || strings.ContainsAny(code, "\\\x00\r\n") {
			return fmt.Errorf("invalid repository-relative memory code scope %q", code)
		}
	}
	return nil
}

func matchesCode(r *record.Record, p *resolve.Projection, paths []string) bool {
	for _, scope := range association.Scopes(r, p.Set) {
		for _, code := range paths {
			if association.Matches(scope, code) {
				return true
			}
		}
	}
	return false
}

func decisionPriority(it Item) int {
	if it.Kind != record.Decision {
		return 2
	}
	if it.Family == "D-SERVES" {
		return 1
	}
	return 0
}

func humanRoutingHeading(title string) bool {
	section, rest, _ := strings.Cut(strings.TrimPrefix(title, "§"), " ")
	return strings.TrimSuffix(section, ".") == "0" && strings.Contains(strings.ToLower(rest), "read to rule")
}

// trimKnowledge balances semantic sections by estimated payload, retaining one
// item in each until even that cannot fit. It never silently drops a policy.
func trimKnowledge(pk *Pack) bool {
	groups := []*[]Item{&pk.Requirements, &pk.Decisions, &pk.Learnings, &pk.Narrative}
	for _, floor := range []int{1, 0} {
		var chosen *[]Item
		largest := -1
		for _, group := range groups {
			if len(*group) <= floor {
				continue
			}
			size := 0
			for _, it := range *group {
				size += it.Tokens
			}
			if size > largest {
				chosen, largest = group, size
			}
		}
		if chosen != nil {
			*chosen = (*chosen)[:len(*chosen)-1]
			return true
		}
	}
	return false
}

func literalSectionSelected(d *record.Record, sec docSection, scope Scope, refs map[string]bool) bool {
	if len(scope.Families) == 0 && subjectIn(d.Subject, scope.Subjects) {
		return true
	}
	for ref := range refs {
		sub, family, ok := strings.Cut(ref, "#")
		if !ok || sub != d.Subject {
			continue
		}
		for _, title := range sec.ancestors {
			first, _, _ := strings.Cut(title, " ")
			if first == family {
				return true
			}
			if suffix, legacy := strings.CutPrefix(family, "decision."); legacy {
				if dot := strings.LastIndexByte(suffix, '.'); dot > 0 && strings.TrimSuffix(first, ".") == suffix[:dot] {
					return true
				}
			}
		}
		// PRD rows and exact native embeds may occur in a literal source. Match
		// delimited identities rather than treating FR-1 as a hit on FR-10.
		tokens := strings.FieldsFunc(sec.text, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '.' && r != '_'
		})
		if slices.Contains(tokens, family) {
			return true
		}
	}
	return false
}
