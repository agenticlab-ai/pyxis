// argus-stamp — the Argus authorship producer: the commit-time stamp that
// appends the contractual `Argus-Drive: <slug>` authorship trailer to every
// commit made with an active drive slug (docs/tech-design/argus-draft-attribution.md
// §3.2). It is the commit-message hook's entry point: the repo's
// core.hooksPath prepare-commit-msg shim delegates to it, and it rewrites the
// commit-message file git hands it, keyed on the active drive's slug the
// caller supplies ($ARGUS_DRIVE_SLUG). During draft preparation it refreshes
// the binding on already stamped commits after rebase.
//
// A commit authored outside any drive (no slug) is left untouched — it carries
// no trailer and derives as human downstream (§3.1). The stamp decision and the
// trailer contract live once in internal/argus/stamp; this main is thin I/O
// (read the file, stamp, write it back). Each supported CLI's commit-authoring
// path is wired to it by a pointer config (.pyxis/stamp), listed by `paths`.
//
// Verbs:
//
//	argus-stamp <commit-msg-file>   stamp the commit message git hands the hook (the hook entry)
//	argus-stamp paths               list each supported CLI and how its commit path is wired
//	argus-stamp key                 print the contractual authorship-trailer key
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/argus/stamp"
)

// gitWorktree locates the worktree the commit is being authored in — the tree
// GitBinding reads to compute the content binding. git runs the
// prepare-commit-msg hook with the working directory at the worktree top, so
// `git rev-parse --show-toplevel` resolves it (honoring GIT_DIR and linked
// worktrees). An explicit ARGUS_STAMP_WORKTREE overrides it (a hermetic test
// hook); a bare cwd is the last resort.
func gitWorktree() string {
	if w := strings.TrimSpace(os.Getenv("ARGUS_STAMP_WORKTREE")); w != "" {
		return w
	}
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		if w := strings.TrimSpace(string(out)); w != "" {
			return w
		}
	}
	return "."
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func usage(w io.Writer) int {
	_, _ = fmt.Fprintln(w, `usage: argus-stamp <commit-msg-file>   stamp the commit message git hands the hook (the hook entry)
       argus-stamp paths               list each supported CLI and how its commit path is wired
       argus-stamp key                 print the contractual authorship-trailer key
flags (stamp):
  --slug string   the active drive's problem slug (default $ARGUS_DRIVE_SLUG); empty → no trailer`)
	return 2
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "paths":
			return listPaths(stdout)
		case "key":
			_, _ = fmt.Fprintln(stdout, stamp.TrailerKey)
			return 0
		case "-h", "--help", "help":
			return usage(stdout)
		}
	}
	return stampFile(args, stderr)
}

// stampFile is the hook entry: parse flags, read the commit-message file the
// prepare-commit-msg hook was handed, stamp it keyed on the active drive's
// slug, and write it back in place. A missing file path or a malformed slug is
// an error that aborts the commit (a nonzero exit from prepare-commit-msg) —
// the producer fails closed on a real misconfiguration rather than author a
// mis-attributed commit. An empty slug is not an error: it is a commit outside
// a drive, left untouched with no trailer.
func stampFile(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("argus-stamp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	slug := fs.String("slug", os.Getenv("ARGUS_DRIVE_SLUG"), "the active drive's problem slug")
	if err := fs.Parse(args); err != nil {
		return usage(stderr)
	}
	// git's prepare-commit-msg hook is invoked as `<file> [<source> [<sha>]]`;
	// the stamper reads only the message file (the first positional) and ignores
	// git's source/sha hints.
	rest := fs.Args()
	if len(rest) < 1 {
		_, _ = fmt.Fprintln(stderr, "argus-stamp: a commit-message file path is required")
		return usage(stderr)
	}
	path := rest[0]

	message, err := os.ReadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "argus-stamp: reading commit-message file %q: %v\n", path, err)
		return 1
	}
	if os.Getenv("ARGUS_STAMP_REBASE") == "1" {
		replayedSlug, stamped, parseErr := attribution.RebaseSlug(string(message))
		if parseErr != nil {
			_, _ = fmt.Fprintf(stderr, "argus-stamp: %v\n", parseErr)
			return 1
		}
		if !stamped {
			return 0
		}
		*slug = replayedSlug
	}

	// A drive commit (a non-empty slug) must carry the in-tree content binding —
	// compute it from the worktree's git state before stamping. A commit outside
	// a drive (empty slug) is left untouched and needs no binding (§3.1, §3.2).
	var binding string
	if strings.TrimSpace(*slug) != "" {
		binding, err = stamp.GitBinding(gitWorktree())
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "argus-stamp: %v\n", err)
			return 1
		}
	}

	stamped, err := stamp.Stamp(string(message), *slug, binding)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "argus-stamp: %v\n", err)
		return 1
	}
	if stamped == string(message) {
		return 0 // no change (no drive, or already stamped) — leave the file untouched
	}
	if err := os.WriteFile(path, []byte(stamped), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "argus-stamp: writing commit-message file %q: %v\n", path, err)
		return 1
	}
	return 0
}

// listPaths prints each supported CLI's commit-authoring path and how the
// producer is wired for it — single-sourced from the adapter registry, so the
// wiring is discoverable and never silently absent (mirrors the write hook's
// `guarantees` listing).
func listPaths(w io.Writer) int {
	_, _ = fmt.Fprintf(w, "argus authorship producer — supported CLIs and commit-path wiring (trailer %s)\n\n", stamp.TrailerKey)
	for _, a := range stamp.Adapters() {
		_, _ = fmt.Fprintf(w, "  %-8s commit path: %s\n", a.Name, a.CommitPath)
		_, _ = fmt.Fprintf(w, "  %-8s wiring:      %s\n\n", "", a.Wiring)
	}
	return 0
}
