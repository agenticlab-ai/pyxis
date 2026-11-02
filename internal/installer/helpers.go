// The init engine's leaf helpers (installer-v1 §3.5 steps 1–3 + 8; plan
// installer/001 T7): the pieces Init orchestrates — the front-loaded checks
// (UB-11/FR-3/FR-5; internal/preflight's admission, toolchain, and
// artifact-source checks, plan init-hardening/001 T1/T2), the interactive
// prompter, knob resolution with the recommended defaults (FR-3/FR-4), the
// UB-10 identity resolution, the git primitives (init, the repo-local
// identity write, the explicit-path commit), the atomic file write, and the
// v2 config renderer whose bytes config.Parse round-trips (parseRendered is
// the guard). Every git invocation honors context; git is a hard
// integration point, so these shell out to it directly — the only exec
// surface in the package.
package installer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/converge"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// The operand-owned paths init writes directly (repo-owned from birth; never
// converge's — DINV-3): the config and the operand .gitignore.
const (
	configRel    = ".pyxis/config.yaml"
	gitignoreRel = ".gitignore"
)

// defaultBranch is the branch init creates a fresh repository on and captures
// into the config it writes (the shape's WR-9 note: v1 honors exactly this
// branch). It resolves from the repo config schema's single source
// (config.Defaults — "main" by default, the ONE place that literal lives,
// DINV-1) rather than a second literal here, so it stays byte-identical to the
// pre-config installer while the default lives in exactly one place
// (config-driven-portability-v1 §3.2, DINV-1/DINV-2).
var defaultBranch = config.Defaults().Branch

// operandGitignore is the minimal operand .gitignore init writes (installer-v1
// §3.5): the gitignored runtime directories the class policy's header
// enumerates (.pyxis/class-policy.yaml), plus optional operational logs —
// the caches and logs the payload never packs,
// so a fresh operand would otherwise commit them the first time anything
// fetched. Repo-owned from birth like the config; not manifest-listed; never
// touched by converge.
const operandGitignore = `# pyxis operand — gitignored runtime directories (installer-v1 §3.5).
# The binary cache, the local queues, and the preview scratch are
# regenerable and never committed.
.pyxis/bin/
/.pyxis/logs/
/.pyxis/maintenance/logs.jsonl
.pyxis/feedback-queue/
.pyxis/contract-gate-reports/
.pyxis/scratch/
# Session logs are optional local history, never a required commit artifact.
/logs/*
!/logs/README.md
`

// admission is Init's own narrow view of the preflight registry's admission
// outcome (UB-11): whether no repository exists yet (init must git init),
// and the branch the init commits land on. The full outcome — refusal
// reason, conflicts, remedy, route — is internal/preflight's; this shorthand
// carries only the two facts the pipeline itself branches on.
type admission struct {
	gitInit bool   // true when no repository exists — init runs git init
	branch  string // the branch captured into the config and committed onto

	// checks is every check the registry evaluated, in evaluation order
	// (admission, toolchain, artifact-source — plan init-hardening/001 T2;
	// T3 grows this set) — carried only for emitInitChecksPassed's EVT-1
	// "checks" property; the pipeline itself branches on nothing else here.
	checks []string
}

// admitTarget is step 1's read-only pass: the preflight registry's
// front-loaded checks (UB-11/FR-3/FR-5; internal/preflight, plan
// init-hardening/001 T1/T2) — every check whose applicability does not
// depend on a not-yet-resolved knob, so it runs before the first question
// and before any write (design §3.1's ordering). Content-based admission
// proceeds on an empty directory or a repository empty of tracked and
// untracked content and refuses everything else, routed by name; the
// toolchain check verifies git presence and floor version; the
// artifact-source check probes reachability on a cold cache. Any refusal
// carries that check's own remedy (and admission's route, where one
// exists). This performs no mutation.
func admitTarget(ctx context.Context, opts Options) (admission, error) {
	report, err := preflight.Run(ctx, preflight.Input{
		Target:       opts.Target,
		Integrations: opts.Integrations,
		System: config.System{
			Version:        opts.Pin.Version,
			ManifestSHA256: opts.Pin.ManifestSHA256,
			ArtifactSource: opts.ArtifactSource,
			Mode:           "release", // matches renderConfig's own literal — an init run always installs
		},
	})
	if err != nil {
		return admission{}, fmt.Errorf("init: %w", err)
	}
	adm := admission{checks: make([]string, 0, len(report))}
	var sawAdmission bool
	for _, out := range report {
		adm.checks = append(adm.checks, out.Check)
		if out.Check == preflight.AdmissionID {
			adm.gitInit, adm.branch = out.GitInit, out.Branch
			sawAdmission = true
		}
	}
	if !sawAdmission {
		return admission{}, fmt.Errorf("init: the preflight registry reported no admission outcome (programmer error)")
	}
	return adm, nil
}

// runKnobChecks is step 3.5's read-only probe (design init-hardening-v1
// §3.1 checks 3/4/6; plan init-hardening/001 T3): the preflight registry's
// knob-dependent checks — GitHub-mode auth/coordinate/existence, the
// best-effort scope advisories, and cross-knob coherence — evaluated once
// the five knobs are resolved but before any write. Mirrors admitTarget's
// shape: a narrow wrapper over preflight.Run, so Init stays a caller, never
// a re-implementer, of the registry (DINV-2).
func runKnobChecks(ctx context.Context, opts Options, knobs config.Knobs) (preflight.Report, error) {
	report, err := preflight.Run(ctx, preflight.Input{
		Target:               opts.Target,
		Knobs:                knobs,
		Coordinate:           opts.Answers.Coordinate,
		WireRunner:           opts.WireRunner,
		Integrations:         opts.Integrations,
		ForgejoClientFactory: opts.ForgejoClientFactory,
	})
	if err != nil {
		return report, fmt.Errorf("init: %w", err)
	}
	return report, nil
}

// headState reads target's HEAD before any mutation (plan init-hardening/001
// T4): the commit sha it resolves to, or born=false when HEAD is unborn (no
// commit exists yet). Called only for a target init did not create (an
// adopted empty repository), it is the pre-run snapshot unwind's
// restoreGitRef restores on a later stage failure, rather than deleting a
// repository init never owned.
func headState(ctx context.Context, target string) (sha string, born bool) {
	out, err := gitOutput(ctx, target, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(out), true
}

// prompter is the interactive elicitation surface (FR-3): one line read per
// question over stdin, the prompt echoed to stdout, an empty answer meaning
// "take the default". It exists only in a pure-interactive run — Init builds
// none when any flag/answers input arrived or --yes is set (FR-4).
type prompter struct {
	out io.Writer
	ctx context.Context
	// lines carries one stdin line per ask, fed by the single reader
	// goroutine newPrompter owns. Buffered by one so the reader's terminal
	// send (at EOF or a read error) always completes and the goroutine
	// exits, rather than parking forever on a channel nobody will read.
	lines chan lineRead
	// readErr latches a real stdin read failure: the reader reports it
	// once, so without this every later ask would silently take its
	// default instead of failing.
	readErr error
	// atEOF records that the last read found no more input. A question
	// whose answer names an EXTERNAL destination has to tell a bare Enter
	// (re-ask) from a closed stdin (stop), which the answer text alone
	// cannot: both arrive as "".
	atEOF bool
}

// lineRead is one result from the stdin reader: a line, or the terminal
// condition that ended the stream.
type lineRead struct {
	text string
	ok   bool  // a line was read; false means EOF or a read error
	err  error // a real read error (nil at clean EOF)
}

// ErrInterrupted is the named refusal an interrupted INTERVIEW returns —
// the pre-write phase, where checks-then-go guarantees the target is still
// untouched, so a caller may say so without guessing. It wraps
// context.Canceled, so the broader "this run was cancelled" class (a
// subprocess killed mid-flight, for instance) still matches
// errors.Is(err, context.Canceled) without matching this (#954).
var ErrInterrupted = fmt.Errorf("init: interrupted: %w", context.Canceled)

// newPrompter builds a prompter over stdin/stdout. The scanner's default
// token is a line, which is exactly one answer.
// newPrompter builds a prompter over stdin/stdout, bound to ctx so an
// interrupt aborts an in-flight question instead of being noticed pages
// later (#954). It owns exactly one reader goroutine: started here, it ends
// at stdin's EOF or read error (the one-slot buffer guarantees its final
// send lands) and on cancellation. The single case it cannot end in is
// blocked mid-read when ctx is cancelled — no portable cancellable read of
// a terminal exists — and there the process is already exiting, because an
// interrupted run returns ErrInterrupted from the next ask.
func newPrompter(ctx context.Context, stdin io.Reader, stdout io.Writer) *prompter {
	if stdout == nil {
		stdout = io.Discard
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if stdin == nil {
		// Behave as a closed stdin (every question takes its default)
		// rather than panicking inside the reader goroutine, where the
		// panic would be unrecoverable by the caller.
		stdin = strings.NewReader("")
	}
	p := &prompter{out: stdout, ctx: ctx, lines: make(chan lineRead, 1)}
	sc := bufio.NewScanner(stdin)
	go func() {
		defer close(p.lines)
		for sc.Scan() {
			select {
			case p.lines <- lineRead{text: sc.Text(), ok: true}:
			case <-ctx.Done():
				return
			}
		}
		select {
		case p.lines <- lineRead{err: sc.Err()}:
		case <-ctx.Done():
		}
	}()
	return p
}

// ask writes "question [default]: " and returns the trimmed line the user
// typed, or def when the answer is empty (or stdin reached EOF — a closed
// stdin defaults every question rather than erroring, so a mis-detected
// interactive run still terminates deterministically).
func (p *prompter) ask(question, def string) (string, error) {
	// An already-interrupted run asks nothing further (#954): before this
	// check a Ctrl-C only cancelled the context, and the interview carried
	// on to the last question against a dead run.
	if p.ctx.Err() != nil {
		return "", ErrInterrupted
	}
	if p.readErr != nil {
		return "", fmt.Errorf("read answer for %q: %w", question, p.readErr)
	}
	if def != "" {
		terminalPrintf(p.out, "%s [%s]: ", question, def)
	} else {
		terminalPrintf(p.out, "%s: ", question)
	}
	select {
	case <-p.ctx.Done():
		terminalPrintf(p.out, "\n") // close the half-written prompt line
		return "", ErrInterrupted
	case r := <-p.lines:
		// The reader closes lines on cancellation too, so a closed-channel
		// read here is an interrupt, not EOF — check the context first.
		if p.ctx.Err() != nil {
			terminalPrintf(p.out, "\n")
			return "", ErrInterrupted
		}
		if r.err != nil {
			p.readErr = r.err // latched: the reader reports it only once
			return "", fmt.Errorf("read answer for %q: %w", question, r.err)
		}
		if !r.ok {
			p.atEOF = true
			return def, nil // EOF — take the default
		}
		ans := strings.TrimSpace(r.text)
		if ans == "" {
			return def, nil
		}
		return ans, nil
	}
}

// question is one interactive knob prompt's FR-12 surface: its closed value
// set, a one-line plain-language gloss of what the answer changes, and its
// default. basis is non-empty only when that default is DERIVED from another
// knob's already-resolved value (ci_mode's on tracker) — FR-12 requires a
// derived default to name what it was derived from.
type question struct {
	key   string // the knobs.* key a refusal or abort names
	label string // the short prompt label ("issue tracker")
	gloss string // one line, plain language: what the answer changes
	def   string
	basis string // non-empty only for a derived default

	allowed []string // the closed value set resolveEnum validates against
	// (unused by a question — cli_surfaces — whose real value set is
	// wider than a closed list; that question sets valuesText instead)
	valuesText string
}

// valueSet renders the closed value set for display and for refusal/abort
// messages, e.g. "github|carina".
func (q question) valueSet() string {
	if q.valuesText != "" {
		return q.valuesText
	}
	return strings.Join(q.allowed, "|")
}

// render writes the question's plain-language line — the gloss IS the
// question (issue #898: the prompt surface speaks to a first-time user,
// not in knob taxonomy) — plus the recommendation's basis when one exists
// (FR-12). The trailing "label (values) [default]: " read-prompt stays
// ask's job so the bounded re-ask loop (FR-13) can re-print just that
// line, plus a short reason, on each retry rather than repeating the
// whole block.
func (q question) render(out io.Writer) {
	terminalPrintf(out, "\n%s\n", q.gloss)
	if q.basis != "" {
		terminalPrintf(out, "  recommended: %s — %s\n", q.def, q.basis)
	}
}

// promptLine is the read-prompt's left-hand side: the knob's short label
// with its value set inline, so the machine grammar rides the input line
// and the question line above stays plain English.
func (q question) promptLine() string {
	return fmt.Sprintf("  %s (%s)", q.label, q.valueSet())
}

// maxAskAttempts bounds FR-13's re-ask loop: the initial ask plus up to two
// re-asks — three attempts total — before a reported abort. A closed input
// stream makes ask (above) return the default immediately with no error, and
// every default is a legal member of its question's value set, so EOF always
// validates on the first attempt and never enters the retry path (today's
// EOF semantics, preserved).
const maxAskAttempts = 3

// ErrKnobAskAborted is FR-13's bounded re-ask abort: three consecutive
// invalid answers to one interactive knob question end the run. Branch with
// errors.Is; cmd/pyxis prints Error() verbatim, which — unlike the pre-T7
// behavior of one bad keystroke silently discarding every prior answer —
// names every knob already resolved (the F-12 finding this closes).
var ErrKnobAskAborted = errors.New("init: interactive knob resolution aborted")

// knobAskAbortError carries the failing question and every answer collected
// before it onto ErrKnobAskAborted.
type knobAskAbortError struct {
	key       string
	lastInput string
	allowed   string
	collected string
}

func (e *knobAskAbortError) Error() string {
	surface := "knobs." + e.key
	switch e.key {
	case "coordinate":
		// The conditional exchange is not a knob — no knobs.coordinate key
		// exists (review:impl #809 CAL-7); name the surface it actually is.
		surface = "the GitHub coordinate exchange"
	case "repository":
		// Likewise: the repository question SETTLES knobs (wiring.repo, and
		// on the local branch three more), but no knobs.repository key
		// exists in the v2 schema to name.
		surface = "the repository question"
	}
	return fmt.Sprintf("%v — %s: %q is not one of %s after %d attempts; answers already given: %s",
		ErrKnobAskAborted, surface, e.lastInput, e.allowed, maxAskAttempts, e.collected)
}

func (e *knobAskAbortError) Is(target error) bool { return target == ErrKnobAskAborted }

// collectedAnswers accumulates each knob's resolved value in asking order
// (FR-13): when a later question aborts, the error reports every answer
// already given rather than discarding the whole session over one bad
// keystroke (the pre-T7 behavior the F-12 finding named).
type collectedAnswers struct {
	pairs []string
}

func (c *collectedAnswers) add(key, value string) {
	c.pairs = append(c.pairs, fmt.Sprintf("%s=%s", key, value))
}

func (c *collectedAnswers) summary() string {
	if len(c.pairs) == 0 {
		return "(none yet)"
	}
	return strings.Join(c.pairs, ", ")
}

// askValidated renders q once (FR-12), then asks up to maxAskAttempts times,
// accepting the first answer resolve approves — resolve returns the CANONICAL
// value, which may differ from what was typed (the interactive lane forgives
// near-misses; #935). When it does differ, the acceptance is echoed so the
// user sees what their shorthand resolved to. An unresolvable answer names
// why and re-asks (FR-13); the final invalid attempt aborts naming the
// question, the last bad input, and every knob resolved so far.
func askValidated(pr *prompter, q question, resolve func(string) (string, bool), collected *collectedAnswers) (string, error) {
	// Checked before render, not just before ask: the gloss IS the question
	// (#898), so an interrupted run that renders it has still put a question
	// to someone who already asked to stop (#954).
	if pr.ctx.Err() != nil {
		return "", ErrInterrupted
	}
	q.render(pr.out)
	var last string
	for attempt := 1; attempt <= maxAskAttempts; attempt++ {
		a, err := pr.ask(q.promptLine(), q.def)
		if err != nil {
			return "", err
		}
		if canonical, ok := resolve(a); ok {
			if canonical != a {
				terminalPrintf(pr.out, "  → %s\n", canonical)
			}
			return canonical, nil
		}
		last = a
		if attempt < maxAskAttempts {
			terminalPrintf(pr.out, "  %q is not one of %s — try again (%d attempt(s) left)\n", a, q.valueSet(), maxAskAttempts-attempt)
		}
	}
	return "", &knobAskAbortError{key: q.key, lastInput: last, allowed: q.valueSet(), collected: collected.summary()}
}

// canonicalizeEnumInput maps a human's interactive answer onto the one
// allowed value it unambiguously names, or reports failure. Forgiveness is
// deliberately narrow (#935): case and surrounding space are ignored,
// underscore and hyphen are equivalent (`required_checks` →
// `required-checks`), and a leading fragment is accepted only when exactly
// one allowed value CONTAINS it — so `req` resolves to `required-checks`
// while `checks`, a substring of two levels, stays ambiguous and re-asks.
// This is the interactive lane only: the flag and answers-file lanes stay
// byte-strict (the v2 schema's enums are closed, and promptless runs are the
// machine surface).
func canonicalizeEnumInput(input string, allowed []string) (string, bool) {
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "_", "-")
	}
	n := norm(input)
	if n == "" {
		return "", false
	}
	for _, a := range allowed {
		if norm(a) == n {
			return a, true
		}
	}
	var prefixOf, containing []string
	for _, a := range allowed {
		if strings.HasPrefix(norm(a), n) {
			prefixOf = append(prefixOf, a)
		}
		if strings.Contains(norm(a), n) {
			containing = append(containing, a)
		}
	}
	if len(prefixOf) == 1 && len(containing) == 1 {
		return prefixOf[0], true
	}
	return "", false
}

// The recommended knob defaults (FR-4): the values an unset knob takes in a
// promptless run and the prompt's suggested answer interactively.
const (
	defaultTracker    = "github"
	defaultWiringRepo = "use"
	defaultProtection = "required-checks"
	surfaceAuto       = "auto" // the sentinel that means "detect the installed CLIs"
)

// detectableCLIs is the set of agent-CLI surfaces auto-detection probes on
// PATH (installer-v1 §3.5). It is the wiring catalog itself, not a second copy
// of it: a surface with no registered convention wires nothing, so detecting it
// would promise something init cannot deliver. The resolved list is whatever
// subset is actually installed — an empty result is legal (a repo with no agent
// CLI), and `pyxis surfaces` is how such a repo answers again later.
var detectableCLIs = converge.SupportedSurfaces()

// lookPath is the PATH probe seam (exec.LookPath); tests set PATH via t.Setenv
// and exercise the real thing, so this stays the stdlib call.
var lookPath = exec.LookPath

// trackerQuestion is knob 1/5 (FR-12).
func trackerQuestion() question {
	return question{
		key:     "tracker",
		label:   "issue tracker",
		gloss:   "Where should pyxis track issues and PRs? (carina is the built-in tracker; it works fully offline)",
		allowed: []string{"github", "carina"},
		def:     defaultTracker,
	}
}

// ciModeDefault derives ci_mode's recommended default from the resolved
// tracker and names the basis (FR-12): a carina/offline repo has no hosted CI
// plane, so only a github tracker earns the hosted leg by default.
func ciModeDefault(tracker string) (def, basis string) {
	if tracker == "github" || tracker == "forgejo" {
		return "both", "your GitHub tracker makes hosted CI available, so local hooks and GitHub Actions both run"
	}
	return "local", "carina works offline, so only the local pre-push hooks run"
}

// ciModeQuestion is knob 2/5: the derived default names its basis, FR-12's
// specific fit criterion.
func ciModeQuestion(tracker string) question {
	def, basis := ciModeDefault(tracker)
	return question{
		key:     "ci_mode",
		label:   "CI mode",
		gloss:   "Where should changes be checked before they land? On your machine (local git hooks), on GitHub (Actions), or both?",
		allowed: []string{"local", "actions", "both"},
		def:     def,
		basis:   basis,
	}
}

// wiringRepoQuestion and wiringProtectionQuestion are knob 3/5 (GitHub wiring
// depth), asked as its two sub-questions.
func wiringRepoQuestion() question {
	return question{
		key:     "wiring.repo",
		label:   "GitHub repo wiring",
		gloss:   "Should pyxis use an existing GitHub repository, or create a new private one for you?",
		allowed: []string{"use", "create"},
		def:     defaultWiringRepo,
	}
}

// protectionDefault derives wiring.protection's recommended default from the
// resolved ci_mode and names the basis (FR-12): requiring the hosted
// docs-gate check when ci_mode excludes hosted CI is exactly the cross-knob-
// coherence check's wedged pair (design init-hardening-v1 §3.1 check 6; plan
// init-hardening/001 T3) — the recommended default must never suggest it, so
// a promptless or EOF-defaulted run can never be born wedged by its own
// unset knobs.
func protectionDefault(ciMode string) (def, basis string) {
	if ciMode == "local" {
		return "none", "your CI runs locally only, so no hosted check could ever report on GitHub"
	}
	return defaultProtection, "hosted CI is on, so GitHub can require its check to pass before merging"
}

func wiringProtectionQuestion(ciMode string) question {
	def, basis := protectionDefault(ciMode)
	return question{
		key:     "wiring.protection",
		label:   "branch protection",
		gloss:   "How strictly should the main branch be protected? Not at all, require checks to pass, or checks plus a required review?",
		allowed: []string{"none", "required-checks", "checks+review"},
		def:     def,
		basis:   basis,
	}
}

// cliSurfacesQuestion is knob 4/5. Its true value set is open beyond the
// detectable subset (resolveSurfaces keeps any explicit list verbatim —
// adapters grow), so the values line names both halves honestly rather than
// claiming a closure that is not real.
func cliSurfacesQuestion() question {
	return question{
		key:        "cli_surfaces",
		label:      "agent CLI surfaces",
		gloss:      "Which coding agents should get pyxis's skills? (auto detects what's installed)",
		valuesText: fmt.Sprintf("%s | a comma-separated list (detectable: %s)", surfaceAuto, strings.Join(detectableCLIs, ", ")),
		def:        surfaceAuto,
	}
}

// argusQuestion is knob 5/5, opt-in and off by default (FR-16).
func argusQuestion() question {
	return question{
		key:     "argus",
		label:   "enable argus",
		gloss:   "Enable argus, the AI engineer that can drive work on its own? (off keeps every decision with you)",
		allowed: []string{"true", "false"},
		def:     "false",
	}
}

// resolveKnobs folds Answers into a fully-resolved config.Knobs (FR-3/FR-4).
// Promptless (any flag/answers input, --yes, or no stdin — pr==nil) takes the
// recommended default for every unset knob; interactive asks each unset knob
// through its FR-12 surface (value set, gloss, default) with FR-13's bounded
// re-ask, tracking every resolved value in collected so a later abort can
// report it. Every value is validated against the closed enums the v2 schema
// pins (config.Parse would reject an out-of-enum value, but resolving it here
// names the offending knob directly). No "auto" surface survives: an
// ["auto"] or unset-in-promptless surface set is expanded by PATH detection
// (an empty installed set is returned as an empty list).
//
// Once the five are resolved, it also runs resolveCoordinate — the ONE
// conditional exchange prd:installer/FR-23 permits beyond the five-knob
// boundary (design init-hardening-v1 §3.7; plan init-hardening/001 T5): a
// sixth question only in the narrow case GitHub mode is applicable, no
// remote resolves, and no promptless coordinate arrived.
func resolveKnobs(ctx context.Context, opts *Options, pr *prompter) (config.Knobs, error) {
	ans := opts.Answers
	var k config.Knobs
	var collected collectedAnswers

	// The repository decision comes FIRST in an interactive run (#1043):
	// it is the one answer that decides whether GitHub features in this
	// install at all, so asking it first is what lets "keep it local" skip
	// the three GitHub-only questions instead of asking and ignoring them.
	// A nil decision means the question did not apply (promptless, a
	// configured remote, no listing) and the older per-knob flow resolves
	// everything exactly as before.
	decided, err := decideRepository(ctx, opts, pr, &collected)
	if err != nil {
		return config.Knobs{}, err
	}
	if decided != nil && decided.Local {
		// Derived, not asked: GitHub is out of the picture, so a tracker on
		// GitHub, hosted CI, and branch protection are all unanswerable.
		// wire.Table declares zero gh-transport ops for this combination —
		// asserted by test — so preflight's GitHub checks stay inert and no
		// config-schema value beyond today's enums is needed.
		k.Tracker, k.CIMode = "carina", "local"
		k.Wiring = config.Wiring{Repo: defaultWiringRepo, Protection: "none"}
		collected.add("tracker", k.Tracker)
		collected.add("ci_mode", k.CIMode)
		collected.add("wiring.protection", k.Wiring.Protection)
		echoLocalDerivation(opts.Stdout, k)
	} else {
		trackerQ := trackerQuestion()
		if opts.Integrations.SchemaVersion != "" {
			trackerQ.allowed = append(trackerQ.allowed, "forgejo")
		}
		tracker, terr := resolveEnum(ans.Tracker, trackerQ, pr, &collected)
		if terr != nil {
			return config.Knobs{}, terr
		}
		k.Tracker = tracker
		collected.add("tracker", tracker)

		// ci_mode's default depends on the tracker: GitHub gets both (hooks +
		// Actions), a carina/offline repo gets local (there is no Actions plane).
		ci, cerr := resolveEnum(ans.CIMode, ciModeQuestion(k.Tracker), pr, &collected)
		if cerr != nil {
			return config.Knobs{}, cerr
		}
		k.CIMode = ci
		collected.add("ci_mode", ci)

		// wiring.repo is DERIVED when the repository question settled it;
		// only a run that never reached that question still asks the enum.
		repo := ""
		if decided != nil {
			repo = decided.WiringRepo
		} else {
			r, rerr := resolveEnum(ans.WiringRepo, wiringRepoQuestion(), pr, &collected)
			if rerr != nil {
				return config.Knobs{}, rerr
			}
			repo = r
		}
		collected.add("wiring.repo", repo)

		prot, perr := resolveEnum(ans.WiringProtection, wiringProtectionQuestion(k.CIMode), pr, &collected)
		if perr != nil {
			return config.Knobs{}, perr
		}
		collected.add("wiring.protection", prot)
		k.Wiring = config.Wiring{Repo: repo, Protection: prot}
	}

	argus, err := resolveArgus(ans.Argus, pr, &collected)
	if err != nil {
		return config.Knobs{}, err
	}
	k.Argus = argus
	collected.add("argus", fmt.Sprintf("%t", argus))

	surfaces, err := resolveSurfaces(ans.CLISurfaces, pr, &collected)
	if err != nil {
		return config.Knobs{}, err
	}
	k.CLISurfaces = surfaces
	collected.add("cli_surfaces", strings.Join(surfaces, ","))

	// The first question OWNS the coordinate once it was asked: a second
	// exchange would put two questions to one decision. Everything else
	// still resolves through the original path (a configured remote, a
	// promptless answer, or the conditional ask).
	if decided != nil {
		opts.Answers.Coordinate = decided.Coordinate
		// Echoed here, not when it was read: a coordinate derived from a
		// configured remote is only NEWS if GitHub features in the install
		// the knobs finally describe. Announcing it to a local install
		// would say GitHub is involved when nothing about it will run.
		if decided.FromRemote && preflight.GHOpsImplied(k) {
			echoCoordinate(opts.Stdout, decided.Coordinate)
		}
	} else if err := resolveCoordinate(ctx, opts, k, pr, &collected); err != nil {
		return config.Knobs{}, err
	}

	return k, nil
}

// echoLocalDerivation names what "keep it local" settled, so the three
// questions it silences are visible as answers rather than as omissions.
func echoLocalDerivation(w io.Writer, k config.Knobs) {
	if w == nil {
		return
	}
	terminalPrintf(w, "\n  local install: tracker=%s · ci_mode=%s · protection=%s — no GitHub wiring\n",
		k.Tracker, k.CIMode, k.Wiring.Protection)
}

// ---- FR-6/FR-8/ABUSE-1: the conditional GitHub-coordinate exchange --------
//
// The exchange design init-hardening-v1 §3.7 and plan init-hardening/001 T5
// add on top of the five-knob surface above: NOT a sixth knob question (T7
// governs those alone), but the one contracted exception prd:installer/FR-23
// names — asked only when GitHub mode is applicable, no configured remote
// resolves, and no promptless coordinate arrived (T3's own resolution,
// unchanged: a flag/answers coordinate or a refusal it names by itself).

// resolveCoordinate is the conditional exchange's gate: it fires only when
// githubModeApplicable(knobs), no coordinate already resolves (a configured
// git remote wins first — FR-6's "origin-present runs ask nothing"; a
// promptless flag/answers coordinate next — T3's own FR-7 contract), and the
// run is interactive (pr != nil; promptless resolution and its refusal stay
// entirely T3's, never duplicated here). A coordinate resolved from ANY
// source is echoed once (FR-8's "the pre-run check summary names the
// coordinate before any creation"); one elicited here is folded onto
// opts.Answers.Coordinate so the unchanged preflight GitHub-mode check
// (runKnobChecks, run right after this) and the apply-time remote
// materialization (stageRemote) both resolve it exactly as a promptless
// answer would — the one seam T3 already built, never a second copy.
func resolveCoordinate(ctx context.Context, opts *Options, knobs config.Knobs, pr *prompter, collected *collectedAnswers) error {
	if opts.Integrations.SchemaVersion != "" {
		return nil
	}
	if !preflight.GHOpsImplied(knobs) {
		return nil
	}
	// A blank target (adopt's promptless default-knob resolution,
	// WriteAdoptionConfig's &Options{}) has no repository to parse a remote
	// from — and `git -C ""` would read the PROCESS's own cwd, leaking
	// whatever repository the caller happens to run inside. No target, no
	// exchange: the coordinate is meaningless there.
	if strings.TrimSpace(opts.Target) == "" {
		return nil
	}
	if slug := preflight.RemoteSlug(ctx, opts.Target, ""); slug != "" {
		echoCoordinate(opts.Stdout, slug)
		return nil
	}
	if slug := strings.TrimSpace(opts.Answers.Coordinate); slug != "" {
		echoCoordinate(opts.Stdout, slug)
		return nil
	}
	if pr == nil {
		return nil // promptless, neither present: T3's own named refusal fires next
	}
	slug, err := askCoordinate(pr, collected)
	if err != nil {
		return err
	}
	if slug == "" {
		return nil // a blank answer/EOF — no default exists; fall through to T3's refusal
	}
	opts.Answers.Coordinate = slug
	collected.add("coordinate", slug)
	// No echo here: the user just typed this exact string one line up
	// (issue #898); the echo is for the derived sources above, where the
	// resolved coordinate is otherwise invisible.
	return nil
}

// coordinateGloss is the exchange's own one-time prompt line (FR-6, ABUSE-1)
// — distinct from the five knobs' FR-12 question surface: it names why it is
// asking (no remote configured) rather than a value set, since the legal
// "values" here are the whole owner/name grammar, not a closed enum.
const coordinateGloss = "No git remote is set — which GitHub repository should pyxis use (or create)? It becomes this directory's git remote."

// coordinateLabel is the read-prompt label askCoordinate re-prints on retry.
const coordinateLabel = "  repository (owner/name)"

// askCoordinate elicits the owner/name coordinate, grammar-validating each
// answer against wire.ValidCoordinate — the SAME grammar the promptless
// preflight check validates against (ABUSE-1: "everything else re-asks",
// never a second, driftable copy) — and re-asking up to maxAskAttempts
// (FR-13's bound, reused here for one consistent re-ask rhythm though this
// exchange is its own contract, not a sixth knob question). A blank answer
// (a bare Enter, or a closed stdin taking today's EOF semantics) is treated
// as "no coordinate supplied" rather than re-asked — ABSENCE, not failure:
// there is no sensible default to fall back to, so the run falls through
// to the preflight GitHub-mode check's own named refusal (T3, FR-7), whose
// remedy names every supply path, this prompt included. Operator-ruled
// (review:impl #809 FND-5's escalation brief, 2026-07-29): FR-6's re-ask
// binds MALFORMED input only; blank and EOF stay indistinguishable at the
// prompter seam by design, and both stop cleanly — a scripted run's closed
// stream never burns re-ask attempts. Three consecutive MALFORMED answers
// still abort loudly, reusing knobAskAbortError so the report names every
// knob already resolved — exactly like a knob's own bounded abort.
func askCoordinate(pr *prompter, collected *collectedAnswers) (string, error) {
	return askCoordinateWithGloss(pr, coordinateGloss, collected)
}

// askCoordinateWithGloss is askCoordinate with the framing chosen by the
// caller: the same validation and the same bounded re-ask, asked for a
// different reason.
func askCoordinateWithGloss(pr *prompter, gloss string, collected *collectedAnswers) (string, error) {
	terminalPrintf(pr.out, "\n%s\n", gloss)
	var last string
	for attempt := 1; attempt <= maxAskAttempts; attempt++ {
		a, err := pr.ask(coordinateLabel, "")
		if err != nil {
			return "", err
		}
		if a == "" {
			return "", nil
		}
		if wire.ValidCoordinate(a) {
			return a, nil
		}
		last = a
		if attempt < maxAskAttempts {
			terminalPrintf(pr.out, "  %q is not a valid owner/name coordinate — try again (%d attempt(s) left)\n", a, maxAskAttempts-attempt)
		}
	}
	return "", &knobAskAbortError{
		key: "coordinate", lastInput: last,
		allowed:   "owner/name (letters, digits, - _ . only; no leading dash on either side)",
		collected: collected.summary(),
	}
}

// echoCoordinate prints the resolved GitHub coordinate for the DERIVED
// sources only — a configured remote or a promptless answer — where the
// value would otherwise be invisible; the interactive exchange no longer
// echoes what the user just typed (#898), and its coordinate surfaces in
// the wiring-checklist header instead. For the derived sources this still
// runs before runKnobChecks and any write, so the echo is pre-create. w is
// never nil by the time resolveKnobs runs (Init defaults opts.Stdout to
// io.Discard first).
func echoCoordinate(w io.Writer, slug string) {
	if w == nil {
		return // a caller outside Init's normalization (a raw Options) — nothing to echo to
	}
	terminalPrintf(w, "GitHub coordinate: %s\n", slug)
}

func terminalPrintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) // Terminal output does not govern the installation result.
}

// resolveEnum resolves one closed-enum knob: the provided value if set (and
// valid), else the prompted value (interactive, FR-12/FR-13) or the default
// (promptless), validated against q.allowed. An out-of-enum PROVIDED value (a
// flag or answers-file value — never re-askable; there is no prompter behind
// it) refuses by name immediately; an out-of-enum TYPED value re-asks through
// askValidated instead.
func resolveEnum(provided string, q question, pr *prompter, collected *collectedAnswers) (string, error) {
	v := strings.TrimSpace(provided)
	if v == "" {
		if pr != nil {
			a, err := askValidated(pr, q, func(s string) (string, bool) { return canonicalizeEnumInput(s, q.allowed) }, collected)
			if err != nil {
				return "", err
			}
			v = a
		} else {
			v = q.def
		}
	}
	if !slices.Contains(q.allowed, v) {
		return "", fmt.Errorf("init: knobs.%s %q is not one of %s (the v2 schema's enums are closed)", q.key, v, q.valueSet())
	}
	return v, nil
}

// resolveArgus resolves the opt-in argus knob (FR-16, off by default). A
// provided pointer is authoritative; interactively it is asked through the
// FR-12/FR-13 surface with the recommended default "false"; promptless-unset
// is false.
func resolveArgus(provided *bool, pr *prompter, collected *collectedAnswers) (bool, error) {
	if provided != nil {
		return *provided, nil
	}
	if pr == nil {
		return false, nil
	}
	a, err := askValidated(pr, argusQuestion(), func(s string) (string, bool) { return canonicalizeEnumInput(s, []string{"true", "false"}) }, collected)
	if err != nil {
		return false, err
	}
	return a == "true", nil
}

// resolveSurfaces resolves the cli_surfaces knob. An explicit non-auto set is
// kept verbatim (open value set — adapters grow). ["auto"], an empty set, or
// an unset knob triggers PATH detection: the resolved list is the installed
// subset of detectableCLIs (possibly empty — a legal result a caller notes).
// Interactively an unset knob is asked through the FR-12/FR-13 surface, "auto"
// being the default; the one re-askable defect is mixing auto with explicit
// names (auto means "detect", not a surface name).
func resolveSurfaces(provided []string, pr *prompter, collected *collectedAnswers) ([]string, error) {
	set := trimNonEmpty(provided)
	if len(set) == 0 {
		if pr != nil {
			// A free-form CSV, not a closed enum: no canonicalization, the
			// answer passes through as typed when the auto/explicit mix rule
			// holds.
			valid := func(raw string) (string, bool) {
				parsed := splitCSV(raw)
				return raw, len(parsed) <= 1 || !slices.Contains(parsed, surfaceAuto)
			}
			a, err := askValidated(pr, cliSurfacesQuestion(), valid, collected)
			if err != nil {
				return nil, err
			}
			set = splitCSV(a)
		} else {
			set = []string{surfaceAuto}
		}
	}
	if len(set) == 1 && set[0] == surfaceAuto {
		return detectSurfaces(), nil
	}
	// An explicit set that still contains "auto" mixed with names is a
	// contradiction — auto means "detect", not a surface name.
	if slices.Contains(set, surfaceAuto) {
		return nil, fmt.Errorf("init: knobs.cli_surfaces mixes %q (detect) with explicit names %v — pass either auto alone or an explicit list", surfaceAuto, set)
	}
	return set, nil
}

// detectSurfaces probes PATH for each detectable agent CLI and returns the
// installed subset, in the canonical order. Empty is a valid result.
func detectSurfaces() []string {
	var out []string
	for _, cli := range detectableCLIs {
		if _, err := lookPath(cli); err == nil {
			out = append(out, cli)
		}
	}
	return out
}

// resolveIdentity is step 3 (UB-10): the committer identity, probed BEFORE any
// mutation so a refusal leaves zero half state. Each half — name, email —
// resolves ambient git config → the answers file → an interactive prompt → a
// named refusal carrying the exact `git config --global …` remedy. The ambient
// probe runs `git config --get` from the target: with no repository yet it
// still reads the global and system scopes (git's own resolution), which is
// exactly the identity a commit there would use.
func resolveIdentity(ctx context.Context, target string, ans Answers, pr *prompter) (Identity, error) {
	name, nameSrc, err := resolveIdentityHalf(ctx, target, "user.name", ans.GitName, pr, "committer name", `git config --global user.name "Your Name"`)
	if err != nil {
		return Identity{}, err
	}
	email, emailSrc, err := resolveIdentityHalf(ctx, target, "user.email", ans.GitEmail, pr, "committer email", `git config --global user.email "you@example.com"`)
	if err != nil {
		return Identity{}, err
	}
	return Identity{Name: name, Email: email, NameSource: nameSrc, EmailSource: emailSrc}, nil
}

// resolveIdentityHalf resolves one identity half through the UB-10 ladder and
// reports its source ("ambient" | "answers" | "prompt"). An unresolved half is
// the named refusal citing the remedy for THIS half.
func resolveIdentityHalf(ctx context.Context, target, key, provided string, pr *prompter, prompt, remedy string) (value, source string, err error) {
	v, found, err := ambientIdentity(ctx, target, key)
	if err != nil {
		return "", "", err
	}
	if found {
		return v, "ambient", nil
	}
	if v := strings.TrimSpace(provided); v != "" {
		return v, "answers", nil
	}
	if pr != nil {
		v, aerr := pr.ask(prompt, "")
		if aerr != nil {
			return "", "", aerr
		}
		if v = strings.TrimSpace(v); v != "" {
			return v, "prompt", nil
		}
	}
	return "", "", fmt.Errorf("init: git %s is unset at every scope and init must create a commit (UB-10) — set it and re-run, e.g. %s (or pass it in the answers file); no repository was created", key, remedy)
}

// ambientIdentity reads one git identity half via `git config --get <key>`
// from the target directory (git's own cross-scope resolution — local when a
// repo exists, else global+system).
// ambientIdentity reads one identity half from git's own configuration.
// The three outcomes are kept distinct (#954): a value, a git-confirmed
// ABSENCE (found=false, err=nil — `git config --get` exits 1 for a key it
// cannot find), and a probe that COULD NOT RUN (err non-nil). Collapsing
// the third into the second — which this did — makes a cancelled context or
// a broken git indistinguishable from an unset key, so init asks a human
// for an identity `git config` would have answered.
func ambientIdentity(ctx context.Context, target, key string) (string, bool, error) {
	out, err := gitOutput(ctx, target, "config", "--get", key)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return "", false, fmt.Errorf("init: probe git %s: %w", key, cerr)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", false, nil // git ran and reported the key unset
		}
		// Anything else (a malformed config file, an unreadable target)
		// means git could not answer at all — refuse with what git said
		// rather than asking a human to retype what it holds.
		return "", false, fmt.Errorf("init: probe git %s: %w — fix the git configuration it names, then re-run", key, err)
	}
	v := strings.TrimSpace(out)
	if v == "" {
		return "", false, nil
	}
	return v, true, nil
}

// gitInitMain initializes the repository on branch (basis FR-7) — the
// admission probe's already-resolved value (Init's step 1); re-probing here
// would just repeat the same read-only check for nothing. git's own
// default-branch name varies by version/config, so the branch is pinned
// with `init -b <branch>` — supported by every git this project floors on;
// a fresh repo's HEAD then points at the intended branch deterministically.
func gitInitMain(ctx context.Context, target, branch string) error {
	if _, err := gitOutput(ctx, target, "init", "-b", branch); err != nil {
		return fmt.Errorf("git init %s: %w", target, err)
	}
	return nil
}

// writeLocalIdentity writes the resolved identity repo-local, but ONLY for
// halves whose source is answers or prompt (UB-10): an ambient half is already
// in git's config (writing it back would duplicate it needlessly). After this,
// no commit init makes can fail for identity. Each half it actually sets is
// journaled (plan init-hardening/001 T4): a later stage failure unsets it on
// unwind, unless init also created the repository, in which case the whole
// .git goes and the individual unset is moot.
func writeLocalIdentity(ctx context.Context, target string, id Identity, j *journal) error {
	if id.NameSource == "answers" || id.NameSource == "prompt" {
		if _, err := gitOutput(ctx, target, "config", "user.name", id.Name); err != nil {
			return fmt.Errorf("set repo-local user.name: %w", err)
		}
		j.recordConfig("user.name")
	}
	if id.EmailSource == "answers" || id.EmailSource == "prompt" {
		if _, err := gitOutput(ctx, target, "config", "user.email", id.Email); err != nil {
			return fmt.Errorf("set repo-local user.email: %w", err)
		}
		j.recordConfig("user.email")
	}
	return nil
}

// gitCommitPaths stages EXACTLY the named paths (never add-all — the
// shared-clone lesson: an add-all sweeps another session's untracked WIP) and
// commits them under subject, returning the new commit's sha. Identity is
// already set repo-local, so no author/committer override is needed. An empty
// commit (nothing staged) cannot happen given init always writes the config;
// if git reports it anyway, it surfaces as the wrapped commit error.
//
// The commit runs with ALL operand hooks disabled for exactly this invocation
// (`-c core.hooksPath=<empty>`): init's two genesis commits are the bootstrap
// that CREATES the gated tree, and the Hooks stage (which runs before the tree
// commit) has just activated the operand's gate hooks — the pre-commit gate AND
// the prepare-commit-msg argus stamp, both of which delegate to the `pyxis`
// binary, which is not yet on PATH on a fresh operand (the bootstrap shim / dev
// shell primes it later, installer-v1 §3.4). Running those hooks on init's own
// commits would fail closed on every fresh install; `--no-verify` alone is
// insufficient (git still runs prepare-commit-msg under it). Pointing
// core.hooksPath at an empty directory for this one commit disables every hook
// deterministically. The gate still governs every SUBSEQUENT operand commit;
// only init's genesis pair is exempt — deterministic engine + seeded content
// the green proof separately verifies (§3.5 step 9).
func gitCommitPaths(ctx context.Context, target string, paths []string, subject string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("commit %q: no paths to stage (programmer error — init always writes the config)", subject)
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := gitOutput(ctx, target, args...); err != nil {
		return "", fmt.Errorf("stage %v: %w", paths, err)
	}
	// core.hooksPath="" (empty) resolves to no hooks directory, so git runs
	// none of them — the deterministic, all-hooks bypass for the genesis pair.
	if _, err := gitOutput(ctx, target, "-c", "core.hooksPath=", "commit", "--no-verify", "-m", subject); err != nil {
		return "", fmt.Errorf("commit %q: %w", subject, err)
	}
	sha, err := gitOutput(ctx, target, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("read commit sha for %q: %w", subject, err)
	}
	return strings.TrimSpace(sha), nil
}

// gitOutput runs one git subcommand in target, honoring ctx, returning stdout.
// A non-zero exit wraps stderr so refusals stay named.
func gitOutput(ctx context.Context, target string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", target}, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out.String(), nil
}

// gitLines runs a git subcommand and splits stdout into non-empty lines.
func gitLines(ctx context.Context, target string, args ...string) ([]string, error) {
	out, err := gitOutput(ctx, target, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for ln := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, ln)
		}
	}
	return lines, nil
}

// writeFileAtomic writes b to path via a tmp file in the same directory + a
// rename, creating the parent directory first, so a reader sees only absent or
// complete bytes. A local copy of converge's discipline — this is
// internal/installer, and the two operand writes (config, .gitignore) are
// small.
func writeFileAtomic(path string, b []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create tmp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName) // best-effort: an abandoned tmp is never trusted
		}
	}()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close() // the write error governs; the tmp is removed by the deferred cleanup
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	tmpName = ""
	return nil
}

// renderConfig emits the pyxis-config/v2 config the installer solution shape
// shows and that config.Parse round-trips (parseRendered is the guard). The
// key order is deterministic; the spelling matches T2's parser exactly — the
// flow-map for wiring and the flow-list for cli_surfaces, quoted pins,
// artifact_source only when set, mode release, every paths: key at its
// compiled default (a non-default value would be the FR-18 refusal), and the
// epoch block iff ep != nil. Values never carry an unquoted " #" (the parser
// would clip an inline comment); pins and paths are simple tokens or quoted.
func renderConfig(pin Pin, artifactSource string, knobs config.Knobs, branch string, ep *config.Epoch) []byte {
	var b strings.Builder
	b.WriteString("schema_version: " + config.SchemaVersionV2 + "\n")

	// system: the pin pair (quoted — the shape's spelling), the optional
	// artifact-source override, mode release (an installed repo).
	b.WriteString("system:\n")
	fmt.Fprintf(&b, "  version: %q\n", pin.Version)
	fmt.Fprintf(&b, "  manifest_sha256: %q\n", pin.ManifestSHA256)
	if s := strings.TrimSpace(artifactSource); s != "" {
		fmt.Fprintf(&b, "  artifact_source: %q\n", s)
	}
	b.WriteString("  mode: release\n")

	// knobs: the five, resolved. wiring is the closed flow map; cli_surfaces
	// the flow list (empty renders as []).
	b.WriteString("knobs:\n")
	fmt.Fprintf(&b, "  tracker: %s\n", knobs.Tracker)
	fmt.Fprintf(&b, "  ci_mode: %s\n", knobs.CIMode)
	fmt.Fprintf(&b, "  wiring: { repo: %s, protection: %s }\n", knobs.Wiring.Repo, knobs.Wiring.Protection)
	fmt.Fprintf(&b, "  cli_surfaces: [%s]\n", strings.Join(knobs.CLISurfaces, ", "))
	fmt.Fprintf(&b, "  argus: %t\n", knobs.Argus)

	// branch: captured (only main honored in v1).
	fmt.Fprintf(&b, "branch: %s\n", branch)

	// paths: every key at its compiled default, in sorted order. Drafts can
	// later be customized by the feature-draft lifecycle.
	b.WriteString("paths:\n")
	defaults := config.PathDefaults()
	keys := make([]string, 0, len(defaults))
	for k := range defaults {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s: %s\n", k, defaults[k])
	}

	// epoch: written only by the epoch commit (step 8).
	if ep != nil {
		b.WriteString("epoch:\n")
		fmt.Fprintf(&b, "  system_version: %q\n", ep.SystemVersion)
		fmt.Fprintf(&b, "  commit: %q\n", ep.Commit)
		fmt.Fprintf(&b, "  manifest_sha256: %q\n", ep.ManifestSHA256)
	}
	return []byte(b.String())
}

// trimNonEmpty trims each item and drops empties, preserving order.
func trimNonEmpty(items []string) []string {
	var out []string
	for _, it := range items {
		if t := strings.TrimSpace(it); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// splitCSV splits a comma-separated answer into trimmed, non-empty items.
func splitCSV(s string) []string {
	return trimNonEmpty(strings.Split(s, ","))
}
