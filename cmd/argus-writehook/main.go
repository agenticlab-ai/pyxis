// argus-writehook — the write-time protected-path deny shim each supported
// agent CLI's own pre-write hook points at (docs/tech-design/argus-write-hook.md).
// It reads the CLI's real hook request on stdin, resolves the drive's declared
// scope, runs the one canonical deny decision (internal/argus/writehook), and
// renders that CLI's real deny/allow response — fail closed on every error path
// (an unparseable request, an unresolvable scope, a panic): a wedged or
// crashing hook denies, never fails open (§3.3, AP-17, DINV-2/3).
//
// This is the fast layer, not the unbypassable one: it cannot stop an
// out-of-band write (a raw shell redirect, a non-honoring CLI). The
// merge-boundary layers (the authority-contract structure arm and the
// attribution gate) carry that guarantee.
//
// Verbs:
//
//	argus-writehook --cli=<name>   evaluate the pending write on stdin (the hook entry)
//	argus-writehook guarantees     list each supported CLI and its stated guarantee
//	argus-writehook version        print the protected-path policy data version
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/argus/writehook"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

var processExit = os.Exit

func main() {
	processExit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func usage(w io.Writer) int {
	fmt.Fprintln(w, `usage: argus-writehook [flags]        evaluate the pending write on stdin (the per-CLI hook entry)
       argus-writehook guarantees     list each supported CLI and its stated guarantee
       argus-writehook version        print the protected-path policy data version
flags (evaluate):
  --cli string    the supported CLI whose pre-write hook request is on stdin (required)
  --slug string   the drive's problem slug (default $ARGUS_DRIVE_SLUG)
  --first-pr      resolve scope as a brand-new problem's first documentation PR (default $ARGUS_DRIVE_FIRST_PR)
  --base string   committed base ref for committed-ledger scope (default $ARGUS_DRIVE_BASE or the config mainline ref, e.g. origin/main)`)
	return 2
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "guarantees":
			return listGuarantees(stdout)
		case "version":
			fmt.Fprintln(stdout, boundary.DataVersion)
			return 0
		case "-h", "--help", "help":
			return usage(stdout)
		}
	}
	return evaluate(args, stdin, stdout, stderr)
}

// evaluate is the hook entry: resolve the drive, run the pending write on stdin
// through the selected CLI's adapter, and render the CLI's response. The whole
// body is guarded fail-closed — a panic denies with the CLI's hard-deny exit,
// never a bare crash that a fail-open CLI would let through (§3.3, DINV-3).
func evaluate(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	// Top-level fail-closed guard: any panic in load/parse/decide/render denies
	// with the CLI's hard-deny exit code (Claude's default is fail-open, so a bare
	// nonzero would let the write through — 2 blocks it). Installed FIRST — before
	// the config load and flag parse below — so those steps are covered too; the
	// deny render is CLI-agnostic and needs no resolved --cli (§3.3, DINV-3).
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(stderr, writehook.DenyForHookError("", fmt.Errorf("hook panicked: %v", r)).Reason())
			code = 2
		}
	}()

	// The target-facing doc roots the classification is keyed on, AND the default
	// committed base ref, resolve from the repo config (config-driven-portability-v1
	// §3.2, FR-1/FR-2). The hook runs at the repo root — the same cwd `git show`
	// reads from — so config loads from ".". An absent or unreadable config yields
	// the defaults (today's locations, origin/main), keeping the hook byte-identical
	// under a default config; the closed-schema validation is the gate's job, and
	// the merge-boundary attribution gate is the unbypassable backstop, so the fast
	// hook does not fail on a config parse defect.
	cfg, _ := config.Load(".")

	fs := flag.NewFlagSet("argus-writehook", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cli := fs.String("cli", "", "the supported CLI whose pre-write hook request is on stdin")
	slug := fs.String("slug", os.Getenv("ARGUS_DRIVE_SLUG"), "the drive's problem slug")
	firstPR := fs.Bool("first-pr", os.Getenv("ARGUS_DRIVE_FIRST_PR") != "", "resolve scope as a first documentation PR")
	// The committed base default resolves the config's mainline ref (<remote>/<branch>,
	// origin/main on a default config) so a repo whose mainline is not origin/main is
	// honored; $ARGUS_DRIVE_BASE (CI) and an explicit --base still override.
	base := fs.String("base", envOr("ARGUS_DRIVE_BASE", cfg.MainlineRef()), "committed base ref for committed-ledger scope")
	if err := fs.Parse(args); err != nil {
		return usage(stderr)
	}

	adapter, ok := writehook.Lookup(*cli)
	if !ok {
		// Unknown/absent CLI: fail closed — refuse rather than silently allow.
		fmt.Fprintln(stderr, writehook.DenyForHookError("",
			fmt.Errorf("no adapter for CLI %q; supported: %s", *cli, strings.Join(adapterNames(), ", "))).Reason())
		return 2
	}

	request, err := io.ReadAll(stdin)
	if err != nil {
		// Even a stdin read failure routes through the adapter's fail-closed
		// deny (an empty request parses as an error → deny).
		request = nil
	}

	drive := resolveDrive(cfg, *slug, *firstPR, *base, gitShowReader(*base))
	return adapter.Evaluate(cfg, request, drive, stdout)
}

// resolveDrive resolves the drive's declared scope from author-independent
// sources, reusing T1's resolver (internal/argus/boundary). An absent slug or
// an unresolvable ledger yields a nil scope, which the canonical decision maps
// to a fail-closed Unresolvable deny (§3.3, DINV-2). readCommitted supplies the
// committed (merge-base) delivery-ledger bytes — never the author-writable
// same-PR working tree — so the committed-ledger width stays author-independent.
// The problems root the ledger lives under resolves from cfg
// (config-driven-portability-v1 §3.2, FR-1; default docs/problems).
func resolveDrive(cfg config.Config, slug string, firstPR bool, base string, readCommitted func(path string) ([]byte, bool)) boundary.Drive {
	if strings.TrimSpace(slug) == "" {
		return boundary.Drive{} // no drive / no scope → Unresolvable deny
	}
	if firstPR {
		return boundary.Drive{Slug: slug, Scope: boundary.ResolveFirstPR(cfg, slug)}
	}
	committed, ok := readCommitted(strings.TrimPrefix(filepath.ToSlash(filepath.Clean(cfg.Problems)), "./") + "/" + slug + "/delivery.md")
	if !ok {
		return boundary.Drive{Slug: slug} // no committed ledger, not first-PR → unresolvable
	}
	sc, ok := boundary.ResolveCommittedLedger(cfg, slug, committed)
	if !ok {
		return boundary.Drive{Slug: slug} // unparseable committed ledger → unresolvable
	}
	return boundary.Drive{Slug: slug, Scope: sc}
}

// gitShowReader reads a repo path's bytes from a committed ref (the merge base),
// never the working tree — the committed delivery ledger a current drive cannot
// retroactively widen (boundary §3.2.1, DINV-6).
func gitShowReader(base string) func(path string) ([]byte, bool) {
	return func(path string) ([]byte, bool) {
		out, err := exec.Command("git", "show", base+":"+path).Output()
		if err != nil {
			return nil, false
		}
		return out, true
	}
}

// listGuarantees prints each supported CLI and its stated guarantee — a
// honoring adapter's write-time fail-closed deny, or a non-honoring CLI's
// merge-boundary backstop (DINV-4, UB-2). Single-sourced from the registry.
func listGuarantees(w io.Writer) int {
	fmt.Fprintf(w, "argus write-hook — supported CLIs and stated guarantees (policy %s)\n\n", boundary.DataVersion)
	for _, a := range writehook.Adapters() {
		if a.Honoring {
			fmt.Fprintf(w, "  %-8s honoring pre-write hook: %s\n", a.Name, a.Hook)
			fmt.Fprintf(w, "  %-8s guarantee: write-time protected-path deny, fail-closed\n\n", "")
			continue
		}
		fmt.Fprintf(w, "  %-8s %s\n", a.Name, a.Hook)
		fmt.Fprintf(w, "  %-8s guarantee: %s\n\n", "", a.Backstop)
	}
	return 0
}

func adapterNames() []string {
	var out []string
	for _, a := range writehook.Adapters() {
		out = append(out, a.Name)
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
