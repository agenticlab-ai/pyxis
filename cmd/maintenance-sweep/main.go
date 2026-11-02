// maintenance-sweep — the thin, verb-less binary for one bounded, read-only
// maintenance sweep pass (maintenance-sweep-v1 §3.1; shape:maintenance-sweep-v1).
// One invocation is one pass: it pins the evaluated state, loads the closed-parse
// policy, runs the three detectors in fixed order each isolated, emits one
// maintenance-report/v1 machine face on stdout and the human face on stderr, and
// exits by the pinned ranking — 0 clean, 1 findings, 2 fault. The invocation is
// fail-closed: an unknown flag or a flag-like positional refuses with usage and
// the fault exit code before any detector runs (DINV-3, incident I3). It writes
// nothing beyond stdout/stderr (DINV-1). Contract: docs/prds/maintenance-sweep/
// + docs/tech-design/maintenance-sweep-v1.md + the shape.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/sweep"
)

var resolveAbs = filepath.Abs
var processExit = os.Exit

func main() {
	processExit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usageText = `usage: maintenance-sweep [--repo <root>]

One bounded, read-only maintenance sweep pass over the checkout at <root>
(default: the current directory). Emits one maintenance-report/v1 JSON object on
stdout and the human face on stderr.

flags:
  --repo <root>   the checkout root to sweep (default: the current directory)

exit codes:
  0  clean     every detector ran, zero findings, zero faults
  1  findings  at least one finding emitted (a coexisting fault stays named)
  2  fault     zero findings and any fault — usage/misparse, unreadable repo
               state, malformed policy (incl. a reasonless waiver), or a
               detector fault`

// run parses the invocation fail-closed, runs the one pass, and returns the exit
// code by the pinned ranking. An invocation misparse — an unknown flag or any
// positional argument — refuses with usage and exit 2 before the pass begins
// (DINV-3): no machine face is emitted, so nothing is mistakable for a clean
// sweep.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("maintenance-sweep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, usageText) }
	repo := fs.String("repo", ".", "the checkout root to sweep (default: the current directory)")

	if err := fs.Parse(args); err != nil {
		// flag already printed the error and the usage text to stderr. No machine
		// face is emitted — nothing on stdout is mistakable for a clean sweep.
		return 2
	}
	if fs.NArg() > 0 {
		// The sweep is verb-less and takes no positional arguments; a leftover
		// argument (flag-like or not) is a misparse, not a silent ignore (I3).
		fmt.Fprintf(stderr, "maintenance-sweep: unexpected argument %q — the sweep takes no positional arguments, only --repo <root>\n", fs.Arg(0))
		fs.Usage()
		return 2
	}

	root, err := resolveAbs(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "maintenance-sweep: cannot resolve --repo %q: %v\n", *repo, err)
		fs.Usage()
		return 2
	}

	report := sweep.Run(context.Background(), root, sweep.ExecRunner{})

	// The machine face on stdout. A marshal failure means a finding breached its
	// four-field contract — a fault, never a clean-looking emit (DINV-5).
	machine, err := report.Marshal()
	if err != nil {
		fmt.Fprintf(stderr, "maintenance-sweep: report marshal failed: %v\n", err)
		return 2
	}
	fmt.Fprintln(stdout, string(machine))

	// The human face on stderr — the same report, explicit on emptiness.
	sweep.WriteHuman(stderr, report)

	return report.ExitCode()
}
