package efficiency_test

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/efficiency"
)

// planFixture mirrors .pyxis/templates/plan-template.md's "## Coverage"
// table: one row per requirement, its Tasks cell listing the covering
// task ids. Three of four rows are covered.
const planFixture = `# Some plan

## Tasks

### T1 — a task

## Coverage

| Requirement | Tasks |
|---|---|
| prd:some-feature/FR-1 | T1, T2 |
| prd:some-feature/FR-2 | T3 |
| prd:some-feature/FR-3 |  |
| prd:some-feature/FR-4 | T4 |

## Deferred

| Requirement | Reason |
|---|---|
| prd:some-feature/FR-5 | out of scope |
`

func TestCoverageRatio(t *testing.T) {
	got := efficiency.CoverageRatio([]byte(planFixture))
	want := 3.0 / 4.0
	if got != want {
		t.Errorf("CoverageRatio = %v, want %v", got, want)
	}
}

func TestCoverageRatioFullyCovered(t *testing.T) {
	const doc = `## Coverage

| Requirement | Tasks |
|---|---|
| prd:x/FR-1 | T1 |
| prd:x/FR-2 | T1, T2 |
`
	if got := efficiency.CoverageRatio([]byte(doc)); got != 1.0 {
		t.Errorf("CoverageRatio = %v, want 1.0", got)
	}
}

func TestCoverageRatioNoSection(t *testing.T) {
	if got := efficiency.CoverageRatio([]byte("# a doc with no coverage table\n")); got != 0 {
		t.Errorf("CoverageRatio = %v, want 0", got)
	}
}

func TestCoverageRatioEmptyTable(t *testing.T) {
	const doc = `## Coverage

| Requirement | Tasks |
|---|---|

## Deferred
`
	if got := efficiency.CoverageRatio([]byte(doc)); got != 0 {
		t.Errorf("CoverageRatio = %v, want 0", got)
	}
}

func TestCoverageRatioNilDoc(t *testing.T) {
	if got := efficiency.CoverageRatio(nil); got != 0 {
		t.Errorf("CoverageRatio(nil) = %v, want 0", got)
	}
}
