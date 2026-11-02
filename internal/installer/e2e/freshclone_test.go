package e2e

// Leg 4 — fresh-clone self-heal (A-9; design §3.4, DINV-1). A fresh clone of an
// installed repo carries the committed tree but NOT the gitignored .pyxis/bin
// cache — so the first tool invocation must acquire and verify from the source.
// Simulate the fresh clone by removing .pyxis/bin (manifest cache + binaries)
// on a converged repo, then:
//
//   - ONLINE (the file:// source reachable): `pyxis exec docgate -- gate`
//     self-heals — re-fetches+verifies the manifest and the docgate binary,
//     runs the real gate, exits 0. The cache is re-populated.
//   - OFFLINE (artifact_source pointed at a nonexistent path): the same
//     invocation refuses BY NAME — the manifest fetch fails, resolve returns
//     the OfflineError naming the remedy (`pyxis converge` / re-run the
//     bootstrap), exit 1, ZERO execution (DINV-1).
//
// A dead HTTP(S)_PROXY tripwire rides the offline sub-leg so a stray http would
// fail loudly; the source is file:// so the tripwire is inert on the happy path.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreshCloneSelfHeal(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)

	t.Run("online first invocation self-heals", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		emptyBinCache(t, repo)

		// `pyxis exec pyxis -- version` on the empty cache: resolve re-fetches
		// the manifest AND the pyxis binary from the file:// source, re-verifies
		// both (DINV-1), execve's, and runs `version` → exit 0. (`version` needs
		// no repo root, so the self-heal proof does not depend on the tool's own
		// root discovery — the docgate-gate variant is a separately-reported
		// defect, TestOperandDocgateGateNeedsGoMod.)
		res := pyxis(t, repo, env, "exec", "pyxis", "--", "version")
		if res.code != 0 {
			t.Fatalf("fresh-clone online `pyxis exec pyxis -- version` = %d, want 0 (self-heal)\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
		}
		// The cache re-populated from the source: the manifest + the pyxis
		// binary are back (fetch-verified, gitignored).
		for _, name := range []string{
			"release-manifest-" + r.version + ".json",
			"pyxis-" + r.version + "-" + r.plat,
		} {
			if _, err := os.Stat(filepath.Join(repo, ".pyxis", "bin", name)); err != nil {
				t.Errorf("self-heal did not re-cache %s: %v", name, err)
			}
		}
	})

	t.Run("offline first invocation refuses by name", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		emptyBinCache(t, repo)

		// Repoint the artifact source at a nonexistent directory (offline: the
		// file:// fetch will fail with no such path) — the fresh clone can reach
		// no verified bytes.
		repointArtifactSource(t, repo, "file://"+filepath.Join(t.TempDir(), "no-such-release"))

		offlineEnv := append([]string{
			"HTTP_PROXY=http://127.0.0.1:1",
			"HTTPS_PROXY=http://127.0.0.1:1",
			"NO_PROXY=",
		}, env...)

		// The manifest fetch fails against the dead source BEFORE any tool
		// resolves — so the tool named is immaterial; the refusal is the
		// OfflineError with its remedy, exit 1, zero execution (DINV-1).
		res := pyxis(t, repo, offlineEnv, "exec", "pyxis", "--", "version")
		if res.code != 1 {
			t.Fatalf("fresh-clone offline exec = %d, want 1 (named refusal, zero execution)\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
		}
		// The refusal names the remedy (OfflineError): pre-warm while online,
		// or re-run the bootstrap.
		for _, want := range []string{"pre-warm while online", "pyxis converge"} {
			if !strings.Contains(res.stderr, want) {
				t.Errorf("offline refusal lacks the remedy fragment %q:\n%s", want, res.stderr)
			}
		}
		// ZERO execution: nothing ran, so the cached pyxis `version` never
		// printed (stdout is empty — the refusal is on stderr).
		if strings.TrimSpace(res.stdout) != "" {
			t.Errorf("offline refusal appears to have executed the tool:\n%s", res.stdout)
		}
	})
}

// emptyBinCache removes the gitignored .pyxis/bin cache dir — the fresh-clone
// condition (the committed tree lacks it; only pre-warm/exec populate it).
func emptyBinCache(t *testing.T, repo string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(repo, ".pyxis", "bin")); err != nil {
		t.Fatalf("remove .pyxis/bin: %v", err)
	}
}

// repointArtifactSource rewrites the operand config's system.artifact_source to
// newSource — a targeted edit of the one repo-owned key, so the config still
// closed-parses (the other keys are untouched). Used to point a fresh clone at
// a dead source for the offline-refusal sub-leg.
func repointArtifactSource(t *testing.T, repo, newSource string) {
	t.Helper()
	cfgPath := at(repo, ".pyxis/config.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	lines := strings.Split(string(b), "\n")
	replaced := false
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "artifact_source:") {
			// Preserve the two-space indent of the system: block key.
			lines[i] = "  artifact_source: \"" + newSource + "\""
			replaced = true
			break
		}
	}
	if !replaced {
		t.Fatalf("config has no artifact_source line to repoint:\n%s", b)
	}
	writeFile(t, cfgPath, []byte(strings.Join(lines, "\n")), 0o644)
	// The rewritten config must still closed-parse (a targeted key edit).
	loadConfig(t, repo)
}
