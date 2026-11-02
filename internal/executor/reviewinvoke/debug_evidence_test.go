package reviewinvoke

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

func TestReviewDebugEvidenceSurvivesRuntimeCleanupWithoutAuthentication(t *testing.T) {
	repo, state := t.TempDir(), t.TempDir()
	i := &Invoker{repoDir: repo, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	evidence := filepath.Join(state, "review-evidence")
	if err := os.Mkdir(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pack.txt", "lenses.json", "screen.json"} {
		if err := os.WriteFile(filepath.Join(evidence, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(state, "auth.json"), []byte("credential"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := i.retainReviewDebug(t.Context(), state, "review-test"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(state); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(repo, runlogs.Reviews, "review-test-*"))
	if err != nil || len(files) != 3 {
		t.Fatalf("retained files=%v err=%v", files, err)
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil || string(data) == "credential" {
			t.Fatalf("retained content: %q %v", data, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("private evidence permissions: %v %v", info, err)
		}
	}
}

func TestReviewDebugEvidenceRefusesEscapesAndUnboundedFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "directory", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			state := t.TempDir()
			i := &Invoker{repoDir: t.TempDir(), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			dir := filepath.Join(state, "review-evidence")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "pack.txt")
			switch kind {
			case "symlink":
				outside := filepath.Join(t.TempDir(), "auth")
				if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "oversize":
				f, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := f.Truncate((8 << 20) + 1); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := i.retainReviewDebug(t.Context(), state, "review-test"); err == nil {
				t.Fatal("unsafe debug source was imported")
			}
		})
	}
	missing := &Invoker{repoDir: t.TempDir(), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := missing.retainReviewDebug(t.Context(), t.TempDir(), "review-test"); err != nil {
		t.Fatal(err)
	}
	if err := missing.retainReviewDebug(t.Context(), filepath.Join(t.TempDir(), "missing"), "review-test"); err == nil {
		t.Fatal("missing runtime accepted")
	}
	if reviewDebugBrief("/private-runtime", "/trusted-host") == "" {
		t.Fatal("missing audit instructions")
	}
}

func TestReviewDebugEvidenceRefusesRedirectedLogDestination(t *testing.T) {
	repo, state := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, runlogs.Root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(repo, runlogs.Reviews)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(state, "review-evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "review-evidence", "pack.txt"), []byte("pack"), 0o600); err != nil {
		t.Fatal(err)
	}
	i := &Invoker{repoDir: repo, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := i.retainReviewDebug(t.Context(), state, "review-test"); err == nil {
		t.Fatal("redirected retention destination accepted")
	}
}
