package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type remediationUsageRuntime struct {
	*fakeRuntime
	awaits          int
	fail            bool
	checkpointSpace *fakeWorkspace
}

func (r *remediationUsageRuntime) Await(ctx context.Context, h port.Handle) (port.RunResult, error) {
	r.awaits++
	if r.awaits == 2 {
		res := port.RunResult{Tokens: 19, Turns: 2, ToolCalls: 3}
		if r.checkpointSpace != nil {
			r.checkpointSpace.last.checkpointErr = errors.New("cannot persist remediation checkpoint")
		}
		if r.fail {
			return res, port.ErrNotQuiescent
		}
		return res, nil
	}
	return r.fakeRuntime.Await(ctx, h)
}

func TestRemediationCheckpointFailureCannotPublish(t *testing.T) {
	deps, _, _, space, rt, host := greenDeps(t)
	deps.Runtime = &remediationUsageRuntime{fakeRuntime: rt, checkpointSpace: space}
	reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "fix finding"), approve("sha-2")}}
	w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
	outcome, err := w.Run(t.Context(), newTestTask())
	if outcome != OutcomeFailed || !errors.Is(err, space.last.checkpointErr) || len(host.openedTasks) != 0 {
		t.Fatalf("unpersisted remediation published: %v %v", outcome, err)
	}
}

func TestRemediationUsageCountsOnceAndUnquiescentWorkCannotCheckpoint(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "not quiescent"}[fail], func(t *testing.T) {
			deps, _, _, space, rt, host := greenDeps(t)
			rt.result = port.RunResult{Tokens: 11, Turns: 1, ToolCalls: 2, TokensObserved: true, TurnsObserved: true, ToolCallsObserved: true}
			deps.Runtime = &remediationUsageRuntime{fakeRuntime: rt, fail: fail}
			reviewer := &fakePrePublishReviewer{reviews: []PrePublishReview{requestChanges("sha-1", "fix finding"), approve("sha-2")}}
			w := mustWorker(t, deps, WithGreenDriver(&fakeGreenDriver{}), WithPrePublishReview(reviewer, 2))
			outcome, err := w.Run(t.Context(), newTestTask())
			if fail {
				if outcome != OutcomeFailed || !errors.Is(err, port.ErrNotQuiescent) || space.last.checkpoints != 1 || len(host.openedTasks) != 0 {
					t.Fatalf("unsafe remediation disposition: %v %v checkpoints=%d", outcome, err, space.last.checkpoints)
				}
			} else if err != nil || outcome != OutcomeSucceeded {
				t.Fatalf("completed remediation: %v %v", outcome, err)
			}
			usage := w.Usage()
			if usage.Tokens != 30 || usage.Turns != 3 || usage.ToolCalls != 5 || usage.TokensObserved || usage.TurnsObserved || usage.ToolCallsObserved {
				t.Fatalf("lost or double-counted usage: %+v", usage)
			}
		})
	}
}
