package worker

import (
	"context"
	"github.com/agenticlab-ai/pyxis/internal/entity"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// SessionContinuity is the Worker's consumer-side seam for task session
// continuity: which harness conversation this task's next launch continues (or
// starts), and the record of what a finished launch ran under. It is OPTIONAL:
// a Worker built without one launches fresh conversations exactly as before.
// A session is per task and per harness — never resumed across tasks, never
// for the reviewer — and the implementation (tasksession.Continuity) decides
// resumability from durable evidence, not from this Worker's memory.
type SessionContinuity interface {
	// Session returns the spec for the task's next implementation launch on
	// harness inside worktree: a resume of the recorded conversation, or a
	// fresh one whose identity is already persisted.
	Session(ctx context.Context, task port.Task, harness entity.Harness, worktree string) (port.SessionSpec, error)
	// Record persists the session a finished launch actually ran under.
	Record(ctx context.Context, task port.Task, harness entity.Harness, worktree string, spec port.SessionSpec, res port.RunResult) error
}

// SessionEvent is one observable continuity moment (a reporting hook).
type SessionEvent struct {
	// Action is "resumed" (an existing session continued), "started" (a fresh
	// session began), or "degraded" (continuity was unavailable and the launch
	// ran fresh with no record).
	Action    string
	SessionID string
	Harness   entity.Harness
	// Purpose names the launch: "implementation", "remediation", "feedback".
	Purpose string
	Err     error
}

// SessionObserver receives SessionEvents; nil is ignored.
type SessionObserver func(task port.Task, event SessionEvent)

// SessionContinuityAware is implemented by a GreenDriver whose fixer launches
// should continue the task's session too (drivetogreen.Driver). The Worker
// hands its continuity over once per Run, after the task's harness is known.
type SessionContinuityAware interface {
	UseSessionContinuity(c SessionContinuity, harness entity.Harness, observe SessionObserver)
}

// WithSessionContinuity installs task session continuity. A nil value is
// ignored (fresh conversations, as before).
func WithSessionContinuity(c SessionContinuity) Option {
	return func(w *Worker) {
		if c != nil {
			w.sessions = c
		}
	}
}

// WithSessionObserver installs the continuity reporting hook. Nil is ignored.
func WithSessionObserver(o SessionObserver) Option {
	return func(w *Worker) {
		if o != nil {
			w.sessionObs = o
		}
	}
}

// sessionFor resolves the session an implementation-role launch continues.
// Continuity failures never fail the task: the launch runs fresh and the
// degradation is observable. purpose names the launch for the observer.
func (w *Worker) sessionFor(ctx context.Context, task port.Task, work port.Workdir, purpose string) *port.SessionSpec {
	if w.sessions == nil || work == nil {
		return nil
	}
	spec, err := w.sessions.Session(ctx, task, task.Harness, work.Dir())
	if err != nil {
		w.observeSession(task, SessionEvent{Action: "degraded", Harness: task.Harness, Purpose: purpose, Err: err})
		return nil
	}
	action := "started"
	if spec.Resume {
		action = "resumed"
	}
	w.observeSession(task, SessionEvent{Action: action, SessionID: spec.ID, Harness: task.Harness, Purpose: purpose})
	return &spec
}

// recordSession persists what a finished implementation-role launch ran under.
func (w *Worker) recordSession(ctx context.Context, task port.Task, work port.Workdir, spec *port.SessionSpec, res port.RunResult, purpose string) {
	if w.sessions == nil || spec == nil || work == nil {
		return
	}
	if err := w.sessions.Record(ctx, task, task.Harness, work.Dir(), *spec, res); err != nil {
		w.observeSession(task, SessionEvent{Action: "degraded", SessionID: spec.ID, Harness: task.Harness, Purpose: purpose, Err: err})
	}
}

func (w *Worker) observeSession(task port.Task, e SessionEvent) {
	if w.sessionObs != nil {
		w.sessionObs(task, e)
	}
}
