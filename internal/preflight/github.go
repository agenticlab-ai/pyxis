// GitHub-mode checks (design init-hardening-v1 §3.1 checks 3/4; plan
// init-hardening/001 T3): gh presence, an authenticated identity, the
// coordinate's resolution, and — for wiring repo=use — its repository's
// existence and readability; a follow-on best-effort scope-prediction
// check. Applicability for both derives from the declared wire operation
// table (never a second knob predicate, design §3.1's own words) — never
// straight from the tracker knob: wiring.repo and wiring.protection are
// tracker-independent per wire.Table's own conditions (a carina-tracker
// repo can still wire GitHub branch protection), so "GitHub mode" here
// means "the resolved knobs declare at least one gh-transport operation",
// not "tracker=github".
//
// The GitHub-mode check's own probes run in a fixed order — presence, auth,
// coordinate, existence — and the first defect refuses (later probes in
// THIS check are skipped); this is a single check's internal short-circuit,
// distinct from the registry's own report-everything contract (Run still
// evaluates every OTHER applicable check, scope-prediction and cross-knob
// coherence included, DINV-1/AP-16).
package preflight

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// GitHubModeID and ScopePredictionID are the registry ids for design §3.1
// checks 3 and 4.
const (
	GitHubModeID      = "github-mode"
	ScopePredictionID = "scope-prediction"
)

var githubModeCheck = Check{
	ID:            GitHubModeID,
	Applicability: githubModeApplicable,
	Probe:         probeGitHubMode,
}

var scopePredictionCheck = Check{
	ID:            ScopePredictionID,
	Applicability: githubModeApplicable,
	Probe:         probeScopePrediction,
}

// wiringFreeRemedy is the alternative every GitHub-mode refusal names
// alongside its own fix: a knob set that declares no gh-transport
// operation at all needs none of this (wire.GHAbsentError's own wording,
// repeated here since this check now fires BEFORE wire ever runs).
const wiringFreeRemedy = "or choose wiring-free knobs (tracker=carina, wiring.protection=none, wiring.repo=use)"

// GHOpsImplied reports whether the resolved-or-prospective knobs declare at
// least one gh-transport operation — the ONE applicability predicate
// (derived from the declared operation table, never a second knob reading)
// exported for every out-of-package consumer: the installer's conditional
// exchange and its remote-materialization stage consume THIS, never a copy
// (DINV-2; review:impl #809 FND-4).
func GHOpsImplied(k config.Knobs) bool { return githubModeApplicable(k) }

// githubModeApplicable derives BOTH GitHub-mode checks' applicability
// straight from the declared operation table (design §3.1 check 3: "derived
// from the declared operation table, never a second knob predicate") —
// true iff the resolved knobs declare at least one gh-transport op. A
// zero-value Knobs (the pre-elicitation call admitTarget makes) declares
// none, so both checks are correctly inert until knobs are actually
// resolved.
func githubModeApplicable(k config.Knobs) bool {
	return len(githubOps(k)) > 0
}

// githubOps returns the declared table's gh-transport subset — the set
// both checks reason about (wire.Table is the one source, AP-11).
func githubOps(k config.Knobs) []wire.Op {
	var out []wire.Op
	for _, op := range wire.Table(wire.Config{Repo: config.Config{Knobs: k}}) {
		if op.Transport == wire.TransportGH {
			out = append(out, op)
		}
	}
	return out
}

// runnerFor resolves the gh transport: the injected Input.WireRunner (tests,
// the e2e matrix) or the production wire.GHRunner{} — the one gh boundary
// (internal/wire.Runner) every gh-facing seam in this build shares.
func runnerFor(in Input) wire.Runner {
	if in.WireRunner != nil {
		return in.WireRunner
	}
	return wire.GHRunner{}
}

// probeGitHubMode is check 3: gh present, an authenticated identity, the
// coordinate resolvable, and — for wiring.repo=use — that its repository
// exists and is readable.
func probeGitHubMode(ctx context.Context, in Input) (Outcome, error) {
	runner := runnerFor(in)

	if out, ok := probeGHPresence(ctx, runner); !ok {
		return out, nil
	}
	if out, ok := probeGHAuth(ctx, runner); !ok {
		return out, nil
	}
	slug, out, ok := resolveAndValidateCoordinate(ctx, in)
	if !ok {
		return out, nil
	}
	if in.Knobs.Wiring.Repo == "use" {
		if out, ok := probeRepoExists(ctx, runner, slug); !ok {
			return out, nil
		}
	}
	// create: absence is the expected state (AP-22's ensure lands at
	// apply) — no existence probe.
	return Outcome{Status: StatusPass}, nil
}

// probeGHPresence runs `gh --version`: absent or unrunnable refuses naming
// the install+auth remedy — the same UB-7-class prerequisite wire.Execute's
// own preflight already names at apply time, moved here so it fires before
// any write rather than six stages in.
func probeGHPresence(ctx context.Context, runner wire.Runner) (Outcome, bool) {
	_, stderr, exit, err := runner.RunGH(ctx, "--version")
	switch {
	case err != nil:
		return refuseGH("the gh CLI is unavailable", err.Error()), false
	case exit != 0:
		return refuseGH("the gh CLI did not run", firstStderrLine(stderr)), false
	}
	return Outcome{}, true
}

// probeGHAuth runs `gh auth status` against github.com specifically (this
// build only ever wires GitHub): a non-zero exit refuses naming the login
// remedy.
func probeGHAuth(ctx context.Context, runner wire.Runner) (Outcome, bool) {
	_, stderr, exit, err := runner.RunGH(ctx, "auth", "status", "--hostname", "github.com")
	switch {
	case err != nil:
		return refuseGH("gh auth status did not run", err.Error()), false
	case exit != 0:
		return Outcome{
			Status: StatusRefuse,
			Reason: fmt.Sprintf("gh is not authenticated to github.com (%s)", firstStderrLine(stderr)),
			Remedy: "run `gh auth login` and re-run — " + wiringFreeRemedy,
		}, false
	}
	return Outcome{}, true
}

// refuseGH renders the shared gh-unavailable refusal shape (presence, auth,
// and existence probes' transport-level misses all name the same install
// remedy alongside the wiring-free escape hatch).
func refuseGH(reason, detail string) Outcome {
	return Outcome{
		Status: StatusRefuse,
		Reason: fmt.Sprintf("%s (%s) — GitHub-mode wiring needs it before any write", reason, detail),
		Remedy: "install gh (https://cli.github.com) and authenticate (`gh auth login`), then re-run — " + wiringFreeRemedy,
	}
}

// resolveAndValidateCoordinate resolves the owner/name coordinate (design
// §3.7): the configured remote when the target already carries one, else
// the promptless Input.Coordinate. Neither present is FR-7's named
// refusal; a present-but-malformed value (from either source) refuses
// rather than ever reaching gh argv (ABUSE-1's grammar mitigation, held
// here for the promptless door T5's interactive elicitation is not the
// only one into this check).
func resolveAndValidateCoordinate(ctx context.Context, in Input) (string, Outcome, bool) {
	slug := RemoteSlug(ctx, in.Target, "")
	source := "the configured git remote"
	if slug == "" {
		slug = strings.TrimSpace(in.Coordinate)
		source = "the promptless coordinate"
	}
	if slug == "" {
		return "", Outcome{
			Status: StatusRefuse,
			Reason: "GitHub mode needs a repository coordinate (owner/name) and none resolved: no configured git remote, and no promptless coordinate was supplied",
			Remedy: "supply it at the interactive prompt, pass --coordinate owner/name, add a `coordinate: owner/name` key to the answers file, or configure the git remote before running init",
		}, false
	}
	if !wire.ValidCoordinate(slug) {
		return "", Outcome{
			Status: StatusRefuse,
			Reason: fmt.Sprintf("%s %q is not a valid owner/name GitHub coordinate", source, slug),
			Remedy: "use a coordinate matching owner/name (letters, digits, - _ . only; no leading dash on either side)",
		}, false
	}
	return slug, Outcome{}, true
}

// RemoteSlug parses the target's configured remote into an owner/name
// coordinate, or "" when no repository exists AT the target, the remote is
// unset, or it does not name a github.com repository. remote "" resolves to
// the schema's one source (config.Defaults().Remote, DINV-6 — never a
// literal); post-config callers pass their captured remote name. This is
// the ONE remote→coordinate derivation every consumer shares — the checks,
// the installer's exchange, preview, and the wiring stage (DINV-2;
// review:impl #809 FND-4).
//
// Containment (review:impl #809 FND-1/CAL-10): only a repository rooted AT
// target speaks for target. `git -C target` would otherwise walk git's
// upward discovery, letting an ENCLOSING repository's remote shadow the
// caller's own resolution — a target nested anywhere inside another repo
// (a homedir repo, a monorepo checkout) would silently inherit that repo's
// coordinate. The .git directory must sit directly in target.
func RemoteSlug(ctx context.Context, target, remote string) string {
	if strings.TrimSpace(target) == "" {
		return "" // no target, no repository of its own (never the process cwd)
	}
	if remote == "" {
		remote = config.Defaults().Remote
	}
	gitDir, err := gitOutput(ctx, target, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "" // no repository reachable at all
	}
	if !samePath(filepath.Dir(strings.TrimSpace(gitDir)), target) {
		return "" // an enclosing repository's — not target's own
	}
	out, err := gitOutput(ctx, target, "remote", "get-url", remote)
	if err != nil {
		return ""
	}
	slug, ok := wire.ParseCoordinate(out)
	if !ok {
		return ""
	}
	return slug
}

// samePath reports whether two paths name the same directory once absolute
// and symlink-resolved — `git rev-parse --absolute-git-dir` returns the
// PHYSICAL path while callers hold the possibly-symlinked spelling (macOS
// temp dirs: /var/folders vs /private/var/folders), so a literal compare
// would reject a repository rooted exactly at target.
func samePath(a, b string) bool {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		return false
	}
	aa, err := filepath.Abs(ra)
	if err != nil {
		return false
	}
	ab, err := filepath.Abs(rb)
	if err != nil {
		return false
	}
	return aa == ab
}

// The existence probe's bounded transient retry (design §3.1 check 3, §3.10;
// risk R4): up to existenceProbeMaxAttempts invocations, exponential
// backoff from existenceProbeBaseDelay capped at existenceProbeMaxDelay —
// the same shape wire.Execute's own retries use, tuned locally since this
// check is not part of that package's walk.
const (
	existenceProbeMaxAttempts = 3
	existenceProbeBaseDelay   = 500 * time.Millisecond
	existenceProbeMaxDelay    = 4 * time.Second
)

// probeSleeper waits between the existence probe's retries, honoring
// cancellation. A package var so tests inject a non-sleeping fake — the
// same hermetic-clock discipline wire.Execute's own Sleeper follows.
type probeSleeper func(ctx context.Context, d time.Duration) error

var probeSleep probeSleeper = func(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// probeRepoExists is the per-mode existence probe (design §3.1 check 3):
// for wiring.repo=use, the coordinate's repository must exist and be
// readable via the API. A transient failure (wire.IsTransient's shared
// pattern vocabulary) retries bounded before refusing with the retry named
// (risk R4); any other non-zero exit refuses naming the missing-or-
// inaccessible remedy — GitHub itself answers a private repository's
// existence probe with the same 404 a genuinely missing repository gets,
// so the two cases collapse into one honest, non-leaking refusal.
func probeRepoExists(ctx context.Context, runner wire.Runner, slug string) (Outcome, bool) {
	var stderr string
	var exit int
	delay := existenceProbeBaseDelay
	for attempt := 1; attempt <= existenceProbeMaxAttempts; attempt++ {
		_, out, code, err := runner.RunGH(ctx, "repo", "view", slug, "--json", "name")
		if err != nil {
			return refuseGH("gh repo view did not run", err.Error()), false
		}
		stderr, exit = out, code
		if exit == 0 {
			return Outcome{}, true
		}
		if !wire.IsTransient(stderr) || attempt == existenceProbeMaxAttempts {
			break
		}
		if serr := probeSleep(ctx, delay); serr != nil {
			return refuseGH("the existence probe was interrupted", serr.Error()), false
		}
		delay = min(delay*2, existenceProbeMaxDelay)
	}
	if wire.IsTransient(stderr) {
		return Outcome{
			Status: StatusRefuse,
			Reason: fmt.Sprintf("could not confirm repository %s exists after %d attempts — the GitHub API kept failing transiently (%s)", slug, existenceProbeMaxAttempts, firstStderrLine(stderr)),
			Remedy: "check GitHub's status and your network, then re-run",
		}, false
	}
	return Outcome{
		Status: StatusRefuse,
		Reason: fmt.Sprintf("repository %s does not exist or is not accessible with the current gh credential (%s)", slug, firstStderrLine(stderr)),
		Remedy: "verify the coordinate and your access, or set wiring.repo=create to have init create it",
	}, false
}

// probeScopePrediction is check 4 (design §3.1; DINV-7): a best-effort,
// advisory-only scope prediction against the declared gh-transport
// operation set. Classic personal-access tokens expose their granted
// scopes on the X-Oauth-Scopes response header (GitHub's own convention);
// fine-grained tokens and GitHub App installation tokens do not — an
// unknowable answer emits nothing, never a refusal (open design question
// Q3 — the per-token-class signal set is intentionally the minimal
// knowable one at this iteration, not a complete taxonomy). Any failure to
// even ask (gh absent, not authenticated, a network blip) is likewise
// unknowable and silent: check 3 already names those defects on its own
// terms, so this check does not repeat them as a second, redundant voice.
func probeScopePrediction(ctx context.Context, in Input) (Outcome, error) {
	ops := githubOps(in.Knobs)
	if len(ops) == 0 {
		return Outcome{Status: StatusPass}, nil
	}
	runner := runnerFor(in)
	stdout, _, exit, err := runner.RunGH(ctx, "api", "-i", "user")
	if err != nil || exit != 0 {
		return Outcome{Status: StatusPass}, nil // unknowable — never refuse (DINV-7)
	}
	scopes, ok := parseOAuthScopes(stdout)
	if !ok || hasScope(scopes, "repo") {
		return Outcome{Status: StatusPass}, nil
	}
	names := make([]string, 0, len(ops))
	for _, op := range ops {
		names = append(names, op.Name)
	}
	return Outcome{
		Status: StatusAdvise,
		Warning: fmt.Sprintf(
			"the authenticated gh token's scopes (%s) do not include \"repo\" — %s may fail with a permission error; run `gh auth refresh -s repo` (or re-authenticate with a token carrying the repo scope)",
			strings.Join(scopes, ", "), strings.Join(names, ", ")),
	}, nil
}

// parseOAuthScopes extracts the X-Oauth-Scopes response header from `gh api
// -i`'s combined header+body stdout — present only for classic personal-
// access tokens (GitHub's own convention). ok is false for every other
// token class (fine-grained PAT, GitHub App installation token), which is
// the unknowable case DINV-7 requires this check to stay silent on.
func parseOAuthScopes(stdout string) (scopes []string, ok bool) {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		k, v, found := strings.Cut(line, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(k), "X-Oauth-Scopes") {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, true // header present but empty — a genuinely scope-less token
		}
		for _, s := range strings.Split(v, ",") {
			scopes = append(scopes, strings.TrimSpace(s))
		}
		return scopes, true
	}
	return nil, false
}

// hasScope reports whether scopes contains want, case-insensitively.
func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// firstStderrLine trims stderr to its first non-empty line for refusal
// reasons — the same readability discipline wire's own firstLine keeps for
// its audit reasons (unexported there; this is preflight's own copy since
// the two packages' refusal prose differ on purpose).
func firstStderrLine(s string) string {
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}
