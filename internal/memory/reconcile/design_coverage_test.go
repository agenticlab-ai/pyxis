package reconcile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestDesignCoverageDraftDefersUntilEveryPRDHeaderTargetIsExact(t *testing.T) {
	body := []byte("# Widget design\n")
	file := File{Path: "docs/tech-design/widget-v1.md", Mode: "100644", SHA256: sourceDigest(body), Body: body}
	design := Design{
		Slug: "widget-v1", Source: file.Path, SHA256: file.SHA256, Status: "live",
		Serves: []string{"prd:widget/FR-1", "prd:widget/FR-2", "external:ignored"},
	}
	document := fixtureDocument("design/widget-v1", "design", file.Path, body, 10)
	first := coveragePRDUnit("FR-1", "First", "1")
	second := coveragePRDUnit("FR-2", "Second", "2")
	target := prdMemoryRecord(first, 11)
	set, proj := loadReconcileRecords(t,
		reconcileRecord{record: document, body: body},
		reconcileRecord{record: target},
	)
	snapshot := coverageSnapshot(file, first, second)
	if draft, ok := designCoverageDraft(snapshot, design, set, proj); ok || !reflect.DeepEqual(draft, Draft{}) {
		t.Fatalf("partial target set produced coverage draft = %#v, %v", draft, ok)
	}
	secondTarget := prdMemoryRecord(second, 12)
	set, proj = loadReconcileRecords(t,
		reconcileRecord{record: document, body: body},
		reconcileRecord{record: target},
		reconcileRecord{record: secondTarget},
	)
	draft, ok := designCoverageDraft(snapshot, design, set, proj)
	if !ok {
		t.Fatal("complete exact design header produced no coverage draft")
	}
	if draft.Kind != record.Decision || draft.Subject != "design/widget-v1" || draft.Family != designCoverageFamily || !strings.Contains(draft.Statement, "prd:widget/FR-2") {
		t.Fatalf("coverage draft = %#v", draft)
	}
	wantRelations := []record.Relation{
		{Type: record.DependsOn, Target: document.ID},
		{Type: record.Serves, Target: target.ID},
		{Type: record.Serves, Target: secondTarget.ID},
	}
	if !reflect.DeepEqual(draft.Relations, wantRelations) {
		t.Fatalf("relations = %#v, want %#v", draft.Relations, wantRelations)
	}
	if !reflect.DeepEqual(draft.Tags, []string{designCoverageTag, "source-design:" + file.SHA256, "source:tech-design"}) || draft.Provenance.Source != file.Path || draft.Provenance.Commit != snapshot.Commit {
		t.Fatalf("source identity = %#v", draft)
	}
}

func TestDesignCoverageDraftIgnoresExactLiveDocgateWorkflowContext(t *testing.T) {
	body := []byte("# Docgate design\n")
	file := File{Path: "docs/tech-design/docgate-v2.md", SHA256: sourceDigest(body), Body: body}
	design := Design{
		Slug: "docgate-v2", Source: file.Path, SHA256: file.SHA256, Status: "live",
		Serves: []string{"prd:docgate/FR-1", "prd:docgate/WR-1"},
	}
	requirement := coveragePRDUnit("FR-1", "Gate documents", "1")
	context := PRDUnit{ID: "WR-1", Prefix: "WR", Status: "live"}
	document := fixtureDocument("design/docgate-v2", "design", file.Path, body, 13)
	target := prdMemoryRecord(requirement, 14)
	target.Subject = "prd/docgate"
	set, proj := loadReconcileRecords(t,
		reconcileRecord{record: document, body: body},
		reconcileRecord{record: target},
	)
	snapshot := Snapshot{
		Commit: strings.Repeat("a", 40), Files: map[string]File{file.Path: file},
		PRDs: []PRD{{Feature: "docgate", Units: []PRDUnit{requirement, context}}},
	}
	draft, ok := designCoverageDraft(snapshot, design, set, proj)
	if !ok {
		t.Fatal("live WR context prevented exact coverage projection")
	}
	want := []record.Relation{
		{Type: record.DependsOn, Target: document.ID},
		{Type: record.Serves, Target: target.ID},
	}
	if !reflect.DeepEqual(draft.Relations, want) || !strings.Contains(draft.Statement, "prd:docgate/WR-1") {
		t.Fatalf("coverage draft = %#v, want exact header and only durable relations %#v", draft, want)
	}

	context.Status = "retired"
	snapshot.PRDs[0].Units[1] = context
	if draft, ok := designCoverageDraft(snapshot, design, set, proj); ok || !reflect.DeepEqual(draft, Draft{}) {
		t.Fatalf("retired WR target was silently omitted: %#v, %v", draft, ok)
	}
}

func TestDesignCoverageDraftFailsClosedWithoutExactPrerequisites(t *testing.T) {
	body := []byte("# Widget design\n")
	file := File{Path: "docs/tech-design/widget-v1.md", SHA256: sourceDigest(body), Body: body}
	design := Design{Slug: "widget-v1", Source: file.Path, SHA256: file.SHA256, Status: "live", Serves: []string{"prd:widget/FR-1"}}
	unit := coveragePRDUnit("FR-1", "First", "1")
	target := prdMemoryRecord(unit, 20)
	driftedTarget := *target
	driftedTarget.Statement = "Drifted"
	document := fixtureDocument("design/widget-v1", "design", file.Path, body, 21)
	snapshot := coverageSnapshot(file, unit)
	linkedDecision := fixtureRecord(record.Decision, "design/widget-v1", "D-1", 22)
	linkedDecision.Relations = []record.Relation{{Type: record.Serves, Target: target.ID}}
	coverageHead := fixtureRecord(record.Decision, "design/widget-v1", designCoverageFamily, 23)

	for _, tt := range []struct {
		name   string
		snap   Snapshot
		design Design
		rows   []reconcileRecord
	}{
		{name: "not live", snap: snapshot, design: func() Design { d := design; d.Status = "superseded"; return d }(), rows: []reconcileRecord{{record: document, body: body}, {record: target}}},
		{name: "no serves", snap: snapshot, design: func() Design { d := design; d.Serves = nil; return d }(), rows: []reconcileRecord{{record: document, body: body}, {record: target}}},
		{name: "invalid design digest", snap: snapshot, design: func() Design { d := design; d.SHA256 = "bad"; return d }(), rows: []reconcileRecord{{record: document, body: body}, {record: target}}},
		{name: "non commit", snap: func() Snapshot { s := snapshot; s.Commit = "main"; return s }(), design: design, rows: []reconcileRecord{{record: document, body: body}, {record: target}}},
		{name: "no exact source document", snap: snapshot, design: design, rows: []reconcileRecord{{record: target}}},
		{name: "no resolvable PRD target", snap: snapshot, design: design, rows: []reconcileRecord{{record: document, body: body}}},
		{name: "drifted PRD target", snap: snapshot, design: design, rows: []reconcileRecord{{record: document, body: body}, {record: &driftedTarget}}},
		{name: "existing decisions already carry every target", snap: snapshot, design: design, rows: []reconcileRecord{{record: document, body: body}, {record: target}, {record: linkedDecision}}},
		{name: "coverage family already projected", snap: snapshot, design: design, rows: []reconcileRecord{{record: document, body: body}, {record: target}, {record: coverageHead}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			set, proj := loadReconcileRecords(t, tt.rows...)
			if draft, ok := designCoverageDraft(tt.snap, tt.design, set, proj); ok || !reflect.DeepEqual(draft, Draft{}) {
				t.Fatalf("designCoverageDraft() = %#v, %v", draft, ok)
			}
		})
	}
}

func TestAnalyzeDesignsOffersCoverageOnlyAfterDependenciesExist(t *testing.T) {
	body := []byte("# Widget design\n\n## 3. Design\n\n**Decision:** Preserve the widget contract.\n")
	file := File{Path: "docs/tech-design/widget-v1.md", SHA256: sourceDigest(body), Body: body}
	design := Design{Slug: "widget-v1", Source: file.Path, SHA256: file.SHA256, Status: "live", Serves: []string{"prd:widget/FR-1"}}
	document := fixtureDocument("design/widget-v1", "design", file.Path, body, 30)
	unit := coveragePRDUnit("FR-1", "First", "1")
	target := prdMemoryRecord(unit, 31)
	set, proj := loadReconcileRecords(t, reconcileRecord{record: document, body: body}, reconcileRecord{record: target})
	var report Report
	snapshot := coverageSnapshot(file, unit)
	snapshot.Designs = []Design{design}
	analyzeDesigns(&report, snapshot, set, proj)
	if len(report.Actions) != 2 || report.Actions[0].Code != "add-design-decision" || report.Actions[1].Code != "add-design-coverage" || report.Actions[1].Scope != "design/widget-v1" {
		t.Fatalf("coverage actions = %#v", report.Actions)
	}
}

func TestValidateDesignCoverageMemoryRejectsSemanticOrRelationDrift(t *testing.T) {
	body := []byte("# Widget design\n")
	file := File{Path: "docs/tech-design/widget-v1.md", SHA256: sourceDigest(body), Body: body}
	design := Design{Slug: "widget-v1", Source: file.Path, SHA256: file.SHA256, Status: "live", Serves: []string{"prd:widget/FR-1"}}
	document := fixtureDocument("design/widget-v1", "design", file.Path, body, 40)
	unit := coveragePRDUnit("FR-1", "First", "1")
	target := prdMemoryRecord(unit, 41)
	set, proj := loadReconcileRecords(t, reconcileRecord{record: document, body: body}, reconcileRecord{record: target})
	snapshot := coverageSnapshot(file, unit)
	expected, ok := expectedDesignCoverage(snapshot, design, set, proj)
	if !ok {
		t.Fatal("expected coverage unavailable")
	}
	makeRecord := func() *record.Record {
		r := fixtureRecord(record.Decision, expected.Subject, expected.Family, 42)
		r.Statement, r.Intent, r.Rationale, r.FitCriterion = expected.Statement, expected.Intent, expected.Rationale, expected.FitCriterion
		r.Tags = append([]string(nil), expected.Tags...)
		r.Relations = append([]record.Relation(nil), expected.Relations...)
		r.Provenance = expected.Provenance
		return r
	}
	if err := validateDesignCoverageMemory(makeRecord(), expected); err != nil {
		t.Fatalf("exact coverage rejected: %v", err)
	}
	successor := makeRecord()
	old := fixtureRecord(record.Decision, expected.Subject, expected.Family, 39)
	successor.Relations = append(successor.Relations, record.Relation{Type: record.Supersedes, Target: old.ID})
	if err := validateDesignCoverageMemory(successor, expected); err != nil {
		t.Fatalf("exact immutable coverage successor rejected: %v", err)
	}
	for _, tt := range []struct {
		name   string
		mutate func(*record.Record)
	}{
		{name: "statement", mutate: func(r *record.Record) { r.Statement = "different" }},
		{name: "source tag", mutate: func(r *record.Record) { r.Tags[1] = "source-design:" + strings.Repeat("0", 64) }},
		{name: "extra code tag", mutate: func(r *record.Record) { r.Tags = append(r.Tags, "code:internal/unmentioned") }},
		{name: "invented consequence", mutate: func(r *record.Record) { r.Consequence = record.High }},
		{name: "provenance", mutate: func(r *record.Record) { r.Provenance.Source = "docs/tech-design/other.md" }},
		{name: "provenance note", mutate: func(r *record.Record) { r.Provenance.Note = "approximate header" }},
		{name: "relation", mutate: func(r *record.Record) { r.Relations = r.Relations[:1] }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := makeRecord()
			tt.mutate(r)
			if err := validateDesignCoverageMemory(r, expected); err == nil {
				t.Fatalf("drifted coverage was accepted: %#v", r)
			}
		})
	}
}

func coveragePRDUnit(id, statement, digestRune string) PRDUnit {
	return PRDUnit{
		ID: id, Prefix: "FR", Status: "live", Kind: record.Promise, Statement: statement,
		SourceSHA256: strings.Repeat(digestRune, 64), Mapped: true,
	}
}

func coverageSnapshot(file File, units ...PRDUnit) Snapshot {
	return Snapshot{
		Commit: strings.Repeat("a", 40), Files: map[string]File{file.Path: file},
		PRDs: []PRD{{Feature: "widget", Units: units}},
	}
}
