package ledger_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
)

const projectionFixture = `# projection parser fixture

| Field | Value |
|---|---|
| **Template version** | prd-template/v2 |

## 0. Read to rule

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-90 | When reviewing, the human shall inspect this routing-only row. | routing | Must | low | read it | Inspection | G1 |

## 5. Requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked, the system shall parse this contract row. | contract | Must | med | row is present | Test | G1 |
`

func TestParseProjectionExcludesHumanOnlySectionZero(t *testing.T) {
	base, defects := ledger.ParseBase("000-base.md", []byte(projectionFixture))
	if len(defects) != 0 {
		t.Fatalf("legacy base parse has defects: %v", defects)
	}
	for _, id := range []string{"FR-90", "FR-1"} {
		if base.Units[id] == nil {
			t.Fatalf("ParseBase behavior changed: inferred unit %s is absent", id)
		}
	}

	projection, defects := ledger.ParseProjection("effective.md", []byte(projectionFixture))
	if len(defects) != 0 {
		t.Fatalf("projection parse has defects: %v", defects)
	}
	if got, want := strings.Join(projection.UnitOrder, ","), "FR-1"; got != want {
		t.Fatalf("projection units = %q, want %q", got, want)
	}
	if projection.Units["FR-1"] == nil {
		t.Fatal("projection omitted the canonical contract table row")
	}
}

func TestParseProjectionRejectsWrongKindInStrictTable(t *testing.T) {
	body := strings.Replace(projectionFixture,
		"| FR-1 | When invoked, the system shall parse this contract row. | contract | Must | med | row is present | Test | G1 |",
		"| NR-1 | When invoked, the system shall reject this mismatched row. | contract | Must | med | row is present | Test | G1 |", 1)
	projection, defects := ledger.ParseProjection("effective.md", []byte(body))
	if projection == nil {
		t.Fatal("strict projection unexpectedly returned nil")
	}
	if projection.Units["NR-1"] != nil {
		t.Fatal("wrong-kind row was accepted as a projection unit")
	}
	for _, d := range defects {
		if strings.Contains(d.Reason, "row NR-1 sits in a FR table") {
			return
		}
	}
	t.Fatalf("missing wrong-kind defect: %v", defects)
}

func TestParseProjectionRecognizesFrozenLegacyHeaders(t *testing.T) {
	tests := []struct {
		name string
		id   string
		cols []string
	}{
		{"job shorthand", "J1", []string{"#", "When…", "I want to…", "So I can…", "Today's workaround", "Evidence"}},
		{"risk shorthand", "R1", []string{"#", "Risk / assumption", "Likelihood", "Impact", "Mitigation or test", "Owner"}},
		{"unwanted behavior shorthand", "UB-1", []string{"ID", "If…", "Then the system shall…", "User sees", "Priority", "Risk", "Fit criterion", "Verify", "Traces to"}},
		{"executor-specific unwanted behavior", "UB-1", []string{"ID", "If…", "Then the executor shall…", "User sees", "Priority", "Risk", "Fit criterion", "Verify", "Traces to"}},
		{"planner-specific unwanted behavior", "UB-1", []string{"ID", "If…", "Then the planner shall…", "User sees", "Priority", "Risk", "Fit criterion", "Verify", "Traces to"}},
		{"gate-specific unwanted behavior", "UB-1", []string{"ID", "If… (trigger)", "Then the gate shall…", "Host sees", "Priority", "Risk", "Fit criterion", "Verify", "Traces to"}},
		{"harness-specific unwanted behavior", "UB-1", []string{"ID", "If… (trigger)", "Then the harness shall…", "Host sees", "Priority", "Risk", "Fit criterion", "Verify", "Traces to"}},
		{"consumer-specific unwanted behavior", "UB-1", []string{"ID", "If… (trigger)", "Then the system shall…", "Consumer sees", "Priority", "Risk", "Fit criterion", "Verify", "Traces to"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := make([]string, len(tc.cols))
			for i := range row {
				row[i] = "value"
			}
			row[0] = tc.id
			separator := make([]string, len(tc.cols))
			for i := range separator {
				separator[i] = "---"
			}
			body := "# frozen legacy table\n\n" +
				"| Field | Value |\n|---|---|\n| **Template version** | prd-template/v1 |\n\n" +
				"## 5. Requirements\n\n| " + strings.Join(tc.cols, " | ") + " |\n" +
				"|" + strings.Join(separator, "|") + "|\n" +
				"| " + strings.Join(row, " | ") + " |\n"
			base, baseDefects := ledger.ParseBase("000-base.md", []byte(body))
			if len(baseDefects) != 0 || base.Units[tc.id] == nil {
				t.Fatalf("ParseBase inference changed: unit=%v defects=%v", base.Units[tc.id], baseDefects)
			}
			projection, defects := ledger.ParseProjection("effective.md", []byte(body))
			if len(defects) != 0 {
				t.Fatalf("parse frozen header: %v", defects)
			}
			unit := projection.Units[tc.id]
			if unit == nil {
				t.Fatalf("legacy row %s was omitted", tc.id)
			}
			if got := projection.Tables[unit.Table].Cols; !slices.Equal(got, tc.cols) {
				t.Fatalf("authored columns changed: got %v, want %v", got, tc.cols)
			}
		})
	}
}

func TestParseProjectionExcludesSectionZeroAndLexicalOnlyRows(t *testing.T) {
	body := `# routing references

| Field | Value |
|---|---|
| **Template version** | prd-template/v2 |

## 0. Read to rule

### Review routes

| Read first | Binding decision | Refused alternative | Why / contract |
|---|---|---|---|
| FR-1 | one-review minimum | zero reviews | contract reference only |
| UB-20 | fail closed | infer success | contract reference only |

## 1. Binding body

| Flow | Happy path | States | Failure coverage |
|---|---|---|---|
| F1 | lexical-only flow | ready | none |

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When invoked, the system shall retain the binding row. | body governs | Must | high | one FR-1 unit | Test | G1 |
`
	projection, defects := ledger.ParseProjection("effective.md", []byte(body))
	if len(defects) != 0 {
		t.Fatalf("routing references produced defects: %v", defects)
	}
	if got, want := strings.Join(projection.UnitOrder, ","), "FR-1"; got != want {
		t.Fatalf("projection units = %q, want %q; §0 references or lexical-only F rows leaked", got, want)
	}
}
