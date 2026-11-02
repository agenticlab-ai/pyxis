package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/agenticlab-ai/pyxis/internal/executor/drivetogreen"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/prdescription/prdescriptiontest"
)

// The stubs stand at TWO boundaries only — the ones that are genuinely external
// and cannot run in-session:
//
//   - GitHub. memTracker stands at the issue API (WorkTracker) and gitHost at the
//     pull-request and Checks API (CodeHost). Neither invents state: gitHost
//     answers every question by reading the REAL bare origin, so a proposal
//     exists only if a branch was really pushed and reads green only if the
//     pushed head really carries green work.
//   - The coding-agent CLI. scriptRuntime stands at AgentRuntime — the model
//     call, the one thing no hermetic run can make. It "implements" by writing
//     and COMMITTING a file inside its own real worktree, exactly where a real
//     headless CLI leaves its diff.
//
// Everything else is the production adapter running for real: the git-worktree
// Workspace, the git-ref CAS Lease (both in gitworld_test.go) and the git Pusher
// below. Compile-time assertions pin every stub to its port contract, so any
// signature drift is a build failure here.
var (
	_ port.WorkTracker               = (*memTracker)(nil)
	_ port.AgentRuntime              = (*scriptRuntime)(nil)
	_ port.Handle                    = scriptHandle{}
	_ port.CodeHost                  = (*gitHost)(nil)
	_ drivetogreen.ProposalInspector = (*gitHost)(nil)
	_ drivetogreen.Pusher            = (*gitPusher)(nil)
	_ drivetogreen.LogFetcher        = cannedLogFetcher("")
	_ drivetogreen.CommandRunner     = fileGateRunner
)

// implFile is the artifact a task's fresh agent process writes and commits into
// its worktree — the file the local gate checks, the Worker publishes, and the
// code host reads back off the pushed head. It makes verify-by-artifact (DINV-3)
// a real data-flow through git, not a scripted boolean.
const implFile = "impl.txt"

// fixerMarker is the sentinel drivetogreen.fixBrief stamps onto a self-heal
// fixer's brief. The stub runtime keys off it to tell an INITIAL implement launch
// from a drive-to-green FIXER launch — the way a real agent would read its
// charter and know it is being asked to repair red checks (§3.6 step 3).
const fixerMarker = "required checks are red"

// prBase offsets minted proposal (PR) numbers away from tracker issue numbers, so
// a proposal number is never confused with the issue it was opened for.
const prBase = 1000

// tagRE recovers the fixture directive a task's brief carries: [[e2e:<id>:<mode>]]
// where mode ∈ {green, hollow, redfix}. The tag survives into a fixer's brief
// (fixBrief prefixes the original charter), so a resumed/fixer launch still reads
// the task's mode.
var tagRE = regexp.MustCompile(`\[\[e2e:([^:\]]+):([^\]]+)\]\]`)

// --- GitHub stub 1: the issue tracker -------------------------------------------

// memTracker is the stub at the GitHub issue API: it holds the ready frontier the
// Dispatcher reads and records progress comments. `land` models the driver
// landing a blocker between sweeps (the blocker leaves the open set, so its
// dependents become ready on the next sweep — §3.6 step 5).
type memTracker struct {
	mu       sync.Mutex
	frontier []port.Task
	comments []trackerComment
	reads    atomic.Int32 // Frontier call count — a re-invocation re-probes it
}

type trackerComment struct {
	issue int
	body  string
}

func (t *memTracker) setFrontier(tasks ...port.Task) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.frontier = append([]port.Task(nil), tasks...)
}

// land removes the issue from the frontier — the driver merged it. Its dependents
// (still on the frontier) become unblocked on the next sweep.
func (t *memTracker) land(issue int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	kept := t.frontier[:0:0]
	for _, task := range t.frontier {
		if task.Number != issue {
			kept = append(kept, task)
		}
	}
	t.frontier = kept
}

func (t *memTracker) Frontier(context.Context) ([]port.Task, error) {
	t.reads.Add(1)
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]port.Task(nil), t.frontier...), nil
}

func (t *memTracker) Comment(_ context.Context, issue int, body string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.comments = append(t.comments, trackerComment{issue: issue, body: body})
	return nil
}

func (t *memTracker) commentedIssues() map[int]bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := map[int]bool{}
	for _, c := range t.comments {
		out[c.issue] = true
	}
	return out
}

// frontierReads is how many times the tracker was probed — every sweep
// re-probes it, so a re-invocation is a fresh read (AP-15).
func (t *memTracker) frontierReads() int { return int(t.reads.Load()) }

// --- the AgentRuntime stub: a fresh process that implements and commits ---------

// scriptRuntime is the stub at the AgentRuntime boundary — the model call. Each
// Launch "implements" a task by writing the impl artifact into the task's REAL
// worktree and committing it there, exactly as a real headless CLI leaves a
// committed diff (Model A: the agent commits, the Worker pushes). Everything
// above it — the local gate, the publish, the proposal, the check read — then
// runs on genuine git state. It reacts to the brief the way a real agent reads
// its charter:
//
//   - a FIXER launch (brief carries fixerMarker) commits a GREEN artifact — the
//     self-heal fix converges;
//   - an INITIAL launch honors the task's fixture mode: green commits a green
//     artifact; redfix commits a red (broken) one the drive-to-green loop must
//     heal; hollow writes nothing at all, so its branch carries no commit beyond
//     base and the run is a clean no-op (DINV-3);
//   - a RESUME launch (the artifact already exists, non-fixer) leaves prior WIP
//     untouched, modelling an idempotent resume from the pushed branch (AP-22).
//
// It is one shared instance across the sweep (a fresh PROCESS per task is the real
// adapter's job, DINV-11; the stub models that with a fresh commit and a
// per-launch record, not a reused session). Every commit lands inside a linked
// worktree, so it passes the real parent-checkout guard the Workspace armed —
// the guard is exercised, not bypassed.
type scriptRuntime struct {
	launches atomic.Int32 // Launch call count == model calls

	mu    sync.Mutex
	specs []port.LaunchSpec // every brief/model/effort that reached a fresh process
}

func (r *scriptRuntime) AuthCheck(context.Context) error { return nil }

func (r *scriptRuntime) ModelRoute() port.ModelRoute {
	return port.ModelRoute{Model: "fake-model-r0", Effort: "fake-effort"}
}

func (r *scriptRuntime) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	r.launches.Add(1)
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()

	if err := r.act(ctx, spec); err != nil {
		return nil, err
	}
	return scriptHandle{}, nil
}

// act performs the fixture's mutation for one launch and commits it in the
// worktree, the way a real agent finishes its turn.
func (r *scriptRuntime) act(ctx context.Context, spec port.LaunchSpec) error {
	impl := filepath.Join(spec.Workdir, implFile)
	isFixer := strings.Contains(spec.Brief, fixerMarker)
	mode := parseMode(spec.Brief)

	var content string
	switch {
	case isFixer:
		content = "green" // the self-heal fixer converges the task to green
	case mode == "hollow":
		return nil // clean exit, nothing written -> no commit -> hollow (DINV-3)
	case artifactExists(impl):
		return nil // resume: prior WIP stands, do not clobber (idempotent, AP-22)
	case mode == "redfix":
		content = "red" // first attempt is broken; drive-to-green must heal it
	default:
		content = "green"
	}
	if err := os.WriteFile(impl, []byte(content), 0o644); err != nil {
		return fmt.Errorf("agent: write artifact %s: %w", impl, err)
	}
	// Name the task in the message, as a real agent's commit would. The identity
	// isn't decoration: git commits are content-addressed, and the harness pins
	// author/committer dates for determinism, so two tasks committing the same
	// artifact under the same message would land on one identical SHA and become
	// indistinguishable to any assertion about commit ancestry.
	return commitWorktree(ctx, spec.Workdir, "agent: implement "+parseTaskID(spec.Brief)+" ("+content+")")
}

// commitWorktree stages and commits the worktree's changes, or does nothing when
// the tree is already clean (a fixer that rewrote identical content). It is a
// real git commit inside a real linked worktree, so the parent-checkout guard
// runs on it.
func commitWorktree(ctx context.Context, dir, message string) error {
	status, err := git(ctx, dir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("agent: read worktree status: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	if _, err := git(ctx, dir, "add", "-A"); err != nil {
		return fmt.Errorf("agent: stage the implementation: %w", err)
	}
	if _, err := git(ctx, dir, "commit", "-q", "-m", message); err != nil {
		return fmt.Errorf("agent: commit the implementation: %w", err)
	}
	return nil
}

func (r *scriptRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	// Completion is signalled by the (stubbed) process finishing; the RunResult is
	// telemetry (tokens/turns), never the success verdict (DINV-3).
	return port.RunResult{Tokens: 128, Turns: 2, Output: prdescriptiontest.Message}, nil
}

func (r *scriptRuntime) Kill(context.Context, port.Handle) error { return nil }

func (r *scriptRuntime) launchCount() int { return int(r.launches.Load()) }

func (r *scriptRuntime) specSnapshot() []port.LaunchSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]port.LaunchSpec(nil), r.specs...)
}

type scriptHandle struct{}

func (scriptHandle) RuntimeHandle() {}

// --- GitHub stub 2: the pull-request and Checks API -----------------------------

// gitHost is the stub at the GitHub CodeHost boundary. Only the PR bookkeeping is
// invented — a proposal number per branch, stable across re-opens (AP-22). Every
// FACT it reports is read from the REAL bare origin:
//
//   - OpenProposal opens from a genuinely published branch. A branch origin does
//     not carry, or one carrying no artifact, is ErrNoProposal — the hollow path
//     (DINV-3), reached because nothing was pushed, not because a flag said so.
//   - Checks is a fresh, exact read of the PUSHED head's artifact (AP-15): green
//     iff what was published is green. A fix that was committed but never pushed
//     therefore still reads red, exactly as CI would report it.
//   - InspectProposal additionally reports the branch's real commit SHA, so the
//     drive-to-green loop's head-identity fence (incident I10) runs on true
//     revisions — the production arrangement, where the code-host adapter serves
//     as the inspector too.
//
// The executor never merges, so this host offers no merge (DINV-6).
type gitHost struct {
	origin string // the bare repository the code host sees

	mu             sync.Mutex
	numberByBranch map[string]int
	openCalls      map[string]int // per branch: how many times OpenProposal was called
	nextPR         int
}

func newGitHost(origin string) *gitHost {
	return &gitHost{
		origin:         origin,
		numberByBranch: map[string]int{},
		openCalls:      map[string]int{},
		nextPR:         prBase,
	}
}

func (h *gitHost) OpenProposal(ctx context.Context, _ port.Task, work port.Workdir, _ prdescription.Description) (port.Proposal, error) {
	branch := work.Branch()
	content, published := h.publishedArtifact(ctx, branch)
	if !published || content == "" {
		// Nothing published to propose: the Worker maps this to hollow (DINV-3).
		return port.Proposal{}, port.ErrNoProposal
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.openCalls[branch]++
	num, exists := h.numberByBranch[branch]
	if !exists {
		num = h.nextPR
		h.nextPR++
		h.numberByBranch[branch] = num
	}
	return port.Proposal{Number: num, Branch: branch}, nil
}

func (h *gitHost) Checks(ctx context.Context, proposal port.Proposal) (port.CheckState, error) {
	content, published := h.publishedArtifact(ctx, proposal.Branch)
	if !published {
		// A proposal whose branch is not on the remote is a wiring fault, never a
		// verdict: fail closed rather than infer a state (DINV-3).
		return port.CheckPending, fmt.Errorf("codehost: proposal #%d has no published branch %s", proposal.Number, proposal.Branch)
	}
	if content == "green" {
		return port.CheckGreen, nil
	}
	return port.CheckRed, nil
}

// InspectProposal joins the proposal's landing, its real published head, and its
// check state in one observation — the exact guard production adapters supply.
// The proposal is always open here: this host never merges and never closes
// (DINV-6), so movement can only come from a head advance, which is exactly what
// the real SHA makes detectable.
func (h *gitHost) InspectProposal(ctx context.Context, proposal port.Proposal) (port.Landing, string, port.CheckState, error) {
	head := h.branchHead(ctx, proposal.Branch)
	if head == "" {
		return port.LandingUnknown, "", port.CheckPending, fmt.Errorf("codehost: proposal #%d has no published branch %s", proposal.Number, proposal.Branch)
	}
	checks, err := h.Checks(ctx, proposal)
	if err != nil {
		return port.LandingUnknown, "", port.CheckPending, err
	}
	return port.LandingOpen, head, checks, nil
}

// branchHead is the commit origin holds for branch, or "" when origin does not
// carry it. A missing ref is not an error — it is the "nothing published yet"
// answer the code host would give.
func (h *gitHost) branchHead(ctx context.Context, branch string) string {
	head, err := git(ctx, h.origin, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		return ""
	}
	return head
}

// publishedArtifact reads the impl artifact off the branch's published head.
func (h *gitHost) publishedArtifact(ctx context.Context, branch string) (content string, published bool) {
	head := h.branchHead(ctx, branch)
	if head == "" {
		return "", false
	}
	out, showErr := git(ctx, h.origin, "show", branch+":"+implFile)
	if showErr != nil {
		// The branch is published but carries no artifact — a real published state,
		// not a fault.
		return "", true
	}
	return strings.TrimSpace(out), true
}

// distinctProposals is the number of unique proposals opened — the idempotency
// witness: a re-invocation must not grow it (no duplicate proposal).
func (h *gitHost) distinctProposals() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.numberByBranch)
}

func (h *gitHost) proposalNumber(branch string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.numberByBranch[branch]
}

// reopenCount is how many times a branch's proposal was opened — a re-invocation
// re-opens the same branch, and the number stays stable (idempotent, AP-22).
func (h *gitHost) reopenCount(branch string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.openCalls[branch]
}

// --- drive-to-green injected boundaries ----------------------------------------

// gitPusher is the drive-to-green Pusher, and it is REAL: the same
// force-with-lease push plus published-head read the production pusher performs
// (cmd/implement-now's gitPusher), against the local origin. A self-heal fix
// therefore only reads green once it has genuinely been published — the whole
// point of moving this harness onto real git. It never merges (DINV-6).
type gitPusher struct {
	remote string
	pushes atomic.Int32
}

func (p *gitPusher) Push(ctx context.Context, work port.Workdir) (string, error) {
	if work == nil {
		return "", fmt.Errorf("push: nil workdir")
	}
	branch := work.Branch()
	if branch == "" {
		return "", fmt.Errorf("push: workdir has no branch")
	}
	if _, err := git(ctx, work.Dir(), "push", "--force-with-lease", p.remote, branch); err != nil {
		return "", fmt.Errorf("push %s: %w", branch, err)
	}
	p.pushes.Add(1)
	// Name the commit that was published so the loop recognises its own write
	// rather than rediscovering it from a lagging read (incident I10).
	head, err := git(ctx, work.Dir(), "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("read the published head of %s: %w", branch, err)
	}
	return head, nil
}

// pushCount is how many re-pushes the drive-to-green loop performed (a healed
// branch was re-pushed at least once).
func (p *gitPusher) pushCount() int { return int(p.pushes.Load()) }

// cannedLogFetcher is the stub at the drive-to-green LogFetcher seam — a GitHub
// check-run/job-log read. It returns a fixed failed-CI log (unredacted) so the
// loop's redact+clamp path runs before a fixer ever sees it (ABUSE-3, exercised
// end to end; the redaction itself is asserted in drivetogreen's own tests).
type cannedLogFetcher string

func (c cannedLogFetcher) FetchFailedLogs(context.Context, port.Proposal) (string, error) {
	return string(c), nil
}

// fileGateRunner is the real Gate's injected CommandRunner: it runs the local gate
// over the worktree the agent just committed into, while the code host reads the
// pushed head — so "passed local" and "green CI" are two reads of the same work
// and cannot diverge (DINV-4, the one CI-mirroring predicate). It is a Go closure,
// not a shell-out — deterministic given the tree. It fails closed if the artifact
// cannot be read (never a silent pass).
func fileGateRunner(_ context.Context, dir string, argv []string) drivetogreen.CommandResult {
	content, ok := readArtifact(dir)
	if !ok {
		return drivetogreen.CommandResult{Passed: false, Output: "gate: no artifact to check"}
	}
	if content == "green" {
		return drivetogreen.CommandResult{Passed: true, Output: "gate: artifact is green"}
	}
	return drivetogreen.CommandResult{Passed: false, Output: "gate: artifact is " + content + " (want green)"}
}

// --- small file helpers --------------------------------------------------------

func readArtifact(dir string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(dir, implFile))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

func artifactExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// parseMode recovers the fixture mode from a brief's [[e2e:id:mode]] tag; an
// untagged brief defaults to green (a plain task implements cleanly).
func parseMode(brief string) string {
	m := tagRE.FindStringSubmatch(brief)
	if len(m) == 3 {
		return m[2]
	}
	return "green"
}

// parseTaskID recovers the task identity from a brief's [[e2e:id:mode]] tag, so
// each task's commit is its own object. An untagged brief has no identity to
// recover and takes a stable placeholder.
func parseTaskID(brief string) string {
	m := tagRE.FindStringSubmatch(brief)
	if len(m) == 3 {
		return m[1]
	}
	return "task"
}

func (h *gitHost) DescribeProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (bool, error) {
	return false, nil
}
