package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

type recoveryRuntime struct {
	scriptedRuntime
	specs []port.LaunchSpec
}

func (r *recoveryRuntime) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	r.specs = append(r.specs, spec)
	return r.scriptedRuntime.Launch(ctx, spec)
}

type diagnosticDriver struct {
	fakeGreenDriver
	diagnostic string
}

func (d *diagnosticDriver) InspectLocal(context.Context, port.Workdir) (worker.LocalGateResult, error) {
	return worker.LocalGateResult{Diagnostic: d.diagnostic}, nil
}

func TestBoundedRecoveryTransportsGateEvidenceAndPreservesTerminalCause(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, ls, ws, _, host := greenFakes(t)
		rt := &recoveryRuntime{scriptedRuntime: scriptedRuntime{blockAttempts: 100}}
		const diagnostic = "memory: evidence-history-unavailable; candidate 123, base 456; full diagnostic: /owned/check.log"
		capture := &captureEmitter{}
		d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: host}, Config{Width: 1, RespawnCap: 1, WorkerBudget: 95 * time.Second}, WithReporter(capture), WithGreenDriverFactory(func(port.AgentRuntime) (worker.GreenDriver, error) {
			return &diagnosticDriver{diagnostic: diagnostic}, nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		tr.setFrontier(task("T1", 1))
		rep, err := d.Sweep(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Results) != 1 || !errors.Is(rep.Results[0].Err, ErrRespawnCapExhausted) || !strings.Contains(rep.Results[0].Err.Error(), diagnostic) {
			t.Fatalf("terminal cause lost: %+v", rep)
		}
		if len(rt.specs) != 2 || rt.specs[0].RecoveryDiagnostic != "" || rt.specs[1].RecoveryDiagnostic != diagnostic || rt.specs[0].Brief != rt.specs[1].Brief {
			t.Fatalf("continuation lost evidence or changed charter: %+v", rt.specs)
		}
		if handoff, ok := capture.find(report.KindRecoveryHandoff, "T1"); !ok || handoff.Reason != diagnostic || handoff.Attempt != 2 || handoff.Cap != 2 {
			t.Fatalf("missing recovery handoff: %+v, %v", handoff, ok)
		}
		if capture.countKind(report.KindAgentProgress) < 8 {
			t.Fatal("long runs remained unobservable")
		}
		if rep.Succeeded != 0 || rep.Failed != 1 || ls.releases.Load() != 2 {
			t.Fatal("red recovery changed success or claim semantics")
		}
	})
}
