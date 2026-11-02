package reconcile

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestReconciliationBaselineRoundTripAndClosedParse(t *testing.T) {
	report := Report{
		Generator: Generator, SourceCommit: strings.Repeat("a", 40), SourceClosureSHA256: strings.Repeat("b", 64),
		CodeInventorySHA256: strings.Repeat("c", 64), DesignIndexSHA256: strings.Repeat("d", 64),
		MemoryClosureSHA256: strings.Repeat("e", 64),
		Findings: []Finding{
			{Code: "same", Scope: "scope", Feature: "widget", Blocking: false, Writable: true},
			{Code: "same", Scope: "scope", Feature: "widget", Blocking: true, Writable: false},
			{Code: "alpha", Scope: "other", Blocking: true},
		},
	}
	baseline := NewBaseline(report)
	if baseline.Schema != BaselineSchema || baseline.FindingCount != 3 || len(baseline.Allowances) != 2 {
		t.Fatalf("NewBaseline() = %#v", baseline)
	}
	if got := baseline.Allowances[1]; got.Code != "same" || got.Count != 2 || !got.Blocking || got.Writable {
		t.Fatalf("duplicate allowance envelope = %#v", got)
	}
	body, err := MarshalBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseBaseline(body)
	if err != nil || !reflect.DeepEqual(parsed, baseline) {
		t.Fatalf("ParseBaseline(NewBaseline()) = %#v, %v", parsed, err)
	}
	reencoded, err := MarshalBaseline(parsed)
	if err != nil || string(reencoded) != string(body) {
		t.Fatalf("canonical baseline encoding changed: %v", err)
	}
	if _, err := MarshalBaseline(Baseline{}); err == nil {
		t.Fatal("MarshalBaseline accepted an invalid baseline")
	}

	valid := Baseline{
		Schema: BaselineSchema, Generator: Generator,
		SourceClosureSHA256: strings.Repeat("b", 64), CodeInventorySHA256: strings.Repeat("c", 64),
		MemoryClosureSHA256: strings.Repeat("e", 64), FindingCount: 2,
		Allowances: []BaselineAllowance{
			{Code: "a", Scope: "a", Count: 1},
			{Code: "b", Scope: "b", Count: 1},
		},
	}
	encode := func(v any) []byte {
		t.Helper()
		body, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	mutate := func(change func(*Baseline)) Baseline {
		b := valid
		b.Allowances = append([]BaselineAllowance(nil), valid.Allowances...)
		change(&b)
		return b
	}
	for _, tc := range []struct {
		name string
		body []byte
		want string
	}{
		{name: "not json", body: []byte("{"), want: "decode reconciliation baseline"},
		{name: "unknown field", body: []byte(`{"schema_version":"memory-reconciliation-baseline/v1","generator":"memory-reconcile/v1","source_closure_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","code_inventory_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","memory_closure_sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","finding_count":0,"allowances":[],"surprise":true}`), want: "unknown field"},
		{name: "case-aliased root field", body: []byte(`{"schema_version":"memory-reconciliation-baseline/v1","Generator":"memory-reconcile/v1","source_closure_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","code_inventory_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","memory_closure_sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","finding_count":0,"allowances":[]}`), want: "unknown field \"Generator\""},
		{name: "case-aliased nested field", body: []byte(`{"schema_version":"memory-reconciliation-baseline/v1","generator":"memory-reconcile/v1","source_closure_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","code_inventory_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","memory_closure_sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","finding_count":1,"allowances":[{"code":"a","scope":"a","count":1,"Blocking":true}]}`), want: "unknown field \"Blocking\""},
		{name: "duplicate top-level member", body: []byte(`{"schema_version":"memory-reconciliation-baseline/v1","schema_version":"memory-reconciliation-baseline/v1","generator":"memory-reconcile/v1","source_closure_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","code_inventory_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","memory_closure_sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","finding_count":0,"allowances":[]}`), want: "duplicate object member \"schema_version\""},
		{name: "duplicate nested member", body: []byte(`{"schema_version":"memory-reconciliation-baseline/v1","generator":"memory-reconcile/v1","source_closure_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","code_inventory_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","memory_closure_sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","finding_count":1,"allowances":[{"code":"a","scope":"a","count":1,"blocking":true,"blocking":false}]}`), want: "duplicate object member \"blocking\""},
		{name: "trailing value", body: append(encode(valid), []byte(" {}")...), want: "trailing JSON value"},
		{name: "trailing junk", body: append(encode(valid), []byte(" x")...), want: "invalid character"},
		{name: "wrong schema", body: encode(mutate(func(b *Baseline) { b.Schema = "old" })), want: "schema_version"},
		{name: "missing generator", body: encode(mutate(func(b *Baseline) { b.Generator = "" })), want: "non-empty single-line"},
		{name: "multiline generator", body: encode(mutate(func(b *Baseline) { b.Generator = "old\nnew" })), want: "non-empty single-line"},
		{name: "missing identities", body: encode(mutate(func(b *Baseline) { b.SourceClosureSHA256 = "" })), want: "input identities"},
		{name: "bad inventory digest", body: encode(mutate(func(b *Baseline) { b.CodeInventorySHA256 = strings.Repeat("G", 64) })), want: "input identities"},
		{name: "missing memory digest", body: encode(mutate(func(b *Baseline) { b.MemoryClosureSHA256 = "" })), want: "input identities"},
		{name: "bad memory digest", body: encode(mutate(func(b *Baseline) { b.MemoryClosureSHA256 = strings.Repeat("G", 64) })), want: "input identities"},
		{name: "null allowances", body: []byte(`{"schema_version":"memory-reconciliation-baseline/v1","generator":"memory-reconcile/v1","source_closure_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","code_inventory_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","memory_closure_sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","finding_count":0,"allowances":null}`), want: "must be a JSON array"},
		{name: "negative total", body: encode(mutate(func(b *Baseline) { b.FindingCount = -1 })), want: "must be non-negative"},
		{name: "bad allowance", body: encode(mutate(func(b *Baseline) { b.Allowances[0].Count = 0 })), want: "non-positive count"},
		{name: "NUL in code", body: encode(mutate(func(b *Baseline) { b.Allowances[0].Code = "a\x00b" })), want: "NUL byte"},
		{name: "NUL in scope", body: encode(mutate(func(b *Baseline) { b.Allowances[0].Scope = "a\x00b" })), want: "NUL byte"},
		{name: "NUL in feature", body: encode(mutate(func(b *Baseline) { b.Allowances[0].Feature = "a\x00b" })), want: "NUL byte"},
		{name: "duplicate", body: encode(mutate(func(b *Baseline) { b.Allowances[1] = b.Allowances[0] })), want: "repeats allowance"},
		{name: "unsorted", body: encode(mutate(func(b *Baseline) { b.Allowances[0], b.Allowances[1] = b.Allowances[1], b.Allowances[0] })), want: "canonical order"},
		{name: "wrong total", body: encode(mutate(func(b *Baseline) { b.FindingCount = 3 })), want: "allowances total"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseBaseline(tc.body); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseBaseline() error = %v, want %q", err, tc.want)
			}
		})
	}
	priorGenerator := mutate(func(b *Baseline) { b.Generator = "memory-reconcile/v0" })
	if _, err := ParseBaseline(encode(priorGenerator)); err != nil {
		t.Fatalf("a structurally valid prior generator must remain readable for renewal: %v", err)
	}
}

func TestCompareBaselineMonotonicDebtCeiling(t *testing.T) {
	allow := func(code string, count int, blocking, writable bool) BaselineAllowance {
		return BaselineAllowance{Code: code, Scope: "scope/" + code, Count: count, Blocking: blocking, Writable: writable}
	}
	baseline := Baseline{
		FindingCount: 9,
		Allowances: []BaselineAllowance{
			allow("count-down", 2, true, false),
			allow("count-up", 1, true, false),
			allow("newly-blocking", 1, false, false),
			allow("now-advisory", 1, true, false),
			allow("now-writable", 1, true, false),
			allow("resolved", 1, true, false),
			allow("stable", 1, true, true),
			allow("writability-lost", 1, true, true),
		},
	}
	var findings []Finding
	add := func(code string, count int, blocking, writable bool) {
		for range count {
			findings = append(findings, Finding{Code: code, Scope: "scope/" + code, Blocking: blocking, Writable: writable})
		}
	}
	add("count-down", 1, true, false)
	add("count-up", 2, true, false)
	add("new-address", 1, true, false)
	add("newly-blocking", 1, true, false)
	add("now-advisory", 1, false, false)
	add("now-writable", 1, true, true)
	add("stable", 1, true, true)
	add("writability-lost", 1, true, false)
	result := CompareBaseline(baseline, Report{Findings: findings})
	if result.Schema != BaselineResultSchema || result.BaselineCount != 9 || result.CurrentCount != 9 || !result.Blocking() || result.Exact() {
		t.Fatalf("CompareBaseline() summary = %#v", result)
	}
	wantRegressions := []string{
		"count-up:finding count increased", "new-address:new finding address",
		"newly-blocking:finding became blocking", "writability-lost:finding lost its deterministic repair",
	}
	var gotRegressions []string
	for _, delta := range result.Regressions {
		gotRegressions = append(gotRegressions, delta.Code+":"+delta.Reason)
	}
	if !reflect.DeepEqual(gotRegressions, wantRegressions) {
		t.Fatalf("regressions = %#v, want %#v", gotRegressions, wantRegressions)
	}
	wantImprovements := []string{
		"count-down:finding count decreased", "now-advisory:finding became advisory",
		"now-writable:finding gained a deterministic repair", "resolved:finding resolved",
	}
	var gotImprovements []string
	for _, delta := range result.Improvements {
		gotImprovements = append(gotImprovements, delta.Code+":"+delta.Reason)
	}
	if !reflect.DeepEqual(gotImprovements, wantImprovements) {
		t.Fatalf("improvements = %#v, want %#v", gotImprovements, wantImprovements)
	}

	clean := CompareBaseline(NewBaseline(Report{Generator: Generator, SourceCommit: "c", SourceClosureSHA256: "s", CodeInventorySHA256: "i"}), Report{})
	if clean.Blocking() || clean.Regressions == nil || clean.Improvements == nil {
		t.Fatalf("empty baseline comparison blocked: %#v", clean)
	}
	prior := Baseline{FindingCount: 1, Allowances: []BaselineAllowance{allow("stable", 1, true, true)}}
	if got := CompareBaselines(prior, prior); got.Blocking() || !got.Exact() {
		t.Fatalf("unchanged checked-in baseline blocked: %#v", got)
	}
	wider := Baseline{FindingCount: 2, Allowances: []BaselineAllowance{
		allow("new-address", 1, true, false), allow("stable", 1, true, true),
	}}
	if got := CompareBaselines(prior, wider); !got.Blocking() || len(got.Regressions) != 1 || got.Regressions[0].Reason != "new finding address" {
		t.Fatalf("wider checked-in baseline passed: %#v", got)
	}
	exactReport := Report{
		Generator: Generator, SourceClosureSHA256: strings.Repeat("a", 64),
		CodeInventorySHA256: strings.Repeat("b", 64), DesignIndexSHA256: strings.Repeat("c", 64),
		MemoryClosureSHA256: strings.Repeat("d", 64),
	}
	exactBaseline := NewBaseline(exactReport)
	if !BaselineIdentitiesMatch(exactBaseline, exactReport) || !CompareBaseline(exactBaseline, exactReport).Exact() {
		t.Fatal("a newly generated baseline must exactly bind its report")
	}
	exactReport.MemoryClosureSHA256 = strings.Repeat("e", 64)
	if BaselineIdentitiesMatch(exactBaseline, exactReport) {
		t.Fatal("a changed memory inventory matched a stale baseline")
	}
}

func TestCompareBaselinesAllowsOnlyExactForwardBackfillPhases(t *testing.T) {
	allow := func(code, scope, feature string, count int, writable bool) BaselineAllowance {
		return BaselineAllowance{Code: code, Scope: scope, Feature: feature, Count: count, Blocking: true, Writable: writable}
	}
	for _, tt := range []struct {
		name      string
		prior     []BaselineAllowance
		candidate []BaselineAllowance
		blocked   bool
	}{
		{
			name:      "claim capture reveals unserved state",
			prior:     []BaselineAllowance{allow("missing-prd-memory", "prd/widget#FR-1", "widget", 1, true)},
			candidate: []BaselineAllowance{allow("unserved-prd-memory", "prd/widget#FR-1", "widget", 1, false)},
		},
		{
			name:      "target capture reveals missing edge",
			prior:     []BaselineAllowance{allow("missing-design-target-memory", "design/widget-v1 -> prd:widget/FR-1", "widget", 1, false)},
			candidate: []BaselineAllowance{allow("missing-serves-memory", "design/widget-v1 -> prd:widget/FR-1", "widget", 1, false)},
		},
		{
			name:      "question classification becomes explicit",
			prior:     []BaselineAllowance{allow("unmapped-prd-unit", "prd/widget#Q-1", "widget", 1, false)},
			candidate: []BaselineAllowance{allow("open-prd-question", "prd/widget#Q-1", "widget", 1, true)},
		},
		{
			name:      "reverse transition",
			prior:     []BaselineAllowance{allow("unserved-prd-memory", "prd/widget#FR-1", "widget", 1, false)},
			candidate: []BaselineAllowance{allow("missing-prd-memory", "prd/widget#FR-1", "widget", 1, true)},
			blocked:   true,
		},
		{
			name:      "different scope",
			prior:     []BaselineAllowance{allow("missing-prd-memory", "prd/widget#FR-1", "widget", 1, true)},
			candidate: []BaselineAllowance{allow("unserved-prd-memory", "prd/widget#FR-2", "widget", 1, false)},
			blocked:   true,
		},
		{
			name:      "different feature",
			prior:     []BaselineAllowance{allow("missing-prd-memory", "prd/widget#FR-1", "widget", 1, true)},
			candidate: []BaselineAllowance{allow("unserved-prd-memory", "prd/widget#FR-1", "other", 1, false)},
			blocked:   true,
		},
		{
			name:  "predecessor retained",
			prior: []BaselineAllowance{allow("missing-prd-memory", "prd/widget#FR-1", "widget", 1, true)},
			candidate: []BaselineAllowance{
				allow("missing-prd-memory", "prd/widget#FR-1", "widget", 1, true),
				allow("unserved-prd-memory", "prd/widget#FR-1", "widget", 1, false),
			},
			blocked: true,
		},
		{
			name:      "multiplicity grows",
			prior:     []BaselineAllowance{allow("missing-prd-memory", "prd/widget#FR-1", "widget", 1, true)},
			candidate: []BaselineAllowance{allow("unserved-prd-memory", "prd/widget#FR-1", "widget", 2, false)},
			blocked:   true,
		},
		{
			name:      "unrelated reclassification",
			prior:     []BaselineAllowance{allow("missing-code-anchor", "internal/widget", "", 1, true)},
			candidate: []BaselineAllowance{allow("stale-code-anchor", "internal/widget", "", 1, true)},
			blocked:   true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sort.Slice(tt.prior, func(i, j int) bool { return compareAllowanceAddress(tt.prior[i], tt.prior[j]) < 0 })
			sort.Slice(tt.candidate, func(i, j int) bool { return compareAllowanceAddress(tt.candidate[i], tt.candidate[j]) < 0 })
			prior := Baseline{FindingCount: allowanceTotal(tt.prior), Allowances: tt.prior}
			candidate := Baseline{FindingCount: allowanceTotal(tt.candidate), Allowances: tt.candidate}
			if got := CompareBaselines(prior, candidate).Blocking(); got != tt.blocked {
				t.Fatalf("CompareBaselines().Blocking() = %v, want %v", got, tt.blocked)
			}
		})
	}
}

func allowanceTotal(xs []BaselineAllowance) int {
	total := 0
	for _, x := range xs {
		total += x.Count
	}
	return total
}

func TestMemoryClosureIdentityBindsEveryMemoryFileWithoutFindingChurn(t *testing.T) {
	first := &store.Set{Bytes: map[string][]byte{
		"z.json":      []byte("zeta\n"),
		"policy.json": []byte("policy\n"),
	}}
	reordered := &store.Set{Bytes: map[string][]byte{
		"policy.json": []byte("policy\n"),
		"z.json":      []byte("zeta\n"),
	}}
	if got, want := MemoryClosureSHA256(first), MemoryClosureSHA256(reordered); got != want || !validDigest(got) {
		t.Fatalf("canonical closure identity = %q, reordered = %q", got, want)
	}
	changed := &store.Set{Bytes: map[string][]byte{
		"policy.json": []byte("policy changed\n"),
		"z.json":      []byte("zeta\n"),
	}}
	renamed := &store.Set{Bytes: map[string][]byte{
		"policy.json": []byte("policy\n"),
		"a.json":      []byte("zeta\n"),
	}}
	if MemoryClosureSHA256(first) == MemoryClosureSHA256(changed) || MemoryClosureSHA256(first) == MemoryClosureSHA256(renamed) {
		t.Fatal("memory content and path changes must each change the closure identity")
	}

	snapshot := Snapshot{
		Commit: strings.Repeat("1", 40), SourceClosureSHA256: strings.Repeat("a", 64),
		CodeInventorySHA256: strings.Repeat("b", 64),
	}
	emptySet, emptyProjection := loadReconcileRecords(t)
	before := Analyze(snapshot, emptySet, emptyProjection, Options{})
	learning := fixtureRecord(record.Learning, "learning/widget", "lesson", 90)
	learningSet, learningProjection := loadReconcileRecords(t, reconcileRecord{record: learning})
	afterAdd := Analyze(snapshot, learningSet, learningProjection, Options{})
	if !reflect.DeepEqual(before.Findings, afterAdd.Findings) {
		t.Fatalf("standalone valid Learning changed findings: before %#v, after %#v", before.Findings, afterAdd.Findings)
	}
	if before.MemoryClosureSHA256 == afterAdd.MemoryClosureSHA256 || BaselineIdentitiesMatch(NewBaseline(before), afterAdd) {
		t.Fatal("adding a valid standalone Learning did not stale the baseline identity")
	}
	if BaselineIdentitiesMatch(NewBaseline(afterAdd), before) {
		t.Fatal("deleting a valid standalone Learning did not stale the baseline identity")
	}
}

func TestLegacyBaselineDoesNotHideCodeDriftAfterStructuralAnchorBackfill(t *testing.T) {
	unit := codeinventory.Unit{
		Kind: codeinventory.GoPackage, Path: "internal/widget",
		Files: []string{"internal/widget/widget.go"}, SHA256: strings.Repeat("a", 64),
	}
	emptySet, emptyProjection := loadReconcileRecords(t)
	before := Analyze(Snapshot{Commit: strings.Repeat("1", 40), Code: []codeinventory.Unit{unit}}, emptySet, emptyProjection, Options{LegacyCodeCoverage: true})
	if !hasFinding(before, "missing-code-anchor", unit.Path) {
		t.Fatalf("pre-backfill report lacks structural-anchor debt: %#v", before.Findings)
	}
	baseline := NewBaseline(before)

	draft := codeAnchorDraft(unit, strings.Repeat("1", 40), "")
	anchor := fixtureRecord(draft.Kind, draft.Subject, draft.Family, 54)
	anchor.Statement = draft.Statement
	anchor.Intent = draft.Intent
	anchor.Rationale = draft.Rationale
	anchor.FitCriterion = draft.FitCriterion
	anchor.Tags = append([]string(nil), draft.Tags...)
	anchor.Provenance = draft.Provenance
	set, projection := loadReconcileRecords(t, reconcileRecord{record: anchor})

	changed := unit
	changed.SHA256 = strings.Repeat("b", 64)
	after := Analyze(Snapshot{Commit: strings.Repeat("2", 40), Code: []codeinventory.Unit{changed}}, set, projection, Options{LegacyCodeCoverage: true})
	if !hasFinding(after, "stale-code-anchor", unit.Path) || hasFinding(after, "missing-code-anchor", unit.Path) {
		t.Fatalf("post-backfill code drift was not reclassified: %#v", after.Findings)
	}
	result := CompareBaseline(baseline, after)
	if !result.Blocking() {
		t.Fatalf("old missing-anchor allowance hid new code drift: %#v", result)
	}
	found := false
	for _, regression := range result.Regressions {
		found = found || (regression.Code == "stale-code-anchor" && regression.Scope == unit.Path && regression.Reason == "new finding address")
	}
	if !found {
		t.Fatalf("stale anchor was not a new ratchet key: %#v", result.Regressions)
	}
}

func TestSourceRetirementRegressionsRequirePreservationOrTrustedCloseout(t *testing.T) {
	prior := []string{"docs/colliding.md", "docs/feature.md"}

	regressions := SourceRetirementRegressions(prior, nil, nil)
	if len(regressions) != 2 || regressions[0].Scope != prior[0] || regressions[0].Reason != "authored source disappeared without a closeout proven at the trusted ref" {
		t.Fatalf("unsealed deletion regressions = %#v", regressions)
	}
	if got := SourceRetirementRegressions(prior, prior, nil); len(got) != 0 {
		t.Fatalf("preserved sources were rejected: %#v", got)
	}
	if got := SourceRetirementRegressions(prior, []string{prior[0]}, map[string]bool{prior[1]: true}); len(got) != 0 {
		t.Fatalf("trusted source retirement was rejected: %#v", got)
	}
}
