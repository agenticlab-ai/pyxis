package installer

// The interview's FIRST question (#1043): where this project lives. One
// choice replaces two that used to sit four questions apart — the
// `wiring.repo` enum (use|create) and the conditional coordinate exchange —
// and adds the answer neither could express: no GitHub at all.
//
// Asking it first is what lets the rest of the interview adapt. "Keep it
// local" settles the tracker, the CI mode, and branch protection, so those
// three questions are not asked at all rather than asked and ignored; a
// fully offline install answers three questions instead of seven. Every
// offered choice is one that works: the list is built from repositories the
// operator's own credential can actually write to, so the interview can no
// longer accept an answer the run will later refuse.
//
// # The menu carries no default
//
// Every other question defaults to a POLICY value — github, both, local,
// none — which a closed stdin may safely take. This one's answer names an
// external destination, so a default would let `pyxis init < /dev/null`
// wire a real repository nobody chose. The codebase already ruled on this
// shape for the coordinate exchange (a blank answer is ABSENCE, not
// failure, and stops cleanly); the same ruling governs here.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// repoListCap bounds the offered list. A long menu is a worse question than
// a short one plus "another repository"; the create branch doubles as the
// way to name anything omitted.
const repoListCap = 15

// repoDecision is what the first question settled.
type repoDecision struct {
	// Local — no GitHub in this install at all. The caller derives
	// tracker/ci_mode/protection from it and skips those questions.
	Local bool
	// WiringRepo is the derived knobs.wiring.repo value ("use" | "create").
	WiringRepo string
	// Coordinate is the chosen or typed owner/name ("" when Local).
	Coordinate string
	// FromRemote marks a coordinate derived from a configured git remote
	// rather than chosen here — the caller announces it only once the
	// resolved knobs prove GitHub is actually in play.
	FromRemote bool
}

// The two standing options every rendering carries, whatever the list holds
// — including when there is no list at all.
const (
	createChoiceLabel = "create a new GitHub repository…"
	localChoiceLabel  = "keep it local — no GitHub at all"
)

// createCoordinateGloss frames the create branch's follow-up. The operator
// has just chosen to create one, so the coordinate exchange's own gloss
// ("No git remote is set — which GitHub repository should pyxis use (or
// create)?") would explain a question nobody asked.
const createCoordinateGloss = "Name the repository to create (owner/name). It becomes this directory's git remote."

// decideRepository asks the first question and returns what it settled, or
// nil when the question does not apply and the caller should fall back to
// the older per-knob questions:
//
//   - the run is promptless (flags/answers own every knob),
//   - the target is blank (adopt's promptless knob resolution),
//   - a promptless coordinate arrived,
//   - a configured git remote already names the repository — answered
//     without asking,
//   - the operator declined to answer (a closed stdin), which resolves to
//     no decision and lands on the same pre-write refusal as today.
//
// A nil decision is never an error: this question is a better surface over
// the same knobs, not a new prerequisite.
func decideRepository(ctx context.Context, opts *Options, pr *prompter, collected *collectedAnswers) (*repoDecision, error) {
	if pr == nil || strings.TrimSpace(opts.Target) == "" {
		return nil, nil
	}
	if slug := strings.TrimSpace(opts.Answers.Coordinate); slug != "" {
		return nil, nil // promptless coordinate: T3's own resolution owns it
	}
	if slug := preflight.RemoteSlug(ctx, opts.Target, ""); slug != "" {
		// A configured remote already names the repository, so there is
		// nothing to choose: it is `use`, at that coordinate.
		return &repoDecision{WiringRepo: "use", Coordinate: slug, FromRemote: true}, nil
	}

	runner := opts.WireRunner
	if runner == nil {
		runner = wire.GHRunner{}
	}
	listing, err := wire.ListEmptyRepositories(ctx, runner)
	if err != nil {
		// Named, never silent (AP-16): the question still gets asked, but
		// WHY it has no list belongs in the journal.
		if opts.Logger != nil {
			opts.Logger.LogAttrs(ctx, slog.LevelInfo, "repo_listing_unavailable",
				slog.String("reason", err.Error()))
		}
		// The menu still renders. An operator whose credential cannot reach
		// GitHub is exactly the one most likely to want "keep it local", so
		// a listing failure must not be what removes that option.
		listing = wire.RepoListing{}
	}

	shown, omitted := listing.Candidates, 0
	if len(shown) > repoListCap {
		shown, omitted = shown[:repoListCap], len(listing.Candidates)-repoListCap
	}
	pick, answered, err := askRepoChoice(pr, shown, omitted, listing.Truncated, collected)
	if err != nil {
		return nil, err
	}
	if !answered {
		// A closed stdin named nothing. Falling through leaves the older
		// path to resolve — and, with no coordinate anywhere, to refuse
		// pre-write exactly as it does today. Nothing is chosen on the
		// operator's behalf.
		return nil, nil
	}

	switch {
	case pick <= len(shown): // an existing repository
		slug := shown[pick-1].Slug
		collected.add("repository", slug)
		return &repoDecision{WiringRepo: "use", Coordinate: slug}, nil
	case pick == len(shown)+1: // create a new one
		slug, err := askCoordinateWithGloss(pr, createCoordinateGloss, collected)
		if err != nil {
			return nil, err
		}
		if slug == "" {
			// Named nothing. As above: no decision, and the older path's
			// pre-write refusal names the missing coordinate.
			return nil, nil
		}
		collected.add("repository", slug)
		// The typed coordinate is also the escape hatch for anything the
		// list omitted, so what it NAMES decides use-vs-create rather than
		// the menu entry the operator went through.
		switch wire.ProbeRepository(ctx, runner, slug) {
		case wire.RepoEmpty:
			return &repoDecision{WiringRepo: "use", Coordinate: slug}, nil
		case wire.RepoPopulated:
			// Naming the credential matters on a machine with more than one
			// login: WHICH account sees this repository as populated is often
			// the fact that explains the surprise.
			return nil, fmt.Errorf("init: %s already exists and is not empty (as %s sees it) — pyxis initializes an empty repository, so choose an empty one or a new name; nothing was written", slug, describeAccount(ctx, runner))
		default:
			// A name that does not resolve is what "create a new GitHub
			// repository" MEANS: the operator declared the intent by
			// choosing that entry and again by naming it. Asking to confirm
			// answers a question nobody asked — and worse, the confirm
			// defaulted to NO, so a bare Enter declined the very thing that
			// was just requested. GitHub's 404 is still ambiguous (absent,
			// or private to an account this credential cannot see), but that
			// ambiguity is resolved by ATTEMPTING the create, which fails by
			// name if the name is taken — not by putting the bet back to
			// someone who already placed it.
			return &repoDecision{WiringRepo: "create", Coordinate: slug}, nil
		}
	default: // keep it local
		collected.add("repository", "local (no GitHub)")
		return &repoDecision{Local: true, WiringRepo: "use"}, nil
	}
}

// describeAccount names the ambient gh login, or says plainly that it could
// not be learned rather than implying a specific one.
func describeAccount(ctx context.Context, runner wire.Runner) string {
	if who := wire.ActiveAccount(ctx, runner); who != "" {
		return "gh account " + who
	}
	return "the active gh credential"
}

// askRepoChoice renders the menu and reads a selection, honoring FR-13's
// bounded re-ask. The returned index is 1-based over: the shown candidates,
// then create, then local. answered is false when the operator named
// nothing (a closed stdin) — the caller treats that as no decision, never
// as a selection.
func askRepoChoice(pr *prompter, shown []wire.RepoCandidate, omitted int, truncated bool, collected *collectedAnswers) (pick int, answered bool, err error) {
	if pr.ctx.Err() != nil {
		return 0, false, ErrInterrupted
	}
	total := len(shown) + 2
	createIdx, localIdx := len(shown)+1, len(shown)+2

	terminalPrintf(pr.out, "\nWhere should this project live?\n")
	width := 0
	for _, c := range shown {
		if len(c.Slug) > width {
			width = len(c.Slug)
		}
	}
	for i, c := range shown {
		terminalPrintf(pr.out, "  %d) %-*s  empty · you have %s\n", i+1, width, c.Slug, strings.ToLower(c.Permission))
	}
	// The two shapes of "there may be more" are different facts and only one
	// of them is a count: a capped list omitted a KNOWN number, while a
	// truncated window means the query never saw far enough to say. Silence
	// on the second would tell an operator with a busy account that they own
	// no empty repositories, which is not something this can know.
	switch {
	case omitted > 0 && truncated:
		terminalPrintf(pr.out, "     (%d more not shown, and more may exist beyond the 100 most recently updated — choose %d to name one)\n", omitted, createIdx)
	case omitted > 0:
		terminalPrintf(pr.out, "     (%d more empty repositor%s not shown — choose %d to name one)\n",
			omitted, plural(omitted, "y", "ies"), createIdx)
	case truncated:
		terminalPrintf(pr.out, "     (only the 100 most recently updated were read — more may exist; choose %d to name one)\n", createIdx)
	}
	terminalPrintf(pr.out, "  %d) %s\n", createIdx, createChoiceLabel)
	terminalPrintf(pr.out, "  %d) %s\n", localIdx, localChoiceLabel)
	terminalPrintf(pr.out, "       work items live in %s, gates run at your\n", config.Defaults().WorkItems)
	terminalPrintf(pr.out, "       desk via git hooks; the initial tree is pushed to your local Git destination\n")

	var last string
	for attempt := 1; attempt <= maxAskAttempts; attempt++ {
		// No default: this answer names an external destination (see the
		// file header), so a blank one is re-asked and a closed stdin ends
		// the question rather than choosing.
		a, err := pr.ask("  choose", "")
		if err != nil {
			return 0, false, err
		}
		if a == "" && pr.atEOF {
			return 0, false, nil
		}
		if n, ok := parseChoice(a, total); ok {
			return n, true, nil
		}
		last = a
		if attempt < maxAskAttempts {
			if a == "" {
				terminalPrintf(pr.out, "  choose 1-%d — this one has no default, because it names where your work goes (%d attempt(s) left)\n", total, maxAskAttempts-attempt)
				continue
			}
			terminalPrintf(pr.out, "  %q is not one of 1-%d — try again (%d attempt(s) left)\n", a, total, maxAskAttempts-attempt)
		}
	}
	return 0, false, &knobAskAbortError{
		key:       "repository",
		lastInput: last,
		allowed:   fmt.Sprintf("1-%d", total),
		collected: collected.summary(),
	}
}

// parseChoice reads a 1-based menu selection, rejecting anything outside the
// offered range rather than clamping it.
func parseChoice(s string, total int) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 3 {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	if n < 1 || n > total {
		return 0, false
	}
	return n, true
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
