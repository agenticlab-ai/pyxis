package e2e

// Leg 2 — the drift matrix (A-4/FR-5/FR-6/UB-5; design §3.2, DINV-3/DINV-4).
// On a converged scratch repo, inject each ownership-class drift and drive the
// REAL `pyxis converge` binary end to end, asserting repair-or-refuse per class
// and — after a repair — that a SECOND converge is a byte-exact no-op (the
// idempotency + convergence contract, proven through the packaged verb, not the
// library). Each class is its own subtest:
//
//   - engine-verbatim tampered → repaired byte-exact (converge overwrites from
//     the verified payload archive); a second run is a no-op.
//   - engine-verbatim deleted → repaired byte-exact (same path — drift is
//     absence-or-mismatch); a second run is a no-op.
//   - generated stale → regenerated (the docgate index generator); no-op after.
//   - generated deleted → regenerated; no-op after.
//   - repo-owned schema-invalid (.pyxis/config.yaml) → REFUSED by name with the
//     file untouched (DINV-3): the config-load gate names the bad key, exit 1,
//     the engine is never reached, and the corrupt bytes are left in place.
//
// Overlaps converge's own unit tests deliberately — T13's version drives the
// contract through the shipped binary on a real scratch repo (AP-14), where the
// unit tests exercise the library directly.

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// engineVerbatimTarget is a converged engine-verbatim file the fixture ships —
// a stable, small one to tamper/delete and watch converge repair byte-exact.
const engineVerbatimTarget = ".pyxis/pipeline.md"

// generatedTarget is a converged generated file — the design index docgate's
// generator owns (converge regenerates, never hashes).
const generatedTarget = "docs/tech-design/index.json"

func TestConvergeInstallsLogExclusionForExistingRepository(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo, env := scratchRepo(t, r)
	// Model an existing install whose repo-owned root ignore predates local
	// logs. The shipped system rule must arrive through normal convergence.
	rootRules := []byte("# operator-owned rules\n/custom-cache/\n.pyxis/bin/\n.pyxis/scratch/\n")
	writeFile(t, at(repo, ".gitignore"), rootRules, 0o644)
	if err := os.Remove(at(repo, ".pyxis/.gitignore")); err != nil {
		t.Fatal(err)
	}
	res := pyxis(t, repo, env, "converge")
	if res.code != 0 {
		t.Fatalf("converge = %d: %s", res.code, res.stderr)
	}
	if got := readTree(t, repo, ".gitignore"); !equalBytes(got, rootRules) {
		t.Fatal("converge changed the operator's root ignore rules")
	}
	if got := readTree(t, repo, ".pyxis/.gitignore"); !equalBytes(got, readSourceFile(t, ".pyxis/.gitignore")) {
		t.Fatal("converge did not install the shipped log exclusion")
	}
	log := assertCLILogEvent(t, repo, "converge", "converge_run")
	check := exec.Command("git", "-C", repo, "check-ignore", log)
	check.Env = env
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("converge log is not ignored after upgrade: %v %s", err, output)
	}
	assertSecondConvergeNoop(t, repo, env)
}

func TestDriftMatrix(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)

	t.Run("engine-verbatim tampered repairs byte-exact", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		want := readTree(t, repo, engineVerbatimTarget) // the converged bytes
		writeFile(t, at(repo, engineVerbatimTarget), []byte("TAMPERED — not the pinned bytes\n"), 0o644)

		res := pyxis(t, repo, env, "converge")
		if res.code != 0 {
			t.Fatalf("converge after verbatim tamper = %d, want 0 (repair)\nstderr: %s", res.code, res.stderr)
		}
		if !strings.Contains(res.stdout, "repaired 1") || !strings.Contains(res.stdout, "repaired "+engineVerbatimTarget) {
			t.Fatalf("converge did not report repairing %s:\n%s", engineVerbatimTarget, res.stdout)
		}
		if got := readTree(t, repo, engineVerbatimTarget); !equalBytes(got, want) {
			t.Fatalf("%s not repaired byte-exact after converge", engineVerbatimTarget)
		}
		assertSecondConvergeNoop(t, repo, env)
	})

	t.Run("engine-verbatim deleted repairs byte-exact", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		want := readTree(t, repo, engineVerbatimTarget)
		if err := os.Remove(at(repo, engineVerbatimTarget)); err != nil {
			t.Fatalf("remove %s: %v", engineVerbatimTarget, err)
		}
		res := pyxis(t, repo, env, "converge")
		if res.code != 0 {
			t.Fatalf("converge after verbatim delete = %d, want 0 (repair)\nstderr: %s", res.code, res.stderr)
		}
		if !strings.Contains(res.stdout, "repaired "+engineVerbatimTarget) {
			t.Fatalf("converge did not re-materialize the deleted %s:\n%s", engineVerbatimTarget, res.stdout)
		}
		if got := readTree(t, repo, engineVerbatimTarget); !equalBytes(got, want) {
			t.Fatalf("%s not restored byte-exact after converge", engineVerbatimTarget)
		}
		assertSecondConvergeNoop(t, repo, env)
	})

	t.Run("generated stale regenerates", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		writeFile(t, at(repo, generatedTarget), []byte("{\"stale\":true}\n"), 0o644)
		res := pyxis(t, repo, env, "converge")
		if res.code != 0 {
			t.Fatalf("converge after generated stale = %d, want 0 (regenerate)\nstderr: %s", res.code, res.stderr)
		}
		if !strings.Contains(res.stdout, "regenerated "+generatedTarget) {
			t.Fatalf("converge did not regenerate %s:\n%s", generatedTarget, res.stdout)
		}
		// The regenerated index is the docgate empty-corpus output — not the
		// stale bytes we planted.
		if strings.Contains(string(readTree(t, repo, generatedTarget)), "stale") {
			t.Fatalf("%s still carries the stale bytes after converge", generatedTarget)
		}
		assertSecondConvergeNoop(t, repo, env)
	})

	t.Run("generated deleted regenerates", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		if err := os.Remove(at(repo, generatedTarget)); err != nil {
			t.Fatalf("remove %s: %v", generatedTarget, err)
		}
		res := pyxis(t, repo, env, "converge")
		if res.code != 0 {
			t.Fatalf("converge after generated delete = %d, want 0 (regenerate)\nstderr: %s", res.code, res.stderr)
		}
		if !strings.Contains(res.stdout, "regenerated "+generatedTarget) {
			t.Fatalf("converge did not regenerate the deleted %s:\n%s", generatedTarget, res.stdout)
		}
		if _, err := os.Stat(at(repo, generatedTarget)); err != nil {
			t.Fatalf("%s absent after converge regenerate: %v", generatedTarget, err)
		}
		assertSecondConvergeNoop(t, repo, env)
	})

	t.Run("repo-owned schema-invalid refuses by name untouched", func(t *testing.T) {
		repo, env := scratchRepo(t, r)
		const cfgRel = ".pyxis/config.yaml"
		// A schema-invalid config: an unknown block key under the closed schema.
		// The closed parse refuses it BY NAME (the file + the offending key),
		// exit 1, before the engine runs. This is the repo-owned "never
		// rewritten, refuse by name" contract (DINV-3): the only repo-owned
		// schema today is the config, validated at the config-load gate; a
		// schema-valid-but-drifted repo-owned file would instead surface as a
		// converge per-file RefusedByName, but no such second schema exists yet.
		corrupt := []byte("bogus:\n  x: y\n")
		writeFile(t, at(repo, cfgRel), corrupt, 0o644)

		res := pyxis(t, repo, env, "converge")
		if res.code != 1 {
			t.Fatalf("converge with a schema-invalid config = %d, want 1 (named refusal)\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
		}
		// The refusal names the offending config: the unknown key and the file.
		if !strings.Contains(res.stderr, "bogus") || !strings.Contains(res.stderr, ".pyxis/config.yaml") {
			t.Fatalf("refusal does not name the offending config/key:\n%s", res.stderr)
		}
		// UNTOUCHED (DINV-3): converge never rewrites repo-owned content — the
		// corrupt bytes are exactly as we left them (and the engine never ran).
		if got := readTree(t, repo, cfgRel); !equalBytes(got, corrupt) {
			t.Fatalf("%s was rewritten on a repo-owned refusal (DINV-3 violated):\n%s", cfgRel, got)
		}
	})
}

// assertSecondConvergeNoop drives a second `pyxis converge` on a repo just
// repaired and asserts it mutates nothing: mode noop AND a byte-identical tree
// snapshot across the run (the FR-6 idempotency proof through the binary).
func assertSecondConvergeNoop(t *testing.T, repo string, env []string) {
	t.Helper()
	before := snap(t, repo)
	res := pyxis(t, repo, env, "converge")
	if res.code != 0 {
		t.Fatalf("second converge = %d, want 0 (noop)\nstderr: %s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "mode noop") {
		t.Fatalf("second converge summary = %q, want noop", res.stdout)
	}
	sameSnap(t, before, snap(t, repo), "second converge (idempotency)")
}

// readTree reads an operand file, fataling on error — the drift legs' byte
// comparisons.
func readTree(t *testing.T, repo, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(at(repo, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
