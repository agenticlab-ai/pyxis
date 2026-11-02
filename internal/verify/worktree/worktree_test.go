package worktree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a hermetic git repo at a fresh temp dir with an isolated
// identity (mirrors the checks/diffset packages' own test helper of the same
// name), with one base commit so HEAD always resolves.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "codegate-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "codegate-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	gitOK(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	commit(t, dir, "base")
	return dir
}

func gitOK(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	gitOK(t, dir, "add", "-A")
	gitOK(t, dir, "commit", "-q", "-m", msg)
	return gitOK(t, dir, "rev-parse", "HEAD")
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return gitOK(t, dir, "rev-parse", "HEAD")
}

// TestCreate_ChecksOutHeadUnderScratch proves the worktree lands under the
// workspace's own scratch directory, at head content, registered with git.
func TestCreate_ChecksOutHeadUnderScratch(t *testing.T) {
	ws := initRepo(t)
	sha := headSHA(t, ws)

	wt, err := Create(context.Background(), ws, sha, "run-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer wt.Remove(context.Background())

	wantPrefix := filepath.Join(ws, ".codegate", "worktrees")
	if !strings.HasPrefix(wt.Path, wantPrefix) {
		t.Errorf("Path = %q, want a path under %q", wt.Path, wantPrefix)
	}
	data, err := os.ReadFile(filepath.Join(wt.Path, "main.go"))
	if err != nil {
		t.Fatalf("read checked-out file: %v", err)
	}
	if string(data) != "package main\n\nfunc main() {}\n" {
		t.Errorf("checked-out content = %q, want the head content", data)
	}

	list := gitOK(t, ws, "worktree", "list")
	if !strings.Contains(list, wt.Path) {
		t.Errorf("git worktree list = %q, want it to include %q", list, wt.Path)
	}
}

// TestCreate_DegenerateRunID proves a run id that would collapse to the
// scratch root itself (empty, ".", "..") is refused rather than silently
// creating a worktree at the wrong path.
func TestCreate_DegenerateRunID(t *testing.T) {
	ws := initRepo(t)
	sha := headSHA(t, ws)

	for _, id := range []string{"", ".", "..", "   "} {
		if _, err := Create(context.Background(), ws, sha, id); err == nil {
			t.Errorf("Create with run id %q: want an error, got nil", id)
		}
	}
}

// TestCreate_UnknownRevision proves a bad head sha surfaces as an error, not a
// silently-empty worktree.
func TestCreate_UnknownRevision(t *testing.T) {
	ws := initRepo(t)
	if _, err := Create(context.Background(), ws, "0000000000000000000000000000000000000000", "run-1"); err == nil {
		t.Error("Create with an unresolvable sha: want an error, got nil")
	}
}

// TestProvision_CopiesDeclaredStateDir proves a declared state dir becomes
// visible inside the worktree at the same relative path with the origin's
// content (a node_modules-class dependency a bare checkout lacks), AND that the
// provisioned copy is independent of the origin: a write through the worktree's
// copy must not reach the origin. The copy (never a symlink) is the security
// boundary — a symlink would let untrusted reverted-test code write back into
// the workspace under verification (a DINV-7 breach and a cross-leg channel).
func TestProvision_CopiesDeclaredStateDir(t *testing.T) {
	ws := initRepo(t)
	writeFile(t, ws, "vendor/dep/data.txt", "vendored content\n")
	sha := headSHA(t, ws) // untracked (not committed) — Provision reads the origin's filesystem, not its git content.

	wt, err := Create(context.Background(), ws, sha, "run-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer wt.Remove(context.Background())

	if err := wt.Provision(context.Background(), []string{"vendor/dep"}); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	copied := filepath.Join(wt.Path, "vendor/dep/data.txt")
	data, err := os.ReadFile(copied)
	if err != nil {
		t.Fatalf("read provisioned state dir: %v", err)
	}
	if string(data) != "vendored content\n" {
		t.Errorf("provisioned content = %q, want the origin's content", data)
	}

	// Write through the worktree's copy — the origin must be untouched (a
	// symlink would have leaked this straight back into the workspace).
	if err := os.WriteFile(copied, []byte("hostile write\n"), 0o644); err != nil {
		t.Fatalf("write through provisioned copy: %v", err)
	}
	origin, err := os.ReadFile(filepath.Join(ws, "vendor/dep/data.txt"))
	if err != nil {
		t.Fatalf("read origin state dir: %v", err)
	}
	if string(origin) != "vendored content\n" {
		t.Errorf("origin content = %q, want it unchanged — the copy is not isolating writes", origin)
	}
}

// TestProvision_MissingStateDirSkippedNotError proves a declared state dir
// absent from the origin workspace is a no-op, never a failure — a repo with
// no such state simply has nothing to provision.
func TestProvision_MissingStateDirSkippedNotError(t *testing.T) {
	ws := initRepo(t)
	sha := headSHA(t, ws)
	wt, err := Create(context.Background(), ws, sha, "run-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer wt.Remove(context.Background())

	if err := wt.Provision(context.Background(), []string{"node_modules", "does/not/exist"}); err != nil {
		t.Fatalf("Provision with an absent state dir: want nil, got %v", err)
	}
}

// TestRemove_CleansDirectoryAndRegistration proves Remove leaves both the
// directory and git's own worktree registration gone.
func TestRemove_CleansDirectoryAndRegistration(t *testing.T) {
	ws := initRepo(t)
	sha := headSHA(t, ws)
	wt, err := Create(context.Background(), ws, sha, "run-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := wt.Remove(context.Background()); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after Remove: err=%v", err)
	}
	list := gitOK(t, ws, "worktree", "list")
	if strings.Contains(list, wt.Path) {
		t.Errorf("git worktree list still references a removed worktree: %q", list)
	}
}

// TestRemove_DirectoryAlreadyGone proves Remove still clears git's
// registration even when the directory was hand-deleted out from under it —
// the fallback (manual removal + prune) must never leave a wedged entry.
func TestRemove_DirectoryAlreadyGone(t *testing.T) {
	ws := initRepo(t)
	sha := headSHA(t, ws)
	wt, err := Create(context.Background(), ws, sha, "run-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.RemoveAll(wt.Path); err != nil {
		t.Fatalf("pre-remove the worktree directory: %v", err)
	}

	if err := wt.Remove(context.Background()); err != nil {
		t.Fatalf("Remove after the directory was already gone: want nil, got %v", err)
	}
	list := gitOK(t, ws, "worktree", "list")
	if strings.Contains(list, wt.Path) {
		t.Errorf("git worktree list still references a removed worktree: %q", list)
	}
}

func TestPruneStaleWorktreesRemovesScratchEntries(t *testing.T) {
	ws := initRepo(t)
	root := filepath.Join(ws, filepath.FromSlash(ScratchRelPath))
	for _, name := range []string{"stale-a", "stale-b"} {
		writeFile(t, root, filepath.Join(name, "residue"), "x")
	}
	writeFile(t, root, "ignore.txt", "not a directory")
	if err := PruneStaleWorktrees(t.Context(), ws); err != nil {
		t.Fatalf("PruneStaleWorktrees: %v", err)
	}
	for _, name := range []string{"stale-a", "stale-b"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("stale entry %s remains: %v", name, err)
		}
	}
}

func TestPruneAndProvisionReportBoundaryFailures(t *testing.T) {
	t.Run("missing scratch is clean", func(t *testing.T) {
		if err := PruneStaleWorktrees(t.Context(), initRepo(t)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("scratch root is not a directory", func(t *testing.T) {
		ws := initRepo(t)
		root := filepath.Join(ws, filepath.FromSlash(ScratchRelPath))
		if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(root, []byte("occupied"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := PruneStaleWorktrees(t.Context(), ws); err == nil {
			t.Fatal("file scratch root was accepted")
		}
	})
	t.Run("cancelled provision", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := (&EphemeralWorktree{}).Provision(ctx, []string{"state"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("Provision(cancelled) = %v", err)
		}
	})
	t.Run("provision destination parent is file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "source", "state"), 0o755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, "target")
		if err := os.WriteFile(target, []byte("occupied"), 0o644); err != nil {
			t.Fatal(err)
		}
		w := &EphemeralWorktree{workspace: filepath.Join(root, "source"), Path: target}
		if err := w.Provision(t.Context(), []string{"state"}); err == nil {
			t.Fatal("Provision over file parent was accepted")
		}
	})
}

// TestCreateProvisionRemove_OriginTreeByteIdentical is the DINV-7 leg at this
// package's level: creating a worktree, provisioning it, mutating files
// inside it, and removing it must never change a single byte the origin
// workspace's own working tree holds outside its scratch directory.
func TestCreateProvisionRemove_OriginTreeByteIdentical(t *testing.T) {
	ws := initRepo(t)
	writeFile(t, ws, "vendor/dep/data.txt", "vendored\n")
	sha := headSHA(t, ws)

	before := hashTree(t, ws)

	wt, err := Create(context.Background(), ws, sha, "run-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := wt.Provision(context.Background(), []string{"vendor/dep"}); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	// Mutate the worktree's own copy of main.go — this must be invisible to
	// the origin workspace.
	if err := os.WriteFile(filepath.Join(wt.Path, "main.go"), []byte("mutated\n"), 0o644); err != nil {
		t.Fatalf("mutate worktree file: %v", err)
	}
	if err := wt.Remove(context.Background()); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	after := hashTree(t, ws)
	if before != after {
		t.Fatalf("origin workspace tree changed across a worktree lifecycle: before=%s after=%s", before, after)
	}
}

// hashTree hashes every path+content under dir, excluding .git (git's own
// internal bookkeeping, expected to shift) and .codegate (codegate's own
// scratch, explicitly out of scope for the tree-identity guarantee).
func hashTree(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if rel == ".git" || rel == ".codegate" {
			return filepath.SkipDir
		}
		h.Write([]byte(rel))
		if d.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		h.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("hash tree %s: %v", dir, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
