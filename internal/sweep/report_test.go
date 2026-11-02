package sweep_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/sweep"
)

// A finding refuses construction without all four contracted fields
// (acceptance 4): detector, subject, evidence, remedy.
func TestNewFindingRequiresFourFields(t *testing.T) {
	cases := []struct {
		name                                string
		detector, subject, evidence, remedy string
	}{
		{"no detector", "", "s", "e", "r"},
		{"no subject", "unwired-packages", "", "e", "r"},
		{"no evidence", "unwired-packages", "s", "", "r"},
		{"no remedy", "unwired-packages", "s", "e", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := sweep.NewFinding(c.detector, c.subject, c.evidence, c.remedy); err == nil {
				t.Fatalf("finding constructed despite a missing field: %s", c.name)
			}
		})
	}
	if _, err := sweep.NewFinding("unwired-packages", "s", "e", "r"); err != nil {
		t.Fatalf("complete finding refused: %v", err)
	}
}

// A report carrying a four-field-incomplete finding refuses to marshal
// (acceptance 4, marshalling clause): the guard holds at the machine face too.
func TestMarshalRefusesIncompleteFinding(t *testing.T) {
	r := sweep.NewReport(
		sweep.EvaluatedState{Commit: "abc", Dirty: false},
		[]sweep.DetectorResult{{
			Name: sweep.DetectorUnwiredPackages,
			// hand-built finding bypassing NewFinding — missing remedy.
			Findings: []sweep.Finding{{
				SchemaVersion: sweep.FindingSchemaVersion,
				Detector:      sweep.DetectorUnwiredPackages,
				Subject:       "internal/orphan",
				Evidence:      "reachable from no binary",
			}},
		}},
	)
	if _, err := r.Marshal(); err == nil {
		t.Fatal("report marshalled with a four-field-incomplete finding")
	}
}

// build a representative report with findings across detectors and waivers.
func sampleReport(t *testing.T) sweep.Report {
	t.Helper()
	f1, err := sweep.NewFinding(sweep.DetectorUnwiredPackages, "internal/zeta", "reachable from no binary", "wire or waive")
	if err != nil {
		t.Fatal(err)
	}
	f2, err := sweep.NewFinding(sweep.DetectorUnwiredPackages, "internal/alpha", "reachable from no binary", "wire or waive")
	if err != nil {
		t.Fatal(err)
	}
	f3, err := sweep.NewFinding(sweep.DetectorRollupStaleness, "docs/prd.md", "stale by 20d (bound 14d)", "refresh or retune")
	if err != nil {
		t.Fatal(err)
	}
	// detectors deliberately out of canonical order; findings out of subject order.
	return sweep.NewReport(
		sweep.EvaluatedState{Commit: "deadbeef", Dirty: true},
		[]sweep.DetectorResult{
			{Name: sweep.DetectorRollupStaleness, Findings: []sweep.Finding{f3}},
			{Name: sweep.DetectorUnwiredPackages, Findings: []sweep.Finding{f1, f2}, WaiversApplied: []sweep.AppliedWaiver{
				{Kind: sweep.WaiverKindPackage, Subject: "internal/waived", Reason: "test-harness library"},
			}},
			{Name: sweep.DetectorTouchesUnderdelivery},
		},
	)
}

// Marshalling the same report value twice yields byte-identical output
// (acceptance 3, determinism clause).
func TestMarshalIsByteDeterministic(t *testing.T) {
	r := sampleReport(t)
	a, err := r.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("marshal not deterministic:\n%s\n%s", a, b)
	}
}

// Marshal emits detectors in the fixed order and subjects lexicographically,
// regardless of construction order (DINV-2).
func TestMarshalCanonicalOrdering(t *testing.T) {
	out := string(mustMarshal(t, sampleReport(t)))

	// detector order: unwired-packages, then touches-underdelivery, then rollup-staleness.
	iUnwired := strings.Index(out, `"name":"unwired-packages"`)
	iTouches := strings.Index(out, `"name":"touches-underdelivery"`)
	iRollup := strings.Index(out, `"name":"rollup-staleness"`)
	if !(iUnwired >= 0 && iUnwired < iTouches && iTouches < iRollup) {
		t.Fatalf("detectors not in fixed order: unwired=%d touches=%d rollup=%d\n%s", iUnwired, iTouches, iRollup, out)
	}

	// subjects within unwired-packages are lexicographic: alpha before zeta.
	iAlpha := strings.Index(out, "internal/alpha")
	iZeta := strings.Index(out, "internal/zeta")
	if !(iAlpha >= 0 && iAlpha < iZeta) {
		t.Fatalf("subjects not lexicographic: alpha=%d zeta=%d", iAlpha, iZeta)
	}
}

// No machine-face field carries wall-clock content (acceptance 3): the marshalled
// report contains nothing that reads as a timestamp.
func TestMachineFaceHasNoWallClock(t *testing.T) {
	out := string(mustMarshal(t, sampleReport(t)))
	// RFC3339-ish timestamps, epoch-looking large integers, or clock-y keys.
	patterns := []string{
		`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`, // ISO datetime
		`"(timestamp|time|now|generated_at|created_at|ran_at|date)"`,
	}
	for _, p := range patterns {
		if regexp.MustCompile(p).MatchString(out) {
			t.Fatalf("machine face carries wall-clock content matching %q:\n%s", p, out)
		}
	}
}

// The outcome field is derived by the pinned ranking findings > fault > clean.
func TestDeriveOutcomeRanking(t *testing.T) {
	clean := sweep.NewReport(sweep.EvaluatedState{Commit: "c"}, []sweep.DetectorResult{
		{Name: sweep.DetectorUnwiredPackages}, {Name: sweep.DetectorRollupStaleness},
	})
	if clean.Outcome != sweep.OutcomeClean {
		t.Fatalf("clean outcome wrong: %q", clean.Outcome)
	}
	fault := sweep.NewReport(sweep.EvaluatedState{Commit: "c"}, []sweep.DetectorResult{
		{Name: sweep.DetectorRollupStaleness, Fault: "toolchain absent"},
	})
	if fault.Outcome != sweep.OutcomeFault {
		t.Fatalf("fault outcome wrong: %q", fault.Outcome)
	}
	// findings win even when a fault coexists.
	f, _ := sweep.NewFinding(sweep.DetectorUnwiredPackages, "internal/x", "e", "r")
	findings := sweep.NewReport(sweep.EvaluatedState{Commit: "c"}, []sweep.DetectorResult{
		{Name: sweep.DetectorUnwiredPackages, Findings: []sweep.Finding{f}},
		{Name: sweep.DetectorRollupStaleness, Fault: "toolchain absent"},
	})
	if findings.Outcome != sweep.OutcomeFindings {
		t.Fatalf("findings-win outcome wrong: %q", findings.Outcome)
	}
}

// An empty detector marshals as [] not null — the stable JSON shape (DINV-2).
func TestEmptySlicesMarshalAsArrays(t *testing.T) {
	out := string(mustMarshal(t, sweep.NewReport(sweep.EvaluatedState{Commit: "c"},
		[]sweep.DetectorResult{{Name: sweep.DetectorUnwiredPackages}})))
	if strings.Contains(out, `"findings":null`) || strings.Contains(out, `"waivers_applied":null`) {
		t.Fatalf("empty slices marshalled as null:\n%s", out)
	}
	if !strings.Contains(out, `"findings":[]`) || !strings.Contains(out, `"waivers_applied":[]`) {
		t.Fatalf("empty slices not [] :\n%s", out)
	}
}

func mustMarshal(t *testing.T, r sweep.Report) []byte {
	t.Helper()
	b, err := r.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return b
}
