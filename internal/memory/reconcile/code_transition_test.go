package reconcile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestCodeTransitionRegressionsFailClosedWithoutRelatedProjections(t *testing.T) {
	set, projection := loadReconcileRecords(t)
	other := &store.Set{}
	for _, tc := range []struct {
		name        string
		priorSet    *store.Set
		priorProj   *resolve.Projection
		currentSet  *store.Set
		currentProj *resolve.Projection
	}{
		{name: "missing prior", currentSet: set, currentProj: projection},
		{name: "missing current", priorSet: set, priorProj: projection},
		{name: "unrelated prior projection", priorSet: other, priorProj: projection, currentSet: set, currentProj: projection},
		{name: "unrelated current projection", priorSet: set, priorProj: projection, currentSet: other, currentProj: projection},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CodeTransitionRegressions(Snapshot{}, Snapshot{}, tc.priorSet, tc.priorProj, tc.currentSet, tc.currentProj)
			if len(got) != 1 || got[0].Code != "code-memory-transition" || got[0].Scope != "code/" || !got[0].CurrentBlocking {
				t.Fatalf("CodeTransitionRegressions() = %#v", got)
			}
		})
	}
}

func TestLegacyCodeTransitionRegressionsRequireFreshSemanticMemory(t *testing.T) {
	unit := func(path, digest string) codeinventory.Unit {
		return codeinventory.Unit{Kind: codeinventory.GoPackage, Path: path, Files: []string{path + "/main.go"}, SHA256: digest}
	}
	oldWidget := unit("internal/widget", strings.Repeat("a", 64))
	newWidget := unit("internal/widget", strings.Repeat("b", 64))
	newGadget := unit("internal/gadget", strings.Repeat("c", 64))
	oldSemantic := fixtureRecord(record.Decision, "design/widget", "architecture", 10)
	oldSemantic.Tags = []string{"code:internal/widget"}
	priorSet, priorProjection := loadReconcileRecords(t, reconcileRecord{record: oldSemantic})

	t.Run("unchanged code needs no new record", func(t *testing.T) {
		if got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{oldWidget}}, Snapshot{Code: []codeinventory.Unit{oldWidget}}, priorSet, priorProjection, priorSet, priorProjection); len(got) != 0 {
			t.Fatalf("unchanged unit regressed: %#v", got)
		}
	})

	t.Run("changed and new code without fresh semantics regress deterministically", func(t *testing.T) {
		got := legacyCodeTransitionRegressions(
			Snapshot{Code: []codeinventory.Unit{oldWidget}},
			Snapshot{Code: []codeinventory.Unit{newWidget, newGadget}},
			priorSet, priorProjection, priorSet, priorProjection,
		)
		want := []BaselineDelta{
			codeTransitionDelta("internal/gadget", "new production code unit has no newly issued active semantic memory that explicitly covers it"),
			codeTransitionDelta("internal/widget", "changed production code unit has no newly issued active semantic memory that explicitly covers it"),
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("regressions = %#v, want %#v", got, want)
		}
	})

	t.Run("a structural anchor cannot launder changed code", func(t *testing.T) {
		anchor := fixtureRecord(record.Need, codeAnchorSubject(newWidget.Path), "anchor", 20)
		anchor.Tags = []string{"code:" + newWidget.Path, "reconcile:anchor"}
		currentSet, currentProjection := loadReconcileRecords(t, reconcileRecord{record: oldSemantic}, reconcileRecord{record: anchor})
		got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{oldWidget}}, Snapshot{Code: []codeinventory.Unit{newWidget}}, priorSet, priorProjection, currentSet, currentProjection)
		if len(got) != 1 || got[0].Scope != newWidget.Path {
			t.Fatalf("fresh anchor hid semantic drift: %#v", got)
		}
	})

	t.Run("a fresh active semantic record reconciles changed code", func(t *testing.T) {
		fresh := fixtureRecord(record.Learning, "learning/widget", "implementation-change", 20)
		fresh.Tags = []string{"code:" + newWidget.Path}
		fresh.Provenance.Evidence = &record.Evidence{Schema: record.EvidenceSchema, Locator: record.EvidenceIntroduction,
			Files: []record.EvidenceFile{{Path: newWidget.Files[0], Mode: "100644", SHA256: strings.Repeat("f", 64)}},
			Units: []record.EvidenceUnit{{Inventory: record.EvidenceInventory, Domain: record.EvidenceUnitDomain, Kind: string(newWidget.Kind), Path: newWidget.Path, SHA256: newWidget.SHA256, Files: newWidget.Files}},
		}
		currentSet, currentProjection := loadReconcileRecords(t, reconcileRecord{record: oldSemantic}, reconcileRecord{record: fresh})
		if got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{oldWidget}}, Snapshot{Code: []codeinventory.Unit{newWidget}}, priorSet, priorProjection, currentSet, currentProjection); len(got) != 0 {
			t.Fatalf("fresh semantic record did not reconcile change: %#v", got)
		}
		fresh.Provenance.Evidence.Units[0].SHA256 = oldWidget.SHA256
		currentSet, currentProjection = loadReconcileRecords(t, reconcileRecord{record: oldSemantic}, reconcileRecord{record: fresh})
		if got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{oldWidget}}, Snapshot{Code: []codeinventory.Unit{newWidget}}, priorSet, priorProjection, currentSet, currentProjection); len(got) != 1 {
			t.Fatal("historical witness acknowledged newer code")
		}
	})

	t.Run("inactive and literal records do not count as fresh semantics", func(t *testing.T) {
		inactive := fixtureRecord(record.Decision, "design/widget", "replacement", 20)
		inactive.Tags = []string{"code:" + newWidget.Path}
		revocation := fixtureRecord(record.Revocation, inactive.Subject, "", 30)
		revocation.Relations = []record.Relation{{Type: record.Revokes, Target: inactive.ID}}
		body := []byte("# Imported notes\n")
		literal := fixtureDocument("source/import", "document", "docs/import.md", body, 40)
		literal.Tags = append(literal.Tags, "code:"+newWidget.Path)
		currentSet, currentProjection := loadReconcileRecords(t,
			reconcileRecord{record: oldSemantic}, reconcileRecord{record: inactive},
			reconcileRecord{record: revocation}, reconcileRecord{record: literal, body: body},
		)
		got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{oldWidget}}, Snapshot{Code: []codeinventory.Unit{newWidget}}, priorSet, priorProjection, currentSet, currentProjection)
		if len(got) != 1 || got[0].Scope != newWidget.Path {
			t.Fatalf("inactive or literal record hid semantic drift: %#v", got)
		}
	})
}

func TestLegacyCodeTransitionRegressionsRequireRemovedIdentityRetirement(t *testing.T) {
	unit := codeinventory.Unit{Kind: codeinventory.ExecutableScript, Path: ".pyxis/scripts/task.sh", Files: []string{".pyxis/scripts/task.sh"}, SHA256: strings.Repeat("d", 64)}
	anchor := fixtureRecord(record.Need, codeAnchorSubject(unit.Path), "anchor", 10)
	anchor.Tags = []string{"code:" + unit.Path, "reconcile:anchor"}
	priorSet, priorProjection := loadReconcileRecords(t, reconcileRecord{record: anchor})

	got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{unit}}, Snapshot{}, priorSet, priorProjection, priorSet, priorProjection)
	if len(got) != 1 || got[0].Scope != unit.Path || !strings.Contains(got[0].Reason, "removed production code unit") {
		t.Fatalf("unacknowledged removal = %#v", got)
	}

	replacement := fixtureRecord(record.Need, anchor.Subject, "anchor", 20)
	replacement.Tags = []string{"reconcile:retired-code"}
	replacement.Relations = []record.Relation{{Type: record.Supersedes, Target: anchor.ID}}
	currentSet, currentProjection := loadReconcileRecords(t, reconcileRecord{record: anchor}, reconcileRecord{record: replacement})
	if got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{unit}}, Snapshot{}, priorSet, priorProjection, currentSet, currentProjection); len(got) != 0 {
		t.Fatalf("retired code identity regressed: %#v", got)
	}
}

func TestLegacyCodeTransitionRegressionsRequireExactRetirementLearningForUnidentifiedCode(t *testing.T) {
	unit := codeinventory.Unit{Kind: codeinventory.GoPackage, Path: "internal/legacy", Files: []string{"internal/legacy/main.go"}, SHA256: strings.Repeat("e", 64)}
	priorSet, priorProjection := loadReconcileRecords(t)

	assertBlocked := func(t *testing.T, entries ...reconcileRecord) {
		t.Helper()
		currentSet, currentProjection := loadReconcileRecords(t, entries...)
		got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{unit}}, Snapshot{}, priorSet, priorProjection, currentSet, currentProjection)
		if len(got) != 1 || got[0].Scope != unit.Path || !strings.Contains(got[0].Reason, "retired-code-unit:"+unit.SHA256) {
			t.Fatalf("retirement evidence = %#v", got)
		}
	}
	assertBlocked(t)

	missingDigest := fixtureRecord(record.Learning, "learning/code-retirements", "", 20)
	missingDigest.Tags = []string{"reconcile:code-retirement"}
	assertBlocked(t, reconcileRecord{record: missingDigest})

	wrongKind := fixtureRecord(record.Decision, "design/code-retirements", "legacy", 21)
	wrongKind.Tags = []string{"reconcile:code-retirement", "retired-code-unit:" + unit.SHA256}
	assertBlocked(t, reconcileRecord{record: wrongKind})

	retirement := fixtureRecord(record.Learning, "learning/code-retirements", "", 22)
	retirement.Tags = []string{"reconcile:code-retirement", "retired-code-unit:" + unit.SHA256}
	currentSet, currentProjection := loadReconcileRecords(t, reconcileRecord{record: retirement})
	if got := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{unit}}, Snapshot{}, priorSet, priorProjection, currentSet, currentProjection); len(got) != 0 {
		t.Fatalf("exact retirement Learning did not reconcile legacy removal: %#v", got)
	}

	staleSet, staleProjection := loadReconcileRecords(t, reconcileRecord{record: retirement})
	assertStale := legacyCodeTransitionRegressions(Snapshot{Code: []codeinventory.Unit{unit}}, Snapshot{}, staleSet, staleProjection, staleSet, staleProjection)
	if len(assertStale) != 1 || assertStale[0].Scope != unit.Path {
		t.Fatalf("a retirement Learning already present at the baseline was reused: %#v", assertStale)
	}

	revocation := fixtureRecord(record.Revocation, retirement.Subject, "", 23)
	revocation.Relations = []record.Relation{{Type: record.Revokes, Target: retirement.ID}}
	inactiveSet, inactiveProjection := loadReconcileRecords(t, reconcileRecord{record: retirement}, reconcileRecord{record: revocation})
	assertBlocked(t, reconcileRecord{record: retirement}, reconcileRecord{record: revocation})
	if inactiveProjection.Status(retirement.ID) != resolve.Revoked || hasFreshCodeRetirement(unit, priorSet, inactiveSet, inactiveProjection) {
		t.Fatal("an inactive retirement Learning satisfied the transition")
	}
}
