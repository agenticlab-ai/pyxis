package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

type PrototypeBuildResult struct {
	Head     string
	Findings string
	Usage    port.RunResult
}

// BuildPrototype owns one launch, without entering the implementation worker
// cycle. In particular, no error after launch cleans up the prototype or state.
func BuildPrototype(ctx context.Context, p PrototypeResult, idea string, bound time.Duration, run RunCommand) (out PrototypeBuildResult, err error) {
	if p.Route.Runtime == nil || run == nil || bound <= 0 {
		return out, fmt.Errorf("prototype build needs a runtime, process adapter, and positive time bound")
	}
	body, err := os.ReadFile(idea)
	if err != nil {
		return out, fmt.Errorf("read prototype idea: %w", err)
	}
	state, _, err := executorruntime.PrepareTaskState(ctx, p.Worktree, p.Route.Harness)
	if err != nil {
		return out, fmt.Errorf("prepare prototype runtime state: %w", err)
	}
	handle, launchErr := p.Route.Runtime.Launch(ctx, port.LaunchSpec{
		Purpose: port.LaunchPurposePrototype, Brief: string(body),
		Workdir: p.Worktree, HeadBranch: p.Branch, RuntimeStateDir: state,
		Model: p.Route.Profile.Model, Effort: p.Route.Profile.Effort,
	})
	if launchErr != nil {
		err = fmt.Errorf("launch prototype: %w", launchErr)
	} else if handle == nil {
		err = fmt.Errorf("prototype launch returned no handle: %w", port.ErrNotQuiescent)
	} else {
		out.Usage, err = awaitPrototype(ctx, p.Route.Runtime, handle, bound, 5*time.Second)
		if err == nil && out.Usage.Incomplete {
			cause := out.Usage.Reason
			if cause == "" {
				cause = "harness reported an incomplete run"
			}
			err = fmt.Errorf("prototype: %s", cause)
		}
	}
	// This sentinel takes precedence over every other outcome. Even inspection
	// for a checkpoint must not become a path to staging a possible live writer.
	if errors.Is(err, port.ErrNotQuiescent) {
		return out, err
	}
	// A launch error without the sentinel, like any terminal Await return,
	// acknowledges that no process tree remains. Preserve partial work too.
	commitCtx := context.WithoutCancel(ctx)
	if commitErr := commitPrototype(commitCtx, p, run); commitErr != nil {
		return out, errors.Join(err, commitErr)
	}
	head, headErr := run(commitCtx, p.Worktree, nil, "git", "rev-parse", "HEAD")
	if headErr != nil {
		return out, errors.Join(err, fmt.Errorf("read prototype head: %w", headErr))
	}
	out.Head = strings.TrimSpace(string(head))
	findings := filepath.Join(p.Worktree, ".pyxis", "scratch", "prototype-findings.md")
	if info, statErr := os.Stat(findings); statErr == nil && info.Mode().IsRegular() {
		out.Findings = findings
	} else if statErr != nil && !os.IsNotExist(statErr) {
		err = errors.Join(err, fmt.Errorf("inspect prototype findings: %w", statErr))
	}
	return out, err
}

func commitPrototype(ctx context.Context, p PrototypeResult, run RunCommand) error {
	branch, err := run(ctx, p.Worktree, nil, "git", "symbolic-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("check prototype branch before commit: %w", err)
	}
	if strings.TrimSpace(string(branch)) != "refs/heads/"+p.Branch {
		return fmt.Errorf("refuse prototype commit: expected branch %s, got %q", p.Branch, strings.TrimSpace(string(branch)))
	}
	if _, err := run(ctx, p.Worktree, nil, "git", "add", "--all", "--", "."); err != nil {
		return fmt.Errorf("stage prototype work: %w", err)
	}
	if _, err := run(ctx, p.Worktree, nil, "git", "commit", "--allow-empty", "-m", "Prototype "+strings.TrimPrefix(p.Branch, "prototype/")); err != nil {
		return fmt.Errorf("commit prototype work: %w", err)
	}
	return nil
}

func awaitPrototype(ctx context.Context, rt port.AgentRuntime, handle port.Handle, bound, grace time.Duration) (port.RunResult, error) {
	awaitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type completion struct {
		usage port.RunResult
		err   error
	}
	done := make(chan completion, 1)
	// The host owns this goroutine. Exit or Kill unblocks Await; cancellation
	// releases its context if the runtime cannot acknowledge within the grace.
	go func() {
		usage, err := rt.Await(awaitCtx, handle)
		done <- completion{usage, err}
	}()
	timer := time.NewTimer(bound)
	defer timer.Stop()
	var cause error
	select {
	case result := <-done:
		return result.usage, result.err
	case <-timer.C:
		cause = errors.New("timed out")
	case <-ctx.Done():
		cause = ctx.Err()
	}
	stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
	defer stopCancel()
	if err := rt.Kill(stopCtx, handle); err != nil {
		return port.RunResult{}, errors.Join(cause, fmt.Errorf("%w: kill prototype: %w", port.ErrNotQuiescent, err))
	}
	// Kill alone is not an exit acknowledgement. Drain Await before touching
	// the index, retaining telemetry and honoring its not-quiescent sentinel.
	select {
	case result := <-done:
		if errors.Is(result.err, port.ErrNotQuiescent) {
			return result.usage, errors.Join(cause, result.err)
		}
		// Once stopped, a killed process's exit error is expected. The bound
		// or cancellation owns the failure cause; retain the reported usage.
		return result.usage, cause
	case <-stopCtx.Done():
		return port.RunResult{}, errors.Join(cause, port.ErrNotQuiescent)
	}
}
