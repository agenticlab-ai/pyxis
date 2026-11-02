package codex

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestInterruptedAwaitRetainsSessionAndAvailableUsage(t *testing.T) {
	for _, tc := range []struct {
		name         string
		stream       string
		cancelled    bool
		wantTokens   int
		wantObserved bool
	}{
		{"cancelled before first turn", `{"type":"thread.started","thread_id":"resume-me"}`, true, 0, false},
		{"signalled before first turn", `{"type":"thread.started","thread_id":"resume-me"}`, false, 0, false},
		{"cancelled with usage", streamJSONL, true, 1234, true},
		{"signalled with usage", streamJSONL, false, 1234, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fault := errors.New("process interrupted")
			p := &fakeProcess{stdout: []byte(tc.stream), waitErr: fault}
			a := mustAdapter(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			res, err := a.Await(ctx, &handle{proc: p})
			if err == nil || (tc.cancelled && !errors.Is(err, context.Canceled)) || (!tc.cancelled && !errors.Is(err, fault)) {
				t.Fatalf("interruption lost its cause: %v", err)
			}
			wantID := "resume-me"
			if tc.wantObserved {
				wantID = "t_abc"
			}
			if res.SessionID != wantID || res.Tokens != tc.wantTokens || res.TokensObserved != tc.wantObserved || res.TurnsObserved != tc.wantObserved {
				t.Fatalf("interruption lost observed metadata: %+v", res)
			}
			if !p.killed || !p.waited {
				t.Fatal("interruption was not acknowledged")
			}
		})
	}
}

func TestPartialStreamStillFailsClosed(t *testing.T) {
	a := mustAdapter(t)
	res, err := a.Await(t.Context(), &handle{proc: &fakeProcess{stdout: []byte(`{"type":"thread.started","thread_id":"resume-me"}`)}})
	if err == nil || res.SessionID != "resume-me" || res.TokensObserved || res.TurnsObserved || res == (port.RunResult{}) {
		t.Fatalf("partial stream became success or lost continuity: %+v, %v", res, err)
	}
}
