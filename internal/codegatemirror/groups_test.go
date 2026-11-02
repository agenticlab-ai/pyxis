package codegatemirror

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
)

func TestGroupsPreserveSharedDefinitionsAndOrder(t *testing.T) {
	cfg, err := Parse([]byte(`code-gate:
  quality:
    command: true
  first-test:
    command: false
    group: tests
    env: CONFINEMENT
  new-check:
    command: true
  second-test:
    command: true
    group: tests
`))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Groups(); !slices.Equal(got, []string{DefaultGroup, "tests"}) {
		t.Fatalf("groups = %v", got)
	}
	for _, tt := range []struct {
		name string
		want []Check
	}{
		{DefaultGroup, []Check{cfg.Checks[0], cfg.Checks[2]}},
		{"tests", []Check{cfg.Checks[1], cfg.Checks[3]}},
	} {
		selected, err := cfg.SelectGroup(tt.name)
		if err != nil || !reflect.DeepEqual(selected.Checks, tt.want) {
			t.Fatalf("group %s = %+v, %v", tt.name, selected, err)
		}
		selected.Checks[0].Command = "changed"
		if slices.ContainsFunc(cfg.Checks, func(c Check) bool { return c.Command == "changed" }) {
			t.Fatal("selection aliases the original check slice")
		}
	}
	if _, err := cfg.SelectGroup("absent"); err == nil {
		t.Fatal("unknown group passed")
	}
	cfg.Checks[1].Phase = 2
	cfg.Checks[3].Phase = 2
	cheap, err := cfg.SelectCheap()
	if err != nil || !reflect.DeepEqual(cheap.Checks, []Check{cfg.Checks[0], cfg.Checks[2]}) {
		t.Fatalf("SelectCheap = %+v, %v", cheap, err)
	}
	if _, err := (Config{}).SelectCheap(); err == nil {
		t.Fatal("empty cheap selection passed")
	}
	phaseTwo, err := cfg.SelectPhase(2)
	if err != nil || !reflect.DeepEqual(phaseTwo.Checks, []Check{cfg.Checks[1], cfg.Checks[3]}) {
		t.Fatalf("SelectPhase(2) = %+v, %v", phaseTwo, err)
	}
	if _, err := cfg.SelectPhase(0); err == nil {
		t.Fatal("phase 0 passed")
	}
	if _, err := cfg.SelectPhase(3); err == nil {
		t.Fatal("empty later phase passed")
	}
	testsPhaseTwo, err := cfg.SelectGroupPhase("tests", 2)
	if err != nil || !reflect.DeepEqual(testsPhaseTwo.Checks, []Check{cfg.Checks[1], cfg.Checks[3]}) {
		t.Fatalf("SelectGroupPhase(tests, 2) = %+v, %v", testsPhaseTwo, err)
	}
	if _, err := cfg.SelectGroupPhase(DefaultGroup, 2); err == nil {
		t.Fatal("default phase 2 passed")
	}
	for _, name := range []string{"", "two groups", "tests;true", "tests*", "tests${TOKEN}", "tests,default", "'tests'", "-tests", "1tests", "é"} {
		if _, err := cfg.SelectGroup(name); err == nil {
			t.Errorf("invalid selection %q passed", name)
		}
		if _, err := Parse([]byte("code-gate:\n  test:\n    command: true\n    group: " + name + "\n")); err == nil {
			t.Errorf("invalid configured group %q passed", name)
		}
	}
}

func TestAdvisoryFailureDoesNotCancelPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := Config{Checks: []Check{
			{Name: "a1", Class: ClassAdvisory},
			{Name: "b1", Class: ClassRequired, Group: "tests"},
			{Name: "a2", Class: ClassRequired},
			{Name: "b2", Class: ClassRequired, Group: "tests"},
		}}
		started := make(chan string, 4)
		release := make(chan struct{})
		defer close(release)
		done := make(chan Report, 1)
		go func() {
			done <- cfg.run(t.Context(), "workspace", func(_ context.Context, workspace string, c Check) Outcome {
				if workspace != "workspace" {
					t.Error("workspace lost")
				}
				started <- c.Name
				if strings.HasSuffix(c.Name, "1") {
					<-release
				}
				return Outcome{Name: c.Name, Class: c.Class, Passed: c.Name != "a1"}
			})
		}()
		synctest.Wait()
		if len(started) != 2 {
			t.Fatalf("started %d checks while first checks blocked; want one per group", len(started))
		}
		first := []string{<-started, <-started}
		slices.Sort(first)
		if !slices.Equal(first, []string{"a1", "b1"}) {
			t.Fatalf("group order changed: %v", first)
		}
		release <- struct{}{}
		release <- struct{}{}
		rep := <-done
		if !rep.Passed || len(rep.Outcomes) != 4 || len(started) != 2 {
			t.Fatalf("advisory failure cancelled peers or flipped verdict: %+v started=%d", rep, len(started))
		}
		for i, outcome := range rep.Outcomes {
			if outcome.Name != cfg.Checks[i].Name {
				t.Fatalf("report lost configuration order: %+v", rep.Outcomes)
			}
		}
	})
}

func TestRequiredFailureCancelsSiblingGroupsAndQueuedChecks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := Config{Checks: []Check{
			{Name: "a1", Class: ClassRequired},
			{Name: "b1", Class: ClassRequired, Group: "tests"},
			{Name: "a2", Class: ClassRequired},
			{Name: "b2", Class: ClassRequired, Group: "tests"},
		}}
		started := make(chan string, 4)
		release := make(chan struct{})
		defer close(release)
		done := make(chan Report, 1)
		go func() {
			done <- cfg.run(t.Context(), "workspace", func(ctx context.Context, workspace string, c Check) Outcome {
				if workspace != "workspace" {
					t.Error("workspace lost")
				}
				started <- c.Name
				if strings.HasSuffix(c.Name, "1") {
					<-release
				}
				if err := ctx.Err(); err != nil && c.Name != "a1" {
					return Outcome{Name: c.Name, Class: c.Class, Detail: "cancelled during run: " + err.Error()}
				}
				return Outcome{Name: c.Name, Class: c.Class, Passed: c.Name != "a1"}
			})
		}()
		synctest.Wait()
		if len(started) != 2 {
			t.Fatalf("started %d checks while first checks blocked; want one per group", len(started))
		}
		release <- struct{}{}
		release <- struct{}{}
		rep := <-done
		if rep.Passed || len(rep.Outcomes) != 4 {
			t.Fatalf("required failure passed or lost outcomes: %+v", rep)
		}
		if len(started) > 3 {
			t.Fatalf("required failure started later queued work: %d", len(started))
		}
		if rep.Outcomes[0].Passed || !strings.Contains(rep.Outcomes[2].Detail, "cancelled") {
			t.Fatalf("same-group queued check was not cancelled: %+v", rep.Outcomes)
		}
	})
}

func TestLaterPhaseDoesNotStartAfterRequiredFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := Config{Checks: []Check{
			{Name: "cheap", Class: ClassRequired},
			{Name: "race", Class: ClassRequired, Group: "tests", Phase: 2},
			{Name: "coverage", Class: ClassRequired, Phase: 2},
		}}
		started := make(chan string, 3)
		rep := cfg.run(t.Context(), "", func(_ context.Context, _ string, c Check) Outcome {
			started <- c.Name
			return Outcome{Name: c.Name, Class: c.Class, Passed: false}
		})
		if rep.Passed || len(started) != 1 || <-started != "cheap" {
			t.Fatalf("phase 2 started after cheap failed: started=%d passed=%v", len(started), rep.Passed)
		}
		if !strings.Contains(rep.Outcomes[1].Detail, "cancelled before run") || !strings.Contains(rep.Outcomes[2].Detail, "cancelled before run") {
			t.Fatalf("phase 2 outcomes = %+v", rep.Outcomes)
		}
	})
}

func TestPhaseTwoStartsOnlyAfterPhaseOne(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := Config{Checks: []Check{
			{Name: "cheap-a", Class: ClassRequired},
			{Name: "cheap-b", Class: ClassRequired, Group: "tests"},
			{Name: "race", Class: ClassRequired, Group: "tests", Phase: 2},
			{Name: "coverage", Class: ClassRequired, Phase: 2},
		}}
		started := make(chan string, 4)
		releaseCheap := make(chan struct{})
		defer close(releaseCheap)
		done := make(chan Report, 1)
		go func() {
			done <- cfg.run(t.Context(), "", func(_ context.Context, _ string, c Check) Outcome {
				started <- c.Name
				if c.ExecutionPhase() == DefaultPhase {
					<-releaseCheap
				}
				return Outcome{Name: c.Name, Class: c.Class, Passed: true}
			})
		}()
		synctest.Wait()
		if len(started) != 2 {
			t.Fatalf("phase 2 overlapped phase 1: started %d", len(started))
		}
		releaseCheap <- struct{}{}
		releaseCheap <- struct{}{}
		rep := <-done
		if !rep.Passed || len(started) != 4 {
			t.Fatalf("phase 2 missing or failed: %+v started=%d", rep, len(started))
		}
		got := []string{<-started, <-started, <-started, <-started}
		if !slices.Contains(got, "race") || !slices.Contains(got, "coverage") {
			t.Fatalf("phase 2 checks not run: %v", got)
		}
	})
}

func TestGroupCancellationWaitsForWorkersAndStopsQueuedChecks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		cfg := Config{Checks: []Check{
			{Name: "a1", Class: ClassAdvisory},
			{Name: "b1", Class: ClassAdvisory, Group: "tests"},
			{Name: "a2", Class: ClassAdvisory},
			{Name: "b2", Class: ClassAdvisory, Group: "tests"},
		}}
		started := make(chan string, 4)
		cleanup := make(chan struct{})
		defer close(cleanup)
		done := make(chan Report, 1)
		go func() {
			done <- cfg.run(ctx, "", func(ctx context.Context, _ string, c Check) Outcome {
				started <- c.Name
				<-ctx.Done()
				<-cleanup
				return Outcome{Name: c.Name, Class: c.Class, Detail: "cancelled"}
			})
		}()
		synctest.Wait()
		if len(started) != 2 {
			t.Fatalf("started %d checks, want both group workers", len(started))
		}
		cancel()
		synctest.Wait()
		if len(done) != 0 {
			t.Fatal("Run returned before active workers finished cleanup")
		}
		cleanup <- struct{}{}
		cleanup <- struct{}{}
		rep := <-done
		if rep.Passed || len(started) != 2 || len(rep.Outcomes) != 4 {
			t.Fatalf("cancelled run passed or started queued work: %+v", rep)
		}
		for _, i := range []int{2, 3} {
			if !strings.Contains(rep.Outcomes[i].Detail, "cancelled before run") {
				t.Fatalf("queued check did not record cancellation: %+v", rep.Outcomes[i])
			}
		}
	})
}

func TestGroupMembershipInvalidatesCachedVerdict(t *testing.T) {
	dir := gitFixture(t)
	cfg := Config{Checks: []Check{
		{Name: "first", Command: "true", Class: ClassRequired},
		{Name: "second", Command: "true", Class: ClassRequired},
	}}
	for i := range 2 {
		rep, hit, err := cfg.RunCached(t.Context(), dir)
		if err != nil || !rep.Passed || hit != (i == 1) {
			t.Fatalf("initial run %d = %+v, hit %v, %v", i, rep, hit, err)
		}
	}
	cfg.Checks[1].Group = "tests"
	if rep, hit, err := cfg.RunCached(t.Context(), dir); err != nil || hit || !rep.Passed {
		t.Fatalf("regrouped gate reused verdict: %+v, hit %v, %v", rep, hit, err)
	}
	cfg.Checks[1].Phase = 2
	if rep, hit, err := cfg.RunCached(t.Context(), dir); err != nil || hit || !rep.Passed {
		t.Fatalf("rephased gate reused verdict: %+v, hit %v, %v", rep, hit, err)
	}
	// A group's green verdict must never certify the omitted group.
	cfg.Checks[1].Command = "false"
	selected, err := cfg.SelectGroup(DefaultGroup)
	if err != nil {
		t.Fatal(err)
	}
	if rep, hit, err := selected.RunCached(t.Context(), dir); err != nil || hit || !rep.Passed {
		t.Fatalf("selected group = %+v, hit %v, %v", rep, hit, err)
	}
	if rep, hit, err := cfg.RunCached(t.Context(), dir); err != nil || hit || rep.Passed {
		t.Fatalf("group verdict certified full gate: %+v, hit %v, %v", rep, hit, err)
	}
}
