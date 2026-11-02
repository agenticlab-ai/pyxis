package preflight_test

// The artifact-source check's fixture matrix (design init-hardening-v1 §3.1
// check 5; DINV-1; prd:init-hardening FR-5; plan init-hardening/001 T2's
// acceptance criteria): no pin yet means nothing to probe; a cold cache
// probes reachability under the resolved credential — an unreachable source
// refuses naming the host and the credential remedy, a reachable one passes
// without ever writing to the operand's own cache; a warm, hash-verified
// cache skips the probe outright — proven here by pointing a verified cache's
// fixture at a deliberately broken source, so a false "skip" cannot hide
// behind a source that happens to work.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// dummyManifestSHA256 is a well-formed (64-char hex) placeholder pin: the
// artifact-source check's cold-cache leg never verifies content against it
// (a reachability probe only, DINV-1 — the real fetch-verify-cache is
// converge's job later), so any well-formed value exercises the same path a
// real pin would.
var dummyManifestSHA256 = strings.Repeat("ab", 32)

func hexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// artifactSourceOutcome runs the registry and returns just the
// artifact-source outcome (admission and toolchain are along for the ride,
// exactly as toolchain_test.go's toolchainOutcome() filters for its own
// check).
func artifactSourceOutcome(t *testing.T, in preflight.Input) (preflight.Outcome, error) {
	t.Helper()
	rep, err := preflight.Run(context.Background(), in)
	for _, out := range rep {
		if out.Check == preflight.ArtifactSourceID {
			return out, err
		}
	}
	t.Fatalf("Report carries no artifact-source outcome: %+v (err=%v)", rep, err)
	return preflight.Outcome{}, err
}

// TestArtifactSourceNoPinSkipsPass proves the pin-less-caller guard: with no
// System.Version/ManifestSHA256 resolved yet there is no URL to probe
// against, so the check passes outright rather than erroring.
func TestArtifactSourceNoPinSkipsPass(t *testing.T) {
	out, err := artifactSourceOutcome(t, preflight.Input{Target: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Status != preflight.StatusPass {
		t.Fatalf("outcome = %+v, want pass (no pin yet — nothing to probe)", out)
	}
}

// TestArtifactSourceColdCacheReachablePasses proves the cold-cache-but-
// reachable leg: a fresh operand (no cache) whose configured source answers
// passes — and the probe never writes anything into the operand's own cache
// (DINV-1: a reachability read, never a cache write).
func TestArtifactSourceColdCacheReachablePasses(t *testing.T) {
	target := t.TempDir()
	srcDir := t.TempDir()
	const ver = "v1.2.3"
	if err := os.WriteFile(filepath.Join(srcDir, "release-manifest-"+ver+".json"),
		[]byte("anything — content is never hashed or parsed by this check"), 0o644); err != nil {
		t.Fatalf("seed fixture source: %v", err)
	}

	in := preflight.Input{
		Target: target,
		System: config.System{
			Version:        ver,
			ManifestSHA256: dummyManifestSHA256,
			ArtifactSource: "file://" + srcDir,
		},
	}
	out, err := artifactSourceOutcome(t, in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Status != preflight.StatusPass {
		t.Fatalf("outcome = %+v, want pass (the source answered)", out)
	}
	if _, statErr := os.Stat(filepath.Join(target, ".pyxis", "bin")); !os.IsNotExist(statErr) {
		t.Errorf(".pyxis/bin exists after a reachability-only probe (stat err=%v) — the check must never write the cache", statErr)
	}
}

// TestArtifactSourceColdCacheUnreachableRefuses proves the cold-cache-and-
// unreachable leg: a source that does not answer refuses pre-write, naming
// the host and the credential remedy (FR-5's fit criterion).
func TestArtifactSourceColdCacheUnreachableRefuses(t *testing.T) {
	target := t.TempDir()
	unreachable := filepath.Join(t.TempDir(), "does-not-exist") // deliberately never created
	const ver = "v1.2.3"

	in := preflight.Input{
		Target: target,
		System: config.System{
			Version:        ver,
			ManifestSHA256: dummyManifestSHA256,
			ArtifactSource: "file://" + unreachable,
		},
	}
	out, err := artifactSourceOutcome(t, in)
	if err == nil {
		t.Fatal("Run = nil error, want an artifact-source refusal")
	}
	if out.Status != preflight.StatusRefuse {
		t.Fatalf("outcome = %+v, want refuse", out)
	}
	blob := out.Reason + " | " + strings.Join(out.Conflicts, "; ") + " | " + out.Remedy
	for _, want := range []string{"release-manifest-" + ver + ".json", "check network access to", "the configured artifact source"} {
		if !strings.Contains(blob, want) {
			t.Errorf("refusal %q does not contain %q", blob, want)
		}
	}
}

// TestArtifactSourceWarmCacheSkipsProbe proves a hash-verified cache never
// even attempts to reach the network: the fixture's ArtifactSource points at
// a path that was never created, so ANY attempt to reach it would refuse —
// the check must still pass because the cache answers the question first
// (design §3.1 check 5's "a warm, hash-verified cache skips the probe").
func TestArtifactSourceWarmCacheSkipsProbe(t *testing.T) {
	target := t.TempDir()
	const ver = "v1.2.3"
	dummy := strings.Repeat("cd", 32)
	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: ver,
		SourceCommit:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Binaries:      map[string]map[string]string{},
		Payload: relmanifest.Payload{
			ArchiveSHA256: dummy,
			Files:         []relmanifest.PayloadFile{{Path: ".pyxis/pipeline.md", SHA256: &dummy, Class: relmanifest.EngineVerbatim}},
		},
		DepsBundleSHA256: dummy,
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	pin := hexOf(mb)
	cacheDir := filepath.Join(target, ".pyxis", "bin")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "release-manifest-"+ver+".json"), mb, 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	in := preflight.Input{
		Target: target,
		System: config.System{
			Version:        ver,
			ManifestSHA256: pin,
			ArtifactSource: "file://" + filepath.Join(t.TempDir(), "unreachable"),
		},
	}
	// Seeding target with a pre-existing .pyxis/bin cache (above) makes it an
	// illegal ADMISSION target (content beyond an empty directory) — expected
	// and irrelevant here: this test isolates the artifact-source check's own
	// outcome, exactly as toolchainOutcome()/admission() do for theirs, so it
	// asserts on the filtered Outcome rather than Run()'s overall error.
	out, _ := artifactSourceOutcome(t, in)
	if out.Status != preflight.StatusPass {
		t.Fatalf("outcome = %+v, want pass (a verified-warm cache skips the probe)", out)
	}
}
