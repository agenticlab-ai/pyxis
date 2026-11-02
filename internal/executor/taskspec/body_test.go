package taskspec

import "testing"

func TestNativeTaskBodyAndIdentity(t *testing.T) {
	s := nativeFixture(t)
	body, err := RenderNative(s)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ParseTask(body, nil); err != nil || got.PlanTaskID != s.PlanTaskID {
		t.Fatalf("body carrier: %+v %v", got, err)
	}
	if _, err := ParseTask(body, []string{body}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		s           Spec
		title, want string
	}{
		{s, "T1", "T1"}, {s, "", "T1"}, {Spec{}, "T2", "T2"},
		{Spec{Requirements: s.Requirements, PlanTaskID: "adhoc/unique"}, "[ignored]", "unique"},
		{Spec{Requirements: s.Requirements, PlanTaskID: "single"}, "", "single"},
	} {
		if got := tc.s.TaskIdentity(tc.title); got != tc.want {
			t.Fatalf("identity=%q want %q", got, tc.want)
		}
	}
	if _, err := ParseTask("<!-- fs-spec:v1 {} -->", nil); err == nil {
		t.Fatal("legacy body became authoritative")
	}
}
