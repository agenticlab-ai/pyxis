package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/drivetogreen"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
)

// newGreenFactory builds the Dispatcher's per-task drive-to-green factory: each
// ready Worker gets a drivetogreen.Driver bound to that task's resolved runtime
// (so the CI-fix fixer runs a fresh process on the same backend, DINV-11), over
// the shared CodeHost and the production heal collaborators. The self-heal cap is
// the config's CISelfHealAttempts (§3.8); each attempt's ci_fix_attempt event is
// routed to the reporter via OnAttempt, joining the one normalized stream (§3.4).
//
// The local gate is the code-gate verb (codegatemirror), run in-process over the
// task workdir: it reads the workdir's own code-gate check list and runs the
// required-CI check set through the shared runner, so the local gate mirrors
// required CI by construction (code-gate-mirror.md, DINV-1) with the fail-on and
// required/advisory semantics the former hand-listed executor.gate could not
// express. An unconfigured code-gate (no `code-gate:` block) vacuously passes and
// CI remains the authoritative oracle (DINV-3). The same one predicate gates every
// self-heal re-push AND the truncated-but-green publish decision, so "local green"
// is one thing (DINV-4) that mirrors CI (UB-5).
type guardedCodeHost interface {
	port.CodeHost
	drivetogreen.ProposalInspector
}

func newGreenFactory(host guardedCodeHost, cfg config.Config, reporter report.Emitter, remote string) dispatch.GreenDriverFactory {
	return newGreenFactoryWithReservation(host, cfg, reporter, remote, nil)
}

func newGreenFactoryWithReservation(host guardedCodeHost, cfg config.Config, reporter report.Emitter, remote string, reserve drivetogreen.AttemptReservation) dispatch.GreenDriverFactory {
	return func(rt port.AgentRuntime) (worker.GreenDriver, error) {
		return newGreenDriver(host, cfg, reporter, remote, rt, nil, reserve)
	}
}

func newGreenDriver(host guardedCodeHost, cfg config.Config, reporter report.Emitter, remote string, rt port.AgentRuntime, resolve func(context.Context) (entity.Harness, port.AgentRuntime, error), reserve drivetogreen.AttemptReservation) (*drivetogreen.Driver, error) {
	var fetcher drivetogreen.LogFetcher = emptyLogFetcher{}
	if native, ok := host.(drivetogreen.LogFetcher); ok {
		fetcher = native
	}
	pusher := gitPusher{remote: remote}
	if local, ok := host.(*localVerifiedHost); ok {
		pusher.afterPush = local.publishLocal
	}
	// The composition root's narrower structural contract requires the exact
	// landing/head/check guard at compile time without growing frozen CodeHost.
	return drivetogreen.New(drivetogreen.Deps{
		Host:           host,
		Inspector:      host,
		Runtime:        rt,
		ResolveRuntime: resolve,
		Fetcher:        fetcher,
		Pusher:         pusher,
		Runner: func(ctx context.Context, dir string, argv []string) drivetogreen.CommandResult {
			task := drivetogreen.GateTask(ctx)
			// GateGreen preserves the issue command's existing observer when
			// no drive task has been added to the recovery context.
			if task.ID != "" || task.Number != 0 {
				ctx = codegatemirror.WithProgressObserver(ctx, func(progress codegatemirror.Progress) {
					elapsed := progress.Duration.Milliseconds()
					reporter.Emit(report.Event{Kind: report.KindGateCheckProgress, TaskID: task.ID, Issue: task.Number, Phase: progress.Name, Status: progress.Status, ElapsedMS: &elapsed, Reason: progress.Detail, ArtifactRef: progress.LogPath})
				})
			}
			return codeGateRunner(ctx, dir, argv)
		},
		Revision: func(ctx context.Context, work port.Workdir) (string, error) {
			out, err := localGit(ctx, work.Dir(), "rev-parse", "HEAD")
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(out)), nil
		},
	}, drivetogreen.Config{
		Cap:            cfg.CISelfHealAttempts(),
		GateChecks:     codeGateChecks(),
		ReserveAttempt: reserve,
		OnAttempt: func(e drivetogreen.AttemptEvent) {
			if e.Local {
				// A local gate fix before any push: no CI run is spent on it.
				reporter.Emit(report.LocalGateAttempt(e.TaskID, e.Attempt, e.Cap))
				return
			}
			reporter.Emit(report.CIFixAttempt(e.TaskID, e.Attempt, e.Cap))
		},
	})
}

// codeGateChecks is the executor's local gate: a single entry that invokes the
// code-gate verb over the task workdir — the design's "one entry" (code-gate-
// mirror.md §3.6) replacing the former hand-listed executor.gate. code-gate reads
// the workdir's own code-gate config and runs the required-CI check set, so the
// Check's argv is unused (codeGateRunner ignores it); the Name carries the
// predicate's identity for diagnostics.
func codeGateChecks() []drivetogreen.Check {
	return []drivetogreen.Check{{Name: "code-gate"}}
}

// codeGateRunner is the production CommandRunner backing the local gate: it runs
// the code-gate verb (codegatemirror) in-process over the task's workdir — the
// workdir's own required-CI check set through the shared runner — and maps the
// aggregate verdict onto the gate's result. Running the same predicate the
// `code-gate` binary runs makes the executor's gate the SAME predicate CI invokes
// (DINV-1), without a subprocess or a "which tree is code-gate built from" problem
// for a non-pyxis subject repo. It runs through RunCached: a green verdict is
// cached by content fingerprint (DINV-6), so a tree the drive-to-green loop just
// gated makes the workdir's push pre-push a no-op. A malformed config or a
// cancelled run fails the gate CLOSED via CommandResult.Err — the gate never
// certifies green on a predicate it could not evaluate. The argv is unused (see
// codeGateChecks).
func codeGateRunner(ctx context.Context, dir string, _ []string) drivetogreen.CommandResult {
	ctx, cleanup, err := runtime.VerificationContext(ctx)
	if err != nil {
		return drivetogreen.CommandResult{Err: err}
	}
	defer cleanup()
	cfg, err := codegatemirror.Load(dir)
	if err != nil {
		return drivetogreen.CommandResult{Err: fmt.Errorf("implement-now: load code-gate config: %w", err)}
	}
	rep, _, _ := cfg.RunCached(ctx, dir)
	if err := ctx.Err(); err != nil {
		return drivetogreen.CommandResult{Err: fmt.Errorf("implement-now: code-gate cancelled: %w", err)}
	}
	return drivetogreen.CommandResult{Passed: rep.Passed, Output: rep.FailureSummary()}
}

// gitPusher is the production Pusher: it re-publishes a task's worktree branch to
// the code host so CI re-runs on the new head (never a merge — DINV-6). It is a
// Go os/exec shell-out to git in the workdir. remote is the configured git remote
// (config-driven-portability-v1 §3.2, FR-2) the composition root resolves — the
// SAME remote the workdir's own publish targets; it carries no literal of its own
// (DINV-1). A blank remote is a wiring fault, so Push fails closed rather than
// guessing a default.
type gitPusher struct {
	remote    string
	afterPush func(context.Context, port.Workdir, string) error
}

func (p gitPusher) Push(ctx context.Context, work port.Workdir) (string, error) {
	if work == nil {
		return "", fmt.Errorf("implement-now: push with a nil workdir")
	}
	if p.remote == "" {
		return "", fmt.Errorf("implement-now: git pusher has no configured remote")
	}
	branch := work.Branch()
	if branch == "" {
		return "", fmt.Errorf("implement-now: workdir has no branch to push")
	}
	status, err := exec.CommandContext(ctx, "git", "-C", work.Dir(), "status", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("implement-now: inspect repaired worktree: %w", err)
	}
	if len(status) != 0 {
		return "", fmt.Errorf("implement-now: repaired branch %s has uncheckpointed changes; refusing publication", branch)
	}
	if guarded, ok := work.(interface{ ValidateIntegration(context.Context) error }); ok {
		if err := guarded.ValidateIntegration(ctx); err != nil {
			return "", err
		}
	}
	// Repository hooks can execute agent-authored code. The supervisor already
	// ran the local gate under confinement; publication runs no repository hook.
	cmd := exec.CommandContext(ctx, "git", "-c", "core.hooksPath=/dev/null", "-C", work.Dir(), "push", "--force-with-lease", p.remote, branch)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("implement-now: git push %s: %w: %s", branch, err, buf.String())
	}
	// Name the commit that was published, so the loop can recognise its own write
	// instead of rediscovering it from a lagging code-host read (incident I10).
	// Read AFTER a successful push: nothing mutates the worktree in between, and
	// a failed push must report no head at all.
	head, err := exec.CommandContext(ctx, "git", "-C", work.Dir(), "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("implement-now: reading the published head of %s: %w", branch, err)
	}
	if p.afterPush != nil {
		if err := p.afterPush(ctx, work, strings.TrimSpace(string(head))); err != nil {
			return "", fmt.Errorf("publish local verification for pushed revision: %w", err)
		}
	}
	return strings.TrimSpace(string(head)), nil
}

// emptyLogFetcher is a minimal LogFetcher that returns no captured CI logs: the
// self-heal fixer then re-derives the failure from the workdir (drivetogreen
// explicitly permits an empty log). Hosts with a native failed-log seam are
// selected above; this fallback remains for adapters without that capability.
type emptyLogFetcher struct{}

func (emptyLogFetcher) FetchFailedLogs(context.Context, port.Proposal) (string, error) {
	return "", nil
}
