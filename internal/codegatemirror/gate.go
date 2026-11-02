package codegatemirror

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// Outcome is one check's result.
type Outcome struct {
	Name     string
	Class    Class
	Passed   bool
	Duration time.Duration
	Detail   string // a short failure reason; empty on a pass
	// Output is the check's captured output, retained on failure and empty on a
	// pass. Detail alone cannot diagnose a red check — it is one line, and for a
	// tool that writes its findings to stdout there is no single line that says
	// what broke. This field is the evidence the design promises a failure will
	// surface (code-gate-mirror §3.4 "Failures name the failing check and
	// surface its captured output").
	Output  string
	LogPath string // private full diagnostic retained by the supervisor
}

// summaryTailLines bounds how much captured output FailureSummary inlines. The
// summary feeds a fixer agent's prompt (cmd/implement-now/heal.go), so it is
// bounded; the console report prints the output in full.
const summaryTailLines = 40

// Report is a whole code-gate run.
type Report struct {
	Outcomes []Outcome
	// Passed is the gate verdict: every REQUIRED check passed. An advisory
	// check's failure is recorded in Outcomes but never flips this (§3.10).
	Passed bool
}

// Run executes phases in ascending order. Inside a phase, groups run
// concurrently and each group's checks keep configuration order. A required
// check's failure cancels sibling groups and skips later phases (fail-fast);
// an advisory check is reported but never gates or cancels. ctx governs the
// run: a cancellation stops it and fails closed (the gate is then
// inconclusive, never a reported pass — DINV-4).
func (c Config) Run(ctx context.Context, workspace string) Report {
	return c.run(ctx, workspace, runCheck)
}

func (c Config) run(ctx context.Context, workspace string, execute func(context.Context, string, Check) Outcome) Report {
	rep := Report{Outcomes: make([]Outcome, len(c.Checks)), Passed: true}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var failedRequired atomic.Bool
	mark := func(i int, out Outcome) {
		rep.Outcomes[i] = out
		if !out.Passed && out.Class == ClassRequired {
			failedRequired.Store(true)
			cancel()
		}
	}
	for _, phase := range c.Phases() {
		if ctx.Err() != nil || failedRequired.Load() {
			break
		}
		c.runPhase(ctx, workspace, phase, execute, mark)
	}
	for i, check := range c.Checks {
		if rep.Outcomes[i].Name != "" {
			continue
		}
		detail := "cancelled before run: later phase not started"
		if err := ctx.Err(); err != nil {
			detail = "cancelled before run: " + err.Error()
		}
		rep.Outcomes[i] = Outcome{Name: check.Name, Class: check.Class, Detail: detail}
		emitProgress(ctx, check, "skipped", rep.Outcomes[i])
	}
	for _, out := range rep.Outcomes {
		if !out.Passed && out.Class == ClassRequired {
			rep.Passed = false
		}
	}
	// Even cancellation during an advisory check makes execution inconclusive.
	rep.Passed = rep.Passed && ctx.Err() == nil && !failedRequired.Load()
	return rep
}

func (c Config) runPhase(ctx context.Context, workspace string, phase int, execute func(context.Context, string, Check) Outcome, mark func(int, Outcome)) {
	groups := map[string][]int{}
	for i, check := range c.Checks {
		if check.ExecutionPhase() == phase {
			groups[check.ExecutionGroup()] = append(groups[check.ExecutionGroup()], i)
		}
	}
	var workers sync.WaitGroup
	for _, indices := range groups {
		workers.Go(func() {
			for _, i := range indices {
				check := c.Checks[i]
				if err := ctx.Err(); err != nil {
					out := Outcome{Name: check.Name, Class: check.Class, Detail: "cancelled before run: " + err.Error()}
					mark(i, out)
					emitProgress(ctx, check, "skipped", out)
					continue
				}
				emitProgress(ctx, check, "started", Outcome{})
				out := execute(ctx, workspace, check)
				if out.Output != "" {
					out.LogPath = retainCheckOutput(ctx, workspace, out.Output)
				}
				mark(i, out)
				status := "passed"
				if !out.Passed {
					status = "failed"
				}
				emitProgress(ctx, check, status, out)
			}
		})
	}
	workers.Wait()
}

// FailureSummary renders the gate's non-passing checks into one diagnostic — the
// signal a caller (e.g. the executor's self-heal loop) feeds, after redaction, to
// the next fixer when the local gate blocks a push. It names each failing check
// with its gating class, inlines its detail, and appends the tail of its captured
// output; an empty string means nothing failed.
//
// The output tail is not decoration. A fixer handed only the one-line detail is
// told which check went red but not what it said, and for a stdout-diagnosing
// tool that line can name a component that PASSED — which is how a red race
// suite reported a green cached package as its reason.
func (r Report) FailureSummary() string {
	var b strings.Builder
	for _, o := range r.Outcomes {
		if o.Passed {
			continue
		}
		fmt.Fprintf(&b, "check %q [%s] failed", o.Name, o.Class)
		if o.Detail != "" {
			fmt.Fprintf(&b, ": %s", o.Detail)
		}
		b.WriteString("\n")
		if o.LogPath != "" {
			fmt.Fprintf(&b, "full diagnostic: %s\n", o.LogPath)
		}
		if tail := failureExcerpt(o.Output); tail != "" {
			for l := range strings.SplitSeq(tail, "\n") {
				fmt.Fprintf(&b, "    %s\n", l)
			}
		}
	}
	return b.String()
}

func retainCheckOutput(ctx context.Context, workspace, output string) string {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	f, err := runlogs.Create(ctx, runlogs.Repository(ctx, workspace), runlogs.Codegate, "check-*.log")
	if err != nil {
		slog.Warn("retain gate diagnostic", "error", err)
		return ""
	}
	_, writeErr := f.WriteString(output)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		slog.Warn("retain gate diagnostic", "path", f.Name(), "error", err)
		return ""
	}
	return f.Name()
}

// Multi-package tools can report their first failure long before the final
// successful package lines. Keep its context as well as the bounded tail.
func failureExcerpt(output string) string {
	lines := nonBlankLines(output)
	tail := tailLines(output, summaryTailLines)
	for i, line := range lines {
		if strings.HasPrefix(line, "--- FAIL:") || strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "fatal error:") {
			if i >= len(lines)-summaryTailLines {
				return tail
			}
			end := min(i+12, len(lines)-summaryTailLines)
			return strings.Join(lines[max(0, i-2):end], "\n") + "\n[intervening output omitted; see full diagnostic]\n" + tail
		}
	}
	return tail
}

// runCheck maps one check onto a runner verb, executes it, and applies the
// check's fail-on semantics. The runner supplies subprocess exec, env-strip,
// timeout+grace, and output capture (internal/verify/runner; the env-strip and
// write-confinement posture is §3.5) — code-gate reimplements none of it,
// layering only the non-empty-output failure class the runner leaves to its
// caller.
func runCheck(ctx context.Context, workspace string, chk Check) Outcome {
	verb := &schema.Verb{Command: chk.Command, Env: envFor(chk.Env), TimeoutS: chk.TimeoutS}
	if base := runtime.VerificationBase(ctx); base != "" {
		verb.Env["PYXIS_CI_BASE"] = schema.EnvEntry{Mode: schema.EnvPin, Value: base}
	}
	res := runner.Exec(ctx, workspace, verb)
	out := Outcome{Name: chk.Name, Class: chk.Class, Duration: res.Duration}

	// An execution-layer fault (spawn/timeout/write-violation) is a check
	// failure, never a clean pass: fail closed on it rather than read a verdict
	// the tool never rendered.
	if res.Class != "" {
		out.Passed, out.Detail = false, fmt.Sprintf("%s: %s", res.Class, oneLine(res.Err))
		out.Output = combinedOutput(res)
		return out
	}

	switch chk.FailOn {
	case FailOnOutput:
		// gofmt shape: a zero exit still fails if it listed offenders on stdout;
		// a non-zero exit is a genuine tool error (e.g. a parse failure).
		if !res.Passed {
			out.Passed, out.Detail = false, exitDetail(res)
			out.Output = combinedOutput(res)
			return out
		}
		if lines := nonBlankLines(res.Stdout); len(lines) > 0 {
			out.Passed = false
			out.Detail = fmt.Sprintf("%d line(s) of output; first: %s", len(lines), lines[0])
			out.Output = combinedOutput(res)
			return out
		}
		out.Passed = true

	default: // FailOnExit
		out.Passed = res.Passed
		if !res.Passed {
			out.Detail = exitDetail(res)
			out.Output = combinedOutput(res)
		}
	}
	return out
}

// combinedOutput joins a result's captured streams for the failure record. Both
// are kept and neither is preferred: which one carries the diagnosis is the
// tool's choice, not ours — `go vet` writes to stderr while `go test` writes its
// failures to stdout.
func combinedOutput(res runner.Result) string {
	parts := make([]string, 0, 2)
	for _, s := range []string{res.Stdout, res.Stderr} {
		if t := strings.TrimRight(s, "\n"); strings.TrimSpace(t) != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n")
}

// tailLines returns the last n non-blank-trimmed lines of s, or "" when empty.
// The TAIL, never the head: a tool announces what broke as it finishes, and the
// head of a multi-package run is whatever happened to run first — usually
// something that passed.
func tailLines(s string, n int) string {
	lines := nonBlankLines(s)
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// envFor builds a check's environment: the go/gofmt toolchain needs PATH and
// HOME (the runner strips everything else, §3.5), plus any names the check
// declares. This mirrors the ecosystem go-verb default proven on the Linux CI
// matrix (internal/verify/ecosystem goVerbEnv); the runner pins
// GIT_TERMINAL_PROMPT itself.
func envFor(extra []string) map[string]schema.EnvEntry {
	env := map[string]schema.EnvEntry{
		"PATH": {Mode: schema.EnvPass},
		"HOME": {Mode: schema.EnvPass},
	}
	for _, name := range extra {
		env[name] = schema.EnvEntry{Mode: schema.EnvPass}
	}
	return env
}

// exitDetail summarises a non-zero exit in one line.
//
// It reads STDERR only. A tool's diagnostics go to stderr, so its first line is
// the error; stdout carries the tool's ordinary output, whose first line says
// nothing about the failure. Reading stdout as a fallback is worse than reading
// nothing: `go test ./...` opens with the first package it finished, so a red
// race suite reported an `ok ... (cached)` line — a PASSING package — as its
// reason, and that string reached both the CI log and the self-heal fixer.
//
// When stderr is silent the exit code is the honest summary, and Outcome.Output
// carries the evidence rather than a guess at which line mattered.
func exitDetail(res runner.Result) string {
	if d := oneLineStr(res.Stderr); d != "" {
		return d
	}
	return fmt.Sprintf("exited %d", res.ExitCode)
}

// nonBlankLines returns the non-blank, trimmed lines of s.
func nonBlankLines(s string) []string {
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func oneLineStr(s string) string {
	for l := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			return t
		}
	}
	return ""
}

func oneLine(err error) string {
	if err == nil {
		return ""
	}
	return oneLineStr(err.Error())
}
