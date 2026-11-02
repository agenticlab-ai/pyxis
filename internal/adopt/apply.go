package adopt

// Apply is the `adopt` verb's write (design existing-repo-onboarding-v2 §3.1,
// §3.3, §3.4; plan existing-repo-onboarding/001 T8): it materialises an
// approved adoption plan and nothing else. It refuses a plan absent or stale
// relative to the tree (prd:existing-repo-onboarding/UB-2), else drives
// converge's write primitives (T1) for the engine-verbatim and generated
// classes, applies the additive collision resolutions (T5 hook chain, T6
// workflow redirect; a collide-manual root file is left untouched, T7),
// records the install epoch as the repository's current commit WITHOUT creating
// a commit of its own (DINV-4; the operator commits the adoption), and stamps
// the plan applied — populating each added path with the content hash undo's
// modification test compares against.
//
// Three invariants govern it. **Confinement** (FR-3, ABUSE-1): apply writes
// only manifest paths the plan names, plus the collision resolutions the plan
// and the resolvers settle — never a path in the never-touch complement.
// **Idempotency** (FR-11): the write set is the CURRENT classification's
// divergence, which shrinks to empty once the tree is converged, so a re-apply
// lands only drift. **Fail-closed refusal** (UB-2): every refusal — no plan, a
// stale plan, an unparseable prior config — is named and precedes any write, so
// a refused apply changes nothing in the tree.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// configRelPath is the repo-owned config surface adoption establishes and
// records the epoch into — the same path the installer owns (installer's
// configRel), duplicated here because adopt cannot import the installer's
// unexported const and the value is a stable part of the layout.
const configRelPath = ".pyxis/config.yaml"

// ErrNoPlan is UB-2's absent-plan refusal: apply was run with no drafted plan
// at PlanPath. It names the remedy (draft one with `adopt`); branch with
// errors.Is.
var ErrNoPlan = errors.New("adopt: no approved plan at " + PlanPath + " — run the adopt verb to inspect the repository and draft a plan first (nothing was changed)")

// ErrStalePlan is UB-2's stale-plan refusal: the tree diverged from the plan
// since it was drafted, so materialising it would act on a repository the
// reviewer never saw. It names the remedy (redraft); branch with errors.Is.
var ErrStalePlan = errors.New("adopt: the plan is stale relative to the tree — the repository changed since it was drafted; re-run the adopt verb to redraft it (nothing was changed)")

// ApplyReport is Apply's result: the applied plan (its rewritten Applied form),
// the write/refusal sets converge's primitives returned, whether a prior hook
// path was chained, and the recorded epoch commit.
type ApplyReport struct {
	Plan        Plan
	PlanPath    string
	Actions     []converge.Action
	Refusals    []converge.Refusal
	Chained     bool
	EpochCommit string
}

// Apply materialises the approved plan at PlanPath against opts.Target. opts
// carries the same pin/source operand Inspect takes (Apply resolves the release
// manifest and archive exactly as classify does). It fails closed on an absent
// or stale plan and on an unparseable prior config (UB-7), each before any
// write. On success the plan at PlanPath is rewritten Applied with the epoch,
// the chained-hook baseline, and the added-path baselines populated.
func Apply(ctx context.Context, opts Options) (ApplyReport, error) {
	if err := ctx.Err(); err != nil {
		return ApplyReport{}, fmt.Errorf("adopt: %w", err)
	}
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		return ApplyReport{}, errors.New("adopt: Options.Target is empty (the target directory is required)")
	}

	// 1. Load the plan — absent is UB-2's named refusal, before anything else.
	planAbs := filepath.Join(target, filepath.FromSlash(PlanPath))
	planBytes, err := os.ReadFile(planAbs)
	if errors.Is(err, fs.ErrNotExist) {
		return ApplyReport{}, ErrNoPlan
	}
	if err != nil {
		return ApplyReport{}, fmt.Errorf("adopt: read plan %s: %w", PlanPath, err)
	}
	plan, err := Parse(planBytes)
	if err != nil {
		return ApplyReport{}, fmt.Errorf("adopt: %w", err)
	}

	// 2. Build the release-mode config classification resolves against — the
	// same overrides Inspect applies (a never-adopted tree's default mode is
	// "source", which converge's guard refuses). A prior config that fails the
	// closed parse is UB-7's named refusal, before any write.
	repo, defects := config.Load(target)
	if len(defects) > 0 {
		return ApplyReport{}, &configSurfaceError{defects: defects}
	}
	if err := requirePin(opts.Pin); err != nil {
		return ApplyReport{}, err
	}
	repo.System.Mode = "release"
	repo.System.Version = opts.Pin.Version
	repo.SystemVersion = opts.Pin.Version
	repo.System.ManifestSHA256 = opts.Pin.ManifestSHA256
	if s := strings.TrimSpace(opts.ArtifactSource); s != "" {
		repo.System.ArtifactSource = s
	}
	cfg := resolve.Config{Root: target, Repo: repo, Logger: opts.Logger, Fetcher: opts.Fetcher}

	// 3. Classify WITH the verified archive (the write-driving classification,
	// not Drift's read-only face): a generated surface whose template the tree
	// lacks renders from the archive here, so a fresh adoption materialises
	// every managed path in one pass (§3.1). This warms the gitignored resolve
	// cache but writes nothing to the tree, so a stale refusal below still
	// leaves the tree untouched.
	cls, archive, err := converge.ClassifyWithArchive(ctx, cfg)
	if err != nil {
		return ApplyReport{}, fmt.Errorf("adopt: %w", err)
	}
	// Recheck repository-owned refusals against the live tree, even when a
	// prior plan was drafted while authority was healthy. Missing authority
	// is operator-owned policy, never an ordinary ClassAdd surface.
	if _, err := entriesFromFindings(cls.Findings); err != nil {
		return ApplyReport{}, err
	}

	// The plan's named paths, and whether the workflow entry was redirected to
	// its distinct collision path (T6 records this at plan time). A redirected
	// plan carries the engine's workflow at DistinctWorkflowPath, never at the
	// manifest's fixed target — so the fixed target is the adopter's own file,
	// which apply must never overwrite.
	named := make(map[string]bool, len(plan.Entries))
	for _, e := range plan.Entries {
		named[e.Path] = true
	}
	_, workflowTarget, err := payload.WorkflowForRoot(target)
	if err != nil {
		return ApplyReport{}, err
	}
	distinctWorkflow := DistinctWorkflowPath(workflowTarget)
	redirected := named[distinctWorkflow]

	// 4. Stale check (UB-2), before any write: every manifest path that is
	// divergent NOW and writable (engine-verbatim/generated) must be named by
	// the plan — a divergent path the plan does not name appeared since the
	// draft, so the reviewer never approved acting on it. The redirected
	// workflow's fixed manifest target is covered by the plan's distinct-path
	// entry, not by its own name, so it is exempt. This is re-apply-safe: a
	// converged path drops out of the divergence set, so a re-apply's set is a
	// subset of the plan's named set and never reads as stale.
	for _, f := range cls.Findings {
		if f.Class != relmanifest.EngineVerbatim && f.Class != relmanifest.Generated {
			continue
		}
		if redirected && f.Path == workflowTarget {
			continue
		}
		if !named[f.Path] {
			return ApplyReport{}, fmt.Errorf("%w: a divergent %s path %q is not named by the plan", ErrStalePlan, f.Class, f.Path)
		}
	}

	report := ApplyReport{Plan: plan, PlanPath: PlanPath}

	// 5. Drive the per-class writes from the current divergence set. The
	// generated batch excludes the redirected workflow's fixed target (that
	// finding is the adopter's own file, landed elsewhere below); every other
	// finding is a manifest path the plan named (the stale check just proved
	// it).
	var evFindings, genFindings []converge.Finding
	for _, f := range cls.Findings {
		switch f.Class {
		case relmanifest.EngineVerbatim:
			evFindings = append(evFindings, f)
		case relmanifest.Generated:
			if redirected && f.Path == workflowTarget {
				continue
			}
			genFindings = append(genFindings, f)
		}
	}
	acts, refs, err := converge.RepairEngineVerbatim(ctx, target, evFindings, archive)
	report.Actions, report.Refusals = append(report.Actions, acts...), append(report.Refusals, refs...)
	if err != nil {
		return report, fmt.Errorf("adopt: apply engine-verbatim: %w", err)
	}
	acts, refs, err = converge.WriteGenerated(ctx, target, genFindings, cls.Generated)
	report.Actions, report.Refusals = append(report.Actions, acts...), append(report.Refusals, refs...)
	if err != nil {
		return report, fmt.Errorf("adopt: apply generated: %w", err)
	}

	// The redirected workflow lands the engine's rendered bytes (keyed in the
	// classification by the fixed manifest target) at the distinct collision
	// path — but only when they differ, so a re-apply of an up-to-date distinct
	// file is a no-op (the distinct path is not a manifest path, so the
	// divergence scan cannot report on it; apply guards idempotency directly).
	if redirected {
		desired := cls.Generated[workflowTarget]
		if desired == nil {
			return report, fmt.Errorf("adopt: apply workflow: the classification rendered no bytes for %s (the release is defective)", workflowTarget)
		}
		if !fileHasBytes(target, distinctWorkflow, desired) {
			acts, refs, err = ApplyWorkflowEntry(ctx, target, Entry{Path: distinctWorkflow, Class: ClassGenerated}, desired)
			report.Actions, report.Refusals = append(report.Actions, acts...), append(report.Refusals, refs...)
			if err != nil {
				return report, fmt.Errorf("adopt: apply workflow: %w", err)
			}
		}
	}

	// 6. Chain the engine's hooks (T5). Called unconditionally: with no prior
	// hook path it is the ordinary activation (core.hooksPath → the engine's
	// directory, prior recorded as ""); with a prior it chains and records it;
	// already chained (a re-apply) it is an idempotent no-op returning the
	// recorded baseline unchanged. The engine's pre-push shim exists by now —
	// the engine-verbatim writes above materialised it — so activation is
	// valid.
	hooks, err := ApplyHookChain(ctx, target, plan.Hooks)
	if err != nil {
		return report, fmt.Errorf("adopt: apply hook chain: %w", err)
	}
	report.Chained = hooks.PriorHooksPath != ""

	// 7. Record the install epoch as the repository's current commit, creating
	// no commit (§3.4, DINV-4). An existing config surface takes a targeted
	// epoch.commit edit that preserves the adopter's other config (idempotent
	// on re-apply); an absent one is established at the compiled defaults with
	// the epoch baked in (adoption's ClassAdd for the config surface).
	head, err := gitOutput(ctx, target, "rev-parse", "HEAD")
	if err != nil {
		return report, fmt.Errorf("adopt: resolve the current commit (a committed repository is required to record the epoch): %w", err)
	}
	head = strings.TrimSpace(head)
	report.EpochCommit = head

	configSHA := ""
	if _, statErr := os.Stat(filepath.Join(target, filepath.FromSlash(configRelPath))); statErr == nil {
		if err := config.SetEpochCommit(target, head); err != nil {
			return report, err
		}
		if configSHA, err = hashTreeFile(target, configRelPath); err != nil {
			return report, err
		}
	} else if errors.Is(statErr, fs.ErrNotExist) {
		if configSHA, err = installer.WriteAdoptionConfig(target, opts.Pin, opts.ArtifactSource, head); err != nil {
			return report, err
		}
	} else {
		return report, fmt.Errorf("adopt: probe %s: %w", configRelPath, statErr)
	}

	// 8. Stamp the plan applied: the epoch, the chained-hook baseline, and a
	// content baseline for every add/chain entry (DINV-7 — the exact set undo
	// deletes; the config surface's baseline is the one just computed, other
	// added paths hash their materialised bytes).
	added, err := addedBaselines(target, plan.Entries, configSHA)
	if err != nil {
		return report, err
	}
	plan.Status = StatusApplied
	plan.Epoch = head
	plan.Hooks = hooks
	plan.AddedPaths = added

	out, err := plan.Encode()
	if err != nil {
		return report, fmt.Errorf("adopt: encode applied plan: %w", err)
	}
	if err := writeFileAtomic(planAbs, out); err != nil {
		return report, fmt.Errorf("adopt: write applied plan: %w", err)
	}
	report.Plan = plan
	return report, nil
}

// addedBaselines computes the DINV-7 undo baseline for every `add` entry: the
// sha256 of the materialised path. The config surface reuses the hash apply
// already computed (configSHA, "" when the config is not an add entry); every
// other add path is hashed from the tree. A path an add entry names but that is
// absent after apply is a named error — apply materialises every path it
// stamps, so an absent one is a defect, not a silent empty baseline. A `chain`
// entry carries NO baseline (DINV-7 as reconciled 2026-07-26): the git-config
// chain is reversed via hooks.prior_hooks_path, not by deleting a path.
func addedBaselines(target string, entries []Entry, configSHA string) ([]AddedPath, error) {
	var added []AddedPath
	for _, e := range entries {
		if e.Class != ClassAdd {
			continue
		}
		sum := ""
		if e.Path == configRelPath && configSHA != "" {
			sum = configSHA
		} else {
			h, err := hashTreeFile(target, e.Path)
			if err != nil {
				return nil, fmt.Errorf("adopt: baseline for added path %q: %w", e.Path, err)
			}
			sum = h
		}
		added = append(added, AddedPath{Path: e.Path, SHA256: sum})
	}
	return added, nil
}

// fileHasBytes reports whether root/rel exists and its bytes equal want — the
// idempotency guard for a write to a non-manifest path the divergence scan
// cannot classify.
func fileHasBytes(root, rel string, want []byte) bool {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && bytes.Equal(b, want)
}

// hashTreeFile returns the lowercase sha256 hex of root/rel — the AddedPath
// baseline spelling (plan.checkHex), computed from the materialised tree bytes.
func hashTreeFile(root, rel string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", fmt.Errorf("adopt: read %s: %w", rel, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
