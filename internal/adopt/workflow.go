package adopt

// Workflow-filename collision resolution (design existing-repo-onboarding-v2
// §3.3, plan existing-repo-onboarding/001 T6): `.github/workflows/` is fixed
// by the CI host and additive by filename — any recognized file there runs,
// and GitHub matches a required status check by the JOB ID inside the file
// (internal/wire.RequiredCheckContext = "docs-gate"), never by the file's
// name — so a filename collision resolves additive-adjacent (design §2,
// §3.3): the engine's workflow lands under a distinct, non-colliding
// filename instead of overwriting whatever the adopter already committed at
// the fixed target (prd:existing-repo-onboarding/FR-5). Content rendering
// stays phase-1's own mechanism (payload.RenderInstalledWorkflow,
// converge.WriteGenerated); this file decides only WHERE the workflow
// lands — at plan time (ResolveWorkflowEntries, redirecting the one entry
// classify.go's entriesFromFindings already carries through regardless of
// state) and at apply time (ApplyWorkflowEntry, reusing T1's generated-write
// primitive unchanged).

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// distinctWorkflowPrefix disambiguates the engine's workflow from whatever
// the adopter already committed at the fixed target: a fixed, deterministic
// choice — never a counter or a timestamp — so classifying the same
// repository again (a re-adopt, UB-4) recomputes the identical path rather
// than a fresh one each time, and a re-apply (FR-11) lands on the same file.
const distinctWorkflowPrefix = "pyxis-"

// DistinctWorkflowPath is the non-colliding path the engine's installed
// workflow lands under when its default target already carries a file: the
// same directory, with distinctWorkflowPrefix on the base filename (design
// §3.3).
func DistinctWorkflowPath(targets ...string) string {
	target := payload.InstalledWorkflowTarget
	if len(targets) > 0 {
		target = targets[0]
	}
	dir, base := path.Dir(target), path.Base(target)
	return path.Join(dir, distinctWorkflowPrefix+base)
}

// ResolveWorkflowEntries redirects entries' installed-workflow entry — the
// one entriesFromFindings already carries through at the manifest's fixed
// target, regardless of classification state (classify.go) — to
// DistinctWorkflowPath when that target already names a file on disk: the
// adopter's own pre-existing workflow, since adoption never writes there
// (this pass is the first read of that fact, and apply, T8, only ever
// touches a path an entry names — design §3.1). Absent a collision, entries
// is returned unchanged: the ordinary generated-class path (T3's baseline)
// already covers that case. A manifest that does not list the
// installed-workflow path at all (unreachable for any coherent release, but
// never assumed here) leaves entries untouched too — this function
// redirects an entry the classifier produced, it never invents one of its
// own.
func ResolveWorkflowEntries(root string, entries []Entry) ([]Entry, error) {
	_, target, err := payload.WorkflowForRoot(root)
	if err != nil {
		return nil, err
	}
	for i, e := range entries {
		if e.Class != ClassGenerated || e.Path != target {
			continue
		}
		collided, err := fileExists(root, target)
		if err != nil {
			return nil, err
		}
		if !collided {
			return entries, nil
		}
		entries[i] = Entry{
			Path:  DistinctWorkflowPath(target),
			Class: ClassGenerated,
			Detail: fmt.Sprintf(
				"the adopter's %s already exists; the engine's workflow lands here instead, and %s is left untouched (design existing-repo-onboarding-v2 §3.3)",
				target, target),
		}
		return entries, nil
	}
	return entries, nil
}

// fileExists reports whether root/rel already names a file or directory on
// disk — the collision predicate: landing generated content at the
// manifest's fixed target would silently discard whatever is already there,
// however it got there (tracked or not — DINV-1 is about the path, not its
// git status).
func fileExists(root, rel string) (bool, error) {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, fmt.Errorf("adopt: probe %s: %w", rel, err)
	}
}

// ApplyWorkflowEntry lands the installed workflow's rendered bytes at e.Path
// — the fixed target or its distinct collision fallback, exactly as classify
// recorded it in the plan — by driving converge's existing generated-write
// primitive (T1) unchanged: this package decides only WHERE the workflow
// lands, never HOW its bytes render (phase-1's own generator, out of scope
// here — desired is the caller's, obtained the same way any other
// generated-class write already does). e must be class ClassGenerated; any
// other class is a programmer error, named here rather than silently
// ignored (converge.WriteGenerated skips a non-generated finding without
// complaint).
func ApplyWorkflowEntry(ctx context.Context, root string, e Entry, desired []byte) ([]converge.Action, []converge.Refusal, error) {
	if e.Class != ClassGenerated {
		return nil, nil, fmt.Errorf("adopt: ApplyWorkflowEntry: entry %q is class %s, want %s", e.Path, e.Class, ClassGenerated)
	}
	finding := converge.Finding{Path: e.Path, Class: relmanifest.Generated}
	return converge.WriteGenerated(ctx, root, []converge.Finding{finding}, map[string][]byte{e.Path: desired})
}
