package adopt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
)

// TestRenderGolden pins the human face for both lifecycle stages against
// committed goldens (T2 acceptance #2: "the render of a fixture plan is a
// stable, reviewable human face"): the draft fixture exercises the
// not-yet-applied/no-hook/unknown-stack degrade wording, and the applied
// fixture exercises every Class, a multi-line ClassCollideManual detail
// (the fenced-block path), a displaced hook path, a detected stack with
// its gate actions, and an added-path baseline. Regenerate with -update
// after an intended rendering change.
func TestRenderGolden(t *testing.T) {
	cases := []struct {
		name   string
		plan   adopt.Plan
		golden string
	}{
		{"draft", validDraftPlan(), "draft-plan.md"},
		{"applied", validAppliedPlan(), "applied-plan.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.plan.Render()
			goldenPath := filepath.Join("testdata", "golden", c.golden)

			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("regenerated %s", goldenPath)
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if got != string(want) {
				t.Errorf("render differs from golden %s:\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
			}
		})
	}
}

// TestRenderDeterministic proves repeated Render calls over the same value
// are byte-identical.
func TestRenderDeterministic(t *testing.T) {
	p := validAppliedPlan()
	a := p.Render()
	b := p.Render()
	if a != b {
		t.Errorf("Render not byte-stable across calls")
	}
}

// TestRenderDetailFencing proves a multi-line Detail embeds safely
// regardless of its own content: a blank line mid-detail renders as a bare
// blank line (never a trailing-space-padded one), and a detail that itself
// contains a triple-backtick run forces a longer fence so it can never
// prematurely close the wrapping block (CommonMark's fenced-code-block
// rule; the goldens' fixtures carry no backticks, so this branch needs its
// own case).
func TestRenderDetailFencing(t *testing.T) {
	p := adopt.Plan{
		SchemaVersion: adopt.SchemaVersion,
		Status:        adopt.StatusDraft,
		Entries: []adopt.Entry{
			{
				Path:   "AGENTS.md",
				Class:  adopt.ClassCollideManual,
				Detail: "first line\n\nan example:\n```\nnested fence\n```",
			},
		},
	}
	got := p.Render()
	if !strings.Contains(got, "\n  ````\n") {
		t.Errorf("expected a 4-backtick outer fence (one longer than the nested 3-backtick run); got:\n%s", got)
	}
	if !strings.Contains(got, "  first line\n\n  an example:\n") {
		t.Errorf("expected the blank line between detail lines to render bare, not indented; got:\n%s", got)
	}
	if strings.Count(got, "````") != 2 {
		t.Errorf("expected exactly one opening and one closing 4-backtick fence; got:\n%s", got)
	}
}

// TestRenderEmptyEntries proves a plan with no entries — e.g. a repository
// already fully adopted, no drift found — renders an explicit "(none)"
// rather than an empty or missing section (neither golden fixture carries
// a zero-entry plan).
func TestRenderEmptyEntries(t *testing.T) {
	p := adopt.Plan{SchemaVersion: adopt.SchemaVersion, Status: adopt.StatusDraft}
	got := p.Render()
	if !strings.Contains(got, "## Entries\n\n(none)\n\n") {
		t.Errorf("expected an explicit (none) entries section; got:\n%s", got)
	}
}
