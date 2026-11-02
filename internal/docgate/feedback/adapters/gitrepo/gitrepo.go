// Package gitrepo is the git adapter for the disposition observer's Repo port
// (docs/tech-design/review-feedback-capture.md §3.4): it answers the observer's
// resolving-event questions from a worktree's git state. It is a real adapter
// package, separate from the feedback core, which imports it — never the reverse
// (AP-1/AP-3/RW-3, DINV-2). The git specifics (ancestry, numstat, object
// existence) live here and die at the Repo boundary; the observer core knows
// only the medium-neutral Landing / bool facts.
//
// Every answer is a DETERMINISTIC read of git, never a guess (DINV-3/I3): an
// edit is "applied" only when its recorded commit is provably an ancestor of the
// authoritative ref; a subject is "stale" only when its file provably no longer
// exists there. Anything git cannot confirm is reported as not-observed, so the
// observer leaves the entry pending rather than close it on an unknown — the
// squash-merge orphaning of a working-branch sha (AP-13, incident I2) shows up
// here as "not an ancestor" → pending, exactly the fail-open the design wants.
package gitrepo

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
)

// Repo answers the disposition observer's questions for a single reviewed
// subject from a worktree's git state. It is constructed per subject: SubjectPath
// is the reviewed document's repo-relative path (its existence at Ref is the
// resolvability test), and ConcludeNoEdit asserts the subject's review has
// concluded — so an entry with no landed edit closes no-action rather than stay
// pending. ConcludeNoEdit is an explicit operator assertion (the pass is run in
// the subject's finalize context), never inferred, keeping no-action an observed
// event (DINV-3).
type Repo struct {
	Worktree       string // the git worktree to read
	Ref            string // the authoritative ref landing is measured against (e.g. HEAD, origin/main)
	Subject        string // the subject stem this adapter serves
	SubjectPath    string // the reviewed document's repo-relative path
	ConcludeNoEdit bool   // assert the subject's review has concluded (enables no-action)
}

var _ feedback.Repo = Repo{}

// SubjectResolves reports whether the subject document still exists at the
// authoritative ref. A deleted or renamed subject fails the object-existence
// check → the entry is retained and flagged stale (UB-6).
func (r Repo) SubjectResolves(string) (bool, error) {
	err := r.git("cat-file", "-e", r.Ref+":"+r.SubjectPath).Run()
	if err == nil {
		return true, nil
	}
	// `cat-file -e` exits non-zero when the object is absent — that is the
	// stale signal, not an adapter error. A genuine invocation failure (git
	// missing, bad worktree) surfaces as a non-ExitError.
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, fmt.Errorf("gitrepo: cat-file %s:%s: %w", r.Ref, r.SubjectPath, err)
}

// EditLanded reports whether the recorded edit-commit is an ancestor of the
// authoritative ref, and — when it is — the stable outcome-ref naming it (its
// resolved sha, which is on the authoritative history and so never a dangling
// working-branch ref, DINV-4) and the landed edit's changed-line count for
// edit_distance (§3.6). An empty edit-commit never lands.
func (r Repo) EditLanded(editCommit string) (feedback.Landing, error) {
	if strings.TrimSpace(editCommit) == "" {
		return feedback.Landing{}, nil
	}
	err := r.git("merge-base", "--is-ancestor", editCommit, r.Ref).Run()
	if err != nil {
		// Exit 1 = not an ancestor (not landed, or squash-orphaned) → pending, no
		// error. Any other exit (bad object, git failure) is a real error.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return feedback.Landing{}, nil
		}
		return feedback.Landing{}, fmt.Errorf("gitrepo: is-ancestor %s..%s: %w", editCommit, r.Ref, err)
	}
	sha, err := r.field("rev-parse", editCommit)
	if err != nil {
		return feedback.Landing{}, fmt.Errorf("gitrepo: rev-parse %s: %w", editCommit, err)
	}
	size, err := r.editSize(editCommit)
	if err != nil {
		return feedback.Landing{}, err
	}
	return feedback.Landing{Landed: true, Outcome: sha, EditSize: size}, nil
}

// SubjectFinalized reports whether the subject's review has concluded. It is the
// operator-asserted ConcludeNoEdit flag (never inferred), paired with the
// authoritative ref's resolved sha as the stable finalizing ref an entry cites.
func (r Repo) SubjectFinalized(string) (bool, string, error) {
	if !r.ConcludeNoEdit {
		return false, "", nil
	}
	sha, err := r.field("rev-parse", r.Ref)
	if err != nil {
		return false, "", fmt.Errorf("gitrepo: rev-parse %s: %w", r.Ref, err)
	}
	return true, sha, nil
}

// editSize sums the changed-line count of the edit-commit from `git show
// --numstat`: added + deleted over its non-binary files. It is the cheap,
// stable diff-size the design freezes as edit_distance (§3.6, Q1 lean).
func (r Repo) editSize(editCommit string) (int, error) {
	out, err := r.git("show", "--numstat", "--format=", editCommit).Output()
	if err != nil {
		return 0, fmt.Errorf("gitrepo: numstat %s: %w", editCommit, err)
	}
	return changedLineCount(out), nil
}

func changedLineCount(out []byte) int {
	total := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Fields(line)
		if len(cols) < 2 {
			continue
		}
		// A binary file's counts are "-"; skip it rather than error — a changed
		// line count over the textual files is the intended metric.
		add, aerr := strconv.Atoi(cols[0])
		del, derr := strconv.Atoi(cols[1])
		if aerr != nil || derr != nil {
			continue
		}
		total += add + del
	}
	return total
}

func (r Repo) git(args ...string) *exec.Cmd {
	return exec.Command("git", append([]string{"-C", r.Worktree}, args...)...)
}

func (r Repo) field(args ...string) (string, error) {
	out, err := r.git(args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
