package attribution

import (
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Evaluate is the git-backed gate over a PR: it derives each of the PR's
// changed diffs' authorship from in-tree commit trailers read via `git` — no
// ambient credential, no mutable badge (§3.1, DINV-3) — resolves each Argus
// drive's declared scope from an author-independent committed source, and
// returns the refusal set (schema.Defect records). The PR is the range
// base..HEAD, where base is the already-resolved merge-base (fork-point) commit
// the caller passes (the docgate gate resolves --merge-base to the fork point
// once, at the CLI boundary, and hands every base-requiring check that commit).
//
// It reads only the repo: the diff, the commit trailers, and the committed
// delivery ledgers and eligible Carina feature items at the merge base.
// Any git failure that prevents
// evaluation surfaces as a fail-closed defect (the gate reds), never a silent
// pass (AP-17). base is guaranteed non-empty by the NeedsMergeBase descriptor;
// an empty base is itself a fail-closed defect. cfg supplies the target-facing
// doc roots (the problems root the delivery ledgers live under, and the roots
// the scope grammar spans) the classification resolves from
// (config-driven-portability-v1 §3.2, FR-1).
func Evaluate(root, base string, cfg config.Config) []schema.Defect {
	if strings.TrimSpace(base) == "" {
		return []schema.Defect{gitDefect("HEAD", "no merge base supplied — the attribution gate needs the PR's fork point",
			"run via docgate gate --merge-base origin/main (a base-requiring check)")}
	}

	signals, err := commitSignals(root, base)
	if err != nil {
		return []schema.Defect{gitDefect(base, fmt.Sprintf("reading commit trailers in %s..HEAD failed: %v", base, err),
			"pass a resolvable --merge-base ref fetched with full history (fetch-depth: 0)")}
	}

	files, err := changedFiles(root, base)
	if err != nil {
		return []schema.Defect{gitDefect(base, fmt.Sprintf("listing changed files vs %s failed: %v", base, err),
			"pass a resolvable --merge-base ref fetched with full history (fetch-depth: 0)")}
	}

	// The positive human-authorship attestation, read once over the PR range —
	// the establishment half of the three-state resolution (§3.3, DINV-5). A git
	// failure reading it fails closed (never a silent pass, AP-17).
	attested, err := positivelyHumanAttested(root, base)
	if err != nil {
		return []schema.Defect{gitDefect(base, fmt.Sprintf("reading the human-authorship attestation in %s..HEAD failed: %v", base, err),
			"pass a resolvable --merge-base ref fetched with full history (fetch-depth: 0)")}
	}

	resolve := gitScopeResolver(root, base, cfg, files...)
	changes := make([]Change, 0, len(files))
	var defects []schema.Defect
	for _, f := range files {
		author, slug, err := fileAuthorship(root, base, f, signals)
		if err != nil {
			// Fail closed on an unattributable file, never drop it silently.
			defects = append(defects, gitDefect(f, fmt.Sprintf("attributing %s failed: %v", f, err),
				"ensure the PR's commit graph is fully fetched (fetch-depth: 0)"))
			continue
		}
		changes = append(changes, Change{Path: f, Author: resolveState(author, attested), Slug: slug})
	}
	return append(defects, Refusals(cfg, changes, resolve)...)
}

// resolveState folds a file's trailer-derived authorship and the PR's positive
// human attestation into the FR-13 three-state resolution, evaluated under the
// positively-Argus-wins precedence (§3.3, DINV-5). It is the derivation-time
// bridge from the trailer read to the states the pure core (Refusals) acts on;
// the core's contract is unchanged (Argus → scope-classified, Human → passed,
// Underivable → fail-closed on the fixed protected classes):
//
//   - Argus — a produced, content-bound `Argus-Drive` stamp on the file (§3.2).
//     Positively-Argus WINS: it stays Argus regardless of any attestation, so a
//     drive-plantable `Governing-Change: operator` line can never launder a
//     positively-Argus protected-path diff (the both-signals collision refuses).
//   - Underivable — a malformed, unverifiable, or REPLAYED trailer, or an
//     unresolvable commit (§3.2). It stays underivable regardless of any
//     attestation: a replayed Argus trailer is not laundered by a planted
//     human attestation either — it fails closed on the protected subset.
//   - Human (no Argus trailer at all on the file's introducing commits) — the
//     absence case. Under this hardening, absence is NO LONGER human by itself
//     (the masquerade the frozen `Human = Argus-trailer-absent` inference left
//     open): it is positively-human (passed) ONLY when the PR carries the
//     positive attestation, else it is underivable (fail closed on the protected
//     subset, DINV-2). This makes DINV-2 and the open-human-path (NR-4)
//     co-satisfiable — a real human PR to a protected path carries the ordinary
//     attestation and passes; a stripped-Argus masquerade carries none and fails
//     closed.
func resolveState(author Author, attested bool) Author {
	if author == Human && !attested {
		return Underivable
	}
	return author
}

// gitDefect is a fail-closed defect for a git/environment failure that prevents
// evaluation — the gate reds rather than passing on an unreadable input.
func gitDefect(file, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: Verb, File: file, Op: "git", Reason: reason, Remedy: remedy}
}

// commitSignal is one commit's derived authorship: the trailer-presence verdict
// (Argus / Human / Underivable) and, for Argus, the driving problem slug.
type commitSignal struct {
	author Author
	slug   string
}

// commitSignals reads every commit in base..HEAD and derives each one's
// authorship signal from its trailer, keyed by full SHA. A commit with a
// well-formed, content-bound Argus trailer is Argus (naming the slug); one with
// no Argus trailer at all is Human; one whose Argus trailer is present but
// yields no derivable slug, carries no binding, or carries a binding that does
// not re-derive from the commit's own tree+parents (a REPLAYED or tampered
// trailer, §3.2) is Underivable.
//
// The binding is re-derived here, in-tree and credential-free: the commit's own
// tree (`%T`) and parents (`%P`), read in the same `git log`, are fed to
// ComputeBinding and compared to the trailer's `Argus-Bind` value. A trailer
// lifted onto a different commit binds a different tree/parents, so the compare
// fails and the commit is Underivable — never Argus (DINV-4).
func commitSignals(root, base string) (map[string]commitSignal, error) {
	// Unit separator \x1f between fields; record separator \x1e between commits —
	// neither occurs in a commit message or an object id, so the parse is
	// unambiguous regardless of message content (a body cannot forge a boundary).
	out, err := gitOutput(root, "log", "--format=%H%x1f%T%x1f%P%x1f%B%x1e", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	sigs := map[string]commitSignal{}
	for rec := range strings.SplitSeq(string(out), "\x1e") {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		fields := strings.SplitN(rec, "\x1f", 4)
		if len(fields) < 4 {
			continue
		}
		sha := strings.TrimSpace(fields[0])
		tree := strings.TrimSpace(fields[1])
		parents := strings.Fields(fields[2])
		info := parseTrailer(fields[3])
		switch {
		case !info.drivePresent:
			sigs[sha] = commitSignal{author: Human}
		case info.slugValid && info.bindPresent && info.bind == ComputeBinding(tree, parents):
			sigs[sha] = commitSignal{author: Argus, slug: info.slug}
		default:
			// A present Argus trailer that is malformed, unbound, or whose binding
			// does not re-derive from this commit (a replay) is unverifiable.
			sigs[sha] = commitSignal{author: Underivable}
		}
	}
	return sigs, nil
}

// fileAuthorship attributes a changed file to the commit(s) in base..HEAD that
// introduced it and folds their signals into one verdict. Precedence is
// Argus > Underivable > Human: an Argus change anywhere in the file's history
// makes it Argus (its scope is judged); else any unverifiable/unresolvable
// commit makes it Underivable (fail closed on the protected subset); else it is
// Human. A file that the diff reports changed but that no commit in the range is
// found to have introduced — an unresolvable commit — is Underivable, never a
// silent pass (§3.3).
func fileAuthorship(root, base, file string, signals map[string]commitSignal) (Author, string, error) {
	out, err := gitOutput(root, "log", "--format=%H", base+"..HEAD", "--", file)
	if err != nil {
		return Underivable, "", err
	}
	shas := splitLines(string(out))
	if len(shas) == 0 {
		// Changed in the diff yet attributable to no range commit — underivable.
		return Underivable, "", nil
	}
	author := Human
	slug := ""
	for _, sha := range shas {
		sig, ok := signals[sha]
		if !ok {
			// A commit that is not in the range's signal map is unreadable here —
			// treat as underivable (never silently human).
			if author != Argus {
				author = Underivable
			}
			continue
		}
		switch sig.author {
		case Argus:
			return Argus, sig.slug, nil
		case Underivable:
			author = Underivable
		}
	}
	return author, slug, nil
}

// gitScopeResolver returns a ScopeResolver that derives a slug's declared scope
// from an author-independent source read at the merge base: the named problem's
// committed (merged) delivery ledger when one exists there — its recorded
// reality, which a current drive cannot retroactively widen (§3.2.1, DINV-6) —
// or, when the problem has no committed ledger at the base (its first PR), the
// slug-derived fixed grammar (§3.2.1(b)). A feature item from the merge-base
// tree can extend either result by exactly one owned path; no same-PR item
// claim does so. Results are cached per slug. The
// problems root the ledger lives under, and the roots the first-PR grammar
// spans, resolve from cfg (config-driven-portability-v1 §3.2, FR-1).
func gitScopeResolver(root, base string, cfg config.Config, changed ...string) ScopeResolver {
	problems := strings.TrimPrefix(path.Clean(cfg.Problems), "./")
	cache := map[string]*boundary.Scope{}
	return func(slug string) *boundary.Scope {
		if slug == "" {
			return nil
		}
		if sc, ok := cache[slug]; ok {
			return sc
		}
		var sc *boundary.Scope
		ledgerPath := problems + "/" + slug + "/delivery.md"
		if existsAtBase(root, base, ledgerPath) {
			// A committed ledger bounds the width from its recorded reality. If it
			// is unreadable or unparseable, scope is unresolvable (nil) — the
			// consumer fails closed rather than widening (AP-17).
			if b, err := gitShowBytes(root, base, ledgerPath); err == nil {
				if resolved, ok := boundary.ResolveCommittedLedger(cfg, slug, b); ok {
					sc = resolved
				}
			}
		} else {
			// No committed ledger at the base: the problem's first PR — the
			// slug-derived fixed grammar (never same-PR self-attribution).
			sc = boundary.ResolveFirstPR(cfg, slug)
		}
		for _, target := range changed {
			if boundary.IsFeatureItemPath(cfg, target) && verifiedCarinaItemOwner(root, base, cfg, target, slug) {
				sc = boundary.WithVerifiedFeatureItem(cfg, sc, target)
			}
		}
		cache[slug] = sc
		return sc
	}
}

// changedFiles lists the PR's changed paths — the files differing between the
// merge base and HEAD — repo-relative with forward slashes. Renames are split
// into their delete+add paths (--no-renames) so each side is classified on its
// own; NUL-delimited so a path with unusual bytes cannot break the parse.
func changedFiles(root, base string) ([]string, error) {
	out, err := gitOutput(root, "diff", "-z", "--name-only", "--no-renames", base, "HEAD")
	if err != nil {
		return nil, err
	}
	var files []string
	for p := range strings.SplitSeq(strings.TrimRight(string(out), "\x00"), "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	return files, nil
}

// trailerLineRe matches a git trailer line `Token: value` (RFC-822-ish key).
var trailerLineRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*):[ \t]*(.*)$`)

// slugRe is the valid problem-slug shape a well-formed Argus trailer must carry.
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// trailerInfo is a commit's parsed Argus authorship trailers: whether the
// `Argus-Drive` trailer is present and its slug's validity, plus whether the
// `Argus-Bind` content-binding trailer is present and its value.
type trailerInfo struct {
	drivePresent bool   // an Argus-Drive trailer is in the final block
	slug         string // the Argus-Drive value
	slugValid    bool   // the slug is well-formed
	bindPresent  bool   // an Argus-Bind trailer is in the final block
	bind         string // the Argus-Bind value (the carried content binding)
}

// RebaseSlug reads a replayed commit's existing stamp so prepare-lock can
// refresh its binding after Git changes the parent. An unstamped commit stays
// unstamped; a partial or malformed stamp aborts the rebase. The old digest is
// intentionally not checked against the new parent: it is replaced by the
// commit hook before the rebased commit is written.
func RebaseSlug(message string) (string, bool, error) {
	info := parseTrailer(message)
	if !info.drivePresent && !info.bindPresent {
		return "", false, nil
	}
	if !info.drivePresent || !info.slugValid || !info.bindPresent || !bindRe.MatchString(info.bind) {
		return "", false, fmt.Errorf("rebase: incomplete or malformed Argus stamp")
	}
	return info.slug, true, nil
}

// VerifyRebaseSource checks a stamp against the source commit before its
// parent changes. The rebase hook may refresh only a stamp that was already
// valid; otherwise it would turn a stale or replayed signal into valid audit
// evidence. Unstamped source commits remain eligible for an unstamped replay.
func VerifyRebaseSource(message, tree string, parents []string) error {
	info := parseTrailer(message)
	if !info.drivePresent && !info.bindPresent {
		return nil
	}
	if _, _, err := RebaseSlug(message); err != nil {
		return err
	}
	if info.bind != ComputeBinding(tree, parents) {
		return fmt.Errorf("rebase: stale or replayed Argus binding")
	}
	return nil
}

var bindRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// parseTrailer reads a commit message's final trailer block and reports the
// Argus authorship trailers it carries — the `Argus-Drive: <slug>` stamp and
// its `Argus-Bind: <hex>` content binding. Only the last paragraph is scanned
// (git's trailer convention), so an "Argus-Drive:" mention in prose earlier in
// the body is not a trailer. A present-but-empty/malformed slug, or a present
// Argus-Drive with no Argus-Bind, is what the caller maps to Underivable —
// distinct from an absent Argus-Drive (a human commit), which is not-present.
func parseTrailer(message string) trailerInfo {
	msg := strings.ReplaceAll(message, "\r\n", "\n")
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	// Walk back to the start of the final non-empty block (the trailer block).
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	start := end
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	var t trailerInfo
	for _, line := range lines[start:end] {
		m := trailerLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		switch m[1] {
		case TrailerKey:
			t.drivePresent = true
			t.slug = strings.TrimSpace(m[2])
			t.slugValid = slugRe.MatchString(t.slug)
		case BindTrailerKey:
			t.bindPresent = true
			t.bind = strings.TrimSpace(m[2])
		}
	}
	return t
}

// existsAtBase reports whether path is tracked in the merge-base tree.
func existsAtBase(root, base, path string) bool {
	err := exec.Command("git", "-C", root, "cat-file", "-e", base+":"+path).Run()
	return err == nil
}

// gitShowBytes reads a tracked file's content at a commit (git show <rev>:path).
func gitShowBytes(root, rev, path string) ([]byte, error) {
	return gitOutput(root, "show", rev+":"+path)
}

// gitOutput runs a git command in root and returns its stdout.
func gitOutput(root string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
}

// splitLines splits git output into non-empty, whitespace-trimmed lines.
func splitLines(s string) []string {
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
