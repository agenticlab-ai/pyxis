package reconcile

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestRequirementsFirstCannotBeIntroducedAfterTheBase(t *testing.T) {
	base := []byte(snapshotPRDBase)
	marked := []byte(strings.Replace(snapshotPRDBase, "| **Template version**", "| **Design coverage** | requirements-first/v1 |\n| **Template version**", 1))
	for _, tt := range []struct {
		name            string
		base, effective []byte
		want            bool
		err             string
	}{
		{"legacy", base, base, false, ""},
		{"explicit", marked, marked, true, ""},
		{"late opt in", base, marked, false, "original 000-base.md"},
		{"lost opt in", marked, base, false, "original 000-base.md"},
		{"unknown value", []byte(strings.ReplaceAll(string(marked), "requirements-first/v1", "draft")), base, false, "unknown Design coverage"},
		{"empty value", []byte(strings.ReplaceAll(string(marked), "requirements-first/v1", "")), base, false, "unknown Design coverage"},
		{"malformed base", []byte("invalid"), base, false, "parse header"},
		{"malformed projection", base, []byte("invalid"), false, "parse header"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := requirementsFirst(tt.base, tt.effective)
			if got != tt.want || (tt.err == "" && err != nil) || (tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err))) {
				t.Fatalf("requirementsFirst = %v, %v; want %v, %q", got, err, tt.want, tt.err)
			}
		})
	}
}

func TestPRDStageDefersOnlyUntilDesignStarts(t *testing.T) {
	first := coveragePRDUnit("FR-1", "First", "1")
	first.Priority = "Must"
	second := coveragePRDUnit("FR-2", "Second", "2")
	second.Priority = "Must"
	claim := prdMemoryRecord(first, 60)
	otherClaim := prdMemoryRecord(second, 61)
	decision := fixtureRecord(record.Decision, "design/widget-v1", "D-1", 62)
	decision.Relations = []record.Relation{{Type: record.Serves, Target: claim.ID}}
	revocation := fixtureRecord(record.Revocation, decision.Subject, "", 63)
	revocation.Relations = []record.Relation{{Type: record.Revokes, Target: decision.ID}}
	unrelated := fixtureRecord(record.Decision, "design/other", "D-1", 64)
	unrelated.Relations = []record.Relation{{Type: record.Serves, Target: "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV"}}
	for _, tt := range []struct {
		name     string
		staged   bool
		designs  []Design
		extra    []reconcileRecord
		unserved int
		deferred bool
	}{
		{name: "legacy debt unchanged", unserved: 2},
		{name: "requirements before designs", staged: true, deferred: true},
		{name: "unrelated design", staged: true, designs: []Design{{Serves: []string{"prd:other/FR-1", "external:context"}}}, deferred: true},
		{name: "first design requires every Must", staged: true, designs: []Design{{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-1"}}}, unserved: 2},
		{name: "partial exact coverage still blocks", staged: true, designs: []Design{{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-1"}}}, extra: []reconcileRecord{{record: decision}}, unserved: 1},
		{name: "superseded design cannot reset stage", staged: true, designs: []Design{{Status: "superseded", Serves: []string{"prd:widget/FR-1"}}}, unserved: 2},
		{name: "retired source and revoked decision cannot reset stage", staged: true, extra: []reconcileRecord{{record: decision}, {record: revocation}}, unserved: 2},
		{name: "unrelated dangling edge cannot start stage", staged: true, extra: []reconcileRecord{{record: unrelated}}, deferred: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := append([]reconcileRecord{{record: claim}, {record: otherClaim}}, tt.extra...)
			set, proj := loadReconcileRecords(t, rows...)
			s := Snapshot{PRDs: []PRD{{Feature: "widget", RequirementsFirst: tt.staged, Units: []PRDUnit{first, second}}}, Designs: tt.designs}
			var report Report
			analyzePRDs(&report, s, set, proj)
			count := 0
			for _, f := range report.Findings {
				if f.Code == "unserved-prd-memory" {
					count++
				}
			}
			if count != tt.unserved || (len(report.DeferredDesignFeatures) == 1) != tt.deferred {
				t.Fatalf("stage report = %+v; want unserved=%d deferred=%v", report, tt.unserved, tt.deferred)
			}
		})
	}
	set, proj := loadReconcileRecords(t)
	var report Report
	analyzePRDs(&report, Snapshot{PRDs: []PRD{{Feature: "widget", RequirementsFirst: true, Units: []PRDUnit{first}}}}, set, proj)
	if !hasFinding(report, "missing-prd-memory", "prd/widget#FR-1") {
		t.Fatalf("staging suppressed missing source claim: %+v", report)
	}
}

func TestBuildSnapshotReadsRequirementsStageFromPinnedBase(t *testing.T) {
	dir := newSnapshotRepo(t)
	writeSnapshotPRD(t, dir, "contracts", "widget")
	legacy := commitSnapshot(t, dir, "legacy contract")
	base := []byte(strings.Replace(snapshotPRDBase, "| **Template version**", "| **Design coverage** | requirements-first/v1 |\n| **Template version**", 1))
	result := fold.Fold([]ledger.Entry{{Name: "000-base.md", Bytes: base}})
	if len(result.Defects) != 0 {
		t.Fatal(result.Defects)
	}
	for name, body := range map[string][]byte{"000-base.md": base, "effective.md": result.EffectiveMD, "effective.source.json": result.SourceJSON} {
		writeSnapshotFile(t, dir, "contracts/widget/"+name, body, 0o644)
	}
	staged := commitSnapshot(t, dir, "marked contract")
	repo := openSnapshotRepo(t, dir)
	for _, tt := range []struct {
		commit string
		want   bool
	}{{legacy, false}, {staged, true}} {
		s := mustBuildSnapshot(t, repo, tt.commit, Options{PRDs: "contracts"})
		if len(s.PRDs) != 1 || s.PRDs[0].RequirementsFirst != tt.want {
			t.Fatalf("stage snapshot: %+v", s.PRDs)
		}
	}
	invalid := []byte(strings.ReplaceAll(string(base), "requirements-first/v1", "unknown/v1"))
	result = fold.Fold([]ledger.Entry{{Name: "000-base.md", Bytes: invalid}})
	for name, body := range map[string][]byte{"000-base.md": invalid, "effective.md": result.EffectiveMD, "effective.source.json": result.SourceJSON} {
		writeSnapshotFile(t, dir, "contracts/widget/"+name, body, 0o644)
	}
	if _, err := BuildSnapshot(t.Context(), repo, commitSnapshot(t, dir, "invalid marker"), Options{PRDs: "contracts"}); err == nil || !strings.Contains(err.Error(), "unknown Design coverage") {
		t.Fatalf("invalid stage accepted: %v", err)
	}
}
