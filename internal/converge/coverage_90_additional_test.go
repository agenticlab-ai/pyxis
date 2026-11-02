package converge

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

type coverageFailFetcher struct{ err error }

func (f coverageFailFetcher) Fetch(context.Context, string) (io.ReadCloser, error) {
	return nil, f.err
}

func TestWriteGeneratedHonorsCancellationBeforeMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	actions, refusals, err := WriteGenerated(ctx, t.TempDir(), []Finding{{Path: "generated.md", Class: relmanifest.Generated}}, map[string][]byte{"generated.md": []byte("data")})
	if !errors.Is(err, context.Canceled) || len(actions) != 0 || len(refusals) != 0 {
		t.Fatalf("WriteGenerated = actions %+v refusals %+v err %v", actions, refusals, err)
	}
}

func TestPreviewReportsIndependentReadFailures(t *testing.T) {
	const version = "v9.9.9"
	validPins := config.Config{System: config.System{Version: version, ManifestSHA256: strings.Repeat("a", 64)}}

	t.Run("reap plan", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "bin"), []byte("blocked"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := preview(context.Background(), resolve.Config{Root: root, Repo: validPins}, Options{}); err == nil || !strings.Contains(err.Error(), "reap") {
			t.Fatalf("preview error = %v", err)
		}
	})

	t.Run("cached manifest", func(t *testing.T) {
		root := t.TempDir()
		manifestPath := filepath.Join(root, ".pyxis", "bin", "release-manifest-"+version+".json")
		if err := os.MkdirAll(manifestPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := preview(context.Background(), resolve.Config{Root: root, Repo: validPins}, Options{}); err == nil || !strings.Contains(err.Error(), "cached") {
			t.Fatalf("preview error = %v", err)
		}
	})

	t.Run("tree scan", func(t *testing.T) {
		root := t.TempDir()
		want := strings.Repeat("b", 64)
		cfg := coverageCachedManifest(t, root, relmanifest.PayloadFile{Path: "managed.txt", Class: relmanifest.EngineVerbatim, SHA256: &want})
		if err := os.Symlink("managed.txt", filepath.Join(root, "managed.txt")); err != nil {
			t.Fatal(err)
		}
		if _, err := preview(context.Background(), cfg, Options{}); err == nil || !strings.Contains(err.Error(), "hash managed.txt") {
			t.Fatalf("preview error = %v", err)
		}
	})

	t.Run("cached payload", func(t *testing.T) {
		root := t.TempDir()
		cfg := coverageCachedManifest(t, root, relmanifest.PayloadFile{Path: ".pyxis/config.yaml", Class: relmanifest.RepoOwned})
		if err := os.Mkdir(filepath.Join(root, ".pyxis", "bin", "payload-"+version+".tar.gz"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := preview(context.Background(), cfg, Options{}); err == nil || !strings.Contains(err.Error(), "cached payload") {
			t.Fatalf("preview error = %v", err)
		}
	})
}

func TestConvergeEntryPointsReportPayloadAndTreeFailures(t *testing.T) {
	t.Run("run payload acquisition", func(t *testing.T) {
		root := t.TempDir()
		cfg := coverageCachedManifest(t, root, relmanifest.PayloadFile{Path: ".pyxis/config.yaml", Class: relmanifest.RepoOwned})
		want := errors.New("payload unavailable")
		cfg.Fetcher = coverageFailFetcher{err: want}
		if _, err := Run(context.Background(), cfg, Options{}); !errors.Is(err, want) {
			t.Fatalf("Run error = %v", err)
		}
	})

	for _, tc := range []struct {
		name string
		call func(context.Context, resolve.Config) error
	}{
		{name: "run tree scan", call: func(ctx context.Context, cfg resolve.Config) error {
			_, err := Run(ctx, cfg, Options{})
			return err
		}},
		{name: "drift tree scan", call: func(ctx context.Context, cfg resolve.Config) error {
			_, err := Drift(ctx, cfg)
			return err
		}},
		{name: "archive classification tree scan", call: func(ctx context.Context, cfg resolve.Config) error {
			_, _, err := ClassifyWithArchive(ctx, cfg)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			want := strings.Repeat("b", 64)
			cfg := coverageCachedManifest(t, root, relmanifest.PayloadFile{Path: "managed.txt", Class: relmanifest.EngineVerbatim, SHA256: &want})
			if err := os.WriteFile(filepath.Join(root, ".pyxis", "bin", "payload-v9.9.9.tar.gz"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("managed.txt", filepath.Join(root, "managed.txt")); err != nil {
				t.Fatal(err)
			}
			if err := tc.call(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "hash managed.txt") {
				t.Fatalf("entry point error = %v", err)
			}
		})
	}
}

func coverageCachedManifest(t *testing.T, root string, file relmanifest.PayloadFile) resolve.Config {
	t.Helper()
	const version = "v9.9.9"
	manifest := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: version,
		SourceCommit:  "coverage",
		Binaries:      map[string]map[string]string{},
		Payload: relmanifest.Payload{
			ArchiveSHA256: sha256Hex(nil),
			Files:         []relmanifest.PayloadFile{file},
		},
		DepsBundleSHA256: strings.Repeat("d", 64),
	}
	body, err := manifest.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, ".pyxis", "bin")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "release-manifest-"+version+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return resolve.Config{Root: root, Repo: config.Config{System: config.System{Version: version, ManifestSHA256: sha256Hex(body)}}}
}
