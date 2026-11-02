package reconcile

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

func TestContractCompletionSeparatesExactUndesignedClaims(t *testing.T) {
	for _, mode := range []string{"ordinary", "requirements-first", "uncaptured", "claim drift", "missing claim"} {
		t.Run(mode, func(t *testing.T) {
			f := newCloseoutFixture(t)
			u := coveragePRDUnit("FR-2", "The system shall expose a second widget.", "2")
			u.Priority = "Must"
			claim := prdMemoryRecord(u, 40)
			f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, u)
			if mode == "claim drift" {
				claim.Statement = "Another claim"
			}
			if mode != "missing claim" {
				f.replaceRecord(t, claim, nil)
			}
			if mode == "uncaptured" {
				delete(f.files, f.prdDoc.Path())
				delete(f.files, f.prdDoc.BodyPath())
			}
			if mode == "requirements-first" {
				f.snap.PRDs[0].RequirementsFirst = true
			}
			set, proj := f.load(t)
			legacy := Analyze(f.snap, set, proj, Options{LegacyContractCoverage: true})
			current := Analyze(f.snap, set, proj, Options{})
			valid := mode == "ordinary" || mode == "requirements-first"
			if current.Generator != Generator || current.Completion == nil || current.Completion.Schema != ContractCompletionSchema {
				t.Fatalf("unversioned completion: %+v", current)
			}
			if valid {
				if !hasFinding(legacy, "unserved-prd-memory", "prd/widget#FR-2") || hasFinding(current, "unserved-prd-memory", "prd/widget#FR-2") || !hasCompletion(current, "undesigned", "prd/widget#FR-2") {
					t.Fatalf("coverage classification: old=%+v new=%+v", legacy, current)
				}
				if _, _, err := BuildCloseout("widget", f.snap, set, proj); err == nil || !strings.Contains(err.Error(), "undesigned prd/widget#FR-2") {
					t.Fatalf("unfinished feature closed: %v", err)
				}
			} else if hasCompletion(current, "undesigned", "prd/widget#FR-2") {
				t.Fatalf("corrupt source/claim became completion: %+v", current)
			}
			if legacy.Generator != "memory-reconcile/v3" || legacy.Completion != nil {
				t.Fatalf("historical classification changed: %+v", legacy)
			}
		})
	}
}

func retiredCompletionFixture(t *testing.T) *closeoutFixture {
	t.Helper()
	f := newCloseoutFixture(t)
	set, proj := f.load(t)
	draft, ok := expectedDesignCoverage(f.snap, f.snap.Designs[0], set, proj)
	if !ok {
		t.Fatal("coverage fixture missing")
	}
	f.addDraft(t, draft, 15)
	baseSet, _ := f.load(t)
	before := f.snap
	before.PRDs = append([]PRD(nil), f.snap.PRDs...)
	before.PRDs[0].Units = append([]PRDUnit(nil), f.snap.PRDs[0].Units...)
	f.snap.ContractHistory = &ContractHistory{Snapshot: before, Set: baseSet}
	f.snap.PRDs[0].Units[0].Status = "retired"
	revocation := fixtureRecord(record.Revocation, "prd/widget", "", 30)
	revocation.Statement = "The approved amendment retires this requirement."
	revocation.Relations = []record.Relation{{Type: record.Revokes, Target: f.promise.ID}}
	f.replaceRecord(t, revocation, nil)
	return f
}

func TestContractCompletionReopensOnlyExactHistoricalDeclarations(t *testing.T) {
	for _, mode := range []string{"valid", "no history", "changed source", "missing source capture", "unknown address", "live missing target", "stale active claim", "backdated new coverage", "wrong coverage statement", "missing native decision"} {
		t.Run(mode, func(t *testing.T) {
			f := retiredCompletionFixture(t)
			scope := "design/widget-v1 -> prd:widget/FR-1"
			switch mode {
			case "no history":
				f.snap.ContractHistory = nil
			case "changed source":
				old := f.snap.ContractHistory.Snapshot.Files[f.designPath]
				old.SHA256 = strings.Repeat("9", 64)
				f.snap.ContractHistory.Snapshot.Files = map[string]File{f.designPath: old}
			case "missing source capture":
				delete(f.files, f.prdDoc.Path())
				delete(f.files, f.prdDoc.BodyPath())
			case "unknown address":
				f.snap.PRDs[0].Units = nil
			case "live missing target":
				f.snap.PRDs[0].Units[0].Status = "live"
			case "stale active claim":
				for name, body := range f.files {
					if strings.Contains(string(body), `"kind": "Revocation"`) {
						delete(f.files, name)
					}
				}
			case "backdated new coverage", "wrong coverage statement":
				set, proj := f.load(t)
				old := proj.Head("design/widget-v1", designCoverageFamily)
				replacement := *old
				replacement.ID = fixtureRecord(record.Decision, old.Subject, old.Family, 16).ID
				replacement.CreatedAt = fixtureRecord(record.Decision, old.Subject, old.Family, 16).CreatedAt
				replacement.Relations = append(append([]record.Relation(nil), old.Relations...), record.Relation{Type: record.Supersedes, Target: old.ID})
				if mode == "wrong coverage statement" {
					replacement.Statement = "Invented header coverage"
				}
				f.replaceRecord(t, &replacement, nil)
				if !historicalContractRecord(f.snap, set.Get(old.ID)) {
					t.Fatal("fixture lost old coverage")
				}
			case "missing native decision":
				delete(f.files, f.sourceDecision.Path())
			}
			set, proj := f.load(t)
			legacy := Analyze(f.snap, set, proj, Options{LegacyContractCoverage: true})
			current := Analyze(f.snap, set, proj, Options{})
			if mode == "valid" {
				if !NeedsContractHistory(f.snap) || !hasCompletion(current, "reopened", scope) || hasFinding(current, "missing-design-target-memory", scope) || hasFinding(current, "design-coverage-memory-drift", "design/widget-v1") {
					t.Fatalf("exact historical declaration blocked: %+v", current)
				}
				for _, finding := range current.Findings {
					if finding.Code == "unauthorized-prd-serves" {
						t.Fatalf("historical serves edge still blocks: %+v", finding)
					}
				}
				if !hasFinding(legacy, "missing-design-target-memory", scope) {
					t.Fatal("v3 replay lost its historical finding")
				}
				if _, _, err := BuildCloseout("widget", f.snap, set, proj); err == nil || !strings.Contains(err.Error(), "reopened") {
					t.Fatalf("reopened closeout = %v", err)
				}
			} else {
				if !current.Blocking() {
					t.Fatalf("invalid evidence admitted: %+v", current)
				}
				if mode == "backdated new coverage" || mode == "wrong coverage statement" {
					if !hasFinding(current, "design-coverage-memory-drift", "design/widget-v1") {
						t.Fatalf("new stale coverage was laundered: %+v", current)
					}
				} else if mode == "missing native decision" {
					if !hasFinding(current, "missing-design-memory", "design/widget-v1") {
						t.Fatal("missing native decision hidden")
					}
				} else if hasCompletion(current, "reopened", scope) {
					t.Fatalf("invalid source/transition became reopened: %+v", current)
				}
			}
		})
	}
}

func TestContractCompletionSupersededNativeTransition(t *testing.T) {
	f := newCloseoutFixture(t)
	baseSet, _ := f.load(t)
	before := f.snap
	before.PRDs = append([]PRD(nil), f.snap.PRDs...)
	before.PRDs[0].Units = append([]PRDUnit(nil), f.snap.PRDs[0].Units...)
	f.snap.ContractHistory = &ContractHistory{Snapshot: before, Set: baseSet}
	next := coveragePRDUnit("FR-2", "The system shall expose the replacement widget.", "2")
	next.Priority = "Must"
	claim := prdMemoryRecord(next, 40)
	claim.Relations = []record.Relation{{Type: record.Supersedes, Target: f.promise.ID}}
	f.replaceRecord(t, claim, nil)
	f.snap.PRDs[0].Units[0].Status = "superseded"
	f.snap.PRDs[0].Units[0].SupersededBy = &next.ID
	f.snap.PRDs[0].Units = append(f.snap.PRDs[0].Units, next)
	set, proj := f.load(t)
	if proj.Status(f.promise.ID) != resolve.Superseded {
		t.Fatal("fixture transition invalid")
	}
	r := Analyze(f.snap, set, proj, Options{})
	if !hasCompletion(r, "reopened", "design/widget-v1 -> prd:widget/FR-1") || !hasCompletion(r, "undesigned", "prd/widget#FR-2") {
		t.Fatalf("superseded transition = %+v", r)
	}
	claim.Relations = nil
	f.replaceRecord(t, claim, nil)
	set, proj = f.load(t)
	r = Analyze(f.snap, set, proj, Options{})
	if hasCompletion(r, "reopened", "design/widget-v1 -> prd:widget/FR-1") || !hasFinding(r, "stale-prd-memory", "prd/widget#FR-1") {
		t.Fatalf("missing native transition admitted: %+v", r)
	}
}

func hasCompletion(r Report, state, scope string) bool {
	if r.Completion == nil {
		return false
	}
	for _, f := range r.Completion.Findings {
		if f.State == state && f.Scope == scope {
			return true
		}
	}
	return false
}
