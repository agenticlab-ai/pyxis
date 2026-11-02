package reconcile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestDesignDecisionDraftPreservesEvidenceAndExactCodeAssociation(t *testing.T) {
	snapshot, design, file, document, _, block := designMemoryFixture(t)
	draft, ok := designDecisionDraft(snapshot, block, document)
	if !ok {
		t.Fatal("exact design decision produced no draft")
	}
	if draft.Kind != record.Decision || draft.Subject != block.Subject || draft.Family != block.Family || draft.Statement != block.Statement || draft.Rationale != "" || draft.FitCriterion != "" {
		t.Fatalf("decision draft = %#v", draft)
	}
	wantRelations := []record.Relation{
		{Type: record.DependsOn, Target: document.ID},
	}
	if !reflect.DeepEqual(draft.Relations, wantRelations) {
		t.Fatalf("relations = %#v, want %#v", draft.Relations, wantRelations)
	}
	for _, want := range []string{designDecisionMemoryTag, "source-block:" + block.EvidenceSHA256, "source-design:" + design.SHA256, "source:tech-design"} {
		if !containsString(draft.Tags, want) {
			t.Errorf("tags omit %s: %#v", want, draft.Tags)
		}
	}
	if !containsString(draft.Tags, "code:internal/widget") {
		t.Fatalf("tags omit exact code association: %#v", draft.Tags)
	}
	if draft.Provenance.Source != file.Path || draft.Provenance.Commit != snapshot.Commit || !strings.Contains(draft.Provenance.Note, block.EvidenceSHA256) || !strings.Contains(draft.Provenance.Note, "lines 7-7") {
		t.Fatalf("provenance = %#v", draft.Provenance)
	}
}

func TestDesignDecisionDraftUsesStableDirectCodeScopeWithoutAnchor(t *testing.T) {
	snapshot, _, _, document, _, block := designMemoryFixture(t)
	draft, ok := designDecisionDraft(snapshot, block, document)
	if !ok || !containsString(draft.Tags, "code:internal/widget") {
		t.Fatalf("draft without structural anchor = %#v, %v", draft, ok)
	}
}

func TestAnalyzeDesignDecisionMemoryReachesExactFixedPoint(t *testing.T) {
	snapshot, design, file, document, _, block := designMemoryFixture(t)
	snapshot.Designs = []Design{design}
	set, proj := loadReconcileRecords(t,
		reconcileRecord{record: document, body: file.Body},
	)
	var report Report
	if !analyzeDesignDecisionMemory(&report, snapshot, design, set, proj) {
		t.Fatal("live pinned design source was not checked")
	}
	if !hasFinding(report, "missing-design-memory", "design/widget-v1") || len(report.Actions) != 1 || report.Actions[0].Code != "add-design-decision" || report.Actions[0].Scope != block.Subject+"#"+block.Family {
		t.Fatalf("initial design analysis = findings %#v actions %#v", report.Findings, report.Actions)
	}

	draft := report.Actions[0].Draft
	decision := fixtureRecord(record.Decision, draft.Subject, draft.Family, 12)
	decision.Statement = draft.Statement
	decision.Tags = draft.Tags
	decision.Relations = draft.Relations
	decision.Provenance = draft.Provenance
	set, proj = loadReconcileRecords(t,
		reconcileRecord{record: document, body: file.Body},
		reconcileRecord{record: decision},
	)
	report = Report{}
	analyzeDesignDecisionMemory(&report, snapshot, design, set, proj)
	if len(report.Findings) != 0 || len(report.Actions) != 0 {
		t.Fatalf("exact decision was not a fixed point: %#v", report)
	}
}

func TestAnalyzeDesignDecisionMemoryAcceptsExactImmutableSuccessor(t *testing.T) {
	snapshot, design, file, document, _, block := designMemoryFixture(t)
	draft, ok := designDecisionDraft(snapshot, block, document)
	if !ok {
		t.Fatal("fixture draft unavailable")
	}
	old := fixtureRecord(record.Decision, draft.Subject, draft.Family, 12)
	old.Statement, old.Tags, old.Relations, old.Provenance = "older source statement", []string{"source:tech-design"}, append([]record.Relation(nil), draft.Relations...), draft.Provenance
	successor := fixtureRecord(record.Decision, draft.Subject, draft.Family, 13)
	successor.Statement, successor.Tags, successor.Relations, successor.Provenance = draft.Statement, append([]string(nil), draft.Tags...), append([]record.Relation(nil), draft.Relations...), draft.Provenance
	successor.Relations = append(successor.Relations, record.Relation{Type: record.Supersedes, Target: old.ID})
	set, proj := loadReconcileRecords(t,
		reconcileRecord{record: document, body: file.Body},
		reconcileRecord{record: old},
		reconcileRecord{record: successor},
	)
	var report Report
	analyzeDesignDecisionMemory(&report, snapshot, design, set, proj)
	if len(report.Findings) != 0 || len(report.Actions) != 0 {
		t.Fatalf("exact immutable successor was rejected: %#v", report)
	}
}

func TestDesignDecisionDirectCodeScopeSurvivesAnchorRefresh(t *testing.T) {
	snapshot, design, file, document, oldAnchor, block := designMemoryFixture(t)
	snapshot.Designs = []Design{design}
	draft, ok := designDecisionDraft(snapshot, block, document)
	if !ok {
		t.Fatal("fixture draft unavailable")
	}
	decision := fixtureRecord(record.Decision, draft.Subject, draft.Family, 12)
	decision.Statement, decision.Tags, decision.Relations, decision.Provenance = draft.Statement, draft.Tags, draft.Relations, draft.Provenance
	newAnchor := fixtureRecord(record.Need, oldAnchor.Subject, oldAnchor.Family, 13)
	newAnchor.Statement = oldAnchor.Statement
	newAnchor.Tags = append([]string(nil), oldAnchor.Tags...)
	newAnchor.Provenance = oldAnchor.Provenance
	newAnchor.Relations = []record.Relation{{Type: record.Supersedes, Target: oldAnchor.ID}}
	set, proj := loadReconcileRecords(t,
		reconcileRecord{record: document, body: file.Body},
		reconcileRecord{record: oldAnchor},
		reconcileRecord{record: decision},
		reconcileRecord{record: newAnchor},
	)
	var report Report
	analyzeDesignDecisionMemory(&report, snapshot, design, set, proj)
	if len(report.Findings) != 0 || len(report.Actions) != 0 {
		t.Fatalf("stable direct code scope drifted after anchor refresh: %#v", report)
	}
}

func TestAnalyzeDesignDecisionMemoryRejectsIncompleteNativeRecord(t *testing.T) {
	snapshot, design, file, document, _, block := designMemoryFixture(t)
	draft, ok := designDecisionDraft(snapshot, block, document)
	if !ok {
		t.Fatal("fixture draft unavailable")
	}
	base := fixtureRecord(record.Decision, draft.Subject, draft.Family, 12)
	base.Statement, base.Tags, base.Relations, base.Provenance = draft.Statement, draft.Tags, draft.Relations, draft.Provenance

	for _, tt := range []struct {
		name   string
		mutate func(*record.Record)
	}{
		{name: "statement drift", mutate: func(r *record.Record) { r.Statement = "different" }},
		{name: "invented rationale", mutate: func(r *record.Record) { r.Rationale = "inferred from adjacent prose" }},
		{name: "block tag drift", mutate: func(r *record.Record) { r.Tags[1] = "source-block:" + strings.Repeat("0", 64) }},
		{name: "extra code tag", mutate: func(r *record.Record) { r.Tags = append(r.Tags, "code:internal/unmentioned") }},
		{name: "source provenance drift", mutate: func(r *record.Record) { r.Provenance.Source = "docs/tech-design/other.md" }},
		{name: "source note drift", mutate: func(r *record.Record) { r.Provenance.Note = "approximate evidence" }},
		{name: "missing document dependency", mutate: func(r *record.Record) { r.Relations = nil }},
		{name: "missing code association", mutate: func(r *record.Record) {
			for i, tag := range r.Tags {
				if strings.HasPrefix(tag, "code:") {
					r.Tags = append(r.Tags[:i], r.Tags[i+1:]...)
					break
				}
			}
		}},
		{name: "extra relation", mutate: func(r *record.Record) {
			r.Relations = append(r.Relations, record.Relation{Type: record.Refines, Target: document.ID})
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := *base
			r.Tags = append([]string(nil), base.Tags...)
			r.Relations = append([]record.Relation(nil), base.Relations...)
			tt.mutate(&r)
			set, proj := loadReconcileRecords(t,
				reconcileRecord{record: document, body: file.Body},
				reconcileRecord{record: &r},
			)
			var report Report
			analyzeDesignDecisionMemory(&report, snapshot, design, set, proj)
			if !hasFinding(report, "design-decision-memory-drift", block.Subject+"#"+block.Family) {
				t.Fatalf("drift was accepted: %#v", report.Findings)
			}
		})
	}
}

func TestAnalyzeDesignDecisionMemoryRejectsObsoleteProjectedBlock(t *testing.T) {
	snapshot, design, file, document, _, block := designMemoryFixture(t)
	draft, ok := designDecisionDraft(snapshot, block, document)
	if !ok {
		t.Fatal("fixture draft unavailable")
	}
	exact := fixtureRecord(record.Decision, draft.Subject, draft.Family, 12)
	exact.Statement, exact.Tags, exact.Relations, exact.Provenance = draft.Statement, draft.Tags, draft.Relations, draft.Provenance
	obsolete := fixtureRecord(record.Decision, draft.Subject, "decision.removed.1", 13)
	obsolete.Tags = []string{designDecisionMemoryTag}
	set, proj := loadReconcileRecords(t,
		reconcileRecord{record: document, body: file.Body},
		reconcileRecord{record: exact},
		reconcileRecord{record: obsolete},
	)
	var report Report
	analyzeDesignDecisionMemory(&report, snapshot, design, set, proj)
	if !hasFinding(report, "stale-design-decision-memory", obsolete.Subject+"#"+obsolete.Family) {
		t.Fatalf("obsolete source-projected Decision was accepted: %#v", report.Findings)
	}
}

func TestMentionedCodeUnitsUsesExactPathTokens(t *testing.T) {
	units := []codeinventory.Unit{
		{Path: "internal/widget", Files: []string{"internal/widget/widget.go"}},
		{Path: "internal/widget-extra", Files: []string{"internal/widget-extra/extra.go"}},
		{Path: ".pyxis/scripts/check.sh", Files: []string{".pyxis/scripts/check.sh"}},
	}
	got := mentionedCodeUnits("Use internal/widget. and .pyxis/scripts/check.sh; not internal/widgetary.", units)
	var paths []string
	for _, unit := range got {
		paths = append(paths, unit.Path)
	}
	want := []string{".pyxis/scripts/check.sh", "internal/widget"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("mentionedCodeUnits() = %#v, want %#v", paths, want)
	}
}

func designMemoryFixture(t *testing.T) (Snapshot, Design, File, *record.Record, *record.Record, DesignDecisionBlock) {
	t.Helper()
	body := []byte("# Widget\n\n## 3. Design\n\n### 3.1 Runtime\n\n**Decision:** Implement the widget in `internal/widget`.\n")
	digest := decisionDigest(body)
	file := File{Path: "docs/tech-design/widget-v1.md", Mode: "100644", SHA256: digest, Body: body}
	design := Design{Slug: "widget-v1", Source: file.Path, SHA256: digest, Status: "live"}
	blocks, err := ExtractLiveDesignDecisions(design, file)
	if err != nil || len(blocks) != 1 {
		t.Fatalf("extract fixture blocks = %#v, %v", blocks, err)
	}
	document := fixtureDocument("design/widget-v1", "design", file.Path, body, 10)
	unit := codeinventory.Unit{Kind: codeinventory.GoPackage, Path: "internal/widget", Files: []string{"internal/widget/widget.go"}, SHA256: strings.Repeat("3", 64)}
	anchor := fixtureRecord(record.Need, codeAnchorSubject(unit.Path), "anchor", 11)
	anchor.Statement = "Production code unit internal/widget exists."
	anchor.Tags = []string{"code:" + unit.Path, "reconcile:anchor"}
	anchor.Provenance = record.Provenance{Source: "code:" + unit.Path, Commit: strings.Repeat("a", 40), Note: "offline code inventory " + unit.SHA256}
	snapshot := Snapshot{Commit: strings.Repeat("a", 40), Files: map[string]File{file.Path: file}, Code: []codeinventory.Unit{unit}}
	return snapshot, design, file, document, anchor, blocks[0]
}
