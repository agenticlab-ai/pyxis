package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/drivetogreen"
	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

type existingWorkspace interface {
	ExistingDescendant(context.Context, port.Task, string) (port.Workdir, error)
}

type issueContinuationHost interface {
	executionHost
	guardedCodeHost
}

// continueAssociatedCI reconnects an admitted issue's existing proposal to the
// native bounded CI driver. Green needs no checkout or implementation route;
// pending/red owns the task claim and exact existing checkout, and only RED can
// reserve authority and resolve a fixer runtime.
func (r *issueRun) continueAssociatedCI(ctx context.Context, host issueContinuationHost, space existingWorkspace, claims port.Lease, cfg config.Config, reporter report.Emitter, remote string, resolve dispatch.RuntimeResolver, continuity worker.SessionContinuity, budget time.Duration, p port.Proposal) error {
	landing, head, checks, err := host.InspectProposal(ctx, p)
	if err != nil {
		return err
	}
	if landing != port.LandingOpen {
		return fmt.Errorf("associated proposal #%d is %s", p.Number, landing)
	}
	task := r.task
	task.HeadBranch = dispatch.HeadBranch(task)
	if p.Branch != task.HeadBranch || !issueentry.ValidRevision(head) {
		return errors.New("associated proposal branch or head differs from the admitted task")
	}
	if checks == port.CheckGreen {
		return nil
	}
	claim, err := claims.Acquire(ctx, task)
	if err != nil {
		if errors.Is(err, port.ErrClaimLost) {
			r.active = true
		}
		return fmt.Errorf("claim associated issue CI continuation: %w", err)
	}
	defer func() {
		if err := claim.Release(context.WithoutCancel(ctx)); err != nil {
			slog.WarnContext(ctx, "release associated issue CI continuation claim", "task", task.ID, "error", err)
		}
	}()
	work, err := space.ExistingDescendant(ctx, task, head)
	if err != nil {
		return err
	}
	retained, ok, err := issueentry.LoadCheckpoint(runtime.TaskStateRoot(work.Dir()))
	if err != nil || !ok {
		if err == nil {
			err = errors.New("associated red/pending proposal has no retained implementer description")
		}
		return err
	}
	if retained.ContractID != r.contract.ID || retained.TaskID != task.ID || retained.Branch != task.HeadBranch {
		return errors.New("associated proposal has no matching retained implementer description")
	}
	if _, err := localGit(ctx, work.Dir(), "merge-base", "--is-ancestor", retained.Revision, "HEAD"); err != nil {
		return fmt.Errorf("associated proposal has no matching retained implementer description: %w", err)
	}
	description, problems := prdescription.FromAgent(retained.Description)
	if len(problems) != 0 {
		return fmt.Errorf("retained implementer description is incomplete: %v", problems)
	}
	reserve := func(ctx context.Context, _ string) (int, int, error) {
		reserved, err := r.store.ReserveRemoteRepair(ctx, r.contract)
		return reserved.Attempt, reserved.Cap, err
	}
	lazyRuntime := func(ctx context.Context) (entity.Harness, port.AgentRuntime, error) {
		harness, rt, err := resolve(task)
		if err == nil {
			err = rt.AuthCheck(ctx)
		}
		return harness, rt, err
	}
	driver, err := newGreenDriver(host, cfg, reporter, remote, nil, lazyRuntime, reserve)
	if err != nil {
		return err
	}
	if continuity != nil {
		driver.UseSessionContinuity(continuity, "", func(task port.Task, event worker.SessionEvent) {
			reason := ""
			if event.Err != nil {
				reason = event.Err.Error()
			}
			reporter.Emit(report.TaskSession(task.ID, task.Number, event.Harness, event.Action, event.SessionID, event.Purpose, reason))
		})
	}
	driver.BindDescriber(func(ctx context.Context, work port.Workdir) error {
		computed, err := worker.Describe(ctx, task, work, description)
		if err != nil {
			return err
		}
		_, err = host.DescribeProposal(ctx, task, work, computed)
		return err
	})
	driveCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	result, err := driver.DriveFrom(driveCtx, task, work, p, head)
	if err != nil {
		return err
	}
	if !result.Green {
		return errors.New("associated issue CI continuation ended without green checks")
	}
	r.mu.Lock()
	r.work = work
	r.mu.Unlock()
	return r.recordLocal(ctx, host, p)
}

func issueRepairReservation(r *issueRun) drivetogreen.AttemptReservation {
	if r == nil {
		return nil
	}
	return func(ctx context.Context, _ string) (int, int, error) {
		reserved, err := r.store.ReserveRemoteRepair(ctx, r.contract)
		return reserved.Attempt, reserved.Cap, err
	}
}
