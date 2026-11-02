// Package runner executes one repo-supplied verb under the bounded contract
// DINV-5 requires: an environment built ONLY from the verb's declared entries
// (the ambient environment is unreachable by construction), a wall-clock
// bound plus a fixed termination grace, and a classification that keeps "the
// process could not be started or kept running" (UB-1/UB-2, infra) distinct
// from "the process ran and said no" (a check's own red).
//
// Verbs execute by direct argv exec, never through a shell: a missing binary
// must surface as a Start() error (UB-1), not a shell's synthesized exit code
// (e.g. 127) that would be indistinguishable from a legitimate check failure.
// This also keeps a run deterministic (NFR-2) — no shell profile or alias can
// change what a verb resolves to.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// DefaultTimeout and DefaultGrace bound a verb whose manifest entry leaves
// TimeoutS/GraceS unset. DefaultGrace is the termination-grace constant
// design:codegate-v1#7 Q3 fixes in code at first release.
const (
	DefaultTimeout = 5 * time.Minute
	DefaultGrace   = 5 * time.Second
)

// maxCapturedBytes caps in-memory stdout/stderr capture per stream so a
// runaway verb cannot exhaust memory before its bound fires; the cap is
// generous relative to any realistic lint/build/test output.
const maxCapturedBytes = 8 << 20 // 8 MiB

// Class distinguishes why a verb execution failed to produce a clean
// pass/fail. It is empty for a completed run — read Passed/ExitCode instead.
type Class string

const (
	// ClassInfra marks a verb that could not be spawned, or could not be
	// waited on for a reason other than its bound (UB-1): a missing binary, a
	// permission error, or any other exec-layer failure.
	ClassInfra Class = "infra"
	// ClassTimeout marks a verb that exceeded its wall-clock bound plus the
	// termination grace and was killed (UB-2).
	ClassTimeout Class = "timeout"
	// ClassViolation marks a verb that attempted a filesystem write outside
	// its confined scope (workspace + scratch) where the platform enforces
	// write-scope confinement (design codegate-v1 §3.5, DINV-5): a rung
	// error distinct from a plain check red, never conflated with either
	// ClassInfra (an execution-layer fault unrelated to what the verb wrote)
	// or a clean check failure.
	ClassViolation Class = "violation"
)

// Result is one verb execution's outcome.
type Result struct {
	ExitCode int
	Passed   bool // true iff the process ran to completion with ExitCode == 0
	Class    Class
	Stdout   string
	Stderr   string
	Duration time.Duration
	Err      error // the spawn/timeout/wait fault; nil for a completed run
}

// Exec runs verb's command in workspace, bounded by its declared (or
// default) timeout plus the fixed termination grace, with an environment
// built only from verb's declared entries (DINV-5). ctx governs the whole
// call: if it is cancelled before the verb finishes, the verb is killed
// immediately and the result classifies as infra.
func Exec(ctx context.Context, workspace string, verb *schema.Verb) Result {
	start := time.Now()

	argv, err := splitCommand(verb.Command)
	if err != nil {
		return Result{Class: ClassInfra, Duration: time.Since(start), Err: fmt.Errorf("parse command %q: %w", verb.Command, err)}
	}

	timeout := boundDuration(verb.TimeoutS, DefaultTimeout)
	grace := boundDuration(verb.GraceS, DefaultGrace)

	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec // repo-supplied verb, run directly (never via shell) so a missing binary is a Start() error, not a shell exit code.
	cmd.Dir = workspace
	cmd.Env = buildEnv(verb)
	// Bound the post-exit/post-kill pipe drain: after the process ends (or is
	// killed at the bound), wait at most `grace` for the captured stdout/stderr
	// pipes to close, then force them shut and let cmd.Wait return. Without this
	// a verb whose descendant escaped the process group and still holds the pipe
	// write-end wedges cmd.Wait — and the whole run — forever, since the timeout
	// path's SIGKILL to the group misses an escaped grandchild.
	cmd.WaitDelay = grace

	var stdout, stderr capBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	setpgid(cmd)
	if err := executorruntime.ConfineVerificationCommand(ctx, cmd); err != nil {
		return Result{Class: ClassInfra, Duration: time.Since(start), Err: fmt.Errorf("protect verification Git metadata: %w", err)}
	}

	// Confine rewrites cmd (where the platform enforces it) so the process it
	// starts can write only beneath the workspace and the toolchain's own
	// scratch/cache dirs (WriteAllowedRoots) — DINV-5's write-scope leg. This
	// must run AFTER every other cmd field is set (Confine only ever touches
	// Path/Args) and BEFORE Start, and confined gates whether a completed-but-
	// failed run below gets scanned for a violation signature.
	confined, err := Confine(cmd, WriteAllowedRoots(workspace))
	if err != nil {
		return Result{Class: ClassInfra, Duration: time.Since(start), Err: fmt.Errorf("confine: %w", err)}
	}

	if err := cmd.Start(); err != nil {
		return Result{Class: ClassInfra, Duration: time.Since(start), Err: fmt.Errorf("spawn %q: %w", verb.Command, err)}
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	class, waitErr := waitForExit(ctx, done, timeout, grace, func(force bool) { killGroup(cmd, force) })
	if class == ClassTimeout {
		waitErr = fmt.Errorf("verb %q exceeded its %s bound (+%s grace)", verb.Command, timeout, grace)
	}
	if class != "" {
		return finalizeConfinement(confined, Result{Class: class, Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(start), Err: waitErr})
	}
	return finalizeConfinement(confined, classify(waitErr, stdout.String(), stderr.String(), time.Since(start)))
}

// waitForExit separates the timer/signal policy from OS process startup so its
// deadline and grace transitions can be exercised with virtual time. done is
// the process's actual Wait acknowledgment on the production path.
func waitForExit(ctx context.Context, done <-chan error, timeout, grace time.Duration, kill func(force bool)) (Class, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		kill(true) // caller cancellation: no grace budget applies here.
		<-done
		return ClassInfra, ctx.Err()

	case waitErr := <-done:
		return "", waitErr

	case <-timer.C:
		kill(false) // SIGTERM (or the platform's best-effort equivalent)
		graceTimer := time.NewTimer(grace)
		defer graceTimer.Stop()
		select {
		case <-done:
		case <-graceTimer.C:
			kill(true) // SIGKILL
			<-done
		}
		return ClassTimeout, nil
	}
}

// classify turns cmd.Wait's error into a Result: nil is a clean pass, an
// *exec.ExitError is a completed-but-nonzero run (a check's own red, never
// infra), and anything else is an execution-layer fault (infra).
func classify(waitErr error, stdout, stderr string, dur time.Duration) Result {
	if waitErr == nil {
		return Result{ExitCode: 0, Passed: true, Stdout: stdout, Stderr: stderr, Duration: dur}
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		if exitErr.ExitCode() < 0 {
			// A negative code means the process was terminated by a signal
			// (SIGSEGV/SIGABRT/OOM-kill), not a clean nonzero exit: the tool
			// never rendered a verdict, so this is an execution fault (infra),
			// never the check's own red.
			return Result{Class: ClassInfra, Stdout: stdout, Stderr: stderr, Duration: dur, Err: fmt.Errorf("verb terminated by signal: %w", waitErr)}
		}
		return Result{ExitCode: exitErr.ExitCode(), Passed: false, Stdout: stdout, Stderr: stderr, Duration: dur}
	}
	return Result{Class: ClassInfra, Stdout: stdout, Stderr: stderr, Duration: dur, Err: fmt.Errorf("wait: %w", waitErr)}
}

// MaxBound is the runner-enforced ceiling on any single verb's wall-clock
// bound, independent of the manifest: untrusted repo input cannot declare an
// effectively-infinite bound (nor, via int64-nanosecond overflow, a negative
// one that would fire the timer immediately and kill a healthy verb).
const MaxBound = time.Hour

// boundDuration returns seconds as a Duration, clamped to (0, MaxBound]: a
// non-positive request falls back to def; a converted value that overflowed to
// non-positive, or that exceeds the ceiling, is capped at MaxBound.
func boundDuration(seconds int, def time.Duration) time.Duration {
	if seconds <= 0 {
		return def
	}
	d := time.Duration(seconds) * time.Second
	if d <= 0 || d > MaxBound {
		return MaxBound
	}
	return d
}

// buildEnv constructs the child's environment from ONLY verb's declared
// entries (DINV-5): a pass-through name inherits the ambient value (when
// set), a pinned entry is fixed regardless of the ambient environment. The
// result is always a non-nil slice — including when empty — so exec.Cmd never
// falls back to inheriting the current process's full environment (a nil
// Cmd.Env means "inherit everything," which strip-by-default must never do).
func buildEnv(verb *schema.Verb) []string {
	env := make([]string, 0, len(verb.Env)+1)
	declared := make(map[string]bool, len(verb.Env))
	for name, entry := range verb.Env {
		declared[name] = true
		switch entry.Mode {
		case schema.EnvPin:
			env = append(env, name+"="+entry.Value)
		case schema.EnvPass:
			if v, ok := os.LookupEnv(name); ok {
				env = append(env, name+"="+v)
			}
		}
	}
	// A non-interactivity guard, not an ambient leak: pin GIT_TERMINAL_PROMPT=0
	// so a verb whose git operation (e.g. a private-module fetch under `go
	// build`) would otherwise open /dev/tty for credentials fails fast instead
	// of blocking on a prompt until the wall-clock bound — or, on the attended
	// surface, hijacking the operator's terminal. A verb that explicitly
	// declares the variable wins.
	if !declared["GIT_TERMINAL_PROMPT"] {
		env = append(env, "GIT_TERMINAL_PROMPT=0")
	}
	sort.Strings(env) // NFR-2 determinism: a stable child environment ordering.
	return env
}

// splitCommand tokenizes a verb's command-line string into argv, honoring
// single- and double-quoted segments so a path or argument containing a space
// can be expressed (e.g. `eslint "src/my file.js"`). It performs NO shell
// interpretation — no globbing, no variable expansion, no operators; see the
// package doc for why verbs never execute through a shell.
func splitCommand(s string) ([]string, error) {
	var argv []string
	var cur bytes.Buffer
	inArg := false
	var quote rune

	flush := func() {
		if inArg {
			argv = append(argv, cur.String())
			cur.Reset()
			inArg = false
		}
	}

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inArg = true
		case r == ' ' || r == '\t':
			flush()
		default:
			inArg = true
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in command: %q", s)
	}
	flush()

	if len(argv) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return argv, nil
}

// capBuffer is a bytes.Buffer capped at maxCapturedBytes: writes past the cap
// are dropped (with a one-time truncation marker) rather than growing
// unbounded — a runaway verb's output must never be able to exhaust memory
// before its wall-clock bound fires. Write always reports success so the
// child is never blocked or errored by its own (over-)output.
type capBuffer struct {
	buf       bytes.Buffer
	truncated bool
}

func (c *capBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxCapturedBytes - c.buf.Len()
	switch {
	case remaining <= 0:
		if !c.truncated {
			c.truncated = true
			c.buf.WriteString("\n... [output truncated]\n")
		}
	case len(p) > remaining:
		c.buf.Write(p[:remaining])
		c.truncated = true
		c.buf.WriteString("\n... [output truncated]\n")
	default:
		c.buf.Write(p)
	}
	return n, nil
}

func (c *capBuffer) String() string { return c.buf.String() }
