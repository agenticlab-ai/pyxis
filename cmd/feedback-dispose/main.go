// feedback-dispose — the review-feedback disposition pass
// (docs/tech-design/review-feedback-capture.md §3.4, PRD FR-3/UB-3/UB-5/UB-6).
// It runs one deterministic disposition pass over a subject's ledger entries:
// it folds the subject directory, asks the git adapter the resolving-event
// questions, and appends a typed disposition to each entry whose resolving event
// is now observed — applied (its recorded edit-commit landed), no-action (the
// subject concluded with no landed edit), leaving an unresolved entry pending
// and a deleted subject flagged stale. Every appended disposition names the
// observed event it was set from; nothing is closed by inference (DINV-3).
//
// The projection is NOT rewritten here — like feedback-capture, this binary only
// touches entry files; `docgate fold docs/feedback/<stem>` regenerates the
// byte-verified projection, keeping this pass off the fold's byte-verify contract.
//
// Usage:
//
//	feedback-dispose --subject-dir docs/feedback/<stem> --subject-path <doc> [--ref HEAD] [--worktree .] [--concluded]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback/adapters/gitrepo"
)

var processExit = os.Exit

func main() { processExit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feedback-dispose", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var subjectDir, subjectPath, ref, worktree string
	var concluded bool
	fs.StringVar(&subjectDir, "subject-dir", "", "the subject's ledger directory (docs/feedback/<stem>)")
	fs.StringVar(&subjectPath, "subject-path", "", "the reviewed document's repo-relative path (its existence is the resolvability test)")
	fs.StringVar(&ref, "ref", "HEAD", "the authoritative ref landing is measured against")
	fs.StringVar(&worktree, "worktree", ".", "the git worktree to read")
	fs.BoolVar(&concluded, "concluded", false, "assert the subject's review has concluded (enables no-action for no-landed-edit entries)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if subjectDir == "" || subjectPath == "" {
		fmt.Fprintln(stderr, "feedback-dispose: --subject-dir and --subject-path are required")
		return 2
	}

	entries, err := readSubject(subjectDir)
	if err != nil {
		fmt.Fprintf(stderr, "feedback-dispose: %v\n", err)
		return 1
	}
	res := feedback.Fold(filepath.Base(subjectDir), entries)
	if len(res.Defects) > 0 {
		for _, d := range res.Defects {
			fmt.Fprintf(stderr, "feedback-dispose: %s: %s\n", d.File, d.Reason)
		}
		return 1
	}

	repo := gitrepo.Repo{
		Worktree:       worktree,
		Ref:            ref,
		Subject:        filepath.Base(subjectDir),
		SubjectPath:    subjectPath,
		ConcludeNoEdit: concluded,
	}
	transitions, err := feedback.Observe(res.Entries, repo)
	if err != nil {
		fmt.Fprintf(stderr, "feedback-dispose: %v\n", err)
		return 1
	}
	if _, err := feedback.ApplyTransitions(subjectDir, transitions); err != nil {
		fmt.Fprintf(stderr, "feedback-dispose: %v\n", err)
		return 1
	}

	for _, t := range transitions {
		if t.Resolved() {
			fmt.Fprintf(stdout, "%s: %s (%s)\n", t.SourceFile, t.Disposition.Disposition, t.Disposition.ResolvedBy)
		} else {
			fmt.Fprintf(stdout, "%s: open — %s\n", t.SourceFile, t.Flag)
		}
	}
	return 0
}

// readSubject loads a subject directory's entry files as feedback.Entry values,
// skipping the generated projection (not an entry) and any non-entry file.
func readSubject(dir string) ([]feedback.Entry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read subject dir %s: %w", dir, err)
	}
	var entries []feedback.Entry
	for _, it := range items {
		if it.IsDir() || it.Name() == feedback.ProjectionFile {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, it.Name()))
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", it.Name(), err)
		}
		entries = append(entries, feedback.Entry{Name: it.Name(), Bytes: b})
	}
	return entries, nil
}
