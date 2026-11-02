package reader_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/efficiency"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/reader"
)

func TestSourceFor_KnownPhaseKinds(t *testing.T) {
	root := "/repo"
	tests := []struct {
		name    string
		phaseID string
		want    efficiency.Source
	}{
		{
			name:    "feature-brainstorm",
			phaseID: "feature-brainstorm",
			want: efficiency.Source{
				RepoRoot:     root,
				AnchorPaths:  []string{"docs/brainstorms/foo.md"},
				LatencyPaths: []string{"docs/brainstorms/foo.md"},
			},
		},
		{
			name:    "tech-design-brainstorm",
			phaseID: "tech-design-brainstorm",
			want: efficiency.Source{
				RepoRoot:     root,
				AnchorPaths:  []string{"docs/brainstorms/foo-tech-design.md"},
				LatencyPaths: []string{"docs/brainstorms/foo-tech-design.md"},
			},
		},
		{
			name:    "prd-new",
			phaseID: "prd-new",
			want: efficiency.Source{
				RepoRoot:     root,
				AmendmentDir: "docs/prds/foo",
				LatencyPaths: []string{"docs/prds/foo"},
			},
		},
		{
			name:    "prd-amend",
			phaseID: "prd-amend",
			want: efficiency.Source{
				RepoRoot:     root,
				AmendmentDir: "docs/prds/foo",
				LatencyPaths: []string{"docs/prds/foo"},
			},
		},
		{
			name:    "tech-design-new",
			phaseID: "tech-design-new",
			want: efficiency.Source{
				RepoRoot:     root,
				LatencyPaths: []string{"docs/tech-design/foo.md"},
			},
		},
		{
			name:    "unrecognized phase kind reads as no signal, never a guess",
			phaseID: "impl-review",
			want:    efficiency.Source{RepoRoot: root},
		},
		{
			name:    "tech-design-review is deliberately unresolved (design-slug ambiguity)",
			phaseID: "tech-design-review",
			want:    efficiency.Source{RepoRoot: root},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reader.SourceFor(root, "foo", tt.phaseID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SourceFor(%q, %q) = %+v, want %+v", "foo", tt.phaseID, got, tt.want)
			}
		})
	}
}

func TestSourceFor_PrdReview_GlobsDigitAnchoredReviews(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs/reviews/foo-000.md"), "review 0")
	writeFile(t, filepath.Join(root, "docs/reviews/foo-001.md"), "review 1")
	// A different, prefix-colliding feature's review must never be pulled in.
	writeFile(t, filepath.Join(root, "docs/reviews/foo-boundary-enforcement-000.md"), "someone else's review")

	got := reader.SourceFor(root, "foo", "prd-review")
	want := []string{"docs/reviews/foo-000.md", "docs/reviews/foo-001.md"}
	if !reflect.DeepEqual(got.ReviewPaths, want) {
		t.Errorf("ReviewPaths = %v, want %v", got.ReviewPaths, want)
	}
	if !reflect.DeepEqual(got.LatencyPaths, want) {
		t.Errorf("LatencyPaths = %v, want %v", got.LatencyPaths, want)
	}
}

func TestSourceFor_PrdReview_NoMatches_IsBareNotError(t *testing.T) {
	root := t.TempDir()
	got := reader.SourceFor(root, "foo", "prd-review")
	if len(got.ReviewPaths) != 0 {
		t.Errorf("ReviewPaths = %v, want none", got.ReviewPaths)
	}
}

func TestSourceFor_PlanNew_PicksHighestNumberedPlan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs/plans/foo/001-plan.md"), "plan 1")
	writeFile(t, filepath.Join(root, "docs/plans/foo/002-plan.md"), "plan 2")

	got := reader.SourceFor(root, "foo", "plan-new")
	if got.PlanPath != "docs/plans/foo/002-plan.md" {
		t.Errorf("PlanPath = %q, want the highest-numbered plan", got.PlanPath)
	}
	want := []string{"docs/plans/foo/001-plan.md", "docs/plans/foo/002-plan.md"}
	if !reflect.DeepEqual(got.LatencyPaths, want) {
		t.Errorf("LatencyPaths = %v, want %v", got.LatencyPaths, want)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
