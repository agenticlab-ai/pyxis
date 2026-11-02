package contractgate

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// updateGolden regenerates the pinned golden human face instead of comparing
// against it (`go test -run TestReportHumanFaceGolden -update`). The standard
// Go golden-file idiom — the golden is checked in and byte-compared by default.
var updateGolden = flag.Bool("update", false, "regenerate the golden report faces")

// emitInto runs the verb against a fixture and emits any report into a fresh temp
// sink, snapshotting the FIXTURE tree before/after to prove the gate wrote
// NOTHING under the read-only contract inputs (DINV-1) — the report lands only in
// the separate sink. It returns the verdict, the sink dir, and the written paths.
func emitInto(t *testing.T, fixture, feature string) (Verdict, string, []string) {
	t.Helper()
	root := filepath.Join("testdata", fixture)
	sink := t.TempDir()

	before := snapshot(t, root)
	v := RunWithConfig(root, feature, config.Defaults())
	paths, err := EmitReport(sink, v)
	if err != nil {
		t.Fatalf("EmitReport: %v", err)
	}
	after := snapshot(t, root)
	// The read-only audit: the fixture (the contract inputs) is byte-unchanged —
	// the gate mutated nothing outside the sink (DINV-1, plan T5 read-only audit).
	assertUnchanged(t, before, after)
	return v, sink, paths
}

// sinkFiles lists the file names materialized in a sink dir, so a test can assert
// exactly how many artifacts a run produced (zero on clean; two on a bounce).
func sinkFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read sink %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestReportEmittedOnBounce table-drives every defect-class fixture: a bounce
// emits EXACTLY ONE report (its two faces — .json + .md), both faces validate /
// parse, and each enumerates every failed check with check-id + offender +
// unblock (plan T5 acceptance #1; design §3.8, FR-8).
func TestReportEmittedOnBounce(t *testing.T) {
	tests := []struct {
		fixture   string
		feature   string
		wantCheck string // the check the fixture reds
		wantClass Class
	}{
		{"struct-nofit", "demo", "structural-completeness", ClassIncomplete},
		{"struct-nodod", "demo", "structural-completeness", ClassIncomplete},
		{"compil-orphan-serves", "demo", "compilability", ClassMalformed},
		{"compil-buildon-cycle", "demo", "compilability", ClassMalformed},
		{"stale-serve", "demo", "currency", ClassStale},
		{"uncovered-must", "demo", "inverse-coverage", ClassIncomplete},
		{"dangling-citation", "demo", "link", ClassUnresolved},
		{"no-designs", "demo", "inverse-coverage", ClassIncomplete},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			v, sink, paths := emitInto(t, tc.fixture, tc.feature)
			if v.Clean() {
				t.Fatalf("%s did not bounce (class %q) — cannot exercise emission", tc.fixture, v.Class)
			}

			// Exactly one report = two faces, no more, no less.
			if len(paths) != 2 {
				t.Fatalf("emitted %d paths, want 2 (both faces): %v", len(paths), paths)
			}
			names := sinkFiles(t, sink)
			if len(names) != 2 {
				t.Fatalf("sink holds %d files, want exactly 2 (one report, two faces): %v", len(names), names)
			}
			var jsonPath, mdPath string
			for _, p := range paths {
				switch filepath.Ext(p) {
				case ".json":
					jsonPath = p
				case ".md":
					mdPath = p
				}
			}
			if jsonPath == "" || mdPath == "" {
				t.Fatalf("expected a .json and a .md face; got %v", paths)
			}

			// --- Machine face: parses back into the closed schema and enumerates
			// the failed checks with check-id + offender + unblock. ---
			raw, err := os.ReadFile(jsonPath)
			if err != nil {
				t.Fatalf("read machine face: %v", err)
			}
			var got DefectReport
			dec := json.NewDecoder(strings.NewReader(string(raw)))
			dec.DisallowUnknownFields() // closed schema — a stray key is a defect
			if err := dec.Decode(&got); err != nil {
				t.Fatalf("machine face does not parse against the closed schema: %v", err)
			}
			if got.SchemaVersion != ReportSchemaVersion {
				t.Errorf("report schema_version = %q, want %q", got.SchemaVersion, ReportSchemaVersion)
			}
			if got.Feature != tc.feature {
				t.Errorf("report feature = %q, want %q", got.Feature, tc.feature)
			}
			if got.Verdict != tc.wantClass {
				t.Errorf("report verdict = %q, want %q", got.Verdict, tc.wantClass)
			}
			if got.GeneratedBy == "" || got.ToolVersion == "" {
				t.Errorf("report header incomplete: by=%q tool=%q", got.GeneratedBy, got.ToolVersion)
			}

			// EVERY failed check in the verdict appears in the report; every defect
			// carries check-id + offender + unblock (the FR-8 triad).
			assertReportEnumeratesBounce(t, v, got)

			// The offending check is present and named.
			found := false
			for _, c := range got.Checks {
				if c.Check == tc.wantCheck {
					found = true
					if c.Class != tc.wantClass {
						t.Errorf("report check %q class = %q, want %q", c.Check, c.Class, tc.wantClass)
					}
				}
			}
			if !found {
				t.Errorf("report does not enumerate the failed check %q; checks: %+v", tc.wantCheck, got.Checks)
			}

			// --- Human face: renders the same bounce and names every failed check
			// + its unblocks. ---
			md, err := os.ReadFile(mdPath)
			if err != nil {
				t.Fatalf("read human face: %v", err)
			}
			human := string(md)
			if !strings.Contains(human, tc.feature) {
				t.Errorf("human face does not name the feature %q", tc.feature)
			}
			for _, c := range got.Checks {
				if !strings.Contains(human, c.Check) {
					t.Errorf("human face omits failed check %q", c.Check)
				}
				for _, d := range c.Defects {
					if !strings.Contains(human, d.Unblock) {
						t.Errorf("human face omits the unblock for a %q defect: %q", c.Check, d.Unblock)
					}
				}
			}

			// The two faces derive from the same DefectReport — the human face
			// must equal Markdown() of the parsed machine face (never diverge).
			if human != got.Markdown() {
				t.Errorf("human face diverges from Markdown() of the machine face — the two faces are not derived from one record")
			}
		})
	}
}

// assertReportEnumeratesBounce proves the report bundles EVERY failed check's
// defects with the FR-8 triad — check id, offender, unblock — and enumerates
// exactly the verdict's failed checks (no passed check leaks in, none omitted).
func assertReportEnumeratesBounce(t *testing.T, v Verdict, r DefectReport) {
	t.Helper()

	// Build the set of failed checks (with defects) from the verdict.
	wantChecks := map[string]int{} // check -> defect count
	for _, res := range v.CheckResults {
		if !res.Passed && len(res.Defects) > 0 {
			wantChecks[res.Check] = len(res.Defects)
		}
	}
	gotChecks := map[string]int{}
	for _, c := range r.Checks {
		gotChecks[c.Check] = len(c.Defects)
		// No passed check may appear.
		if _, want := wantChecks[c.Check]; !want {
			t.Errorf("report enumerates check %q that did not fail with defects", c.Check)
		}
		// Every defect carries the full triad.
		for _, d := range c.Defects {
			if d.Check == "" {
				t.Errorf("report defect carries no check id: %+v", d)
			}
			if d.Offender == "" {
				t.Errorf("report defect carries no offender: %+v", d)
			}
			if d.Reason == "" {
				t.Errorf("report defect carries no reason: %+v", d)
			}
			if d.Unblock == "" {
				t.Errorf("report defect carries no unblock: %+v", d)
			}
			if d.Check != c.Check {
				t.Errorf("defect check %q disagrees with its enclosing check %q", d.Check, c.Check)
			}
		}
	}
	// Every failed check is enumerated, with its full defect count.
	for check, n := range wantChecks {
		if gotChecks[check] != n {
			t.Errorf("report enumerates %d defects for %q, want %d", gotChecks[check], check, n)
		}
	}
}

// TestCleanEmitsNothing proves DINV-6 with a filesystem audit: a clean verdict
// emits ZERO report and materializes ZERO artifact — both faces are absent and
// the sink stays empty (plan T5 acceptance #2). EmitReport returns no paths, and
// the read-only audit inside emitInto proves the fixture itself is untouched.
func TestCleanEmitsNothing(t *testing.T) {
	v, sink, paths := emitInto(t, "wellformed", "demo")
	if !v.Clean() {
		t.Fatalf("well-formed fixture is not clean: %q — %s", v.Class, v.Reason)
	}
	if len(paths) != 0 {
		t.Errorf("clean verdict emitted %d paths, want 0 (DINV-6): %v", len(paths), paths)
	}
	// The sink is empty — nothing materialized at all (not even an empty file).
	if names := sinkFiles(t, sink); len(names) != 0 {
		t.Errorf("clean verdict materialized %d files in the sink, want 0 (DINV-6): %v", len(names), names)
	}
	// The report builder itself refuses a clean verdict (the second guard).
	if _, ok := ReportForVerdict(v); ok {
		t.Errorf("ReportForVerdict returned a report for a clean verdict — DINV-6 forbids it")
	}
}

// TestExactlyOneReportPerBounce proves a bounce emits EXACTLY one report even
// when MULTIPLE checks fail: the report is one record bundling every failed
// check, not one-per-check (design §3.8: "one structured, dual-face report").
// The no-designs fixture with a missing-DoD twist is contrived here by driving a
// fixture that reds more than one check; we assert one record, all checks in it.
func TestExactlyOneReportPerBounce(t *testing.T) {
	// no-designs reds inverse-coverage (every Must unserved); it may also red
	// structural/link depending on the fixture. Whatever the failure set, the
	// emission is a single report (two faces) bundling all of them.
	v, sink, paths := emitInto(t, "no-designs", "demo")
	if v.Clean() {
		t.Fatalf("no-designs did not bounce")
	}
	if len(paths) != 2 {
		t.Fatalf("emitted %d paths, want exactly 2 (one report, two faces): %v", len(paths), paths)
	}
	if names := sinkFiles(t, sink); len(names) != 2 {
		t.Fatalf("sink holds %d files, want exactly 2 (one report): %v", len(names), names)
	}

	// The one report enumerates every failed check the verdict carries.
	r, ok := ReportForVerdict(v)
	if !ok {
		t.Fatalf("ReportForVerdict returned no report for a bounce")
	}
	var failed []string
	for _, res := range v.CheckResults {
		if !res.Passed && len(res.Defects) > 0 {
			failed = append(failed, res.Check)
		}
	}
	if len(r.Checks) != len(failed) {
		t.Errorf("report enumerates %d checks, verdict has %d failed: report=%v verdict-failed=%v",
			len(r.Checks), len(failed), reportCheckNames(r), failed)
	}
}

func reportCheckNames(r DefectReport) []string {
	out := make([]string, len(r.Checks))
	for i, c := range r.Checks {
		out[i] = c.Check
	}
	return out
}

// TestReportDeterministic proves the report content is byte-reproducible across
// runs over identical inputs (DINV-5): no clock, no map iteration in the content.
func TestReportDeterministic(t *testing.T) {
	root := filepath.Join("testdata", "struct-nofit")
	a := RunWithConfig(root, "demo", config.Defaults())
	b := RunWithConfig(root, "demo", config.Defaults())

	ra, oka := ReportForVerdict(a)
	rb, okb := ReportForVerdict(b)
	if !oka || !okb {
		t.Fatalf("both runs must bounce; got %v / %v", oka, okb)
	}
	ja, err := ra.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	jb, err := rb.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if string(ja) != string(jb) {
		t.Errorf("machine face not byte-stable across runs")
	}
	if ra.Markdown() != rb.Markdown() {
		t.Errorf("human face not byte-stable across runs")
	}
}

// TestReportGoldenFaces pins BOTH faces of one representative bounce
// (struct-nofit — a Must FR lacking a fit criterion) byte-for-byte against
// checked-in golden files (plan T5 DoD: "a golden test pins the human face").
// Both faces are pinned so a change to either the schema or the rendering is a
// visible, reviewed diff. Regenerate with `-update` after an intended change.
func TestReportGoldenFaces(t *testing.T) {
	root := filepath.Join("testdata", "struct-nofit")
	v := RunWithConfig(root, "demo", config.Defaults())
	r, ok := ReportForVerdict(v)
	if !ok {
		t.Fatalf("struct-nofit did not bounce — cannot pin its report")
	}
	jsonBytes, err := r.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	human := []byte(r.Markdown())

	goldenMD := filepath.Join("testdata", "golden", "struct-nofit-defect-report.md")
	goldenJSON := filepath.Join("testdata", "golden", "struct-nofit-defect-report.json")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenMD), 0o755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		if err := os.WriteFile(goldenMD, human, 0o644); err != nil {
			t.Fatalf("write golden md: %v", err)
		}
		if err := os.WriteFile(goldenJSON, jsonBytes, 0o644); err != nil {
			t.Fatalf("write golden json: %v", err)
		}
		t.Logf("regenerated golden faces")
		return
	}

	wantMD, err := os.ReadFile(goldenMD)
	if err != nil {
		t.Fatalf("read golden md (run with -update to create): %v", err)
	}
	if string(human) != string(wantMD) {
		t.Errorf("human face differs from golden %s:\n--- got ---\n%s\n--- want ---\n%s", goldenMD, human, wantMD)
	}

	wantJSON, err := os.ReadFile(goldenJSON)
	if err != nil {
		t.Fatalf("read golden json (run with -update to create): %v", err)
	}
	if string(jsonBytes) != string(wantJSON) {
		t.Errorf("machine face differs from golden %s:\n--- got ---\n%s\n--- want ---\n%s", goldenJSON, jsonBytes, wantJSON)
	}
}

// TestReportPathTraversalSafe proves a feature slug can never escape the sink:
// the report base name neutralizes path separators, so a slug like "../evil"
// still writes inside the sink dir (a defect report is scratch, never a
// path-traversal vector).
func TestReportPathTraversalSafe(t *testing.T) {
	got := reportBaseName("../../etc/passwd")
	if strings.ContainsAny(got, `/\`) {
		t.Errorf("report base name %q still carries a path separator", got)
	}
	if strings.Contains(got, "..") {
		t.Errorf("report base name %q still carries a parent-dir token", got)
	}
}
