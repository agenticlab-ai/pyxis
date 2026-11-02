package e2e

// Leg 3 — the supply-chain tamper drill set (DOD-5/A-2/UB-3; design §3.9 leg 1,
// ABUSE-1/ABUSE-4). For each acquired artifact — a tool binary, the payload
// archive, and the bootstrap script's fetched pyxis —
// corrupt it after it is built/served and assert the CONSUMER refuses with zero
// execution, naming expected-vs-actual. Nothing tampered runs, nothing tampered
// is trusted (DINV-1, fail closed, AP-17). Plus the DECLARED-OPERATION audit
// drill: the wiring's executed set never exceeds the declared table (ABUSE-3).
//
// Each drill drives the artifact's REAL verifier at its own layer — the honest
// consumer for that artifact, not a re-implementation:
//
//   - binary        → resolve.Tool + `pyxis exec` (the per-exec re-hash, DINV-1)
//   - payload        → converge / resolve.EnsurePayload (the converge fetch)
//   - bootstrap      → the real pyxis-bootstrap.sh shim (the one first-mover)
//   - declared ops   → a github-mode init's wiring audit (executed ⊆ declared)
//
// Hermetic: file:// fixtures, the real shim over a file:// source, no network.

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// TestTamperBinaryRefusesZeroExecution corrupts the served docgate binary on a
// fresh clone and asserts BOTH the resolve library and the shipped `pyxis exec`
// path refuse with a named sha256 mismatch and zero execution (DINV-1).
func TestTamperBinaryRefusesZeroExecution(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo, env := scratchRepo(t, r)
	emptyBinCache(t, repo) // fresh clone: the cache must re-fetch from the source

	// Corrupt the docgate binary AT THE SOURCE so the fetch delivers tampered
	// bytes (a real registry/mirror compromise); the pin in the manifest is
	// untouched, so the re-hash catches the mismatch.
	servedBinary := filepath.Join(r.src, "docgate-"+r.version+"-"+r.plat)
	overwrite(t, servedBinary, []byte("#!/bin/sh\necho PWNED\n"))

	// (a) The resolve library: Tool returns a *TamperError naming expected vs
	// actual, and NO exec path (the returned path is empty on a refusal).
	rcfg := resolve.Config{
		Root:   repo,
		Repo:   loadConfig(t, repo),
		Logger: slog.New(slog.NewTextHandler(discardWriter{}, nil)),
	}
	path, err := resolve.Tool(context.Background(), rcfg, "docgate", "")
	var te *resolve.TamperError
	if !errors.As(err, &te) {
		t.Fatalf("tampered docgate: resolve.Tool err = %v, want *TamperError", err)
	}
	if path != "" {
		t.Fatalf("tampered docgate: a usable exec path was returned alongside the refusal: %q (DINV-1 violated)", path)
	}
	if te.Expected == "" || te.Actual == "" || te.Expected == te.Actual {
		t.Fatalf("TamperError does not name distinct expected/actual: %+v", te)
	}

	// (b) The shipped exec path: `pyxis exec docgate -- gate` refuses (exit 1),
	// names expected-vs-actual on stderr, and executes NOTHING (empty stdout —
	// the tampered "PWNED" never printed).
	res := pyxis(t, repo, env, "exec", "docgate", "--", "gate")
	if res.code != 1 {
		t.Fatalf("`pyxis exec docgate` over a tampered binary = %d, want 1 (named refusal)\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
	}
	if strings.Contains(res.stdout, "PWNED") || strings.TrimSpace(res.stdout) != "" {
		t.Fatalf("tampered binary appears to have executed (stdout non-empty):\n%s", res.stdout)
	}
	for _, want := range []string{"sha256 mismatch", te.Expected, te.Actual} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("exec refusal does not name %q:\n%s", want, res.stderr)
		}
	}
}

// TestTamperPayloadRefusesConverge corrupts the served payload archive and
// asserts converge (a fresh-clone reconcile that must re-fetch it) refuses with
// a TamperError naming the payload — the tree is never written from unverified
// bytes.
func TestTamperPayloadRefusesConverge(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo, _ := scratchRepo(t, r)
	emptyBinCache(t, repo) // drop the warm payload+manifest cache: force a re-fetch

	// Corrupt the payload archive at the source; the manifest's
	// payload.archive_sha256 pin is untouched, so the re-hash catches it.
	servedPayload := filepath.Join(r.src, "payload-"+r.version+".tar.gz")
	overwrite(t, servedPayload, []byte("not a valid gzip payload — tampered"))

	// Drive resolve.EnsurePayload directly (converge's payload acquisition):
	// it must return a *TamperError naming the payload archive.
	rcfg := resolve.Config{
		Root:   repo,
		Repo:   loadConfig(t, repo),
		Logger: slog.New(slog.NewTextHandler(discardWriter{}, nil)),
	}
	man, err := resolve.EnsureManifest(context.Background(), rcfg)
	if err != nil {
		t.Fatalf("manifest fetch (untampered) failed: %v", err)
	}
	_, perr := resolve.EnsurePayload(context.Background(), rcfg, man)
	var te *resolve.TamperError
	if !errors.As(perr, &te) {
		t.Fatalf("tampered payload: EnsurePayload err = %v, want *TamperError", perr)
	}
	if !strings.Contains(te.Artifact, "payload-"+r.version) {
		t.Fatalf("TamperError names %q, want the payload archive", te.Artifact)
	}

	// And converge itself refuses — a fresh-clone converge over the tampered
	// source never writes the tree from unverified bytes.
	_, cerr := converge.Run(context.Background(), rcfg, converge.Options{})
	if cerr == nil {
		t.Fatalf("converge over a tampered payload succeeded — it must refuse (DINV-1)")
	}
	if !errors.As(cerr, &te) {
		t.Fatalf("converge over a tampered payload err = %v, want a *TamperError", cerr)
	}
}

// TestTamperBootstrapScriptRefusesZeroInstall drives the REAL bootstrap shim
// (the one shell first-mover) against a file:// source whose pyxis binary is
// tampered, and asserts it refuses with a named expected-vs-actual mismatch and
// installs NOTHING (ABUSE-4). The shim is the one artifact no Go can gate; this
// is its tamper drill at the e2e layer (the release package also drills it).
func TestTamperBootstrapScriptRefusesZeroInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the bootstrap shim is POSIX-only")
	}
	for _, tool := range []string{"bash", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s unavailable: %v", tool, err)
		}
	}
	shim := filepath.Join(repoRoot(), ".pyxis", "scripts", "pyxis-bootstrap.sh")
	if _, err := os.Stat(shim); err != nil {
		t.Skipf("bootstrap shim not present: %v", err)
	}

	plat := resolve.Platform() // e.g. darwin-arm64 — the shim derives the same
	src := t.TempDir()
	repo := t.TempDir()
	// The served (tampered) binary — its bytes will NOT match the documented pin.
	served := filepath.Join(src, "pyxis-"+fixtureVersion+"-"+plat)
	overwrite(t, served, []byte("tampered pyxis bytes — not the documented pin\n"))

	// The DOCUMENTED pin is a hash the served bytes do not have (a wrong pin is
	// indistinguishable from tampered bytes — both are a mismatch the shim must
	// refuse). Use an all-zero sha256 as the "documented" pin.
	badPin := strings.Repeat("0", 64)
	platKey := "PYXIS_SHA256_" + strings.ToUpper(strings.NewReplacer("-", "_").Replace(plat))

	cmd := exec.Command("bash", shim)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"PYXIS_VERSION="+fixtureVersion,
		platKey+"="+badPin,
		"PYXIS_ARTIFACT_SOURCE=file://"+src,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bootstrap shim accepted a tampered/mismatched binary — it must refuse (ABUSE-4):\n%s", out)
	}
	// Zero installation: nothing landed in .pyxis/bin (not the versioned binary,
	// not the stable entry).
	for _, name := range []string{"pyxis-" + fixtureVersion + "-" + plat, "pyxis"} {
		if _, serr := os.Stat(filepath.Join(repo, ".pyxis", "bin", name)); serr == nil {
			t.Fatalf("bootstrap left %s installed on a mismatch (must be zero install)", name)
		}
	}
	// The refusal names the mismatch and the documented pin.
	msg := string(out)
	if !strings.Contains(msg, "mismatch") || !strings.Contains(msg, badPin) {
		t.Fatalf("bootstrap refusal does not name the mismatch/pin:\n%s", msg)
	}
}

// TestDeclaredOperationAuditIsClean is the ABUSE-3 audit drill at the e2e layer:
// a github-mode install's wiring never executes an operation outside the
// declared table. It drives the same in-process github init as leg 1 with a
// recording transcript and asserts the audit's walked set equals wire.Table's
// declared set — the executed set never exceeds the declared set.
func TestDeclaredOperationAuditIsClean(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	c := ghCombo{name: "audit", ciMode: "both", repo: "create", protection: "checks+review"}

	declared := wire.Table(wire.Config{
		Repo:            repoConfigForCombo(c),
		Slug:            fixtureSlug,
		OnboardingItems: installer.OnboardingWorkItems(repoConfigForCombo(c)),
	})
	transcript := &ghTranscript{}
	repo := githubScratchRepo(t)
	rep := runGitHubInit(t, repo, r, c, transcript)

	if rep.T8.Wiring == nil {
		t.Fatalf("github-mode install recorded no wiring audit")
	}
	assertAuditMatchesDeclared(t, declared, *rep.T8.Wiring)
	assertTranscriptWithinTable(t, declared, c, transcript)
	// The audit's op count never exceeds the declared table (the ABUSE-3 bound,
	// stated directly).
	if got := len(rep.T8.Wiring.Operations); got > len(declared) {
		t.Fatalf("wiring audit walked %d op(s), the declared table has %d — the executed set exceeded the declared set (ABUSE-3)", got, len(declared))
	}
}

// ---- drill helpers ----------------------------------------------------------

// overwrite truncates and rewrites a file with new bytes — the "tamper after
// it is served" primitive.
func overwrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o755); err != nil {
		t.Fatalf("overwrite %s: %v", path, err)
	}
}
