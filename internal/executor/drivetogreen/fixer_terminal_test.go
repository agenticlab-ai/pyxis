package drivetogreen

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestFixerStopsWithoutReplayingGateAndRetainsFailureUsage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result port.RunResult
		err    error
		want   string
	}{
		{"partial", port.RunResult{Tokens: 11, Incomplete: true, Reason: "permission refused"}, nil, "stopped before completion"},
		{"no progress", port.RunResult{Tokens: 12}, nil, "made no progress"},
		{"await failed", port.RunResult{Tokens: 13}, errors.New("cancelled"), "cancelled"},
		{"nonretryable", port.RunResult{Tokens: 14, Incomplete: true, NonRetryable: true, Reason: "HTTP 400"}, nil, "HTTP 400"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			rt := &fakeRuntime{result: tc.result, awaitErr: tc.err}
			d := mustDriver(t, Deps{Host: &fakeHost{}, Runtime: rt, Fetcher: &fakeFetcher{}, Pusher: &fakePusher{}, Runner: func(context.Context, string, []string) CommandResult {
				calls++
				return CommandResult{Output: "tests failed"}
			}, Revision: func(context.Context, port.Workdir) (string, error) { return "unchanged", nil }}, Config{GateChecks: gateChecks()})
			usage := 0
			d.BindUsageObserver(func(result port.RunResult) { usage += result.Tokens })
			work := &fakeWorkdir{dir: t.TempDir(), branch: "fix/task"}
			_, err := d.DriveLocal(t.Context(), port.Task{ID: "T1"}, work)
			if err == nil || !strings.Contains(err.Error(), tc.want) || usage != tc.result.Tokens || calls != len(gateChecks()) || len(rt.briefs) != 1 || !work.checkpointed {
				t.Fatalf("terminal=%v usage=%d gates=%d launches=%d checkpoint=%t", err, usage, calls, len(rt.briefs), work.checkpointed)
			}
			if tc.result.NonRetryable && !errors.Is(err, port.ErrRunNonRetryable) {
				t.Fatal("non-retryable classification was lost")
			}
		})
	}
}
