package e2e

// Leg 3 — kill / re-run (UB-5; design §3.5 "kill-and-re-run converges", AP-22).
// An interrupted converge must re-run to the IDENTICAL end state. Two proofs:
//
//   - Primary (deterministic): simulate a killed run by deleting a subset of
//     the converged engine-verbatim files AND planting an abandoned fetch tmp
//     (`.pyxis/bin/<x>.tmp-<n>` — the exact garbage a mid-fetch kill leaves;
//     the janitor's reapScan skips `.tmp-` names by contract), then re-run
//     `pyxis converge` and assert the tree converges to the pre-damage
//     snapshot (minus the tolerated tmp, which converge neither trusts nor is
//     tripped by). This tests converge.go's stated invariant directly: "a
//     killed run leaves only converged-or-untouched files plus at most an
//     abandoned tmp, and a re-run converges to the identical end state."
//
//   - Secondary (true SIGKILL, best-effort): start `pyxis converge` as a child,
//     kill it, re-run, assert convergence. Over a tiny fixture converge may
//     finish before the kill lands — that is a vacuous pass (a re-run is still
//     a no-op), so this subtest tolerates the finished-early case and never
//     gates the leg. Per-file atomic writes (tmp+rename) mean even a
//     mid-rename kill leaves old-or-new bytes, never a torn file.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestKillReRun(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)

	t.Run("deterministic partial tree re-converges", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		converged := snap(t, repo) // the healthy end state

		// Simulate the aftermath of a killed converge: several engine-verbatim
		// files gone, plus an abandoned fetch tmp in the cache. (Not the hosted
		// workflow: this scratch repo is ci_mode=local, so §3.8 excludes it from
		// the desired set entirely — there is nothing there to delete or repair.)
		for _, rel := range []string{
			".pyxis/pipeline.md",
			".pyxis/templates/product-vision-skeleton.md",
		} {
			if err := os.Remove(at(repo, rel)); err != nil {
				t.Fatalf("remove %s: %v", rel, err)
			}
		}
		abandonedTmp := filepath.Join(repo, ".pyxis", "bin", "docgate-"+r.version+"-"+r.plat+".tmp-999999")
		writeFile(t, abandonedTmp, []byte("half-fetched garbage"), 0o644)
		atomicTmp := at(repo, ".pyxis/.pyxis-converge-123456789")
		writeFile(t, atomicTmp, []byte("interrupted atomic write"), 0o644)

		// Re-run converge: it repairs the missing files.
		res := pyxis(t, repo, env, "converge")
		if res.code != 0 {
			t.Fatalf("re-run converge = %d, want 0 (converge the partial tree)\nstderr: %s", res.code, res.stderr)
		}

		// Remove the tolerated tmp before the identity check — converge neither
		// trusts nor is tripped by it (reapScan skips `.tmp-`); its presence or
		// absence is not part of "the identical end state" the contract means.
		_ = os.Remove(abandonedTmp)
		dropAtomicWriteTemps(t, at(repo, ".pyxis"))
		sameSnap(t, converged, snap(t, repo), "kill/re-run (deterministic)")

		// And it is now genuinely converged: a further run is a plain no-op.
		assertSecondConvergeNoop(t, repo, env)
	})

	t.Run("SIGKILL mid-flight re-converges (best-effort)", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		converged := snap(t, repo)

		// Damage the tree so a fresh converge has real work to do, then start it
		// and try to kill it mid-flight. Over a tiny fixture it may finish first
		// — tolerated (a re-run is still a no-op either way).
		for _, rel := range []string{".pyxis/pipeline.md", ".pyxis/templates/product-vision-skeleton.md"} {
			_ = os.Remove(at(repo, rel))
		}
		cmd := killableConverge(t, repo, env)
		time.Sleep(2 * time.Millisecond)
		_ = cmd.Process.Kill()
		_ = cmd.Wait() // reap; the exit status is irrelevant (killed or clean)

		// Re-run to completion — this is the convergence guarantee regardless of
		// whether the kill interrupted anything.
		res := pyxis(t, repo, env, "converge")
		if res.code != 0 {
			t.Fatalf("re-run after SIGKILL = %d, want 0\nstderr: %s", res.code, res.stderr)
		}
		// Drop any abandoned tmp a mid-fetch kill may have left, then compare.
		dropTmpFiles(t, filepath.Join(repo, ".pyxis", "bin"))
		dropAtomicWriteTemps(t, at(repo, ".pyxis"))
		sameSnap(t, converged, snap(t, repo), "kill/re-run (SIGKILL)")
	})
}

// A SIGKILL can also abandon writeTreeAtomic's dot-temp in the target's
// directory. Ignore only regular files with its generated numeric suffix;
// managed payload, directories and symlinks remain part of the identity check.
func dropAtomicWriteTemps(t *testing.T, root string) {
	t.Helper()
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		const prefix = ".pyxis-converge-"
		if !entry.Type().IsRegular() || !strings.HasPrefix(entry.Name(), prefix) {
			return nil
		}
		if _, err := strconv.ParseUint(strings.TrimPrefix(entry.Name(), prefix), 10, 32); err != nil {
			return nil
		}
		return os.Remove(path)
	}); err != nil {
		t.Fatalf("remove abandoned atomic-write temps: %v", err)
	}
}

// dropTmpFiles removes any `*.tmp-*` fetch tmp left in the cache dir (a
// mid-fetch kill's garbage), so the end-state comparison ignores it — exactly
// as converge and the janitor do.
func dropTmpFiles(t *testing.T, cacheDir string) {
	t.Helper()
	ents, err := os.ReadDir(cacheDir)
	if err != nil {
		return // no cache dir yet is fine
	}
	for _, e := range ents {
		if name := e.Name(); containsTmpInfix(name) {
			_ = os.Remove(filepath.Join(cacheDir, name))
		}
	}
}

func containsTmpInfix(name string) bool {
	for i := 0; i+5 <= len(name); i++ {
		if name[i:i+5] == ".tmp-" {
			return true
		}
	}
	return false
}
