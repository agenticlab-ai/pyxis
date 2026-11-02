package sweep

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestPolicyClosedGrammarBoundaryDefects(t *testing.T) {
	cases := map[string]string{
		"missing staleness value": "schema_version: maintenance-policy/v1\nstaleness_bound:\n",
		"waivers scalar":          "schema_version: maintenance-policy/v1\nwaivers: no\n",
		"unexpected indent two":   "schema_version: maintenance-policy/v1\nunknown:\n  packages:\n",
		"list block scalar":       "schema_version: maintenance-policy/v1\nwaivers:\n  packages: no\n",
		"entry without block":     "schema_version: maintenance-policy/v1\n    - package: example/x\n",
		"entry without dash":      "schema_version: maintenance-policy/v1\nwaivers:\n  packages:\n    package: example/x\n",
		"field without entry":     "schema_version: maintenance-policy/v1\n      reason: missing entry\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, defects := Parse([]byte(body)); len(defects) == 0 {
				t.Fatal("Parse accepted a closed-grammar violation")
			}
		})
	}

	root := t.TempDir()
	rel := "policy.yaml"
	if err := os.Mkdir(filepath.Join(root, rel), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, defects := LoadPolicyFile(root, rel); len(defects) == 0 || !strings.Contains(defects[0].Reason, "unreadable") {
		t.Fatalf("unreadable policy defects = %v", defects)
	}
}

type coverageRunner struct {
	lines []string
	err   error
}

func (r coverageRunner) Run(context.Context, string, string, ...string) ([]string, error) {
	return r.lines, r.err
}

func TestStalenessHelpersCoverMalformedAndRenderedDeltas(t *testing.T) {
	state := State{Root: t.TempDir(), Runner: coverageRunner{lines: []string{"not-a-timestamp"}}}
	if _, _, err := newestCommit(context.Background(), state, []string{"docs/prd.md"}); err == nil {
		t.Fatal("newestCommit malformed timestamp: want error")
	}

	state.Runner = coverageRunner{lines: []string{"123"}}
	got, shipped, err := newestCommit(context.Background(), state, []string{"docs/prd.md"})
	if err != nil || !shipped || got.Unix() != 123 {
		t.Fatalf("newestCommit valid = (%v, %t, %v), want unix 123", got, shipped, err)
	}

	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30m0s"},
		{48 * time.Hour, "2d"},
		{49 * time.Hour, "2d1h"},
	} {
		if got := humanizeDelta(tc.d); got != tc.want {
			t.Errorf("humanizeDelta(%s) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestDetectUnwiredReportsModuleReadFault(t *testing.T) {
	result := DetectUnwired(context.Background(), State{
		Root:   t.TempDir(),
		Runner: coverageRunner{err: errors.New("go unavailable")},
	}, Policy{})
	if result.Fault == "" || !strings.Contains(result.Fault, "module path") || len(result.Findings) != 0 {
		t.Fatalf("DetectUnwired fault = %+v", result)
	}
}

func TestCoverage90HumanFaceAndPassBoundaries(t *testing.T) {
	report := NewReport(EvaluatedState{}, []DetectorResult{{
		Name:  DetectorRollupStaleness,
		Fault: "git history unavailable",
	}}, "repository state unavailable")
	face := report.HumanFace()
	for _, want := range []string{"evaluated commit (unknown)", "fault: repository state unavailable", "FAULT — git history unavailable"} {
		if !strings.Contains(face, want) {
			t.Fatalf("HumanFace() = %q, want %q", face, want)
		}
	}

	got := isolate("panicking-detector", func() DetectorResult { panic("boom") })
	if !strings.Contains(got.Fault, "boom") {
		t.Fatalf("isolate panic result = %+v", got)
	}
}

func TestCoverage90AdditionalPolicyDefects(t *testing.T) {
	cases := []string{
		"schema_version: maintenance-policy/v1\nwaivers:\n  packages:\n      - reason: misplaced\n",
		"schema_version: maintenance-policy/v1\nwaivers:\n  packages:\n    - reason: names no package\n",
	}
	for _, body := range cases {
		if _, defects := Parse([]byte(body)); len(defects) == 0 {
			t.Fatalf("Parse accepted malformed policy:\n%s", body)
		}
	}
	if got := policyUnquote(`value # trailing comment`); got != "value" {
		t.Fatalf("policyUnquote(comment) = %q, want value", got)
	}
}

func TestCoverage90StalenessEmptyInputs(t *testing.T) {
	state := State{Root: t.TempDir(), Runner: coverageRunner{}}
	if _, shipped, err := newestCommit(context.Background(), state, nil); err != nil || shipped {
		t.Fatalf("newestCommit(empty) = shipped %t, err %v", shipped, err)
	}
	if got := humanizeDelta(0); got != "0s" {
		t.Fatalf("humanizeDelta(0) = %q, want 0s", got)
	}
	result := DetectRollupStaleness(context.Background(), State{
		Root:   t.TempDir(),
		Runner: coverageRunner{lines: []string{"123"}},
		Config: config.Config{CapabilityPaths: []string{"internal"}},
	}, Empty())
	if result.Fault != "" || len(result.Findings) != 0 {
		t.Fatalf("blank rollups result = %+v", result)
	}
}

func TestCoverage90TouchesHelperFailures(t *testing.T) {
	if !touchesDelivered(t.TempDir(), "") {
		t.Fatal("an empty touch token must be treated as delivered")
	}
	if _, err := touchesReadFace(t.TempDir(), "missing-plan.json"); err == nil {
		t.Fatal("touchesReadFace must report a missing plan face")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "wrong-schema.json"), []byte(`{"schema_version":"wrong"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := touchesReadFace(root, "wrong-schema.json"); err == nil {
		t.Fatal("touchesReadFace must reject a non-plan schema")
	}
}

func TestCoverage90UnwiredHelperBoundaries(t *testing.T) {
	state := State{Root: t.TempDir(), Runner: coverageRunner{}}
	if _, fault := modulePath(context.Background(), state); fault == "" {
		t.Fatal("modulePath must reject empty command output")
	}
	packages, fault := firstPartyPackages(context.Background(), State{
		Root:   t.TempDir(),
		Runner: coverageRunner{lines: []string{"malformed"}},
	}, "example.test/module")
	if fault != "" || len(packages) != 0 {
		t.Fatalf("firstPartyPackages malformed line = %v, %q", packages, fault)
	}
	if got := moduleRelative("example.test/module", "example.test/module"); got != "." {
		t.Fatalf("moduleRelative(root) = %q, want .", got)
	}
	if got := moduleRelative("outside.test/pkg", "example.test/module"); got != "outside.test/pkg" {
		t.Fatalf("moduleRelative(external) = %q", got)
	}
}
