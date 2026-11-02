package resolve_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// ---- fixture scaffolding -------------------------------------------------

func hexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// corrupt flips the first byte of a file in place.
func corrupt(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(b) == 0 {
		t.Fatalf("corrupt %s: empty file", path)
	}
	b[0] ^= 0xff
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("rewrite %s: %v", path, err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// fx is a fixture release laid out in a directory a file:// artifact source
// (or an httptest file server) serves.
type fx struct {
	dir    string // the artifact-source directory
	rawVer string // the version as configured (may lack the leading v)
	ver    string // the tag form every artifact name embeds
	pin    string // sha256 of the manifest bytes — the config trust root
}

// newFixture writes a complete, valid fixture release: one artifact per
// tool×platform plus the canonically-encoded manifest pinning them all.
// bins maps tool → platform → binary content.
func newFixture(t *testing.T, version string, bins map[string]map[string][]byte) fx {
	t.Helper()
	dir := t.TempDir()
	ver := version
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}
	dummy := strings.Repeat("ab", 32)
	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: ver,
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{},
		Payload: relmanifest.Payload{
			ArchiveSHA256: dummy,
			Files: []relmanifest.PayloadFile{
				{Path: ".pyxis/pipeline.md", SHA256: &dummy, Class: relmanifest.EngineVerbatim},
			},
		},
		DepsBundleSHA256: dummy,
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
	return fx{dir: dir, rawVer: version, ver: ver, pin: hexOf(mb)}
}

func (f fx) manifestName() string { return "release-manifest-" + f.ver + ".json" }

func (f fx) binName(tool, plat string) string { return tool + "-" + f.ver + "-" + plat }

// cfg builds a release-mode resolver config rooted at root, sourcing
// artifacts from the fixture directory over file://.
func (f fx) cfg(root string) resolve.Config {
	return resolve.Config{
		Root: root,
		Repo: config.Config{System: config.System{
			Version:        f.rawVer,
			ManifestSHA256: f.pin,
			ArtifactSource: "file://" + f.dir,
			Mode:           "release",
		}},
		Logger: quiet(),
	}
}

func cachePath(root, name string) string { return filepath.Join(root, ".pyxis", "bin", name) }

// countingFetcher records every fetch URL around an inner fetcher.
type countingFetcher struct {
	inner resolve.Fetcher
	mu    sync.Mutex
	urls  []string
}

func (c *countingFetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	c.mu.Lock()
	c.urls = append(c.urls, url)
	c.mu.Unlock()
	return c.inner.Fetch(ctx, url)
}

func (c *countingFetcher) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.urls)
}

func (c *countingFetcher) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.urls = nil
}

// scriptedFetcher serves canned bodies by artifact name and records URLs —
// the seam for asserting the default base-URL convention with zero network.
type scriptedFetcher struct {
	mu     sync.Mutex
	urls   []string
	bodies map[string][]byte
}

func (s *scriptedFetcher) Fetch(_ context.Context, url string) (io.ReadCloser, error) {
	s.mu.Lock()
	s.urls = append(s.urls, url)
	s.mu.Unlock()
	name := url[strings.LastIndex(url, "/")+1:]
	b, ok := s.bodies[name]
	if !ok {
		return nil, fmt.Errorf("scripted: no artifact %q", name)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

// ---- tests ----------------------------------------------------------------

func TestPlatform(t *testing.T) {
	want := runtime.GOOS + "-" + runtime.GOARCH
	if got := resolve.Platform(); got != want {
		t.Fatalf("Platform() = %q, want %q", got, want)
	}
}

func TestToolColdWarmAndCacheLayout(t *testing.T) {
	plat := resolve.Platform()
	content := []byte("#!/bin/sh\necho docgate fixture\n")
	f := newFixture(t, "0.2.0", map[string]map[string][]byte{"docgate": {plat: content}})
	root := t.TempDir()
	cfg := f.cfg(root)
	counter := &countingFetcher{inner: resolve.ByScheme{}}
	cfg.Fetcher = counter

	// Cold: the manifest then the binary fetch-verify into the cache.
	path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
	if err != nil {
		t.Fatalf("cold Tool: %v", err)
	}
	wantPath := cachePath(root, "docgate-v0.2.0-"+plat) // tag-normalized from "0.2.0"
	if path != wantPath {
		t.Fatalf("exec path = %q, want %q", path, wantPath)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("cached binary bytes wrong (err %v)", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat cached binary: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("cached binary mode = %v, want 0755", info.Mode().Perm())
	}
	if !exists(cachePath(root, "release-manifest-v0.2.0.json")) {
		t.Fatalf("manifest not cached at the shape's named location")
	}
	if counter.count() != 2 {
		t.Fatalf("cold resolve fetches = %d (%v), want 2 (manifest + binary)", counter.count(), counter.urls)
	}
	if !strings.HasSuffix(counter.urls[0], "/"+f.manifestName()) || !strings.HasSuffix(counter.urls[1], "/"+f.binName("docgate", plat)) {
		t.Fatalf("fetch order/names wrong: %v", counter.urls)
	}

	// Warm: a second resolve reuses the cached binary — no re-hash, no fetch.
	counter.reset()
	again, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
	if err != nil {
		t.Fatalf("warm Tool: %v", err)
	}
	if again != path {
		t.Fatalf("warm path %q != cold path %q", again, path)
	}
	if counter.count() != 0 {
		t.Fatalf("warm resolve fetches = %d (%v), want 0", counter.count(), counter.urls)
	}

	// Empty platform derives Platform() — same artifact resolves.
	if derived, err := resolve.Tool(context.Background(), cfg, "docgate", ""); err != nil || derived != path {
		t.Fatalf("derived-platform Tool = (%q, %v), want (%q, nil)", derived, err, path)
	}
}

func TestToolGuardsAndMissingPins(t *testing.T) {
	plat := resolve.Platform()
	f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: []byte("x")}})

	t.Run("empty root", func(t *testing.T) {
		cfg := f.cfg("")
		if _, err := resolve.Tool(context.Background(), cfg, "docgate", plat); err == nil || !strings.Contains(err.Error(), "Config.Root") {
			t.Fatalf("err = %v, want Config.Root refusal", err)
		}
	})
	t.Run("empty tool", func(t *testing.T) {
		cfg := f.cfg(t.TempDir())
		if _, err := resolve.Tool(context.Background(), cfg, "", plat); err == nil || !strings.Contains(err.Error(), "empty tool") {
			t.Fatalf("err = %v, want empty-tool refusal", err)
		}
	})

	pins := []struct {
		name, key string
		mutate    func(*resolve.Config)
	}{
		{"missing version", "system.version", func(c *resolve.Config) { c.Repo.System.Version = "" }},
		{"missing manifest hash", "system.manifest_sha256", func(c *resolve.Config) { c.Repo.System.ManifestSHA256 = "" }},
	}
	for _, tc := range pins {
		t.Run(tc.name, func(t *testing.T) {
			cfg := f.cfg(t.TempDir())
			tc.mutate(&cfg)
			path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
			var mp *resolve.MissingPinError
			if !errors.As(err, &mp) {
				t.Fatalf("err = %v, want *MissingPinError", err)
			}
			if mp.Key != tc.key || path != "" {
				t.Fatalf("got key %q path %q, want key %q path \"\"", mp.Key, path, tc.key)
			}
		})
	}

	t.Run("malformed manifest pin", func(t *testing.T) {
		cfg := f.cfg(t.TempDir())
		cfg.Repo.System.ManifestSHA256 = "zz"
		if _, err := resolve.Tool(context.Background(), cfg, "docgate", plat); err == nil || !strings.Contains(err.Error(), "not a 64-char") {
			t.Fatalf("err = %v, want malformed-pin refusal", err)
		}
	})
}

func TestToolTamperAndTruncationRefusals(t *testing.T) {
	plat := resolve.Platform()
	content := []byte("genuine binary bytes")

	t.Run("tampered manifest", func(t *testing.T) {
		f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: content}})
		corrupt(t, filepath.Join(f.dir, f.manifestName()))
		root := t.TempDir()
		cfg := f.cfg(root)
		counter := &countingFetcher{inner: resolve.ByScheme{}}
		cfg.Fetcher = counter

		path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		var te *resolve.TamperError
		if !errors.As(err, &te) {
			t.Fatalf("err = %v, want *TamperError", err)
		}
		if te.Artifact != f.manifestName() || te.Expected != f.pin || te.Actual == f.pin || te.Actual == "" {
			t.Fatalf("tamper detail wrong: %+v", te)
		}
		if path != "" {
			t.Fatalf("path = %q, want zero execution", path)
		}
		if counter.count() != 1 {
			t.Fatalf("fetches = %d, want 1 (no binary fetch after a tampered manifest)", counter.count())
		}
		if exists(cachePath(root, f.manifestName())) || exists(cachePath(root, f.binName("docgate", plat))) {
			t.Fatalf("tampered bytes were cached")
		}
	})

	bins := []struct {
		name   string
		mutate func(t *testing.T, artifact string)
	}{
		{"tampered binary", func(t *testing.T, artifact string) { corrupt(t, artifact) }},
		{"truncated binary fetch", func(t *testing.T, artifact string) {
			if err := os.Truncate(artifact, 3); err != nil {
				t.Fatalf("truncate: %v", err)
			}
		}},
	}
	for _, tc := range bins {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: content}})
			tc.mutate(t, filepath.Join(f.dir, f.binName("docgate", plat)))
			root := t.TempDir()
			cfg := f.cfg(root)

			path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
			var te *resolve.TamperError
			if !errors.As(err, &te) {
				t.Fatalf("err = %v, want *TamperError", err)
			}
			if te.Artifact != f.binName("docgate", plat) {
				t.Fatalf("artifact = %q, want the binary named", te.Artifact)
			}
			if want := hexOf(content); te.Expected != want || te.Actual == want {
				t.Fatalf("expected/actual not named truthfully: %+v", te)
			}
			if path != "" {
				t.Fatalf("path = %q, want zero execution", path)
			}
			if exists(cachePath(root, f.binName("docgate", plat))) {
				t.Fatalf("unverified binary was installed into the cache")
			}
		})
	}
}

func TestToolOfflineRefusalNamesRemedy(t *testing.T) {
	plat := resolve.Platform()
	f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: []byte("x")}})
	cfg := f.cfg(t.TempDir())
	cfg.Repo.System.ArtifactSource = "file://" + filepath.Join(f.dir, "no-such-dir")

	path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
	var off *resolve.OfflineError
	if !errors.As(err, &off) {
		t.Fatalf("err = %v, want *OfflineError", err)
	}
	if path != "" {
		t.Fatalf("path = %q, want zero execution", path)
	}
	for _, needle := range []string{"pyxis converge", "bootstrap"} {
		if !strings.Contains(err.Error(), needle) {
			t.Fatalf("offline refusal does not name the remedy %q: %v", needle, err)
		}
	}
}

func TestToolWrongPlatform(t *testing.T) {
	plat := resolve.Platform()
	f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {"otheros-otherarch": []byte("x")}})
	cfg := f.cfg(t.TempDir())

	t.Run("platform not shipped", func(t *testing.T) {
		_, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		var wp *resolve.WrongPlatformError
		if !errors.As(err, &wp) {
			t.Fatalf("err = %v, want *WrongPlatformError", err)
		}
		if wp.Tool != "docgate" || wp.Platform != plat || len(wp.Known) != 1 || wp.Known[0] != "otheros-otherarch" {
			t.Fatalf("detail wrong: %+v", wp)
		}
		if !strings.Contains(err.Error(), "otheros-otherarch") {
			t.Fatalf("refusal does not name what the release ships: %v", err)
		}
	})
	t.Run("tool not in manifest", func(t *testing.T) {
		_, err := resolve.Tool(context.Background(), cfg, "nosuchtool", plat)
		var wp *resolve.WrongPlatformError
		if !errors.As(err, &wp) {
			t.Fatalf("err = %v, want *WrongPlatformError", err)
		}
		if wp.Tool != "nosuchtool" || len(wp.Known) != 0 {
			t.Fatalf("detail wrong: %+v", wp)
		}
	})
}

// The cache is verified once, at fetch, then trusted: a version-named binary
// already present is returned as-is on later calls — no re-hash, no refetch —
// even after an on-disk edit. The per-exec re-hash floor was removed as cost
// without a matching threat (guarding .pyxis/bin/ from local edits is the
// filesystem's job). Fetch-time verification of a tampered SOURCE still refuses
// — covered by the tampered/truncated-binary cases elsewhere in this file.
func TestToolTrustsCachedBinary(t *testing.T) {
	plat := resolve.Platform()
	content := []byte("genuine binary bytes")
	f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: content}})
	root := t.TempDir()
	cfg := f.cfg(root)
	counter := &countingFetcher{inner: resolve.ByScheme{}}
	cfg.Fetcher = counter

	path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
	if err != nil {
		t.Fatalf("cold Tool: %v", err)
	}

	// A later call reuses the cached binary with no fetch.
	counter.reset()
	again, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
	if err != nil || again != path {
		t.Fatalf("warm Tool = (%q, %v), want (%q, nil)", again, err, path)
	}
	if counter.count() != 0 {
		t.Fatalf("warm fetches = %d, want 0 (cached binary trusted)", counter.count())
	}

	// An on-disk edit to the cache is trusted, not caught: still no fetch, and
	// the cached path is returned as-is (the per-exec re-hash is gone).
	corrupt(t, path)
	counter.reset()
	trusted, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
	if err != nil {
		t.Fatalf("Tool over an edited cache entry: %v", err)
	}
	if trusted != path {
		t.Fatalf("path = %q, want the cached %q trusted as-is", trusted, path)
	}
	if counter.count() != 0 {
		t.Fatalf("fetches = %d, want 0 (a present cache entry is trusted, never re-hashed)", counter.count())
	}
}

func TestToolSourceMode(t *testing.T) {
	plat := resolve.Platform()

	t.Run("mode source with the cmd tree", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "cmd", "docgate"), 0o755); err != nil {
			t.Fatal(err)
		}
		// No pins at all: source mode must not require the release pin pair.
		cfg := resolve.Config{Root: root, Repo: config.Config{System: config.System{Mode: "source"}}, Logger: quiet()}
		path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		if path != "" {
			t.Fatalf("path = %q, want \"\" in source mode", path)
		}
		if !errors.Is(err, resolve.ErrSourceMode) {
			t.Fatalf("err = %v, want ErrSourceMode", err)
		}
		var sm *resolve.SourceMode
		if !errors.As(err, &sm) {
			t.Fatalf("err = %v, want *SourceMode", err)
		}
		if sm.Tool != "docgate" || sm.RepoRoot != root || sm.CmdDir != filepath.Join(root, "cmd", "docgate") {
			t.Fatalf("SourceMode detail wrong: %+v", sm)
		}
	})

	t.Run("mode source without the cmd tree refuses", func(t *testing.T) {
		cfg := resolve.Config{Root: t.TempDir(), Repo: config.Config{System: config.System{Mode: "source"}}, Logger: quiet()}
		_, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		var sta *resolve.SourceTreeAbsentError
		if !errors.As(err, &sta) {
			t.Fatalf("err = %v, want *SourceTreeAbsentError", err)
		}
		if !strings.Contains(err.Error(), "DINV-8") {
			t.Fatalf("refusal does not cite DINV-8: %v", err)
		}
	})

	t.Run("legacy per-tool source pin signals during the window", func(t *testing.T) {
		f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"codegate": {plat: []byte("codegate bytes")}})
		yaml := "schema_version: pyxis-config/v2\n" +
			"system:\n" +
			"  version: \"" + f.rawVer + "\"\n" +
			"  manifest_sha256: \"" + f.pin + "\"\n" +
			"  artifact_source: \"file://" + f.dir + "\"\n" +
			"  mode: release\n" +
			"docgate:\n" +
			"  version: \"source\"\n"
		repo, defects := config.Parse([]byte(yaml))
		if len(defects) != 0 {
			t.Fatalf("fixture config has defects: %v", defects)
		}
		if len(repo.Deprecations()) != 1 {
			t.Fatalf("want exactly one dual-read deprecation notice, got %v", repo.Deprecations())
		}
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "cmd", "docgate"), 0o755); err != nil {
			t.Fatal(err)
		}
		cfg := resolve.Config{Root: root, Repo: repo, Logger: quiet()}

		// docgate: the legacy "source" pin wins — source fall-through.
		_, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		if !errors.Is(err, resolve.ErrSourceMode) {
			t.Fatalf("docgate err = %v, want ErrSourceMode via the legacy signal", err)
		}
		// codegate: no legacy source pin — the release path resolves.
		path, err := resolve.Tool(context.Background(), cfg, "codegate", plat)
		if err != nil {
			t.Fatalf("codegate Tool: %v", err)
		}
		if got, _ := os.ReadFile(path); !bytes.Equal(got, []byte("codegate bytes")) {
			t.Fatalf("codegate resolved to wrong bytes")
		}
	})

	t.Run("the per-tool default is never a signal", func(t *testing.T) {
		// A pure v2 release config never mentions docgate:, yet T2's
		// DocgateVersion field defaults to "source" — that default must not
		// push an installed repo into source mode.
		content := []byte("release docgate")
		f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: content}})
		cfg := f.cfg(t.TempDir())
		cfg.Repo.DocgateVersion = "source" // exactly what config.Defaults() yields
		path, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		if err != nil {
			t.Fatalf("Tool: %v (the default must resolve via the release path)", err)
		}
		if got, _ := os.ReadFile(path); !bytes.Equal(got, content) {
			t.Fatalf("resolved wrong bytes")
		}
	})
}

func TestPreWarm(t *testing.T) {
	plat := resolve.Platform()
	bins := map[string]map[string][]byte{
		"docgate":   {plat: []byte("docgate bytes")},
		"codegate":  {plat: []byte("codegate bytes")},
		"othertool": {"otheros-otherarch": []byte("elsewhere")},
	}

	t.Run("warms the current platform set, then no refetch", func(t *testing.T) {
		f := newFixture(t, "v0.1.0", bins)
		root := t.TempDir()
		cfg := f.cfg(root)
		counter := &countingFetcher{inner: resolve.ByScheme{}}
		cfg.Fetcher = counter

		if err := resolve.PreWarm(context.Background(), cfg); err != nil {
			t.Fatalf("PreWarm: %v", err)
		}
		if counter.count() != 3 {
			t.Fatalf("fetches = %d (%v), want 3 (manifest + 2 platform binaries)", counter.count(), counter.urls)
		}
		for _, tool := range []string{"docgate", "codegate"} {
			if !exists(cachePath(root, f.binName(tool, plat))) {
				t.Fatalf("%s not pre-warmed", tool)
			}
		}
		if exists(cachePath(root, f.binName("othertool", "otheros-otherarch"))) {
			t.Fatalf("othertool fetched despite not shipping for this platform")
		}

		counter.reset()
		if err := resolve.PreWarm(context.Background(), cfg); err != nil {
			t.Fatalf("second PreWarm: %v", err)
		}
		if counter.count() != 0 {
			t.Fatalf("warm PreWarm fetches = %d, want 0", counter.count())
		}
	})

	t.Run("source mode has nothing to warm", func(t *testing.T) {
		counter := &countingFetcher{inner: resolve.ByScheme{}}
		cfg := resolve.Config{Root: t.TempDir(), Repo: config.Config{System: config.System{Mode: "source"}}, Fetcher: counter, Logger: quiet()}
		if err := resolve.PreWarm(context.Background(), cfg); err != nil {
			t.Fatalf("PreWarm: %v", err)
		}
		if counter.count() != 0 {
			t.Fatalf("source-mode PreWarm fetched %d times", counter.count())
		}
	})

	t.Run("a missing artifact fails by name without hiding the rest", func(t *testing.T) {
		f := newFixture(t, "v0.1.0", bins)
		if err := os.Remove(filepath.Join(f.dir, f.binName("codegate", plat))); err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		cfg := f.cfg(root)
		err := resolve.PreWarm(context.Background(), cfg)
		var off *resolve.OfflineError
		if !errors.As(err, &off) {
			t.Fatalf("err = %v, want a joined *OfflineError", err)
		}
		if !strings.Contains(err.Error(), "codegate") {
			t.Fatalf("failure does not name the missing tool: %v", err)
		}
		if !exists(cachePath(root, f.binName("docgate", plat))) {
			t.Fatalf("docgate not warmed despite codegate's failure (attempt-all contract)")
		}
	})
}

func TestPinBumpPreWarmThenReap(t *testing.T) {
	plat := resolve.Platform()
	root := t.TempDir()

	f1 := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: []byte("old")}})
	if _, err := resolve.Tool(context.Background(), f1.cfg(root), "docgate", plat); err != nil {
		t.Fatalf("warm v0.1.0: %v", err)
	}
	writeFile(t, cachePath(root, "notes.txt"), []byte("not a cache entry"))

	f2 := newFixture(t, "v0.2.0", map[string]map[string][]byte{
		"docgate":  {plat: []byte("new docgate")},
		"codegate": {plat: []byte("new codegate")},
	})
	cfg2 := f2.cfg(root)
	if err := resolve.PreWarm(context.Background(), cfg2); err != nil {
		t.Fatalf("PreWarm v0.2.0: %v", err)
	}

	removed, err := resolve.Reap(cfg2)
	if err != nil {
		t.Fatalf("Reap: %v", err)
	}
	want := []string{"docgate-v0.1.0-" + plat, "release-manifest-v0.1.0.json"}
	if len(removed) != len(want) || removed[0] != want[0] || removed[1] != want[1] {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	for _, gone := range want {
		if exists(cachePath(root, gone)) {
			t.Fatalf("%s survived the reap", gone)
		}
	}
	for _, kept := range []string{
		f2.manifestName(), f2.binName("docgate", plat), f2.binName("codegate", plat), "notes.txt",
	} {
		if !exists(cachePath(root, kept)) {
			t.Fatalf("%s was wrongly reaped", kept)
		}
	}
}

func TestReapGrammarAndSafety(t *testing.T) {
	root := t.TempDir()
	cfg := resolve.Config{
		Root:   root,
		Repo:   config.Config{System: config.System{Version: "v0.1.0", Mode: "release"}},
		Logger: quiet(),
	}

	t.Run("missing cache dir is a clean no-op", func(t *testing.T) {
		removed, err := resolve.Reap(cfg)
		if err != nil || removed != nil {
			t.Fatalf("Reap = (%v, %v), want (nil, nil)", removed, err)
		}
	})

	t.Run("missing version pin refuses", func(t *testing.T) {
		_, err := resolve.Reap(resolve.Config{Root: root, Logger: quiet()})
		var mp *resolve.MissingPinError
		if !errors.As(err, &mp) || mp.Key != "system.version" {
			t.Fatalf("err = %v, want MissingPinError(system.version)", err)
		}
	})

	t.Run("only off-version cache entries go", func(t *testing.T) {
		keep := []string{
			"docgate-v0.1.0-darwin-arm64",        // pinned version
			"release-manifest-v0.1.0.json",       // pinned version
			"release-manifest-.json",             // empty version: not an entry
			"docgate-source-darwin-arm64",        // no v<digit> segment: not an entry
			"notes.txt",                          // not an entry
			"SHA256SUMS",                         // not an entry
			"docgate-v0.9.9-darwin-arm64.tmp-42", // in-flight fetch turf
		}
		reap := []string{
			"docgate-v0.9.9-darwin-arm64",
			"docgate-v0.9.9-rc1-darwin-arm64", // multi-segment (prerelease) version
			"release-manifest-v0.9.9.json",
		}
		for _, name := range append(append([]string{}, keep...), reap...) {
			writeFile(t, cachePath(root, name), []byte(name))
		}
		writeFile(t, filepath.Join(root, ".pyxis", "bin", "sub", "inner"), []byte("subdir content"))

		removed, err := resolve.Reap(cfg)
		if err != nil {
			t.Fatalf("Reap: %v", err)
		}
		if len(removed) != len(reap) {
			t.Fatalf("removed = %v, want exactly %v", removed, reap)
		}
		for i, name := range reap {
			if removed[i] != name {
				t.Fatalf("removed = %v, want sorted %v", removed, reap)
			}
			if exists(cachePath(root, name)) {
				t.Fatalf("%s survived", name)
			}
		}
		for _, name := range keep {
			if !exists(cachePath(root, name)) {
				t.Fatalf("%s was wrongly reaped", name)
			}
		}
		if !exists(filepath.Join(root, ".pyxis", "bin", "sub", "inner")) {
			t.Fatalf("subdirectory content touched")
		}
	})
}

func TestToolOverrideAlternateSourceSameVerification(t *testing.T) {
	plat := resolve.Platform()
	content := []byte("identical binary bytes")
	primary := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: content}})
	alternate := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: content}})
	if primary.pin != alternate.pin {
		t.Fatalf("fixture encoding not deterministic: %s vs %s", primary.pin, alternate.pin)
	}

	// The override changes where bytes come from…
	cfg := primary.cfg(t.TempDir())
	cfg.Repo.System.ArtifactSource = "file://" + alternate.dir
	if _, err := resolve.Tool(context.Background(), cfg, "docgate", plat); err != nil {
		t.Fatalf("Tool via override source: %v", err)
	}

	// …never whether they are accepted: a tampered alternate still refuses.
	corrupt(t, filepath.Join(alternate.dir, alternate.binName("docgate", plat)))
	cfg2 := primary.cfg(t.TempDir())
	cfg2.Repo.System.ArtifactSource = "file://" + alternate.dir
	_, err := resolve.Tool(context.Background(), cfg2, "docgate", plat)
	var te *resolve.TamperError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want *TamperError under the override", err)
	}
}

func TestDefaultBaseURLConvention(t *testing.T) {
	plat := resolve.Platform()
	f := newFixture(t, "0.3.0", map[string]map[string][]byte{"docgate": {plat: []byte("x")}})
	manifestBytes, err := os.ReadFile(filepath.Join(f.dir, f.manifestName()))
	if err != nil {
		t.Fatal(err)
	}
	binBytes, err := os.ReadFile(filepath.Join(f.dir, f.binName("docgate", plat)))
	if err != nil {
		t.Fatal(err)
	}
	scripted := &scriptedFetcher{bodies: map[string][]byte{
		f.manifestName():           manifestBytes,
		f.binName("docgate", plat): binBytes,
	}}
	cfg := f.cfg(t.TempDir())
	cfg.Repo.System.ArtifactSource = "" // the built-in channel applies
	cfg.Fetcher = scripted

	if _, err := resolve.Tool(context.Background(), cfg, "docgate", plat); err != nil {
		t.Fatalf("Tool: %v", err)
	}
	base := resolve.DefaultChannelRoot + "/v0.3.0"
	wantURLs := []string{base + "/release-manifest-v0.3.0.json", base + "/docgate-v0.3.0-" + plat}
	if len(scripted.urls) != 2 || scripted.urls[0] != wantURLs[0] || scripted.urls[1] != wantURLs[1] {
		t.Fatalf("fetched URLs = %v, want %v", scripted.urls, wantURLs)
	}
}

func TestManifestMetadataDefects(t *testing.T) {
	plat := resolve.Platform()

	t.Run("system_version incoherent with the pin", func(t *testing.T) {
		f := newFixture(t, "v9.9.9", map[string]map[string][]byte{"docgate": {plat: []byte("x")}})
		// Serve the v9.9.9 manifest under the v0.1.0 name the v0.1.0 pin asks for.
		mb, err := os.ReadFile(filepath.Join(f.dir, "release-manifest-v9.9.9.json"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(f.dir, "release-manifest-v0.1.0.json"), mb)
		cfg := f.cfg(t.TempDir())
		cfg.Repo.System.Version = "v0.1.0"
		_, err = resolve.Tool(context.Background(), cfg, "docgate", plat)
		if err == nil || !strings.Contains(err.Error(), "v9.9.9") || !strings.Contains(err.Error(), "v0.1.0") {
			t.Fatalf("err = %v, want the pin-pair incoherence named with both versions", err)
		}
	})

	t.Run("pinned bytes that do not parse", func(t *testing.T) {
		dir := t.TempDir()
		junk := []byte(`{"nope": true}`)
		writeFile(t, filepath.Join(dir, "release-manifest-v0.1.0.json"), junk)
		cfg := resolve.Config{
			Root: t.TempDir(),
			Repo: config.Config{System: config.System{
				Version:        "v0.1.0",
				ManifestSHA256: hexOf(junk), // pin-true, schema-false
				ArtifactSource: "file://" + dir,
				Mode:           "release",
			}},
			Logger: quiet(),
		}
		_, err := resolve.Tool(context.Background(), cfg, "docgate", plat)
		if err == nil || !strings.Contains(err.Error(), "does not parse") {
			t.Fatalf("err = %v, want the pinned-metadata parse defect named", err)
		}
	})
}

func TestToolContextCanceled(t *testing.T) {
	plat := resolve.Platform()
	f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: []byte("x")}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolve.Tool(ctx, f.cfg(t.TempDir()), "docgate", plat); !errors.Is(err, context.Canceled) {
		t.Fatalf("Tool err = %v, want context.Canceled", err)
	}
	if err := resolve.PreWarm(ctx, f.cfg(t.TempDir())); !errors.Is(err, context.Canceled) {
		t.Fatalf("PreWarm err = %v, want context.Canceled", err)
	}
}

func TestToolConcurrentColdResolve(t *testing.T) {
	plat := resolve.Platform()
	content := []byte("racing binary bytes")
	f := newFixture(t, "v0.1.0", map[string]map[string][]byte{"docgate": {plat: content}})
	cfg := f.cfg(t.TempDir())

	const n = 8
	paths := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			paths[i], errs[i] = resolve.Tool(context.Background(), cfg, "docgate", plat)
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if paths[i] != paths[0] {
			t.Fatalf("divergent paths: %q vs %q", paths[i], paths[0])
		}
	}
	if got, err := os.ReadFile(paths[0]); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("cache corrupted by concurrent cold resolve (err %v)", err)
	}
}
