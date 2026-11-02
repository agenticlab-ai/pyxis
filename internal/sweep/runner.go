// This file owns the maintenance sweep's toolchain seam (maintenance-sweep-v1
// §3.9): the injectable runner every detector's go/git subprocess reads sit
// behind, and the evaluated-state value the detectors are pure functions of
// (§3.2). The seam is the substitution point the hermetic fixtures use to run
// the real toolchain against temp modules; the production runner (ExecRunner)
// is the only place the sweep touches a subprocess, and it passes only the
// read-only subcommands the detectors form — the read-only contract made
// structural (DINV-1): the package holds no write-capable client. Stdlib only.
package sweep

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// Runner is the injectable seam the toolchain reads sit behind (design §3.9).
// Run executes one read-only command (go or git) in dir and returns its stdout
// split into non-empty lines, or a non-nil error wrapping the command's stderr.
// A Runner never mutates the repository: production runs pass only the read-only
// subcommands the detectors form, and no writer is reachable from here (DINV-1).
// It is shared by all three detectors — the go reads (unwired) and the git reads
// (rollup-staleness) both flow through this one seam.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) ([]string, error)
}

// ExecRunner is the production runner: it execs the named tool with os/exec in
// dir and returns its stdout lines. It is the sweep's single subprocess surface;
// it holds no writer and issues only the detectors' read-only go/git subcommands
// (DINV-1). The zero value is ready to use.
type ExecRunner struct{}

// Run execs name with args in dir, honoring ctx cancellation. On a non-zero
// exit it returns an error naming the command and its trimmed stderr — the
// fail-loud posture a detector turns into a named in-band fault (DINV-3/DINV-4).
func (ExecRunner) Run(ctx context.Context, dir, name string, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, msg)
	}
	return splitLines(stdout.String()), nil
}

// splitLines splits toolchain stdout into lines, dropping empty ones so a
// trailing newline never yields a spurious "" element the detectors must guard.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// State is the evaluated repository state a detector reads: the checkout root
// (filesystem reads) and the runner seam (toolchain subprocesses). It is the
// "evaluated state" half of every detector's pure (state, policy) signature
// (design §3.2) — a detector consults nothing else, and it never writes.
type State struct {
	Root   string
	Runner Runner
	// Config is the evaluated checkout's resolved configuration surface — the
	// capability paths and rollup keys the staleness detector reads live here,
	// never in policy (design §3.2).
	Config config.Config
}

// Detector is one mechanical rot detector: a pure function of the evaluated
// state and the policy yielding one DetectorResult (design §3.2). A fault is
// named in the result's Fault field, never returned out-of-band, so the pass
// can keep it in-band and let the remaining detectors complete (DINV-4). All
// three detectors share this signature.
type Detector func(ctx context.Context, state State, policy Policy) DetectorResult
