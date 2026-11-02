package ghcodehost

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// The pre-claim duplicate-dispatch probe (port.ProposalProbe): which candidate
// tasks already carry an open proposal on the branch a Worker would open one
// from? It deliberately reuses findOpenPRByHead — the same read OpenProposal
// uses for idempotency — so the probe and the opener can never disagree about
// what "already proposed" means, and it reads nothing else: existence alone
// decides, because check colour is not recovery evidence.

func probeTask(id, head string) port.Task {
	return port.Task{ID: id, Number: 1, Title: "task " + id, HeadBranch: head}
}

// Only tasks with an open proposal appear; a task with none is simply absent,
// which the Dispatcher reads as "safe to start".
func TestOpenProposals_ReportsOnlyTasksThatHaveOne(t *testing.T) {
	fc := &fakeClient{
		perHead: map[string]int{"feat/t1-1": 960},
		headSHA: "sha1",
		runs:    []checkRun{{Status: "completed", Conclusion: "success"}},
	}
	h := newHost(fc, "main")

	got, err := h.OpenProposals(context.Background(), []port.Task{
		probeTask("T1", "feat/t1-1"),
		probeTask("T2", "feat/t2-2"),
	})
	if err != nil {
		t.Fatalf("OpenProposals: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v, want exactly one entry (T1)", got)
	}
	p, ok := got["T1"]
	if !ok {
		t.Fatalf("got %+v, want an entry keyed by task ID T1", got)
	}
	if p.Number != 960 {
		t.Fatalf("T1 = %+v, want proposal 960", p)
	}
	if _, present := got["T2"]; present {
		t.Fatal("T2 has no open proposal and must be absent from the result")
	}
}

// The probe reports an open proposal whatever its checks say, and never reads
// them: check colour is not recovery evidence, so consulting it would only
// invite an inference the guard must not make.
func TestOpenProposals_ReportsRegardlessOfCheckColourAndReadsNoChecks(t *testing.T) {
	for _, tc := range []struct {
		name string
		runs []checkRun
	}{
		{"green", []checkRun{{Status: "completed", Conclusion: "success"}}},
		{"red", []checkRun{{Status: "completed", Conclusion: "failure"}}},
		{"pending", []checkRun{{Status: "in_progress"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{perHead: map[string]int{"feat/t1-1": 960}, headSHA: "sha1", runs: tc.runs}
			h := newHost(fc, "main")

			got, err := h.OpenProposals(context.Background(), []port.Task{probeTask("T1", "feat/t1-1")})
			if err != nil {
				t.Fatalf("OpenProposals: %v", err)
			}
			if got["T1"].Number != 960 {
				t.Fatalf("T1 = %+v, want proposal 960 reported whatever the checks say", got["T1"])
			}
			if fc.checkCalls != 0 {
				t.Fatalf("checkRuns called %d times, want 0 — the probe must not consult check colour", fc.checkCalls)
			}
		})
	}
}

// A frontier with nothing proposed costs one lookup per candidate and nothing
// more — the guard is cheap on the common path.
func TestOpenProposals_NoExtraReadsWhenNothingIsProposed(t *testing.T) {
	fc := &fakeClient{perHead: map[string]int{}}
	h := newHost(fc, "main")

	got, err := h.OpenProposals(context.Background(), []port.Task{
		probeTask("T1", "feat/t1-1"),
		probeTask("T2", "feat/t2-2"),
	})
	if err != nil {
		t.Fatalf("OpenProposals: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want an empty result", got)
	}
	if fc.checkCalls != 0 {
		t.Fatalf("checkRuns called %d times, want 0 — nothing was proposed to check", fc.checkCalls)
	}
	if len(fc.findHeads) != 2 {
		t.Fatalf("lookups = %v, want one per candidate", fc.findHeads)
	}
}

// A task with no head branch is reported unproposed rather than guessed at: the
// probe withholds on positive evidence, and an underived branch is none.
func TestOpenProposals_SkipsTaskWithNoHeadBranch(t *testing.T) {
	fc := &fakeClient{existingPR: 960, existingOK: true, headSHA: "sha1"}
	h := newHost(fc, "main")

	got, err := h.OpenProposals(context.Background(), []port.Task{probeTask("T1", "  ")})
	if err != nil {
		t.Fatalf("OpenProposals: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want an empty result", got)
	}
	if len(fc.findHeads) != 0 {
		t.Fatalf("lookups = %v, want none — there was no branch to ask about", fc.findHeads)
	}
}

// A lookup failure is surfaced, never silently reported as "nothing proposed" —
// which the Dispatcher would read as permission to dispatch.
func TestOpenProposals_LookupErrorIsSurfaced(t *testing.T) {
	fc := &fakeClient{findErr: errors.New("gh: API rate limit exceeded")}
	h := newHost(fc, "main")

	got, err := h.OpenProposals(context.Background(), []port.Task{probeTask("T1", "feat/t1-1")})
	if err == nil {
		t.Fatalf("OpenProposals returned %+v and no error, want the lookup failure surfaced", got)
	}
	if got != nil {
		t.Fatalf("got %+v on error, want a nil map", got)
	}
}

// An unreadable checks API cannot affect the probe at all — it never asks.
func TestOpenProposals_UnreadableChecksAreIrrelevant(t *testing.T) {
	fc := &fakeClient{perHead: map[string]int{"feat/t1-1": 960}, headSHA: "sha1", runsErr: errors.New("checks API down")}
	h := newHost(fc, "main")

	got, err := h.OpenProposals(context.Background(), []port.Task{probeTask("T1", "feat/t1-1")})
	if err != nil {
		t.Fatalf("OpenProposals: %v — a checks outage must not affect a probe that reads no checks", err)
	}
	if got["T1"].Number != 960 {
		t.Fatalf("T1 = %+v, want proposal 960", got["T1"])
	}
}
