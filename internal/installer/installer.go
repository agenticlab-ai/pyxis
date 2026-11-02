// Package installer owns `pyxis init`: admit the target, collect settings and
// a Git destination, converge the payload, seed the repository, install hooks,
// commit and verify the initial tree, and publish it before final host wiring.
// Seeding, hooks, wiring, green proof and welcome use the registered stage
// slots below. A possibly published installation can resume its exact head.
//
// # The two-commit epoch resolution (§3.5 step 8)
//
// The epoch cannot cite a commit that contains it: epoch.commit is "the
// initial commit init creates" (the solution shape's operand config), but
// the epoch block lives inside .pyxis/config.yaml, which that commit
// carries. Squaring the self-reference honestly takes two commits:
//
//	commit #1 "pyxis init — converged tree": every init write — the
//	converged tree, the operand .gitignore, and the config WITHOUT an
//	epoch block. This IS the initial commit §3.5 step 1 speaks of.
//	commit #2 "pyxis init — epoch": the config gains exactly one block,
//	epoch{system_version, commit: <sha of #1>, manifest_sha256}.
//
// Deterministic, no self-reference, and the epoch points at the commit
// that introduced the converged tree — consistent with the design's "the
// epoch commit (the initial commit init creates)".
//
// # Committer identity (UB-10)
//
// The five knobs carry no identity, and the frozen verb surface has no
// identity flags — the answers file is the vehicle: optional git_name /
// git_email keys beyond the five knobs. Init probes `git config --get
// user.name` / `user.email` (every scope git itself would use for the
// commit) BEFORE any mutation — before git init — so an identity refusal
// leaves zero half state. Each half resolves ambient config → answers →
// interactive prompt (pure-interactive mode only); an unresolved half is
// the named refusal carrying the exact `git config --global …` remedy.
// Halves resolved from answers or a prompt are written repo-local after
// git init, so no commit init attempts can fail for identity.
//
// # The pin handoff
//
// Init must write the system pin pair (DINV-2) but the shape's flag list
// carries no pin flags — the bootstrap shim is what knows them (it just
// verified pyxis against the documented pin). As implementation latitude,
// `pyxis init` grows --system-version and --manifest-sha256 (required; the
// bootstrap invocation passes them) plus --artifact-source (the FR-22
// mirror override, captured into the config and loudly surfaced by preview
// and doctor).
//
// # The operand .gitignore
//
// The payload stages tracked .pyxis/** plus the index/workflow overlays
// (internal/release/stage.go) — the source repo's root .gitignore is NOT
// payload content, so a fresh operand would commit the .pyxis/bin cache
// the first time anything fetched. Init therefore writes a minimal operand
// .gitignore (the gitignored runtime dirs) as an init write: repo-owned
// from birth like the config, not manifest-listed, never touched by
// converge.
//
// # EVT-1
//
// install_completed semantically covers the WHOLE init, T8's tail
// included, so the pipeline is an ordered stage list with T8's slots as
// named no-ops and EVT-1 fires only after every stage succeeds — when T8
// fills the slots the event's meaning is already right. Preview emits no
// EVT-1: nothing was installed. Zero telemetry egress (DINV-7): the event
// goes to the local logger only. No verb here performs model inference
// (NFR-6).
package installer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// Pin is the system pin pair the bootstrap hands to init (DINV-2): the one
// release version and the manifest hash everything else verifies through.
type Pin struct {
	Version        string // release tag ("v0.4.0"; a missing leading v is normalized)
	ManifestSHA256 string // 64-char sha256 hex, the trust root
}

// Answers carries the flag/answers-file knob inputs plus the optional
// UB-10 identity halves. Zero values mean "unset": under a promptless run
// an unset knob takes its recommended default (FR-4), and interactively it
// is asked. Argus is a pointer because false is a real answer, not unset.
type Answers struct {
	Tracker          string   // github | carina
	CIMode           string   // local | actions | both
	WiringRepo       string   // use | create
	WiringProtection string   // none | required-checks | checks+review
	CLISurfaces      []string // explicit set, or exactly ["auto"] to detect; nil = unset
	Argus            *bool    // nil = unset
	GitName          string   // optional identity half (UB-10)
	GitEmail         string   // optional identity half (UB-10)
	// Coordinate is the promptless owner/name GitHub coordinate (plan
	// init-hardening/001 T3; prd:init-hardening FR-7): consulted by the
	// GitHub-mode preflight check only when the target carries no
	// configured git remote to parse one from — a configured remote
	// always wins. "" means none was supplied.
	Coordinate string
	// RemoteURL names the Git publication destination when no remote is set.
	// It accepts a local bare repository path or a Git transport URL.
	RemoteURL string
}

// Options is Init's closed operand.
type Options struct {
	// Target is the directory to install into — cmd/pyxis passes the
	// working directory (the verb takes no positional). It must exist.
	Target string
	// Pin is required: init cannot write a config without it (DINV-2).
	Pin Pin
	// ArtifactSource is the optional FR-22 override, captured verbatim
	// into system.artifact_source.
	ArtifactSource string
	// Answers are the flag/answers-file inputs (flags already folded over
	// the file by the caller; flags win).
	Answers Answers
	// AnswersProvided marks that ANY flag or answers-file input arrived:
	// the run is then promptless (FR-4) and unset knobs take the
	// recommended defaults.
	AnswersProvided bool
	// AcceptDefaults is --yes: promptless, every unset knob defaulted.
	AcceptDefaults bool
	// Preview reports the full would-do set and performs nothing — zero
	// writes, zero git init, zero fetches (FR-24, formerly FR-19): every
	// check outcome, the exact write set, and the declared wiring operation
	// table, cold cache included (plan init-hardening/001 T9).
	Preview bool
	// Stdin feeds interactive elicitation; nil refuses pure-interactive
	// runs by name (the FR-4 remedy).
	Stdin io.Reader
	// Stdout is the prompt/echo surface; nil discards.
	Stdout io.Writer
	// Logger receives the structured events (EVT-1 and converge's EVT-2);
	// nil uses slog.Default().
	Logger *slog.Logger
	// Fetcher overrides converge's artifact transport (tests); nil uses
	// the scheme dispatch.
	Fetcher resolve.Fetcher
	// WireRunner overrides the github-mode wiring transport — the one gh
	// boundary (internal/wire.Runner). nil uses the production gh CLI
	// (wire.GHRunner{}); an out-of-process caller that needs hermetic wiring
	// (the e2e matrix's GitHub-mode combos, plan installer/001 T14) injects a
	// transcript fake here. Symmetric to Fetcher: a consequential external
	// operation behind an injectable seam, default unchanged.
	WireRunner wire.Runner
	// PublicationGit injects the Git transport for initial publication.
	// Nil uses the real Git CLI; local Git, GitHub, and Forgejo share it.
	PublicationGit func(context.Context, string, ...string) (string, error)
	// Integrations is an optional capability profile supplied before init; its
	// validated credential references are captured into the repository config.
	Integrations         integrations.Config
	ForgejoClientFactory func(integrations.ResolvedRepository) (*forgejo.Client, error)
	CredentialExecutable string // optional durable pyxis executable for the Git credential helper
}

// Commit is one init-created commit.
type Commit struct {
	SHA     string
	Subject string
}

// Identity is the resolved committer identity and where each half came
// from: "ambient" (git config already answers), "answers" (the answers
// file's git_name/git_email — written repo-local), or "prompt".
type Identity struct {
	Name        string
	Email       string
	NameSource  string
	EmailSource string
}

// Report is Init's one result struct — real runs fill the performed sets,
// preview fills the would-do sets.
type Report struct {
	RunID   string
	Preview bool

	// Admission.
	GitInited    bool   // real runs: no repository existed, init ran git init
	WouldGitInit bool   // preview: a real run would git init
	Branch       string // the branch the init commits land on (captured into the config)

	// Resolution.
	Knobs    config.Knobs // the five, resolved; an "auto" surface set is already expanded
	Identity Identity

	// Checks is the knob-dependent preflight registry's full outcome set
	// (design init-hardening-v1 §3.1 checks 3/4/6; plan init-hardening/001
	// T3): GitHub-mode auth/coordinate/existence, best-effort scope
	// advisories, and cross-knob coherence — evaluated once the knobs are
	// resolved but before the first write. Preview evaluates the identical
	// registry call (plan init-hardening/001 T9, previewRun below) and
	// reports every outcome here too — a refusal included, since preview's
	// whole point is showing everything in one pass rather than stopping.
	Checks preflight.Report

	// Writes.
	ConfigPath       string // the operand config, slash-relative
	GitIgnoreWritten bool
	ConfigBytes      []byte      // preview only: the exact config a real run would first write (epoch absent)
	WouldSeed        []SeededDoc // preview only: repository-owned seeds created outside converge

	// The converge invocation's own report (real or preview).
	Converge converge.Report

	// WiringOps is preview's declared wiring operation table (design
	// init-hardening-v1 §3.1/§3.7; prd:installer/FR-24; plan
	// init-hardening/001 T9): the same wire.Table(...) a real run's Wiring
	// stage (stageWiringGitHub) builds and walks, computed and reported
	// here WITHOUT executing any of it — Table is a pure derivation, no IO
	// (internal/wire's own contract). Empty when the resolved knobs declare
	// no gh-transport operation, or on a real run: T8.Wiring carries the
	// executed audit there instead.
	WiringOps []wire.Op

	// Commits + epoch (real runs: exactly two commits — [0] the converged
	// tree, [1] the epoch write; Epoch.Commit == Commits[0].SHA).
	Commits      []Commit
	EpochWritten bool
	Publication  PublicationResult
	Resumed      bool // resumed an installer-owned pending publication

	// Stages is the ordered pipeline: the stages that ran (real runs) or
	// would run (preview). T8's slots appear by name either way.
	Stages []string

	// T8 carries the tail stages' outcomes (plan installer/001 T8): the
	// seeded set, the dependency/hooks/wiring/green-proof/welcome results.
	// Defined in tail.go beside the stages that fill it; zero on preview.
	T8 t8Report

	// Rollback is populated (plan init-hardening/001 T4, FR-10) exactly when
	// a stage failure past the point mutation began triggered the journal's
	// reverse unwind — nil on every successful run and on preview.
	Rollback *RollbackReport
}

// The two init commit subjects (the two-commit epoch resolution above).
const (
	commitTreeSubject  = "pyxis init — converged tree"
	commitEpochSubject = "pyxis init — epoch"
)

// now is the clock seam (hermetic tests never depend on it beyond
// presence; EVT-1's duration_ms reads through it).
var now = time.Now

// stageFunc is one init-pipeline stage body over the shared run state.
type stageFunc func(ctx context.Context, run *initRun) error

func noopStage(context.Context, *initRun) error { return nil }

// t8Slots are the pipeline's not-yet-landed tail stages (plan installer/001
// T8: seeding, hooks, wiring, green proof, welcome). They are
// registered no-ops so the pipeline ORDER and EVT-1's fire-at-the-end
// contract are fixed now; T8 fills the fields without touching Init. Tests
// inject a failing slot to prove EVT-1 stays silent on a failed pipeline.
var t8Slots = struct {
	Seed, Hooks, Wiring, GreenProof, Welcome stageFunc
}{noopStage, noopStage, noopStage, noopStage, noopStage}

// pipelineStages assembles the ordered installation stages at run time so
// injected slots take effect. The verified initial commits are published
// before branch protection and hosted welcome work; local work items are
// seeded before the tree commit so they travel in the first push.
func pipelineStages() []struct {
	name string
	run  stageFunc
} {
	return []struct {
		name string
		run  stageFunc
	}{
		{"remote", stageRemote},
		{"converge", stageConverge},
		{"seed", t8Slots.Seed},
		{"hooks", t8Slots.Hooks},
		{"wiring", t8Slots.Wiring},
		{"welcome-seed", stageSeedLocalWelcome},
		{"commit-tree", stageCommitTree},
		{"epoch", stageEpoch},
		{"green-proof", t8Slots.GreenProof},
		{"first-push", stageFirstPush},
		{"finish-wiring", stageFinishWiring},
		{"welcome", stageFinalizeWelcome},
		{"complete-publication", stageCompletePublication},
	}
}

// initRun is the pipeline's shared state.
type initRun struct {
	opts   Options
	logger *slog.Logger
	knobs  config.Knobs
	branch string
	repo   config.Config // the parsed just-written config — converge's operand
	rep    *Report

	// j is the write-journal every apply-phase mutation routes through (plan
	// init-hardening/001 T4, DINV-3): created once mutation begins, consumed
	// by unwind on a stage failure.
	j *journal
	// headSHA / headBorn capture the target's HEAD BEFORE any mutation, but
	// only when this run did not create the repository (adm.gitInit false —
	// when it did, the whole .git is removed wholesale on unwind and the
	// prior HEAD is moot). headBorn false means HEAD was unborn (no commit
	// existed yet); restoreGitRef reads these to undo this run's own commits
	// without disturbing a repository init did not create.
	headSHA  string
	headBorn bool

	// seeded is the operand-relative (slash-form) path set the tail stages
	// wrote and the tree commit must stage — the four skeletons + AGENTS.md +
	// the registries (stageSeed) and the installed hook variants (stageHooks).
	// stageCommitTree folds it into its explicit-path staging (plan T8).
	seeded         []string
	wiringDeferred []wire.Op
	published      bool

	// welcomeTitle / welcomeBody are the rendered welcome-item content stageSeed
	// prepares for the Wiring (github) and Welcome (carina) stages. In github
	// mode the wire welcome-item op creates the item from these; in carina mode
	// the Welcome stage writes them into the local store. The Welcome stage
	// dispatches on the tracker knob, so no double-create is possible.
	welcomeTitle string
	welcomeBody  string
}

// payloadTemplatesDir is the operand's converged templates directory — where
// converge materialized the shipped instantiables (.pyxis/templates by
// default, the config's captured templates_dir). Seed reads the skeletons and
// the operand manual from here (they are engine-verbatim payload content the
// converge stage already wrote), never from the source repo.
func (run *initRun) payloadTemplatesDir() string {
	dir := run.repo.TemplatesDir
	if dir == "" {
		dir = ".pyxis/templates"
	}
	return filepath.Join(run.opts.Target, filepath.FromSlash(dir))
}

// Init runs the install pipeline's front half at opts.Target (installer-v1
// §3.5; prd:installer FR-3/FR-4/FR-7/UB-11/UB-10). Refusals happen before
// any write: target admission (UB-11, content-based — internal/preflight),
// knob resolution, then the identity probe (UB-10) — only then does a real
// run mutate (git init where no repository exists, the config + .gitignore
// writes, converge, the two commits). Preview performs none of it and
// reports the full would-do set.
func Init(ctx context.Context, opts Options) (Report, error) {
	start := now()
	rep := Report{RunID: newRunID(), Preview: opts.Preview, ConfigPath: configRel}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if err := validateOptions(&opts); err != nil {
		return rep, err
	}
	if err := prepareIntegrations(&opts); err != nil {
		return rep, err
	}
	if err := ctx.Err(); err != nil {
		return rep, fmt.Errorf("init: %w", err)
	}
	if pending, err := readPendingPublication(ctx, opts.Target); err != nil {
		return rep, err
	} else if pending != nil {
		return resumePublication(ctx, opts, rep, logger, *pending)
	}
	if !opts.Preview {
		terminalPrintf(opts.Stdout, "pyxis init — setting up this directory on pyxis %s\n", tag(opts.Pin.Version))
	}

	// 1. Target admission (read-only; UB-11 refusals fire here — EVT-2) plus
	// every other front-loaded check (toolchain floors, cold-cache
	// artifact-source reachability — plan init-hardening/001 T2): admitTarget
	// runs the WHOLE preflight registry, so a miss on any of them refuses
	// here too, before knob resolution, identity, or any write. Their
	// combined pass fires EVT-1 below.
	checksStart := now()
	adm, err := admitTarget(ctx, opts)
	if err != nil {
		emitInitRefused(ctx, logger, err)
		return rep, err
	}
	rep.WouldGitInit = opts.Preview && adm.gitInit
	rep.Branch = adm.branch
	emitInitChecksPassed(ctx, logger, rep.RunID, adm.checks, now().Sub(checksStart))
	if !opts.Preview {
		terminalPrintf(opts.Stdout, "preflight: all checks passed ✓\n")
	}

	// 2. Knobs — flags/answers promptless (FR-4) or interactive (FR-3).
	var pr *prompter
	if !opts.AnswersProvided && !opts.AcceptDefaults && opts.Stdin != nil {
		pr = newPrompter(ctx, opts.Stdin, opts.Stdout)
	}
	knobs, err := resolveKnobs(ctx, &opts, pr)
	if err != nil {
		return rep, err
	}
	rep.Knobs = knobs

	// 3. Identity — probed BEFORE any mutation (git init included), so a
	// refusal leaves no half state (UB-10).
	id, err := resolveIdentity(ctx, opts.Target, opts.Answers, pr)
	if err != nil {
		return rep, err
	}
	rep.Identity = id
	if err := resolvePublicationDestination(ctx, &opts, knobs, pr); err != nil {
		return rep, err
	}
	if knobs.Argus {
		if err := probeAuthoritySeed(ctx, opts.Target); err != nil {
			emitInitRefused(ctx, logger, err)
			return rep, err
		}
	}

	if opts.Preview {
		return previewRun(ctx, opts, rep, adm, knobs, logger)
	}

	// 3.5. Knob-dependent checks — GitHub-mode auth/coordinate/existence,
	// best-effort scope advisories, cross-knob coherence (design
	// init-hardening-v1 §3.1 checks 3/4/6; plan init-hardening/001 T3):
	// read-only, evaluated now that the knobs are resolved but before the
	// first write (DINV-1). T2's toolchain/artifact-source checks (§3.1
	// checks 2/5) land in the same registry beside these; T9 wires preview
	// onto this leg.
	checkRep, err := runKnobChecks(ctx, opts, knobs)
	rep.Checks = checkRep
	if err != nil {
		emitInitRefused(ctx, logger, err)
		return rep, err
	}

	// 4. Mutations begin: repository, identity halves, .gitignore, config —
	// every write from here on routes through run.j, the write-journal
	// rollback's primitive (plan init-hardening/001 T4, DINV-3), so a
	// failure before publication unwinds in reverse. A possibly published
	// installation is retained for exact-head resumption.
	run := &initRun{opts: opts, logger: logger, knobs: knobs, branch: adm.branch, rep: &rep, j: newJournal(opts.Target)}
	if !adm.gitInit {
		// Captured only when init did not create the repository: the
		// pre-run HEAD an unwind restores instead of deleting .git wholesale.
		run.headSHA, run.headBorn = headState(ctx, opts.Target)
	}
	if adm.gitInit {
		// A gitInitMain failure bypasses failAndUnwind deliberately: the
		// journal is still empty (nothing to remove) and — unlike every
		// later failure — .git may not exist at all here, so unwind's
		// "restore a pre-existing repository's ref" leg (which assumes one
		// does) would be the wrong branch to take. Nothing was journaled, so
		// nothing needs restoring.
		if err := gitInitMain(ctx, opts.Target, adm.branch); err != nil {
			return rep, fmt.Errorf("init: %w", err)
		}
		run.j.recordGitInit()
		rep.GitInited = true
	}
	if err := writeLocalIdentity(ctx, opts.Target, id, run.j); err != nil {
		return run.failAndUnwind(ctx, "identity", err)
	}
	if err := run.j.writeFile(gitignoreRel, []byte(operandGitignore), 0o644); err != nil {
		return run.failAndUnwind(ctx, "gitignore", err)
	}
	rep.GitIgnoreWritten = true
	cfgBytes := renderConfig(opts.Pin, opts.ArtifactSource, knobs, adm.branch, nil)
	cfgBytes, err = appendIntegrations(cfgBytes, opts.Integrations)
	if err != nil {
		return run.failAndUnwind(ctx, "config", err)
	}
	repo, err := parseRendered(cfgBytes)
	if err != nil {
		return run.failAndUnwind(ctx, "config", err)
	}
	if err := run.j.writeFile(configRel, cfgBytes, 0o644); err != nil {
		return run.failAndUnwind(ctx, "config", err)
	}
	run.repo = repo

	// 5. The pipeline: converge, T8's slots, the two commits, the epoch.
	for _, st := range pipelineStages() {
		if err := st.run(ctx, run); err != nil {
			return run.failAndUnwind(ctx, st.name, err)
		}
		rep.Stages = append(rep.Stages, st.name)
	}

	// 6. EVT-1 — only here, after the WHOLE pipeline succeeded. The
	// human landing is the welcome surface the pipeline already printed;
	// no second epilogue.
	emitInstallCompleted(ctx, logger, rep, opts, knobs, now().Sub(start))
	return rep, nil
}

// resolveWiringSlug is the ONE wiring-coordinate derivation preview and the
// apply-time wiring stage share (review:impl #809 FND-3): the target's own
// remote first (containment-guarded, preflight.RemoteSlug), else the
// supplied coordinate — grammar-gated, so an unvalidated string can never
// become an operation slug (FND-1's sink rule, applied to the table too).
func resolveWiringSlug(ctx context.Context, target, remote, coordinate string) string {
	if s := originSlug(ctx, target, remote); s != "" {
		return s
	}
	if c := strings.TrimSpace(coordinate); c != "" && wire.ValidCoordinate(c) {
		return c
	}
	return ""
}

// previewRun is the FR-24 leg (plan init-hardening/001 T9, formerly FR-19):
// the full would-do set, zero performance — no git init, no writes, no
// fetches (converge's own preview contract, consumed as-is). It runs the
// SAME two seams a real run's later stages use — runKnobChecks (design
// §3.1 checks 3/4/6: GitHub-mode, scope prediction, cross-knob coherence)
// and wire.Table (the declared wiring operation set stageWiringGitHub
// would walk) — so preview reports every check outcome and the exact
// operation table for the resolved knobs and coordinate, cold cache
// included, rather than staying silent about the wiring leg. A refusing
// check does not stop preview: FR-24 wants the whole picture in one pass,
// and preview performs nothing regardless of what the checks find; only a
// check's own infra failure (never a refusal — errors.As finds no
// *RefusalError) aborts early, the same distinction emitInitRefused draws
// for a real run.
func previewRun(ctx context.Context, opts Options, rep Report, adm admission, knobs config.Knobs, logger *slog.Logger) (Report, error) {
	checkRep, cerr := runKnobChecks(ctx, opts, knobs)
	rep.Checks = checkRep
	if cerr != nil {
		var refusal *preflight.RefusalError
		if !errors.As(cerr, &refusal) {
			return rep, cerr
		}
	}

	cfgBytes := renderConfig(opts.Pin, opts.ArtifactSource, knobs, adm.branch, nil)
	cfgBytes, err := appendIntegrations(cfgBytes, opts.Integrations)
	if err != nil {
		return rep, err
	}
	repo, err := parseRendered(cfgBytes)
	if err != nil {
		return rep, err
	}
	rep.ConfigBytes = cfgBytes
	crep, err := converge.Run(ctx, resolve.Config{Root: opts.Target, Repo: repo, Logger: logger, Fetcher: opts.Fetcher, Integrations: &opts.Integrations},
		converge.Options{Preview: true, CredentialExecutable: opts.CredentialExecutable, AuthoritySeedPending: knobs.Argus})
	if err != nil {
		return rep, fmt.Errorf("init: preview converge: %w", err)
	}
	rep.Converge = crep
	if knobs.Argus {
		rep.WouldSeed = append(rep.WouldSeed, SeededDoc{Path: payload.AuthorityContractTarget, Source: "template:" + filepath.Base(payload.AuthorityContractTemplate)})
	}

	// The declared wiring operation table (design §3.6/§3.7): Table is a
	// pure derivation with no IO (internal/wire's own contract), so it is
	// safe to compute even when the GitHub-mode check above refused. The
	// coordinate resolves exactly as the real Wiring stage resolves it
	// (originSlug, tail.go — a package-local helper this preview leg
	// shares rather than reimplements), so preview and a real run against
	// the same fixture agree on the declared set.
	slug := resolveWiringSlug(ctx, opts.Target, repo.Remote, opts.Answers.Coordinate)
	rep.WiringOps, err = wire.TableForIntegrations(wire.Config{
		Repo: repo,
		Slug: slug,
		// The onboarding standing set rides the preview table too (#947):
		// FR-24 enumerates EVERY external operation, and the real run
		// declares this row — preview and the walk must agree on the set.
		OnboardingItems: OnboardingWorkItems(repo),
	}, opts.Integrations)
	if err != nil {
		return rep, err
	}
	destination := opts.Answers.RemoteURL
	if current, err := gitOutput(ctx, opts.Target, "remote", "get-url", "--push", repo.Remote); err == nil {
		destination = strings.TrimSpace(current)
	} else if destination == "" && opts.Integrations.Bindings.CodeHost != (integrations.Binding{}) {
		host, err := opts.Integrations.CodeHost(opts.Answers.Coordinate)
		if err != nil {
			return rep, err
		}
		destination = host.BaseURL + "/" + host.Repository + ".git"
	} else if destination == "" {
		destination = wire.RemoteURL(slug)
	}
	rep.Publication = PublicationResult{Remote: repo.Remote, Branch: repo.Branch, Destination: destination, WouldPush: true}

	for _, st := range pipelineStages() {
		rep.Stages = append(rep.Stages, st.name)
	}
	return rep, nil
}

// stageConverge materializes the payload per ownership class (§3.5 step 3)
// through the T5 engine — pre-warm and the cache janitor ride it. A
// converge error (including per-file refusals) fails the pipeline before
// any commit: init never commits a tree converge refused (fail closed).
//
// Converge writes through its own atomic primitive, never run.j — so its
// contribution is INGESTED from the returned Report immediately on return,
// before the pipeline proceeds (design init-hardening-v1 §3.6, DINV-3):
// unconditionally, even when err is non-nil, since converge's per-file
// atomicity means every path in Repaired/Regenerated is real bytes already
// on disk that a later stage's failure must still unwind.
func stageConverge(ctx context.Context, run *initRun) error {
	terminalPrintf(run.opts.Stdout, "setting up the factory tree (a first run fetches the pinned toolset — this can take a minute)…\n")
	crep, err := converge.Run(ctx,
		resolve.Config{Root: run.opts.Target, Repo: run.repo, Logger: run.logger, Fetcher: run.opts.Fetcher, Integrations: &run.opts.Integrations},
		converge.Options{CredentialExecutable: run.opts.CredentialExecutable, AuthoritySeedPending: run.knobs.Argus})
	run.rep.Converge = crep
	run.j.recordCredentials(crep.Credentials)
	for _, a := range crep.Repaired {
		run.j.recordFile(a.Path)
	}
	for _, a := range crep.Regenerated {
		run.j.recordFile(a.Path)
	}
	if err == nil {
		terminalPrintf(run.opts.Stdout, "factory tree ready ✓ (%d files)\n", len(crep.Repaired)+len(crep.Regenerated))
	}
	return err
}

// stageCommitTree is commit #1: every init write, staged BY EXPLICIT PATH
// — the config, the operand .gitignore, exactly what converge reported
// writing, AND everything the T8 tail stages wrote before it in the pipeline
// (the seeded skeletons + AGENTS.md + registries + the installed hook
// variants — run.seeded, filled by stageSeed/stageHooks; plan installer/001
// T8). Never an add-all: the gitignored cache (.pyxis/bin) and any unrelated
// state stay out by construction. Paths are de-duplicated in first-seen order
// — the seeded design index overwrites converge's regenerated index at the
// same path, so both stages name it; git add tolerates a repeated path, but
// deduping keeps the commit's staged set honest and the report clean.
func stageCommitTree(ctx context.Context, run *initRun) error {
	paths := dedupPaths(collectCommitPaths(run))
	sha, err := gitCommitPaths(ctx, run.opts.Target, paths, commitTreeSubject)
	if err != nil {
		return err
	}
	run.rep.Commits = append(run.rep.Commits, Commit{SHA: sha, Subject: commitTreeSubject})
	return nil
}

// collectCommitPaths gathers every operand-relative path the pre-commit
// pipeline stages wrote, in a stable order: the two init-owned writes, then
// converge's repaired/regenerated set, then the T8 seeded set (skeletons,
// manual, registries, hooks). The order is deterministic so the staged set is
// reproducible.
func collectCommitPaths(run *initRun) []string {
	paths := []string{configRel, gitignoreRel}
	for _, a := range run.rep.Converge.Repaired {
		paths = append(paths, a.Path)
	}
	for _, a := range run.rep.Converge.Regenerated {
		paths = append(paths, a.Path)
	}
	paths = append(paths, run.seeded...)
	return paths
}

// dedupPaths returns paths with duplicates removed, preserving first-seen
// order (the seeded index and converge's regenerated index share a path).
func dedupPaths(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0:0]
	for _, p := range in {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// stageEpoch is §3.5 step 8: rewrite the config with the epoch block —
// installed system version, the epoch commit (commit #1), the manifest
// hash — then commit #2 carrying exactly that one change.
func stageEpoch(ctx context.Context, run *initRun) error {
	if len(run.rep.Commits) == 0 {
		return errors.New("epoch write before the tree commit — pipeline order violated (programmer error)")
	}
	ep := &config.Epoch{
		SystemVersion:  run.opts.Pin.Version,
		Commit:         run.rep.Commits[0].SHA,
		ManifestSHA256: run.opts.Pin.ManifestSHA256,
	}
	b := renderConfig(run.opts.Pin, run.opts.ArtifactSource, run.knobs, run.branch, ep)
	b, err := appendIntegrations(b, run.opts.Integrations)
	if err != nil {
		return err
	}
	repo, err := parseRendered(b)
	if err != nil {
		return err
	}
	if err := run.j.writeFile(configRel, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configRel, err)
	}
	run.repo = repo
	sha, err := gitCommitPaths(ctx, run.opts.Target, []string{configRel}, commitEpochSubject)
	if err != nil {
		return err
	}
	run.rep.Commits = append(run.rep.Commits, Commit{SHA: sha, Subject: commitEpochSubject})
	run.rep.EpochWritten = true
	return nil
}

// emitInstallCompleted logs EVT-1 (prd:installer §5.7) with the contracted
// fields plus the run id (§3.8 correlation). Local logger only (DINV-7).
func emitInstallCompleted(ctx context.Context, logger *slog.Logger, rep Report, opts Options, knobs config.Knobs, took time.Duration) {
	logger.LogAttrs(ctx, slog.LevelInfo, "install_completed",
		slog.String("run_id", rep.RunID),
		slog.String("version", opts.Pin.Version),
		slog.String("tracker", knobs.Tracker),
		slog.String("ci_mode", knobs.CIMode),
		slog.Any("cli_surfaces", knobs.CLISurfaces),
		slog.Bool("argus_enabled", knobs.Argus),
		slog.Int64("duration_ms", took.Milliseconds()),
		slog.String("platform", resolve.Platform()),
	)
}

// emitInitChecksPassed logs EVT-1 (prd:init-hardening §5.7) once every
// front-loaded check in the preflight registry has passed for this run —
// admission, the toolchain floors, and (on a cold cache) artifact-source
// reachability today (plan init-hardening/001 T2; T3 grows the set behind
// knob-gated applicability, never reshaping this call). checks names every
// evaluated check in registry order (EVT-1's "checks" property); took is the
// registry's own wall-clock cost, not the whole init run's (EVT-1's
// "duration_ms"). Fires exactly once per successful pass — real run or
// preview alike, since both share this one admitTarget call and DINV-1's
// ordering makes no distinction between them. Local logger only (DINV-7).
func emitInitChecksPassed(ctx context.Context, logger *slog.Logger, runID string, checks []string, took time.Duration) {
	logger.LogAttrs(ctx, slog.LevelInfo, "init_checks_passed",
		slog.String("run_id", runID),
		slog.Any("checks", checks),
		slog.Int64("duration_ms", took.Milliseconds()),
	)
}

// emitInitRolledBack logs EVT-3 (prd:init-hardening EVT-3) with exactly its
// contracted fields — failed_stage, restored — plus the run id (§3.8
// correlation). The full account (removed paths, foreign survivors, remote
// cleanup) lives on the returned Report.Rollback, not the event.
func emitInitRolledBack(ctx context.Context, logger *slog.Logger, runID string, rb RollbackReport) {
	logger.LogAttrs(ctx, slog.LevelWarn, "init_rolled_back",
		slog.String("run_id", runID),
		slog.String("failed_stage", rb.FailedStage),
		slog.Bool("restored", rb.Restored),
	)
}

// emitInitRefused logs EVT-2 (prd:init-hardening §5.7) when a preflight
// check refuses before any write: the refusing check's id and whether its
// outcome carried a remedy/route — never their text, which stays in the
// returned error for the caller to print (EVT-2's remedy_present/
// route_present are presence flags, not the prose). Local logger only
// (DINV-7). A non-refusal error (a probe's own infra failure) is not EVT-2
// and emits nothing here — errors.As finds no *RefusalError to report.
func emitInitRefused(ctx context.Context, logger *slog.Logger, err error) {
	var refusal *preflight.RefusalError
	if !errors.As(err, &refusal) {
		return
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "init_refused",
		slog.String("check", refusal.Outcome.Check),
		slog.Bool("remedy_present", refusal.Outcome.Remedy != ""),
		slog.Bool("route_present", refusal.Outcome.Route != ""),
	)
}

// validateOptions normalizes and refuses the operand: a present target
// directory and the full pin pair (DINV-2) are preconditions of everything
// else — including preview, whose report would otherwise be fiction.
func validateOptions(opts *Options) error {
	if strings.TrimSpace(opts.Target) == "" {
		return errors.New("init: Options.Target is empty (the target directory is required)")
	}
	st, err := os.Stat(opts.Target)
	switch {
	case err != nil:
		return fmt.Errorf("init: target %s: %w", opts.Target, err)
	case !st.IsDir():
		return fmt.Errorf("init: target %s is not a directory", opts.Target)
	}
	if strings.TrimSpace(opts.Pin.Version) == "" || strings.TrimSpace(opts.Pin.ManifestSHA256) == "" {
		return errors.New("init: the system pin pair is required — pass --system-version and --manifest-sha256 (the bootstrap invocation carries both; see the install instructions) (DINV-2)")
	}
	opts.Pin.Version = tag(opts.Pin.Version)
	opts.Pin.ManifestSHA256 = strings.ToLower(strings.TrimSpace(opts.Pin.ManifestSHA256))
	if !isHex64(opts.Pin.ManifestSHA256) {
		return fmt.Errorf("init: --manifest-sha256 %q is not a 64-char sha256 hex (DINV-2)", opts.Pin.ManifestSHA256)
	}
	opts.ArtifactSource = strings.TrimSpace(opts.ArtifactSource)
	return nil
}

// parseRendered closed-parses config bytes this package just rendered. A
// defect here is a render↔schema drift — a bug, named as one — and doubles
// as the production-path round-trip guard: every config init writes has
// already re-parsed clean under internal/docgate/config.
func parseRendered(b []byte) (config.Config, error) {
	repo, defects := config.Parse(b)
	if len(defects) > 0 {
		msgs := make([]string, len(defects))
		for i, d := range defects {
			msgs[i] = d.String()
		}
		return config.Config{}, fmt.Errorf("init: the rendered config does not closed-parse — renderConfig and the config schema drifted (a bug in this build): %s", strings.Join(msgs, "; "))
	}
	return repo, nil
}

// tag normalizes a version to release-tag form (exactly one leading "v") —
// the same spelling resolve pins and artifact names use.
func tag(version string) string {
	v := strings.TrimSpace(version)
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// isHex64 reports whether v is a 64-char lowercase sha256 hex string.
func isHex64(v string) bool {
	if len(v) != 64 {
		return false
	}
	for _, c := range v {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// newRunID mints the run correlation id (installer-v1 §3.8) — the same
// 8-byte-hex shape converge and the wiring audit use.
func newRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
