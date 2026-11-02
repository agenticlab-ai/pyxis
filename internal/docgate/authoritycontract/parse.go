package authoritycontract

import (
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// tmplVersionRe matches the `| **Template version** | <value> |` header row.
var tmplVersionRe = regexp.MustCompile(`^\|\s*\*\*Template version\*\*\s*\|\s*(.+?)\s*\|`)

// italicNoteRe matches a trailing `*(…)*` italic parenthetical on a bundle
// segment (e.g. tier-4's `*(activates only after …)*`) — display prose, never
// a token, so it is stripped before the token is read.
var italicNoteRe = regexp.MustCompile(`\*\(.*\)\*\s*$`)

// region is a body-line range [start, end) into the normalized lines.
type region struct {
	lines []string
}

// delimitRegions maps each `## <title>` region to its body lines (after the
// heading, up to the next `## ` heading or EOF). First occurrence wins, matching
// the ledger parse's region delimitation.
func delimitRegions(lines []string) map[string]region {
	type head struct {
		name  string
		start int
	}
	var heads []head
	for i, l := range lines {
		if m := schema.LedgerRegionRe.FindStringSubmatch(l); m != nil {
			heads = append(heads, head{strings.TrimSpace(m[1]), i})
		}
	}
	out := map[string]region{}
	for k, h := range heads {
		bodyStart := h.start + 1
		bodyEnd := len(lines)
		if k+1 < len(heads) {
			bodyEnd = heads[k+1].start
		}
		if _, seen := out[h.name]; !seen {
			out[h.name] = region{lines: lines[bodyStart:bodyEnd]}
		}
	}
	return out
}

// dataRow is one table data row's cells plus its 1-based source-relative line.
type dataRow struct {
	line  int // 1-based within the whole file is not known here; see dataRowsWithLine
	cells []string
}

// dataRowsWithLine returns a region's table DATA rows (header and separator
// rows dropped), each with its 1-based line index WITHIN the region body — good
// enough to name a row; callers that need only the cells use dataRows.
func dataRowsWithLine(r region) []dataRow {
	var out []dataRow
	seenHeader := false
	for i, l := range r.lines {
		cells := splitRow(l)
		if cells == nil {
			continue // not a table row (guidance prose, blank line)
		}
		if isSeparatorRow(cells) {
			continue
		}
		if !seenHeader {
			seenHeader = true // the first pipe row is the column header
			continue
		}
		out = append(out, dataRow{line: i + 1, cells: cells})
	}
	return out
}

func dataRows(r region) [][]string {
	rows := dataRowsWithLine(r)
	out := make([][]string, 0, len(rows))
	for _, dr := range rows {
		out = append(out, dr.cells)
	}
	return out
}

// splitRow splits a markdown table row into trimmed cells, or nil for a line
// that is not a table row (no leading pipe).
func splitRow(line string) []string {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "|") {
		return nil
	}
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

var sepCellRe = regexp.MustCompile(`^:?-+:?$`)

// isSeparatorRow reports whether every cell is a markdown table separator
// (`---`, `:--`, …) — the `|---|---|` row under a table header.
func isSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if !sepCellRe.MatchString(c) {
			return false
		}
	}
	return true
}

// cell returns the i-th cell or "" if the row is shorter — so a change-history
// row missing a trailing field yields "" for the DINV-3 evaluation to name.
func cell(cells []string, i int) string {
	if i < 0 || i >= len(cells) {
		return ""
	}
	return cells[i]
}

// bundleTokens extracts the action-class tokens a ladder bundle cell grants. A
// cell beginning with the empty-bundle marker (∅) grants none — its prose is
// never token-scanned. Otherwise each `·`-separated segment contributes one
// token: its leading `+` (the "adds to the prior tier" notation) and any
// trailing `*(…)*` italic note are stripped, and the token is the first
// whitespace field of what remains (§3.2; the template's bundle grammar).
func bundleTokens(bundle string) []string {
	b := strings.TrimSpace(bundle)
	if b == "" || strings.HasPrefix(b, schema.EmptyBundleMarker) {
		return nil
	}
	var tokens []string
	for _, seg := range strings.Split(b, "·") {
		seg = strings.TrimSpace(seg)
		seg = strings.TrimPrefix(seg, "+")
		seg = italicNoteRe.ReplaceAllString(seg, "")
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		if tok := stripToken(fields[0]); tok != "" {
			tokens = append(tokens, tok)
		}
	}
	return tokens
}

// stripToken removes backtick and angle-bracket wrapping and surrounding
// whitespace from a cell token (tier names are written “ `tier-2` “; a
// template placeholder is `<…>`, which strips to a non-tier and fails
// membership as it should).
func stripToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	s = strings.TrimSpace(s)
	return s
}
