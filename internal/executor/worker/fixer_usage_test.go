package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type usageDriver struct{ observer func(port.RunResult) }

func (d *usageDriver) BindUsageObserver(observe func(port.RunResult)) { d.observer = observe }
func (d *usageDriver) DriveLocal(context.Context, port.Task, port.Workdir) (LocalDriveResult, error) {
	d.observer(port.RunResult{Tokens: 91, Turns: 3, ToolCalls: 5})
	return LocalDriveResult{Attempts: 1, Cap: 3}, errors.New("partial fixer refused")
}
func (*usageDriver) GateGreen(context.Context, port.Workdir) (bool, error) { return false, nil }
func (*usageDriver) Drive(context.Context, port.Task, port.Workdir, port.Proposal) (GreenDriveResult, error) {
	return GreenDriveResult{}, nil
}

func TestWorkerIncludesPartialFixerUsageBeforeLocalFailure(t *testing.T) {
	deps, _, _, _, rt, host := greenDeps(t)
	rt.result = port.RunResult{Tokens: 17, Turns: 1, ToolCalls: 2, TokensObserved: true, TurnsObserved: true, ToolCallsObserved: true}
	w := mustWorker(t, deps, WithGreenDriver(&usageDriver{}))
	outcome, err := w.Run(t.Context(), newTestTask())
	if err == nil || outcome != OutcomeFailed || w.Usage().Tokens != 108 || w.Usage().Turns != 4 || w.Usage().ToolCalls != 7 || len(host.openedTasks) != 0 || w.VerificationAttempted() {
		t.Fatalf("partial fixer accounting: %v %v %+v", outcome, err, w.Usage())
	}
	if w.Usage().TokensObserved || w.Usage().TurnsObserved || w.Usage().ToolCallsObserved {
		t.Fatal("unknown fixer usage became a measured total")
	}
}
