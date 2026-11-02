package converge

// Doctor is the read-only drift and health face of the converge library
// (installer-v1 §3.5 "pyxis doctor is the read-only face of the same
// library", §3.8; plan installer/001 T6; prd:installer/FR-14, UB-7, NFR-4).
//
// Zero mutations, structurally: doctor never writes the repo tree, never
// writes the cache, and never fetches. Where converge's real run answers a
// cache miss by fetch-verifying, doctor consumes resolve's read-only probes
// (CachedManifest) and planners (ReapPlan) instead — an absent cache is a
// NAMED advisory, never a fetch (FR-14's empty-filesystem-diff criterion).
//
// Source mode (the T5↔T12 tension, resolved here): converge.Run and
// converge.Drift refuse in source mode (ErrSourceMode — the source tree is
// authoritative, nothing converges against a fetched release), and that
// refusal is deliberately NOT weakened. Doctor still works in the source
// repo (T12's acceptance: "doctor reports source mode and zero deprecation
// notices post-flip") by branching AROUND the refusal: it reports mode
// "source", records the drift check as skipped-with-the-named-reason (there
// is nothing to diff — the tree is the payload's source), and still runs
// every environment, epoch, deprecation, cache, and skeleton check. In
// release mode it reads THE same drift predicate converge repairs from
// (scanTree — one classification, one place, AP-11), so doctor and converge
// can never disagree about what drift is.
//
// The check taxonomy (the Health row classes, in report order):
//
//	drift        — per-file divergence from the pinned manifest, by
//	               ownership class; "drift unknown" advisory when the
//	               manifest is not verifiably cached; "skipped" advisory in
//	               source mode; the hosted workflow is knob-conditional
//	               (ci_mode, FR-11, §3.8) — excluded means unlisted (never a
//	               drift row), but a repo carrying it anyway (a prior
//	               including ci_mode, since flipped) gets a named advisory
//	               with both remedies, independent of manifest availability
//	prerequisite — git binary present (UB-7); gh present only
//	               when the captured knobs declare any gh-transport wiring
//	               operation (the wire table is the single source of
//	               "GitHub mode", installer-v1 §3.6) — an absent gh in a
//	               non-GitHub config is deliberately NOT a finding
//	hooks        — core.hooksPath configured to the expected value (the
//	               payload's hook-activation convention, FR-9)
//	tracker      — carina: the work-item store root exists (existence only,
//	               exactly where cmd/carina roots its store); github: live
//	               reachability is NOT probed (doctor is network-free) —
//	               reported as a named advisory
//	code-host    — selected local adapter checks the configured bare origin
//	               and mainline without fetching; reports proposal capability
//	               and shared-account assurance limits
//	surfaces     — each configured CLI surface's pointer(s) exist and
//	               resolve into the system tree (.pyxis/, AP-25)
//	epoch        — the config's epoch block, or its absence (an advisory:
//	               init writes it once, §3.5 step 8)
//	deprecation  — the config dual-read window's notices, verbatim
//	               (Config.Deprecations(), installer-v1 §3.10)
//	cache        — off-version cache entries the janitor would remove
//	               (resolve.ReapPlan — the planner, read-only; §3.4 "doctor
//	               names any stale entry the janitor missed")
//	skeleton     — unauthored governance-doc skeletons (advisory per doc,
//	               prd:installer/FR-14; the templates themselves promise
//	               this report)
//	verification — recognized code marker files present with no
//	               verification manifest (an advisory naming the codegate
//	               seed verb and the hand-authoring fallback,
//	               prd:init-hardening/FR-15, design init-hardening-v1
//	               §3.9); a manifest already present, or no recognized
//	               code at all, reports nothing — the same ecosystem
//	               marker set adopt's stack detection reads is the one
//	               detection source (prd:implementation-verification/
//	               FR-20), never a second, hand-kept list
//
// Severity: "finding" reds the run (Healthy == zero findings); "advisory"
// informs and never reds. Every row names its cause and remediation (NFR-4).
//
// Observability: one EVT-3 `doctor_run` slog event per run — healthy, mode,
// the finding/advisory totals, the per-class row counts, the run id, and
// (loudly, §3.9) the artifact-source override when set. Local logger only;
// zero telemetry egress (DINV-7). No model inference anywhere (NFR-6).

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/githooks"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
)

// Severity grades one Health row: a finding reds the run, an advisory
// informs (unauthored skeletons, deprecation notices, skipped probes).
type Severity string

const (
	SeverityFinding  Severity = "finding"
	SeverityAdvisory Severity = "advisory"
)

// CheckClass is the Health taxonomy — the closed set of check families the
// report groups by.
type CheckClass string

const (
	CheckDrift        CheckClass = "drift"
	CheckPrerequisite CheckClass = "prerequisite"
	CheckHooks        CheckClass = "hooks"
	CheckTracker      CheckClass = "tracker"
	CheckCodeHost     CheckClass = "code-host"
	CheckSurfaces     CheckClass = "surfaces"
	CheckEpoch        CheckClass = "epoch"
	CheckDeprecation  CheckClass = "deprecation"
	CheckCache        CheckClass = "cache"
	CheckSkeleton     CheckClass = "skeleton"
	CheckVerification CheckClass = "verification"
)

// checkTaxonomy is the report (and event) order of the classes.
var checkTaxonomy = []CheckClass{
	CheckDrift, CheckPrerequisite, CheckHooks, CheckTracker, CheckCodeHost, CheckSurfaces,
	CheckEpoch, CheckDeprecation, CheckCache, CheckSkeleton, CheckVerification,
}

// HealthFinding is one named check result row.
type HealthFinding struct {
	Class    CheckClass
	Name     string // what the row is about: a path, a binary, a surface, a cache entry
	Detail   string // the named cause and remediation (NFR-4)
	Severity Severity
}

// Health is Doctor's report: the mode/pin/epoch summary plus every check
// row, grouped by taxonomy order (stable within a class). Healthy is true
// iff zero rows carry SeverityFinding — advisories never red the run.
type Health struct {
	RunID          string
	Mode           string // "release" | "source"
	SystemVersion  string // the config's system.version, verbatim
	ManifestPinned bool   // system.manifest_sha256 present in the config
	ArtifactSource string // the FR-22 override, surfaced loudly when set (§3.9)

	EpochPresent bool
	Epoch        config.Epoch

	Findings []HealthFinding
	Healthy  bool
	CodeHost *codehost.Readiness
}

// hooksPathExpected is the payload's hook-activation convention: the
// repo-root .githooks pointer (a symlink into .pyxis/hooks) that
// `git config core.hooksPath .githooks` activates (FR-9; the source repo's
// AGENTS.md one-time step, automated by init in installed repos).
const hooksPathExpected = ".githooks"

// SurfacePointer is one vendor pointer of a CLI surface: the link path and the
// exact relative target it must point at — a skill/script pointer into .pyxis/,
// or a CLI's manual pointer to the canonical AGENTS.md.
type SurfacePointer struct {
	Link   string // the vendor pointer path (e.g. ".claude/skills", "CLAUDE.md")
	Target string // the relative symlink target (e.g. "../.pyxis/skills", "AGENTS.md")
}

// surfaceConventions is the fixed catalog of vendor-pointer sets per known
// CLI-surface adapter (AP-25: vendor surfaces hold pointers into .pyxis/ or to
// the canonical AGENTS.md, never content — the source repo's live convention).
// Every target is a SYSTEM-tree-internal path (the neutral .pyxis/ namespace)
// or the operand-manual convention (AGENTS.md), never a target-facing config
// path (NG-1) — so the catalog itself carries no config-driven value; it is the
// per-adapter shape SurfacePointers filters by the configured knob. Claude Code
// additionally reads CLAUDE.md, so the claude surface points it at AGENTS.md.
var surfaceConventions = map[string][]SurfacePointer{
	"claude": {
		{Link: ".claude/skills", Target: "../.pyxis/generated/harnesses/claude/skills"},
		{Link: ".claude/scripts", Target: "../.pyxis/scripts"},
		{Link: "CLAUDE.md", Target: "AGENTS.md"},
	},
	"codex": {
		{Link: ".agents/skills", Target: "../.pyxis/generated/harnesses/neutral/skills"},
		{Link: ".codex/skills", Target: "../.pyxis/generated/harnesses/codex/skills"},
	},
	"cursor": {
		{Link: ".cursor/skills", Target: "../.pyxis/generated/harnesses/cursor/skills"},
	},
	"opencode": {
		{Link: ".agents/skills", Target: "../.pyxis/generated/harnesses/neutral/skills"},
		{Link: ".opencode/skills", Target: "../.pyxis/generated/harnesses/opencode/skills"},
	},
}

// SupportedSurfaces is every agent-CLI surface this build ships wiring for,
// sorted. It is derived from surfaceConventions rather than listed again,
// because a surface with no registered convention wires nothing: the catalog
// that HAS the pointers is the only honest answer to "what is supported".
func SupportedSurfaces() []string {
	out := make([]string, 0, len(surfaceConventions))
	for name := range surfaceConventions {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// SurfacePointers derives the vendor-pointer map for exactly the CLI surfaces
// the repo config enables (cfg.Knobs.CLISurfaces) — a PURE FUNCTION of the
// configured surface knob, recomputed at every use (converge / doctor / seed)
// with no cached or materialized derived state (config-driven-portability-v1
// §3.2, FR-3; DINV-1). The knob's value set is open (adapters grow); an enabled
// surface with no registered convention is absent from the result — a caller's
// `pointers, known := SurfacePointers(cfg)[s]` reads it as unknown and reports
// it a named advisory. On a default config (its knob unset) the result is empty;
// a surface named twice collapses to one entry.
func SurfacePointers(cfg config.Config) map[string][]SurfacePointer {
	out := make(map[string][]SurfacePointer, len(cfg.Knobs.CLISurfaces))
	for _, s := range cfg.Knobs.CLISurfaces {
		if pointers, known := surfaceConventions[s]; known {
			out[s] = pointers
		}
	}
	return out
}

// Doctor reports manifest drift by ownership class and environment health
// for the repository at cfg.Root, performing zero filesystem or remote
// mutations. It returns an error only for a dead context or an empty root;
// every checkable defect is a Health row, so a partially broken repo still
// yields the fullest possible report (doctor is the debugging surface,
// installer-v1 §3.8). See the file comment for the taxonomy and the
// source-mode posture.
func Doctor(ctx context.Context, cfg resolve.Config) (Health, error) {
	if err := ctx.Err(); err != nil {
		return Health{}, fmt.Errorf("doctor: %w", err)
	}
	if cfg.Root == "" {
		return Health{}, errors.New("doctor: Config.Root is empty (the operand repo root is required)")
	}
	source := cfg.Repo.System.Mode == "source"
	h := Health{
		RunID:          newRunID(),
		Mode:           "release",
		SystemVersion:  cfg.Repo.System.Version,
		ManifestPinned: strings.TrimSpace(cfg.Repo.System.ManifestSHA256) != "",
		ArtifactSource: strings.TrimSpace(cfg.Repo.System.ArtifactSource),
		Epoch:          cfg.Repo.Epoch,
	}
	if source {
		h.Mode = "source"
	}

	var c checks
	if err := c.drift(ctx, cfg, source); err != nil {
		return Health{}, err // only a dead context escalates out of a check
	}
	gitPresent := c.prerequisites(ctx, cfg)
	c.hooks(ctx, cfg, gitPresent)
	c.tracker(cfg)
	var err error
	h.CodeHost, err = c.codeHost(ctx, cfg)
	if err != nil {
		return Health{}, err
	}
	c.surfaces(cfg)
	h.EpochPresent = c.epoch(cfg)
	c.deprecations(cfg)
	c.cache(cfg)
	c.skeletons(cfg)
	c.verification(cfg)

	rank := make(map[CheckClass]int, len(checkTaxonomy))
	for i, cl := range checkTaxonomy {
		rank[cl] = i
	}
	slices.SortStableFunc(c.rows, func(a, b HealthFinding) int { return rank[a.Class] - rank[b.Class] })
	h.Findings = c.rows
	h.Healthy = true
	for _, f := range h.Findings {
		if f.Severity == SeverityFinding {
			h.Healthy = false
			break
		}
	}
	emitDoctor(ctx, cfg, h)
	return h, nil
}

// checks accumulates Health rows; per-class helpers keep every row named.
type checks struct {
	rows []HealthFinding
}

func (c *checks) finding(cl CheckClass, name, detail string) {
	c.rows = append(c.rows, HealthFinding{Class: cl, Name: name, Detail: detail, Severity: SeverityFinding})
}

func (c *checks) advisory(cl CheckClass, name, detail string) {
	c.rows = append(c.rows, HealthFinding{Class: cl, Name: name, Detail: detail, Severity: SeverityAdvisory})
}

func (c *checks) codeHost(ctx context.Context, cfg resolve.Config) (*codehost.Readiness, error) {
	probe := cfg.CodeHostDiagnostics
	if probe == nil {
		profile, err := integrations.Load(cfg.Root)
		if cfg.Integrations != nil {
			profile, err = *cfg.Integrations, nil
		}
		if err == nil && profile.Bindings.CodeHost.Provider == integrations.LocalGit {
			c.finding(CheckCodeHost, integrations.LocalGit, "no local Git diagnostics adapter was composed; use the installed pyxis doctor command")
		}
		return nil, nil
	}
	r, err := probe.Inspect(ctx)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		c.finding(CheckCodeHost, "configured origin", err.Error())
		return nil, nil
	}
	c.advisory(CheckCodeHost, "configured origin", fmt.Sprintf("%s remote %q resolves to bare origin %s", r.Provider, r.Remote, r.Origin))
	if r.MainlineOID == "" {
		c.finding(CheckCodeHost, r.MainlineRef, "configured mainline is absent — "+r.MainlineRemedy)
	} else {
		c.advisory(CheckCodeHost, r.MainlineRef, "origin commit "+r.MainlineOID)
	}
	if !r.ProposalReady {
		c.advisory(CheckCodeHost, "proposals", r.ProposalRemedy)
	}
	if r.Trust.SharedAccount {
		c.advisory(CheckCodeHost, "trust", "shared OS account: approver identity is unauthenticated, mainline has no host-level protection, and copied origins can retain the same records; verification provides record consistency without independent origin identity")
	}
	return &r, nil
}

// drift is check 1: the manifest-drift report over THE drift predicate
// (scanTree — the same classification converge repairs from, AP-11).
//
//   - source mode: skipped with the named reason (the branch AROUND
//     converge.Drift's ErrSourceMode refusal, which stands untouched).
//   - release mode, manifest verifiably cached (resolve.CachedManifest —
//     the read-only probe; the cached bytes re-hashed against the config
//     pin on this call): scanTree findings verbatim, one row per divergent
//     file, named by ownership class and state.
//   - release mode, not verifiably cached: the named "drift unknown"
//     advisory — doctor performs no fetch, so the drift verdict is honestly
//     unavailable until `pyxis converge` warms the cache.
//
// The returned error is non-nil only when the context died mid-scan.
func (c *checks) drift(ctx context.Context, cfg resolve.Config, source bool) error {
	if source {
		c.authority(ctx, cfg)
		c.advisory(CheckDrift, "manifest-drift",
			"skipped — source mode: the source tree is authoritative and nothing diffs against a fetched release (installer-v1 §3.10, DINV-8); converge and repair refuse here too")
		return nil
	}
	// Independent of manifest availability (§3.8): an already-installed repo
	// carrying the hosted workflow under an excluding ci_mode is a knob/tree
	// mismatch doctor can name without the manifest or the archive.
	c.ciModeWorkflowMismatch(cfg)
	m, ok, err := resolve.CachedManifest(cfg)
	if err != nil {
		// Real defects: a missing/malformed pin, an unreadable cache file,
		// or pin-true bytes that do not parse. Named, and the rest of the
		// report still runs.
		c.finding(CheckDrift, "release-manifest", err.Error())
		c.authority(ctx, cfg)
		return nil
	}
	if !ok {
		c.authority(ctx, cfg)
		c.advisory(CheckDrift, "release-manifest",
			"drift unknown — run `pyxis converge`: the pinned release manifest is not verifiably cached, and doctor performs no fetch (FR-14: a doctor run mutates nothing)")
		return nil
	}
	// Doctor is mutation-free and never fetches/extracts (FR-14): scan the tree
	// AS IS, archive-free — a generated surface whose template the tree lacks is
	// named drift, not materialized.
	sc, err := scanTree(ctx, cfg, m, "", false)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("doctor: %w", err)
		}
		c.finding(CheckDrift, "tree-scan", fmt.Sprintf("the drift scan failed: %v — fix the unreadable path and re-run", err))
		return nil
	}
	for _, f := range configuredFindings(sc.Findings, cfg) {
		detail := fmt.Sprintf("%s %s", f.Class, f.State)
		if f.Detail != "" {
			detail += " — " + f.Detail
		}
		c.finding(CheckDrift, f.Path, detail)
	}
	return nil
}

// ciModeWorkflowMismatch is release-mode-only (an "already-installed repo",
// §3.8) and independent of manifest availability: it needs neither the
// manifest nor the archive, only the resolved knob and the tree. The hosted
// workflow is knob-conditional in the operand's desired set — excluded means
// UNLISTED, and converge never acts on an unlisted path (no deletion
// machinery exists anywhere, #D-4) — so a repo that captured the workflow
// under a formerly-including ci_mode and later excludes it is left carrying a
// file converge will neither repair nor remove. Doctor names the mismatch,
// advisory-only (never a finding — the file is unlisted, not drifted), with
// both remedies: flip the knob back, or delete the file by hand. A born-right
// install (no file, excluding ci_mode) is silent.
func (c *checks) ciModeWorkflowMismatch(cfg resolve.Config) {
	cfg.Repo = workflowMode(cfg.Root, cfg.Repo)
	if hostedWorkflowIncluded(cfg.Repo) {
		return
	}
	_, target, err := payload.WorkflowForRoot(cfg.Root)
	if err != nil {
		return
	}
	if _, err := os.Stat(treePath(cfg.Root, target)); err != nil {
		return
	}
	mode := cfg.Repo.Knobs.CIMode
	modeDesc := fmt.Sprintf("%q", mode)
	if mode == "" {
		modeDesc = "uncaptured"
	}
	c.advisory(CheckDrift, target,
		fmt.Sprintf("present but ci_mode (%s) excludes hosted CI (prd:init-hardening/FR-11, design:init-hardening-v1#3.8) — converge treats an excluded path as unlisted and neither repairs nor deletes it; either set knobs.ci_mode to \"actions\" or \"both\" in .pyxis/config.yaml, or delete %s by hand",
			modeDesc, target))
}

// prerequisites is check 3a, delegated to the preflight suite's
// post-install pass (design init-hardening-v1 §3.1, DINV-2; plan
// init-hardening/001 T10): the presence/floor predicates and the
// gh-when-required applicability (the declared-operation table, never a
// second knob predicate) are implemented exactly once, in
// internal/preflight; this method only renders the returned rows onto
// doctor's finding shapes — texts, classes, and severities unchanged for
// missing-tool and best-effort below-floor rows. Reports
// whether git is present (the hooks probe needs it).
func (c *checks) prerequisites(ctx context.Context, cfg resolve.Config) (gitPresent bool) {
	gitPresent = true
	if boundary := preflight.ProcessBoundary(ctx); boundary.Status != preflight.StatusPass {
		c.finding(CheckPrerequisite, preflight.ProcessBoundaryID, boundary.Warning)
	}
	profile, err := integrations.Load(cfg.Root)
	if err != nil {
		c.finding(CheckPrerequisite, "integrations", err.Error())
		return gitPresent
	}
	prerequisites, err := preflight.PrerequisitesForIntegrations(ctx, cfg.Repo.Knobs, nil, profile)
	if err != nil {
		c.finding(CheckPrerequisite, "integrations", err.Error())
		return gitPresent
	}
	for _, tool := range prerequisites {
		switch {
		case !tool.Present:
			switch tool.Name {
			case "git":
				gitPresent = false
				c.finding(CheckPrerequisite, "git",
					"not found on PATH — git is a hard prerequisite of a pyxis-operated repository (prd:installer/UB-7); install git and re-run")
			case "gh":
				c.finding(CheckPrerequisite, "gh",
					"not found on PATH — the captured knobs put this repository in GitHub mode (tracker=github or active wiring), making the gh CLI a prerequisite (installer-v1 §3.6); install gh (https://cli.github.com) and authenticate, or choose wiring-free knobs")
			default:
				// A tool the suite grows later renders generically rather than
				// vanishing — the absent-row mirror of the below-floor branch.
				c.finding(CheckPrerequisite, tool.Name,
					fmt.Sprintf("not found on PATH — %s is a release prerequisite the preflight suite probes; install it and re-run (prd:installer/UB-7)", tool.Name))
			}
		case tool.BelowFloor:
			c.finding(CheckPrerequisite, tool.Name,
				fmt.Sprintf("version %s is below the release's documented floor %s — upgrade %s (prd:init-hardening/FR-3)", tool.Version, tool.Floor, tool.Name))
		}
	}
	return gitPresent
}

// hooks is check 3b: core.hooksPath configured to the expected value, read
// from git's effective config for the repo (`git -C <root> config --get`).
// With git absent the probe is honestly skipped as an advisory — the git
// prerequisite finding already reds the run and names the remedy.
func (c *checks) hooks(ctx context.Context, cfg resolve.Config, gitPresent bool) {
	if !gitPresent {
		c.advisory(CheckHooks, "core.hooksPath",
			"not probed — git is unavailable (the prerequisite finding carries the remedy)")
		return
	}
	out, err := exec.CommandContext(ctx, "git", "-C", cfg.Root, "config", "--get", "core.hooksPath").Output()
	val, set := strings.TrimSpace(string(out)), true
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != 1 {
			c.finding(CheckHooks, "core.hooksPath", fmt.Sprintf("reading git config failed: %v — fix the repository's git state and re-run", err))
			return
		}
		val, set = "", false // exit 1: the key is unset
	}
	if !set || val != hooksPathExpected {
		if active, err := githooks.Active(ctx, cfg.Root, hooksPathExpected); err != nil {
			c.finding(CheckHooks, "core.hooksPath", err.Error())
			return
		} else if active {
			return
		}
		got := "unset"
		if set {
			got = fmt.Sprintf("%q", val)
		}
		// Adoption-aware: a repository adoption chained (the engine records the
		// displaced prior path in `pyxis.chainedhookspath` — the same key
		// internal/adopt.ApplyHookChain sets; named literally here because
		// converge cannot import adopt without a cycle) whose core.hooksPath is
		// no longer the engine's directory has been displaced since adoption —
		// e.g. a later hook-manager re-install (existing-repo-onboarding-v2
		// §3.9, NFR-3). Name it as such so re-running converge/`adopt` restores
		// the chain, distinct from a repo whose gate was simply never activated.
		if prior, chained := gitConfigValue(ctx, cfg.Root, "pyxis.chainedhookspath"); chained && prior != "" {
			c.finding(CheckHooks, "core.hooksPath",
				fmt.Sprintf("%s, expected %q — the engine's hook chain has been displaced since adoption (the adopter's prior path %q is recorded); re-run converge or `adopt` to restore it (existing-repo-onboarding-v2 §3.9)", got, hooksPathExpected, prior))
			return
		}
		c.finding(CheckHooks, "core.hooksPath",
			fmt.Sprintf("%s, expected %q — the hook gate is inactive; activate it with `git config core.hooksPath %s` (prd:installer/FR-9)", got, hooksPathExpected, hooksPathExpected))
	}
}

// gitConfigValue reads a local git-config value in root, returning the value
// and whether the key is set. An unset key (git exit 1) or any other probe
// failure returns ("", false) — doctor never fails on this ancillary read.
func gitConfigValue(ctx context.Context, root, key string) (string, bool) {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "config", "--get", key).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// tracker is check 3c: reachability per the captured tracker knob.
// carina — the work-item store root exists (existence only; the store root
// is <root>/<paths.work_items>, exactly where cmd/carina roots its
// localfs store). github — live reachability is deliberately NOT probed:
// doctor is network-free; the named advisory says so. An uncaptured knob
// checks nothing (no tracker was chosen).
func (c *checks) tracker(cfg resolve.Config) {
	profile, err := integrations.Load(cfg.Root)
	if cfg.Integrations != nil {
		profile = *cfg.Integrations
		err = nil
	}
	if err != nil {
		return
	} // prerequisites reports the same invalid configuration.
	if profile.Bindings.Tasks != (integrations.Binding{}) {
		selected, err := profile.Tasks("", cfg.Repo.Knobs.Tracker, cfg.Repo.WorkItems)
		if err != nil {
			c.finding(CheckTracker, "integrations", err.Error())
			return
		}
		if selected.Provider != integrations.Carina {
			c.advisory(CheckTracker, selected.Provider, "reachability not probed (network-free) — selected task repository "+selected.Key())
			return
		}
		cfg.Repo.Knobs.Tracker = integrations.Carina
		cfg.Repo.WorkItems = selected.Root
	}
	switch cfg.Repo.Knobs.Tracker {
	case "carina":
		rel := cfg.Repo.WorkItems
		root := treePath(cfg.Root, rel)
		if filepath.IsAbs(rel) {
			root = rel
		}
		st, err := os.Stat(root)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			c.finding(CheckTracker, "carina",
				fmt.Sprintf("work-item store root %q is absent — the carina tracker is unreachable; initialize it (`carina init`) or re-run init's wiring step", rel))
		case err != nil:
			c.finding(CheckTracker, "carina", fmt.Sprintf("work-item store root %q: probe failed: %v", rel, err))
		case !st.IsDir():
			c.finding(CheckTracker, "carina",
				fmt.Sprintf("work-item store root %q is not a directory — the carina tracker is unreachable; restore the store directory", rel))
		}
	case "github":
		c.advisory(CheckTracker, "github",
			"reachability not probed (network-free) — doctor performs no network operations (prd:installer/FR-14); wiring health beyond reachability is out of scope")
	}
}

// surfaces is check 3d: for each configured CLI surface, every conventional
// pointer exists and is a symlink pointing at exactly its registered target
// (a skill/script pointer into .pyxis/, or the CLI's manual pointer to
// AGENTS.md — AP-25: vendor surfaces hold pointers, never content). A surface
// with no registered pointer convention is a named advisory, not a guess.
func (c *checks) surfaces(cfg resolve.Config) {
	if len(cfg.Repo.Knobs.CLISurfaces) == 0 {
		return
	}
	pointerSet := SurfacePointers(cfg.Repo)
	for _, s := range cfg.Repo.Knobs.CLISurfaces {
		pointers, known := pointerSet[s]
		if !known {
			c.advisory(CheckSurfaces, s,
				fmt.Sprintf("no pointer convention is registered for CLI surface %q — nothing probed", s))
			continue
		}
		for _, p := range pointers {
			abs := treePath(cfg.Root, p.Link)
			fi, lerr := os.Lstat(abs)
			if lerr != nil {
				if errors.Is(lerr, fs.ErrNotExist) {
					c.finding(CheckSurfaces, s,
						fmt.Sprintf("pointer %s is absent — the %s surface is not wired; restore it as a symlink to %s (AP-25)", p.Link, s, p.Target))
				} else {
					c.finding(CheckSurfaces, s, fmt.Sprintf("pointer %s: probe failed: %v", p.Link, lerr))
				}
				continue
			}
			if fi.Mode()&fs.ModeSymlink == 0 {
				c.finding(CheckSurfaces, s,
					fmt.Sprintf("pointer %s is not a symlink — vendor surfaces are symlinks, never content (AP-25)", p.Link))
				continue
			}
			tgt, rerr := os.Readlink(abs)
			if rerr != nil {
				c.finding(CheckSurfaces, s, fmt.Sprintf("pointer %s: read symlink: %v", p.Link, rerr))
				continue
			}
			if filepath.ToSlash(tgt) != p.Target {
				c.finding(CheckSurfaces, s,
					fmt.Sprintf("pointer %s points at %q, want %q — re-point the %s surface (AP-25)", p.Link, tgt, p.Target, s))
			}
		}
	}
	for _, defect := range HarnessSurfaceDefects(cfg.Root, cfg.Repo) {
		c.finding(CheckSurfaces, "generated-skills", defect+" — run `pyxis converge` to regenerate the harness views")
	}
}

// epoch is check 4: report the config's epoch block or its absence. Absence
// is an advisory — the block is written once by init (installer-v1 §3.5
// step 8); pre-installer repos (the source repo included) have none until
// the adoption follow-up.
func (c *checks) epoch(cfg resolve.Config) (present bool) {
	e := cfg.Repo.Epoch
	if e.SystemVersion == "" && e.Commit == "" && e.ManifestSHA256 == "" {
		c.advisory(CheckEpoch, "epoch",
			"no epoch block in .pyxis/config.yaml — `pyxis init` writes it once (installer-v1 §3.5 step 8); doctor reports it once present")
		return false
	}
	return true
}

// deprecations is check 5: the config dual-read window's notices, verbatim
// (Config.Deprecations() — one per legacy per-tool pin section still
// present in a v2 config, installer-v1 §3.10).
func (c *checks) deprecations(cfg resolve.Config) {
	for _, d := range cfg.Repo.Deprecations() {
		c.advisory(CheckDeprecation, ".pyxis/config.yaml", d)
	}
}

// cache is check 6: cache hygiene through the janitor's read-only planner
// (resolve.ReapPlan) — the off-version entries a reap would remove, named
// as advisories and left in place (§3.4: doctor names any stale entry the
// janitor missed; removal stays converge's).
func (c *checks) cache(cfg resolve.Config) {
	victims, err := resolve.ReapPlan(cfg)
	if err != nil {
		c.finding(CheckCache, ".pyxis/bin", fmt.Sprintf("cache scan failed: %v", err))
		return
	}
	for _, v := range victims {
		c.advisory(CheckCache, v,
			"off-version cache entry — the janitor removes it after the next successful pre-warm (`pyxis converge`, AP-12)")
	}
}

// Skeleton-detection marker — exactly what instantiation leaves behind (the
// seeded governance pair, installed byte-verbatim per payload.SkeletonTargets):
// product vision and architecture principles each carry the permanent
// authoring line, and the doc is unauthored while NOTHING follows it (the
// skeleton's own guidance: "While nothing follows the authoring line, `pyxis
// doctor` reports this document as an unauthored skeleton").
//
// A doc without the marker was never seeded from a skeleton — this source
// repo's hand-authored governance docs, and any opt-in top-level document a
// repo adds itself (the living maps included) — and is never an advisory.
const authoringLine = "*Authored content appends below this line.*"

// skeletons is check 7: an advisory per unauthored governance-doc skeleton,
// over the two governing docs init seeds (payload.SkeletonTargets). An absent
// doc is not a skeleton state (seeding is init's turf) and reports nothing.
func (c *checks) skeletons(cfg resolve.Config) {
	skel := payload.SkeletonTargets(cfg.Repo)
	targets := make([]string, 0, len(skel))
	for _, tgt := range skel {
		targets = append(targets, tgt)
	}
	slices.Sort(targets)
	for _, tgt := range targets {
		b, err := os.ReadFile(treePath(cfg.Root, tgt))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			c.advisory(CheckSkeleton, tgt, fmt.Sprintf("unreadable: %v — skeleton state unknown", err))
			continue
		}
		if unauthored, why := skeletonState(b); unauthored {
			c.advisory(CheckSkeleton, tgt,
				fmt.Sprintf("unauthored skeleton: %s — the governing content is the owner's to author (the factory never writes it, prd:installer/NG-1); the welcome item carries the authoring pointers", why))
		}
	}
}

// skeletonState classifies one governance doc's bytes: a doc carrying the
// authoring-line marker is unauthored while nothing follows it; a doc without
// the marker was never seeded from a skeleton and is never a skeleton state.
func skeletonState(b []byte) (unauthored bool, why string) {
	lines := strings.Split(string(b), "\n")
	marker := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == authoringLine {
			marker = i
		}
	}
	if marker < 0 {
		return false, ""
	}
	for _, l := range lines[marker+1:] {
		if strings.TrimSpace(l) != "" {
			return false, ""
		}
	}
	return true, "nothing follows the authoring line"
}

// verification is check 8 (prd:init-hardening/FR-15; design init-hardening-v1
// §3.9): recognized code marker files present with no verification manifest
// is an advisory naming the codegate seed verb and the hand-authoring
// fallback — a manifest already present, or a tree with no recognized code
// at all, reports nothing. Both the manifest path and the marker set read
// from their single sources (internal/verify/invoke, internal/verify/
// ecosystem — the same sources the codegate seed verb itself and adopt's
// stack detection read, prd:implementation-verification/FR-20), never a
// second, hand-kept copy. The verb this advisory routes to is a sibling
// build (design init-hardening-v1 §3.9, #D-6) — doctor only names it.
func (c *checks) verification(cfg resolve.Config) {
	if _, err := os.Stat(treePath(cfg.Root, invoke.ManifestRelPath)); err == nil {
		return // a manifest already exists — operator-owned, nothing to advise
	}
	if !anyStackMarkerPresent(cfg.Root, ecosystem.New()) {
		return // no recognized code at all — nothing to gate yet
	}
	c.advisory(CheckVerification, invoke.ManifestRelPath,
		fmt.Sprintf("recognized code is present with no verification manifest — seed one with `pyxis exec codegate -- seed` (it detects your stack from marker files and writes a template manifest for you to own), or hand-author %s yourself (prd:implementation-verification/FR-20)", invoke.ManifestRelPath))
}

// anyStackMarkerPresent reports whether any known stack's marker file exists
// at root — existence only, the same predicate adopt's own stack detection
// applies (internal/adopt.hasMarker) evaluated independently here: converge
// cannot import internal/adopt (adopt already imports converge for its
// classification reuse), so this is the second reader of the one ecosystem
// source rather than a second source.
func anyStackMarkerPresent(root string, defs *ecosystem.Defaults) bool {
	for _, name := range defs.StackNames() {
		for _, marker := range defs.ForStack(name).MarkerFiles {
			if _, err := os.Stat(treePath(root, marker)); err == nil {
				return true
			}
		}
	}
	return false
}

// emitDoctor logs the EVT-3 doctor_run event (installer-v1 §3.8): healthy,
// mode, the finding/advisory totals, the per-class row counts (every
// taxonomy class, zeros included — a stable machine shape), the run id, and
// the artifact-source override when set (§3.9, loud).
func emitDoctor(ctx context.Context, cfg resolve.Config, h Health) {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	counts := map[CheckClass]int{}
	findings, advisories := 0, 0
	for _, f := range h.Findings {
		counts[f.Class]++
		if f.Severity == SeverityAdvisory {
			advisories++
		} else {
			findings++
		}
	}
	byClass := make([]slog.Attr, 0, len(checkTaxonomy))
	for _, cl := range checkTaxonomy {
		byClass = append(byClass, slog.Int(string(cl), counts[cl]))
	}
	attrs := []slog.Attr{
		slog.String("run_id", h.RunID),
		slog.Bool("healthy", h.Healthy),
		slog.String("mode", h.Mode),
		slog.Int("findings", findings),
		slog.Int("advisories", advisories),
		{Key: "findings_by_class", Value: slog.GroupValue(byClass...)},
	}
	if h.ArtifactSource != "" {
		attrs = append(attrs, slog.String("artifact_source", h.ArtifactSource))
	}
	log.LogAttrs(ctx, slog.LevelInfo, "doctor_run", attrs...)
}
