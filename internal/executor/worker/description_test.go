package worker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

func TestRun_ProposalCarriesTheAgentsDescriptionWithComputedFields(t *testing.T) {
	deps, _, _, space, runtime, host := greenDeps(t)
	space.size = changesize.Size{Files: []changesize.File{{Path: "a.go", Class: changesize.Code, Added: 5, Removed: 2}}}
	task := newTestTask()
	task.DemandID = "demand-1"
	task.DemandProfile = executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityStrong, Deliberation: executiondemand.DeliberationDeep,
		Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceElevated}

	outcome, err := mustWorker(t, deps).Run(context.Background(), task)
	if outcome != OutcomeSucceeded || err != nil {
		t.Fatalf("Run = %v, %v", outcome, err)
	}
	if !runtime.launches[0].RequestDescription {
		t.Fatal("the implementation launch did not ask for a description")
	}
	if len(host.opened) != 1 {
		t.Fatalf("OpenProposal calls = %d", len(host.opened))
	}
	d := host.opened[0]
	if d.LinesOfCode != "7 (+5/-2)" || d.Complexity != "medium (implementation capability: strong)" || d.BlastRadius != "low" || d.What != "- Implement the task." {
		t.Fatalf("published description = %#v", d)
	}
	if _, problems := prdescription.Parse(d.Render()); len(problems) != 0 {
		t.Fatalf("the published description misses the template: %v", problems)
	}
	if len(host.describes) != 1 || host.describedAfterPushes[0] != 0 {
		t.Fatalf("an open proposal is described before the push: %v", host.describedAfterPushes)
	}
}

func TestRun_AgentComplexityStandsWithoutARoutingAssessment(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	if outcome, err := mustWorker(t, deps).Run(context.Background(), newTestTask()); outcome != OutcomeSucceeded || err != nil {
		t.Fatalf("Run = %v, %v", outcome, err)
	}
	if got := host.opened[0]; got.Complexity != "low" || got.LinesOfCode != changesize.NotApplicable {
		t.Fatalf("description = %#v", got)
	}
}

func TestRun_MissingOrIncompleteDescriptionRespawnsWithTheReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result port.RunResult
		want   string
	}{
		{"missing", port.RunResult{Output: "Done; the tests pass."}, "no description between"},
		{"incomplete", port.RunResult{Output: strings.Replace(agentDescription, "- Decision type: two-way\n", "", 1)}, "Decision type: missing"},
		{"truncated", port.RunResult{Turns: 150, Incomplete: true, Reason: "error_max_turns"}, "no description between"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, _, space, runtime, host := greenDeps(t)
			runtime.noDescription = true
			runtime.result = tc.result
			w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{gateGreen: true}))
			outcome, err := w.Run(context.Background(), newTestTask())
			if outcome != OutcomeUnconverged || err != nil {
				t.Fatalf("Run = %v, %v; want unconverged for a respawn", outcome, err)
			}
			if len(host.openedTasks) != 0 || len(host.describes) != 0 || space.last.pushes != 0 {
				t.Fatal("work without a description was published")
			}
			if space.last.checkpoints == 0 {
				t.Fatal("the work was not kept for the next attempt")
			}
			if diagnostic := w.RecoveryDiagnostic(); !strings.Contains(diagnostic, tc.want) || !strings.Contains(diagnostic, "pull request description") {
				t.Fatalf("recovery diagnostic = %q", diagnostic)
			}
		})
	}
}

// The described checkpoint is reported once, after it is committed and before
// local verification can fail, so a failed attempt does not lose the
// description an authorized recovery would publish.
func TestRun_ReportsTheDescribedCheckpointBeforeLocalVerification(t *testing.T) {
	for _, tc := range []struct {
		name      string
		described bool
	}{{"described", true}, {"undescribed", false}} {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, _, space, runtime, host := greenDeps(t)
			if !tc.described {
				runtime.noDescription = true
				runtime.result = port.RunResult{Output: "Done."}
			}
			driver := &fakeGreenDriver{localErr: errors.New("local gate red: lint dependency download failed")}
			var seen []prdescription.Description
			w := mustWorker(t, deps, WithGreenDriver(driver), WithDescribedCheckpointObserver(func(_ context.Context, task port.Task, work port.Workdir, d prdescription.Description) {
				if task.ID != "T-42" || work != port.Workdir(space.last) || space.last.checkpoints != 1 || driver.localCalled != 0 {
					t.Errorf("observed task %s in %v after %d checkpoints and %d local drives", task.ID, work, space.last.checkpoints, driver.localCalled)
				}
				seen = append(seen, d)
			}))
			outcome, err := w.Run(context.Background(), newTestTask())
			if tc.described && (outcome != OutcomeFailed || err == nil || len(seen) != 1 || seen[0].What != "- Implement the task.") {
				t.Fatalf("Run = %v, %v; observed %#v", outcome, err, seen)
			}
			if !tc.described && (outcome != OutcomeUnconverged || len(seen) != 0) {
				t.Fatalf("undescribed attempt: %v %v observed %d", outcome, err, len(seen))
			}
			if space.last.pushes != 0 || len(host.openedTasks) != 0 {
				t.Fatal("an unverified checkpoint was published")
			}
		})
	}
}

func TestRun_DescriptionFailuresFailClosedBeforeThePush(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*fakeWorkspace, *fakeHost)
		want  string
	}{
		{"measure", func(s *fakeWorkspace, _ *fakeHost) { s.sizeErr = errors.New("git diff failed") }, "measure task T-42's change"},
		{"describe", func(_ *fakeWorkspace, h *fakeHost) { h.describeErr = errors.New("code host down") }, "describe the open proposal for task T-42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, _, space, _, host := greenDeps(t)
			tc.setup(space, host)
			outcome, err := mustWorker(t, deps).Run(context.Background(), newTestTask())
			if outcome != OutcomeFailed || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Run = %v, %v; want failed with %q", outcome, err, tc.want)
			}
			if space.last.pushes != 0 || len(host.openedTasks) != 0 {
				t.Fatal("a revision was pushed without its description")
			}
		})
	}
}

func TestRun_RemediationLaunchesDoNotAskForADescription(t *testing.T) {
	deps, _, _, _, runtime, host := greenDeps(t)
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "- fix"), approve("sha-2")}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
	if outcome, err := w.Run(context.Background(), newTestTask()); outcome != OutcomeSucceeded || err != nil {
		t.Fatalf("Run = %v, %v", outcome, err)
	}
	if len(runtime.launches) != 2 || !runtime.launches[0].RequestDescription || runtime.launches[1].RequestDescription {
		t.Fatalf("description requests by launch: %+v", runtime.launches)
	}
	if len(host.opened) != 1 || host.opened[0].What != "- Implement the task." {
		t.Fatalf("the implementation run's description is published: %#v", host.opened)
	}
}

// describingGreenDriver records the describer the Worker binds for re-pushes.
type describingGreenDriver struct {
	*fakeGreenDriver
	describe func(context.Context, port.Workdir) error
}

func (d *describingGreenDriver) BindDescriber(describe func(context.Context, port.Workdir) error) {
	d.describe = describe
}

func TestRun_BindsTheDescriberForDriveToGreenRepushes(t *testing.T) {
	deps, _, _, space, _, host := greenDeps(t)
	driver := &describingGreenDriver{fakeGreenDriver: &fakeGreenDriver{}}
	if outcome, err := mustWorker(t, deps, WithGreenDriver(driver)).Run(context.Background(), newTestTask()); outcome != OutcomeSucceeded || err != nil {
		t.Fatalf("Run = %v, %v", outcome, err)
	}
	if driver.describe == nil {
		t.Fatal("no describer was bound")
	}
	space.last.size = changesize.Size{Files: []changesize.File{{Path: "a.go", Class: changesize.Code, Added: 9}}}
	if err := driver.describe(context.Background(), space.last); err != nil {
		t.Fatal(err)
	}
	if last := host.describes[len(host.describes)-1]; last.LinesOfCode != "9 (+9/-0)" || last.What != "- Implement the task." {
		t.Fatalf("re-push description = %#v", last)
	}
	space.last.sizeErr = errors.New("git diff failed")
	if err := driver.describe(context.Background(), space.last); err == nil {
		t.Fatal("a failed measurement was not reported to the loop")
	}
}
