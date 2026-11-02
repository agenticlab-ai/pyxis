package reconcile

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestCodeInventoryDoesNotCreateSemanticObligations(t *testing.T) {
	unit := codeinventory.Unit{Kind: codeinventory.GoPackage, Path: "internal/widget", Files: []string{"internal/widget/widget.go"}, SHA256: strings.Repeat("a", 64)}
	set, proj := loadReconcileRecords(t)
	snapshot := Snapshot{Commit: strings.Repeat("a", 40), Code: []codeinventory.Unit{unit}}
	report := Analyze(snapshot, set, proj, Options{})
	if report.Blocking() || len(report.Actions) != 0 || len(report.Findings) != 0 {
		t.Fatalf("code inventory manufactured semantic work: %#v", report)
	}
	legacy := Analyze(snapshot, set, proj, Options{LegacyCodeCoverage: true})
	if legacy.Generator != "memory-reconcile/v6" || !hasFinding(legacy, "missing-code-anchor", unit.Path) || !hasFinding(legacy, "missing-code-memory", unit.Path) {
		t.Fatalf("historical inventory rules changed: %#v", legacy)
	}

	anchor := fixtureRecord(record.Need, codeAnchorSubject(unit.Path), "anchor", 10)
	anchor.Tags = []string{"reconcile:anchor", "code:" + unit.Path}
	anchor.Provenance.Note = "offline code inventory " + unit.SHA256
	set, proj = loadReconcileRecords(t, reconcileRecord{record: anchor})
	unit.SHA256 = strings.Repeat("b", 64)
	snapshot.Code = []codeinventory.Unit{unit}
	for _, current := range []Snapshot{snapshot, {}} {
		got := Analyze(current, set, proj, Options{})
		if got.Blocking() || len(got.Actions) != 0 {
			t.Fatalf("historical anchor forced a new record after code change/removal: %#v", got)
		}
		if got := CodeTransitionRegressions(snapshot, current, set, proj, set, proj); len(got) != 0 {
			t.Fatalf("code-only transition required semantic issuance: %#v", got)
		}
	}
}

func TestSemanticAssociationsStillRequireValidCurrentScopes(t *testing.T) {
	r := fixtureRecord(record.Decision, "design/widget", "D-1", 10)
	r.Tags = []string{"code:../escape", "code:internal/missing"}
	set, proj := loadReconcileRecords(t, reconcileRecord{record: r})
	report := Analyze(Snapshot{}, set, proj, Options{})
	if !hasFinding(report, "invalid-code-scope", r.ID+" -> ../escape") || !hasFinding(report, "dangling-code-scope", r.ID+" -> internal/missing") {
		t.Fatalf("authored associations escaped validation: %#v", report)
	}
}
