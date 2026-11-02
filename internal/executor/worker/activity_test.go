package worker

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestBudgetHeartbeatsStopAndPartialSessionSurvivesCheckpoint(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rt := newBoundRuntime(true)
		rt.result = port.RunResult{SessionID: "native-thread", Tokens: 17, TokensObserved: true}
		deps, _, _ := boundDeps(t, rt)
		continuity := &fakeContinuity{}
		var events []Activity
		w := mustWorker(t, deps, WithBudget(95*time.Second), WithSessionContinuity(continuity), WithActivityObserver(func(_ port.Task, a Activity) { events = append(events, a) }))
		outcome, err := w.Run(t.Context(), newTestTask())
		if err != nil || outcome != OutcomeUnconverged {
			t.Fatalf("budget = %v, %v", outcome, err)
		}
		if len(events) != 5 || events[0].Status != "started" || events[4].Status != "budget_exhausted" {
			t.Fatalf("activity = %+v", events)
		}
		for i, remaining := range []int64{65000, 35000, 5000} {
			e := events[i+1]
			if e.Status != "running" || e.RemainingMS == nil || *e.RemainingMS != remaining {
				t.Fatalf("heartbeat = %+v, want remaining %d", e, remaining)
			}
		}
		if len(continuity.records) != 1 || continuity.records[0].SessionID != "native-thread" || w.Usage().Tokens != 17 || !w.Usage().TokensObserved || w.Usage().TurnsObserved {
			t.Fatalf("partial telemetry lost: records=%+v usage=%+v", continuity.records, w.Usage())
		}
		time.Sleep(time.Minute)
		if len(events) != 5 {
			t.Fatal("heartbeat survived its owner")
		}
	})
}
