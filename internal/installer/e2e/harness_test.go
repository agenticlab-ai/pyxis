// Package e2e is the installer's scratch-repo end-to-end harness (installer-v1
// §3.11 — "the e2e matrix — the deployment context (AP-14)"; plan
// installer/001 T13). It proves the LOCAL half of the install matrix by
// driving the REAL built `pyxis` binary as a subprocess against a file://
// fixture artifact source whose manifest pins REAL built binaries (pyxis +
// docgate). This is the packaged deployment context the design demands: no
// ambient toolchain assumption on the operand path, every tool resolved and
// re-verified through the release cache exactly as a shipped install would.
//
// # Why real binaries
//
// The matrix needs `pyxis exec docgate -- gate` to actually run a tool from
// the cache and the operand hook to shell `pyxis exec docgate -- gate` — so
// the fixture manifest's binaries must be genuine executables. TestMain builds
// pyxis + docgate ONCE (≈0.5s warm) into a shared temp dir; the fixture
// release copies them in under their release-artifact names, packs the REAL
// source-repo instantiables/hooks into the payload archive, and derives the
// manifest by hashing exactly those bytes. A file:// source then serves the
// set under the same verification an http source gets (resolve hashes before
// trusting a single byte — DINV-1).
//
// # -short gating
//
// The binary build (and every subprocess leg that needs the built pyxis) is
// gated behind !testing.Short(): `go test -short ./...` skips the whole
// harness fast, the full `go test ./...` runs it. TestMain builds only
// when not short; requireHarness(t) skips a leg when the binaries are absent.
//
// # Network denial
//
// The offline combo (A-5/FR-13) proves the zero-egress invariant two ways: an
// in-process converge/doctor/resolve leg with a denyFetcher that FAILS on any
// http(s) scheme and COUNTS attempts (the deny-fetcher IS the egress scan —
// zero http attempts post-fetch), plus a subprocess leg driving the packaged
// binary path (exec + the pre-commit hook) against the prewarmed file:// cache
// with HTTP(S)_PROXY pointed at a dead address so any stray http would fail
// loudly. Only file:// and warm-cache hits are ever legal.
//
// Hermetic throughout: file:// sources + built binaries, no real network, no
// real gh (carina/local tracker only — the GitHub-mode combos are T14).
package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

// fixtureVersion is the system tag every scratch install in this harness pins.
const fixtureVersion = "v9.9.9"

// builtBins holds the real binaries TestMain compiles once, keyed by tool
// name ("pyxis", "docgate"). Empty in -short mode (the build is skipped), so
// requireHarness(t) skips any leg that needs them.
var builtBins = map[string]string{}

// TestMain builds the real pyxis + docgate binaries once (unless -short) and
// shares them with every leg through builtBins. Building is cheap (≈0.5s warm)
// and is the honest way to exercise the packaged exec path: a scratch repo's
// `pyxis exec docgate -- gate` resolves, re-hashes, and runs a genuine
// executable from its release cache.
func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	if code, helper := runForgejoModelHelper(); helper {
		return code
	}
	flag.Parse() // makes testing.Short() readable here (m.Run parses otherwise)
	if !testing.Short() {
		dir, err := os.MkdirTemp("", "pyxis-e2e-bins-*")
		if err != nil {
			panic("e2e: build temp dir: " + err.Error())
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				panic("e2e: remove build temp dir: " + err.Error())
			}
		}()
		root := repoRoot()
		for _, tool := range []string{"pyxis", "docgate"} {
			out := filepath.Join(dir, tool)
			cmd := exec.Command("go", "build", "-o", out, "./cmd/"+tool)
			cmd.Dir = root
			if b, berr := cmd.CombinedOutput(); berr != nil {
				panic("e2e: build " + tool + ": " + berr.Error() + "\n" + string(b))
			}
			builtBins[tool] = out
		}
	}
	return m.Run()
}

// requireHarness skips a leg when the built binaries are absent — the -short
// path (TestMain skipped the build) or a missing Go toolchain. Every
// subprocess leg calls it first so `go test -short` stays fast and green.
func requireHarness(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode: the e2e harness builds real binaries and drives subprocess installs")
	}
	if len(builtBins) == 0 {
		t.Skip("e2e binaries were not built (no Go toolchain in TestMain?)")
	}
}

// repoRoot resolves the source repo root from this package directory
// (internal/installer/e2e → ../../..). The fixture payload's real
// instantiables and the go build both root here.
func repoRoot() string {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		panic("e2e: resolve repo root: " + err.Error())
	}
	return root
}

// ---- the fixture release ----------------------------------------------------

// fixtureRelease is a complete on-disk release directory a file:// source
// serves: the two real binaries under their artifact names, the payload
// archive of real instantiables, and the derived release manifest.
type fixtureRelease struct {
	src     string // the release directory (served as file://src)
	version string // fixtureVersion
	pin     string // sha256 of the manifest bytes — the config's manifest_sha256 (trust root)
	plat    string // resolve.Platform() — the tuple the binaries + manifest use
}

// source returns the file:// artifact-source base for this release.
func (r fixtureRelease) source() string { return "file://" + r.src }

// pf is one payload file of the fixture release: its operand-relative path,
// bytes, ownership class, and exec bit.
type pf struct {
	path  string
	b     []byte
	class relmanifest.Class
	exec  bool
}

// payloadFiles is the fixture payload tree: the REAL source-repo instantiables
// + installed hook variants the T8 tail seeds from (loaded from the source
// repo so the seed step runs against genuine skeletons/manual/hooks), plus a
// verbatim system file, a workflow, and the generated design indexes — the
// same shape the existing installer/main tests pack, so the full pipeline
// (T7 front + T8 tail) runs end to end. The operand config is NOT packed: init
// writes it before converge (repo-owned from birth).
func payloadFiles(t *testing.T) []pf {
	t.Helper()
	files := []pf{
		{path: ".pyxis/pipeline.md", b: []byte("# pipeline map (fixture)\n"), class: relmanifest.EngineVerbatim},
		{path: ".pyxis/skills/help/SKILL.md", b: []byte("---\nname: help\n---\n"), class: relmanifest.EngineVerbatim},
		{path: ".pyxis/skills/implement-now/SKILL.md", b: []byte("---\nname: implement-now\n---\n"), class: relmanifest.EngineVerbatim},
		{path: ".github/workflows/docs-gate.yml", b: []byte("name: docs-gate\n"), class: relmanifest.EngineVerbatim},
		{path: "docs/tech-design/index.md", b: []byte("stale packed index\n"), class: relmanifest.Generated},
		{path: "docs/tech-design/index.json", b: []byte("{}\n"), class: relmanifest.Generated},
	}
	for _, rel := range []string{
		".pyxis/.gitignore",
		".pyxis/templates/product-vision-skeleton.md",
		".pyxis/templates/architecture-principles-skeleton.md",
		".pyxis/templates/operand-agents-manual.md",
		".pyxis/templates/harness-implement-now-skill.md",
		".pyxis/templates/welcome-item.md",
		".pyxis/security/bwrap.apparmor",
		".pyxis/dist/operand-hooks/pre-commit",
		".pyxis/dist/operand-hooks/pre-push",
		".pyxis/dist/operand-hooks/prepare-commit-msg",
	} {
		files = append(files, pf{
			path:  rel,
			b:     readSourceFile(t, rel),
			class: relmanifest.EngineVerbatim,
			exec:  strings.HasPrefix(rel, ".pyxis/dist/operand-hooks/"),
		})
	}
	return files
}

var sharedFixtureRelease struct {
	sync.Mutex
	release *fixtureRelease
}

// buildFixtureRelease copies an immutable package-scoped release into a fresh
// directory. Tamper drills may change their copies without changing the source
// used by any other test. TestMain owns and removes the shared directory.
func buildFixtureRelease(t *testing.T) fixtureRelease {
	t.Helper()
	requireHarness(t)
	sharedFixtureRelease.Lock()
	defer sharedFixtureRelease.Unlock()
	if sharedFixtureRelease.release == nil {
		src := filepath.Join(filepath.Dir(builtBins["pyxis"]), "fixture-release")
		release := assembleFixtureRelease(t, src)
		sharedFixtureRelease.release = &release
	}
	release := *sharedFixtureRelease.release
	dst := t.TempDir()
	for _, name := range []string{"release-manifest-" + release.version + ".json", "payload-" + release.version + ".tar.gz"} {
		copyFile(t, filepath.Join(release.src, name), filepath.Join(dst, name), 0o644)
	}
	for _, tool := range []string{"pyxis", "docgate"} {
		name := tool + "-" + release.version + "-" + release.plat
		copyFile(t, filepath.Join(release.src, name), filepath.Join(dst, name), 0o755)
	}
	release.src = dst
	return release
}

// assembleFixtureRelease assembles a complete fixture release directory serving
// the two real binaries + the real-instantiable payload + the derived
// manifest. It hand-rolls the set (rather than release.Pack, which scans all
// of cmd/ and needs every tool built for every platform) so the release is
// cheap and platform-local while still carrying genuine executables the
// packaged exec path runs.
func assembleFixtureRelease(t *testing.T, src string) fixtureRelease {
	t.Helper()
	requireHarness(t)
	plat := resolve.Platform()
	files := payloadFiles(t)
	archive := tarGz(t, files)

	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: fixtureVersion,
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries: map[string]map[string]string{
			"pyxis":   {plat: hashFile(t, builtBins["pyxis"])},
			"docgate": {plat: hashFile(t, builtBins["docgate"])},
		},
		Payload: relmanifest.Payload{ArchiveSHA256: hexOf(archive)},
	}
	for _, f := range files {
		entry := relmanifest.PayloadFile{Path: f.path, Class: f.class}
		if f.class == relmanifest.EngineVerbatim {
			sum := hexOf(f.b)
			entry.SHA256 = &sum
		}
		man.Payload.Files = append(man.Payload.Files, entry)
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(src, "release-manifest-"+fixtureVersion+".json"), mb, 0o644)
	writeFile(t, filepath.Join(src, "payload-"+fixtureVersion+".tar.gz"), archive, 0o644)
	// The two binaries under their release-artifact names (resolve fetches
	// these into the operand's gitignored cache and re-verifies per exec).
	copyFile(t, builtBins["pyxis"], filepath.Join(src, "pyxis-"+fixtureVersion+"-"+plat), 0o755)
	copyFile(t, builtBins["docgate"], filepath.Join(src, "docgate-"+fixtureVersion+"-"+plat), 0o755)
	return fixtureRelease{src: src, version: fixtureVersion, pin: hexOf(mb), plat: plat}
}

// ---- the deny fetcher (the egress scan) -------------------------------------

// denyFetcher wraps FileFetcher and REFUSES every http(s) URL, counting the
// attempts. Injected via resolve.Config.Fetcher / installer.Options.Fetcher on
// the in-process offline legs, it IS the egress scan: post-fetch, HTTPAttempts
// must be zero, and any resolution that reached for the network fails loudly
// instead of silently touching the release host (A-5/DINV-7).
type denyFetcher struct {
	HTTPAttempts int
}

func (d *denyFetcher) Fetch(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		d.HTTPAttempts++
		return nil, &egressDeniedError{URL: rawURL}
	}
	return resolve.FileFetcher{}.Fetch(ctx, rawURL)
}

// egressDeniedError is the network-denied refusal the deny-fetcher returns —
// a real run offline would classify this into resolve's OfflineError, so the
// remedy still travels; here it also fails the leg by name if egress happens.
type egressDeniedError struct{ URL string }

func (e *egressDeniedError) Error() string {
	return "e2e: network egress denied (offline combo) — http(s) fetch of " + e.URL + " is forbidden; only file:// and warm-cache hits are legal (A-5/FR-13/DINV-7)"
}

// ---- tar + hashing ----------------------------------------------------------

func hexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hashFile returns a file's sha256 hex — the manifest's binary-entry value for
// a built binary the fixture ships.
func hashFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func tarGz(t *testing.T, files []pf) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		mode := int64(0o644)
		if f.exec {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: f.path, Mode: mode, Size: int64(len(f.b))}); err != nil {
			t.Fatalf("tar header %s: %v", f.path, err)
		}
		if _, err := tw.Write(f.b); err != nil {
			t.Fatalf("tar write %s: %v", f.path, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ---- file helpers -----------------------------------------------------------

func readSourceFile(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read source payload file %s: %v", rel, err)
	}
	return b
}

func writeFile(t *testing.T, path string, b []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func copyFile(t *testing.T, src, dst string, mode fs.FileMode) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer func() { _ = in.Close() }() // The source is read-only; copy errors are checked below.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		t.Fatalf("copy %s to %s: %v", src, dst, copyErr)
	}
	if closeErr != nil {
		t.Fatalf("close %s: %v", dst, closeErr)
	}
}

func at(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

// discardWriter is a no-op io.Writer — the in-process legs' logger sink and the
// subprocess-output sink where a leg does not inspect a child's streams.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// ---- tree snapshot ----------------------------------------------------------

// snap captures every path under root as rel → sha256+mode (dirs "dir"),
// skipping .git contents and local execution diagnostics. The idempotency and
// removal legs compare project state; diagnostic retention is checked separately.
func snap(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	skippedLogs := false
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == runlogs.Root && d.IsDir() {
			skippedLogs = true
			return filepath.SkipDir
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			// Contents stay unsnapshotted (churn), but PRESENCE is recorded:
			// a git init that slipped in before a refusal must flip the
			// byte-identical assertions (review:impl #812 advisory — the
			// skip used to blind them to exactly that regression).
			out[".git"] = "git-dir"
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		out[rel] = hexOf(b) + " " + info.Mode().Perm().String()
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	if skippedLogs {
		// A refused or rolled-back init can leave only .pyxis/logs. Keep
		// every other .pyxis entry visible, including an empty directory.
		onlyLogs := true
		for rel := range out {
			if strings.HasPrefix(rel, ".pyxis/") {
				onlyLogs = false
				break
			}
		}
		if onlyLogs {
			delete(out, ".pyxis")
		}
	}
	return out
}

// assertCLILogEvent proves the shipped binary retained a diagnostic on disk.
func assertCLILogEvent(t *testing.T, root, verb, event string) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, runlogs.CLI, verb+"-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if findLogEvent(t, string(data), event) != nil {
			return path
		}
	}
	t.Fatalf("no retained %s event in %s logs: %v", event, verb, paths)
	return ""
}

// sameSnap asserts two snapshots are byte-and-mode identical — no change, no
// appearance, no disappearance. The idempotency / kill-re-run "identical end
// state" proof.
func sameSnap(t *testing.T, before, after map[string]string, what string) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s: %s changed (%s → %s)", what, k, v, after[k])
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s: %s appeared", what, k)
		}
	}
}

// ---- config load ------------------------------------------------------------

// loadConfig reads and closed-parses the operand config at root — a scratch
// install's own config must parse or converge would refuse it. The legs read
// it to inject the same resolve.Config a subprocess would build.
func loadConfig(t *testing.T, root string) config.Config {
	t.Helper()
	c, defects := config.Load(root)
	if len(defects) > 0 {
		msgs := make([]string, len(defects))
		for i, d := range defects {
			msgs[i] = d.String()
		}
		t.Fatalf("operand config at %s does not closed-parse: %s", root, strings.Join(msgs, "; "))
	}
	return c
}
