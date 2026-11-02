// The init pipeline's tail (installer-v1 §3.5 steps 4–7 + 9; plan
// installer/001 T8): the five stages installer.go registered as no-ops in
// t8Slots — seed, hooks, wiring, green proof, welcome — filled
// here without reshaping Init. registerT8Stages() (init() below) swaps the
// slots to the bodies in this file; installer.go still owns the ORDER
// (pipelineStages) and EVT-1's fire-at-the-end contract.
//
// # Seed (§3.5 step 4, FR-7/FR-15/FR-16/NG-1)
//
// Instantiate the four governance skeletons byte-verbatim into their operand
// doc paths, the operand manual knob-substituted into AGENTS.md, and the
// registries the gate needs on a fresh tree: the empty-but-valid incident
// registry and the empty design index (docgate's generated output). The
// welcome-item body is rendered and stashed for the Welcome stage. Rollups
// (docs/prd.md + docs/techspec.md) stay ABSENT — rule 3 keeps them out until
// the first capability ships (the green proof's lint-rollups vacuous pass
// depends on this). Argus off seeds NO authority contract (§3.5 step 4: "init
// instantiates no delegated-authority surface"). Argus on instantiates the
// shipped authority template once at floor; the Source repository's live
// policy is never shipped. The skill reads the knob at runtime. Every
// seeded path is recorded on run.seeded so stageCommitTree stages it.
//
// # Hooks (§3.5 step 6, FR-9)
//
// Install the INSTALLED-repo hook variants (.pyxis/dist/operand-hooks/**,
// carried finding (b): they call the `pyxis` BINARY, never `go run`) into the
// operand's .githooks and set core.hooksPath .githooks — the value doctor
// expects. No manual step; a gate-violating commit is then rejected.
//
// # Wiring (§3.5 step 7, FR-10)
//
// Dispatch on the tracker knob: carina → the local store root is prepared (the
// welcome item, created in the Welcome stage, materializes it); github → the
// wire operation table executed through the gh runner (EVT-4). Actions mode's
// workflow already rode the converge write set (engine-verbatim payload), so
// the wire push-workflow row records it converge-carried.
//
// # Green proof (§3.5 step 9, FR-8)
//
// Run the REAL docgate gate over the seeded + converged tree and require zero
// defects. Base-free (a fresh single-commit operand has no meaningful merge
// base against origin), so history-scoped checks are skipped exactly as the
// gate skips them at the desk. This is where carried finding (a) is proven:
// lint-rollups passes vacuously on the rollup-absent tree.
//
// # Welcome (§3.5 step 9, FR-15)
//
// Compute the epilogue's next-steps onto the Report (the cmd report tail
// prints them as the run's closing block, #937) and create the welcome work
// item: carina → written into the local store; github → already created by
// the Wiring stage's welcome-item op (Welcome does not double-create). EVT-5.
package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/wire"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// registerT8Stages swaps the pipeline's t8Slots no-ops for the real stage
// bodies. Done in init() so a plain build wires the tail; the EVT-1 silence
// test (installer_test) still overrides t8Slots.Seed via the export seam after
// this ran, and its restore returns the real Seed, not the no-op — the seam
// captures whatever is installed, so order with init() does not matter.
func init() { registerT8Stages() }

func registerT8Stages() {
	t8Slots.Seed = stageSeed
	t8Slots.Hooks = stageHooks
	t8Slots.Wiring = stageWiring
	t8Slots.GreenProof = stageGreenProof
	t8Slots.Welcome = stageWelcome
}

// ---- run-state seams the tail adds (test injection) -------------------------

// wireExecute is the wiring dispatch seam: production walks the wire operation
// table through the given Runner; the in-package tests inject a transcript fake
// by swapping this package var (SetWireExecuteForTest). The default composes
// internal/wire exactly over the runner the caller resolved — GHRunner{} in
// production, or an injected transcript fake via Options.WireRunner (the e2e
// matrix's out-of-process GitHub-mode combos). runner is never nil here:
// stageWiringGitHub defaults it before calling.
var wireExecute = func(ctx context.Context, ops []wire.Op, runner wire.Runner, logger *slog.Logger, runID string, prior *wire.Audit) (wire.Audit, error) {
	options := []wire.ExecOption{wire.WithLogger(logger), wire.WithRunID(runID)}
	if prior != nil {
		options = append(options, wire.WithPriorAudit(*prior))
	}
	return wire.Execute(ctx, ops, runner, options...)
}

// ---- Report additions (T8 stage outcomes) -----------------------------------

// SeededDoc is one governance/registry artifact the Seed stage instantiated:
// its operand path and where it came from (a shipped template, or an
// init-generated registry — record D-9).
type SeededDoc struct {
	Path   string // operand-relative, slash form
	Source string // "template:<name>" | "generated" | "substituted:<name>"
}

// HooksResult is the Hooks stage's outcome (§3.5 step 6, FR-9).
type HooksResult struct {
	Installed []string // the operand hook names installed into .githooks
	HooksPath string   // the core.hooksPath value set (.githooks)
}

// GreenProofResult is the green-proof stage's outcome (§3.5 step 9, FR-8).
type GreenProofResult struct {
	Green bool // the gate exited with zero defects on the seeded tree
}

// WelcomeResult is the Welcome stage's outcome (§3.5 step 9, FR-15).
type WelcomeResult struct {
	Emitted   bool     // the welcome path ran (item filed, steps computed)
	Title     string   // the welcome item's title
	ItemID    int      // the created carina item id (0 for github — wire created it)
	Ref       string   // where the guide lives: the issue URL (github) or item id + store path (carina); "" when unknown
	NextSteps []string // the terminal epilogue lines — printed by the cmd report tail (#937), last on screen
}

// t8Report is the tail's slice of the Report — carried on rep by the stages.
// It lives as fields ON Report (below, via the embedded T8 struct) so the cmd
// summary and tests read one struct. Defined here beside the stages that fill
// it to keep installer.go's seam untouched beyond the one path-collection edit.
type t8Report struct {
	Seeded     []SeededDoc
	Hooks      HooksResult
	Wiring     *wire.Audit // nil when no wiring ran (carina, or no gh ops)
	GreenProof GreenProofResult
	Welcome    WelcomeResult
	Onboarding *OnboardingResult // nil when no onboarding items were ensured
}

// OnboardingResult is the onboarding-items outcome (#947): what was filed
// into the tracker for the human to walk, and where.
type OnboardingResult struct {
	Detail string   // one human line — counts, or the hand-off reason
	Refs   []string // created-item references (issue URLs, or carina #ids)
}

// ---- Remote (design init-hardening-v1 §3.7; plan init-hardening/001 T5) ----
//
// Materializes the git remote at apply — a journaled config write, under the
// config-resolved name (run.repo.Remote; DINV-6, never a literal) — the beat
// the design's apply-order mermaid places right after "git init?" and before
// "converge": resolveCoordinate (helpers.go) already left the resolved
// coordinate on run.opts.Answers.Coordinate by the time apply begins,
// whether it arrived by flag/answers (T3) or this run's own interactive
// elicitation (T5); an already-configured remote never reaches that field
// (FR-6's "origin-present runs ask nothing" extends to "and materializes
// nothing"). Running this BEFORE stageWiringGitHub is what lets `create`
// bootstrap from a bare folder at all: that stage's own slug derivation
// (originSlug) is unchanged — it simply now finds the remote this stage just
// wrote instead of finding nothing.

// stageRemote is the pipeline's first stage: it adds the git remote when a
// resolved coordinate exists and no remote already carries it. Re-checks
// fresh (AP-15) rather than trusting resolveCoordinate's own gate already
// ran, so a redundant promptless coordinate alongside an already-configured
// remote is a harmless no-op, never a "remote already exists" git error.
func stageRemote(ctx context.Context, run *initRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if run.opts.Integrations.SchemaVersion != "" {
		if run.opts.Integrations.Bindings.CodeHost != (integrations.Binding{}) || run.opts.Answers.RemoteURL == "" {
			return stageIntegrationRemote(ctx, run)
		}
	}
	if destination := run.opts.Answers.RemoteURL; destination != "" {
		if current, err := gitOutput(ctx, run.opts.Target, "remote", "get-url", run.repo.Remote); err == nil {
			if strings.TrimSpace(current) != destination {
				return fmt.Errorf("remote: --remote-url differs from configured %s", run.repo.Remote)
			}
			return nil
		}
		if _, err := gitOutput(ctx, run.opts.Target, "remote", "add", run.repo.Remote, destination); err != nil {
			return err
		}
		run.j.recordRemote(run.repo.Remote)
		return nil
	}
	slug := strings.TrimSpace(run.opts.Answers.Coordinate)
	if slug == "" {
		return nil // no coordinate arrived, or a configured remote already resolved it
	}
	// A coordinate alongside wiring-free knobs is inert — no gh-transport
	// operation will consume it. Named, never silent (AP-16); and it must
	// not reach git argv (review:impl #809 FND-1: this path used to skip
	// every validation gate).
	if !preflight.GHOpsImplied(run.repo.Knobs) {
		run.logger.LogAttrs(ctx, slog.LevelInfo, "init: coordinate ignored — the resolved knobs declare no gh-transport operation",
			slog.String("run_id", run.rep.RunID))
		return nil
	}
	// Sink validation (FND-1): whatever path delivered the coordinate, it
	// enters argv/URL construction only through the one grammar.
	if !wire.ValidCoordinate(slug) {
		return fmt.Errorf("remote: coordinate %q is not a valid owner/name — it reached the remote step unvalidated; use letters, digits, - _ . with one slash (prd:init-hardening ABUSE-1)", slug)
	}
	remote := run.repo.Remote
	// Existence, not parseability (FND-2): an existing remote of ANY URL —
	// GitHub-parseable or not — means `remote add` would fail, so it is
	// never attempted. A matching remote is the ensured end state (AP-22);
	// any other existing remote WINS over the supplied coordinate — the
	// resolution order's own rule ("a configured remote always wins",
	// Input.Coordinate's contract) — and the yield is named, never silent
	// (AP-16).
	if cur, err := gitOutput(ctx, run.opts.Target, "remote", "get-url", remote); err == nil {
		if s, ok := wire.ParseCoordinate(cur); ok && s == slug {
			return nil // already carries the coordinate: nothing to materialize
		}
		run.logger.LogAttrs(ctx, slog.LevelInfo, "init: supplied coordinate yields to the configured remote",
			slog.String("run_id", run.rep.RunID),
			slog.String("remote", remote),
			slog.String("remote_url", strings.TrimSpace(cur)),
			slog.String("coordinate", slug))
		return nil
	}
	if _, err := gitOutput(ctx, run.opts.Target, "remote", "add", remote, wire.RemoteURL(slug)); err != nil {
		return fmt.Errorf("remote: add %s %s: %w", remote, slug, err)
	}
	run.j.recordRemote(remote)
	run.logger.LogAttrs(ctx, slog.LevelInfo, "init: remote configured",
		slog.String("run_id", run.rep.RunID),
		slog.String("remote", remote),
		slog.String("coordinate", slug))
	return nil
}

// ---- 4. Seed ----------------------------------------------------------------

// stageSeed instantiates the governance skeletons + operand manual + the
// empty-but-valid registries into the operand tree (§3.5 step 4). It writes
// repo-owned content only; the tree commit (which runs AFTER this stage) stages
// exactly run.seeded. It renders the welcome-item body once and stashes it on
// run for the Welcome/Wiring stages.
func stageSeed(ctx context.Context, run *initRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := run.opts.Target
	tmplDir := run.payloadTemplatesDir()

	add := func(rel, source string) {
		run.rep.T8.Seeded = append(run.rep.T8.Seeded, SeededDoc{Path: rel, Source: source})
		run.seeded = append(run.seeded, rel)
	}

	// The two governance skeletons — byte-verbatim; their fill points are the
	// human author's (NG-1), so no substitution runs. The target of each is the
	// configured governing-doc path (config-driven-portability-v1 §3.2, FR-3).
	for tmpl, target := range payload.SkeletonTargets(run.repo) {
		b, err := os.ReadFile(filepath.Join(tmplDir, filepath.Base(tmpl)))
		if err != nil {
			return fmt.Errorf("seed: read skeleton %s: %w", tmpl, err)
		}
		if err := run.j.writeFile(target, b, 0o644); err != nil {
			return fmt.Errorf("seed: write %s: %w", target, err)
		}
		add(target, "template:"+filepath.Base(tmpl))
	}
	if run.knobs.Argus {
		if err := seedAuthorityContract(ctx, run); err != nil {
			return err
		}
		add(payload.AuthorityContractTarget, "template:"+filepath.Base(payload.AuthorityContractTemplate))
	}

	// The operand manual → AGENTS.md, knob-substituted (fail closed on an
	// unresolved variable — payload.Substitute).
	manualSrc, err := os.ReadFile(filepath.Join(tmplDir, filepath.Base(payload.OperandAgentsManual)))
	if err != nil {
		return fmt.Errorf("seed: read operand manual: %w", err)
	}
	manual, err := payload.Substitute(manualSrc, manualVars(run.knobs, run.branch))
	if err != nil {
		return fmt.Errorf("seed: substitute operand manual (AGENTS.md): %w", err)
	}
	if err := run.j.writeFile(payload.OperandAgentsManualTarget, manual, 0o644); err != nil {
		return fmt.Errorf("seed: write %s: %w", payload.OperandAgentsManualTarget, err)
	}
	add(payload.OperandAgentsManualTarget, "substituted:"+filepath.Base(payload.OperandAgentsManual))

	// The taxonomy directories the pipeline authors into — structure only
	// (NG-1). Created BEFORE the design index so its generator (which reads
	// docs/tech-design) has an existing, empty corpus to enumerate. The empty
	// PRD registry also needs a tracked marker: docgate enumerates it before
	// running any checks, including on a fresh clone. Other empty dirs exist so
	// the first `/prd-new` / `/tech-design-new` lands cleanly. Journaled
	// individually (run.j.mkdirAll) since a directory left genuinely empty (no
	// file ever lands in it during init) has no other unwind path back to it.
	for _, dir := range []string{run.repo.PRDs, run.repo.TechDesigns, run.repo.SolutionShapes, run.repo.Plans, run.repo.Brainstorms} {
		if err := run.j.mkdirAll(dir); err != nil {
			return fmt.Errorf("seed: create taxonomy dir %s: %w", dir, err)
		}
	}
	registryMarker := filepath.ToSlash(filepath.Join(run.repo.PRDs, ".gitkeep"))
	// Git's admission status omits ignored files. Exclusive creation preserves
	// any operator-owned marker, even when that file was hidden from admission.
	marker, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(registryMarker)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("seed: preserve empty PRD registry %s: %w", run.repo.PRDs, err)
	}
	run.j.recordFile(registryMarker)
	if err := marker.Close(); err != nil {
		return fmt.Errorf("seed: close empty PRD registry marker: %w", err)
	}
	// This preserves directory structure, so include it in the commit without
	// reporting it as an authored document in T8.Seeded.
	run.seeded = append(run.seeded, registryMarker)

	// The empty-but-valid registries (record D-9: init-created, never shipped
	// templates). The incident registry — the fresh shape lint-incidents admits;
	// the design index — docgate's generated output over the empty corpus.
	if err := run.j.writeFile(run.repo.Incidents, []byte(emptyIncidentsRegistry), 0o644); err != nil {
		return fmt.Errorf("seed: write incident registry %s: %w", run.repo.Incidents, err)
	}
	add(run.repo.Incidents, "generated")

	if err := seedDesignIndex(run.j, run.repo, add); err != nil {
		return err
	}
	// ReconcileHarnessSurfaces writes through its own atomic primitive, never
	// run.j (it is shared with converge, which does the same) — its
	// contribution is ingested from its returned actions immediately, even
	// when it errors, since every path it names is already real bytes on
	// disk that a later stage's failure must still unwind (DINV-3). The
	// report/seeded-set additions stay gated on success, matching the
	// pre-existing behavior exactly.
	actions, err := converge.ReconcileHarnessSurfaces(root, run.repo)
	for _, action := range actions {
		run.j.recordFile(action.Path)
	}
	if err != nil {
		return err
	}
	for _, action := range actions {
		add(action.Path, "generated:harness-surface")
	}

	// Render the welcome-item body once (Substitute fail-closed) for the Welcome
	// + Wiring stages. The template's guidance matter above the horizontal rule
	// is stripped — only the item body ships.
	return prepareWelcome(run)
}

func prepareWelcome(run *initRun) error {
	welcomeSrc, err := os.ReadFile(filepath.Join(run.payloadTemplatesDir(), filepath.Base(payload.WelcomeItem)))
	if err != nil {
		return fmt.Errorf("seed: read welcome item: %w", err)
	}
	body, err := payload.Substitute(welcomeItemBody(welcomeSrc), welcomeVars(run.knobs, run.repo))
	if err != nil {
		return fmt.Errorf("seed: substitute welcome item: %w", err)
	}
	run.welcomeTitle = welcomeItemTitle
	run.welcomeBody = string(body)
	return nil
}

// seedDesignIndex writes the empty-corpus design index (index.md + index.json)
// — docgate's generated output, byte-identical to what converge/verify-index
// expect on a fresh tree. converge may have already laid a stale packed index;
// this overwrites it with the canonical empty-corpus form, and the tree commit
// stages exactly these paths.
func seedDesignIndex(j *journal, cfg config.Config, add func(rel, source string)) error {
	md, js, defects := gate.GenIndexBytes(j.target, cfg)
	if len(defects) > 0 {
		return fmt.Errorf("seed: generate empty design index: %s", defectsLine(defects))
	}
	mdRel := cfg.TechDesigns + "/index.md"
	jsRel := cfg.TechDesigns + "/index.json"
	if err := j.writeFile(mdRel, md, 0o644); err != nil {
		return fmt.Errorf("seed: write %s: %w", mdRel, err)
	}
	add(mdRel, "generated")
	if err := j.writeFile(jsRel, js, 0o644); err != nil {
		return fmt.Errorf("seed: write %s: %w", jsRel, err)
	}
	add(jsRel, "generated")
	return nil
}

// ---- 6. Hooks ---------------------------------------------------------------

// operandHookNames are the three installed-repo hook variants shipped under
// .pyxis/dist/operand-hooks/ (carried finding (b)): they call the pyxis BINARY
// directly, so they work in a source-less operand. Installed into the operand's
// hooks directory by name and made executable, then core.hooksPath points at
// that directory.
var operandHookNames = []string{"pre-commit", "pre-push", "prepare-commit-msg"}

// operandHooksSrcDir is the payload home of the installed-repo hook variants.
const operandHooksSrcDir = ".pyxis/dist/operand-hooks"

// operandHooksPath is the operand's activated hooks directory — the value
// core.hooksPath is set to and doctor expects (converge's hooksPathExpected).
const operandHooksPath = ".githooks"

// stageHooks installs the operand hook variants into .githooks and sets
// core.hooksPath (§3.5 step 6, FR-9). No manual step. The .githooks directory
// is repo-owned operand content (not manifest-listed), staged by the tree
// commit that follows.
func stageHooks(ctx context.Context, run *initRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := run.opts.Target
	srcDir := filepath.Join(root, filepath.FromSlash(operandHooksSrcDir))
	var installed []string
	for _, name := range operandHookNames {
		b, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			return fmt.Errorf("hooks: read operand hook variant %s: %w", name, err)
		}
		rel := operandHooksPath + "/" + name
		if err := run.j.writeFile(rel, b, 0o755); err != nil {
			return fmt.Errorf("hooks: install %s: %w", name, err)
		}
		installed = append(installed, name)
		run.seeded = append(run.seeded, rel)
	}
	if _, err := gitOutput(ctx, root, "config", "core.hooksPath", operandHooksPath); err != nil {
		return fmt.Errorf("hooks: set core.hooksPath: %w", err)
	}
	run.j.recordConfig("core.hooksPath")
	run.rep.T8.Hooks = HooksResult{Installed: installed, HooksPath: operandHooksPath}
	return nil
}

// ---- 7. Tracker + CI wiring -------------------------------------------------

// stageWiring dispatches on the tracker knob (§3.5 step 7, FR-10). carina — the
// local store root is prepared (created lazily; the welcome item materializes
// it). github — the wire operation table is executed through the gh runner
// (EVT-4). The workflow, when ci_mode wires it, already rode the converge write
// set; the wire push-workflow row records it converge-carried.
func stageWiring(ctx context.Context, run *initRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if run.opts.Integrations.SchemaVersion != "" {
		return stageWiringGitHub(ctx, run)
	}
	switch run.knobs.Tracker {
	case "carina":
		// The localfs store is created on first write (MkdirAll in Create); the
		// Welcome stage writes the welcome item into it. Nothing to wire here
		// beyond ensuring the store root's parent exists — Seed already made the
		// taxonomy dirs; the store root itself is created by the adapter.
		return nil
	case "github":
		return stageWiringGitHub(ctx, run)
	default:
		// A tracker outside the closed enum cannot reach here (resolveKnobs
		// validated it), but fail closed rather than silently skip.
		return fmt.Errorf("wiring: unsupported tracker %q (github | carina)", run.knobs.Tracker)
	}
}

// stageWiringGitHub builds the wire operation table from the captured config +
// the rendered welcome content and executes it. The repository coordinate
// (owner/name) is discovered from the git `origin` remote; when absent AND any
// gh op is declared, wire refuses by name (its Slug pre-action guard) — so the
// remedy travels through wire's own error, not a silent skip.
func stageWiringGitHub(ctx context.Context, run *initRun) error {
	slug := resolveWiringSlug(ctx, run.opts.Target, run.repo.Remote, run.opts.Answers.Coordinate)
	ops, err := wire.TableForIntegrations(wire.Config{
		Repo:            run.repo,
		Slug:            slug,
		WelcomeTitle:    run.welcomeTitle,
		WelcomeBody:     run.welcomeBody,
		OnboardingItems: OnboardingWorkItems(run.repo),
	}, run.opts.Integrations)
	if err != nil {
		return err
	}
	// A new branch must exist before its protection is installed, and hosted
	// welcome work should describe a published installation. Keep the declared
	// operation order, executing that suffix after publication.
	for i, op := range ops {
		if op.Name == wire.OpSetProtection || op.Name == wire.OpWelcomeItem || op.Name == wire.OpOnboardingItems {
			run.wiringDeferred = ops[i:]
			ops = ops[:i]
			break
		}
	}
	// The wiring transport: the injected runner when provided (hermetic e2e
	// GitHub-mode combos, plan installer/001 T14), else the production gh CLI.
	var runner wire.Runner = wire.GHRunner{}
	if run.opts.WireRunner != nil {
		runner = run.opts.WireRunner
	}
	audit, err := executeWiring(ctx, run, ops, runner, nil)
	run.rep.T8.Wiring = &audit
	for i := range audit.Operations {
		if r := &audit.Operations[i]; r.Name == wire.OpOnboardingItems {
			run.rep.T8.Onboarding = &OnboardingResult{
				Detail: r.Reason,
				Refs:   append([]string(nil), r.Refs...),
			}
		}
	}
	printWiringChecklist(run.opts.Stdout, slug, audit)
	// A repository this run created remotely is never deleted on rollback
	// (DINV-4) — journaled as a remote-op instead, so a later stage's
	// failure reports the manual cleanup command rather than silently
	// forgetting the account-facing mutation ever happened. Ingested
	// regardless of err: a create-repo earlier in the table can have landed
	// even when a later op aborts the walk.
	recordRemoteCreations(run.j, ops, audit)
	if err != nil {
		// No extra label: failAndUnwind already prefixes the stage name,
		// and the wire error names the failed ops with reasons (#898).
		return err
	}
	return nil
}

func stageFinishWiring(ctx context.Context, run *initRun) error {
	if len(run.wiringDeferred) == 0 {
		return nil
	}
	var runner wire.Runner = wire.GHRunner{}
	if run.opts.WireRunner != nil {
		runner = run.opts.WireRunner
	}
	prior := run.rep.T8.Wiring
	audit, err := executeWiring(ctx, run, run.wiringDeferred, runner, prior)
	if len(audit.Operations) > 0 {
		run.rep.T8.Wiring = &audit
	}
	for _, r := range audit.Operations {
		if r.Name == wire.OpOnboardingItems {
			run.rep.T8.Onboarding = &OnboardingResult{Detail: r.Reason, Refs: append([]string(nil), r.Refs...)}
		}
	}
	checklist := audit
	if prior != nil && len(audit.Operations) >= len(prior.Operations) {
		checklist.Operations = audit.Operations[len(prior.Operations):]
	}
	printWiringChecklist(run.opts.Stdout, resolveWiringSlug(ctx, run.opts.Target, run.repo.Remote, run.opts.Answers.Coordinate), checklist)
	return err
}

func executeWiring(ctx context.Context, run *initRun, ops []wire.Op, runner wire.Runner, prior *wire.Audit) (wire.Audit, error) {
	if run.opts.ForgejoClientFactory == nil {
		return wireExecute(ctx, ops, runner, run.logger, run.rep.RunID, prior)
	}
	options := []wire.ExecOption{wire.WithLogger(run.logger), wire.WithRunID(run.rep.RunID), wire.WithForgejoClientFactory(run.opts.ForgejoClientFactory)}
	if prior != nil {
		options = append(options, wire.WithPriorAudit(*prior))
	}
	return wire.Execute(ctx, ops, runner, options...)
}

// printWiringChecklist renders the wiring walk for the human running init
// (issue #898): one outcome line per operation — the reasons are already
// written for people — with hand-offs marked "waiting on you". The hand-off
// COMMANDS print once, in the final report (cmd's printInitTail), never
// here too; the forensic half stays in the audit/EVT-4 slog record.
func printWiringChecklist(w io.Writer, slug string, audit wire.Audit) {
	if len(audit.Operations) == 0 {
		return
	}
	provider := "GitHub"
	for _, r := range audit.Operations {
		if r.Transport == wire.TransportForgejo || r.Connection.Provider == "forgejo" {
			provider = "integrations"
			break
		}
	}
	_, _ = fmt.Fprintf(w, "wiring %s (%s):\n", provider, slug) // Checklist output is advisory; the returned audit remains authoritative.
	for _, r := range audit.Operations {
		plain := wiringPlain(r.Name, r.Outcome == wire.OutcomeApplied)
		if provider == "integrations" && r.Name == wire.OpPushWorkflow {
			plain = "CI workflow staged — it reaches your code host with your first push"
		}
		switch r.Outcome {
		case wire.OutcomeApplied:
			_, _ = fmt.Fprintf(w, "  ✓ %s\n", plain) // Preserve operation results even when the terminal closes.
		case wire.OutcomeSkipped:
			_, _ = fmt.Fprintf(w, "  – %s\n", plain) // Preserve operation results even when the terminal closes.
		case wire.OutcomeHandedOff:
			_, _ = fmt.Fprintf(w, "  → %s: waiting on you — %s\n", r.Name, r.Reason) // The handoff is also retained in the returned audit.
		case wire.OutcomeFailed:
			_, _ = fmt.Fprintf(w, "  ✗ %s: %s\n", r.Name, r.Reason) // The operation error still reaches the caller.
		}
	}
}

// wiringPlain renders an op's short human checklist row (#937). The full
// mechanism-and-citation reason stays in the audit and the journal — the
// terminal names the outcome, not the internals. Only the applied/skipped
// rows flatten this way: a handed-off or failed row keeps its full reason,
// because there the why IS the action.
func wiringPlain(op string, applied bool) string {
	switch op {
	case wire.OpCreateRepo:
		if applied {
			return "repository created"
		}
		return "repository already exists"
	case wire.OpEnsureLabels:
		if applied {
			return "routing labels ensured"
		}
		return "routing labels already in place"
	case wire.OpPushWorkflow:
		return "CI workflow staged — it reaches GitHub with your first push"
	case wire.OpSetProtection:
		if applied {
			return "branch protection set"
		}
		return "branch protection already set"
	case wire.OpWelcomeItem:
		if applied {
			return "welcome item filed"
		}
		return "welcome item already filed"
	case wire.OpOnboardingItems:
		if applied {
			return "onboarding items filed in the tracker"
		}
		return "onboarding items already in place"
	}
	return op
}

// recordRemoteCreations journals a remote-op entry for every declared
// create-repo op the wiring walk actually landed (wire.OutcomeApplied covers
// both a genuine create and the benign already-existed-by-create-time race —
// DINV-4 treats both conservatively as "this run may have created it").
// audit.Operations is a prefix of ops in table order (a transport-level abort
// stops the walk early), so it is indexed defensively.
func recordRemoteCreations(j *journal, ops []wire.Op, audit wire.Audit) {
	for i, res := range audit.Operations {
		if i >= len(ops) {
			break
		}
		if res.Name != wire.OpCreateRepo || res.Outcome != wire.OutcomeApplied {
			continue
		}
		if ops[i].Transport == wire.TransportForgejo {
			j.recordRemoteOp(fmt.Sprintf("curl --fail --request DELETE --header \"Authorization: token ${%s}\" '%s/api/v1/repos/%s'", ops[i].Connection.TokenEnv, ops[i].Connection.BaseURL, ops[i].Slug))
		} else {
			j.recordRemoteOp(fmt.Sprintf("gh repo delete %s --yes", ops[i].Slug))
		}
	}
}

// originSlug derives owner/name from the configured remote's URL, or "" when the
// remote is not set (a fresh bare-dir install has none until stageRemote — or
// the operator — adds one) or does not name a github.com repository. The
// remote resolves from the repo config (config.Remote; blank falls back
// to the config schema's default "origin", the ONE place that literal lives,
// DINV-1) so a repo whose remote is not "origin" is honored (config-driven-
// portability-v1 §3.2, FR-2). Parsing is wire.ParseCoordinate's — the ONE
// evaluator (AP-11) for both URL shapes gh/git use, shared with the
// preflight GitHub-mode check's own coordinate resolution (design
// init-hardening-v1 §3.7) rather than a second, driftable copy.
func originSlug(ctx context.Context, target, remote string) string {
	// One derivation, one home (DINV-2; review:impl #809 FND-4): the
	// containment-guarded read lives in the suite; this name survives as
	// the package-local spelling its callers already use.
	return preflight.RemoteSlug(ctx, target, remote)
}

// ---- 9. Green proof ---------------------------------------------------------

// stageGreenProof runs the REAL docgate gate over the seeded + converged tree
// and requires it green (§3.5 step 9, FR-8). Base-free: a fresh single-commit
// operand has no merge base against origin, so history-scoped checks are
// skipped exactly as the gate skips them (mergeBase ""). This is where carried
// finding (a) is proven — lint-rollups vacuous-passes on the rollup-absent
// tree. A red gate fails the pipeline, naming the defects (fail closed): init
// never reports green over a tree the gate rejects.
func stageGreenProof(ctx context.Context, run *initRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := run.opts.Target
	dirs, err := gate.LedgerDirs(root, run.repo)
	if err != nil {
		return fmt.Errorf("green-proof: enumerate ledger dirs: %w", err)
	}
	defects := gate.Gate(root, "", dirs, run.repo)
	if len(defects) > 0 {
		run.rep.T8.GreenProof = GreenProofResult{Green: false}
		return fmt.Errorf("green-proof: the gate is not green on the freshly seeded tree (%d defect(s)) — %s", len(defects), defectsLine(defects))
	}
	run.rep.T8.GreenProof = GreenProofResult{Green: true}
	run.logger.LogAttrs(ctx, slog.LevelInfo, "install_green_proof",
		slog.String("run_id", run.rep.RunID),
		slog.Bool("green", true))
	return nil
}

// ---- 9. Welcome -------------------------------------------------------------

// Local work items belong in the initial commit and first push. Hosted welcome
// work runs after publication and protection; only its final stage emits it.
func stageSeedLocalWelcome(ctx context.Context, run *initRun) error {
	if run.knobs.Tracker != "carina" {
		return nil
	}
	return t8Slots.Welcome(ctx, run)
}

func stageFinalizeWelcome(ctx context.Context, run *initRun) error {
	if run.rep.T8.Welcome.Emitted {
		return nil
	}
	return t8Slots.Welcome(ctx, run)
}

// stageWelcome computes the epilogue's next-steps onto the Report (the cmd
// report tail prints them last, #937) and creates the welcome work item where
// the tracker owns one that the Wiring stage did not already create (§3.5
// step 9, FR-15). carina → written into the local store; github → the Wiring
// stage's welcome-item op already created it (no double-create).
func stageWelcome(ctx context.Context, run *initRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	res := WelcomeResult{Emitted: true, Title: run.welcomeTitle}

	if run.knobs.Tracker == "carina" {
		id, err := createCarinaWelcome(ctx, run)
		if err != nil {
			return err
		}
		res.ItemID = id
		res.Ref = fmt.Sprintf("item #%d in %s", id, run.repo.WorkItems)
		onboarding, oerr := createCarinaOnboarding(ctx, run)
		if oerr != nil {
			return oerr
		}
		run.rep.T8.Onboarding = onboarding
	} else if a := run.rep.T8.Wiring; a != nil {
		// The wire welcome-item op collected the issue's URL — created this
		// run or found by the probe (#948).
		for _, op := range a.Operations {
			if op.Name == wire.OpWelcomeItem && len(op.Refs) > 0 {
				res.Ref = op.Refs[0]
			}
		}
	}

	// The steps are computed LAST — after both trackers' item creation —
	// so the guide link and the onboarding refs render into the epilogue;
	// printed by the caller's report tail (#937), the final block on
	// screen.
	res.NextSteps = welcomeNextSteps(run.knobs, run.repo, res.Ref, run.rep.T8.Onboarding)
	run.rep.T8.Welcome = res

	run.logger.LogAttrs(ctx, slog.LevelInfo, "welcome_emitted",
		slog.String("run_id", run.rep.RunID),
		slog.String("tracker", run.knobs.Tracker),
		slog.Int("carina_item", res.ItemID))
	return nil
}

// createCarinaWelcome writes the welcome work item into the operand's local
// carina store — the same localfs adapter cmd/carina roots at
// <root>/<paths.work_items> (so doctor's tracker check finds it). Returns the
// assigned item id.
func createCarinaWelcome(ctx context.Context, run *initRun) (int, error) {
	id, err := createCarinaItem(ctx, run, run.welcomeTitle, run.welcomeBody)
	if err != nil {
		return 0, fmt.Errorf("welcome: create the carina welcome item: %w", err)
	}
	return id, nil
}

// createCarinaItem writes one item through the local store and JOURNALS the
// item file (the #947 review's finding: the store writes through its own
// primitive, not the journal, so an unjournaled item would survive a
// mid-stage failure as stranded debris — blocking re-admission on a
// fresh-dir target, or duplicating on an adopted-empty re-run).
func createCarinaItem(ctx context.Context, run *initRun, title, body string) (int, error) {
	store := localfs.New(filepath.Join(run.opts.Target, filepath.FromSlash(run.repo.WorkItems)), nil)
	// Publication can succeed before a later welcome write fails. A resumed
	// install must reuse its welcome and onboarding items.
	items, defects := store.List(ctx, model.Query{})
	if len(defects) > 0 {
		return 0, errors.New(defectsLine(defects))
	}
	for _, item := range items {
		if item.Title == title && item.Body == body {
			return item.ID, nil
		}
	}
	item, defects := store.Create(ctx, model.CreateSpec{
		Type:  model.TypeTask,
		Title: title,
		Body:  body,
	})
	if len(defects) > 0 {
		return 0, errors.New(defectsLine(defects))
	}
	run.j.recordFile(path.Join(run.repo.WorkItems, "items", strconv.Itoa(item.ID)+".md"))
	run.seeded = append(run.seeded, path.Join(run.repo.WorkItems, "items", strconv.Itoa(item.ID)+".md"))
	return item.ID, nil
}

// createCarinaOnboarding files the onboarding work items into the local
// store (#947) — the same standing set the gh lane declares. No
// finish-the-wiring item here: the local standing set is committed before
// publication. createCarinaItem reuses exact existing items during resumption.
func createCarinaOnboarding(ctx context.Context, run *initRun) (*OnboardingResult, error) {
	items := OnboardingWorkItems(run.repo)
	res := &OnboardingResult{}
	for _, it := range items {
		id, err := createCarinaItem(ctx, run, it.Title, it.Body)
		if err != nil {
			return nil, fmt.Errorf("welcome: create onboarding item %q: %w", it.Title, err)
		}
		res.Refs = append(res.Refs, fmt.Sprintf("#%d", id))
	}
	res.Detail = fmt.Sprintf("filed %d onboarding item(s) in the local store", len(items))
	return res, nil
}

// ---- helpers ----------------------------------------------------------------

// manualVars maps the captured knobs onto the operand-manual substitution set
// (payload.Vars[OperandAgentsManual]): argus_enabled, branch, ci_mode, tracker.
func manualVars(k config.Knobs, branch string) map[string]string {
	return map[string]string{
		"tracker":       k.Tracker,
		"ci_mode":       k.CIMode,
		"branch":        branch,
		"argus_enabled": strconv.FormatBool(k.Argus),
	}
}

// welcomeVars maps the captured knobs + paths onto the welcome-item
// substitution set (payload.Vars[WelcomeItem]): gates_active, prds_path,
// tech_designs_path, tracker.
func welcomeVars(k config.Knobs, cfg config.Config) map[string]string {
	return map[string]string{
		"tracker":           k.Tracker,
		"prds_path":         cfg.PRDs,
		"tech_designs_path": cfg.TechDesigns,
		"gates_active":      gatesActiveSentence(k.CIMode),
	}
}

// gatesActiveLine names the docs gate's active reach — desk (git hooks)
// always, hosted CI only when ci_mode wires it (design init-hardening-v1
// §3.9, prd:init-hardening/FR-14) — in the terminal epilogue's terse
// register; the welcome-item body renders the same truth through
// gatesActiveSentence below, both from the one ci_mode knob so the two
// surfaces can never disagree about which gates are active. The code gate's
// pending status is knob-INDEPENDENT (a fresh install never carries a
// verification manifest) and is stated separately, alongside this line.
func gatesActiveLine(ciMode string) string {
	if ciMode == "local" {
		return "gates: docs gate live at your desk (git hooks) — hosted CI off (ci_mode=local)"
	}
	return fmt.Sprintf("gates: docs gate live at your desk (git hooks) + hosted CI (ci_mode=%s)", ciMode)
}

// gatesActiveSentence is the same truth in the welcome ITEM's prose register
// — the item body flows as sentences, the terminal epilogue as terse rows
// (#937); both derive from the one ci_mode knob so they can never disagree.
func gatesActiveSentence(ciMode string) string {
	if ciMode == "local" {
		return "Active gate: the docs gate at your desk (git hooks) — hosted CI is off (ci_mode=local)."
	}
	return fmt.Sprintf("Active gates: the docs gate at your desk (git hooks) and in hosted CI (ci_mode=%s).", ciMode)
}

// welcomeItemBody strips the welcome template's guidance matter — everything up
// to and including the first horizontal rule (a line that is exactly "---") —
// leaving only the item body that ships (the template's own contract: "files
// everything below the horizontal rule as a work item"). When no rule is
// present the whole document is the body (defensive).
func welcomeItemBody(doc []byte) []byte {
	lines := strings.Split(string(doc), "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "---" {
			rest := strings.Join(lines[i+1:], "\n")
			return []byte(strings.TrimLeft(rest, "\n"))
		}
	}
	return doc
}

// welcomeNextSteps is the TERMINAL epilogue (§3.5 step 9) — a handful of
// short lines (#937): the two truth lines T8 contracts (active gates, the
// pending code gate with its seed command), the two documents only the
// owner can write, the pipeline in one line, the guide LINKED rather than
// described (#948), the filed todo list, and the send-off. Derived from
// the captured knobs + paths so the paths quoted match the operand; the
// cmd report tail prints it as the run's closing block. The full guidance
// is the welcome work item's body, not this surface.
func welcomeNextSteps(k config.Knobs, cfg config.Config, guideRef string, onboarding *OnboardingResult) []string {
	steps := []string{
		gatesActiveLine(k.CIMode),
		codeGatePendingLine,
		"yours to author: " + cfg.ProductVision + " · " + cfg.ArchitecturePrinciples + " (`pyxis doctor` lists what is still a skeleton)",
		"first feature: /feature-brainstorm → /prd-new → /prd-review → /tech-design-brainstorm → /tech-design-new → /tech-design-review → /plan-new → /implement-now",
	}
	switch {
	case guideRef != "":
		steps = append(steps, "full guide: "+guideRef)
	case k.Tracker == "carina":
		steps = append(steps, "full guide: the welcome item in your local store ("+cfg.WorkItems+")")
	case k.Tracker == "forgejo":
		steps = append(steps, "full guide: the welcome issue in your Forgejo tracker")
	default:
		steps = append(steps, "full guide: the welcome issue in your GitHub tracker")
	}
	if onboarding != nil && len(onboarding.Refs) > 0 {
		// "in the tracker", not "filed": the refs include items an earlier
		// run already filed (AP-22 skips carry their refs too).
		steps = append(steps, fmt.Sprintf("your todo list: %d onboarding item(s) in the tracker (%s)",
			len(onboarding.Refs), strings.Join(shortRefs(onboarding.Refs), " · ")))
	}
	steps = append(steps, "your first feature is one /feature-brainstorm away ✨")
	return steps
}

// shortRefs compresses item references for one terminal line: an issue URL
// becomes its #number, a carina "#n" passes through, anything unrecognized
// stays verbatim.
func shortRefs(refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if i := strings.LastIndex(r, "/"); i >= 0 && i < len(r)-1 {
			if _, err := strconv.Atoi(r[i+1:]); err == nil {
				out = append(out, "#"+r[i+1:])
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// defectsLine renders a bounded, single-line summary of a defect list for a
// named refusal message.
func defectsLine(defects []schema.Defect) string {
	const limit = 5
	parts := make([]string, 0, limit+1)
	for i, d := range defects {
		if i == limit {
			parts = append(parts, fmt.Sprintf("(+%d more)", len(defects)-limit))
			break
		}
		parts = append(parts, d.String())
	}
	return strings.Join(parts, "; ")
}
