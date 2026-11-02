package worker

import "github.com/agenticlab-ai/pyxis/internal/executor/port"

type LocalGateResult struct {
	Green      bool
	Diagnostic string // bounded and redacted by the gate owner
}

func WithRecoveryDiagnostic(diagnostic string) Option {
	return func(w *Worker) { w.recoveryDiagnostic = diagnostic }
}

func (w *Worker) RecoveryDiagnostic() string { return w.recoveryDiagnostic }

// Numeric usage is additive; availability is complete only if every launched
// process reported the field. An interrupted window is not a measured zero.
func (w *Worker) addUsage(res port.RunResult) {
	if !w.usageSeen {
		w.usage.TokensObserved, w.usage.TurnsObserved, w.usage.ToolCallsObserved = res.TokensObserved, res.TurnsObserved, res.ToolCallsObserved
		w.usageSeen = true
	} else {
		w.usage.TokensObserved = w.usage.TokensObserved && res.TokensObserved
		w.usage.TurnsObserved = w.usage.TurnsObserved && res.TurnsObserved
		w.usage.ToolCallsObserved = w.usage.ToolCallsObserved && res.ToolCallsObserved
	}
	w.usage.Tokens += res.Tokens
	w.usage.Turns += res.Turns
	w.usage.ToolCalls += res.ToolCalls
}
