package resolve_test

// Tests for the T5 acquisition additions: the exported manifest ensure, the
// payload-archive ensure (fetch-verify-cache under the same floor binaries
// get), the read-only probes/planners preview and doctor ride, and the
// janitor's payload grammar (the cache entry and its reaper arrive in the
// same change, AP-12).

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// tarGz builds a small gzip'd tar of name→content (all mode 0644) — enough
// archive to exercise the payload fetch-verify path.
func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, b := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(b))}); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}
		if _, err := tw.Write(b); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// payloadFixture is a fixture release whose manifest pins a REAL payload
// archive (newFixture's dummy archive hash cannot serve the payload legs).
type payloadFixture struct {
	fx
	archive []byte
	man     relmanifest.Manifest
}

func newPayloadFixture(t *testing.T, version string, bins map[string]map[string][]byte) payloadFixture {
	t.Helper()
	dir := t.TempDir()
	ver := version
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}
	fileBytes := []byte("# pipeline\n")
	archive := tarGz(t, map[string][]byte{".pyxis/pipeline.md": fileBytes})
	sum := hexOf(fileBytes)
	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: ver,
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{},
		Payload: relmanifest.Payload{
			ArchiveSHA256: hexOf(archive),
			Files: []relmanifest.PayloadFile{
				{Path: ".pyxis/pipeline.md", SHA256: &sum, Class: relmanifest.EngineVerbatim},
			},
		},
		DepsBundleSHA256: strings.Repeat("ab", 32),
	}
	for tool, plats := range bins {
		man.Binaries[tool] = map[string]string{}
		for plat, content := range plats {
			man.Binaries[tool][plat] = hexOf(content)
			writeFile(t, filepath.Join(dir, tool+"-"+ver+"-"+plat), content)
		}
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(dir, "release-manifest-"+ver+".json"), mb)
	writeFile(t, filepath.Join(dir, "payload-"+ver+".tar.gz"), archive)
	return payloadFixture{fx: fx{dir: dir, rawVer: version, ver: ver, pin: hexOf(mb)}, archive: archive, man: man}
}

func TestEnsureManifestExportedAccessor(t *testing.T) {
	f := newPayloadFixture(t, "0.3.0", nil)
	root := t.TempDir()
	cfg := f.cfg(root)
	counter := &countingFetcher{inner: resolve.ByScheme{}}
	cfg.Fetcher = counter

	m, err := resolve.EnsureManifest(context.Background(), cfg)
	if err != nil {
		t.Fatalf("EnsureManifest cold: %v", err)
	}
	if m.SystemVersion != "v0.3.0" || counter.count() != 1 {
		t.Fatalf("cold ensure = version %q, %d fetches; want v0.3.0 fetched once", m.SystemVersion, counter.count())
	}
	counter.reset()
	if _, err := resolve.EnsureManifest(context.Background(), cfg); err != nil {
		t.Fatalf("EnsureManifest warm: %v", err)
	}
	if counter.count() != 0 {
		t.Fatalf("warm ensure fetched %d times, want 0 (cached bytes re-hash against the pin)", counter.count())
	}

	// The pin is checked on THIS call: tamper the cached manifest and the
	// ensure refetches the true bytes.
	corrupt(t, cachePath(root, f.manifestName()))
	if _, err := resolve.EnsureManifest(context.Background(), cfg); err != nil {
		t.Fatalf("EnsureManifest after cache tamper: %v", err)
	}
	if counter.count() != 1 {
		t.Fatalf("tampered cache refetched %d times, want 1", counter.count())
	}

	// Missing pins refuse by name.
	_, err = resolve.EnsureManifest(context.Background(), resolve.Config{Root: root, Repo: config.Config{System: config.System{Version: "v0.3.0", Mode: "release"}}, Logger: quiet()})
	var mp *resolve.MissingPinError
	if !errors.As(err, &mp) || mp.Key != "system.manifest_sha256" {
		t.Fatalf("err = %v, want MissingPinError(system.manifest_sha256)", err)
	}
}

func TestCachedManifestProbeNeverFetches(t *testing.T) {
	f := newPayloadFixture(t, "0.3.0", nil)
	root := t.TempDir()
	cfg := f.cfg(root)
	deny := &countingFetcher{inner: failFetcher{}}
	cfg.Fetcher = deny

	// Cold: not cached — ok false, nil error, zero fetches.
	if _, ok, err := resolve.CachedManifest(cfg); ok || err != nil {
		t.Fatalf("cold CachedManifest = (ok %v, err %v), want (false, nil)", ok, err)
	}

	// Warm it through the real ensure, then the probe answers offline.
	online := f.cfg(root)
	if _, err := resolve.EnsureManifest(context.Background(), online); err != nil {
		t.Fatalf("prime: %v", err)
	}
	m, ok, err := resolve.CachedManifest(cfg)
	if err != nil || !ok || m.SystemVersion != "v0.3.0" {
		t.Fatalf("warm CachedManifest = (%q, %v, %v), want (v0.3.0, true, nil)", m.SystemVersion, ok, err)
	}

	// Tampered cache: the probe reports "not verifiably cached", never serves
	// tampered bytes, never fetches.
	corrupt(t, cachePath(root, f.manifestName()))
	if _, ok, err := resolve.CachedManifest(cfg); ok || err != nil {
		t.Fatalf("tampered CachedManifest = (ok %v, err %v), want (false, nil)", ok, err)
	}
	if deny.count() != 0 {
		t.Fatalf("the probe fetched %d times, want 0 (read-only contract)", deny.count())
	}
}

// failFetcher refuses every fetch — the "must not fetch" seam.
type failFetcher struct{}

func (failFetcher) Fetch(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("fetch performed on a read-only path")
}

func TestEnsurePayloadFetchVerifyCache(t *testing.T) {
	f := newPayloadFixture(t, "0.3.0", nil)
	root := t.TempDir()
	cfg := f.cfg(root)
	counter := &countingFetcher{inner: resolve.ByScheme{}}
	cfg.Fetcher = counter
	ctx := context.Background()

	m, err := resolve.EnsureManifest(ctx, cfg)
	if err != nil {
		t.Fatalf("EnsureManifest: %v", err)
	}
	counter.reset()

	// Cold: fetch-verify into the cache under the artifact name, mode 0644.
	path, err := resolve.EnsurePayload(ctx, cfg, m)
	if err != nil {
		t.Fatalf("EnsurePayload cold: %v", err)
	}
	if want := cachePath(root, "payload-v0.3.0.tar.gz"); path != want {
		t.Fatalf("payload cached at %q, want %q", path, want)
	}
	b, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(b, f.archive) {
		t.Fatalf("cached archive differs from the fixture (err %v)", err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o644 {
		t.Fatalf("payload mode = %v (err %v), want 0644 — an archive is data, not a binary", fi.Mode(), err)
	}
	if counter.count() != 1 {
		t.Fatalf("cold ensure fetched %d times, want 1", counter.count())
	}

	// Warm: re-hash only, zero fetches.
	counter.reset()
	if _, err := resolve.EnsurePayload(ctx, cfg, m); err != nil {
		t.Fatalf("EnsurePayload warm: %v", err)
	}
	if counter.count() != 0 {
		t.Fatalf("warm ensure fetched %d times, want 0", counter.count())
	}

	// Tampered cache self-heals online.
	corrupt(t, path)
	if _, err := resolve.EnsurePayload(ctx, cfg, m); err != nil {
		t.Fatalf("EnsurePayload after tamper: %v", err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, f.archive) {
		t.Fatal("tampered archive not restored byte-exact")
	}

	// A tampered SOURCE refuses with the typed tamper error and caches nothing.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	corrupt(t, filepath.Join(f.dir, "payload-v0.3.0.tar.gz"))
	_, err = resolve.EnsurePayload(ctx, cfg, m)
	var te *resolve.TamperError
	if !errors.As(err, &te) || te.Artifact != "payload-v0.3.0.tar.gz" {
		t.Fatalf("err = %v, want TamperError(payload-v0.3.0.tar.gz)", err)
	}
	if exists(path) {
		t.Fatal("a tampered payload was cached")
	}

	// Offline miss refuses with the named remedy.
	offline := f.cfg(root)
	offline.Repo.System.ArtifactSource = "file:///no/such/dir"
	_, err = resolve.EnsurePayload(ctx, offline, m)
	var oe *resolve.OfflineError
	if !errors.As(err, &oe) {
		t.Fatalf("err = %v, want OfflineError", err)
	}
}

func TestCachedPayloadProbe(t *testing.T) {
	f := newPayloadFixture(t, "0.3.0", nil)
	root := t.TempDir()
	cfg := f.cfg(root)
	ctx := context.Background()

	m, err := resolve.EnsureManifest(ctx, cfg)
	if err != nil {
		t.Fatalf("EnsureManifest: %v", err)
	}
	if _, ok, err := resolve.CachedPayload(cfg, m); ok || err != nil {
		t.Fatalf("cold CachedPayload = (ok %v, err %v), want (false, nil)", ok, err)
	}
	path, err := resolve.EnsurePayload(ctx, cfg, m)
	if err != nil {
		t.Fatalf("EnsurePayload: %v", err)
	}
	got, ok, err := resolve.CachedPayload(cfg, m)
	if err != nil || !ok || got != path {
		t.Fatalf("warm CachedPayload = (%q, %v, %v), want (%q, true, nil)", got, ok, err, path)
	}
	corrupt(t, path)
	if _, ok, err := resolve.CachedPayload(cfg, m); ok || err != nil {
		t.Fatalf("tampered CachedPayload = (ok %v, err %v), want (false, nil)", ok, err)
	}
}

func TestPreWarmPlanIsReadOnlyAndExact(t *testing.T) {
	plat := resolve.Platform()
	f := newPayloadFixture(t, "0.3.0", map[string]map[string][]byte{
		"docgate":  {plat: []byte("docgate bytes")},
		"codegate": {plat: []byte("codegate bytes")},
		"elsewhre": {"plan9-386": []byte("other platform")},
	})
	root := t.TempDir()
	cfg := f.cfg(root)
	ctx := context.Background()
	m, err := resolve.EnsureManifest(ctx, cfg)
	if err != nil {
		t.Fatalf("EnsureManifest: %v", err)
	}

	// Cold plan: both this-platform tools, sorted; the off-platform tool is
	// skipped exactly as PreWarm skips it.
	plan, err := resolve.PreWarmPlan(cfg, m)
	if err != nil {
		t.Fatalf("PreWarmPlan: %v", err)
	}
	if want := []string{"codegate", "docgate"}; !slicesEqual(plan, want) {
		t.Fatalf("cold plan = %v, want %v", plan, want)
	}

	// The plan performed nothing: the cache still holds only the manifest.
	ents, err := os.ReadDir(filepath.Join(root, ".pyxis", "bin"))
	if err != nil || len(ents) != 1 {
		t.Fatalf("cache after plan = %d entries (err %v), want the manifest only", len(ents), err)
	}

	if err := resolve.PreWarm(ctx, cfg); err != nil {
		t.Fatalf("PreWarm: %v", err)
	}
	plan, err = resolve.PreWarmPlan(cfg, m)
	if err != nil {
		t.Fatalf("PreWarmPlan warm: %v", err)
	}
	if len(plan) != 0 {
		t.Fatalf("warm plan = %v, want empty", plan)
	}

	// A present-but-edited cached binary does NOT re-enter the plan — the
	// planner reports presence, not per-byte integrity (the per-exec re-hash is
	// gone; pre-warm fetches only what is absent, matching ensureBinary).
	corrupt(t, cachePath(root, "docgate-v0.3.0-"+plat))
	plan, err = resolve.PreWarmPlan(cfg, m)
	if err != nil || len(plan) != 0 {
		t.Fatalf("edited-but-present plan = (%v, %v), want empty (presence, not re-hash)", plan, err)
	}
}

func TestReapPlanAndPayloadJanitorGrammar(t *testing.T) {
	root := t.TempDir()
	cfg := resolve.Config{
		Root:   root,
		Repo:   config.Config{System: config.System{Version: "v0.3.0", Mode: "release"}},
		Logger: quiet(),
	}
	keep := []string{
		"payload-v0.3.0.tar.gz", // pinned version — the janitor keeps it
		"payload-.tar.gz",       // empty version: not an entry
		"payload-notes",         // neither archive nor binary grammar
	}
	reap := []string{
		"payload-v0.1.0.tar.gz", // off-version archive — the new grammar (AP-12)
		"release-manifest-v0.1.0.json",
	}
	for _, name := range append(append([]string{}, keep...), reap...) {
		writeFile(t, cachePath(root, name), []byte(name))
	}

	plan, err := resolve.ReapPlan(cfg)
	if err != nil {
		t.Fatalf("ReapPlan: %v", err)
	}
	if !slicesEqual(plan, reap) {
		t.Fatalf("ReapPlan = %v, want %v (sorted)", plan, reap)
	}
	for _, name := range append(append([]string{}, keep...), reap...) {
		if !exists(cachePath(root, name)) {
			t.Fatalf("ReapPlan removed %s — the planner is read-only", name)
		}
	}

	removed, err := resolve.Reap(cfg)
	if err != nil {
		t.Fatalf("Reap: %v", err)
	}
	if !slicesEqual(removed, reap) {
		t.Fatalf("Reap removed %v, want %v", removed, reap)
	}
	for _, name := range keep {
		if !exists(cachePath(root, name)) {
			t.Fatalf("%s was wrongly reaped", name)
		}
	}
}

func TestArtifactReportersNameTheOverrideURL(t *testing.T) {
	f := newPayloadFixture(t, "0.3.0", nil)
	cfg := f.cfg(t.TempDir())

	name, url, err := resolve.ManifestArtifact(cfg)
	if err != nil || name != "release-manifest-v0.3.0.json" || url != "file://"+f.dir+"/"+name {
		t.Fatalf("ManifestArtifact = (%q, %q, %v)", name, url, err)
	}
	name, url, err = resolve.PayloadArtifact(cfg)
	if err != nil || name != "payload-v0.3.0.tar.gz" || url != "file://"+f.dir+"/"+name {
		t.Fatalf("PayloadArtifact = (%q, %q, %v)", name, url, err)
	}

	// Without the override the built-in channel — the door (FR-28) — carries
	// the system tag.
	cfg.Repo.System.ArtifactSource = ""
	_, url, err = resolve.ManifestArtifact(cfg)
	if err != nil || url != resolve.DefaultChannelRoot+"/v0.3.0/release-manifest-v0.3.0.json" {
		t.Fatalf("default-host URL = %q (err %v)", url, err)
	}

	// Unpinned configs refuse: reporting an acquisition for them is fiction.
	if _, _, err := resolve.ManifestArtifact(resolve.Config{Root: "x", Repo: config.Config{}, Logger: quiet()}); err == nil {
		t.Fatal("ManifestArtifact accepted an unpinned config")
	}
}

func slicesEqual(a, b []string) bool { return slices.Equal(a, b) }
