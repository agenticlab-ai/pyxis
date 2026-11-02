package plan

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/contractgate"
)

func TestCoverage90MalformedTraceReferences(t *testing.T) {
	var reasons []string
	bad := func(_ string, reason string, _ string) { reasons = append(reasons, reason) }
	checkRef("prd:malformed", "T1", nil, "", nil, t.TempDir(), config.Config{}, nil, bad)
	checkRef("design:UPPERCASE", "T1", nil, "", nil, t.TempDir(), config.Config{}, nil, bad)
	if len(reasons) != 2 {
		t.Fatalf("malformed trace defects = %v, want two", reasons)
	}
}

func TestCoverage90FoldAndDocumentAccessBoundaries(t *testing.T) {
	result := FoldFile(t.TempDir(), config.Config{}, "docs/plans/demo/001-plan.md")
	if len(result.Defects) == 0 {
		t.Fatal("FoldFile accepted a missing source")
	}
	doc := &Doc{sourceFile: "docs/plans/demo/001-plan.md"}
	if got := doc.SourceFile(); got != doc.sourceFile {
		t.Fatalf("SourceFile() = %q", got)
	}
}

func TestCoverage90ValidateHeaderAndEmptyPlanFailures(t *testing.T) {
	doc := &Doc{
		sourceFile:      "docs/plans/demo/not-a-plan.md",
		Feature:         "demo",
		PRD:             "docs/prds/demo",
		Dialect:         DialectHarbormaster,
		TemplateVersion: PlanTemplateVersion,
	}
	defects := Validate(t.TempDir(), config.Config{}, doc)
	if len(defects) < 3 {
		t.Fatalf("Validate sparse plan defects = %v, want filename, epic, and task failures", defects)
	}
}

func TestCoverage90ParserStructuralFailures(t *testing.T) {
	tests := []string{
		"# first\n# second\n",
		"# first\n## Unknown\n",
		"# first\n## Tasks\norphan content\n",
	}
	for _, source := range tests {
		if doc, defects := Parse("docs/plans/demo/001-plan.md", []byte(source)); doc != nil || len(defects) == 0 {
			t.Fatalf("Parse(%q) = doc %#v, defects %v", source, doc, defects)
		}
	}
}

func TestCoverage90ParserTextHelperBoundaries(t *testing.T) {
	task := &Task{}
	appendText(task, "Goal", "")
	appendText(task, "unknown", "ignored")
	appendText(task, "Goal", "first")
	appendText(task, "Goal", "second")
	if task.Goal != "first second" {
		t.Fatalf("appendText Goal = %q", task.Goal)
	}
}

func TestCoverage90PrecheckSynthesizesMissingReason(t *testing.T) {
	defects := precheckDefects("demo", contractgate.Verdict{
		Trigger: contractgate.TriggerPlanningEntry,
		Class:   contractgate.ClassMalformed,
	})
	if len(defects) == 0 || !strings.Contains(defects[0].Reason, "verdict is malformed") {
		t.Fatalf("precheckDefects = %v", defects)
	}
}

func TestCoverage90CoverageCheckSkipsMissingGoverningSurface(t *testing.T) {
	called := false
	checkCoverage(&Env{Doc: &Doc{}, Gov: governing{}}, func(string, string, string) {
		called = true
	})
	if called {
		t.Fatal("checkCoverage emitted a duplicate defect after the governing surface failed to load")
	}
}

func TestCoverage90CoverageCheckRejectsMalformedRequirement(t *testing.T) {
	called := false
	checkCoverage(&Env{
		Doc: &Doc{Coverage: []CoverageRow{{Requirement: "not-a-prd-reference"}}},
		Gov: governing{live: map[string]bool{}},
	}, func(string, string, string) {
		called = true
	})
	if !called {
		t.Fatal("checkCoverage accepted a malformed requirement reference")
	}
}
