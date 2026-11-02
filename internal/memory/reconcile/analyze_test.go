package reconcile

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestAnalyze_MemoryUnavailablePreservesCompilerFindingsAndCounts(t *testing.T) {
	s := Snapshot{
		Commit: "commit", SourceClosureSHA256: "sources", CodeInventorySHA256: "code", DesignIndexSHA256: "design",
		AuthoredSources: []string{"a.md", "b.md"},
		PRDs:            []PRD{{Units: []PRDUnit{{ID: "FR-1"}, {ID: "FR-2"}}}},
		Designs:         []Design{{Slug: "widget"}},
		CompilerFindings: []Finding{{
			Code: "compiler-gap", Scope: "source", Blocking: true,
		}},
	}
	for _, tt := range []struct {
		name string
		set  *store.Set
		proj *resolve.Projection
	}{
		{name: "both nil"},
		{name: "projection nil", set: &store.Set{}},
		{name: "set nil", proj: &resolve.Projection{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report := Analyze(s, tt.set, tt.proj, Options{})
			if report.Schema != ReportSchema || report.Generator != Generator || report.SourceCommit != s.Commit || report.SourceClosureSHA256 != s.SourceClosureSHA256 || report.CodeInventorySHA256 != s.CodeInventorySHA256 || report.DesignIndexSHA256 != s.DesignIndexSHA256 || report.MemoryClosureSHA256 != MemoryClosureSHA256(tt.set) {
				t.Fatalf("report identity = %#v", report)
			}
			if !hasFinding(report, "compiler-gap", "source") || !hasFinding(report, "memory-unavailable", "memory/") {
				t.Fatalf("findings = %#v", report.Findings)
			}
			if report.Counts.Sources != 2 || report.Counts.PRDUnits != 2 || report.Counts.Designs != 1 || report.Counts.Findings != 2 || report.Counts.Blocking != 2 || report.Counts.Actions != 0 {
				t.Fatalf("counts = %#v", report.Counts)
			}
		})
	}
}

func TestFinishReportUsesEmptyCollectionsAtFixedPoint(t *testing.T) {
	var report Report
	finishReport(&report, Snapshot{})
	if report.Findings == nil || report.Actions == nil {
		t.Fatalf("fixed-point collections must encode as JSON arrays: findings=%#v actions=%#v", report.Findings, report.Actions)
	}
}

func TestAnalyze_ReportsLoadedMemoryIntegrityFailures(t *testing.T) {
	a := fixtureRecord(record.Promise, "prd/widget", "FR-1", 1)
	b := fixtureRecord(record.Promise, "prd/widget", "FR-1", 2)
	c := fixtureRecord(record.Promise, "prd/other", "FR-2", 3)
	c.Relations = []record.Relation{{Type: record.Supersedes, Target: a.ID}}
	q := fixtureRecord(record.Question, "prd/widget", "", 4)
	a1 := fixturePRDAnswer(q, q.Subject, 5)
	a2 := fixturePRDAnswer(q, q.Subject, 6)
	target := fixtureRecord(record.Need, "prd/widget", "G-1", 7)
	revokeA := fixtureRecord(record.Revocation, target.Subject, "", 8)
	revokeA.Relations = []record.Relation{{Type: record.Revokes, Target: target.ID}}
	revokeB := fixtureRecord(record.Revocation, target.Subject, "", 9)
	revokeB.Relations = []record.Relation{{Type: record.Revokes, Target: target.ID}}
	set, proj := loadReconcileRecords(t, reconcileRecord{record: a}, reconcileRecord{record: b}, reconcileRecord{record: c}, reconcileRecord{record: q}, reconcileRecord{record: a1}, reconcileRecord{record: a2}, reconcileRecord{record: target}, reconcileRecord{record: revokeA}, reconcileRecord{record: revokeB})
	set.Problems = append(set.Problems, store.Problem{Path: "broken.json", Err: errors.New("bad envelope")})
	set.Dangling = append(set.Dangling, store.Dangling{From: a.ID, Type: record.Serves, Target: "Promise-missing"})

	report := Analyze(Snapshot{}, set, proj, Options{})
	for _, want := range []struct{ code, scope string }{
		{"invalid-memory-file", "broken.json"},
		{"dangling-memory-relation", a.ID},
		{"invalid-memory-projection", c.ID},
		{"answer-conflict", q.ID},
		{"revocation-conflict", target.ID},
		{"memory-family-conflict", "prd/widget#FR-1"},
	} {
		if !hasFinding(report, want.code, want.scope) {
			t.Errorf("missing %s/%s in %#v", want.code, want.scope, report.Findings)
		}
	}
}

func TestAnalyzeCode_FailClosedAnchorCases(t *testing.T) {
	unit := codeinventory.Unit{Kind: codeinventory.GoPackage, Path: "internal/widget", Files: []string{"internal/widget/widget.go"}, SHA256: strings.Repeat("a", 64)}
	tests := []struct {
		name      string
		unit      codeinventory.Unit
		records   func() []reconcileRecord
		wantCode  string
		wantScope string
	}{
		{name: "invalid associated scope", records: func() []reconcileRecord {
			r := fixtureRecord(record.Decision, "design/widget", "architecture", 50)
			r.Tags = []string{"code:../widget"}
			return []reconcileRecord{{record: r}}
		}, wantCode: "invalid-code-scope"},
		{name: "unmappable unit", unit: codeinventory.Unit{Kind: codeinventory.GoPackage, Path: "Internal/Widget", SHA256: strings.Repeat("b", 64)}, wantCode: "unmappable-code-anchor", wantScope: "Internal/Widget"},
		{name: "derived address occupied", unit: unit, records: func() []reconcileRecord {
			r := fixtureRecord(record.Promise, codeAnchorSubject(unit.Path), "anchor", 51)
			return []reconcileRecord{{record: r}}
		}, wantCode: "code-anchor-collision", wantScope: unit.Path},
		{name: "duplicate exact anchors", unit: unit, records: func() []reconcileRecord {
			a := fixtureRecord(record.Need, "code/first", "anchor", 52)
			a.Tags = []string{"code:" + unit.Path}
			b := fixtureRecord(record.Need, "code/second", "anchor", 53)
			b.Tags = []string{"code:" + unit.Path}
			return []reconcileRecord{{record: a}, {record: b}}
		}, wantCode: "duplicate-code-anchor", wantScope: unit.Path},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []reconcileRecord
			if tt.records != nil {
				entries = tt.records()
			}
			set, proj := loadReconcileRecords(t, entries...)
			var report Report
			snapshot := Snapshot{}
			if tt.unit.Path != "" {
				snapshot.Code = []codeinventory.Unit{tt.unit}
			}
			analyzeCode(&report, snapshot, set, proj)
			if tt.wantScope == "" {
				for _, finding := range report.Findings {
					if finding.Code == tt.wantCode {
						return
					}
				}
				t.Fatalf("missing %s in %#v", tt.wantCode, report.Findings)
			}
			if !hasFinding(report, tt.wantCode, tt.wantScope) {
				t.Fatalf("missing %s/%s in %#v", tt.wantCode, tt.wantScope, report.Findings)
			}
			if len(report.Actions) != 0 {
				t.Fatalf("fail-closed case proposed actions: %#v", report.Actions)
			}
		})
	}
}

func TestRemoveCollidingActions(t *testing.T) {
	report := Report{
		Actions: []Action{
			{Code: "capture", Scope: "a.md", Draft: Draft{Subject: "source/shared", Family: "document"}},
			{Code: "capture", Scope: "b.md", Draft: Draft{Subject: "source/shared", Family: "document"}},
			{Code: "capture", Scope: "c.md", Draft: Draft{Subject: "source/unique", Family: "document"}},
		},
		Findings: []Finding{
			{Code: "missing", Scope: "a.md", Remedy: "write", Writable: true, Blocking: true},
			{Code: "missing", Scope: "b.md", Remedy: "write", Writable: true, Blocking: true},
			{Code: "missing", Scope: "c.md", Remedy: "write", Writable: true, Blocking: true},
		},
	}
	removeCollidingActions(&report)
	if len(report.Actions) != 1 || report.Actions[0].Scope != "c.md" {
		t.Fatalf("actions = %#v", report.Actions)
	}
	if !hasFinding(report, "reconcile-action-collision", "source/shared#document") {
		t.Fatalf("findings = %#v", report.Findings)
	}
	for _, finding := range report.Findings {
		switch finding.Scope {
		case "a.md", "b.md":
			if finding.Writable || finding.Remedy != "resolve the derived address collision before writing" {
				t.Errorf("colliding finding = %#v", finding)
			}
		case "c.md":
			if !finding.Writable || finding.Remedy != "write" {
				t.Errorf("unrelated finding changed = %#v", finding)
			}
		}
	}
}

func TestAnalyzeSources_BindingsAndRetirement(t *testing.T) {
	body := []byte("# Source\n")
	file := File{Path: "docs/wiki/widget.md", Mode: "100644", SHA256: sourceDigest(body), Body: body}
	tests := []struct {
		name       string
		records    func() []reconcileRecord
		snapshot   Snapshot
		sealed     map[string]map[string]bool
		wantCode   string
		wantScope  string
		wantAction bool
	}{
		{name: "declared source absent from inconsistent snapshot", snapshot: Snapshot{Files: map[string]File{}, AuthoredSources: []string{"docs/missing.md"}}},
		{name: "missing", snapshot: Snapshot{Commit: "commit", Files: map[string]File{file.Path: file}, AuthoredSources: []string{file.Path}}, wantCode: "missing-document", wantScope: file.Path, wantAction: true},
		{name: "duplicate exact", records: func() []reconcileRecord {
			return []reconcileRecord{{record: fixtureDocument("wiki/widget", "entry", file.Path, body, 10), body: body}, {record: fixtureDocument("source/widget-copy", "document", file.Path, body, 11), body: body}}
		}, snapshot: Snapshot{Commit: "commit", Files: map[string]File{file.Path: file}, AuthoredSources: []string{file.Path}}, wantCode: "duplicate-source-binding", wantScope: file.Path},
		{name: "duplicate stale is not writable", records: func() []reconcileRecord {
			oldA := []byte("old a\n")
			oldB := []byte("old b\n")
			return []reconcileRecord{{record: fixtureDocument("wiki/widget", "entry", file.Path, oldA, 10), body: oldA}, {record: fixtureDocument("source/widget-copy", "document", file.Path, oldB, 11), body: oldB}}
		}, snapshot: Snapshot{Commit: "commit", Files: map[string]File{file.Path: file}, AuthoredSources: []string{file.Path}}, wantCode: "duplicate-source-binding", wantScope: file.Path},
		{name: "native mode conflict", records: func() []reconcileRecord {
			r := fixtureDocument("wiki/widget", "entry", file.Path, body, 12)
			r.LiteralBody = false
			return []reconcileRecord{{record: r, body: body}}
		}, snapshot: Snapshot{Commit: "commit", Files: map[string]File{file.Path: file}, AuthoredSources: []string{file.Path}}, wantCode: "source-mode-conflict", wantScope: file.Path},
		{name: "derived address occupied", records: func() []reconcileRecord {
			otherBody := []byte("other\n")
			return []reconcileRecord{{record: fixtureDocument("wiki/widget", "entry", "docs/wiki/other.md", otherBody, 13), body: otherBody}}
		}, snapshot: Snapshot{Commit: "commit", Files: map[string]File{file.Path: file}, AuthoredSources: []string{file.Path}}, wantCode: "source-address-collision", wantScope: file.Path},
		{name: "same provenance non document occupies address", records: func() []reconcileRecord {
			claim := fixtureRecord(record.Promise, "wiki/widget", "entry", 13)
			claim.Provenance.Source = file.Path
			return []reconcileRecord{{record: claim}}
		}, snapshot: Snapshot{Commit: "commit", Files: map[string]File{file.Path: file}, AuthoredSources: []string{file.Path}}, wantCode: "source-address-collision", wantScope: file.Path},
		{name: "active orphan", records: func() []reconcileRecord {
			return []reconcileRecord{{record: fixtureDocument("wiki/widget", "entry", file.Path, body, 14), body: body}}
		}, snapshot: Snapshot{Files: map[string]File{}}, wantCode: "orphan-document-source", wantScope: file.Path},
		{name: "sealed active orphan", records: func() []reconcileRecord {
			return []reconcileRecord{{record: fixtureDocument("wiki/widget", "entry", file.Path, body, 15), body: body}}
		}, snapshot: Snapshot{Files: map[string]File{}}, sealed: map[string]map[string]bool{file.Path: {fixtureDocument("wiki/widget", "entry", file.Path, body, 15).ID: true}}},
		{name: "historical orphan", records: func() []reconcileRecord {
			old := fixtureDocument("wiki/widget", "entry", file.Path, body, 16)
			newBody := []byte("new\n")
			newPath := "docs/wiki/current.md"
			current := fixtureDocument("wiki/widget", "entry", newPath, newBody, 17)
			current.Relations = []record.Relation{{Type: record.Supersedes, Target: old.ID}}
			return []reconcileRecord{{record: old, body: body}, {record: current, body: newBody}}
		}, snapshot: func() Snapshot {
			newBody := []byte("new\n")
			newPath := "docs/wiki/current.md"
			return Snapshot{Files: map[string]File{newPath: {Path: newPath, Mode: "100644", SHA256: sourceDigest(newBody), Body: newBody}}}
		}(), wantCode: "orphan-document-source", wantScope: file.Path},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []reconcileRecord
			if tt.records != nil {
				entries = tt.records()
			}
			set, proj := loadReconcileRecords(t, entries...)
			var report Report
			analyzeSources(&report, tt.snapshot, set, proj, Options{}, tt.sealed)
			if tt.wantCode == "" {
				if len(report.Findings) != 0 || len(report.Actions) != 0 {
					t.Fatalf("unexpected result = %#v", report)
				}
				return
			}
			if !hasFinding(report, tt.wantCode, tt.wantScope) {
				t.Fatalf("missing %s/%s in %#v", tt.wantCode, tt.wantScope, report.Findings)
			}
			if got := len(report.Actions) > 0; got != tt.wantAction {
				t.Fatalf("has action = %v, want %v: %#v", got, tt.wantAction, report.Actions)
			}
		})
	}
}

func TestAnalyzeSources_SealedRetirementRejectsSecondActiveBinding(t *testing.T) {
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	seal, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout: %v", err)
	}
	f.addDraft(t, seal, 90)

	body := append([]byte(nil), f.snap.Files[f.prdPath].Body...)
	rogue := fixtureDocument("source/rogue", "document", f.prdPath, body, 91)
	f.replaceRecord(t, rogue, body)
	set, proj = f.load(t)

	retired := f.snap
	retired.Files = cloneFiles(f.snap.Files)
	delete(retired.Files, f.prdPath)
	retired.AuthoredSources = []string{f.designPath}
	report := Analyze(retired, set, proj, Options{})
	if !hasFinding(report, "orphan-document-source", f.prdPath) {
		t.Fatalf("sealed source admitted a second active binding: %#v", report.Findings)
	}
	for _, finding := range report.Findings {
		if finding.Code != "orphan-document-source" || finding.Scope != f.prdPath {
			continue
		}
		if !strings.Contains(finding.Detail, rogue.ID) || strings.Contains(finding.Detail, f.prdDoc.ID) {
			t.Fatalf("orphan finding did not isolate the unauthorized binding: %#v", finding)
		}
	}
}

func TestIsSemanticCodeMemory(t *testing.T) {
	tests := []struct {
		name   string
		record *record.Record
		want   bool
	}{
		{name: "need", record: fixtureRecord(record.Need, "prd/widget", "G-1", 60), want: true},
		{name: "promise", record: fixtureRecord(record.Promise, "prd/widget", "FR-1", 61), want: true},
		{name: "expectation", record: fixtureRecord(record.Expectation, "prd/widget", "DOD-1", 62), want: true},
		{name: "decision", record: fixtureRecord(record.Decision, "design/widget", "architecture", 63), want: true},
		{name: "policy", record: fixtureRecord(record.Policy, "policy/widget", "rule", 64), want: true},
		{name: "learning", record: fixtureRecord(record.Learning, "learning/widget", "lesson", 65), want: true},
		{name: "native document", record: fixtureRecord(record.Document, "wiki/widget", "entry", 66), want: true},
		{name: "literal source document", record: func() *record.Record {
			r := fixtureRecord(record.Document, "wiki/source", "entry", 72)
			r.LiteralBody = true
			return r
		}()},
		{name: "code anchor", record: fixtureRecord(record.Need, "code/widget", "anchor", 67)},
		{name: "tagged anchor", record: func() *record.Record {
			r := fixtureRecord(record.Need, "prd/widget", "G-2", 68)
			r.Tags = []string{"reconcile:anchor"}
			return r
		}()},
		{name: "plan", record: fixtureRecord(record.Plan, "plan/widget", "", 69)},
		{name: "closeout", record: fixtureRecord(record.Document, "closeout/widget", "seal", 70)},
		{name: "assumption", record: fixtureRecord(record.Assumption, "prd/widget", "assumption", 71)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSemanticCodeMemory(tt.record); got != tt.want {
				t.Fatalf("isSemanticCodeMemory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeCode_LiteralSourceDocumentDoesNotSatisfySemanticCoverage(t *testing.T) {
	unit := codeinventory.Unit{
		Kind:   codeinventory.GoPackage,
		Path:   "internal/widget",
		Files:  []string{"internal/widget/widget.go"},
		SHA256: strings.Repeat("a", 64),
	}
	anchor := fixtureRecord(record.Need, codeAnchorSubject(unit.Path), "anchor", 73)
	anchor.Tags = []string{"code:" + unit.Path, "reconcile:anchor"}
	anchor.Provenance = record.Provenance{
		Source: "code:" + unit.Path,
		Note:   "offline code inventory " + unit.SHA256,
	}
	literalBody := []byte("# Captured source\n")
	literal := fixtureDocument("wiki/widget", "entry", "docs/wiki/widget.md", literalBody, 74)
	literal.Tags = append(literal.Tags, "code:"+unit.Path)
	set, proj := loadReconcileRecords(t,
		reconcileRecord{record: anchor},
		reconcileRecord{record: literal, body: literalBody},
	)

	var report Report
	analyzeCode(&report, Snapshot{Code: []codeinventory.Unit{unit}}, set, proj)
	if !hasFinding(report, "missing-code-memory", unit.Path) {
		t.Fatalf("literal source document falsely satisfied semantic coverage: %#v", report.Findings)
	}
	if hasFinding(report, "missing-code-anchor", unit.Path) {
		t.Fatalf("fixture anchor was not recognized: %#v", report.Findings)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("literal-document regression should isolate one gap: %#v", report.Findings)
	}
}

func TestSourceBindingPredicates(t *testing.T) {
	body := []byte("source\n")
	f := File{Path: "docs/source.md", SHA256: sourceDigest(body), Body: body}
	exact := fixtureDocument("source/example", "document", f.Path, body, 20)
	tests := []struct {
		name string
		rec  *record.Record
		body []byte
		want bool
	}{
		{name: "exact", rec: exact, body: body, want: true},
		{name: "nil"},
		{name: "wrong kind", rec: fixtureRecord(record.Promise, "prd/widget", "FR-1", 21), body: body},
		{name: "native", rec: func() *record.Record { r := *exact; r.LiteralBody = false; return &r }(), body: body},
		{name: "body differs", rec: exact, body: []byte("different\n")},
		{name: "envelope hash differs", rec: func() *record.Record { r := *exact; r.BodySHA256 = strings.Repeat("0", 64); return &r }(), body: body},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := documentMatchesSource(tt.rec, tt.body, f); got != tt.want {
				t.Fatalf("documentMatchesSource() = %v, want %v", got, tt.want)
			}
		})
	}

	for _, tt := range []struct {
		path string
		want bool
	}{{"docs/a.md", true}, {"", false}, {".", false}, {"../a", false}, {"docs/../a", false}, {"/abs", false}, {"C:\\a", false}, {"git:main:a", false}} {
		if got := fileProvenance(tt.path); got != tt.want {
			t.Errorf("fileProvenance(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestAnalyzePRDs_DispositionsAndActions(t *testing.T) {
	base := PRDUnit{ID: "FR-1", Prefix: "FR", Status: "live", Kind: record.Promise, Statement: "Provide widgets.", Priority: "Must", SourceSHA256: strings.Repeat("1", 64), Mapped: true}
	tests := []struct {
		name       string
		unit       PRDUnit
		records    func() []reconcileRecord
		wantCode   string
		wantAction bool
		check      func(*testing.T, Report)
	}{
		{name: "missing mapped unit", unit: base, wantCode: "missing-prd-memory", wantAction: true},
		{name: "retired source with active head", unit: func() PRDUnit { u := base; u.Status = "retired"; return u }(), records: func() []reconcileRecord { return []reconcileRecord{{record: prdMemoryRecord(base, 30)}} }, wantCode: "stale-prd-memory"},
		{name: "open question", unit: PRDUnit{ID: "Q-1", Prefix: "Q", Status: "live", Statement: "Which mechanism?", Intent: "design", Rationale: "open", FitCriterion: "design gate", SourceSHA256: strings.Repeat("2", 64)}, wantCode: "open-prd-question", wantAction: true, check: func(t *testing.T, report Report) {
			if len(report.Actions) != 1 || report.Actions[0].Code != "add-prd-question" || report.Actions[0].Draft.Kind != record.Question {
				t.Fatalf("question action = %#v", report.Actions)
			}
		}},
		{name: "unmapped unknown", unit: PRDUnit{ID: "X-1", Prefix: "X", Status: "live"}, wantCode: "unmapped-prd-unit"},
		{name: "narrative context ignored", unit: PRDUnit{ID: "NG-1", Prefix: "NG", Status: "live"}},
		{name: "rewrite context ignored", unit: PRDUnit{ID: "WR-1", Prefix: "WR", Status: "live"}},
		{name: "semantic drift", unit: func() PRDUnit { u := base; u.Statement = "Changed"; return u }(), records: func() []reconcileRecord { return []reconcileRecord{{record: prdMemoryRecord(base, 31)}} }, wantCode: "prd-memory-drift"},
		{name: "exact but unserved", unit: base, records: func() []reconcileRecord { return []reconcileRecord{{record: prdMemoryRecord(base, 32)}} }, wantCode: "unserved-prd-memory"},
		{name: "exact non-Must claim needs no design edge", unit: func() PRDUnit { u := base; u.Priority = "Should"; return u }(), records: func() []reconcileRecord { return []reconcileRecord{{record: prdMemoryRecord(base, 35)}} }},
		{name: "successor relation", unit: func() PRDUnit { u := base; old := "FR-0"; u.Supersedes = &old; return u }(), records: func() []reconcileRecord {
			old := base
			old.ID = "FR-0"
			return []reconcileRecord{{record: prdMemoryRecord(old, 33)}}
		}, wantCode: "missing-prd-memory", wantAction: true, check: func(t *testing.T, report Report) {
			if len(report.Actions) != 1 || len(report.Actions[0].Draft.Relations) != 1 || report.Actions[0].Draft.Relations[0].Type != record.Supersedes {
				t.Fatalf("successor action = %#v", report.Actions)
			}
		}},
		{name: "successor kind conflict", unit: func() PRDUnit { u := base; old := "G-1"; u.Supersedes = &old; return u }(), records: func() []reconcileRecord {
			old := base
			old.ID, old.Kind = "G-1", record.Need
			return []reconcileRecord{{record: prdMemoryRecord(old, 34)}}
		}, wantCode: "prd-lineage-kind-conflict"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []reconcileRecord
			if tt.records != nil {
				entries = tt.records()
			}
			set, proj := loadReconcileRecords(t, entries...)
			var report Report
			analyzePRDs(&report, Snapshot{Commit: "commit", PRDs: []PRD{{Feature: "widget", Source: "docs/prds/widget/effective.md", Fingerprint: "sha256:test", Units: []PRDUnit{tt.unit}}}}, set, proj)
			if tt.wantCode == "" {
				if len(report.Findings) != 0 || len(report.Actions) != 0 {
					t.Fatalf("unexpected result = %#v", report)
				}
			} else if !hasFinding(report, tt.wantCode, "prd/widget#"+tt.unit.ID) {
				t.Fatalf("missing %s in %#v", tt.wantCode, report.Findings)
			}
			if got := len(report.Actions) > 0; got != tt.wantAction {
				t.Fatalf("has action = %v, want %v: %#v", got, tt.wantAction, report.Actions)
			}
			if tt.check != nil {
				tt.check(t, report)
			}
		})
	}
}

func TestAnalyzePRDs_ExplicitNativeDispositions(t *testing.T) {
	prd := PRD{Feature: "widget", Source: "docs/prds/widget/effective.md", Fingerprint: "sha256:test"}
	question := PRDUnit{ID: "Q-1", Prefix: "Q", Status: "live", Statement: "Which mechanism?", Intent: "design", Rationale: "open", FitCriterion: "design gate", SourceSHA256: strings.Repeat("1", 64)}
	risk := PRDUnit{ID: "R-1", Prefix: "R", Status: "live", SourceSHA256: strings.Repeat("2", 64)}

	tests := []struct {
		name     string
		unit     PRDUnit
		records  func() []*record.Record
		wantCode string
	}{
		{name: "answered question is complete", unit: question, records: func() []*record.Record {
			q := fixturePRDDisposition(record.Question, prd, question, 70)
			return []*record.Record{q, fixturePRDAnswer(q, "prd/widget", 71)}
		}},
		{name: "unanswered exact question remains open", unit: question, wantCode: "open-prd-question", records: func() []*record.Record {
			return []*record.Record{fixturePRDDisposition(record.Question, prd, question, 72)}
		}},
		{name: "duplicate answers invalidate disposition", unit: question, wantCode: "invalid-prd-disposition", records: func() []*record.Record {
			q := fixturePRDDisposition(record.Question, prd, question, 73)
			return []*record.Record{q, fixturePRDAnswer(q, "prd/widget", 74), fixturePRDAnswer(q, "prd/widget", 75)}
		}},
		{name: "accepted risk is complete", unit: risk, records: func() []*record.Record {
			return []*record.Record{fixturePRDDisposition(record.Decision, prd, risk, 76)}
		}},
		{name: "stale risk disposition is invalid", unit: risk, wantCode: "invalid-prd-disposition", records: func() []*record.Record {
			r := fixturePRDDisposition(record.Decision, prd, risk, 77)
			r.Tags[1] = "source-unit:" + strings.Repeat("9", 64)
			return []*record.Record{r}
		}},
		{name: "retired question with active disposition is stale", unit: func() PRDUnit {
			u := question
			u.Status = "superseded"
			return u
		}(), wantCode: "stale-prd-memory", records: func() []*record.Record {
			return []*record.Record{fixturePRDDisposition(record.Question, prd, question, 78)}
		}},
		{name: "live question missing source digest is invalid", unit: PRDUnit{ID: "Q-1", Prefix: "Q", Status: "live"}, wantCode: "invalid-prd-disposition"},
		{name: "live risk with invalid source digest is invalid", unit: PRDUnit{ID: "R-1", Prefix: "R", Status: "live", SourceSHA256: "not-a-digest"}, wantCode: "invalid-prd-disposition"},
		{name: "risk without a disposition remains unmapped", unit: risk, wantCode: "unmapped-prd-unit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []reconcileRecord
			if tt.records != nil {
				for _, r := range tt.records() {
					entries = append(entries, reconcileRecord{record: r})
				}
			}
			set, proj := loadReconcileRecords(t, entries...)
			var report Report
			current := prd
			current.Units = []PRDUnit{tt.unit}
			analyzePRDs(&report, Snapshot{Commit: strings.Repeat("a", 40), PRDs: []PRD{current}}, set, proj)
			scope := "prd/widget#" + tt.unit.ID
			if tt.wantCode == "" {
				if len(report.Findings) != 0 || len(report.Actions) != 0 {
					t.Fatalf("unexpected disposition report = %#v", report)
				}
				return
			}
			if !hasFinding(report, tt.wantCode, scope) {
				t.Fatalf("missing %s/%s in %#v", tt.wantCode, scope, report.Findings)
			}
			if len(report.Actions) != 0 {
				t.Fatalf("native disposition must not produce automatic writes: %#v", report.Actions)
			}
		})
	}
}

func TestAnalyzePRDs_OnlyAuthorizedLiveDesignEdgesCountAsServed(t *testing.T) {
	unit := PRDUnit{
		ID: "FR-1", Prefix: "FR", Status: "live", Kind: record.Promise,
		Statement: "Provide widgets.", Priority: "Must", SourceSHA256: strings.Repeat("1", 64), Mapped: true,
	}
	prd := PRD{
		Feature: "widget", Source: "docs/prds/widget/effective.md",
		Fingerprint: "sha256:test", Units: []PRDUnit{unit},
	}
	tests := []struct {
		name        string
		subject     string
		designs     []Design
		wantInvalid bool
	}{
		{
			name:        "decision outside design namespace",
			subject:     "ops/widget",
			wantInvalid: true,
		},
		{
			name:        "memory edge absent from indexed source design",
			subject:     "design/widget-v1",
			designs:     []Design{{Slug: "widget-v1", Status: "live"}},
			wantInvalid: true,
		},
		{
			name:    "exact live indexed source edge",
			subject: "design/widget-v1",
			designs: []Design{{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-1"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head := prdMemoryRecord(unit, 80)
			decision := fixtureRecord(record.Decision, tt.subject, "architecture", 81)
			decision.Relations = []record.Relation{{Type: record.Serves, Target: head.ID}}
			set, proj := loadReconcileRecords(t,
				reconcileRecord{record: head},
				reconcileRecord{record: decision},
			)
			var report Report
			analyzePRDs(&report, Snapshot{PRDs: []PRD{prd}, Designs: tt.designs}, set, proj)
			invalidScope := decision.ID + " -> " + head.ID
			if tt.wantInvalid {
				if !hasFinding(report, "unauthorized-prd-serves", invalidScope) || !hasFinding(report, "unserved-prd-memory", "prd/widget#FR-1") {
					t.Fatalf("unauthorized edge was allowed to serve the PRD: %#v", report.Findings)
				}
				return
			}
			if hasFinding(report, "unauthorized-prd-serves", invalidScope) || hasFinding(report, "unserved-prd-memory", "prd/widget#FR-1") {
				t.Fatalf("valid indexed edge was rejected: %#v", report.Findings)
			}
		})
	}
}

func TestAnalyzePRDsRejectsDurableHeadForLiveContextRow(t *testing.T) {
	unit := PRDUnit{ID: "WR-1", Prefix: "WR", Status: "live"}
	head := fixtureRecord(record.Promise, "prd/docgate", unit.ID, 79)
	set, proj := loadReconcileRecords(t, reconcileRecord{record: head})
	var report Report
	analyzePRDs(&report, Snapshot{PRDs: []PRD{{Feature: "docgate", Units: []PRDUnit{unit}}}}, set, proj)
	if !hasFinding(report, "unexpected-prd-context-memory", "prd/docgate#WR-1") {
		t.Fatalf("invented WR truth was accepted: %#v", report.Findings)
	}
}

func TestAnalyzePRDsRejectsServesEdgeToInventedPRDFamily(t *testing.T) {
	target := fixtureRecord(record.Promise, "prd/widget", "FR-999", 79)
	decision := fixtureRecord(record.Decision, "design/widget-v1", "architecture", 80)
	decision.Relations = []record.Relation{{Type: record.Serves, Target: target.ID}}
	set, proj := loadReconcileRecords(t, reconcileRecord{record: target}, reconcileRecord{record: decision})
	var report Report
	analyzePRDs(&report, Snapshot{PRDFeatures: []string{"widget"}, Designs: []Design{{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-999"}}}}, set, proj)
	if !hasFinding(report, "unauthorized-prd-serves", decision.ID+" -> "+target.ID) {
		t.Fatalf("invented PRD target was accepted: %#v", report.Findings)
	}
}

func TestNativeRequirementServesKeepsLedgerAuthority(t *testing.T) {
	claim := fixtureRecord(record.Promise, "prd/widget", "FR-1", 90)
	decision := fixtureRecord(record.Decision, "design/widget", "architecture", 91)
	decision.Relations = []record.Relation{{Type: record.Serves, Target: claim.ID}}
	set, proj := loadReconcileRecords(t, reconcileRecord{record: claim}, reconcileRecord{record: decision})
	scope := decision.ID + " -> " + claim.ID
	for _, tt := range []struct {
		name     string
		snapshot Snapshot
		opts     Options
		refused  bool
	}{
		{name: "native", snapshot: Snapshot{}},
		{name: "candidate ledger", snapshot: Snapshot{PRDFeatures: []string{"widget"}}, refused: true},
		{name: "trusted ledger", snapshot: Snapshot{TrustedPRDFeatures: map[string]bool{"widget": true}}, refused: true},
		{name: "historical evaluator", opts: Options{LegacyNativeServes: true, LegacyNativeEvaluator: true}, refused: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report := Analyze(tt.snapshot, set, proj, tt.opts)
			if got := hasFinding(report, "unauthorized-prd-serves", scope); got != tt.refused {
				t.Fatalf("unauthorized edge = %t, want %t: %#v", got, tt.refused, report.Findings)
			}
			if !tt.refused && report.Blocking() {
				t.Fatalf("native claim was blocked: %#v", report.Findings)
			}
			if tt.name == "historical evaluator" && report.Generator != "memory-reconcile/v5" {
				t.Fatalf("historical generator = %q", report.Generator)
			}
		})
	}
}

func TestAnalyzeDesigns(t *testing.T) {
	targetUnit := PRDUnit{ID: "FR-1", Prefix: "FR", Kind: record.Promise, Statement: "Provide widgets.", SourceSHA256: strings.Repeat("1", 64), Mapped: true}
	target := prdMemoryRecord(targetUnit, 40)
	tests := []struct {
		name      string
		design    Design
		records   func() []reconcileRecord
		wantCode  string
		wantScope string
	}{
		{name: "live design missing decisions and target", design: Design{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-1"}}, wantCode: "missing-design-memory", wantScope: "design/widget-v1"},
		{name: "missing target", design: Design{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-1"}}, records: func() []reconcileRecord {
			return []reconcileRecord{{record: fixtureRecord(record.Decision, "design/widget-v1", "architecture", 41)}}
		}, wantCode: "missing-design-target-memory", wantScope: "design/widget-v1 -> prd:widget/FR-1"},
		{name: "missing serves relation", design: Design{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-1"}}, records: func() []reconcileRecord {
			return []reconcileRecord{{record: target}, {record: fixtureRecord(record.Decision, "design/widget-v1", "architecture", 42)}}
		}, wantCode: "missing-serves-memory", wantScope: "design/widget-v1 -> prd:widget/FR-1"},
		{name: "linked", design: Design{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-1", "external:ignored"}}, records: func() []reconcileRecord {
			d := fixtureRecord(record.Decision, "design/widget-v1", "architecture", 43)
			d.Relations = []record.Relation{{Type: record.Serves, Target: target.ID}}
			return []reconcileRecord{{record: target}, {record: d}}
		}},
		{name: "superseded with active decision", design: Design{Slug: "widget-v1", Status: "superseded", CurrentSuccessor: "widget-v2", Serves: []string{"prd:widget/FR-1"}}, records: func() []reconcileRecord {
			return []reconcileRecord{{record: fixtureRecord(record.Decision, "design/widget-v1", "architecture", 44)}}
		}, wantCode: "superseded-design-memory", wantScope: "design/widget-v1"},
		{name: "superseded without decision", design: Design{Slug: "widget-v1", Status: "superseded", CurrentSuccessor: "widget-v2", Serves: []string{"prd:widget/FR-1"}}},
		{name: "non serving live design", design: Design{Slug: "widget-v1", Status: "live"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []reconcileRecord
			if tt.records != nil {
				entries = tt.records()
			}
			set, proj := loadReconcileRecords(t, entries...)
			var report Report
			analyzeDesigns(&report, Snapshot{Designs: []Design{tt.design}}, set, proj)
			if tt.wantCode == "" {
				if len(report.Findings) != 0 {
					t.Fatalf("unexpected findings = %#v", report.Findings)
				}
			} else if !hasFinding(report, tt.wantCode, tt.wantScope) {
				t.Fatalf("missing %s/%s in %#v", tt.wantCode, tt.wantScope, report.Findings)
			}
		})
	}
}

func TestAnalyzeDesignsIgnoresExactLiveDocgateWorkflowContext(t *testing.T) {
	context := PRDUnit{ID: "WR-1", Prefix: "WR", Status: "live"}
	design := Design{Slug: "docgate-v2", Status: "live", Serves: []string{"prd:docgate/WR-1"}}
	decision := fixtureRecord(record.Decision, "design/docgate-v2", "architecture", 48)
	set, proj := loadReconcileRecords(t, reconcileRecord{record: decision})
	var report Report
	analyzeDesigns(&report, Snapshot{
		PRDs:    []PRD{{Feature: "docgate", Units: []PRDUnit{context}}},
		Designs: []Design{design},
	}, set, proj)
	if len(report.Findings) != 0 {
		t.Fatalf("live WR context required native memory: %#v", report.Findings)
	}

	// The exemption is exact and live. A retired target is stale design
	// metadata and must still remain visible as missing target debt.
	context.Status = "retired"
	report = Report{}
	analyzeDesigns(&report, Snapshot{
		PRDs:    []PRD{{Feature: "docgate", Units: []PRDUnit{context}}},
		Designs: []Design{design},
	}, set, proj)
	if !hasFinding(report, "missing-design-target-memory", "design/docgate-v2 -> prd:docgate/WR-1") {
		t.Fatalf("retired WR reference was silently ignored: %#v", report.Findings)
	}
}

func TestAnalyzeDesigns_DistinguishesSharedMissingTargetByDesign(t *testing.T) {
	first := fixtureRecord(record.Decision, "design/widget-v1", "architecture", 60)
	second := fixtureRecord(record.Decision, "design/widget-v2", "architecture", 61)
	set, proj := loadReconcileRecords(t, reconcileRecord{record: first}, reconcileRecord{record: second})
	snapshot := Snapshot{Designs: []Design{
		{Slug: "widget-v1", Status: "live", Serves: []string{"prd:widget/FR-1"}},
		{Slug: "widget-v2", Status: "live", Serves: []string{"prd:widget/FR-1"}},
	}}
	var report Report
	analyzeDesigns(&report, snapshot, set, proj)
	for _, scope := range []string{
		"design/widget-v1 -> prd:widget/FR-1",
		"design/widget-v2 -> prd:widget/FR-1",
	} {
		if !hasFinding(report, "missing-design-target-memory", scope) {
			t.Errorf("missing design-scoped target finding %q in %#v", scope, report.Findings)
		}
	}
}

func TestDesignReferenceHelpers(t *testing.T) {
	d := Design{Serves: []string{"prd:zeta/FR-1", "prd:alpha/NFR-1", "prd:zeta/FR-2", "external:x", "prd:/bad", "prd:missing-family"}}
	if got, want := designFeatures(d), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("designFeatures() = %#v, want %#v", got, want)
	}
	for _, tt := range []struct {
		ref             string
		subject, family string
		ok              bool
	}{
		{"prd:widget/FR-1", "prd/widget", "FR-1", true},
		{"design:widget", "", "", false},
		{"prd:/FR-1", "", "", false},
		{"prd:widget/", "", "", false},
		{"prd:widget", "", "", false},
	} {
		subject, family, ok := memoryPRDRef(tt.ref)
		if subject != tt.subject || family != tt.family || ok != tt.ok {
			t.Errorf("memoryPRDRef(%q) = %q, %q, %v", tt.ref, subject, family, ok)
		}
	}
}

type reconcileRecord struct {
	record *record.Record
	body   []byte
}

func loadReconcileRecords(t *testing.T, entries ...reconcileRecord) (*store.Set, *resolve.Projection) {
	t.Helper()
	files := store.MapSource{}
	for _, entry := range entries {
		encoded, err := record.Marshal(entry.record)
		if err != nil {
			t.Fatalf("marshal %s: %v", entry.record.ID, err)
		}
		files[entry.record.Path()] = encoded
		if entry.record.Kind == record.Document {
			files[entry.record.BodyPath()] = append([]byte(nil), entry.body...)
		}
	}
	set, err := store.Load(context.Background(), files)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Problems) > 0 {
		t.Fatalf("memory problems: %#v", set.Problems)
	}
	return set, resolve.New(set)
}

func prdMemoryRecord(unit PRDUnit, seq byte) *record.Record {
	r := fixtureRecord(unit.Kind, "prd/widget", unit.ID, seq)
	r.Statement = unit.Statement
	r.Intent = unit.Intent
	r.Rationale = unit.Rationale
	r.FitCriterion = unit.FitCriterion
	r.Consequence = unit.Consequence
	r.Tags = []string{"source-unit:" + unit.SourceSHA256}
	return r
}
