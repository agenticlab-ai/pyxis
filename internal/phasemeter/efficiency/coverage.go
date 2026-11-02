package efficiency

import "strings"

// CoverageRatio computes a plan document's Coverage-matrix completion —
// the fraction of its "## Coverage" rows whose Tasks cell names at least
// one task (design §3.10: "coverage from the plan matrix";
// .pyxis/templates/plan-template.md "## Coverage": "every live Must
// FR/UB of the governing PRD maps to >=1 task"). A plan with no Coverage
// section, or an empty table, yields 0 — absent, not fabricated.
func CoverageRatio(doc []byte) float64 {
	rows := tableRows(section(doc, "## Coverage"))
	if len(rows) == 0 {
		return 0
	}
	rows = rows[1:] // the header row ("| Requirement | Tasks |")

	total, covered := 0, 0
	for _, cells := range rows {
		if isSeparatorRow(cells) {
			continue
		}
		total++
		if len(cells) >= 2 && cells[1] != "" {
			covered++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total)
}

// section returns the body of doc's first "## <heading>" section — the
// lines between that heading and the next "## " heading (or EOF) — or nil
// if the heading is absent.
func section(doc []byte, heading string) []byte {
	lines := strings.Split(string(doc), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return nil
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return []byte(strings.Join(lines[start:end], "\n"))
}

// tableRows parses every "| cell | cell |" line in body into its trimmed
// cells, in order, including header and separator rows — the caller
// distinguishes those (they need positional and content context tableRows
// doesn't have).
func tableRows(body []byte) [][]string {
	var rows [][]string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
		cells := strings.Split(line, "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

// isSeparatorRow reports whether every cell is a markdown table separator
// cell (only "-" and ":" once whitespace is stripped) — e.g. "|---|---|".
func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}
