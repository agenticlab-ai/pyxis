package dispatch

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/prdescription/prdescriptiontest"
)

// scriptedRuntime drives the bounded-execution + resume tests. Each Launch is one
// Worker attempt; the leading `blockAttempts` attempts park in Await until Kill —
// a non-converging worker that hits its wall-clock bound — while later attempts
// return promptly (a converged run resuming from the pushed WIP). Each handle
// carries its own kill channel, so respawns (fresh Launches) get independent
// bound-kills. It counts launches and kills, and can signal Await entry through
// `entered` for a deterministic mid-run cancel.
type scriptedRuntime struct {
	blockAttempts int           // leading attempts that block until Kill
	entered       chan struct{} // optional Await-entry barrier (non-blocking send)

	mu       sync.Mutex
	launched int
	killed   atomic.Int32
}

type scriptedHandle struct {
	kill  chan struct{}
	once  *sync.Once
	block bool
}

func (scriptedHandle) RuntimeHandle() {}

func (r *scriptedRuntime) AuthCheck(context.Context) error { return nil }

func (r *scriptedRuntime) ModelRoute() port.ModelRoute {
	return port.ModelRoute{Model: "fake-model", Effort: "fake-effort"}
}

func (r *scriptedRuntime) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	r.mu.Lock()
	r.launched++
	block := r.launched <= r.blockAttempts
	r.mu.Unlock()
	return scriptedHandle{kill: make(chan struct{}), once: &sync.Once{}, block: block}, nil
}

func (r *scriptedRuntime) Await(ctx context.Context, h port.Handle) (port.RunResult, error) {
	hd := h.(scriptedHandle)
	if !hd.block {
		return port.RunResult{Tokens: 1, Turns: 1, Output: prdescriptiontest.Message}, nil // converged
	}
	if r.entered != nil {
		select {
		case r.entered <- struct{}{}:
		default:
		}
	}
	select {
	case <-hd.kill:
		return port.RunResult{}, nil
	case <-ctx.Done():
		// A real Await honors ctx by terminating the process before returning
		// (CommandContext semantics), so "Await returned on cancel" implies the
		// process is dead — the contract the Worker's <-done path relies on. Model
		// it here; without it the kill is observed only when the Worker's own
		// ctx.Done case wins a scheduling race (incident I5).
		r.terminate(hd)
		return port.RunResult{}, ctx.Err()
	}
}

// terminate records the (idempotent) end of a scripted run: the single place that
// bumps the kill counter and unblocks a parked Await. Both the Worker's explicit
// Kill and a context cancellation converge here, so the "who terminated it" race
// (the Worker's Kill vs Await's own ctx.Done) can never change the observable kill
// count — a test whose verdict depended on that race was non-hermetic (incident
// I5). Idempotent per handle via the shared *sync.Once.
func (r *scriptedRuntime) terminate(hd scriptedHandle) {
	hd.once.Do(func() {
		r.killed.Add(1)
		close(hd.kill)
	})
}

func (r *scriptedRuntime) Kill(_ context.Context, h port.Handle) error {
	r.terminate(h.(scriptedHandle))
	return nil
}

func (r *scriptedRuntime) launchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.launched
}

// idempotentHost proves OpenProposal is idempotent on a task's deterministic head
// branch: a second open for the same branch returns the SAME proposal and creates
// no new one, so a re-invocation (or a respawn) yields zero duplicate proposals
// (AP-22, prd:implement-now/FR-14). It tracks opens per branch and the count of
// distinct proposals ever created. Checks are green by default; a per-branch
// override lets a test model an in-flight (not-yet-green) run.
type idempotentHost struct {
	mu         sync.Mutex
	prByBranch map[string]port.Proposal
	opens      map[string]int
	check      map[string]port.CheckState
	created    int
	nextNum    int
}

func newIdempotentHost() *idempotentHost {
	return &idempotentHost{
		prByBranch: map[string]port.Proposal{},
		opens:      map[string]int{},
		check:      map[string]port.CheckState{},
	}
}

func (h *idempotentHost) OpenProposal(_ context.Context, _ port.Task, work port.Workdir, _ prdescription.Description) (port.Proposal, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	br := work.Branch()
	h.opens[br]++
	if p, ok := h.prByBranch[br]; ok {
		return p, nil // idempotent reuse — the existing PR, never a duplicate
	}
	h.nextNum++
	h.created++
	p := port.Proposal{Number: h.nextNum, Branch: br}
	h.prByBranch[br] = p
	return p, nil
}

func (h *idempotentHost) Checks(_ context.Context, p port.Proposal) (port.CheckState, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if st, ok := h.check[p.Branch]; ok {
		return st, nil
	}
	return port.CheckGreen, nil
}

func (h *idempotentHost) setCheck(branch string, st port.CheckState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.check[branch] = st
}

func (h *idempotentHost) stats(branch string) (created, opens int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.created, h.opens[branch]
}

var (
	_ port.AgentRuntime = (*scriptedRuntime)(nil)
	_ port.Handle       = scriptedHandle{}
	_ port.CodeHost     = (*idempotentHost)(nil)
)

// --- A-7: turn bound → respawn up to the cap → escalate -----------------------

// A Worker that never converges (its Await parks until the wall-clock bound kills
// it) is respawned up to the cap, then escalated: the task is classed failed and
// carries ErrRespawnCapExhausted, the unconverged outcome is preserved, and the
// bound fired on every attempt (1 initial + cap respawns). Each attempt claimed
// and released (fire-or-release) — the run stays recoverable throughout (DINV-9,
// AP-16, prd:implement-now/FR-13).
func TestSweep_UnconvergedRespawnsThenEscalates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const cap = 2
		tr := &fakeTracker{}
		ls := &fakeLease{claimErr: map[int]error{}}
		ws := &fakeWorkspace{dir: t.TempDir()}
		rt := &scriptedRuntime{blockAttempts: 1000} // every attempt hits the bound
		hs := newIdempotentHost()
		d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
			Config{Width: 1, RespawnCap: cap, WorkerBudget: 30 * time.Minute})
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		tr.setFrontier(task("T1", 1))
		rep, err := d.Sweep(context.Background())
		if err != nil {
			t.Fatalf("sweep: %v", err)
		}

		if rep.Failed != 1 || rep.Succeeded != 0 || rep.Blocked != 0 || rep.Skipped != 0 {
			t.Fatalf("counts = %+v, want 1 failed (escalated)", rep)
		}
		r := rep.Results[0]
		if r.Class != ClassFailed {
			t.Errorf("class = %v, want failed", r.Class)
		}
		if r.Outcome != worker.OutcomeUnconverged {
			t.Errorf("outcome = %v, want unconverged preserved on the escalated result", r.Outcome)
		}
		if !errors.Is(r.Err, ErrRespawnCapExhausted) {
			t.Errorf("err = %v, want it to wrap ErrRespawnCapExhausted", r.Err)
		}
		// The bound was enforced on every attempt: 1 initial + cap respawns.
		if got := rt.launchCount(); got != cap+1 {
			t.Errorf("launches = %d, want %d (1 initial + %d respawns)", got, cap+1, cap)
		}
		if got := rt.killed.Load(); got != int32(cap+1) {
			t.Errorf("kills = %d, want %d (each attempt hit its bound)", got, cap+1)
		}
		// Recoverable across respawns: one acquire+release per attempt, and — because a
		// bound-hit exits before verify — zero proposals opened (zero duplicates).
		if got := ls.acquiredCount(); got != cap+1 {
			t.Errorf("claims acquired = %d, want %d (one per attempt)", got, cap+1)
		}
		if got := ls.releases.Load(); got != int32(cap+1) {
			t.Errorf("claims released = %d, want %d (fire-or-release per attempt)", got, cap+1)
		}
		if created, _ := hs.stats(task("T1", 1).HeadBranch); created != 0 {
			t.Errorf("proposals created = %d, want 0 (a bound-hit exits before verify)", created)
		}

	})
}

// A non-converging first attempt that converges on respawn succeeds — the
// cumulative-progress path: the respawn resumes from the pushed WIP and reaches a
// green proposal. Only the first (bounded) attempt is killed, and exactly one
// proposal is opened (zero duplicates) (DINV-9, AP-16, AP-22).
func TestSweep_UnconvergedThenConvergesOnRespawn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := &fakeTracker{}
		ls := &fakeLease{claimErr: map[int]error{}}
		ws := &fakeWorkspace{dir: t.TempDir()}
		rt := &scriptedRuntime{blockAttempts: 1} // attempt 1 bounded; attempt 2 converges
		hs := newIdempotentHost()
		d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
			Config{Width: 1, RespawnCap: 2, WorkerBudget: 30 * time.Minute})
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		tr.setFrontier(task("T1", 1))
		rep, err := d.Sweep(context.Background())
		if err != nil {
			t.Fatalf("sweep: %v", err)
		}
		if rep.Succeeded != 1 || rep.Failed != 0 {
			t.Fatalf("counts = %+v, want 1 succeeded (converged on respawn)", rep)
		}
		if got := rt.launchCount(); got != 2 {
			t.Errorf("launches = %d, want 2 (1 bounded + 1 converging respawn)", got)
		}
		if got := rt.killed.Load(); got != 1 {
			t.Errorf("kills = %d, want 1 (only the first, bounded attempt was killed)", got)
		}
		if created, _ := hs.stats(task("T1", 1).HeadBranch); created != 1 {
			t.Errorf("proposals created = %d, want 1 (the converging attempt, no duplicate)", created)
		}

	})
}

// --- A-8: re-invocation resumes in-flight tasks, zero duplicate proposals ------

// A killed mid-run leaves an in-flight task: its proposal is opened on the pushed
// branch but not yet green. A re-invocation (a fresh Sweep — the executor never
// merges, so the task is still in the frontier) resumes from the SAME branch,
// OpenProposal returns the existing proposal, and the task converges to green.
// Exactly one proposal is ever created across the two sweeps — zero duplicates
// (AP-22, prd:implement-now/FR-14).
func TestSweep_ReinvocationResumesWithNoDuplicateProposal(t *testing.T) {
	tr := &fakeTracker{}
	ls := &fakeLease{claimErr: map[int]error{}}
	ws := &fakeWorkspace{dir: t.TempDir()}
	rt := &fakeRuntime{} // returns promptly; the resume re-runs the agent on the branch
	hs := newIdempotentHost()
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tk := task("T1", 1) // deterministic HeadBranch

	// Sweep 1 — in flight: the proposal is opened on the pushed branch but not yet
	// green (a run interrupted before it converged). The task stays open.
	hs.setCheck(tk.HeadBranch, port.CheckRed)
	tr.setFrontier(tk)
	rep1, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep 1: %v", err)
	}
	if rep1.Failed != 1 || rep1.Succeeded != 0 {
		t.Fatalf("sweep 1 counts = %+v, want 1 failed (in flight, not yet green)", rep1)
	}

	// Sweep 2 — the re-invocation: the branch now goes green. The Worker resumes
	// from the SAME pushed branch; OpenProposal returns the existing proposal.
	hs.setCheck(tk.HeadBranch, port.CheckGreen)
	tr.setFrontier(tk)
	rep2, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep 2: %v", err)
	}
	if rep2.Succeeded != 1 || rep2.Failed != 0 {
		t.Fatalf("sweep 2 counts = %+v, want 1 succeeded (resumed to green)", rep2)
	}

	created, opens := hs.stats(tk.HeadBranch)
	if created != 1 {
		t.Fatalf("proposals created = %d across the re-invocation, want 1 (no duplicate)", created)
	}
	if opens != 2 {
		t.Errorf("OpenProposal calls for %s = %d, want 2 (once per sweep, idempotent reuse)", tk.HeadBranch, opens)
	}
}

// A run killed mid-flight (its context cancelled while the agent is in Await)
// terminates the process and releases the claim, leaving the task recoverable for
// a re-invocation — the resumability half of FR-14 (fire-or-release, DINV-5).
func TestSweep_CancelledMidRunKillsAndReleases(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := &fakeTracker{}
		ls := &fakeLease{claimErr: map[int]error{}}
		ws := &fakeWorkspace{dir: t.TempDir()}
		rt := &scriptedRuntime{blockAttempts: 1, entered: make(chan struct{}, 1)}
		hs := newIdempotentHost()
		d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs},
			Config{Width: 1, WorkerBudget: time.Minute}) // budget won't fire; the cancel wins
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		tr.setFrontier(task("T1", 1))

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan RunReport, 1)
		go func() {
			rep, err := d.Sweep(ctx)
			if err != nil {
				t.Errorf("sweep: %v", err)
			}
			done <- rep
		}()

		<-rt.entered // the agent is in flight
		cancel()
		rep := <-done

		if got := rt.killed.Load(); got != 1 {
			t.Errorf("Kill count = %d, want 1 (cancellation kills the in-flight process)", got)
		}
		if got := ls.releases.Load(); got != 1 {
			t.Errorf("claims released = %d, want 1 (fire-or-release on cancellation)", got)
		}
		if rep.Succeeded != 0 {
			t.Errorf("succeeded = %d, want 0 (the run was cancelled mid-flight)", rep.Succeeded)
		}
		// Nothing was proposed on the cancelled attempt — no duplicate awaits a resume.
		if created, _ := hs.stats(task("T1", 1).HeadBranch); created != 0 {
			t.Errorf("proposals created = %d, want 0 (cancelled before verify)", created)
		}

	})
}

// --- config: the respawn cap defaults + is honored ----------------------------

func TestNew_RespawnCapDefaultsWhenUnset(t *testing.T) {
	full := Deps{
		Tracker: &fakeTracker{}, Lease: &fakeLease{}, Space: &fakeWorkspace{},
		Runtime: &fakeRuntime{}, Host: &fakeHost{},
	}
	for _, c := range []int{0, -2} {
		d, err := New(full, Config{RespawnCap: c})
		if err != nil {
			t.Fatalf("New(respawnCap=%d): %v", c, err)
		}
		if d.respawnCap != DefaultRespawnCap {
			t.Errorf("respawnCap for cfg %d = %d, want default %d", c, d.respawnCap, DefaultRespawnCap)
		}
	}
	d, err := New(full, Config{RespawnCap: 5})
	if err != nil {
		t.Fatalf("New(respawnCap=5): %v", err)
	}
	if d.respawnCap != 5 {
		t.Errorf("respawnCap = %d, want 5", d.respawnCap)
	}
}

func (h *idempotentHost) DescribeProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (bool, error) {
	return false, nil
}
