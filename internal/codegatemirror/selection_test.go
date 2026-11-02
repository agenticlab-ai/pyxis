package codegatemirror

import (
	"reflect"
	"testing"
)

func TestSelectUsesCanonicalCommand(t *testing.T) {
	cfg := Config{Checks: []Check{
		{Name: "memory", Command: "false", Class: ClassRequired},
		{Name: "test", Command: "true", Class: ClassRequired},
	}}
	all, err := cfg.Select("")
	if err != nil || !reflect.DeepEqual(all, cfg) {
		t.Fatalf("complete gate = %+v, %v", all, err)
	}
	selected, err := cfg.Select("memory")
	if err != nil || len(selected.Checks) != 1 || !reflect.DeepEqual(selected.Checks[0], cfg.Checks[0]) {
		t.Fatalf("selected = %+v, %v", selected, err)
	}
	if cfg.Checks[0].Command = "true"; selected.Checks[0].Command != "false" {
		t.Fatal("selection aliases the original slice")
	}
	if _, err := cfg.Select("nonexistent"); err == nil {
		t.Fatal("unknown check passed")
	}
}

func TestSelectionsPartitionConfigInCanonicalOrder(t *testing.T) {
	cfg := Config{Checks: []Check{
		{Name: "quality", Command: "quality-command", Class: ClassRequired},
		{Name: "boundary", Command: "boundary-command", Class: ClassRequired, Env: []string{"CONFINEMENT"}},
		{Name: "race", Command: "race-command", Class: ClassRequired},
		{Name: "new-check", Command: "new-command", Class: ClassRequired},
	}}
	selected, err := cfg.Select("race,boundary")
	if err != nil || !reflect.DeepEqual(selected.Checks, cfg.Checks[1:3]) {
		t.Fatalf("selected = %+v, %v; want canonical checks in config order", selected, err)
	}
	remaining, err := cfg.SelectExcept("race,boundary")
	want := []Check{cfg.Checks[0], cfg.Checks[3]}
	if err != nil || !reflect.DeepEqual(remaining.Checks, want) {
		t.Fatalf("complement = %+v, %v; want quality and the newly configured check", remaining, err)
	}
	remaining.Checks[0].Command = "mutated"
	if cfg.Checks[0].Command != "quality-command" {
		t.Fatal("complement aliases the original check slice")
	}
}

func TestMalformedSelectionsRefuse(t *testing.T) {
	cfg := Config{Checks: []Check{{Name: "one"}, {Name: "two"}}}
	for _, names := range []string{"unknown", "one,unknown", "one,one", "one,", ",one", "one,,two", " one", "one, two"} {
		for _, selectChecks := range []func(string) (Config, error){cfg.Select, cfg.SelectExcept} {
			if _, err := selectChecks(names); err == nil {
				t.Errorf("invalid selection %q passed", names)
			}
		}
	}
	for _, names := range []string{"", "one,two"} {
		if _, err := cfg.SelectExcept(names); err == nil {
			t.Errorf("empty complement %q passed", names)
		}
	}
}

func TestSelectedGreenCannotCertifyOtherChecks(t *testing.T) {
	dir := gitFixture(t)
	cfg := Config{Checks: []Check{
		{Name: "green", Command: "true", Class: ClassRequired},
		{Name: "red", Command: "false", Class: ClassRequired},
	}}
	green, err := cfg.Select("green")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		report, hit, err := green.RunCached(t.Context(), dir)
		if err != nil || !report.Passed || hit != (i == 1) {
			t.Fatalf("selected green run %d = %+v, hit %v, %v", i, report, hit, err)
		}
	}
	other, err := cfg.SelectExcept("green")
	if err != nil {
		t.Fatal(err)
	}
	for _, checks := range []Config{other, cfg} {
		report, hit, err := checks.RunCached(t.Context(), dir)
		if err != nil || report.Passed || hit {
			t.Fatalf("cached subset certified unexecuted red check: %+v, hit %v, %v", report, hit, err)
		}
	}
}
