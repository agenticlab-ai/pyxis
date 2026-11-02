// Package calibration is the closed-parse loader for the impl-review-calibration/v1
// table (.pyxis/skills/impl-review/calibration.md) and a report.CalLookup over it
// (docs/tech-design/impl-review-v1.md §3.5, FR-12, DINV-3).
//
// The calibration table is the REVIEWED DATA DIAL: every finding cites a row, and
// the finding's severity is whatever that row carries — not a lens's free choice.
// This package parses the one markdown table into a Table, which resolves a cal_row
// to its severity (report.CalLookup), so report.Validate rejects any finding whose
// severity contradicts its cited row (DINV-3). The table is the single source of
// truth: a data-only edit to calibration.md flips a finding's validity with NO Go
// edit and NO lens-instruction edit (design A-7), because this package only PARSES
// the file — it hard-codes no row.
//
// The parse is CLOSED (AP-6, per the repo Go rules): the table must have exactly
// the four expected column headers in order, every data row must carry all four
// cells, the severity cell must be one of report.Severity{Blocking,Advisory}, and
// a duplicate row id is an error. A malformed table is a data defect surfaced here,
// never a lenient read that silently drops a row.
//
// Stdlib only.
package calibration

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// Version is the calibration schema id this loader parses — it must match the
// `impl-review-calibration/v1` the payloads' provenance records (report.Provenance
// CalVersion). A table declaring another version is a different dial and is not
// parsed leniently.
const Version = "impl-review-calibration/v1"

// The four required column headers, in order. The parse is closed: the table's
// header row must be exactly these, so a renamed or reordered column is an error
// rather than a silently-misread cell.
var wantColumns = [4]string{"Row", "Lens", "Default severity", "Escalation condition"}

// Row is one calibration row — the reviewed dial for a class of finding. ID is the
// citation a finding carries in its cal_row; Severity is the severity a finding
// citing this row MUST have (DINV-3); Lens and Escalation are the reader's context
// (which lens owns the row and when the round's verdict escalates).
type Row struct {
	ID         string
	Lens       string
	Severity   string
	Escalation string
}

// Table is the parsed calibration table — a closed set of rows keyed by id. It
// implements report.CalLookup, so it is the real dial report.Validate consults for
// DINV-3. Rows preserves file order for the reader; byID is the lookup index.
type Table struct {
	Rows []Row
	byID map[string]Row
}

// Severity implements report.CalLookup: it returns the severity the cited row maps
// to and whether the row is known. A finding citing an id absent from the table
// yields (_, false), which report.Validate reports as an unknown-cal_row defect
// (DINV-3) — the citation must resolve.
func (t *Table) Severity(calRow string) (string, bool) {
	r, ok := t.byID[calRow]
	if !ok {
		return "", false
	}
	return r.Severity, true
}

// Row returns the full row for an id (its lens and escalation condition, not just
// the severity) — for callers that render or reason over the dial, not just
// validate against it.
func (t *Table) Row(id string) (Row, bool) {
	r, ok := t.byID[id]
	return r, ok
}

// Compile-time assurance the Table is a report.CalLookup — the whole reason this
// package exists is to be the real dial Validate consults.
var _ report.CalLookup = (*Table)(nil)

// LoadFile reads and parses the calibration table at path (the canonical file is
// .pyxis/skills/impl-review/calibration.md). It is the single-source loader: the
// table lives in that one markdown file, and this reads it rather than duplicating
// any row in Go, so a data-only edit to the file flips validity with no code edit.
func LoadFile(path string) (*Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("calibration: open %s: %w", path, err)
	}
	defer f.Close()
	t, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("calibration: %s: %w", path, err)
	}
	return t, nil
}

// Parse reads the one markdown pipe-table out of r and parses it closed. It scans
// for the header row (the row whose cells are exactly wantColumns), skips the
// alignment separator, and reads every subsequent pipe-row as a data row until the
// table ends (a blank line or a non-table line). A document with no such table,
// with the wrong columns, with a malformed row, with an invalid severity, or with
// a duplicate row id is an error (AP-6 closed parse).
func Parse(r io.Reader) (*Table, error) {
	sc := bufio.NewScanner(r)
	// Rows can be long (the escalation condition is a sentence); grow the buffer so
	// a wide row is never silently truncated into a malformed parse.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		sawHeader    bool
		sawSeparator bool
		sawRow       bool
		rows         []Row
		byID         = map[string]Row{}
		lineNumber   int
	)

	for sc.Scan() {
		lineNumber++
		line := strings.TrimSpace(sc.Text())

		if !isTableRow(line) {
			if sawHeader && sawRow {
				// The table has ended (a blank or prose line after data rows). One
				// table is the whole dial; stop at its end rather than hunting for a
				// second one.
				break
			}
			// Still scanning for the table (or between prose and the table).
			continue
		}

		cells := splitRow(line)

		if !sawHeader {
			if !columnsMatch(cells) {
				// A pipe-row before the calibration header that is not it — skip it
				// (a doc may carry other tables above). The header is recognized by
				// its exact columns; only then does the table begin.
				continue
			}
			sawHeader = true
			continue
		}

		// The alignment separator (|---|---|…) is ONLY the row immediately after the
		// header — checked once, then never again. Checking it on every body row
		// would silently swallow a malformed all-dash data row instead of rejecting
		// it (AP-6 closed parse); so once the separator is consumed, every further
		// table row is a data row parsed by parseRow.
		if !sawSeparator {
			if !isSeparatorRow(cells) {
				return nil, fmt.Errorf("expected the alignment separator row after the header, got %q (line %d)", line, lineNumber)
			}
			sawSeparator = true
			continue
		}

		row, err := parseRow(cells, lineNumber)
		if err != nil {
			return nil, err
		}
		if _, dup := byID[row.ID]; dup {
			return nil, fmt.Errorf("duplicate row id %q (line %d)", row.ID, lineNumber)
		}
		byID[row.ID] = row
		rows = append(rows, row)
		sawRow = true
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	if !sawHeader {
		return nil, fmt.Errorf("no calibration table found (expected a pipe-table with columns %v)", wantColumns)
	}
	if !sawRow {
		return nil, fmt.Errorf("calibration table has no data rows")
	}

	return &Table{Rows: rows, byID: byID}, nil
}

// parseRow validates and builds one data row's cells. Every cell must be present
// and non-empty, and the severity cell must be a known severity — a closed parse
// rejects a row missing a cell or carrying an out-of-set severity rather than
// admitting a half-specified dial entry.
func parseRow(cells []string, lineNumber int) (Row, error) {
	if len(cells) != len(wantColumns) {
		return Row{}, fmt.Errorf("row at line %d has %d cells, want %d", lineNumber, len(cells), len(wantColumns))
	}
	id := strings.TrimSpace(cells[0])
	lens := strings.TrimSpace(cells[1])
	sev := strings.TrimSpace(cells[2])
	esc := strings.TrimSpace(cells[3])

	if id == "" {
		return Row{}, fmt.Errorf("row at line %d has an empty Row id", lineNumber)
	}
	if lens == "" {
		return Row{}, fmt.Errorf("row %q (line %d) has an empty Lens", id, lineNumber)
	}
	if esc == "" {
		return Row{}, fmt.Errorf("row %q (line %d) has an empty Escalation condition", id, lineNumber)
	}
	if !validSeverity(sev) {
		return Row{}, fmt.Errorf("row %q (line %d) has invalid severity %q (want %q or %q)",
			id, lineNumber, sev, report.SeverityBlocking, report.SeverityAdvisory)
	}

	return Row{ID: id, Lens: lens, Severity: sev, Escalation: esc}, nil
}

// validSeverity reports whether s is one of the two closed severities.
func validSeverity(s string) bool {
	return s == report.SeverityBlocking || s == report.SeverityAdvisory
}

// isTableRow reports whether line is a markdown pipe-table row (starts with `|`).
// The calibration table is written with leading/trailing pipes, so this is the
// cheap discriminator between a table row and prose.
func isTableRow(line string) bool {
	return strings.HasPrefix(line, "|")
}

// splitRow splits a `| a | b | c |` row into its trimmed cells, splitting on
// UNESCAPED pipes only (a markdown-escaped `\|` inside a cell — legitimate in an
// escalation sentence that writes `A \| B` — stays part of the cell and is
// unescaped to a literal `|`), and dropping the empty leading/trailing fields the
// outer pipes produce. Splitting naively on every `|` would turn one escaped pipe
// into a spurious extra cell and brick the whole dial (parseRow's cell-count check).
func splitRow(line string) []string {
	parts := splitUnescapedPipes(line)
	// A leading `|` yields an empty first field and a trailing `|` an empty last;
	// drop exactly those, never an interior empty cell (which is a malformed row the
	// caller rejects).
	if len(parts) > 0 && strings.TrimSpace(parts[0]) == "" {
		parts = parts[1:]
	}
	if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// splitUnescapedPipes splits line on `|` characters that are not escaped by a
// preceding backslash, unescaping a `\|` to a literal `|` in the emitted field. A
// lone trailing backslash is kept verbatim. It is the small state machine that lets
// a cell legitimately contain a pipe.
func splitUnescapedPipes(line string) []string {
	var (
		fields []string
		cur    strings.Builder
	)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\\' && i+1 < len(line) && line[i+1] == '|' {
			// An escaped pipe: emit a literal `|` into the current field, consume both.
			cur.WriteByte('|')
			i++
			continue
		}
		if c == '|' {
			fields = append(fields, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	fields = append(fields, cur.String())
	return fields
}

// columnsMatch reports whether cells are exactly the wanted headers in order — the
// closed-column check that makes a renamed/reordered column an error.
func columnsMatch(cells []string) bool {
	if len(cells) != len(wantColumns) {
		return false
	}
	for i := range wantColumns {
		if cells[i] != wantColumns[i] {
			return false
		}
	}
	return true
}

// isSeparatorRow reports whether cells are the alignment separator that follows a
// markdown header (each cell is only dashes and optional colons, e.g. `---`,
// `:--:`). An EMPTY cells slice is NOT a separator (a bare `|` row splits to zero
// cells) — returning true there would silently swallow a malformed one-pipe row via
// the vacuous for-loop; the explicit length guard makes such a row a parse error
// instead.
func isSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if c == "" {
			return false
		}
		for _, r := range c {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}
