package efficiency_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/efficiency"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

func TestComputeCombinesAllSignals(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docs/brainstorms/foo.md"), brainstormFixture)
	writeFile(t, filepath.Join(dir, "docs/prds/foo/000-base.md"), "base")
	writeFile(t, filepath.Join(dir, "docs/prds/foo/001-amend.md"), "amendment 1")
	writeFile(t, filepath.Join(dir, "docs/prds/foo/002-amend.md"), "amendment 2")
	writeFile(t, filepath.Join(dir, "docs/reviews/foo-000.md"), reviewFixture)
	writeFile(t, filepath.Join(dir, "docs/plans/foo/001-plan.md"), planFixture)

	src := efficiency.Source{
		RepoRoot:     dir,
		AnchorPaths:  []string{"docs/brainstorms/foo.md"},
		AmendmentDir: "docs/prds/foo",
		ReviewPaths:  []string{"docs/reviews/foo-000.md"},
		PlanPath:     "docs/plans/foo/001-plan.md",
	}
	got, err := efficiency.Compute(src)
	if err != nil {
		t.Fatal(err)
	}
	want := record.Efficiency{
		Questions:      3,
		Fog:            2,
		Rework:         3, // 1 superseded D-n (in the anchor doc) + 2 amendments
		Defects:        3,
		CoverageRatio:  3.0 / 4.0,
		LatencySeconds: 0,
	}
	if got != want {
		t.Errorf("Compute = %+v, want %+v", got, want)
	}
}

func TestComputeMissingArtifactsReadAsZero(t *testing.T) {
	dir := t.TempDir()
	src := efficiency.Source{
		RepoRoot:     dir,
		AnchorPaths:  []string{"docs/brainstorms/never-written.md"},
		AmendmentDir: "docs/prds/never-amended",
		ReviewPaths:  []string{"docs/reviews/never-reviewed.md"},
		PlanPath:     "docs/plans/never-planned/001-plan.md",
	}
	got, err := efficiency.Compute(src)
	if err != nil {
		t.Fatalf("a Source naming absent artifacts must not error, got %v", err)
	}
	if got != (record.Efficiency{}) {
		t.Errorf("Compute = %+v, want the zero value (absent artifacts read as no signal)", got)
	}
}

func TestJoin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docs/brainstorms/foo.md"), brainstormFixture)

	records := []record.Record{
		{SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "foo", PhaseID: "feature-brainstorm", Harness: "claude-code"},
		{SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "foo", PhaseID: "prd-new", Harness: "claude-code"},
	}
	sources := map[string]efficiency.Source{
		"feature-brainstorm": {RepoRoot: dir, AnchorPaths: []string{"docs/brainstorms/foo.md"}},
	}

	got, err := efficiency.Join(records, sources)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Efficiency.Questions != 3 {
		t.Errorf("joined record's Questions = %d, want 3", got[0].Efficiency.Questions)
	}
	if got[1].Efficiency != (record.Efficiency{}) {
		t.Errorf("unmatched record should keep the zero-value Efficiency, got %+v", got[1].Efficiency)
	}
	if records[0].Efficiency != (record.Efficiency{}) {
		t.Errorf("Join must not mutate its input slice, but records[0].Efficiency = %+v", records[0].Efficiency)
	}
}

// TestComputeIsHarnessAgnostic is plan pipeline-measurement/001 T9's first
// acceptance criterion: "for a reference run the efficiency signals are
// byte-identical when computed from two different harnesses' artifacts."
// Two independent repos stand in for two harnesses that produced
// byte-identical content on byte-identical git history; Compute must
// yield a byte-identical record.Efficiency from each.
func TestComputeIsHarnessAgnostic(t *testing.T) {
	build := func(t *testing.T) string {
		dir := newGitRepo(t)
		commitAt(t, dir, "docs/brainstorms/foo.md", brainstormFixture, "2026-07-01T00:00:00Z")
		commitAt(t, dir, "docs/prds/foo/000-base.md", "base\n", "2026-07-01T00:00:00Z")
		commitAt(t, dir, "docs/prds/foo/001-amend.md", "amendment\n", "2026-07-02T00:00:00Z")
		commitAt(t, dir, "docs/reviews/foo-000.md", reviewFixture, "2026-07-01T12:00:00Z")
		commitAt(t, dir, "docs/plans/foo/001-plan.md", planFixture, "2026-07-03T00:00:00Z")
		return dir
	}
	harnessA := build(t)
	harnessB := build(t)

	source := func(root string) efficiency.Source {
		return efficiency.Source{
			RepoRoot:     root,
			AnchorPaths:  []string{"docs/brainstorms/foo.md"},
			AmendmentDir: "docs/prds/foo",
			ReviewPaths:  []string{"docs/reviews/foo-000.md"},
			PlanPath:     "docs/plans/foo/001-plan.md",
			LatencyPaths: []string{"docs/brainstorms/foo.md", "docs/prds/foo/001-amend.md"},
		}
	}

	effA, err := efficiency.Compute(source(harnessA))
	if err != nil {
		t.Fatal(err)
	}
	effB, err := efficiency.Compute(source(harnessB))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effA, effB) {
		t.Errorf("efficiency signals differ across harnesses:\nA: %+v\nB: %+v", effA, effB)
	}
	if effA == (record.Efficiency{}) {
		t.Fatal("test fixture produced an all-zero result — it isn't exercising the signals")
	}
}
