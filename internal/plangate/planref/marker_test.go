package planref

import "testing"

func TestPlanReferenceAndMarkerScope(t *testing.T) {
	for _, bad := range []string{"", "feature", "feature/1", "feature/1234", "a/b/c", "../x/001"} {
		if _, _, err := ParsePlanRef(bad); err == nil {
			t.Errorf("ParsePlanRef(%q) accepted an invalid reference", bad)
		}
	}
	f, v, err := ParsePlanRef("self-drive-wake/001")
	if err != nil || f != "self-drive-wake" || v != "001" {
		t.Fatalf("valid plan reference = (%q, %q, %v)", f, v, err)
	}
	for _, bad := range []string{
		"<!-- pyxis:plan-task ../../etc/001/T2 -->",
		"<!-- pyxis:plan-task feature/../T2 -->",
		"<!-- pyxis:plan-task --upload-pack=x/001/T2 -->",
		"<!-- pyxis:plan-task feature/001/design.md -->",
	} {
		if _, _, _, ok := ParseMarker(bad); ok {
			t.Errorf("ParseMarker(%q) accepted a malformed marker", bad)
		}
	}
	brief := "charter\n<!-- pyxis:plan-task self-drive-wake/001/T3 -->"
	if !InPlan(brief, f, v) || InPlan(brief, f, "002") || InPlan("no marker", f, v) {
		t.Fatal("plan scope did not match exactly the task's marker")
	}
}
