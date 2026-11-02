package worker

import (
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestNonRetryableProviderFailurePreservesWorkWithoutRespawn(t *testing.T) {
	deps, _, lease, space, runtime, host := greenDeps(t)
	runtime.result = port.RunResult{Tokens: 17, Incomplete: true, NonRetryable: true, Reason: "APIError: HTTP 400: Select Global"}
	w := mustWorker(t, deps)
	outcome, err := w.Run(t.Context(), newTestTask())
	if outcome != OutcomeFailed || !errors.Is(err, port.ErrRunNonRetryable) || !strings.Contains(err.Error(), "Select Global") || w.Usage().Tokens != 17 || w.VerificationAttempted() {
		t.Fatalf("outcome=%s err=%v usage=%+v", outcome, err, w.Usage())
	}
	if space.last.checkpoints != 1 || space.last.pushes != 0 || len(host.openedTasks) != 0 || lease.claim.releases != 1 {
		t.Fatal("refused run was published or not checkpointed/released")
	}
}
