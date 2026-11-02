package drivetogreen

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestCoverage90TerminalDiagnostics(t *testing.T) {
	if got := (&EscalationError{TaskID: "T1", Attempts: 2, Cap: 2}).Error(); !strings.Contains(got, "required CI checks still red") {
		t.Fatalf("empty-reason escalation = %q", got)
	}
	for _, tc := range []struct {
		landing port.Landing
		want    string
	}{
		{landing: port.LandingLanded, want: "landed while repair"},
		{landing: port.LandingClosedUnlanded, want: "closed without landing"},
		{landing: port.LandingUnknown, want: "became unavailable"},
	} {
		got := (&ProposalChangedError{Proposal: 7, Landing: tc.landing, Checks: port.CheckRed}).Error()
		if !strings.Contains(got, tc.want) {
			t.Errorf("landing %q diagnostic = %q, want %q", tc.landing, got, tc.want)
		}
	}
}

type coverageRoute struct{ model, effort string }

func (coverageRoute) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	return nil, nil
}
func (coverageRoute) Await(context.Context, port.Handle) (port.RunResult, error) {
	return port.RunResult{}, nil
}
func (coverageRoute) Kill(context.Context, port.Handle) error { return nil }
func (coverageRoute) AuthCheck(context.Context) error         { return nil }
func (r coverageRoute) ModelRoute() port.ModelRoute {
	return port.ModelRoute{Model: r.model, Effort: r.effort}
}

type coverageUnrouted struct{}

func (coverageUnrouted) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	return nil, nil
}
func (coverageUnrouted) Await(context.Context, port.Handle) (port.RunResult, error) {
	return port.RunResult{}, nil
}
func (coverageUnrouted) Kill(context.Context, port.Handle) error { return nil }
func (coverageUnrouted) AuthCheck(context.Context) error         { return nil }

func TestLaunchRoute(t *testing.T) {
	d := &Driver{runtime: coverageUnrouted{}}
	if _, _, err := d.launchRoute(); err == nil {
		t.Fatal("runtime without a route was accepted")
	}
	for _, rt := range []coverageRoute{{effort: "low"}, {model: "m"}} {
		d.runtime = rt
		if _, _, err := d.launchRoute(); err == nil {
			t.Fatalf("empty route %+v was accepted", rt)
		}
	}
	d.runtime = coverageRoute{model: "m", effort: "low"}
	model, effort, err := d.launchRoute()
	if err != nil || model != "m" || effort != "low" {
		t.Fatalf("provider route = %q %q, %v", model, effort, err)
	}
	d.BindRoute("", "low")
	if _, _, err := d.launchRoute(); err == nil {
		t.Fatal("empty bound route was accepted")
	}
	d.BindRoute("bound", "high")
	model, effort, err = d.launchRoute()
	if err != nil || model != "bound" || effort != "high" {
		t.Fatalf("bound route = %q %q, %v", model, effort, err)
	}
}
