package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/planbasis"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

// retainCheckpoint keeps the implementer's description beside the selected
// task's committed checkpoint before verification can fail or be interrupted.
// A retention failure leaves the attempt unchanged; recovery then refuses.
func (r *issueRun) retainCheckpoint(ctx context.Context, task port.Task, work port.Workdir, description prdescription.Description) {
	if task.ID != r.task.ID {
		return
	}
	revision, err := (&localVerifiedHost{git: localGit}).localRevisionAt(ctx, work.Dir())
	if err == nil {
		err = issueentry.RetainCheckpoint(runtime.TaskStateRoot(work.Dir()), issueentry.Checkpoint{Schema: issueentry.CheckpointSchema,
			ContractID: r.contract.ID, TaskID: task.ID, Branch: work.Branch(), Revision: revision, Description: description.Render()})
	}
	if err != nil {
		slog.WarnContext(ctx, "issue checkpoint description not retained; checkpoint recovery will refuse it", "task", task.ID, "error", err)
		return
	}
	r.mu.Lock()
	r.described = revision
	r.mu.Unlock()
}

// checkpointSpace reopens the selected task's existing isolated worktree.
type checkpointSpace interface {
	Existing(ctx context.Context, task port.Task, revision string) (port.Workdir, error)
}

// checkpointRecovery is the operator-authorized publication of a committed
// checkpoint that a failed attempt left unpublished on the admitted issue's
// task branch: the independently authorized recovery of
// implement-now-issue-entry §3.4. It launches no model of any role and
// leaves every cap as the original attempt spent it. The exact checkpoint
// passes the complete local gate before the worker's own publication
// boundaries carry it to a proposal; hosted checks and the independent review
// stay with the ordinary issue invocation.
type checkpointRecovery struct {
	host       executionHost
	space      checkpointSpace
	lease      port.Lease
	reviews    reviewobligation.Store
	repository string
	remote     string
	revision   string
}

// recoveryStop is a recovery that ended before publication: the phase it
// stopped in, the operator's next action, and whether another worker holds
// the task's claim.
type recoveryStop struct {
	phase, next string
	active      bool
	err         error
}

func (s *recoveryStop) Error() string { return s.phase + ": " + s.err.Error() }
func (s *recoveryStop) Unwrap() error { return s.err }

// recoverCheckpoint publishes the checkpoint, or reports the proposal an
// earlier publication or reconciliation already retained, then closes with
// the ordinary terminal verification. The original attempt stays failed.
func (r *issueRun) recoverCheckpoint(ctx context.Context, rc checkpointRecovery, existing port.Proposal, published bool, stdout, stderr io.Writer) int {
	if published {
		_, _ = fmt.Fprintf(stdout, "implement-now: issue already has proposal #%d; checkpoint recovery published nothing and launched no model\n", existing.Number)
		return r.finish(ctx, rc.host, rc.reviews, rc.repository, stdout, stderr)
	}
	finished := r.logPhase("checkpoint_recovery")
	p, stop := r.publishCheckpoint(ctx, rc)
	if stop != nil {
		finished(stop)
		_, _ = fmt.Fprintf(stderr, "implement-now: checkpoint recovery stopped before publication; no model was launched: %v\n", stop)
		r.close(stderr, issueTerminal{result: "recovery_refused", phase: "recovery_" + stop.phase, checkpoint: r.branchHead(ctx), active: stop.active, next: stop.next})
		return 1
	}
	finished(nil)
	_, _ = fmt.Fprintf(stdout, "implement-now: recovered checkpoint %s as PR #%d without launching a model; the original implementation attempt remains failed\n", rc.revision, p.Number)
	return r.finish(ctx, rc.host, rc.reviews, rc.repository, stdout, stderr)
}

func (r *issueRun) publishCheckpoint(ctx context.Context, rc checkpointRecovery) (port.Proposal, *recoveryStop) {
	newAttempt := "rerun the issue command without --recover-checkpoint to authorize a new implementation attempt"
	stop := func(phase, next string, err error) (port.Proposal, *recoveryStop) {
		return port.Proposal{}, &recoveryStop{phase: phase, next: next, err: err}
	}
	retry := func(phase string, err error) (port.Proposal, *recoveryStop) {
		next := newAttempt
		if head := r.branchHead(ctx); head != "" {
			next = "resolve the cause above, then rerun the issue command with --recover-checkpoint " + head
		}
		return stop(phase, next, err)
	}
	task := r.task
	task.HeadBranch = dispatch.HeadBranch(task)
	claim, err := rc.lease.Acquire(ctx, task)
	if errors.Is(err, port.ErrClaimLost) {
		return port.Proposal{}, &recoveryStop{phase: "claim", active: true, err: err,
			next: "wait for the worker holding this task's claim to stop, then rerun the recovery"}
	}
	if err != nil {
		return retry("claim", err)
	}
	defer func() {
		// Like a worker, release on every path; an unreleased claim ages out.
		if err := claim.Release(context.WithoutCancel(ctx)); err != nil {
			slog.WarnContext(ctx, "release checkpoint recovery claim", "task", task.ID, "error", err)
		}
	}()
	work, err := rc.space.Existing(ctx, task, rc.revision)
	if err != nil {
		return stop("checkout", "inspect the task worktree, then recover only a committed head you have reviewed, or "+newAttempt, err)
	}
	// Publish only what the original attempt would have: its own complete
	// description, for a checkpoint that contains the one it described.
	retained, ok, err := issueentry.LoadCheckpoint(runtime.TaskStateRoot(work.Dir()))
	switch {
	case err != nil:
	case !ok:
		err = errors.New("no implementer description was retained for this task branch")
	case retained.ContractID != r.contract.ID || retained.TaskID != task.ID || retained.Branch != task.HeadBranch:
		err = errors.New("the retained implementer description belongs to another contract or branch")
	case !r.contains(ctx, retained.Revision, rc.revision):
		err = fmt.Errorf("the retained implementer description covers %s, which %s does not contain", retained.Revision, rc.revision)
	}
	if err != nil {
		return stop("description", newAttempt, err)
	}
	// LoadCheckpoint already refused a description with any problem.
	description, _ := prdescription.FromAgent(retained.Description)
	// The worker's local gate integrates mainline first; publication then
	// refuses a base that moved after verification.
	if integration, ok := work.(interface {
		RefreshMainline(context.Context) (string, error)
	}); ok {
		if _, err := integration.RefreshMainline(ctx); err != nil {
			return stop("integration", newAttempt, err)
		}
	}
	if err := requireContractRefs(ctx, work.Dir(), rc.remote); err != nil {
		return retry("contract_refs", err)
	}
	// The publication predicate itself, so publication revalidates from the
	// same exact-input cache entry.
	if _, err := (&localVerifiedHost{run: codeGateRunner, git: localGit}).verifyLocal(ctx, work); err != nil {
		return retry("verification", err)
	}
	if description, err = worker.Describe(ctx, task, work, description); err != nil {
		return retry("publication", err)
	}
	pushed, err := work.Push(ctx)
	if err != nil {
		return retry("publication", err)
	}
	if !pushed {
		return stop("publication", newAttempt, errors.New("the checkpoint carries nothing beyond mainline"))
	}
	p, err := (&issueHost{executionHost: rc.host, session: r}).OpenProposal(ctx, task, work, description)
	if err != nil {
		return retry("publication", err)
	}
	return p, nil
}

// requireContractRefs makes every lock a plan in the checkout names as its
// Basis resolvable before the gate's checks need it. A missing tag is fetched
// by its exact name from the configured remote; a different local tag is
// never overwritten, and no tag is ever created.
func requireContractRefs(ctx context.Context, dir, remote string) error {
	cfg, defects := repoconfig.Load(dir)
	if len(defects) != 0 {
		return fmt.Errorf("read plan configuration: %v", defects)
	}
	bases, err := planbasis.Required(dir, cfg)
	if err != nil {
		return fmt.Errorf("list plan contract locks: %w", err)
	}
	for _, basis := range bases {
		ref := "refs/tags/" + basis
		if _, err := localGit(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			continue
		}
		if _, err := localGit(ctx, dir, "fetch", "--no-tags", "--no-write-fetch-head", remote, ref+":"+ref); err != nil {
			return fmt.Errorf("required contract ref %s is missing and could not be fetched from %s (%v); fetch it with `git fetch %s %s:%s`", ref, remote, err, remote, ref, ref)
		}
		if _, err := localGit(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
			return fmt.Errorf("required contract ref %s fetched from %s does not name a commit", ref, remote)
		}
	}
	return nil
}

// reconcileProposal closes the external-create/local-retain interruption gap.
// It runs before the ordinary duplicate guard and never starts implementation.
func (r *issueRun) reconcileProposal(ctx context.Context, host executionHost, repo, remote string) (port.Proposal, bool, error) {
	p, associated, err := r.store.Proposal(ctx, r.contract)
	if err != nil || associated {
		return p, associated, err
	}
	task := r.task
	task.HeadBranch = dispatch.HeadBranch(task)
	open, err := host.OpenProposals(ctx, []port.Task{task})
	if err != nil {
		return port.Proposal{}, false, fmt.Errorf("find interrupted issue publication: %w", err)
	}
	native, exists := open[task.ID]
	if len(open) > 1 || (len(open) == 1 && !exists) || (exists && native.Number <= 0) {
		return port.Proposal{}, false, errors.New("conflicting native issue proposal discovery")
	}
	if !exists {
		return port.Proposal{}, false, nil
	}
	p = port.Proposal{Number: native.Number, Branch: task.HeadBranch}
	revision, err := host.ProposalHead(ctx, p)
	if err != nil {
		return p, false, err
	}
	// Validate the untrusted native revision before passing it to git. No
	// proof is assumed merely because the remote proposal already exists.
	if _, err := r.store.LocalVerified(ctx, r.contract, p, revision); err != nil {
		return p, false, err
	}
	if err := verifyPublishedLocal(ctx, repo, remote, revision, localGit); err != nil {
		return p, false, err
	}
	after, err := host.ProposalHead(ctx, p)
	if err != nil {
		return p, false, err
	}
	if after != revision {
		return p, false, errors.New("issue proposal changed during recovered local verification")
	}
	if local, ok := host.(*localVerifiedHost); ok {
		if err := local.status.SetStatus(ctx, revision, localGateContext, "success", "", "Required local code-gate checks passed for this exact commit"); err != nil {
			return p, false, err
		}
	}
	landing, after, _, err := host.InspectProposal(ctx, p)
	if err != nil {
		return p, false, err
	}
	if landing != port.LandingOpen || after != revision {
		return p, false, errors.New("recovered issue proposal is no longer open at its locally verified revision")
	}
	if err := r.store.AssociateVerified(ctx, r.contract, p, revision); err != nil {
		return p, false, err
	}
	return p, true, nil
}

// verifyPublishedLocal restores only the native PR's exact revision into a
// disposable clone. Existing task worktrees and their uncommitted work stay
// untouched, and a fresh command clone needs no machine-local saved proof.
func verifyPublishedLocal(ctx context.Context, repo, remote, revision string, git func(context.Context, string, ...string) ([]byte, error)) error {
	dir, err := os.MkdirTemp("", "pyxis-issue-recovery-*")
	if err != nil {
		return fmt.Errorf("create issue recovery checkout: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			slog.WarnContext(ctx, "remove disposable issue recovery checkout", "error", err)
		}
	}()
	url, err := git(ctx, repo, "remote", "get-url", remote)
	if err != nil {
		return fmt.Errorf("resolve issue recovery remote: %w", err)
	}
	// Use the configured parent's credentials without copying them into the
	// verification checkout or updating shared refs/FETCH_HEAD.
	if _, err := git(ctx, repo, "fetch", "--no-tags", "--no-write-fetch-head", strings.TrimSpace(string(url)), revision); err != nil {
		return fmt.Errorf("fetch exact issue recovery revision: %w", err)
	}
	if _, err := git(ctx, repo, "-c", "core.hooksPath=/dev/null", "clone", "--no-checkout", "--no-local", "--origin", remote, "--", repo, dir); err != nil {
		return fmt.Errorf("clone issue recovery checkout: %w", err)
	}
	if _, err := git(ctx, dir, "fetch", "--no-tags", "--", repo, revision); err != nil {
		return fmt.Errorf("copy exact issue recovery revision: %w", err)
	}
	// Keep tracking refs used by comparison gates, while removing the URL
	// and fetch configuration that would point back into the shared checkout.
	if _, err := git(ctx, dir, "config", "--remove-section", "remote."+remote); err != nil {
		return fmt.Errorf("isolate issue recovery checkout: %w", err)
	}
	if _, err := git(ctx, dir, "-c", "core.hooksPath=/dev/null", "checkout", "--detach", revision); err != nil {
		return fmt.Errorf("checkout exact issue recovery revision: %w", err)
	}
	gate := &localVerifiedHost{run: codeGateRunner, git: localGit}
	verified, err := gate.verifyLocalAt(ctx, dir)
	if err != nil {
		return err
	}
	if verified != revision {
		return errors.New("recovered local verification covers a different issue revision")
	}
	return nil
}

// issueVerifiedInspector prevents a missing handoff from admitting a moved PR
// head on green CI alone. Capture persists only a revision with local proof.
type issueVerifiedInspector struct {
	executionHost
	session *issueRun
}

func (h issueVerifiedInspector) InspectProposal(ctx context.Context, p port.Proposal) (port.Landing, string, port.CheckState, error) {
	landing, revision, checks, err := h.executionHost.InspectProposal(ctx, p)
	if err != nil {
		return landing, revision, checks, err
	}
	local, err := h.session.store.LocalVerified(ctx, h.session.contract, p, revision)
	if err != nil {
		return landing, revision, checks, err
	}
	if !local {
		return landing, revision, checks, errors.New("missing review handoff requires local verification of the current issue revision")
	}
	return landing, revision, checks, nil
}

// captureMissingReview creates the missing exact-green obligation; dispatch's
// existing repairer then emits the implementation terminal, completes the
// handoff and invokes the independent reviewer after implementation release.
func (r *issueRun) captureMissingReview(ctx context.Context, p port.Proposal, harness entity.Harness, resolve dispatch.RuntimeResolver, factory dispatch.ReviewInvokerFactory) error {
	if factory == nil {
		return errors.New("missing issue review recovery service")
	}
	if r.task.NativeRouting != nil {
		harness = r.task.NativeRouting.Review.Harness
	}
	if harness == "" {
		if resolve == nil {
			return errors.New("missing inherited issue review harness resolver")
		}
		var err error
		harness, _, err = resolve(r.task)
		if err != nil {
			return fmt.Errorf("resolve inherited issue review harness: %w", err)
		}
	}
	reviewer, err := factory(harness)
	if err != nil {
		return err
	}
	if reviewer == nil {
		return errors.New("missing issue review recovery service")
	}
	if err := reviewer.Preflight(ctx); err != nil {
		return err
	}
	capture := reviewer.Capture(ctx, r.task, p)
	if capture.Err != nil {
		return fmt.Errorf("recover issue review obligation: %w", capture.Err)
	}
	if capture.Failed() || !capture.Persisted {
		return errors.New("issue review recovery did not persist a complete obligation")
	}
	return nil
}
