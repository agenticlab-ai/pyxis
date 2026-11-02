package adopt_test

// Hook-path collision resolution tests (existing-repo-onboarding/001 T5;
// design existing-repo-onboarding-v2 §3.3/§3.5, A-4): focused unit tests over
// ResolveHookChain / ApplyHookChain / UndoHookChain against throwaway git
// repos — the same isolation shape as the workflow (T6) and root-convention
// (T7) resolution tests.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
)

// gitRepo makes a throwaway git repo with the given core.hooksPath ("" leaves
// it unset) and returns its root.
func gitRepo(t *testing.T, hooksPath string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	if hooksPath != "" {
		git(t, root, "config", "--local", "core.hooksPath", hooksPath)
	}
	return root
}

func gitGet(t *testing.T, root, key string) (string, bool) {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "config", "--local", "--get", key).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// ---- ResolveHookChain -------------------------------------------------------

func TestResolveHookChainDetectsExistingPath(t *testing.T) {
	root := gitRepo(t, ".husky")
	entry, ok, err := adopt.ResolveHookChain(context.Background(), root)
	if err != nil || !ok {
		t.Fatalf("ResolveHookChain: ok=%v err=%v, want a chain entry", ok, err)
	}
	if entry.Class != adopt.ClassChain {
		t.Errorf("class = %q, want %q", entry.Class, adopt.ClassChain)
	}
	if !strings.Contains(entry.Detail, ".husky") {
		t.Errorf("Detail does not name the prior path .husky: %q", entry.Detail)
	}
}

func TestResolveHookChainNoPathNoChain(t *testing.T) {
	root := gitRepo(t, "") // no core.hooksPath set
	_, ok, err := adopt.ResolveHookChain(context.Background(), root)
	if err != nil {
		t.Fatalf("ResolveHookChain: %v", err)
	}
	if ok {
		t.Error("ok=true for a repo with no core.hooksPath; want no chain (ordinary activation applies)")
	}
}

func TestResolveHookChainAlreadyEngineNoChain(t *testing.T) {
	root := gitRepo(t, ".githooks") // already the engine's own directory
	_, ok, err := adopt.ResolveHookChain(context.Background(), root)
	if err != nil {
		t.Fatalf("ResolveHookChain: %v", err)
	}
	if ok {
		t.Error("ok=true for an already-chained repo (core.hooksPath=.githooks); want no fresh chain (re-adopt, UB-4)")
	}
}

// ---- ApplyHookChain ---------------------------------------------------------

func TestApplyHookChainRecordsPriorChainsAndLeavesHookByteIdentical(t *testing.T) {
	root := gitRepo(t, ".husky")
	// A pre-existing adopter hook the engine must not touch (A-4).
	huskyHook := filepath.Join(root, ".husky", "pre-push")
	if err := os.MkdirAll(filepath.Dir(huskyHook), 0o755); err != nil {
		t.Fatal(err)
	}
	const hookBody = "#!/bin/sh\necho adopter-hook\n"
	if err := os.WriteFile(huskyHook, []byte(hookBody), 0o755); err != nil {
		t.Fatal(err)
	}

	hooks, err := adopt.ApplyHookChain(context.Background(), root, adopt.Hooks{})
	if err != nil {
		t.Fatalf("ApplyHookChain: %v", err)
	}
	if hooks.PriorHooksPath != ".husky" {
		t.Errorf("recorded prior = %q, want %q", hooks.PriorHooksPath, ".husky")
	}
	if got, _ := gitGet(t, root, "core.hooksPath"); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want %q (engine chained ahead)", got, ".githooks")
	}
	if got, _ := gitGet(t, root, "pyxis.chainedhookspath"); got != ".husky" {
		t.Errorf("chain marker = %q, want %q (the shim's delegation pointer)", got, ".husky")
	}
	// The adopter's hook is byte-identical — the engine wrote no file into it (FR-4).
	if b, _ := os.ReadFile(huskyHook); string(b) != hookBody {
		t.Errorf("adopter hook mutated: %q, want %q", b, hookBody)
	}
}

func TestApplyHookChainIdempotentNoOpWhenAlreadyChained(t *testing.T) {
	// Simulate an already-applied repo: core.hooksPath is the engine's dir and
	// the first apply recorded a prior of .husky (a re-apply, FR-11).
	root := gitRepo(t, ".githooks")
	git(t, root, "config", "--local", "pyxis.chainedhookspath", ".husky")
	recorded := adopt.Hooks{PriorHooksPath: ".husky"}

	got, err := adopt.ApplyHookChain(context.Background(), root, recorded)
	if err != nil {
		t.Fatalf("ApplyHookChain (re-apply): %v", err)
	}
	if got.PriorHooksPath != ".husky" {
		t.Errorf("re-apply overwrote the recorded prior: got %q, want %q — never the engine's own dir", got.PriorHooksPath, ".husky")
	}
	// The marker must not have been rewritten to .githooks either.
	if m, _ := gitGet(t, root, "pyxis.chainedhookspath"); m != ".husky" {
		t.Errorf("re-apply clobbered the chain marker: %q, want %q", m, ".husky")
	}
}

// ---- UndoHookChain ----------------------------------------------------------

func TestUndoHookChainRestoresPrior(t *testing.T) {
	root := gitRepo(t, ".husky")
	hooks, err := adopt.ApplyHookChain(context.Background(), root, adopt.Hooks{})
	if err != nil {
		t.Fatalf("ApplyHookChain: %v", err)
	}
	if err := adopt.UndoHookChain(context.Background(), root, hooks); err != nil {
		t.Fatalf("UndoHookChain: %v", err)
	}
	if got, _ := gitGet(t, root, "core.hooksPath"); got != ".husky" {
		t.Errorf("core.hooksPath after undo = %q, want the restored prior %q", got, ".husky")
	}
	if _, set := gitGet(t, root, "pyxis.chainedhookspath"); set {
		t.Error("chain marker still set after undo; want it cleared")
	}
}

func TestUndoHookChainUnsetsWhenNoPrior(t *testing.T) {
	// A repo adoption chained from no prior hook path: undo unsets core.hooksPath.
	root := gitRepo(t, ".githooks")
	git(t, root, "config", "--local", "pyxis.chainedhookspath", "")
	if err := adopt.UndoHookChain(context.Background(), root, adopt.Hooks{PriorHooksPath: ""}); err != nil {
		t.Fatalf("UndoHookChain: %v", err)
	}
	if got, set := gitGet(t, root, "core.hooksPath"); set {
		t.Errorf("core.hooksPath = %q still set after undo with no prior; want unset", got)
	}
}
