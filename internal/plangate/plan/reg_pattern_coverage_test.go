package plan

import (
	"strings"
	"testing"
)

func TestCheckPatternRejectsShapeAndMissingEdges(t *testing.T) {
	var reasons []string
	bad := func(_, reason, _ string) { reasons = append(reasons, reason) }
	checkPattern(&Env{Doc: &Doc{Tasks: []Task{{ID: "T1", Pattern: PatternExpand}, {ID: "T2", Pattern: PatternExpand}}}}, bad)
	if len(reasons) != 1 || !strings.Contains(reasons[0], "2 expand") {
		t.Fatalf("shape defects = %v", reasons)
	}

	reasons = nil
	doc := &Doc{Tasks: []Task{
		{ID: "T1", Pattern: PatternExpand},
		{ID: "T2", Pattern: PatternMigrate},
		{ID: "T3", Pattern: PatternContract},
	}}
	checkPattern(&Env{Doc: doc, Reach: map[string]map[string]bool{"T1": {}, "T2": {}, "T3": {}}}, bad)
	if len(reasons) != 2 || !strings.Contains(strings.Join(reasons, "\n"), "not blocked") {
		t.Fatalf("edge defects = %v", reasons)
	}
}

func TestCheckCharterReportsMechanicalAndBehaviorGaps(t *testing.T) {
	var reasons []string
	bad := func(_, reason, _ string) { reasons = append(reasons, reason) }
	checkCharter(nil, &Task{ID: "T1", Mechanical: true, Band: SizeBand{Lo: 1, Hi: 1}}, bad)
	if len(reasons) != 4 {
		t.Fatalf("mechanical defects = %v", reasons)
	}
	reasons = nil
	checkCharter(nil, &Task{ID: "T2", Provenance: []string{"wrong"}}, bad)
	if len(reasons) != 9 {
		t.Fatalf("behavior defects = %v", reasons)
	}
}
