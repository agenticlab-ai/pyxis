// Admission is the registry's first check (design init-hardening-v1 §3.1
// check 1; DINV-5; prd:init-hardening FR-1/FR-2; prd:installer/UB-11):
// content-based, admitting exactly an empty directory or a repository
// empty of tracked and untracked content — the repository metadata
// directory and local execution diagnostics are the only tolerated entries;
// OS droppings are content.
// Anything else refuses, routed by name: a content-holding repository
// names `pyxis adopt`; a non-git target with content names the runnable
// two-step (`git init -b <branch>`, then `pyxis adopt`) — adopt's own
// dispatch and design stay untouched (§0 row 3; brainstorm O-1). This is a
// read-only probe: it performs no mutation on any path, and — like every
// Check.Probe — returns a non-nil error only for an infra failure (it
// could not even complete the probe); a refusal is always a StatusRefuse
// Outcome with a nil error, never the reverse.
package preflight

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

// AdmissionID is the admission check's registry id.
const AdmissionID = "admission"

// adoptRoute is the one follow-up command both refusal classes name
// (DINV-5: "every admission refusal names the adoption route").
const adoptRoute = "pyxis adopt"

// defaultBranch mirrors internal/installer's own resolution — both read
// config.Defaults() (config-driven-portability-v1's one source, DINV-1/
// DINV-2) rather than each pinning a second literal; this is the small,
// deliberate git-probe duplication internal/adopt's own dispatch.go
// already documents the precedent for (the two packages' admission
// verdicts differ on purpose, so they do not share the unexported probe).
var defaultBranch = config.Defaults().Branch

var admissionCheck = Check{
	ID:            AdmissionID,
	Applicability: nil, // always applicable — admission runs before any knob exists
	Probe:         probeAdmission,
}

// probeAdmission is the check's read-only probe. No .git entry and no
// content → a bare directory the caller will `git init`. A .git entry
// present with zero tracked files and a clean worktree → an adoptable
// empty repo, kept on its current branch (or the default when HEAD is
// unborn). Any tracked content, any dirt, or — for a non-git target — any
// content beyond local execution diagnostics refuses naming the conflicts
// and the route.
func probeAdmission(ctx context.Context, in Input) (Outcome, error) {
	gitDir := filepath.Join(in.Target, ".git")
	_, statErr := os.Stat(gitDir)
	switch {
	case statErr == nil:
		return admitGitRepo(ctx, in.Target)
	case os.IsNotExist(statErr):
		return admitBareDir(in.Target)
	default:
		return Outcome{}, fmt.Errorf("preflight: probe %s: %w", gitDir, statErr)
	}
}

// admitGitRepo probes an existing repository's tree: empty of tracked and
// untracked content passes; anything else refuses naming `pyxis adopt`.
func admitGitRepo(ctx context.Context, target string) (Outcome, error) {
	tracked, err := gitLines(ctx, target, "ls-files")
	if err != nil {
		return Outcome{}, fmt.Errorf("preflight: list tracked files in %s: %w", target, err)
	}
	dirty, err := gitLines(ctx, target, "status", "--porcelain", "--untracked-files=all", "--", ".", ":(exclude)"+runlogs.Root)
	if err != nil {
		return Outcome{}, fmt.Errorf("preflight: read worktree status in %s: %w", target, err)
	}
	if len(tracked) > 0 || len(dirty) > 0 {
		return refuseContentRepo(conflictPaths(tracked, dirty)), nil
	}
	return Outcome{Status: StatusPass, GitInit: false, Branch: currentBranch(ctx, target)}, nil
}

// admitBareDir probes a target carrying no .git entry: empty apart from local
// diagnostics passes (the caller will git init); other content, including OS
// droppings, refuses naming the runnable two-step.
func admitBareDir(target string) (Outcome, error) {
	entries, err := os.ReadDir(target)
	if err != nil {
		return Outcome{}, fmt.Errorf("preflight: read %s: %w", target, err)
	}
	// An interrupted init may leave only its own diagnostics behind.
	if len(entries) == 1 && entries[0].Name() == ".pyxis" && entries[0].IsDir() {
		children, err := os.ReadDir(filepath.Join(target, ".pyxis"))
		if err == nil && len(children) == 1 && children[0].Name() == "logs" && children[0].IsDir() {
			entries = nil
		}
	}
	if len(entries) > 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
		sort.Strings(names)
		return refuseNonGitContent(names), nil
	}
	return Outcome{Status: StatusPass, GitInit: true, Branch: defaultBranch}, nil
}

// refuseContentRepo is the content-holding-repository refusal class: the
// target already has a repository, so the route is the plain adopt
// invocation (FR-2).
func refuseContentRepo(conflicts []string) Outcome {
	return Outcome{
		Status:    StatusRefuse,
		Reason:    "the target repository holds tracked or uncommitted content beyond its metadata directory — init only sets up empty repositories",
		Conflicts: conflicts,
		Remedy:    fmt.Sprintf("run `%s` to negotiate the existing content instead of an empty-target install", adoptRoute),
		Route:     adoptRoute,
	}
}

// refuseNonGitContent is the non-git-with-content refusal class: there is
// no repository yet to hand to adopt, so the remedy is the runnable
// two-step — create one on the default branch, then adopt (design §3.1's
// #D-2; adopt's own dispatch and design stay untouched).
func refuseNonGitContent(conflicts []string) Outcome {
	return Outcome{
		Status:    StatusRefuse,
		Reason:    "the target holds content and is not a git repository — init only sets up empty directories",
		Conflicts: conflicts,
		Remedy:    fmt.Sprintf("run `git init -b %s`, then `%s` to negotiate the existing content", defaultBranch, adoptRoute),
		Route:     adoptRoute,
	}
}

// conflictPaths merges the tracked set and the porcelain-reported set into
// one sorted, de-duplicated list for the refusal message. Porcelain lines
// are "XY <path>"; the path starts at column 3.
func conflictPaths(tracked, porcelain []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, p := range tracked {
		add(p)
	}
	for _, line := range porcelain {
		if len(line) > 3 {
			add(line[3:])
		} else {
			add(line)
		}
	}
	sort.Strings(out)
	return out
}

// currentBranch reads the target repo's current branch, or the default
// when HEAD is unborn or unreadable.
func currentBranch(ctx context.Context, target string) string {
	out, err := gitOutput(ctx, target, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return defaultBranch
	}
	if b := strings.TrimSpace(out); b != "" {
		return b
	}
	return defaultBranch
}

// gitOutput runs one git subcommand in target, honoring ctx, returning
// stdout. A non-zero exit wraps stderr so infra failures stay named.
func gitOutput(ctx context.Context, target string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", target}, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out.String(), nil
}

// gitLines runs one git subcommand and splits stdout into non-empty lines.
func gitLines(ctx context.Context, target string, args ...string) ([]string, error) {
	out, err := gitOutput(ctx, target, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, ln := range strings.Split(out, "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, ln)
		}
	}
	return lines, nil
}
