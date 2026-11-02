package project

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

func TestNativeIntakeProjectionOmitsFeatureDesign(t *testing.T) {
	root := writeRepo(t, nativeSource())
	body, err := os.ReadFile("../plan/testdata/header/intake-001-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(body), "plan-template/v1", "plan-template/v2")
	source = strings.ReplaceAll(source, "harbormaster/fs-spec:v1", "harbormaster/fs-spec:v2")
	source = strings.Replace(source, "| **Routing** | rung=2 |", "| **Requirements** | "+requirementstest.Input+" |", 1)
	rel := "docs/plans/maintenance/001-plan.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	f := newFake()
	rep, defects := Project(root, config.Defaults(), rel, f, dialect.Harbormaster(), opts())
	if len(defects) != 0 || rep == nil || len(rep.Items) != 1 {
		t.Fatalf("native intake report=%+v defects=%v", rep, defects)
	}
	comments := strings.Join(f.comments[rep.Items[0].Issue], "\n")
	if !strings.Contains(comments, "<!-- fs-spec:v2 ") || strings.Contains(comments, `"design"`) {
		t.Fatalf("native intake spec=%s", comments)
	}
}

type changingNativeTracker struct {
	*fake
	reads    map[int]int
	changeAt int
	change   string
}

func (f *changingNativeTracker) Comments(id int) ([]string, error) {
	f.reads[id]++
	comments, err := f.fake.Comments(id)
	if f.reads[id] != f.changeAt {
		return comments, err
	}
	switch f.change {
	case "unavailable":
		return nil, errors.New("tracker unavailable")
	case "missing":
		return nil, nil
	case "changed":
		return []string{strings.ReplaceAll(strings.Join(comments, "\n"), "complex", "invalid")}, nil
	}
	return comments, err
}

func TestNativeProjectionRefusesLostOrChangedReadback(t *testing.T) {
	root := writeRepo(t, nativeSource())
	for _, at := range []int{2, 3} {
		for _, change := range []string{"unavailable", "missing", "changed"} {
			f := &changingNativeTracker{fake: newFake(), reads: map[int]int{}, changeAt: at, change: change}
			if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), Options{Log: io.Discard}); len(defects) == 0 {
				t.Fatalf("accepted %s readback %d", change, at)
			}
			for _, issue := range f.issues {
				if issue.HasLabel("local-agent") {
					t.Fatalf("routed after %s readback %d", change, at)
				}
			}
		}
	}
}

type corruptNativeTracker struct {
	*fake
	corrupt bool
}

func (f *corruptNativeTracker) Comment(id int, body string) error {
	if f.corrupt {
		body = strings.ReplaceAll(body, "complex", "invalid")
	}
	return f.fake.Comment(id, body)
}

func TestNativeProjectionReadbackAndRetry(t *testing.T) {
	root := writeRepo(t, nativeSource())
	for _, corrupt := range []bool{false, true} {
		f := &corruptNativeTracker{fake: newFake(), corrupt: corrupt}
		opts := Options{Log: io.Discard}
		rep, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts)
		if corrupt {
			if len(defects) == 0 {
				t.Fatal("accepted a corrupted write")
			}
			for _, issue := range f.issues {
				if issue.HasLabel("local-agent") {
					t.Fatal("corrupted task became dispatchable")
				}
			}
			continue
		}
		if len(defects) > 0 {
			t.Fatal(defects)
		}
		for _, item := range rep.Items {
			s, err := taskspec.Parse(f.comments[item.Issue])
			if err != nil || s.Requirements == nil {
				t.Fatalf("task not native: %+v %v", s, err)
			}
		}
		before := len(f.issues)
		if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts); len(defects) > 0 {
			t.Fatal(defects)
		}
		if len(f.issues) != before {
			t.Fatal("retry duplicated tasks")
		}
		for _, item := range rep.Items {
			if len(f.comments[item.Issue]) != 1 {
				t.Fatal("retry duplicated spec")
			}
		}
		item := rep.Items[0]
		f.comments[item.Issue][0] = strings.Replace(f.comments[item.Issue][0], "complex", "invalid", 1)
		if _, defects := Project(root, config.Defaults(), planRel, f, dialect.Harbormaster(), opts); len(defects) == 0 {
			t.Fatal("accepted stale spec on rerun")
		}
	}
}

func nativeSource() string { return fixturePlan("internal/widget/export") }

func TestHistoricalProjectionRefusedBeforeTrackerWrites(t *testing.T) {
	source := strings.ReplaceAll(fixturePlan("internal/widget/export"), "plan-template/v2", "plan-template/v1")
	source = strings.ReplaceAll(source, "harbormaster/fs-spec:v2", "harbormaster/fs-spec:v1")
	source = strings.ReplaceAll(source, "| **Requirements** | "+requirementstest.Input+" |", "| **Routing** | rung=3 |")
	root := writeRepo(t, source)
	tr := newFake()
	report, defects := Project(root, config.Defaults(), planRel, tr, dialect.Harbormaster(), Options{Log: io.Discard})
	if len(defects) == 0 || report == nil || report.Routing != "withheld" {
		t.Fatalf("historical projection was not refused: %+v %v", report, defects)
	}
	if len(tr.issues) != 0 {
		t.Fatalf("created tracker items before refusal: %+v", tr.issues)
	}
}
