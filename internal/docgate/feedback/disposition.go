// disposition.go is the T4 slice of the review-feedback ledger
// (docs/tech-design/review-feedback-capture.md §3.4): the disposition state
// machine, the deterministic resolving-event (commit-landing) observation, and
// the pending / no-action / stale-subject handling.
//
// The load-bearing rule is DINV-3 / I3: the writer sets a typed disposition
// ONLY from an observed resolving event, never by inference. Every appended
// transition names the event it was set from (Resolved-by); a transition with
// no event is rejected, so inference has no admissible path. Anything not yet
// observable stays OPEN — flagged pending (UB-5) — and a subject whose anchor no
// longer resolves is retained, flagged stale (UB-6); neither pending nor stale
// is a disposition, so the closed four-disposition set is never widened by a
// guess.
//
// The observer depends on one consumer-side port — Repo (§3.4, DINV-2/RW-3) —
// carrying the only repository facts a disposition needs. A git adapter
// implements it for production; a fake drives the tests hermetically. The
// decision itself (decide) is pure: given the observed facts it returns a
// transition with no filesystem, clock, or network of its own.
package feedback

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Flag is the open-state a still-un-disposed entry carries when the observer
// runs. A flag is never persisted to the entry file — it is recomputed each
// pass, so it can never go stale itself: an entry that becomes resolvable later
// simply gets its terminal disposition then. Exactly one of a transition's
// Disposition / Flag is set.
type Flag string

const (
	// FlagNone is a resolved entry — its transition carries a Disposition instead.
	FlagNone Flag = ""
	// FlagPending: the resolving event is not yet observable; the entry stays
	// open (UB-5). This is the default open state, surfaced explicitly.
	FlagPending Flag = "pending"
	// FlagStale: the entry's subject anchor no longer resolves — the subject was
	// superseded or deleted; the entry is retained and flagged (UB-6).
	FlagStale Flag = "stale"
)

// Landing is the observed result of asking whether an entry's recorded
// edit-commit has landed on the subject's authoritative history.
type Landing struct {
	// Landed reports whether the recorded edit-commit is on the subject's
	// authoritative history.
	Landed bool
	// Outcome is the STABLE outcome-ref naming the observed landing — a
	// merged/squash sha, a PR number, or an amendment id, never a working-branch
	// sha (DINV-4). Set only when Landed.
	Outcome string
	// EditSize is the landed edit's changed size, frozen as the entry's
	// edit_distance at applied-write (§3.6, AP-21 judge-once). Set only when Landed.
	EditSize int
}

// Repo is the consumer-side port the disposition observer needs to observe a
// resolving event deterministically (§3.4, DINV-2/RW-3). Its three questions are
// the only repository facts a disposition depends on; the observer core imports
// no implementation — a git adapter provides one for production, a fake for the
// tests. Every method may fail (a broken repo, an unreadable ref); the observer
// treats an error as "not observable" and leaves the entry pending rather than
// guess (DINV-3).
type Repo interface {
	// SubjectResolves reports whether an entry's subject anchor still resolves to
	// a live location. false → the subject was superseded or deleted (UB-6).
	SubjectResolves(anchor string) (bool, error)
	// EditLanded reports whether the recorded edit-commit has landed on the
	// subject's authoritative history, and the stable outcome-ref naming it. An
	// empty editCommit (the comment produced no edit) is never "landed".
	EditLanded(editCommit string) (Landing, error)
	// SubjectFinalized reports whether the subject's own change has landed (its
	// review concluded), so an entry with no landed edit is now a settled
	// no-action rather than still-pending. ref names the finalizing event stably.
	SubjectFinalized(subject string) (finalized bool, ref string, err error)
}

// Transition is the observer's per-entry verdict. Exactly one of Disposition
// (a terminal transition to append) and Flag (an open state to surface) is set.
type Transition struct {
	Entry       string       // the entry's dense sequence (NNN)
	SourceFile  string       // the entry's filename
	Disposition *Disposition // non-nil → append this terminal transition
	Flag        Flag         // pending | stale when Disposition is nil
}

// Resolved reports whether the transition closes the entry (a disposition) as
// opposed to leaving it open (a flag).
func (t Transition) Resolved() bool { return t.Disposition != nil }

// Observe runs one deterministic disposition pass over a subject's captured
// entries, asking the Repo the resolving-event questions and returning one
// Transition per still-open entry (an already-disposed entry is skipped — a
// disposition is terminal and append-once, DINV-1). It performs NO filesystem
// writes; ApplyTransitions persists the terminal ones.
func Observe(entries []*Captured, repo Repo) ([]Transition, error) {
	if repo == nil {
		return nil, fmt.Errorf("observe: nil repo")
	}
	var out []Transition
	for _, c := range entries {
		if c == nil || c.Disposition != nil {
			continue // already resolved — terminal, never re-observed
		}
		t, err := observeOne(c, repo)
		if err != nil {
			return nil, fmt.Errorf("observe entry %s: %w", c.SourceFile, err)
		}
		out = append(out, t)
	}
	return out, nil
}

// observeOne gathers the observed facts for one entry and hands them to the pure
// decision. A Repo error on any question is not a disposition trigger — the
// entry is left pending, never closed on an unknown (DINV-3, fail-open on the
// observation, fail-closed on the disposition).
func observeOne(c *Captured, repo Repo) (Transition, error) {
	resolves, err := repo.SubjectResolves(c.SubjectAnchor)
	if err != nil {
		return Transition{}, fmt.Errorf("subject-resolves: %w", err)
	}
	if !resolves {
		// UB-6: retained + flagged stale; no further questions — the subject is gone.
		return Transition{Entry: c.Entry, SourceFile: c.SourceFile, Flag: FlagStale}, nil
	}
	var land Landing
	if c.EditCommit != "" {
		if land, err = repo.EditLanded(c.EditCommit); err != nil {
			return Transition{}, fmt.Errorf("edit-landed: %w", err)
		}
	}
	finalized, finalRef, err := repo.SubjectFinalized(subjectOf(c.SubjectAnchor))
	if err != nil {
		return Transition{}, fmt.Errorf("subject-finalized: %w", err)
	}
	return decide(c, land, finalized, finalRef), nil
}

// decide is the pure disposition decision for one open entry given the observed
// facts. It NEVER guesses (DINV-3/I3): a disposition is returned only with the
// resolving event that produced it named in Resolved-by; anything not yet
// observable stays pending.
func decide(c *Captured, land Landing, finalized bool, finalRef string) Transition {
	t := Transition{Entry: c.Entry, SourceFile: c.SourceFile}
	switch {
	// Applied — the recorded edit-commit has landed on the subject. Close with the
	// STABLE outcome-ref naming the observed commit (DINV-4) and freeze
	// edit_distance from the observed edit size (§3.6, AP-21).
	case c.EditCommit != "" && land.Landed:
		ed := land.EditSize
		t.Disposition = &Disposition{
			Disposition:  "applied",
			Outcome:      land.Outcome,
			ResolvedBy:   "commit " + land.Outcome + " landed on the subject",
			EditDistance: &ed,
		}
	// No-action — the subject's own review concluded and no edit from this comment
	// landed: a settled no-change close, citing the finalizing event as its reason
	// (UB-3). Only reachable once the subject is finalized, so it is an observed
	// event, never an inference from silence.
	case finalized:
		t.Disposition = &Disposition{
			Disposition: "no-action",
			Outcome:     finalRef,
			ResolvedBy:  "subject finalized at " + finalRef + " with no landed edit from this comment",
		}
	// Otherwise the resolving event is not yet observable — stay open, pending (UB-5).
	default:
		t.Flag = FlagPending
	}
	return t
}

// subjectOf returns the subject stem of an anchor (`<subject-stem>#<section>`),
// or the whole string when it carries no `#`.
func subjectOf(anchor string) string {
	stem, _, _ := strings.Cut(anchor, "#")
	return strings.TrimSpace(stem)
}

// ApplyTransitions persists the terminal transitions of a pass under dir (the
// subject directory), appending each disposition to its entry file. Open
// (flagged) transitions are left untouched — pending and stale are never written
// to a file. It returns the entry paths it disposed, in order.
func ApplyTransitions(dir string, transitions []Transition) ([]string, error) {
	var disposed []string
	for _, t := range transitions {
		if !t.Resolved() {
			continue
		}
		path := filepath.Join(dir, t.SourceFile)
		if err := AppendDisposition(path, *t.Disposition); err != nil {
			return disposed, fmt.Errorf("apply %s: %w", t.SourceFile, err)
		}
		disposed = append(disposed, path)
	}
	return disposed, nil
}

// AppendDisposition appends a rendered `## Disposition` block to an entry file
// on disk, enforcing the two invariants a transition must hold:
//
//   - DINV-3 — the disposition names its resolving event (Resolved-by non-empty)
//     and is one of the closed four; a transition with neither is rejected, so no
//     code path sets a disposition without an observed event.
//   - DINV-1 — an entry is append-once for its disposition: a file that already
//     carries a `## Disposition` section is never rewritten. The capture block is
//     left byte-for-byte intact; the block is only appended.
func AppendDisposition(path string, d Disposition) error {
	block, err := RenderDisposition(d)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read entry %s: %w", path, err)
	}
	if hasDispositionSection(b) {
		return fmt.Errorf("entry %s already carries a disposition — a disposition is terminal and append-once (DINV-1)", path)
	}
	// Append after a single blank-line separator, preserving the entry's existing
	// bytes verbatim (the capture block is never rewritten, DINV-1).
	body := strings.TrimRight(string(b), "\n") + "\n\n" + string(block)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write entry %s: %w", path, err)
	}
	return nil
}

// RenderDisposition renders a `## Disposition` block from a typed Disposition,
// enforcing DINV-3 (a resolving event is mandatory) and the closed set. It is
// pure and round-trips through parseDisposition: same Disposition in, same
// fields out. Edit-distance is emitted only when present (frozen for an applied
// outcome, §3.6).
func RenderDisposition(d Disposition) ([]byte, error) {
	if !dispositions[d.Disposition] {
		return nil, fmt.Errorf("disposition %q is not one of the closed set (applied, declined-with-reason, became-incident, no-action)", d.Disposition)
	}
	if strings.TrimSpace(d.ResolvedBy) == "" {
		return nil, fmt.Errorf("disposition %q names no resolving event — every transition must cite the observed event it was set from (DINV-3)", d.Disposition)
	}
	if strings.TrimSpace(d.Outcome) == "" {
		return nil, fmt.Errorf("disposition %q has no outcome-ref — a transition names a stable outcome (DINV-4)", d.Disposition)
	}
	for _, f := range []struct{ name, val string }{
		{"outcome", d.Outcome},
		{"resolved-by", d.ResolvedBy},
	} {
		if strings.ContainsAny(f.val, "|\n") {
			return nil, fmt.Errorf("disposition %s %q contains a table-breaking character ('|' or newline)", f.name, f.val)
		}
	}

	var b strings.Builder
	b.WriteString("## Disposition\n\n")
	b.WriteString("| Field | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| **Disposition** | %s |\n", d.Disposition)
	fmt.Fprintf(&b, "| **Outcome** | %s |\n", d.Outcome)
	fmt.Fprintf(&b, "| **Resolved-by** | %s |\n", d.ResolvedBy)
	if d.EditDistance != nil {
		fmt.Fprintf(&b, "| **Edit-distance** | %d |\n", *d.EditDistance)
	}
	return []byte(b.String()), nil
}

// hasDispositionSection reports whether the entry bytes already carry a
// `## Disposition` heading — the append-once guard (DINV-1).
func hasDispositionSection(b []byte) bool {
	for _, l := range strings.Split(string(schema.Normalize(b)), "\n") {
		if title, ok := heading(l); ok && title == "Disposition" {
			return true
		}
	}
	return false
}
