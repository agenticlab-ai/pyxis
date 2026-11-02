package adopt

// Undo is the `adopt` verb's reversal (design existing-repo-onboarding-v2 §3.5;
// plan existing-repo-onboarding/001 T10): it removes the installation an applied
// plan recorded and preserves the operator's post-adoption work. It unchains the
// hook (restoring the prior path), deletes the managed paths that are still
// pristine, clears the install epoch, names the plan's collide-manual entries
// for the human to reverse, and removes the plan record itself — leaving every
// post-adoption commit and edit in place.
//
// It FAILS CLOSED on operator edits (DINV-5, prd:existing-repo-onboarding/UB-6):
// a managed path the operator has modified since adoption is preserved and
// named, never deleted. "Modified since adoption" is decided PER OWNERSHIP CLASS
// against the baseline apply captured (DINV-7): an engine-verbatim path against
// its manifest SHA (a divergent classification finding = edited), a generated
// path by regenerate-and-compare, an add/chain path against the sha256 stamped
// into its added_paths entry. A hash matching the baseline is pristine (delete);
// a mismatch is preserved-and-named. On an immediate round trip with no
// divergence the whole tree returns byte-identical to its pre-adoption state
// (A-7) — proven by git's tree SHA in the harness (T11).

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// ErrNoAppliedPlan is undo's refusal when no applied plan is present to reverse:
// PlanPath is absent, or it is still a draft that no apply materialized. Branch
// with errors.Is.
var ErrNoAppliedPlan = errors.New("adopt: no applied plan at " + PlanPath + " to undo — undo reverses an adoption `apply` performed (nothing was changed)")

// UndoReport is Undo's result: the managed paths removed, the paths preserved
// because they are operator-owned authority or the operator modified them
// since adoption (named, never deleted), the
// collide-manual entries named for the human to reverse by hand, and whether a
// prior hook path was restored.
type UndoReport struct {
	Removed       []string
	Preserved     []string
	ManualEntries []Entry
	Unchained     bool
}

// Undo reverses the applied plan at PlanPath against opts.Target. opts carries
// the same pin/source operand apply took — undo re-derives the ownership
// classification to decide, per class, which managed paths are still pristine
// (safe to delete) and which the operator has since edited (preserve and name).
// It refuses an absent or not-yet-applied plan, and an unparseable prior config
// (UB-7), each before any change.
func Undo(ctx context.Context, opts Options) (UndoReport, error) {
	if err := ctx.Err(); err != nil {
		return UndoReport{}, fmt.Errorf("adopt: %w", err)
	}
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		return UndoReport{}, errors.New("adopt: Options.Target is empty (the target directory is required)")
	}

	// 1. Load the applied plan — absent or still-draft is a named refusal.
	planAbs := filepath.Join(target, filepath.FromSlash(PlanPath))
	planBytes, err := os.ReadFile(planAbs)
	if errors.Is(err, fs.ErrNotExist) {
		return UndoReport{}, ErrNoAppliedPlan
	}
	if err != nil {
		return UndoReport{}, fmt.Errorf("adopt: read plan %s: %w", PlanPath, err)
	}
	plan, err := Parse(planBytes)
	if err != nil {
		return UndoReport{}, fmt.Errorf("adopt: %w", err)
	}
	if plan.Status != StatusApplied {
		return UndoReport{}, fmt.Errorf("%w: the plan is %q, not applied", ErrNoAppliedPlan, plan.Status)
	}

	// 2. Build the release-mode config the classification resolves against — the
	// same overrides apply applies. A prior config that fails the closed parse is
	// UB-7's named refusal, before any change.
	repo, defects := config.Load(target)
	if len(defects) > 0 {
		return UndoReport{}, &configSurfaceError{defects: defects}
	}
	if err := requirePin(opts.Pin); err != nil {
		return UndoReport{}, err
	}
	repo.System.Mode = "release"
	repo.System.Version = opts.Pin.Version
	repo.SystemVersion = opts.Pin.Version
	repo.System.ManifestSHA256 = opts.Pin.ManifestSHA256
	if s := strings.TrimSpace(opts.ArtifactSource); s != "" {
		repo.System.ArtifactSource = s
	}
	cfg := resolve.Config{Root: target, Repo: repo, Logger: opts.Logger, Fetcher: opts.Fetcher}

	// 3. Re-derive the ownership classification (the per-class modified-since-
	// adoption baseline for engine-verbatim and generated paths): a divergent
	// finding means the operator edited the path since adoption.
	cls, _, err := converge.ClassifyWithArchive(ctx, cfg)
	if err != nil {
		return UndoReport{}, fmt.Errorf("adopt: %w", err)
	}
	findingByPath := make(map[string]converge.Finding, len(cls.Findings))
	for _, f := range cls.Findings {
		findingByPath[f.Path] = f
	}
	baseline := make(map[string]string, len(plan.AddedPaths))
	for _, a := range plan.AddedPaths {
		baseline[a.Path] = a.SHA256
	}
	_, workflowTarget, err := payload.WorkflowForRoot(target)
	if err != nil {
		return UndoReport{}, err
	}
	distinctWorkflow := DistinctWorkflowPath(workflowTarget)
	redirected := false
	for _, e := range plan.Entries {
		if e.Class == ClassGenerated && e.Path == distinctWorkflow {
			redirected = true
			break
		}
	}

	var report UndoReport

	// 4. The per-class removal pass: delete a pristine managed path, preserve and
	// name a modified one, and never touch a never-touch or collide-manual path.
	for _, e := range plan.Entries {
		// Old plans may still label live authority as engine-managed. Its
		// ownership does not depend on that historical label, and undo never
		// deletes repository policy. Preserve the object or its absence.
		if e.Path == payload.AuthorityContractTarget {
			if _, err := os.Lstat(filepath.Join(target, filepath.FromSlash(e.Path))); !errors.Is(err, fs.ErrNotExist) {
				report.Preserved = append(report.Preserved, e.Path)
			}
			continue
		}
		switch e.Class {
		case ClassEngineVerbatim:
			// A divergent finding = edited (StateModified) or already gone
			// (StateMissing); no finding = pristine (matches the manifest SHA).
			f, diverged := findingByPath[e.Path]
			switch {
			case diverged && f.State == converge.StateMissing:
				// already absent — nothing to remove
			case diverged:
				report.Preserved = append(report.Preserved, e.Path)
			default:
				if err := report.remove(target, e.Path); err != nil {
					return report, err
				}
			}
		case ClassGenerated:
			if redirected && e.Path == distinctWorkflow {
				// The distinct workflow path is not a manifest path, so the
				// classification cannot report on it: compare directly against the
				// engine's rendered bytes (keyed by the fixed manifest target).
				if err := report.removeIfPristine(target, e.Path, cls.Generated[workflowTarget]); err != nil {
					return report, err
				}
				continue
			}
			f, diverged := findingByPath[e.Path]
			switch {
			case diverged && f.State == converge.StateMissing:
				// already absent
			case diverged:
				report.Preserved = append(report.Preserved, e.Path)
			default:
				if err := report.remove(target, e.Path); err != nil {
					return report, err
				}
			}
		case ClassAdd:
			// An add path carries no manifest entry and no generator, so it is
			// judged against the sha256 apply stamped into added_paths; a missing
			// baseline fails closed to preserve.
			abs := filepath.Join(target, filepath.FromSlash(e.Path))
			if _, statErr := os.Stat(abs); errors.Is(statErr, fs.ErrNotExist) {
				continue
			} else if statErr != nil {
				return report, fmt.Errorf("adopt: undo probe %s: %w", e.Path, statErr)
			}
			cur, err := hashTreeFile(target, e.Path)
			if err != nil {
				return report, err
			}
			if base, ok := baseline[e.Path]; ok && cur == base {
				if err := report.remove(target, e.Path); err != nil {
					return report, err
				}
			} else {
				report.Preserved = append(report.Preserved, e.Path)
			}
		case ClassChain:
			// The chain is git-config-only (no wrapper file); undo reverses it via
			// hooks.prior_hooks_path (UndoHookChain, below), never by deleting a
			// path — the entry is disclosure only (DINV-7 as reconciled 2026-07-26).
		case ClassCollideManual:
			// Adoption never wrote this owned file, so undo cannot reverse it —
			// it names the entry for the human to reverse by hand (symmetric to
			// adopt emitting it; design §3.5).
			report.ManualEntries = append(report.ManualEntries, e)
		case ClassNeverTouch:
			// outside adoption's write confinement — never touched
		}
	}

	// 5. Unchain the hook: restore the prior core.hooksPath (or unset it when
	// adoption chained from none) and clear the runtime chain marker (T5).
	if err := UndoHookChain(ctx, target, plan.Hooks); err != nil {
		return report, err
	}
	report.Unchained = true

	// 6. Clear the install epoch. When the config surface was an added path it
	// is already gone (removed above, taking the epoch with it); when it survives
	// — preserved, or present before adoption and only stamped with the epoch —
	// the epoch.commit apply wrote is cleared surgically so grandfathering stops,
	// leaving the operator's other config intact.
	if _, statErr := os.Stat(filepath.Join(target, filepath.FromSlash(configRelPath))); statErr == nil {
		if err := config.ClearEpochCommit(target); err != nil {
			return report, err
		}
	}

	// 7. Remove the plan record itself — the last of adoption's own footprint.
	if err := os.Remove(planAbs); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return report, fmt.Errorf("adopt: undo remove plan %s: %w", PlanPath, err)
	}
	removeEmptyParents(target, PlanPath)

	return report, nil
}

// remove deletes root/rel (a no-op if already absent), records it in Removed,
// and prunes any parent directories the deletion left empty.
func (r *UndoReport) remove(target, rel string) error {
	abs := filepath.Join(target, filepath.FromSlash(rel))
	if err := os.Remove(abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("adopt: undo remove %s: %w", rel, err)
	}
	r.Removed = append(r.Removed, rel)
	removeEmptyParents(target, rel)
	return nil
}

// removeIfPristine deletes root/rel when its bytes equal want (pristine since
// adoption), else preserves and names it — the non-manifest generated path
// (the redirected workflow) whose baseline is the engine's rendered bytes.
func (r *UndoReport) removeIfPristine(target, rel string, want []byte) error {
	abs := filepath.Join(target, filepath.FromSlash(rel))
	if _, err := os.Stat(abs); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if want != nil && fileHasBytes(target, rel, want) {
		return r.remove(target, rel)
	}
	r.Preserved = append(r.Preserved, rel)
	return nil
}

// removeEmptyParents prunes now-empty ancestor directories of rel up to (but not
// including) target — best effort, so a leftover empty directory is never an
// error, and a non-empty ancestor stops the walk. It keeps the working tree
// tidy after a removal; git tree-identity (A-7) is unaffected either way, since
// git tracks no empty directories.
func removeEmptyParents(target, rel string) {
	dir := filepath.Dir(filepath.FromSlash(rel))
	for dir != "." && dir != "" && dir != string(filepath.Separator) {
		abs := filepath.Join(target, dir)
		if err := os.Remove(abs); err != nil {
			return // non-empty or gone — stop
		}
		dir = filepath.Dir(dir)
	}
}
