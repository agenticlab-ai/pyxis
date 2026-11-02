package adopt

// The hook-path collision resolution (design existing-repo-onboarding-v2
// §3.3, §3.5; prd:existing-repo-onboarding/FR-4, NFR-3): git honours a single
// `core.hooksPath`, so a repository that already sets one (a hook manager
// like husky's `.husky`, or its own `.githooks`) cannot simply be
// overwritten — that would silently disable the adopter's hooks. Adoption
// instead **chains forward**: it points `core.hooksPath` at the engine's own
// hook directory (engineHooksPath) whose pre-push shim, after the engine's
// checks pass, invokes the adopter's previously-configured hook (the shim
// reads chainMarkerConfig). The displaced path is recorded in the plan's
// Hooks.PriorHooksPath so undo restores it exactly — captured BEFORE the
// overwrite and idempotently, so a re-apply never records the engine's own
// directory as the prior path.
//
// This file is the resolution's logic, callable by the future apply (T8) and
// undo (T10) orchestrators and tested in isolation over a git fixture, the
// same shape as the workflow (T6) and root-convention (T7) resolutions. It
// decides only the hook-path chaining; the shim's forward-delegation body
// lives in the engine hook shim (.githooks/pre-push), driven by the marker
// this apply sets.

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// engineHooksPath is the engine's own hook directory — the value
// `git config core.hooksPath` activates for a pyxis-operated repo (the
// payload's hook-activation convention, mirrored by converge/doctor's
// hooksPathExpected across the package boundary). Adoption chains by pointing
// core.hooksPath here; a core.hooksPath already resolved here means the repo
// is already chained (the idempotency no-op).
const engineHooksPath = ".githooks"

// chainMarkerConfig is the git-config key `apply` sets to the displaced prior
// hook path when it chains — the runtime pointer the engine's pre-push shim
// reads to delegate forward (a bash-readable store distinct from the plan's
// Hooks.PriorHooksPath undo record; git lowercases config keys). It also lets
// doctor tell an adoption-chained repo whose core.hooksPath has since been
// displaced (a later hook-manager re-install, NFR-3) from an ordinary one.
const chainMarkerConfig = "pyxis.chainedhookspath"

// ResolveHookChain reports whether root already configures a `core.hooksPath`
// the engine must chain onto rather than overwrite. When it does — the path
// is set and is not already the engine's own directory — ok is true and the
// returned Entry is the ClassChain resolution naming the prior path in its
// Detail (apply captures the concrete value into the plan's Hooks; this pass
// only records that a chain is needed). ok is false when no hook path is set
// (nothing to chain — the ordinary activation applies) or it already resolves
// to the engine's directory (already chained: a re-adopt, UB-4, plans no
// fresh chain). Errors other than an unset key are named, never swallowed.
func ResolveHookChain(ctx context.Context, root string) (Entry, bool, error) {
	prior, set, err := gitConfigGet(ctx, root, "core.hooksPath")
	if err != nil {
		return Entry{}, false, err
	}
	// Unset, empty (a degenerate value, no hook manager to chain onto), or
	// already the engine's own directory (a re-adopt, UB-4) — no chain.
	if !set || prior == "" || prior == engineHooksPath {
		return Entry{}, false, nil
	}
	return Entry{
		Path:  "core.hooksPath",
		Class: ClassChain,
		Detail: fmt.Sprintf(
			"the adopter's core.hooksPath is %q; the engine chains its checks ahead of it — apply points core.hooksPath at %s and its pre-push invokes %q after the engine's checks; undo restores %q (design existing-repo-onboarding-v2 §3.3)",
			prior, engineHooksPath, prior, prior),
	}, true, nil
}

// ApplyHookChain chains the engine's hooks in front of the adopter's, at
// apply time, and returns the Hooks undo baseline the plan records. It reads
// the current core.hooksPath and, when the repository is not already chained
// (core.hooksPath is not the engine's directory), records that prior value —
// BEFORE overwriting anything — then points core.hooksPath at the engine's
// directory and sets the runtime chain marker the shim delegates through.
// **Idempotent**: a core.hooksPath already resolved to the engine's directory
// (a re-apply, FR-11) is left untouched and the existing baseline returned
// unchanged, so re-apply never overwrites the recorded prior path with the
// engine's own directory (design §3.3, the Hooks contract). It writes no file
// into the adopter's hook-manager directory (FR-4) — only git config changes.
func ApplyHookChain(ctx context.Context, root string, current Hooks) (Hooks, error) {
	prior, set, err := gitConfigGet(ctx, root, "core.hooksPath")
	if err != nil {
		return Hooks{}, err
	}
	if set && prior == engineHooksPath {
		// Already chained — the idempotency no-op: keep the baseline the
		// first apply recorded, never re-capture the engine's own directory.
		return current, nil
	}
	// Record the prior path BEFORE the overwrite (design §3.3): the marker for
	// the shim's runtime delegation, then the hooks-path overwrite last.
	if err := gitConfigSet(ctx, root, chainMarkerConfig, prior); err != nil {
		return Hooks{}, err
	}
	if err := gitConfigSet(ctx, root, "core.hooksPath", engineHooksPath); err != nil {
		return Hooks{}, err
	}
	return Hooks{PriorHooksPath: prior}, nil
}

// UndoHookChain reverses ApplyHookChain: it restores the adopter's prior
// core.hooksPath from the plan's recorded baseline (unsetting core.hooksPath
// when the repository configured none before adoption, i.e. the prior path is
// empty) and clears the chain marker. A one-value restore — the whole of
// undo's hook work, since adoption wrote no file into the adopter's hook
// directory (design §3.5; prd:existing-repo-onboarding/FR-9).
func UndoHookChain(ctx context.Context, root string, hooks Hooks) error {
	if hooks.PriorHooksPath == "" {
		if err := gitConfigUnset(ctx, root, "core.hooksPath"); err != nil {
			return err
		}
	} else if err := gitConfigSet(ctx, root, "core.hooksPath", hooks.PriorHooksPath); err != nil {
		return err
	}
	return gitConfigUnset(ctx, root, chainMarkerConfig)
}

// gitConfigGet reads a local git-config value in root. set is false when the
// key is absent (git exit status 1 with empty output — not an error here);
// any other failure is named.
func gitConfigGet(ctx context.Context, root, key string) (value string, set bool, err error) {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "config", "--local", "--get", key).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 && len(out) == 0 {
			return "", false, nil // key not set
		}
		return "", false, fmt.Errorf("adopt: read git config %s in %s: %w", key, root, err)
	}
	return strings.TrimRight(string(out), "\n"), true, nil
}

// gitConfigSet writes a local git-config value in root.
func gitConfigSet(ctx context.Context, root, key, value string) error {
	if err := exec.CommandContext(ctx, "git", "-C", root, "config", "--local", key, value).Run(); err != nil {
		return fmt.Errorf("adopt: set git config %s=%q in %s: %w", key, value, root, err)
	}
	return nil
}

// gitConfigUnset removes a local git-config key in root; an already-absent key
// (git exit status 5) is not an error — unset is idempotent.
func gitConfigUnset(ctx context.Context, root, key string) error {
	err := exec.CommandContext(ctx, "git", "-C", root, "config", "--local", "--unset", key).Run()
	if err == nil {
		return nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 5 {
		return nil // key was not set
	}
	return fmt.Errorf("adopt: unset git config %s in %s: %w", key, root, err)
}
