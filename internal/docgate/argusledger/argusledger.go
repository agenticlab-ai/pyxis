// Package argusledger is the one per-class parse both Argus ledger-lint arms
// read (docs/tech-design/argus-ledger-lint.md §3.1, §3.2, §3.4): a single
// pass turns a problem or delivery ledger into an anchor-keyed map of entry
// blocks and, for a delivery ledger, its two section-heading-delimited
// regions — the derived-state block and the journal. One parse, one place,
// so the two arms never diverge on how a ledger is read (AP-11).
//
// The parse is closed against the version-pinned registries in
// internal/docgate/schema: a ledger declaring an unknown template version is
// rejected rather than parsed leniently, and a ledger whose entries or
// derived-state region cannot be delimited yields a named parse defect
// identifying the region — never a crash, never a false pass (UB-3, DINV-2).
package argusledger

import (
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Region is one section-heading-delimited span of a ledger: its `## ` title,
// the body-line range [Start, End) into the ledger's normalized lines, and
// the joined body bytes (one trailing newline).
type Region struct {
	Name  string
	Start int // first body line index, after the heading
	End   int // exclusive
	Bytes []byte
}

// Ledger is one parsed Argus ledger. Entries maps each entry anchor
// ("A-1", "E-3") to its block bytes — the entry from its `### <anchor>`
// heading up to the next entry delimiter or the region/file end (§3.1);
// Order lists the anchors in document order. EntryRegion is the region whose
// body holds the entries (Journal for a delivery ledger, Authority entries
// for a problem ledger); Derived is the delivery ledger's regenerable
// derived-state region, nil for a problem ledger.
type Ledger struct {
	Class   schema.LedgerClass
	Path    string
	Entries map[string][]byte
	// RawEntries preserves each entry's original bytes, including line endings
	// and trailing spaces, without inter-entry separators. Correction digests
	// and retained-history checks must not silently normalize historical truth.
	RawEntries  map[string][]byte
	Order       []string
	EntryRegion *Region
	Derived     *Region
}

// tmplVersionRe matches the `| **Template version** | <value> |` header row.
var tmplVersionRe = regexp.MustCompile(`^\|\s*\*\*Template version\*\*\s*\|\s*(.+?)\s*\|`)

// ledgerPathRes compiles, once per distinct problems root, the two in-scope
// ledger-location patterns anchored at that root — <root>/<slug>/problem.md and
// <root>/<slug>/delivery.md. The problems root is a target-facing doc path the
// engine resolves from config (config-driven-portability-v1 §3.2, FR-1; default
// docs/problems), so the grammar is a pure function of the configured root
// rather than a fixed literal. Compilation is cached because ClassForPath is on
// the hot path (every corpus file, every ls-tree entry) and the root is stable
// across a run.
type ledgerPathRes struct {
	problem  *regexp.Regexp
	delivery *regexp.Regexp
}

var ledgerPathReCache sync.Map // problems-root string → ledgerPathRes

func ledgerPathResFor(problemsRoot string) ledgerPathRes {
	root := path.Clean(strings.ReplaceAll(problemsRoot, "\\", "/"))
	if v, ok := ledgerPathReCache.Load(root); ok {
		return v.(ledgerPathRes)
	}
	q := regexp.QuoteMeta(root)
	res := ledgerPathRes{
		problem:  regexp.MustCompile(`(^|/)` + q + `/[^/]+/problem\.md$`),
		delivery: regexp.MustCompile(`(^|/)` + q + `/[^/]+/delivery\.md$`),
	}
	ledgerPathReCache.Store(root, res)
	return res
}

// ClassForPath returns the ledger class a repo-relative path names, matching
// exactly the two in-scope ledger locations under the configured problems root —
// <problemsRoot>/*/problem.md and <problemsRoot>/*/delivery.md (§3.1). The root
// is resolved from config by the caller (default docs/problems); any other path
// is not an Argus ledger.
func ClassForPath(problemsRoot, p string) (schema.LedgerClass, bool) {
	res := ledgerPathResFor(problemsRoot)
	s := path.Clean(strings.ReplaceAll(p, "\\", "/"))
	switch {
	case res.problem.MatchString(s):
		return schema.LedgerClasses["problem-ledger"], true
	case res.delivery.MatchString(s):
		return schema.LedgerClasses["delivery-ledger"], true
	}
	return schema.LedgerClass{}, false
}

func def(verb, file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

// Parse reads one Argus ledger by the class its path names under the configured
// problems root, emitting named parse defects (never a panic) when the shape
// cannot be delimited. It returns (nil, nil) for a path that is not an in-scope
// Argus ledger — a non-ledger file is simply not this parse's concern. Defects
// carry the caller's verb so a shared parse names the right arm. The problems
// root is resolved from config by the caller (default docs/problems).
func Parse(problemsRoot, verb, filePath string, b []byte) (*Ledger, []schema.Defect) {
	class, ok := ClassForPath(problemsRoot, filePath)
	if !ok {
		return nil, nil
	}
	return ParseAs(verb, filePath, class, b)
}

// ParseAs reads one ledger against an explicit class — used where the caller
// already resolved the class (and by the parse's own tests).
func ParseAs(verb, filePath string, class schema.LedgerClass, b []byte) (*Ledger, []schema.Defect) {
	normalized := schema.Normalize(b)
	lines := strings.Split(strings.TrimSuffix(string(normalized), "\n"), "\n")
	prose := make([]bool, len(lines))
	for i := range ProseLines(normalized) {
		prose[i] = true
	}

	// Version pin: reject a ledger whose declared template version is not the
	// one this closed parse is pinned to, rather than guessing its shape.
	declared := ""
	for i, l := range lines {
		if !prose[i] {
			continue
		}
		if m := tmplVersionRe.FindStringSubmatch(l); m != nil {
			declared = m[1]
			break
		}
	}
	if !strings.Contains(declared, class.TemplateVersion) {
		got := declared
		if got == "" {
			got = "(absent)"
		}
		return nil, []schema.Defect{def(verb, filePath, "ledger-version",
			"declared template version "+got+" is not the pinned "+class.TemplateVersion,
			"instantiate from .pyxis/templates/"+class.Name+"-template.md; the region-delimiter registry parses only the pinned version")}
	}

	// Regions, delimited by `## ` heading. Each region body runs from the
	// line after its heading to the next `## ` heading or file end.
	type span struct{ start, end int }
	regions := map[string]span{}
	var order []struct {
		name  string
		start int
	}
	for i, l := range lines {
		if !prose[i] {
			continue
		}
		if m := schema.LedgerRegionRe.FindStringSubmatch(l); m != nil {
			order = append(order, struct {
				name  string
				start int
			}{strings.TrimSpace(m[1]), i})
		}
	}
	for k, h := range order {
		bodyStart := h.start + 1
		bodyEnd := len(lines)
		if k+1 < len(order) {
			bodyEnd = order[k+1].start
		}
		// First occurrence wins; a duplicate `## ` heading is left to the
		// structure arm — this parse keys on the region the registry names.
		if _, seen := regions[h.name]; !seen {
			regions[h.name] = span{bodyStart, bodyEnd}
		}
	}

	mkRegion := func(name string, s span) *Region {
		return &Region{Name: name, Start: s.start, End: s.end, Bytes: joinBody(lines, s.start, s.end)}
	}

	var defects []schema.Defect

	// The entry-bearing region must be delimitable, else there is nothing to
	// key entries against — a named parse defect, never a silent empty map.
	entrySpan, ok := regions[class.EntryRegion]
	if !ok {
		defects = append(defects, def(verb, filePath, "region",
			"the "+class.EntryRegion+" region could not be delimited (no \"## "+class.EntryRegion+"\" heading)",
			"a "+class.Name+" must carry its \"## "+class.EntryRegion+"\" section"))
	}

	// The delivery ledger's derived-state region must also be delimitable —
	// the cross-document arm classifies a citation by region, so an
	// undelimitable derived block is a named parse defect (UB-3), never a
	// pass that would let a derived-block citation resolve to nothing.
	var derived *Region
	if class.DerivedRegion != "" {
		if s, ok := regions[class.DerivedRegion]; ok {
			derived = mkRegion(class.DerivedRegion, s)
		} else {
			defects = append(defects, def(verb, filePath, "region",
				"the "+class.DerivedRegion+" region could not be delimited (no \"## "+class.DerivedRegion+"\" heading)",
				"a "+class.Name+" must carry its \"## "+class.DerivedRegion+"\" section"))
		}
	}

	if len(defects) > 0 {
		return nil, defects
	}

	entryRegion := mkRegion(class.EntryRegion, entrySpan)

	// Entry blocks within the entry region: each runs from its
	// `### <anchor>` heading to the next entry heading or the region end.
	entries := map[string][]byte{}
	rawEntries := map[string][]byte{}
	rawStarts := rawLineStarts(b)
	var entryOrder []string
	var anchorLines []int
	var anchorIDs []string
	for i := entrySpan.start; i < entrySpan.end; i++ {
		if !prose[i] {
			continue
		}
		m := schema.LedgerAnchorRe.FindStringSubmatch(lines[i])
		if m == nil || m[1] != class.AnchorFamily {
			continue // a foreign-family heading is not one of this class's entries
		}
		anchorLines = append(anchorLines, i)
		anchorIDs = append(anchorIDs, m[1]+"-"+m[2])
	}
	for k, start := range anchorLines {
		id := anchorIDs[k]
		blockEnd := entrySpan.end
		if k+1 < len(anchorLines) {
			blockEnd = anchorLines[k+1]
		}
		if _, dup := entries[id]; dup {
			defects = append(defects, def(verb, filePath, "entry",
				"duplicate entry anchor "+id,
				"ledger entries are dense and unique; never reuse an anchor"))
			continue
		}
		entries[id] = blockBytes(lines, start, blockEnd)
		contentEnd := blockContentEnd(lines, start, blockEnd)
		if start < len(rawStarts) && contentEnd < len(rawStarts) {
			rawEntries[id] = append([]byte(nil), b[rawStarts[start]:rawStarts[contentEnd]]...)
		}
		entryOrder = append(entryOrder, id)
	}

	if len(defects) > 0 {
		return nil, defects
	}

	return &Ledger{
		Class:       class,
		Path:        filePath,
		Entries:     entries,
		RawEntries:  rawEntries,
		Order:       entryOrder,
		EntryRegion: entryRegion,
		Derived:     derived,
	}, nil
}

// joinBody joins a body line range with newlines and one trailing newline.
func joinBody(lines []string, start, end int) []byte {
	if start >= end {
		return []byte{}
	}
	return []byte(strings.Join(lines[start:end], "\n") + "\n")
}

// blockBytes extracts an entry block's bytes: the heading line through the
// line before the next entry, with trailing blank lines and the `---` entry
// separator dropped so a block's identity is its content, not the layout
// around it. One trailing newline.
func blockBytes(lines []string, start, end int) []byte {
	e := blockContentEnd(lines, start, end)
	return []byte(strings.Join(lines[start:e], "\n") + "\n")
}

func blockContentEnd(lines []string, start, end int) int {
	e := end
	for e > start {
		t := strings.TrimSpace(lines[e-1])
		if t == "" || isRule(t) {
			e--
			continue
		}
		break
	}
	return e
}

// rawLineStarts follows the same CRLF/CR/LF line boundaries as Normalize,
// retaining offsets into the original input. The sentinel is the end of input.
func rawLineStarts(content []byte) []int {
	starts := []int{0}
	for i := 0; i < len(content); i++ {
		switch content[i] {
		case '\r':
			if i+1 < len(content) && content[i+1] == '\n' {
				i++
			}
			starts = append(starts, i+1)
		case '\n':
			starts = append(starts, i+1)
		}
	}
	if starts[len(starts)-1] != len(content) {
		starts = append(starts, len(content))
	}
	return starts
}

// isRule reports whether a trimmed line is a markdown horizontal rule.
func isRule(t string) bool {
	if len(t) < 3 {
		return false
	}
	for _, r := range t {
		if r != '-' {
			return false
		}
	}
	return true
}
