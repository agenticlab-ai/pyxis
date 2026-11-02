package worker

import (
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// Activity carries observations only; it cannot authorize a workflow transition.
type Activity struct {
	Phase                string
	Status               string
	ElapsedMS            int64
	RemainingMS          *int64
	StdoutLog, StderrLog string
	Usage                *port.RunResult
	Err                  error
}

func WithActivityObserver(observe func(port.Task, Activity)) Option {
	return func(w *Worker) { w.activityObserver = observe }
}

func (w *Worker) activity(a Activity) {
	if w.activityObserver != nil {
		w.activityObserver(w.activeTask, a)
	}
}
