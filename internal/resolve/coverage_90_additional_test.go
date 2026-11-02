package resolve

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

func coverage90Config(root string) Config {
	return Config{Root: root, Repo: config.Config{System: config.System{Version: "1.0.0", ManifestSHA256: strings.Repeat("a", 64), Mode: "release"}}}
}

func TestCoverage90EnsureReadOnlyAndGuardBoundaries(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EnsureManifest(canceled, coverage90Config(t.TempDir())); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled manifest ensure = %v", err)
	}
	if _, err := EnsurePayload(t.Context(), Config{}, relmanifest.Manifest{}); err == nil {
		t.Fatal("payload ensure accepted an empty root")
	}
	if _, _, err := CachedPayload(Config{}, relmanifest.Manifest{}); err == nil {
		t.Fatal("cached payload accepted an empty root")
	}
	if _, err := PreWarmPlan(Config{}, relmanifest.Manifest{}); err == nil {
		t.Fatal("pre-warm plan accepted an empty root")
	}

	root := t.TempDir()
	cfg := coverage90Config(root)
	manifestPath := filepath.Join(cfg.cacheDir(), manifestName("v1.0.0"))
	if err := os.MkdirAll(manifestPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CachedManifest(cfg); err == nil {
		t.Fatal("cached manifest accepted a directory as the manifest file")
	}

	payload := relmanifest.Manifest{SystemVersion: "1.0.0", Payload: relmanifest.Payload{ArchiveSHA256: strings.Repeat("b", 64)}}
	payloadPath := filepath.Join(cfg.cacheDir(), payloadName("v1.0.0"))
	if err := os.MkdirAll(payloadPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CachedPayload(cfg, payload); err == nil {
		t.Fatal("cached payload accepted a directory as the archive")
	}

	loopRoot := t.TempDir()
	loopCfg := coverage90Config(loopRoot)
	loopCfg.Platform = "test-os"
	loopManifest := relmanifest.Manifest{SystemVersion: "1.0.0", Binaries: map[string]map[string]string{"tool": {"test-os": strings.Repeat("c", 64)}}}
	loopPath := filepath.Join(loopCfg.cacheDir(), binaryName("tool", "v1.0.0", "test-os"))
	if err := os.MkdirAll(filepath.Dir(loopPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(loopPath, loopPath); err != nil {
		t.Fatal(err)
	}
	if _, err := PreWarmPlan(loopCfg, loopManifest); err == nil {
		t.Fatal("pre-warm plan ignored a cache stat failure")
	}
}

func TestCoverage90GitHubAssetTransportAndPaginationBoundaries(t *testing.T) {
	assets := newAssetMap()
	assets.put("o/r/v1", map[string]int64{"asset": 7})
	assetURL := "https://github.test/o/r/releases/download/v1/asset"

	g := githubAuthFetcher{downloadHost: "github.test", apiBase: "https://api.test", assets: assets, client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport failed")
	})}}
	if _, err := g.Fetch(t.Context(), assetURL); err == nil {
		t.Fatal("asset transport failure was ignored")
	}
	g.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return stringResp(r, http.StatusServiceUnavailable, "offline"), nil
	})}
	if _, err := g.Fetch(t.Context(), assetURL); err == nil {
		t.Fatal("non-OK asset response was accepted")
	}
	var nilContext context.Context
	if _, err := g.Fetch(nilContext, assetURL); err == nil {
		t.Fatal("asset fetch accepted a nil context")
	}

	g = githubAuthFetcher{apiBase: "https://api.test", client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/tags/") {
			return stringResp(r, http.StatusOK, `{"id":99}`), nil
		}
		return stringResp(r, http.StatusInternalServerError, "failed"), nil
	})}}
	if _, err := g.listReleaseAssets(t.Context(), "o", "r", "v1"); err == nil {
		t.Fatal("asset-list failure was ignored")
	}

	var page strings.Builder
	page.WriteByte('[')
	for i := range assetsPerPage {
		if i > 0 {
			page.WriteByte(',')
		}
		fmt.Fprintf(&page, `{"name":"asset-%d","id":%d}`, i, i+1)
	}
	page.WriteByte(']')
	g.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/tags/") {
			return stringResp(r, http.StatusOK, `{"id":99}`), nil
		}
		return stringResp(r, http.StatusOK, page.String()), nil
	})}
	if _, err := g.listReleaseAssets(t.Context(), "o", "r", "v1"); err == nil {
		t.Fatal("unbounded full-page asset listing was accepted")
	}
}

func TestCoverage90ResolvedTokenBoundary(t *testing.T) {
	priorCached := tokenCached
	tokenOnce, tokenCached = sync.Once{}, ""
	t.Setenv("GH_TOKEN", "coverage-token")
	t.Cleanup(func() { tokenOnce, tokenCached = sync.Once{}, priorCached })
	if got := resolvedToken(); got != "coverage-token" {
		t.Fatalf("resolved environment token = %q", got)
	}
}

func TestCoverage90ResolverUtilityBoundaries(t *testing.T) {
	if got := (Config{Platform: "custom-os"}).platform(); got != "custom-os" {
		t.Fatalf("custom platform = %q", got)
	}
	if _, ok := (Config{Repo: config.Config{System: config.System{ArtifactSource: "file:///tmp/releases"}}}).fetcher().(ByScheme); !ok {
		t.Fatal("file source did not select the scheme dispatcher")
	}
	if sourceSignaled(config.Config{}, "foreign") {
		t.Fatal("foreign legacy tool signaled source mode")
	}
	if isHex64(strings.Repeat("g", 64)) {
		t.Fatal("non-hex digest was accepted")
	}
	if _, err := Reap(Config{}); err == nil {
		t.Fatal("reap accepted an empty root")
	}

	root := t.TempDir()
	path := filepath.Join(root, "cmd", "tool")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, path); err != nil {
		t.Fatal(err)
	}
	if err := sourceFallThrough(Config{Root: root}, "tool"); err == nil {
		t.Fatal("source fall-through ignored a source-tree stat failure")
	}
}
