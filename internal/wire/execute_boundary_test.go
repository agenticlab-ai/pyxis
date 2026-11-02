package wire

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func boundaryExecutor(runner Runner, sleeper Sleeper) *executor {
	return &executor{
		runner: runner,
		log:    discardLogger(),
		sleep:  sleeper,
		now:    tick(),
		runID:  "boundary-run",
	}
}

func TestContextSleepAndDefaultRunIDBoundaries(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ctxSleep(cancelled, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled sleep = %v", err)
	}
	if err := ctxSleep(context.Background(), 0); err != nil {
		t.Fatalf("completed sleep = %v", err)
	}

	audit, err := Execute(context.Background(), nil, nil, WithLogger(discardLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if audit.RunID == "" {
		t.Fatal("default execution omitted its run identity")
	}
	_, err = Execute(context.Background(), []Op{{Name: OpCreateRepo, Transport: TransportGH, Slug: testSlug}}, nil,
		WithLogger(discardLogger()), WithRunID("boundary-run"))
	if err == nil || !strings.Contains(err.Error(), "nil Runner") {
		t.Fatalf("nil runner error = %v", err)
	}
}

func TestFailureSummaryAndPreflightBoundaries(t *testing.T) {
	if got := failureSummary(Audit{}, []string{"missing-row"}); got != "missing-row failed" {
		t.Fatalf("failureSummary = %q", got)
	}

	t.Run("transport error", func(t *testing.T) {
		runner := newFake(t).onErr(1, errors.New("transport unavailable"), "--version")
		err := boundaryExecutor(runner, func(context.Context, time.Duration) error { return nil }).preflight(context.Background())
		if err == nil || !strings.Contains(err.Error(), "gh cannot be run") {
			t.Fatalf("preflight error = %v", err)
		}
	})
	t.Run("nonzero exit", func(t *testing.T) {
		runner := newFake(t).on(1, "", "broken gh", 9, "--version")
		err := boundaryExecutor(runner, func(context.Context, time.Duration) error { return nil }).preflight(context.Background())
		if err == nil || !strings.Contains(err.Error(), "exited 9") {
			t.Fatalf("preflight error = %v", err)
		}
	})
}

func TestRunOpRejectsContradictoryTransports(t *testing.T) {
	x := boundaryExecutor(nil, func(context.Context, time.Duration) error { return nil })
	for _, op := range []Op{
		{Name: OpPushWorkflow, Transport: TransportGH},
		{Name: OpCreateRepo, Transport: TransportConverge, Slug: testSlug},
	} {
		state := &walkState{}
		result, err := x.runOp(context.Background(), op, state)
		if err != nil || result.Outcome != OutcomeFailed || len(state.failed) != 1 {
			t.Fatalf("runOp(%s) = %+v, state=%+v err=%v", op.Name, result, state, err)
		}
	}
}

func TestFreshRepositoryProbeAbortAndInterruptedBackoff(t *testing.T) {
	t.Run("transport abort", func(t *testing.T) {
		runner := newFake(t).onErr(1, errors.New("transport died"), "probe")
		x := boundaryExecutor(runner, func(context.Context, time.Duration) error { return nil })
		if _, err := x.probeFreshRepo(context.Background(), &OpResult{Name: "probe"}, &walkState{}, []string{"probe"}); err == nil {
			t.Fatal("transport abort was ignored")
		}
	})
	t.Run("sleep interruption", func(t *testing.T) {
		runner := newFake(t).on(-1, "", "", 0, "probe")
		sleepErr := errors.New("sleep interrupted")
		x := boundaryExecutor(runner, func(context.Context, time.Duration) error { return sleepErr })
		if _, err := x.probeFreshRepo(context.Background(), &OpResult{Name: "probe"}, &walkState{repoCreated: true}, []string{"probe"}); !errors.Is(err, sleepErr) {
			t.Fatalf("probe error = %v", err)
		}
	})
	if reason := freshRepoLagWhy(stepOutcome{attempts: maxAttempts}); !strings.Contains(reason, "empty body") {
		t.Fatalf("freshRepoLagWhy = %q", reason)
	}
}

func TestCreateRepositoryFailureClassBoundaries(t *testing.T) {
	const notFound = "GraphQL: Could not resolve to a Repository with the name 'owner/repo'."
	op := Op{Name: OpCreateRepo, Transport: TransportGH, Slug: testSlug}
	cases := []struct {
		name         string
		runner       func(*testing.T) *fakeRunner
		wantOutcome  Outcome
		wantCreated  bool
		wantDeferred bool
	}{
		{
			name: "benign create race",
			runner: func(t *testing.T) *fakeRunner {
				return newFake(t).
					on(-1, "", notFound, 1, "repo view").
					on(-1, "", "already exists", 1, "repo create")
			},
			wantOutcome: OutcomeApplied,
			wantCreated: true,
		},
		{
			name: "transient create",
			runner: func(t *testing.T) *fakeRunner {
				return newFake(t).
					on(-1, "", notFound, 1, "repo view").
					on(-1, "", "HTTP 500 internal server error", 1, "repo create")
			},
			wantOutcome:  OutcomeHandedOff,
			wantDeferred: true,
		},
		{
			name: "unclassified create",
			runner: func(t *testing.T) *fakeRunner {
				return newFake(t).
					on(-1, "", notFound, 1, "repo view").
					on(-1, "", "unexpected create failure", 1, "repo create")
			},
			wantOutcome:  OutcomeFailed,
			wantDeferred: true,
		},
		{
			name: "permission probe",
			runner: func(t *testing.T) *fakeRunner {
				return newFake(t).on(-1, "", "HTTP 403 forbidden", 1, "repo view")
			},
			wantOutcome:  OutcomeHandedOff,
			wantDeferred: true,
		},
		{
			name: "transient probe",
			runner: func(t *testing.T) *fakeRunner {
				return newFake(t).on(-1, "", "HTTP 500 internal server error", 1, "repo view")
			},
			wantOutcome:  OutcomeHandedOff,
			wantDeferred: true,
		},
		{
			name: "unclassified probe",
			runner: func(t *testing.T) *fakeRunner {
				return newFake(t).on(-1, "", "unexpected probe failure", 1, "repo view")
			},
			wantOutcome:  OutcomeFailed,
			wantDeferred: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := &walkState{}
			result := &OpResult{Name: op.Name, Transport: op.Transport}
			x := boundaryExecutor(tc.runner(t), func(context.Context, time.Duration) error { return nil })
			if err := x.opCreateRepo(context.Background(), op, result, state); err != nil {
				t.Fatal(err)
			}
			if result.Outcome != tc.wantOutcome || state.repoCreated != tc.wantCreated || state.repoDeferred != tc.wantDeferred {
				t.Fatalf("result=%+v state=%+v", result, state)
			}
		})
	}
}
