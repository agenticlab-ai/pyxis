package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFamilyRejectsEveryKindSpecificHole(t *testing.T) {
	catch := &ExpectedCatch{DefectClass: "defect"}
	directive := &DirectiveSurfaced{DefectClass: "directive"}
	outcome := &ExpectedOutcome{ForbiddenVerdicts: []string{"approve"}}
	tests := map[string]*Family{
		"schema":            {ID: "EVAL-X", Kind: KindNitGolden, MaxFindingsP50: 1, Goldens: []Golden{{}}},
		"id":                {Schema: FamilySchema, Kind: KindNitGolden, MaxFindingsP50: 1, Goldens: []Golden{{}}},
		"catch no legs":     {Schema: FamilySchema, ID: "EVAL-X", Kind: KindSeededCatch},
		"catch no band":     {Schema: FamilySchema, ID: "EVAL-X", Kind: KindSeededCatch, Legs: []Leg{{MustCatch: true, ExpectedCatch: catch, Diff: "diff"}}},
		"catch no expected": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindSeededCatch, Legs: []Leg{{Band: "deep", MustCatch: true, Diff: "diff"}}},
		"catch no class": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindSeededCatch, Legs: []Leg{{Band: "deep", MustCatch: true,
			ExpectedCatch: &ExpectedCatch{}, Diff: "diff"}}},
		"catch no diff":      {Schema: FamilySchema, ID: "EVAL-X", Kind: KindSeededCatch, Legs: []Leg{{Band: "deep", MustCatch: true, ExpectedCatch: catch}}},
		"catch no mandatory": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindSeededCatch, Legs: []Leg{{Band: "standard"}}},
		"floor comparator":   {Schema: FamilySchema, ID: "EVAL-X", Kind: KindFloorExclusion},
		"floor no legs": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindFloorExclusion,
			Comparator: ComparatorDefectClassMembership},
		"floor no candidates": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindFloorExclusion,
			Comparator: ComparatorDefectClassMembership, Legs: []Leg{{}}},
		"injection comparator": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindInjectionInvariance},
		"injection no legs": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindInjectionInvariance,
			Comparator: ComparatorVerdictEquality},
		"injection no diffs": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindInjectionInvariance,
			Comparator: ComparatorVerdictEquality, Legs: []Leg{{}}},
		"injection no verdict": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindInjectionInvariance,
			Comparator: ComparatorVerdictEquality, Legs: []Leg{{BaselineDiff: "a", InjectedDiff: "b"}}},
		"injection no directive": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindInjectionInvariance,
			Comparator: ComparatorVerdictEquality, Legs: []Leg{{BaselineDiff: "a", InjectedDiff: "b", BaselineVerdict: "approve"}}},
		"nit bound":     {Schema: FamilySchema, ID: "EVAL-X", Kind: KindNitGolden, Goldens: []Golden{{}}},
		"nit goldens":   {Schema: FamilySchema, ID: "EVAL-X", Kind: KindNitGolden, MaxFindingsP50: 1},
		"trunc compare": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindTruncationResistance},
		"trunc no legs": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindTruncationResistance,
			Comparator: ComparatorNoApproveOverOversize},
		"trunc no bytes": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindTruncationResistance,
			Comparator: ComparatorNoApproveOverOversize, Legs: []Leg{{ExpectedOutcome: outcome}}},
		"trunc no outcome": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindTruncationResistance,
			Comparator: ComparatorNoApproveOverOversize, Legs: []Leg{{DiffBytes: 1}}},
		"trunc no verdicts": {Schema: FamilySchema, ID: "EVAL-X", Kind: KindTruncationResistance,
			Comparator: ComparatorNoApproveOverOversize, Legs: []Leg{{DiffBytes: 1, ExpectedOutcome: &ExpectedOutcome{}}}},
		"unknown kind": {Schema: FamilySchema, ID: "EVAL-X", Kind: "future"},
	}
	_ = directive
	for name, family := range tests {
		t.Run(name, func(t *testing.T) {
			if err := validateFamily(family); err == nil {
				t.Fatal("malformed family validated")
			}
		})
	}
}

func TestFamilyLoadersRejectFilesystemParseAndDuplicateFailures(t *testing.T) {
	if _, err := LoadFamily(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("LoadFamily missing file: want error")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFamily(bad); err == nil {
		t.Fatal("LoadFamily malformed JSON: want error")
	}
	if _, err := LoadFamilies(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("LoadFamilies missing directory: want error")
	}
	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "README"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(empty, "nested.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFamilies(empty); err == nil {
		t.Fatal("LoadFamilies without fixtures: want error")
	}
}
