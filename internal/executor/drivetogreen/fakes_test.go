package drivetogreen

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

// The fakes implement the ports the self-heal loop consumes, in-process, with no
// network, git or CLI (§3.10 — the substrate-first hermetic harness). Compile-time
// assertions pin them to the port contracts so signature drift is a build failure.
var (
	_ port.CodeHost     = (*fakeHost)(nil)
	_ port.AgentRuntime = (*fakeRuntime)(nil)
	_ port.Workdir      = (*fakeWorkdir)(nil)
	_ port.Handle       = fakeHandle{}
	_ LogFetcher        = (*fakeFetcher)(nil)
	_ Pusher            = (*fakePusher)(nil)
)

// --- fake Workdir -------------------------------------------------------------

type fakeWorkdir struct {
	dir           string // a real t.TempDir(): the gate and the fixer read/write it
	branch        string
	checkpointErr error
	checkpointed  bool
}

func (w *fakeWorkdir) Dir() string    { return w.dir }
func (w *fakeWorkdir) Branch() string { return w.branch }
func (w *fakeWorkdir) Checkpoint(context.Context) (bool, error) {
	w.checkpointed = true
	return true, w.checkpointErr
}
func (w *fakeWorkdir) Push(context.Context) (bool, error) { return true, nil }
func (w *fakeWorkdir) Cleanup(context.Context) error      { return nil }

// --- fake CodeHost (only Checks is exercised) ---------------------------------

// fakeHost returns required-check states from a scripted queue: the i-th Checks
// call yields states[i], and the last entry repeats once the queue is drained, so
// a test scripts a red→…→green (or red-forever) sequence.
type fakeHost struct {
	states       []port.CheckState
	calls        int
	checkErr     error
	observations []proposalObservation
	inspectCalls int
	inspectErr   error
}

func (h *fakeHost) InspectProposal(context.Context, port.Proposal) (port.Landing, string, port.CheckState, error) {
	if h.inspectErr != nil {
		return port.LandingUnknown, "", port.CheckPending, h.inspectErr
	}
	i := h.inspectCalls
	h.inspectCalls++
	var observed proposalObservation
	switch {
	case len(h.observations) == 0:
		observed = proposalObservation{landing: port.LandingOpen, checks: port.CheckRed}
	case i >= len(h.observations):
		observed = h.observations[len(h.observations)-1]
	default:
		observed = h.observations[i]
	}
	return observed.landing, observed.head, observed.checks, nil
}

func (h *fakeHost) OpenProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (port.Proposal, error) {
	return port.Proposal{}, nil // not used by the loop
}

func (h *fakeHost) Checks(context.Context, port.Proposal) (port.CheckState, error) {
	if h.checkErr != nil {
		return port.CheckPending, h.checkErr
	}
	i := h.calls
	h.calls++
	switch {
	case len(h.states) == 0:
		return port.CheckRed, nil
	case i >= len(h.states):
		return h.states[len(h.states)-1], nil
	default:
		return h.states[i], nil
	}
}

// --- fake AgentRuntime --------------------------------------------------------

// fakeRuntime records each fixer brief (so a test can assert the diagnostic was
// redacted before the fixer saw it) and lets the test run a side effect per launch
// (e.g. the fixer "fixing" the tree by writing a marker the gate then reads).
type fakeRuntime struct {
	briefs    []string
	onLaunch  func(spec port.LaunchSpec)
	launchErr error
	awaitErr  error
	result    port.RunResult
}

func (r *fakeRuntime) Launch(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
	r.briefs = append(r.briefs, spec.Brief)
	if r.onLaunch != nil {
		r.onLaunch(spec)
	}
	if r.launchErr != nil {
		return nil, r.launchErr
	}
	return fakeHandle{}, nil
}

func (r *fakeRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	return r.result, r.awaitErr
}

func (r *fakeRuntime) Kill(context.Context, port.Handle) error { return nil }
func (r *fakeRuntime) AuthCheck(context.Context) error         { return nil }
func (r *fakeRuntime) ModelRoute() port.ModelRoute {
	return port.ModelRoute{Model: "fake-model", Effort: "fake-effort"}
}

type fakeHandle struct{}

func (fakeHandle) RuntimeHandle() {}

// --- fake LogFetcher ----------------------------------------------------------

// fakeFetcher returns scripted failed-CI logs per call; the last entry repeats.
type fakeFetcher struct {
	logs  []string
	calls int
	err   error
}

func (f *fakeFetcher) FetchFailedLogs(context.Context, port.Proposal) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	i := f.calls
	f.calls++
	switch {
	case len(f.logs) == 0:
		return "", nil
	case i >= len(f.logs):
		return f.logs[len(f.logs)-1], nil
	default:
		return f.logs[i], nil
	}
}

// --- fake Pusher --------------------------------------------------------------

type fakePusher struct {
	pushes int
	err    error
	// heads are the commits successive pushes publish. An exhausted (or empty)
	// list reports no head, which is the documented "pusher cannot name its
	// commit" fallback — the pre-I10 first-observation fence.
	heads []string
}

func (p *fakePusher) Push(context.Context, port.Workdir) (string, error) {
	p.pushes++
	if p.err != nil {
		return "", p.err
	}
	if len(p.heads) == 0 {
		return "", nil
	}
	head := p.heads[0]
	p.heads = p.heads[1:]
	return head, nil
}

func (h *fakeHost) DescribeProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (bool, error) {
	return false, nil
}

func (w *fakeWorkdir) ChangeSize(context.Context) (changesize.Size, error) {
	return changesize.Size{}, nil
}
