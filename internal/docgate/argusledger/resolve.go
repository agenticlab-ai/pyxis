package argusledger

import (
	"regexp"
	"strings"
)

// AnchorResolution classifies where a recognized `delivery:<slug>#<anchor>`
// citation lands once its target ledger is parsed — the cross-document arm's
// region resolution (docs/tech-design/argus-ledger-lint.md §3.2). The three
// outcomes are the arm's three verdicts: a journal `E-n` is legal, a
// derived-state field is the DINV-6 defect, and an anchor resolving to neither
// is the distinct unresolvable-anchor defect (UB-2).
type AnchorResolution int

const (
	// AnchorUnresolved — the anchor names neither a journal E-n entry nor a
	// derived-state field within a real ledger (a dangling/malformed anchor).
	AnchorUnresolved AnchorResolution = iota
	// AnchorJournal — the anchor names a `## Journal` E-n entry — legal (FR-4).
	AnchorJournal
	// AnchorDerived — the anchor names a field within the `## Derived state`
	// region, which is never citable — the DINV-6 defect (FR-3).
	AnchorDerived
)

// derivedFieldRe matches a derived-state table row's bold label in its first
// column — `| **Status** | … |` (argus-artifacts §3.2's derived-block field
// table). Only the row label is a citable-looking anchor target; a bold span
// inside a value cell is not a field, so the label is anchored to the line
// start's first-column position, never any `**…**` anywhere in the region.
var derivedFieldRe = regexp.MustCompile(`^\|\s*\*\*([^*|]+?)\*\*\s*\|`)

// ResolveAnchor classifies a citation anchor (the text after `#`) against this
// parsed delivery ledger's two regions. It is meaningful only on a delivery
// ledger (the one class with a derived-state block); a problem ledger's A-n
// entries are not a `delivery:` target (§3.2, #O-4).
func (l *Ledger) ResolveAnchor(anchor string) AnchorResolution {
	if _, ok := l.Entries[anchor]; ok {
		return AnchorJournal
	}
	if l.Derived != nil {
		for _, f := range DerivedFields(l.Derived.Bytes) {
			if f == anchor {
				return AnchorDerived
			}
		}
	}
	return AnchorUnresolved
}

// DerivedFields returns the field labels of a delivery ledger's derived-state
// region — each `**Label**` first-column cell of the region's field table.
func DerivedFields(regionBytes []byte) []string {
	var out []string
	for _, line := range strings.Split(string(regionBytes), "\n") {
		if m := derivedFieldRe.FindStringSubmatch(line); m != nil {
			out = append(out, strings.TrimSpace(m[1]))
		}
	}
	return out
}
