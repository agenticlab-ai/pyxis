// Command code-gate is the single-source code quality gate: it runs a repo's
// code-gate check list (from .pyxis/config.yaml) through the shared
// internal/verify runner and returns one verdict. Every surface — the executor
// gate, the pre-push hook, and required CI — invokes this same verb. CI may
// partition the list; ci-parity verifies its complete required-check coverage
// (docs/tech-design/code-gate-mirror.md, DINV-1). It is distinct from the
// existing `codegate` verification-ladder binary and never runs the doc-gate.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
)

// Process exit codes. Fail closed: any inconclusive outcome is non-zero.
const (
	exitPass      = 0 // every required check passed (advisory reds allowed)
	exitRed       = 1 // a required check failed
	exitConfigErr = 2 // config missing/malformed, or a usage error
)

var resolveRepoRoot = repoRoot

func main() {
	// The hidden confinement re-exec mode: the runner re-execs THIS binary with
	// runner.ConfineExecArg to apply Linux write-scope confinement (opt-in, off
	// by default), exactly as cmd/codegate does. Checked before any normal
	// parsing; never a documented subcommand.
	if len(os.Args) >= 2 && os.Args[1] == runner.ConfineExecArg {
		runner.RunConfinedExec(os.Args[2:])
		return
	}
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

// dispatch routes the command line. `run` (the default) runs the gate; `list`
// prints the resolved check set without running; version/help are informational.
// `run --no-cache` (or a bare `--no-cache`) forces a fresh run, bypassing the
// verdict cache.
func dispatch(args []string, stdout, stderr io.Writer) int {
	verb := "run"
	if len(args) > 0 {
		verb = args[0]
	}
	switch verb {
	case "run":
		root, err := resolveRepoRoot()
		if err != nil {
			return writeResult(stderr, exitConfigErr, "code-gate: %v\n", err)
		}
		runArgs := args
		if len(runArgs) > 0 {
			runArgs = runArgs[1:]
		}
		return runWithFlags(root, runArgs, stdout, stderr)
	case "--no-cache":
		root, err := resolveRepoRoot()
		if err != nil {
			return writeResult(stderr, exitConfigErr, "code-gate: %v\n", err)
		}
		return runWithFlags(root, args, stdout, stderr)
	case "list", "--list":
		root, err := resolveRepoRoot()
		if err != nil {
			return writeResult(stderr, exitConfigErr, "code-gate: %v\n", err)
		}
		return listGate(root, stdout, stderr)
	case "version", "--version", "-v":
		return writeResult(stdout, exitPass, "code-gate v1\n")
	case "help", "--help", "-h":
		return usage(stdout)
	default:
		writeResult(stderr, exitConfigErr, "code-gate: unknown command %q\n\n", verb)
		usage(stderr)
		return exitConfigErr
	}
}

// runGate loads the code-gate config under root and runs it, reusing a cached
// green verdict for an unchanged tree (DINV-6) unless noCache forces a fresh run.
// It prints a human-readable report to stderr and returns the exit code; stdout
// is left clean so a caller can capture it.
func runGate(root string, noCache bool, _, stderr io.Writer) int {
	return runSelectedGate(root, noCache, "", "", "", false, 0, stderr)
}

func runWithFlags(root string, args []string, _, stderr io.Writer) int {
	fs := flag.NewFlagSet("code-gate run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	noCache := fs.Bool("no-cache", false, "run without a cached verdict")
	check := fs.String("check", "", "run comma-separated named checks from the shared configuration")
	exclude := fs.String("exclude-check", "", "run every shared check except the comma-separated names")
	group := fs.String("group", "", "run one shared execution group")
	cheap := fs.Bool("cheap", false, "run phase-1 checks only (skip memory and coverage)")
	phase := fs.Int("phase", 0, "run one execution phase; may combine with --group")
	if err := fs.Parse(args); err != nil {
		return exitConfigErr
	}
	if fs.NArg() != 0 {
		return writeResult(stderr, exitConfigErr, "code-gate: run accepts flags only\n")
	}
	var selectionFlags int
	phaseSet := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "check", "exclude-check", "cheap", "group":
			selectionFlags++
		case "phase":
			phaseSet = true
			if *group == "" {
				selectionFlags++
			}
		}
	})
	if phaseSet && *phase < 1 {
		return writeResult(stderr, exitConfigErr, "code-gate: --phase must be >= 1\n")
	}
	if selectionFlags > 1 || (selectionFlags == 1 && !*cheap && *check == "" && *exclude == "" && *group == "" && *phase == 0) {
		return writeResult(stderr, exitConfigErr, "code-gate: use one non-empty --group, --check, --exclude-check, or --cheap selection; --phase may combine with --group\n")
	}
	return runSelectedGate(root, *noCache, *check, *exclude, *group, *cheap, *phase, stderr)
}

func runSelectedGate(root string, noCache bool, check, exclude, group string, cheap bool, phase int, stderr io.Writer) int {
	cfg, err := codegatemirror.Load(root)
	if err != nil {
		return writeResult(stderr, exitConfigErr, "code-gate: %v\n", err)
	}
	if cheap {
		cfg, err = cfg.SelectCheap()
	} else if group != "" && phase > 0 {
		cfg, err = cfg.SelectGroupPhase(group, phase)
	} else if group != "" {
		cfg, err = cfg.SelectGroup(group)
	} else if phase > 0 {
		cfg, err = cfg.SelectPhase(phase)
	} else if exclude != "" {
		cfg, err = cfg.SelectExcept(exclude)
	} else {
		cfg, err = cfg.Select(check)
	}
	if err != nil {
		return writeResult(stderr, exitConfigErr, "%v\n", err)
	}
	if len(cfg.Checks) == 0 {
		return writeResult(stderr, exitPass, "code-gate: no checks configured — empty gate, vacuously green\n")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	codegatemirror.PrefetchMainline(ctx, root, stderr)
	if noCache {
		return printReport(cfg.Run(ctx, root), stderr)
	}
	rep, hit, _ := cfg.RunCached(ctx, root)
	if hit {
		return writeResult(stderr, exitPass, "code-gate: cached — unchanged tree, green\n")
	}
	return printReport(rep, stderr)
}

// printReport renders each check's outcome to stderr and returns the exit code.
// The one-line-per-check table is the summary; every non-passing check then gets
// its captured output printed in full beneath it. A summary line alone cannot
// diagnose a red check, and a gate whose failure cannot be read from its own log
// sends the reader to reproduce it locally — the truncation lesson of incident
// I3, and what code-gate-mirror §3.4 requires a failure to surface.
func printReport(rep codegatemirror.Report, stderr io.Writer) int {
	var lines []string
	for _, o := range rep.Outcomes {
		status := "ok"
		if !o.Passed {
			status = "FAIL"
			if o.Class == codegatemirror.ClassAdvisory {
				status = "warn"
			}
		}
		line := fmt.Sprintf("  %-4s  %-18s %8s", status, o.Name, o.Duration.Round(time.Millisecond))
		if o.Detail != "" {
			line += "  — " + o.Detail
		}
		lines = append(lines, line)
	}
	for _, o := range rep.Outcomes {
		if o.Passed || o.Output == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("\n--- %s: captured output ---\n%s\n--- end %s ---", o.Name, o.Output, o.Name))
	}
	code := exitRed
	if rep.Passed {
		lines = append(lines, "code-gate: PASS")
		code = exitPass
	} else {
		lines = append(lines, "code-gate: FAIL — a required check is red or execution was cancelled")
	}
	return writeResult(stderr, code, "%s\n", strings.Join(lines, "\n"))
}

// listGate prints the resolved check set to stdout (the queryable output).
func listGate(root string, stdout, stderr io.Writer) int {
	cfg, err := codegatemirror.Load(root)
	if err != nil {
		return writeResult(stderr, exitConfigErr, "code-gate: %v\n", err)
	}
	for _, chk := range cfg.Checks {
		if code := writeResult(stdout, exitPass, "%-18s [%s, fail-on:%s, group:%s, phase:%d] %s\n", chk.Name, chk.Class, chk.FailOn, chk.ExecutionGroup(), chk.ExecutionPhase(), chk.Command); code != exitPass {
			return code
		}
	}
	return exitPass
}

// hasFlag reports whether flag appears in args.
func hasFlag(args []string, flag string) bool {
	return slices.Contains(args, flag)
}

// A verdict whose output could not be delivered must not advertise success.
func writeResult(w io.Writer, code int, format string, args ...any) int {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return exitRed
	}
	return code
}

// repoRoot resolves the repository root: the git top-level when available, else
// the working directory.
func repoRoot() (string, error) {
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		if root := strings.TrimSpace(string(out)); root != "" {
			return root, nil
		}
	}
	return os.Getwd()
}

func usage(w io.Writer) int {
	return writeResult(w, exitPass, "%s", `code-gate — the single-source code quality gate

usage:
  code-gate [run]        run the repo's code-gate checks; exit 0 pass, 1 required-red, 2 config error
  code-gate run --no-cache   run fresh, ignoring the verdict cache
  code-gate run --group NAME   run one shared group; the full gate runs groups concurrently per phase
  code-gate run --check NAME[,NAME...]   run named checks with shared group scheduling
  code-gate run --exclude-check NAME[,NAME...]   run all other configured checks
  code-gate run --cheap   run phase-1 checks only (gofmt/lint/vet/build; skip memory and coverage)
  code-gate run --phase N   run one barrier; combine with --group to finish a hosted group after --cheap
  code-gate list         print the resolved check set without running
  code-gate version
  code-gate help
`)
}
