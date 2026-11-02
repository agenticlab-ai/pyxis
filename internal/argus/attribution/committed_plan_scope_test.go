package attribution

import (
	"path"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const committedPlanProblem = `# Widget — problem ledger

| Field | Value |
|---|---|
| **Slug** | widget |
| **Template version** | problem-ledger/v1 |

## Idea

Build the widget.

## Authority entries

### A-1 — fixture ruling

- **Date**: 2026-10-05
- **Ruling**: Build the widget.
`

func TestGitProblemLedgerAloneCannotGrantDownstreamPlanScope(t *testing.T) {
	if _, defects := argusledger.ParseAs("test", "docs/problems/widget/problem.md", schema.LedgerClasses["problem-ledger"], []byte(committedPlanProblem)); len(defects) != 0 {
		t.Fatalf("invalid problem fixture: %v", defects)
	}
	derivedAt := strings.Index(committedWidget, "## Derived state")
	journalAt := strings.Index(committedWidget, "## Journal")
	reordered := committedWidget[:derivedAt] + committedWidget[journalAt:] + "\n" + committedWidget[derivedAt:journalAt]
	for _, tc := range []struct{ name, delivery string }{
		{"absent", ""}, {"malformed", "not a delivery ledger\n"},
		{"missing slug", strings.Replace(committedWidget, "| **Slug** | widget |\n", "", 1)},
		{"mismatched slug", strings.Replace(committedWidget, "| **Slug** | widget |", "| **Slug** | other |", 1)},
		{"incomplete derived state", strings.Replace(committedWidget, "| **Open items** | — |\n", "", 1)},
		{"invalid derived status", strings.Replace(committedWidget, "| **Status** | open |", "| **Status** | approved |", 1)},
		{"reordered regions", reordered},
		{"unframed tables", strings.ReplaceAll(strings.ReplaceAll(committedWidget, "| Field | Value |\n", ""), "|---|---|\n", "")},
		{"empty journal", committedWidget[:strings.Index(committedWidget, "### E-1")]},
		{"untyped journal", strings.Replace(committedWidget, "### E-1 — stage-entered — prd-new", "### E-1 nonsense", 1)},
		{"invalid journal date", strings.Replace(committedWidget, "- **Date**: 2026-10-06", "- **Date**: 2026-02-30", 1)},
		{"external placeholder", strings.Replace(strings.Replace(committedWidget, "stage-entered", "item-materialized", 1), "- **External effect**: docs/prds/widget/000-base.md (+ effective.md, effective.source.json; sha256:abc)", "- **External effect**: —", 1)},
		{"principal placeholder", strings.Replace(committedWidget, "- **Principal**: Argus", "- **Principal**: —", 1)},
		{"annotated principal placeholder", strings.Replace(committedWidget, "- **Principal**: Argus", "- **Principal**: — (unknown)", 1)},
		{"annotated external placeholder", strings.Replace(strings.Replace(committedWidget, "stage-entered", "item-materialized", 1), "- **External effect**: docs/prds/widget/000-base.md (+ effective.md, effective.source.json; sha256:abc)", "- **External effect**: none (not recorded)", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepo(t)
			r.write("docs/problems/widget/problem.md", committedPlanProblem)
			if tc.delivery != "" {
				r.write("docs/problems/widget/delivery.md", tc.delivery)
			}
			base := r.commit("docs: seed problem without valid committed delivery")
			// A candidate repair does not change the trusted base's scope input.
			r.write("docs/problems/widget/delivery.md", committedWidget)
			targets := []string{"docs/plans/widget/001-plan.md", "docs/plans/widget/001-plan.json"}
			if tc.delivery != "" {
				// A missing delivery keeps the existing first-proposal grammar,
				// which already includes own-problem captures. An invalid existing
				// delivery gains no new scope, even after a candidate repair.
				targets = append(targets, "memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.json", "memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.md")
			}
			for _, target := range targets {
				r.write(target, "fixture plan\n")
			}
			if tc.name == "invalid derived status" || tc.name == "reordered regions" {
				r.write("docs/prds/widget/000-base.md", "historically owned document\n")
			}
			r.commitArgus("docs: propose plans before committed delivery", "widget")
			got := r.run(base)
			for _, target := range targets {
				if _, refused := got[target]; !refused {
					t.Errorf("%s committed delivery admitted %s: %v", tc.name, target, got)
				}
			}
			if tc.name == "invalid derived status" || tc.name == "reordered regions" {
				if _, refused := got["docs/prds/widget/000-base.md"]; refused {
					t.Errorf("historical path lost scope: %v", got)
				}
			}
		})
	}
}

func TestGitCommittedProblemAdmitsOnlyItsBoundedPlanAndCaptureArtifacts(t *testing.T) {
	for _, plansRoot := range []string{"docs/plans", "."} {
		t.Run(plansRoot, func(t *testing.T) {
			testGitCommittedProblemPlanRoot(t, plansRoot, committedWidget)
		})
	}
}

func TestGitInternalJournalAdmitsBoundedPlanAndCaptureArtifacts(t *testing.T) {
	var lines []string
	for line := range strings.SplitSeq(committedWidget, "\n") {
		if !strings.HasPrefix(line, "- **External effect**:") {
			lines = append(lines, line)
		}
	}
	for _, kind := range []string{"stage-entered", "proposal-disposed"} {
		t.Run(kind, func(t *testing.T) {
			testGitCommittedProblemPlanRoot(t, "docs/plans", strings.Replace(strings.Join(lines, "\n"), "stage-entered", kind, 1))
		})
	}
}

func testGitCommittedProblemPlanRoot(t *testing.T, plansRoot, delivery string) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Plans = plansRoot
	r := newRepo(t)
	r.write("docs/problems/widget/delivery.md", delivery)
	base := r.commit("docs: seed committed problem")
	allowed := []string{
		path.Join(plansRoot, "widget/001-plan.md"), path.Join(plansRoot, "widget/001-plan.json"),
		"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.json",
		"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.md",
	}
	denied := []string{
		path.Join(plansRoot, "other/001-plan.md"), path.Join(plansRoot, "widget-other/001-plan.md"),
		path.Join(plansRoot, "widget/code.py"), path.Join(plansRoot, "widget/nested/001-plan.md"),
		"memory/problem/other/Document-01M4767N12EEK6YZWRM29DF0RP.json", "memory/source/pyxis-config/Document-01M4767N12EEK6YZWRM29DF0RP.json",
		"aec_memory/store.py", ".pyxis/config.yaml",
		"memory/problem/widget/notes.md", "memory/problem/widget/code.py",
		"memory/problem/widget/nested/Document-01M4767N12EEK6YZWRM29DF0RP.json",
		"memory/problem/widget/Decision-01M4767N12EEK6YZWRM29DF0RP.json",
		"memory/problem/widget/Document-short.json",
	}
	for _, target := range append(append([]string{}, allowed...), denied...) {
		r.write(target, "fixture artifact\n")
	}
	r.commitArgus("docs: add next-stage artifacts", "widget")
	got := map[string]string{}
	for _, defect := range Evaluate(r.dir, base, cfg) {
		got[defect.File] = defect.Op
	}
	for _, target := range allowed {
		if _, refused := got[target]; refused {
			t.Errorf("bounded stage artifact %s refused: %v", target, got)
		}
	}
	for _, target := range denied {
		if got[target] != "out-of-scope" {
			t.Errorf("unowned target %s not refused: %v", target, got)
		}
	}
}
