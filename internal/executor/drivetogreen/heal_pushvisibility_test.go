package drivetogreen

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// The fixtures here all share one shape: the code host keeps answering with the
// PRE-push head for a while after the loop publishes its fix. That lag is the
// ordinary case against a real code host, and a test whose fake flips to the new
// head on the very next read cannot observe any of this — which is exactly how
// incident I10 shipped green.

// A code host that has not caught up with our own push is NOT a foreign actor
// moving the proposal. The loop must wait for the head it published, then judge
// THAT head — reaching green on the first attempt (incident I10).
//
// Before the fix this failed twice over: the lagging read's stale RED verdict was
// accepted as the fix's own result (burning the attempt), and the next attempt
// then saw the real pushed head and failed the task closed as foreign movement.
func TestDrive_LaggingCodeHostReadIsNotForeignMovement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		host := &fakeHost{observations: []proposalObservation{
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed}, // initial settlement
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed}, // attempt 1 pre-launch
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed}, // attempt 1 pre-push
			// The push lands here. These two reads still answer with the pre-push head
			// and its already-settled RED — the verdict of the commit the fix replaced.
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			// Caught up: the published head, and its own verdict.
			{landing: port.LandingOpen, head: "head-b", checks: port.CheckGreen},
		}}
		runtime := &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)}
		pusher := &fakePusher{heads: []string{"head-b"}}
		d := mustDriver(t, Deps{
			Host: host, Inspector: host, Runtime: runtime, Fetcher: &fakeFetcher{}, Pusher: pusher, Runner: markerRunner,
		}, Config{Cap: 3, GateChecks: gateChecks(), SettleInterval: DefaultSettleInterval})

		res, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 1075})
		if err != nil {
			t.Fatalf("Drive: %v", err)
		}
		if !res.Green {
			t.Fatalf("result = %+v, want green", res)
		}
		// The stale reads must not consume the repair budget: one fixer, one attempt.
		if res.Attempts != 1 {
			t.Fatalf("attempts = %d, want 1 — a lagging read burned an attempt on the pre-fix verdict", res.Attempts)
		}
		if len(runtime.briefs) != 1 {
			t.Fatalf("fixer launches = %d, want 1", len(runtime.briefs))
		}
		if pusher.pushes != 1 {
			t.Fatalf("pushes = %d, want 1", pusher.pushes)
		}

	})
}

// The guard still has to work. A head that is neither the pre-push one nor the
// one we published is genuine concurrent movement and fails closed — recognising
// our own write must not blind the loop to somebody else's.
func TestDrive_ForeignHeadAdvanceAfterOurPushStillFailsClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		host := &fakeHost{observations: []proposalObservation{
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			// Someone else pushed while the repair ran: not our head, not the old one.
			{landing: port.LandingOpen, head: "head-stranger", checks: port.CheckRed},
		}}
		d := mustDriver(t, Deps{
			Host: host, Inspector: host, Runtime: &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)},
			Fetcher: &fakeFetcher{}, Pusher: &fakePusher{heads: []string{"head-b"}}, Runner: markerRunner,
		}, Config{Cap: 3, GateChecks: gateChecks(), SettleInterval: DefaultSettleInterval})

		_, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 1075})
		var changed *ProposalChangedError
		if !errors.As(err, &changed) {
			t.Fatalf("err = %v, want ProposalChangedError", err)
		}
		if changed.ActualHead != "head-stranger" || changed.ExpectedHead != "head-b" {
			t.Fatalf("error = %+v, want the stranger's head against the one we published", changed)
		}

	})
}

// Tolerating the lag must stay bounded: a push the code host never shows is a
// real fault. It fails closed naming what actually happened — our commit never
// appeared — never as somebody else moving the proposal.
func TestDrive_PushThatNeverBecomesVisibleFailsClosedAsItsOwnFault(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		host := &fakeHost{observations: []proposalObservation{
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
		}} // drained: every later read repeats the pre-push head forever
		d := mustDriver(t, Deps{
			Host: host, Inspector: host, Runtime: &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)},
			Fetcher: &fakeFetcher{}, Pusher: &fakePusher{heads: []string{"head-b"}}, Runner: markerRunner,
		}, Config{Cap: 3, GateChecks: gateChecks(), SettleInterval: DefaultSettleInterval, PushVisibilityPolls: 3})

		_, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 1075})
		var notVisible *PushNotVisibleError
		if !errors.As(err, &notVisible) {
			t.Fatalf("err = %v, want PushNotVisibleError", err)
		}
		if notVisible.Pushed != "head-b" || notVisible.Observed != "head-a" {
			t.Fatalf("error = %+v, want the published head against the one still reported", notVisible)
		}
		var changed *ProposalChangedError
		if errors.As(err, &changed) {
			t.Fatalf("a push that never appeared was reported as concurrent movement: %v", err)
		}

	})
}

// A Pusher that cannot name its commit keeps the pre-I10 behaviour rather than
// inventing movement from an unknown head: the loop still drives to green.
func TestDrive_PusherWithoutAHeadKeepsTheFirstObservationFence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		host := &fakeHost{observations: []proposalObservation{
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckRed},
			{landing: port.LandingOpen, head: "head-a", checks: port.CheckGreen},
		}}
		d := mustDriver(t, Deps{
			Host: host, Inspector: host, Runtime: &fakeRuntime{onLaunch: writeMarkerOnLaunch(dir)},
			Fetcher: &fakeFetcher{}, Pusher: &fakePusher{}, Runner: markerRunner,
		}, Config{Cap: 3, GateChecks: gateChecks(), SettleInterval: DefaultSettleInterval})

		res, err := d.Drive(context.Background(), newTask(), &fakeWorkdir{dir: dir}, port.Proposal{Number: 1075})
		if err != nil {
			t.Fatalf("Drive: %v", err)
		}
		if !res.Green || res.Attempts != 1 {
			t.Fatalf("result = %+v, want green on one attempt", res)
		}

	})
}
