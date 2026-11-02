package plan

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
)

func TestNativeRoutingGate(t *testing.T) {
	valid := nativePlan()
	for _, tc := range []struct {
		name, source string
		bad          bool
	}{
		{"supported", valid, false},
		{"unknown reasoning", strings.Replace(valid, "complex", "missing", 1), true},
		{"missing coupling", strings.Replace(valid, `"coupling":"composed",`, "", 1), true},
		{"unknown review role", strings.Replace(valid, "system_core", "unknown", 1), true},
		{"missing mechanical assessment", strings.Replace(valid, "| **Mechanical** | yes |", "| **Mechanical** | yes |\n| **Requirements** | {} |", 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := foldFixture(t, tc.source)
			if (len(res.Defects) > 0) != tc.bad {
				t.Fatalf("defects: %v", res.Defects)
			}
		})
	}
}

func TestHistoricalRoutingHasNoExecutableMeaning(t *testing.T) {
	for _, value := range []string{"—", "rung=1", "rung=2", "rung=3", "historical text"} {
		res := foldFixture(t, strings.Replace(goldenPlan, "rung=2", value, 1))
		if len(res.Defects) > 0 {
			t.Fatal(res.Defects)
		}
	}

}

func TestNativeRoutingGateRejectsMissingProgrammaticRoute(t *testing.T) {
	for _, task := range []Task{{ID: "T1"}, {ID: "T1", Requirements: &taskrequirements.Spec{}}} {
		rejected := false
		checkRouting(&Env{Doc: &Doc{Schema: NativeSchemaVersion}}, &task, func(_, _, _ string) { rejected = true })
		if !rejected {
			t.Fatal("accepted a native task without an explicit route")
		}
	}
}
