package calibration_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// realTablePath is the one source-of-truth calibration file, relative to this
// package. The Go loader parses THIS file — it hard-codes no row — so a data-only
// edit there flips validity with no code edit.
func realTablePath() string {
	return filepath.Join("..", "..", "..", ".pyxis", "skills", "impl-review", "calibration.md")
}

// TestLoadRealTable — the shipped calibration.md parses closed, carries a row per
// lens, and every row's severity is a valid severity. This pins that the real dial
// the runtime cites is well-formed (FR-12).
func TestLoadRealTable(t *testing.T) {
	t.Parallel()
	tbl, err := calibration.LoadFile(realTablePath())
	if err != nil {
		t.Fatalf("load real calibration table: %v", err)
	}
	if len(tbl.Rows) == 0 {
		t.Fatal("real calibration table has no rows")
	}

	// Every one of the four lenses must own at least one row — a lens with no row
	// could cite none.
	lenses := map[string]int{
		report.LensSpecFidelity:      0,
		report.LensDesignConformance: 0,
		report.LensSecurityAbuse:     0,
		report.LensCorrectnessCritic: 0,
	}
	for _, r := range tbl.Rows {
		if r.Severity != report.SeverityBlocking && r.Severity != report.SeverityAdvisory {
			t.Fatalf("row %q has non-severity %q", r.ID, r.Severity)
		}
		if _, ok := lenses[r.Lens]; !ok {
			t.Fatalf("row %q cites unknown lens %q", r.ID, r.Lens)
		}
		lenses[r.Lens]++
	}
	for lens, n := range lenses {
		if n == 0 {
			t.Errorf("lens %q has no calibration row", lens)
		}
	}

	// The Table is a report.CalLookup, and every declared row resolves.
	var lookup report.CalLookup = tbl
	for _, r := range tbl.Rows {
		sev, ok := lookup.Severity(r.ID)
		if !ok {
			t.Errorf("row %q does not resolve through CalLookup", r.ID)
		}
		if sev != r.Severity {
			t.Errorf("row %q: lookup severity %q != row severity %q", r.ID, sev, r.Severity)
		}
	}
	// A row the table does not carry does not resolve.
	if _, ok := lookup.Severity("CAL-does-not-exist"); ok {
		t.Error("an unknown row resolved through CalLookup")
	}
}

// TestParseClosed — the parse is closed: an unknown column, a malformed/short row,
// an out-of-set severity, or a duplicate row id is an error, never a lenient read.
func TestParseClosed(t *testing.T) {
	t.Parallel()

	const header = "| Row | Lens | Default severity | Escalation condition |\n" +
		"|---|---|---|---|\n"

	cases := []struct {
		name    string
		table   string
		wantErr string // substring the error must contain; "" means expect success
	}{
		{
			name:    "well-formed",
			table:   header + "| CAL-1 | spec-fidelity | blocking | a stated Must is unmet |\n",
			wantErr: "",
		},
		{
			name: "unknown column header",
			table: "| Row | Lens | Severity | Escalation condition |\n" + // "Severity" != "Default severity"
				"|---|---|---|---|\n" +
				"| CAL-1 | spec-fidelity | blocking | x |\n",
			wantErr: "no calibration table found",
		},
		{
			name: "reordered columns",
			table: "| Lens | Row | Default severity | Escalation condition |\n" +
				"|---|---|---|---|\n" +
				"| spec-fidelity | CAL-1 | blocking | x |\n",
			wantErr: "no calibration table found",
		},
		{
			name:    "short row (missing a cell)",
			table:   header + "| CAL-1 | spec-fidelity | blocking |\n",
			wantErr: "cells, want 4",
		},
		{
			name:    "invalid severity",
			table:   header + "| CAL-1 | spec-fidelity | critical | x |\n",
			wantErr: "invalid severity",
		},
		{
			name:    "empty row id",
			table:   header + "|  | spec-fidelity | blocking | x |\n",
			wantErr: "empty Row id",
		},
		{
			name:    "empty escalation condition",
			table:   header + "| CAL-1 | spec-fidelity | blocking |  |\n",
			wantErr: "empty Escalation",
		},
		{
			name: "duplicate row id",
			table: header +
				"| CAL-1 | spec-fidelity | blocking | x |\n" +
				"| CAL-1 | design-conformance | advisory | y |\n",
			wantErr: "duplicate row id",
		},
		{
			name:    "no data rows",
			table:   header,
			wantErr: "no data rows",
		},
		{
			name:    "no table at all",
			table:   "just prose, no pipe table here\n",
			wantErr: "no calibration table found",
		},
		{
			// A stray all-dash row among the data must be REJECTED, not silently
			// swallowed as a separator (the separator is only the row after the
			// header) — AP-6 closed parse.
			name: "mid-table dash row is rejected, not swallowed",
			table: header +
				"| CAL-1 | spec-fidelity | blocking | x |\n" +
				"| --- | --- | --- | --- |\n" +
				"| CAL-2 | spec-fidelity | advisory | y |\n",
			wantErr: "invalid severity",
		},
		{
			// A bare one-pipe row splits to zero cells; it must be a parse error, not
			// a vacuously-true separator that is silently skipped.
			name:    "bare one-pipe row is rejected, not swallowed",
			table:   header + "| CAL-1 | spec-fidelity | blocking | x |\n" + "|\n",
			wantErr: "want 4",
		},
		{
			// The header must be immediately followed by the alignment separator; a
			// data row where the separator should be is a malformed table.
			name: "header not followed by separator",
			table: "| Row | Lens | Default severity | Escalation condition |\n" +
				"| CAL-1 | spec-fidelity | blocking | x |\n",
			wantErr: "expected the alignment separator",
		},
		{
			// A legitimately escaped pipe inside the escalation sentence stays part of
			// the cell (unescaped to a literal `|`) — it must NOT split into a 5th cell.
			name:    "escaped pipe in a cell is not a column split",
			table:   header + "| CAL-1 | spec-fidelity | blocking | use A \\| B alternation |\n",
			wantErr: "", // parses fine
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := calibration.Parse(strings.NewReader(tc.table))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestEscapedPipeUnescapedInCell — an escaped pipe in a cell parses as one cell
// whose value carries a LITERAL pipe (not the backslash), so a legitimate `A \| B`
// escalation sentence round-trips to `A | B`.
func TestEscapedPipeUnescapedInCell(t *testing.T) {
	t.Parallel()
	const table = "| Row | Lens | Default severity | Escalation condition |\n" +
		"|---|---|---|---|\n" +
		"| CAL-1 | spec-fidelity | blocking | use A \\| B alternation |\n"
	tbl, err := calibration.Parse(strings.NewReader(table))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	row, ok := tbl.Row("CAL-1")
	if !ok {
		t.Fatal("CAL-1 not parsed")
	}
	if want := "use A | B alternation"; row.Escalation != want {
		t.Fatalf("escalation = %q, want %q (escaped pipe should unescape to a literal)", row.Escalation, want)
	}
}

// TestDataOnlySeverityFlip is the load-bearing A-7/FR-12/DINV-3 test: a DATA-only
// change to the calibration table flips a finding's validity through report.Validate
// with NO Go edit and NO lens-instruction edit. It parses two tables that differ
// ONLY in a row's severity cell, and shows the SAME finding validates under one and
// fails ErrSeverityMismatch under the other — the whole point of a data dial.
func TestDataOnlySeverityFlip(t *testing.T) {
	t.Parallel()

	// A finding citing CAL-9 with severity "blocking".
	report9Blocking := func() *report.Report {
		return &report.Report{
			SchemaVersion: report.SchemaVersion,
			ProposalRef:   "github:x#1",
			ReviewID:      "rv-1",
			Band:          report.BandStandard,
			Verdict:       report.VerdictRequestChanges,
			Rounds: []report.Round{{
				ReviewedRevision: "deadbeef",
				Findings: []report.Finding{{
					ID:         "FND-1",
					Message:    "a genuine judgment defect",
					RuleID:     "security-abuse/fail-open-on-auth",
					Lens:       report.LensSecurityAbuse,
					Severity:   report.SeverityBlocking,
					CalRow:     "CAL-9",
					Acceptance: "A-1",
				}},
				FindingsCount: 1,
			}},
		}
	}

	const headerAnd = "| Row | Lens | Default severity | Escalation condition |\n" +
		"|---|---|---|---|\n"

	// The ONLY difference between these two data strings is CAL-9's severity cell.
	tableBlocking := headerAnd + "| CAL-9 | security-abuse | blocking | a fail-open on an authority path |\n"
	tableAdvisory := headerAnd + "| CAL-9 | security-abuse | advisory | a fail-open on an authority path |\n"

	calBlocking, err := calibration.Parse(strings.NewReader(tableBlocking))
	if err != nil {
		t.Fatalf("parse blocking table: %v", err)
	}
	calAdvisory, err := calibration.Parse(strings.NewReader(tableAdvisory))
	if err != nil {
		t.Fatalf("parse advisory table: %v", err)
	}

	// Under the blocking dial, a blocking finding citing CAL-9 is VALID.
	if err := report.Validate(report9Blocking(), calBlocking); err != nil {
		t.Fatalf("finding should validate under the blocking dial: %v", err)
	}

	// Flip ONLY the data (CAL-9 → advisory). The SAME blocking finding now
	// contradicts its cited row and is INVALID — no Go edit, no lens edit, only the
	// table changed.
	err = report.Validate(report9Blocking(), calAdvisory)
	if err == nil {
		t.Fatal("finding should FAIL validation under the advisory dial (data flip)")
	}
	if !isSeverityMismatch(err) {
		t.Fatalf("want ErrSeverityMismatch after the data flip, got %v", err)
	}
}

// isSeverityMismatch reports whether err is report.ErrSeverityMismatch, without a
// direct errors.Is import in the assertion site.
func isSeverityMismatch(err error) bool {
	return err != nil && strings.Contains(err.Error(), "contradicts its cited cal_row")
}
