package ghtracker

import (
	"context"
	"strings"
	"testing"
)

func TestFrontierRefusesDemandWithoutRepositoryIdentity(t *testing.T) {
	c := &fakeClient{issues: []item{{Number: 11, Title: "[T1] task", State: "open", Labels: []string{DefaultRoutingLabel}}}}
	tasks, err := newTrackerWithRepository(c, DefaultRoutingLabel, " ").Frontier(context.Background())
	if err == nil || !strings.Contains(err.Error(), "native task repository is required") || len(tasks) != 0 {
		t.Fatalf("unscoped task demand: tasks=%+v err=%v", tasks, err)
	}
	if len(c.blocked) != 0 {
		t.Fatalf("read dependencies after refusing task identity: %v", c.blocked)
	}
}

func TestValidatePlanRefUsesClosedScopeGrammar(t *testing.T) {
	for _, tc := range []struct {
		ref   string
		valid bool
	}{
		{ref: "feature/001", valid: true},
		{ref: "feature/001/T1"},
		{ref: "../feature/001"},
		{ref: "feature/latest"},
		{ref: ""},
	} {
		if err := ValidatePlanRef(tc.ref); (err == nil) != tc.valid {
			t.Errorf("ValidatePlanRef(%q)=%v, valid=%v", tc.ref, err, tc.valid)
		}
	}
}
