// Package converge is the installer's ownership-class reconcile engine
// (installer-v1 §3.2, §3.5, DINV-3/DINV-4; plan installer/001 T5): one
// library that converges a pyxis-operated repository onto its pinned release
// manifest, idempotently and raceably, with a preview mode that reports the
// exact write and external-operation set and performs none of it (FR-19).
//
// The operand is resolve.Config — the same root/parsed-config/fetcher/logger
// seams the acquisition core reads; converge adds none. Acquisition itself
// stays in internal/resolve (one acquisition owner, AP-10/AP-11): converge
// calls resolve.EnsureManifest (the config-pinned chain, DINV-2),
// resolve.EnsurePayload (the archive verified against the manifest's
// payload.archive_sha256 — part of the offline pre-fetch floor, §3.3),
// resolve.PreWarm, and resolve.Reap.
//
// Per ownership class (the manifest's closed enum, installer-v1 §3.2):
//
//   - engine-verbatim — the tree file's sha256 must equal the manifest's;
//     drift or absence repairs BYTE-EXACT from the verified payload archive
//     (each extracted file is re-hashed against its manifest entry before
//     any write — a mismatch is a named defect of the pinned release, not a
//     write), atomically (tmp + rename beside the target), preserving the
//     archive's owner-exec bit (0755/0644).
//   - generated — regenerated through the EXISTING sole generator, never a
//     second implementation: docs/tech-design index files regenerate via
//     internal/docgate/gate.GenIndexBytes — exactly what `docgate gen-index`
//     writes — falling back to the same generator over an empty corpus when
//     the design directory does not exist yet. A generated path with no
//     registered generator, or a corpus the generator refuses (defects), is
//     a named refusal — never an invented file.
//   - repo-owned — NEVER written (DINV-3). Validation only: .pyxis/config.yaml
//     must parse under the closed schema (internal/docgate/config.Parse);
//     every other repo-owned path (today .pyxis/stamp/** — no schema exists
//     for it) is validated existence-only. A validation failure — including
//     absence, since converge may not create repo-owned content — is a named
//     refusal in the report; converge continues the other files and returns
//     one summarizing error at the end.
//
// Idempotency and interruption (FR-6, UB-5, AP-22): a write happens only
// when bytes differ or the file is missing, so a second run on a converged
// tree performs zero mutations and reports mode "noop"; every write is
// atomic per file, so a killed run leaves only converged-or-untouched files
// plus at most an abandoned tmp, and a re-run converges to the identical end
// state.
//
// Preview (FR-19) is honest about acquisition: it performs NO repo-tree
// writes, NO cache writes, and NO fetches. With the manifest verifiably
// cached (re-hashed against the config pin on this call) it enumerates the
// exact per-file write set, the payload/pre-warm fetches a real run would
// perform, and the cache entries the janitor would reap. Without it, the
// write set is not computable without fetching — so preview reports the
// manifest fetch itself as the pending external operation and says the rest
// honestly in Report.Incomplete instead of guessing.
//
// Source mode (installer-v1 §3.10, DINV-8): the source repo is not an
// installed repo — the tree IS the source of the payload, so there is
// nothing to converge against a fetched release. Run and Drift refuse by
// name (ErrSourceMode) before any acquisition, pre-warm, or reap; doctor's
// read-only face remains available separately (T6).
//
// The drift predicate is ONE function (scanTree) shared by Run and the
// exported read-only Drift face doctor consumes (T6) — one classification,
// one place (AP-11).
//
// Observability: every finished reconcile (real or preview) emits the EVT-2
// `converge_run` slog event — mode install|repair|noop|preview, repaired
// count, drift classes — plus the run id (installer-v1 §3.8) and, loudly,
// the artifact-source override when set (§3.9). Zero telemetry egress: the
// logger is local.
package converge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/gitcredential"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// Options is Run's closed operand.
type Options struct {
	CredentialExecutable string // optional durable pyxis path; empty resolves the running installed binary
	// Preview reports every write and external operation a real run would
	// perform and performs none of them (FR-19).
	Preview bool
	// Repair names the run: one converge either way (the design's "repair is
	// its name on a damaged tree"); only the report/event mode string reads
	// "repair" instead of "install".
	Repair bool
	// AuthoritySeedPending is used only by init's admitted empty-target
	// pipeline: payload materialization precedes the create-once policy seed.
	// It defers only an absent policy's health finding, never invalid policy
	// or legacy-byte findings, and never permits a policy write. The ordinary
	// converge CLI leaves this false; init's subsequent seed/green proof must
	// succeed before it can commit or publish an installation.
	AuthoritySeedPending bool
}

// The EVT-2 mode enum (prd:installer EVT-2: enum(install,repair,noop,preview)).
const (
	ModeInstall = "install"
	ModeRepair  = "repair"
	ModeNoop    = "noop"
	ModePreview = "preview"
)

// ErrSourceMode is the source-repo refusal (installer-v1 §3.10): the source
// tree is authoritative — nothing to converge, nothing to pre-warm, nothing
// to reap. Branch with errors.Is.
var ErrSourceMode = errors.New("source mode: the source tree is authoritative and nothing converges against a fetched release (installer-v1 §3.10, DINV-8) — converge runs only in installed repos; doctor remains available read-only")

// Action is one performed (or, in preview, enumerated) repo-tree write.
type Action struct {
	Path  string
	Class relmanifest.Class
}

// Refusal is one named per-file refusal: repo-owned validation failures
// (DINV-3 — the file is left untouched) and ungeneratable generated files.
type Refusal struct {
	Path   string
	Class  relmanifest.Class
	Reason string
}

// Fetch is one external acquisition preview enumerates (FR-19); the URL
// surfaces the artifact-source override loudly (installer-v1 §3.9).
type Fetch struct {
	Artifact string
	URL      string
}

// Report is the one converge result struct — real runs fill the performed
// sets, preview fills the would-do sets; both fill Mode, RunID, and
// DriftClasses (the EVT-2 payload).
type Report struct {
	Credentials []gitcredential.Change // local Git credential changes; preview lists planned values
	RunID       string
	Mode        string // install | repair | noop | preview

	// Real-run outcomes.
	Repaired      []Action  // engine-verbatim files written byte-exact
	Regenerated   []Action  // generated files rewritten by their generator
	RefusedByName []Refusal // per-file refusals; also filled by preview (what a real run would refuse)

	// Preview outcomes (empty on real runs).
	WouldWrite []Action // the exact repo-tree write set of a subsequent real run
	WouldFetch []Fetch  // the external fetches a real run would perform
	Incomplete []string // what preview could NOT enumerate without fetching, and why

	// Acquisition-side outcomes (cache, never the repo tree). On a real run:
	// performed; in preview: the would-do sets, enumerated read-only.
	PreWarmed []string // tools fetch-verified into the binary cache (resolve.PreWarm)
	Reaped    []string // off-version cache entries removed (resolve.Reap)

	// DriftClasses are the ownership classes with at least one divergence,
	// sorted — the EVT-2 drift_classes field.
	DriftClasses []string
}

// Run converges the repository at cfg.Root against its pinned release
// (installer-v1 §3.5 step 3): ensure the verified manifest, ensure the
// verified payload archive, reconcile per ownership class through the one
// drift predicate, then pre-warm the binary cache and run the janitor.
// Per-file refusals do not stop the other files; they are named in the
// report and joined into the returned error. A non-nil error with a
// non-zero Report means converge ran and the report is real; a zero-value
// Report accompanies pre-reconcile refusals (source mode, missing pins,
// offline acquisition, unreadable tree).
func Run(ctx context.Context, cfg resolve.Config, opts Options) (Report, error) {
	if err := guard(ctx, cfg); err != nil {
		return Report{}, err
	}
	if opts.Preview {
		return preview(ctx, cfg, opts)
	}
	// Freeze the artifact transport once so the manifest, payload, and pre-warm
	// fetches share a single per-release asset-map memo (one listing, not one
	// per artifact).
	cfg = cfg.WithFetcher()
	m, err := resolve.EnsureManifest(ctx, cfg)
	if err != nil {
		return Report{}, fmt.Errorf("converge: %w", err)
	}
	archive, err := resolve.EnsurePayload(ctx, cfg, m)
	if err != nil {
		return Report{}, fmt.Errorf("converge: %w", err)
	}
	// A mutating run hands the verified archive to the scan so a generated
	// surface rendered from an engine-verbatim template (the installed workflow)
	// sources its authoritative bytes there, materializing on a fresh tree in
	// one pass (drift.go desiredWorkflow).
	sc, err := scanTree(ctx, cfg, m, archive, opts.AuthoritySeedPending)
	if err != nil {
		return Report{}, fmt.Errorf("converge: %w", err)
	}
	// Narrow to this operand's knob-conditional desired set (§3.8): the hosted
	// workflow is unlisted — never repaired, never counted as drift — when
	// ci_mode excludes hosted CI.
	sc.Findings = configuredFindings(sc.Findings, cfg)

	rep := Report{RunID: newRunID(), DriftClasses: driftClasses(sc.Findings)}
	var errs []error

	// The per-class writes: two independent primitives (T1;
	// existing-repo-onboarding-v2 §3.1) any caller — Run included — can
	// drive from a classification and the verified archive. Repo-owned is
	// never written (DINV-3), so it stays a direct refusal here, not a
	// primitive.
	repaired, refusedVerbatim, err := RepairEngineVerbatim(ctx, cfg.Root, sc.Findings, archive)
	rep.Repaired = repaired
	if err != nil {
		return rep, fmt.Errorf("converge: %w", err)
	}

	regenerated, refusedGenerated, err := WriteGenerated(ctx, cfg.Root, sc.Findings, sc.Generated)
	rep.Regenerated = regenerated
	if err != nil {
		return rep, fmt.Errorf("converge: %w", err)
	}
	harnessActions, err := ReconcileHarnessSurfaces(cfg.Root, cfg.Repo)
	if err != nil {
		return rep, fmt.Errorf("converge: reconcile harness surfaces: %w", err)
	}
	rep.Regenerated = append(rep.Regenerated, harnessActions...)

	var refusedRepoOwned []Refusal
	for _, f := range sc.Findings {
		if f.Class != relmanifest.RepoOwned {
			continue
		}
		if err := ctx.Err(); err != nil {
			return rep, fmt.Errorf("converge: %w", err)
		}
		// Never written (DINV-3): the divergence itself is the refusal.
		refusedRepoOwned = append(refusedRepoOwned, Refusal{Path: f.Path, Class: f.Class, Reason: f.Detail})
	}

	// sc.Findings is path-sorted (scanTree), so each per-class refusal set
	// above is already a path-sorted subsequence of it; re-sorting the union
	// restores the single global path order the former one-pass loop produced.
	rep.RefusedByName = append(append(refusedVerbatim, refusedGenerated...), refusedRepoOwned...)
	slices.SortFunc(rep.RefusedByName, func(a, b Refusal) int { return strings.Compare(a.Path, b.Path) })

	// Pre-warm the binary cache, then the janitor (installer-v1 §3.4: the
	// reap follows a successful pre-warm). The plan is computed first so the
	// report names exactly the tools this run fetched.
	plan, perr := resolve.PreWarmPlan(cfg, m)
	if perr != nil {
		errs = append(errs, fmt.Errorf("converge: pre-warm plan: %w", perr))
	}
	if werr := resolve.PreWarm(ctx, cfg); werr != nil {
		errs = append(errs, fmt.Errorf("converge: pre-warm: %w", werr))
	} else {
		rep.PreWarmed = plan
		reaped, rerr := resolve.Reap(cfg)
		rep.Reaped = reaped
		if rerr != nil {
			errs = append(errs, fmt.Errorf("converge: reap: %w", rerr))
		}
	}

	if n := len(rep.RefusedByName); n > 0 {
		paths := make([]string, n)
		for i, r := range rep.RefusedByName {
			paths[i] = r.Path
		}
		errs = append(errs, fmt.Errorf("converge: %d file(s) refused by name (%s) — repo-owned content is never rewritten (DINV-3) and nothing unverifiable is written; fix the named files (or pin a coherent release) and re-run", n, strings.Join(paths, ", ")))
	}
	if len(errs) == 0 {
		if cfg.Integrations != nil {
			rep.Credentials, err = gitcredential.ConfigureForProfile(ctx, cfg.Root, opts.CredentialExecutable, *cfg.Integrations)
		} else {
			rep.Credentials, err = gitcredential.Configure(ctx, cfg.Root, opts.CredentialExecutable)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("converge: configure Git credentials: %w", err))
		}
	}

	rep.Mode = mode(opts, len(rep.Repaired)+len(rep.Regenerated)+len(rep.Credentials))
	emit(ctx, cfg, rep)
	return rep, errors.Join(errs...)
}

// Drift is the read-only face of the one drift predicate (shared with Run;
// doctor consumes it in T6): the per-file divergences of the tree at
// cfg.Root against the pinned manifest, classified per ownership class,
// sorted by path, plus the generated class's desired bytes (Classification.
// Generated) — what RepairEngineVerbatim/WriteGenerated need to act on a
// chosen path set without a second render (adopt: existing-repo-onboarding-v2
// §3.1). It writes nothing to the repo tree; obtaining the manifest goes
// through resolve's ensure (a verifiably cached manifest is re-hashed in
// place — no mutation; a cold cache fetch-verifies into the gitignored
// cache, resolve's own contract).
func Drift(ctx context.Context, cfg resolve.Config) (Classification, error) {
	if err := guard(ctx, cfg); err != nil {
		return Classification{}, err
	}
	m, err := resolve.EnsureManifest(ctx, cfg)
	if err != nil {
		return Classification{}, fmt.Errorf("converge: %w", err)
	}
	// Read-only: no archive — the scan audits the tree AS IS (a generated
	// surface whose template the tree lacks is named drift, never fetched).
	sc, err := scanTree(ctx, cfg, m, "", false)
	if err != nil {
		return Classification{}, fmt.Errorf("converge: %w", err)
	}
	return sc, nil
}

// ClassifyWithArchive is the archive-backed classification an adopting `apply`
// consumes to drive the write primitives (RepairEngineVerbatim, WriteGenerated;
// existing-repo-onboarding-v2 §3.1, plan existing-repo-onboarding/001 T8).
// Unlike Drift — write-free, archive-absent, for the read-only faces — it
// ensures the verified payload archive, so a generated surface whose template
// the tree lacks renders from the archive's authoritative bytes and a fresh
// adoption materializes every managed path in one pass: exactly the setup Run
// performs, minus Run's write loop. It returns the classification (Findings +
// the generated desired bytes) and the archive path the primitives extract
// from, and writes nothing to the repo tree.
func ClassifyWithArchive(ctx context.Context, cfg resolve.Config) (Classification, string, error) {
	if err := guard(ctx, cfg); err != nil {
		return Classification{}, "", err
	}
	m, err := resolve.EnsureManifest(ctx, cfg)
	if err != nil {
		return Classification{}, "", fmt.Errorf("converge: %w", err)
	}
	archive, err := resolve.EnsurePayload(ctx, cfg, m)
	if err != nil {
		return Classification{}, "", fmt.Errorf("converge: %w", err)
	}
	sc, err := scanTree(ctx, cfg, m, archive, false)
	if err != nil {
		return Classification{}, "", fmt.Errorf("converge: %w", err)
	}
	return sc, archive, nil
}

// preview is the FR-19 leg: enumerate, perform nothing — no repo writes, no
// cache writes, no fetches. Per-file refusals a real run would make are
// surfaced in the report but do not fail the preview itself: enumerating a
// broken tree is the point.
func preview(ctx context.Context, cfg resolve.Config, opts Options) (Report, error) {
	rep := Report{RunID: newRunID(), Mode: ModePreview}
	var err error
	if cfg.Integrations != nil {
		rep.Credentials, err = gitcredential.PlanForProfile(ctx, cfg.Root, opts.CredentialExecutable, *cfg.Integrations)
	} else {
		rep.Credentials, err = gitcredential.Plan(ctx, cfg.Root, opts.CredentialExecutable)
	}
	if err != nil {
		return rep, fmt.Errorf("converge: preview Git credentials: %w", err)
	}

	// The janitor's would-remove set needs only the version pin.
	victims, err := resolve.ReapPlan(cfg)
	if err != nil {
		return Report{}, fmt.Errorf("converge: preview: %w", err)
	}
	rep.Reaped = victims

	m, ok, err := resolve.CachedManifest(cfg)
	if err != nil {
		return Report{}, fmt.Errorf("converge: preview: %w", err)
	}
	if !ok {
		// Presence and closed-schema health need no release fetch. Even a
		// cold preview names an enabled operand's policy refusal honestly.
		if f, found := authorityHealth(ctx, cfg, relmanifest.Manifest{}); found && (!opts.AuthoritySeedPending || f.State != StateMissing) {
			rep.RefusedByName = append(rep.RefusedByName, Refusal{Path: f.Path, Class: f.Class, Reason: f.Detail})
		}
		// Not verifiably cached: fetching it here would violate FR-19, and
		// the write set is not computable without it — say both honestly.
		name, url, aerr := resolve.ManifestArtifact(cfg)
		if aerr != nil {
			return Report{}, fmt.Errorf("converge: preview: %w", aerr)
		}
		pname, purl, aerr := resolve.PayloadArtifact(cfg)
		if aerr != nil {
			return Report{}, fmt.Errorf("converge: preview: %w", aerr)
		}
		rep.WouldFetch = append(rep.WouldFetch, Fetch{Artifact: name, URL: url})
		rep.Incomplete = append(rep.Incomplete,
			fmt.Sprintf("the release manifest %s is not verifiably cached, and preview performs no fetch (FR-19): the exact repo write set, the payload-archive check (%s from %s), and the pre-warm set are computable only after that fetch — run `pyxis converge` online (or re-run --preview once the cache is warm)", name, pname, purl))
		emit(ctx, cfg, rep)
		return rep, nil
	}

	// Preview performs no fetch/extract (FR-19): scan the tree AS IS, archive-free.
	sc, err := scanTree(ctx, cfg, m, "", opts.AuthoritySeedPending)
	if err != nil {
		return Report{}, fmt.Errorf("converge: preview: %w", err)
	}
	// Same knob-conditional narrowing as Run (§3.8): preview's enumeration must
	// agree with what a real run would do.
	sc.Findings = configuredFindings(sc.Findings, cfg)
	rep.DriftClasses = driftClasses(sc.Findings)
	for _, f := range sc.Findings {
		switch {
		case f.Class == relmanifest.EngineVerbatim,
			f.Class == relmanifest.Generated && f.State != StateUngeneratable:
			rep.WouldWrite = append(rep.WouldWrite, Action{Path: f.Path, Class: f.Class})
		case f.Class == relmanifest.Generated:
			rep.RefusedByName = append(rep.RefusedByName, Refusal{Path: f.Path, Class: f.Class, Reason: f.Detail})
		case f.Class == relmanifest.RepoOwned:
			rep.RefusedByName = append(rep.RefusedByName, Refusal{Path: f.Path, Class: f.Class, Reason: f.Detail})
		}
	}

	// A real run ensures the payload archive unconditionally (the offline
	// floor, §3.3): report the fetch iff its cached bytes do not verify.
	if _, pok, perr := resolve.CachedPayload(cfg, m); perr != nil {
		return Report{}, fmt.Errorf("converge: preview: %w", perr)
	} else if !pok {
		name, url, aerr := resolve.PayloadArtifact(cfg)
		if aerr != nil {
			return Report{}, fmt.Errorf("converge: preview: %w", aerr)
		}
		rep.WouldFetch = append(rep.WouldFetch, Fetch{Artifact: name, URL: url})
	}

	plan, perr := resolve.PreWarmPlan(cfg, m)
	if perr != nil {
		return Report{}, fmt.Errorf("converge: preview: %w", perr)
	}
	rep.PreWarmed = plan

	emit(ctx, cfg, rep)
	return rep, nil
}

// guard is the shared precondition check: a live context, a repo root, and
// not the source repo (ErrSourceMode — before any acquisition or cache op).
func guard(ctx context.Context, cfg resolve.Config) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("converge: %w", err)
	}
	if cfg.Root == "" {
		return errors.New("converge: Config.Root is empty (the operand repo root is required)")
	}
	if cfg.Repo.System.Mode == "source" {
		return fmt.Errorf("converge: %w", ErrSourceMode)
	}
	return nil
}

// mode derives the EVT-2 mode string: preview is decided by the caller;
// zero repo-tree mutations is a noop (FR-6's "report convergence"); a
// mutating run is named by the Repair option (one converge either way).
func mode(opts Options, mutations int) string {
	switch {
	case mutations == 0:
		return ModeNoop
	case opts.Repair:
		return ModeRepair
	default:
		return ModeInstall
	}
}

// emit logs the EVT-2 converge_run event (installer-v1 §3.8) through the
// operand's logger: mode, repaired count, drift classes, the run id, and —
// loudly, when set — the artifact-source override (§3.9).
func emit(ctx context.Context, cfg resolve.Config, rep Report) {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	attrs := []slog.Attr{
		slog.String("run_id", rep.RunID),
		slog.String("mode", rep.Mode),
		slog.Int("repaired", len(rep.Repaired)),
		slog.Any("drift_classes", rep.DriftClasses),
		slog.Int("regenerated", len(rep.Regenerated)),
		slog.Int("refused", len(rep.RefusedByName)),
		slog.Int("would_write", len(rep.WouldWrite)),
		slog.Int("would_fetch", len(rep.WouldFetch)),
		slog.Int("prewarmed", len(rep.PreWarmed)),
		slog.Int("reaped", len(rep.Reaped)),
	}
	if s := strings.TrimSpace(cfg.Repo.System.ArtifactSource); s != "" {
		attrs = append(attrs, slog.String("artifact_source", s))
	}
	log.LogAttrs(ctx, slog.LevelInfo, "converge_run", attrs...)
}

// driftClasses reduces findings to the sorted unique ownership classes with
// drift — the EVT-2 drift_classes payload.
func driftClasses(findings []Finding) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range findings {
		c := string(f.Class)
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	slices.Sort(out) // a stable event payload regardless of finding order
	return out
}

// newRunID mints the run correlation id (installer-v1 §3.8) — the same
// 8-byte-hex shape the wiring audit uses.
func newRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
