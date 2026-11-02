// Inspect is the `adopt` verb's read (design existing-repo-onboarding-v2
// §3.1): dispatch by tree state, classify a non-empty repository against
// the pinned release manifest by reusing converge's classification (one
// classifier, AP-11/DINV-2), and assemble a draft adoption-plan — writing
// nothing else to the repository tree (prd:existing-repo-onboarding/FR-1).
// It never invokes converge.Run and never applies anything; apply and undo
// (plan existing-repo-onboarding/001 T8, T10) are separate operations built
// on top of the same plan.Plan artifact (T2). Classifying a cold cache
// fetch-verifies the pinned manifest into converge's gitignored resolve
// cache (converge.Drift's own contract, unchanged here) — never a write to
// the repository's tracked content.
package adopt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// PlanPath is the draft/applied adoption-plan record's fixed, repo-relative
// location: the engine's own system namespace (design §3.2), a path no
// release manifest lists, so the plan itself never collides with — or is
// classified by — the very manifest it records against. apply (T8) and undo
// (T10) bind to this same path; there is no second location for it.
const PlanPath = ".pyxis/adopt/plan.json"

// installerInitFn is the empty-tree route's callee — a package var so an
// internal test can prove Inspect's dispatch hands installer.Init exactly
// the right Options without needing a full init-pipeline fixture (seeding,
// hooks, wiring); production always uses the real installer.Init.
var installerInitFn = installer.Init

// ErrPinRequired is returned when Options.Pin is incomplete at the point
// Inspect actually needs it: routing an empty tree to init, or classifying a
// non-empty one. The UB-5 (not a git repository) and UB-7 (invalid prior
// config) refusals are checked first and need no pin — the more fundamental
// problem, when present, surfaces before this one.
var ErrPinRequired = errors.New("adopt: the system pin pair is required — pass --system-version and --manifest-sha256, or install via install.sh (which records them at $XDG_CONFIG_HOME/pyxis/release.env)")

// ErrConfigSurfaceInvalid is UB-7's refusal: a prior .pyxis/config.yaml is
// present but fails to parse under the closed schema (partial or
// malformed) — adopt cannot trust a classification built against it
// (prd:existing-repo-onboarding/UB-7). Branch with errors.Is.
var ErrConfigSurfaceInvalid = errors.New("adopt: the prior pyxis config surface does not parse under the closed schema")

// configSurfaceError carries the exact defects docgate's config parser
// named, so the refusal names the offending key and remedy (UB-7's fit
// criterion), not just that something was wrong.
type configSurfaceError struct {
	defects []schema.Defect
}

func (e *configSurfaceError) Error() string {
	parts := make([]string, len(e.defects))
	for i, d := range e.defects {
		parts[i] = d.String()
	}
	return fmt.Sprintf("%v: %s", ErrConfigSurfaceInvalid, strings.Join(parts, "; "))
}

func (e *configSurfaceError) Is(target error) bool { return target == ErrConfigSurfaceInvalid }

// Options is Inspect's operand.
type Options struct {
	// Target is the directory to inspect — cmd/pyxis passes the working
	// directory (FR-1's flow: "runs the adopt verb ... at the repo root").
	// It must exist.
	Target string
	// Pin is the system release pin classification needs to resolve the
	// manifest against — required only once dispatch reaches a branch that
	// classifies or installs (see ErrPinRequired).
	Pin installer.Pin
	// ArtifactSource is the optional FR-22 mirror/fork override, forwarded
	// to both the routed-to-init leg and the classify leg.
	ArtifactSource string
	// Fetcher overrides converge's/init's artifact transport (tests); nil
	// uses the scheme dispatch.
	Fetcher resolve.Fetcher
	// Logger receives the structured events converge/init emit; nil uses
	// slog.Default().
	Logger *slog.Logger
}

// Report is Inspect's result: either the empty-tree route to init ran
// (RoutedToInit, carrying init's own Report), or a fresh draft plan was
// classified, assembled, and written (Plan/PlanPath).
type Report struct {
	RoutedToInit bool
	Init         installer.Report

	Plan     Plan
	PlanPath string // repo-relative; always PlanPath when RoutedToInit is false
}

// Inspect runs the tree-state dispatch (UB-5, UB-1, UB-7) and, for a
// non-empty git repository carrying a valid (or absent) prior config,
// classifies it against the pinned release manifest by reusing converge's
// classification and writes a draft adoption-plan naming every acted-on
// path plus the never-touch set (the manifest's complement). It performs no
// other write TO THE REPOSITORY TREE: an empty tree is routed to init
// (which performs its own, separate, fully-disclosed writes) and every
// other leg either refuses or writes exactly PlanPath — classification's
// own manifest-ensure may separately warm converge's gitignored resolve
// cache (.pyxis/bin/), the same cache Drift itself relies on and never a
// repository-tree write.
func Inspect(ctx context.Context, opts Options) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, fmt.Errorf("adopt: %w", err)
	}
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		return Report{}, errors.New("adopt: Options.Target is empty (the target directory is required)")
	}

	// UB-5: refused by name before any inspection of the tree's content.
	isGit, err := isGitRepository(target)
	if err != nil {
		return Report{}, err
	}
	if !isGit {
		return Report{}, ErrNotGitRepository
	}

	// UB-1: a git repository with zero tracked files and a clean worktree
	// has nothing to negotiate — route to the empty-repo installer instead.
	tracked, dirty, err := treeStatus(ctx, target)
	if err != nil {
		return Report{}, err
	}
	if len(tracked) == 0 && len(dirty) == 0 {
		if err := requirePin(opts.Pin); err != nil {
			return Report{}, err
		}
		irep, err := installerInitFn(ctx, installer.Options{
			Target:         target,
			Pin:            opts.Pin,
			ArtifactSource: opts.ArtifactSource,
			AcceptDefaults: true,
			Logger:         opts.Logger,
			Fetcher:        opts.Fetcher,
		})
		if err != nil {
			return Report{}, fmt.Errorf("adopt: route to init: %w", err)
		}
		return Report{RoutedToInit: true, Init: irep}, nil
	}

	// UB-7: a prior config surface present but failing the closed-schema
	// parse is refused by name — an absent one is fine (Defaults, the
	// ordinary first-time-adopt case).
	repo, defects := config.Load(target)
	if len(defects) > 0 {
		return Report{}, &configSurfaceError{defects: defects}
	}

	if err := requirePin(opts.Pin); err != nil {
		return Report{}, err
	}
	// The loaded (or defaulted) config's own pin never applies here — a
	// never-adopted tree defaults to system.mode "source" (Defaults()),
	// which converge's guard refuses outright; a real prior pin belongs to
	// apply's re-apply idempotency (T8, UB-4), not this classify pass.
	repo.System.Mode = "release"
	repo.System.Version = opts.Pin.Version
	repo.SystemVersion = opts.Pin.Version
	repo.System.ManifestSHA256 = opts.Pin.ManifestSHA256
	if s := strings.TrimSpace(opts.ArtifactSource); s != "" {
		repo.System.ArtifactSource = s
	}

	cfg := resolve.Config{Root: target, Repo: repo, Logger: opts.Logger, Fetcher: opts.Fetcher}
	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		return Report{}, fmt.Errorf("adopt: %w", err)
	}
	// The never-touch set needs the FULL manifest path list, not just the
	// divergent findings Drift returns (a fully-converged re-adopt would
	// otherwise see its own paths as untouched). resolve.EnsureManifest
	// re-hashes a verifiably cached manifest rather than re-fetching, so a
	// second call here (Drift already made one) costs a re-read, not a
	// second fetch.
	m, err := resolve.EnsureManifest(ctx, cfg)
	if err != nil {
		return Report{}, fmt.Errorf("adopt: %w", err)
	}

	entries, err := entriesFromFindings(cls.Findings)
	if err != nil {
		return Report{}, err
	}
	entries, err = ResolveWorkflowEntries(target, entries)
	if err != nil {
		return Report{}, err
	}
	m, err = converge.ProjectWorkflowManifest(target, m)
	if err != nil {
		return Report{}, err
	}
	entries = append(entries, neverTouchEntries(m.Payload.Files, tracked)...)
	// Root convention-file collision (T7, design existing-repo-onboarding-v2
	// §3.3; prd:existing-repo-onboarding/FR-6): an existing AGENTS.md is not the
	// engine's to rewrite — the DRAFT plan must DISCLOSE it as a collide-manual
	// entry carrying the exact lines for the human to add, not as a silent
	// never-touch. When it collides, replace its never-touch entry (the
	// manifest's-complement pass emits one for a tracked AGENTS.md) with the
	// collide-manual entry the resolver builds; apply then writes nothing to it
	// (byte-identical, FR-6) and undo names it for reversal (§3.5).
	if conv, ok, err := ResolveRootConvention(target); err != nil {
		return Report{}, err
	} else if ok {
		entries = append(dropEntry(entries, conv.Path), conv)
	}
	// Hook-path collision (T5, design §3.3; prd:existing-repo-onboarding/FR-4):
	// when the repository already sets core.hooksPath, the DRAFT plan discloses
	// the chain — the engine repoints core.hooksPath and its shim delegates
	// forward to the adopter's prior hook — as a chain entry naming the prior
	// path in its detail. The chain is git-config-only (no wrapper file), so it
	// carries no added_paths baseline (DINV-7 as reconciled 2026-07-26): apply
	// performs the chaining and records hooks.prior_hooks_path, and undo reverses
	// it from there — the entry is disclosure only.
	if hook, ok, err := ResolveHookChain(ctx, target); err != nil {
		return Report{}, err
	} else if ok {
		entries = append(entries, hook)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	// Detect the stack and surface it in the draft plan (T4, design
	// existing-repo-onboarding-v2 §3.6; prd:existing-repo-onboarding/FR-7):
	// name every stack a marker file identifies plus the gate-command actions
	// the adopter must supply, degrading to the single unknown-stack line when
	// nothing matches (UB-3) — never an error, never a fabricated command.
	plan := Plan{
		SchemaVersion: SchemaVersion,
		Status:        StatusDraft,
		Entries:       entries,
		Stack:         DetectStack(target),
	}
	b, err := plan.Encode()
	if err != nil {
		return Report{}, fmt.Errorf("adopt: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(target, filepath.FromSlash(PlanPath)), b); err != nil {
		return Report{}, fmt.Errorf("adopt: write plan: %w", err)
	}

	return Report{Plan: plan, PlanPath: PlanPath}, nil
}

// dropEntry returns entries with any entry naming path removed — used to
// replace a path's baseline classification (a tracked AGENTS.md's never-touch
// entry) with its more specific collision resolution, so the plan never holds
// two entries for one path.
func dropEntry(entries []Entry, path string) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Path != path {
			out = append(out, e)
		}
	}
	return out
}

// requirePin reports ErrPinRequired unless both pin halves are set;
// hex/tag normalization is downstream's own concern (installer.Init,
// resolve.EnsureManifest), never duplicated here.
func requirePin(pin installer.Pin) error {
	if strings.TrimSpace(pin.Version) == "" || strings.TrimSpace(pin.ManifestSHA256) == "" {
		return ErrPinRequired
	}
	return nil
}

var (
	createAdoptTemp = os.CreateTemp
	chmodAdoptTemp  = os.Chmod
)

// writeFileAtomic writes b to path via a tmp file in the same directory + a
// rename, creating the parent directory first — the same discipline
// converge and installer already use, so a reader sees only absent or
// complete bytes and a killed run leaves only an abandoned tmp, never a torn
// plan.
func writeFileAtomic(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := createAdoptTemp(dir, ".pyxis-adopt-*")
	if err != nil {
		return fmt.Errorf("create tmp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName) // best-effort: an abandoned tmp is scratch, never content
		}
	}()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close() // the write error governs; the tmp is removed by the deferred cleanup
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := chmodAdoptTemp(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	tmpName = ""
	return nil
}
