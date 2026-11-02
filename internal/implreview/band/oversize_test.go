package band_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestSizeCheckWithinBudgetProceeds — a diff within the band's honest read
// budget yields no escalation (the review proceeds normally).
func TestSizeCheckWithinBudgetProceeds(t *testing.T) {
	t.Parallel()
	small := bytes.Repeat([]byte("x"), 1024) // well under any band budget
	for _, bnd := range []band.Band{report.BandStandard, report.BandDeep} {
		esc, err := band.SizeCheck(bnd, small)
		if err != nil {
			t.Fatalf("band %q: in-budget diff escalated: %v", bnd, err)
		}
		if esc != nil {
			t.Fatalf("band %q: in-budget diff returned an escalation: %+v", bnd, esc)
		}
	}
	// An empty diff never escalates.
	if esc, err := band.SizeCheck(report.BandStandard, nil); err != nil || esc != nil {
		t.Fatalf("empty diff escalated: esc=%+v err=%v", esc, err)
	}
}

// TestSizeCheckOversizeEscalatesToHuman — UB-7: a diff exceeding the band's
// budget records a size-escalation routing HUMAN, attesting the un-read extent,
// with ErrOversize.
func TestSizeCheckOversizeEscalatesToHuman(t *testing.T) {
	t.Parallel()

	budget := band.BandBudgets[report.BandStandard].MaxDiffBytes
	over := bytes.Repeat([]byte("y"), budget+4096) // over the standard budget

	esc, err := band.SizeCheck(report.BandStandard, over)
	if err == nil {
		t.Fatal("oversize diff did not escalate")
	}
	if !errors.Is(err, band.ErrOversize) {
		t.Fatalf("oversize error is not ErrOversize: %v", err)
	}
	if esc == nil {
		t.Fatal("oversize diff returned no SizeEscalation")
	}
	// Routes HUMAN.
	if !esc.RouteHuman {
		t.Fatal("size-escalation does not route HUMAN")
	}
	// Attests the un-read extent (the bytes beyond the budget).
	if esc.UnreadBytes != len(over)-budget {
		t.Fatalf("un-read extent = %d, want %d", esc.UnreadBytes, len(over)-budget)
	}
	if esc.DiffBytes != len(over) || esc.BudgetBytes != budget {
		t.Fatalf("escalation sizes wrong: %+v", esc)
	}
	// The marker names the un-read extent (harvestable).
	m := esc.Marker()
	if !strings.HasPrefix(m, "size-escalation:") || !strings.Contains(m, "unread=") {
		t.Fatalf("size-escalation marker malformed: %q", m)
	}
}

// TestDeepBandLargerBudgetThanStandard — the deep band reads a larger diff whole
// (the risk justifies the cost): a diff over the standard budget but under the
// deep budget escalates in standard but NOT in deep.
func TestDeepBandLargerBudgetThanStandard(t *testing.T) {
	t.Parallel()
	stdBudget := band.BandBudgets[report.BandStandard].MaxDiffBytes
	deepBudget := band.BandBudgets[report.BandDeep].MaxDiffBytes
	if deepBudget <= stdBudget {
		t.Fatalf("deep budget (%d) is not larger than standard (%d)", deepBudget, stdBudget)
	}
	between := bytes.Repeat([]byte("z"), stdBudget+1024) // > standard, < deep

	if esc, err := band.SizeCheck(report.BandStandard, between); err == nil || esc == nil {
		t.Fatal("a diff over the standard budget should escalate in the standard band")
	}
	if esc, err := band.SizeCheck(report.BandDeep, between); err != nil || esc != nil {
		t.Fatalf("a diff under the deep budget should NOT escalate in the deep band: esc=%+v err=%v", esc, err)
	}
}

// TestUnknownBandFailsClosed — a band with no budget entry never silently reads
// an arbitrarily large diff: any non-empty diff is over budget.
func TestUnknownBandFailsClosed(t *testing.T) {
	t.Parallel()
	esc, err := band.SizeCheck("no-such-band", []byte("anything non-empty"))
	if err == nil || esc == nil {
		t.Fatal("an unknown band should fail closed (any non-empty diff over budget)")
	}
	if !esc.RouteHuman {
		t.Fatal("unknown-band escalation does not route HUMAN")
	}
}

// TestGuardApproveNeverApprovesOverPartialRead — UB-7's core guarantee made
// structural: a size-escalated review NEVER carries an approve verdict. Any
// approve/approve-with-advisories proposed after an escalation is forced to
// request-changes; a non-approving verdict is kept; and with no escalation the
// proposed verdict is unchanged.
func TestGuardApproveNeverApprovesOverPartialRead(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		proposed   string
		escalated  bool
		want       string
		wantOverch bool
	}{
		{"approve over escalation → forced request-changes", report.VerdictApprove, true, report.VerdictRequestChanges, true},
		{"approve-with-advisories over escalation → forced request-changes", report.VerdictApproveWithAdvisories, true, report.VerdictRequestChanges, true},
		{"request-changes over escalation → kept", report.VerdictRequestChanges, true, report.VerdictRequestChanges, false},
		{"escalate over escalation → kept", report.VerdictEscalate, true, report.VerdictEscalate, false},
		{"approve, no escalation → unchanged", report.VerdictApprove, false, report.VerdictApprove, false},
		{"approve-with-advisories, no escalation → unchanged", report.VerdictApproveWithAdvisories, false, report.VerdictApproveWithAdvisories, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, overridden := band.GuardApprove(tc.proposed, tc.escalated)
			if got != tc.want {
				t.Fatalf("verdict = %q, want %q", got, tc.want)
			}
			if overridden != tc.wantOverch {
				t.Fatalf("overridden = %t, want %t", overridden, tc.wantOverch)
			}
			// The invariant: an escalated review is NEVER left at an approve.
			if tc.escalated && (got == report.VerdictApprove || got == report.VerdictApproveWithAdvisories) {
				t.Fatalf("escalated review left at an approve verdict %q (UB-7 violated)", got)
			}
		})
	}
}
