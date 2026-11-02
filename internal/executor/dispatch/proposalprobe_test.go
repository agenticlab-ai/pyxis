package dispatch

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
)

// This file exercises the pre-claim duplicate-dispatch guard (withholdProposed):
// a ready task that already carries an open proposal must never reach a claim, a
// worktree, or a model — no matter which scheduler invoked the sweep.
//
// The defect it guards is real: a Worker releases its claim on every terminal
// path including success, while the task's tracker item stays open until its
// proposal merges. In that window the task reads as open ∧ unblocked ∧ unclaimed,
// so any sweep restarts it — and because the re-open is idempotent on the task's
// deterministic head branch, the second Worker pushes fresh commits onto the
// proposal already under review rather than opening a second one.

// scriptedProbe answers from a canned map (task ID → open proposal); a task
// absent from it has no open proposal. A non-nil err fails the whole probe.
type scriptedProbe struct {
	open map[string]port.OpenProposal
	err  error

	calls atomic.Int32
	mu    sync.Mutex
	heads []string // head branches the probe was asked about
}

func (p *scriptedProbe) OpenProposals(_ context.Context, tasks []port.Task) (map[string]port.OpenProposal, error) {
	p.calls.Add(1)
	p.mu.Lock()
	for _, t := range tasks {
		p.heads = append(p.heads, t.HeadBranch)
	}
	p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	out := make(map[string]port.OpenProposal, len(p.open))
	maps.Copy(out, p.open)
	return out, nil
}

// acquired reports whether the lease was ever asked to claim task 928.
func acquired(ls *fakeLease) bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return slices.Contains(ls.acquired, 928)
}

func probeDispatcher(t *testing.T, probe port.ProposalProbe) (*Dispatcher, *fakeTracker, *fakeLease, *fakeRuntime) {
	t.Helper()
	tr, ls, ws, rt, hs := greenFakes(t)
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 2}, WithProposalProbe(probe))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d, tr, ls, rt
}

// An open proposal is work already done and awaiting its reviewers. Restarting
// the task cannot advance it, so the sweep withholds it before any claim — and
// an independent sibling with no proposal is untouched.
func TestSweep_OpenProposalIsWithheldBeforeClaim(t *testing.T) {
	probe := &scriptedProbe{open: map[string]port.OpenProposal{
		"T1": {Number: 960},
	}}
	d, tr, ls, rt := probeDispatcher(t, probe)
	tr.setFrontier(task("T1", 928), task("T2", 929))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := classOf(t, rep, 928); got != ClassAwaitingProposal {
		t.Fatalf("T1 class = %s, want awaiting-proposal", got)
	}
	if got := classOf(t, rep, 929); got != ClassSucceeded {
		t.Fatalf("T2 class = %s, want succeeded — a sibling with no proposal still runs", got)
	}
	if rep.AwaitingProposal != 1 {
		t.Fatalf("AwaitingProposal = %d, want 1", rep.AwaitingProposal)
	}
	if acquired(ls) {
		t.Fatal("T1 was claimed — the guard must withhold BEFORE the claim")
	}
	if got := rt.launches.Load(); got != 1 {
		t.Fatalf("model launches = %d, want 1 (T2 only) — a withheld task spends no model call", got)
	}
}

// EVERY open proposal is withheld, whatever its checks say. A red proposal is
// not evidence that its Worker died — far more often it exhausted its bounded
// self-heal, recorded the failure and released its claim, and restarting that
// resets the per-sweep bounds and retries forever across sweeps (the unbounded
// retry DINV-9 forbids). Resuming an open proposal needs an explicit recovery
// authorization, never an inference from check colour.
func TestSweep_EveryOpenProposalIsWithheldWhateverItsChecksSay(t *testing.T) {
	probe := &scriptedProbe{open: map[string]port.OpenProposal{"T1": {Number: 960}}}
	d, tr, ls, rt := probeDispatcher(t, probe)
	tr.setFrontier(task("T1", 928))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := classOf(t, rep, 928); got != ClassAwaitingProposal {
		t.Fatalf("T1 class = %s, want awaiting-proposal", got)
	}
	if acquired(ls) || rt.launches.Load() != 0 {
		t.Fatal("T1 was restarted — no open proposal is resumed without explicit recovery authorization")
	}
	if !rep.NothingReady() {
		t.Fatal("NothingReady = false — a sweep that only awaited proposals spawned no worker (DINV-8)")
	}
}

// A dispatch act requires the sweep's own positive evidence (AP-17). With the
// probe failing there is none, so the task is withheld — but as its OWN typed
// failure, never as healthy awaiting: it is counted, it reddens the run, and it
// carries the probe error. A code-host outage must never read as an idle sweep.
func TestSweep_ProbeErrorIsATypedFailureNotHealthyAwaiting(t *testing.T) {
	probe := &scriptedProbe{err: errors.New("gh: API rate limit exceeded")}
	d, tr, ls, rt := probeDispatcher(t, probe)
	tr.setFrontier(task("T1", 928))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v — a probe failure is classed, never fatal to the sweep", err)
	}
	if got := classOf(t, rep, 928); got != ClassProbeUnavailable {
		t.Fatalf("T1 class = %s, want probe-unavailable (NOT awaiting-proposal)", got)
	}
	if rep.AwaitingProposal != 0 {
		t.Fatalf("AwaitingProposal = %d, want 0 — an unreadable probe is not healthy awaiting", rep.AwaitingProposal)
	}
	if rep.ProbeUnavailable != 1 {
		t.Fatalf("ProbeUnavailable = %d, want 1", rep.ProbeUnavailable)
	}
	if rep.NothingReady() {
		t.Fatal("NothingReady = true — ready work WAS found and could not be judged; this must not look idle")
	}
	if acquired(ls) || rt.launches.Load() != 0 {
		t.Fatal("T1 was dispatched despite an unreadable probe — the guard must fail closed")
	}
	var reason string
	for _, res := range rep.Results {
		if res.Task.ID == "T1" && res.Err != nil {
			reason = res.Err.Error()
		}
	}
	if !strings.Contains(reason, "rate limit") {
		t.Fatalf("withheld reason = %q, want the probe error carried through", reason)
	}
}

// The probe-unavailable class must reach the unattended escalation surface, or
// a drive that withheld its whole frontier looks exactly like an idle one.
func TestProbeUnavailable_EscalatesOnTheEventStream(t *testing.T) {
	probe := &scriptedProbe{err: errors.New("gh: API rate limit exceeded")}
	tr, ls, ws, rt, hs := greenFakes(t)
	cap := &captureEmitter{}
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
		Config{Width: 2}, WithProposalProbe(probe), WithReporter(cap))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tr.setFrontier(task("T1", 928))

	if _, err := d.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	var escalated bool
	for _, e := range cap.snapshot() {
		if report.IsEscalation(e) {
			escalated = true
		}
	}
	if !escalated {
		t.Fatal("no escalation on the event stream — an unreadable preflight must surface to the human")
	}
}

// run_finished is the ONLY record an unattended drive leaves, so both preflight
// classes must appear in its canonical counts. A sweep whose whole ready set was
// withheld by an unreadable probe previously published a run_finished
// byte-identical to a run that found nothing ready — an outage indistinguishable
// from an idle sweep on the delivery journal (AP-16).
func TestRunFinishedCountsCarryBothPreflightClasses(t *testing.T) {
	for _, tc := range []struct {
		name              string
		probe             *scriptedProbe
		awaiting, unavail int
	}{
		{
			name:     "already proposed",
			probe:    &scriptedProbe{open: map[string]port.OpenProposal{"T1": {Number: 960}}},
			awaiting: 1,
		},
		{
			name:    "probe unreadable",
			probe:   &scriptedProbe{err: errors.New("gh: API rate limit exceeded")},
			unavail: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, ls, ws, rt, hs := greenFakes(t)
			cap := &captureEmitter{}
			d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
				Config{Width: 2}, WithProposalProbe(tc.probe), WithReporter(cap))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			tr.setFrontier(task("T1", 928))

			if _, err := d.Sweep(context.Background()); err != nil {
				t.Fatalf("Sweep: %v", err)
			}
			var counts *report.Counts
			for _, e := range cap.snapshot() {
				if e.Kind == report.KindRunFinished {
					counts = e.Counts
				}
			}
			if counts == nil {
				t.Fatal("no run_finished event carried counts")
			}
			if counts.AwaitingProposal != tc.awaiting {
				t.Errorf("run_finished awaiting_proposal = %d, want %d", counts.AwaitingProposal, tc.awaiting)
			}
			if counts.ProbeUnavailable != tc.unavail {
				t.Errorf("run_finished probe_unavailable = %d, want %d", counts.ProbeUnavailable, tc.unavail)
			}
			// The whole point: this must not read as an empty, healthy sweep.
			if counts.Succeeded+counts.Failed+counts.Blocked+counts.Skipped != 0 {
				t.Fatalf("counts = %+v, want the outcome carried ONLY by the new fields", counts)
			}
			if counts.AwaitingProposal+counts.ProbeUnavailable == 0 {
				t.Fatalf("run_finished = 0/0/0/0 — indistinguishable from a run that found nothing ready")
			}
		})
	}
}

// The class string dispatch emits and the literal the escalation reader matches
// are the round trip; pin them together so they cannot drift apart silently.
func TestProbeUnavailableClassStringMatchesTheEscalationLiteral(t *testing.T) {
	if got := ClassProbeUnavailable.String(); got != report.ClassProbeUnavailable {
		t.Fatalf("dispatch class string = %q, report literal = %q — these must match", got, report.ClassProbeUnavailable)
	}
}

// The probe must be asked about the very branch a Worker would open the
// proposal from, or it would answer about the wrong thing. The tracker leaves
// HeadBranch empty (both adapters do), so the guard derives it exactly as
// runOne does.
func TestSweep_ProbeIsAskedAboutTheWorkersHeadBranch(t *testing.T) {
	// The production case: both tracker adapters leave HeadBranch empty, so the
	// guard must derive the same branch runOne would.
	t.Run("derived when the tracker leaves it empty", func(t *testing.T) {
		probe := &scriptedProbe{}
		d, tr, _, _ := probeDispatcher(t, probe)
		tk := task("T1", 928)
		tk.HeadBranch = ""
		tr.setFrontier(tk)

		if _, err := d.Sweep(context.Background()); err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		want := headBranchFor(tk)
		probe.mu.Lock()
		got := append([]string(nil), probe.heads...)
		probe.mu.Unlock()
		if len(got) != 1 || got[0] != want {
			t.Fatalf("probed heads = %v, want exactly [%s] — the branch the Worker would propose from", got, want)
		}
	})

	// A charter that names its own head branch is honored, not overwritten.
	t.Run("honored when the task carries one", func(t *testing.T) {
		probe := &scriptedProbe{}
		d, tr, _, _ := probeDispatcher(t, probe)
		tk := task("T1", 928)
		tr.setFrontier(tk)

		if _, err := d.Sweep(context.Background()); err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		probe.mu.Lock()
		got := append([]string(nil), probe.heads...)
		probe.mu.Unlock()
		if len(got) != 1 || got[0] != tk.HeadBranch {
			t.Fatalf("probed heads = %v, want exactly [%s]", got, tk.HeadBranch)
		}
	})
}

// A blocked task never reaches the guard: readiness is decided first, so the
// probe is only ever asked about work the sweep would otherwise start.
func TestSweep_ProbeIsNotAskedAboutBlockedTasks(t *testing.T) {
	probe := &scriptedProbe{}
	d, tr, _, _ := probeDispatcher(t, probe)
	tr.setFrontier(task("T1", 928), task("T2", 929, 928)) // T2 blocked by open T1

	if _, err := d.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	probe.mu.Lock()
	got := append([]string(nil), probe.heads...)
	probe.mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("probed heads = %v, want only the one ready task", got)
	}
}

// Without a probe wired the Dispatcher behaves exactly as it did before this
// stage existed — every ready task proceeds to claim.
func TestSweep_NoProbeWiredLeavesDispatchUnchanged(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	d := mustDispatcher(t, 2, tr, ls, ws, rt, hs)
	tr.setFrontier(task("T1", 928))

	rep, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := classOf(t, rep, 928); got != ClassSucceeded {
		t.Fatalf("T1 class = %s, want succeeded", got)
	}
	if rep.AwaitingProposal != 0 {
		t.Fatalf("AwaitingProposal = %d, want 0 with no probe wired", rep.AwaitingProposal)
	}
	if !acquired(ls) {
		t.Fatal("T1 was not claimed — the base path must be untouched")
	}
}
