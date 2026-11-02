package codex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime/procgroup"
)

// commandSpec is the closed set of inputs one child process needs. Keeping it a
// plain value (not an *exec.Cmd) is what lets the adapter be unit-tested against
// a fake commander with no real `codex` binary on PATH.
type commandSpec struct {
	Access *runtime.CommandAccess // supervisor-owned OS boundary for model launches
	Name   string                 // executable, e.g. "codex"
	Args   []string               // arguments, without the executable
	Dir    string                 // working directory; the task's isolated worktree for a launch
	Stdin  string                 // piped to the child's stdin (the assembled brief for a launch)
	Env    []string               // full child environment; nil inherits the parent's
}

// commander spawns child processes. The production implementation wraps os/exec;
// tests inject a fake that records specs and replays recorded output, so command
// construction, JSON parsing, and auth-degrade are all exercised hermetically
// (docs/tech-design/implement-now.md §3.10).
type commander interface {
	// start launches the command without blocking and returns a process to await
	// or kill. It is the fresh, non-interactive process per task (DINV-11).
	start(ctx context.Context, spec commandSpec) (process, error)

	// run executes the command to completion and returns its output and exit
	// code — the zero-cost preflight AuthCheck uses. A nil error with a non-zero
	// exitCode means the command ran and exited non-zero; a non-nil error means
	// it could not be run at all (e.g. the binary is missing).
	run(ctx context.Context, spec commandSpec) (out []byte, exitCode int, err error)
}

// process is one launched child. wait reports the process's completion; kill
// terminates it. Both are idempotent-friendly: kill on an already-exited process
// is a no-op (os.ErrProcessDone), so a bounded-out or cancelled Worker can kill
// without racing the exit.
type process interface {
	// wait blocks until the child finishes and returns its captured stdout and
	// exit code. A nil error means the child ran and exited (with any code) —
	// completion, per AP-13; the artifact, not the exit, decides success
	// (DINV-3). A non-nil error means the child could not be run or awaited
	// (killed, signalled, an I/O failure) — an operational failure.
	wait() (stdout []byte, exitCode int, err error)

	// kill terminates the child. Killing an already-finished process returns nil.
	kill() error
}

// osCommander is the production commander over os/exec.
type processLifetime interface {
	ObserveStart() error
	Kill() error
}

type osCommander struct {
	lifetime func(*exec.Cmd) processLifetime // injectable OS ownership seam
}

var _ commander = osCommander{}

func (c osCommander) start(ctx context.Context, spec commandSpec) (process, error) {
	ctx, stopInvocation := runtime.InvocationContext(ctx)
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	// Native command executors can create separate process groups. Own their
	// observed descendants by kernel identity and wait for cleanup acknowledgement.
	// WaitDelay also bounds the post-stop pipe drain.
	var lifetime processLifetime = procgroup.NewLifetime(cmd)
	if c.lifetime != nil {
		lifetime = c.lifetime(cmd)
	}
	cmd.Cancel = lifetime.Kill
	cmd.WaitDelay = procgroup.DefaultGrace
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Codex emits progress and fatal invocation diagnostics on stderr. Keep
	// them with the JSONL stream so Await can fail loudly instead of turning an
	// error-only run into a zero-turn hollow result; parseResult ignores
	// non-JSON diagnostic lines.
	cmd.Stderr = &stdout
	logOut, logErr, closeLogs := runtime.CaptureOutput(ctx, spec.Dir, "codex", &stdout, &stdout)
	cmd.Stdout, cmd.Stderr = logOut, logErr
	procgroup.SetOwnedSession(cmd) // keep ownership across groups and short-lived parents
	if err := runtime.ConfineCommand(ctx, cmd, spec.Access); err != nil {
		closeLogs()
		return nil, errors.Join(err, stopInvocation())
	}
	if err := cmd.Start(); err != nil {
		closeLogs()
		return nil, errors.Join(fmt.Errorf("start %s: %w", spec.Name, err), stopInvocation())
	}
	if err := lifetime.ObserveStart(); err != nil {
		stopErr := lifetime.Kill()
		if stopErr == nil {
			_ = cmd.Wait() // the failed launch has been stopped; reap its direct child
		}
		closeLogs()
		return nil, errors.Join(fmt.Errorf("%w: observe %s: %w", port.ErrNotQuiescent, spec.Name, err), stopErr, stopInvocation())
	}
	return &osProcess{ctx: ctx, cmd: cmd, lifetime: lifetime, stopInvocation: stopInvocation, closeLogs: closeLogs, stdout: &stdout}, nil
}

func (osCommander) run(ctx context.Context, spec commandSpec) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return out.Bytes(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Ran and exited non-zero: report the code, no operational error.
		return out.Bytes(), exitErr.ExitCode(), nil
	}
	// Could not run at all (binary missing, context cancelled before start, …).
	return out.Bytes(), -1, err
}

// osProcess is one launched child under os/exec.
type osProcess struct {
	ctx            context.Context
	lifetime       processLifetime
	stopInvocation func() error
	closeLogs      func()
	cmd            *exec.Cmd
	stdout         *bytes.Buffer
}

var _ process = (*osProcess)(nil)

func (p *osProcess) wait() ([]byte, int, error) {
	err, stopErr := procgroup.WaitOwned(p.ctx, p.cmd, p.lifetime)
	if stopErr != nil {
		err = errors.Join(err, fmt.Errorf("%w: %w", port.ErrNotQuiescent, stopErr))
	}
	if p.stopInvocation != nil {
		if stopErr := p.stopInvocation(); stopErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: %w", port.ErrNotQuiescent, stopErr))
		}
	}
	if p.closeLogs != nil {
		p.closeLogs()
	}
	if errors.Is(err, port.ErrNotQuiescent) {
		if p.cmd.ProcessState == nil {
			return nil, -1, err // a live output writer cannot be read safely
		}
		return p.stdout.Bytes(), -1, err
	}
	if st := p.cmd.ProcessState; st != nil && st.Exited() {
		// Only the command's own nonzero status is normal completion. A joined
		// inspection, cancellation or I/O error must remain operational.
		if _, normalExit := err.(*exec.ExitError); err != nil && !normalExit {
			return p.stdout.Bytes(), -1, err
		}
		// Normal exit with any code — completion, not a failure (DINV-3).
		return p.stdout.Bytes(), st.ExitCode(), nil
	}
	// No normal exit: signalled (e.g. killed) or an I/O failure — operational.
	if err == nil {
		err = errors.New("process did not exit normally")
	}
	return p.stdout.Bytes(), -1, err
}

func (p *osProcess) kill() error {
	// Stop the invocation's native descendants and delegated verifiers before
	// returning control to the checkpoint owner.
	var err error
	if p.lifetime != nil {
		err = p.lifetime.Kill()
	} else {
		err = procgroup.KillChecked(p.cmd, true)
	}
	if p.stopInvocation != nil {
		err = errors.Join(err, p.stopInvocation())
	}
	return err
}
