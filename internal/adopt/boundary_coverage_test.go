package adopt

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
)

type failingFetcher struct{ err error }

func (f failingFetcher) Fetch(context.Context, string) (io.ReadCloser, error) {
	return nil, f.err
}

func coveragePin() installer.Pin {
	return installer.Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("a", 64)}
}

func writeCoveragePlan(t *testing.T, root string, status Status) {
	t.Helper()
	plan := Plan{SchemaVersion: SchemaVersion, Status: status}
	if status == StatusApplied {
		plan.Epoch = "epoch"
	}
	b, err := plan.Encode()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(PlanPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDispatchAndInspectBoundaryFailures(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Inspect(canceled, Options{Target: t.TempDir()}); err == nil {
		t.Fatal("Inspect ignored a canceled context")
	}
	if _, err := Inspect(context.Background(), Options{Target: " \t"}); err == nil {
		t.Fatal("Inspect accepted an empty target")
	}

	invalidRepo := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalidRepo, ".git"), []byte("not a gitdir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(context.Background(), Options{Target: invalidRepo, Pin: coveragePin()}); err == nil {
		t.Fatal("Inspect hid the git status failure")
	}

	t.Run("routed init failure", func(t *testing.T) {
		root := t.TempDir()
		gitInit(t, root)
		swapInstallerInit(t, func(context.Context, installer.Options) (installer.Report, error) {
			return installer.Report{}, errors.New("init failed")
		})
		if _, err := Inspect(context.Background(), Options{Target: root, Pin: coveragePin()}); err == nil {
			t.Fatal("Inspect hid the routed installer failure")
		}
	})

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(filepath.Join(blocked, "plan.json"), []byte("x")); err == nil {
		t.Fatal("writeFileAtomic accepted a file as its parent")
	}
	destinationDir := filepath.Join(t.TempDir(), "destination")
	if err := os.Mkdir(destinationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(destinationDir, []byte("x")); err == nil {
		t.Fatal("writeFileAtomic replaced a directory")
	}
}

func TestApplyBoundaryFailures(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Apply(canceled, Options{Target: t.TempDir()}); err == nil {
		t.Fatal("Apply ignored a canceled context")
	}
	if _, err := Apply(context.Background(), Options{Target: ""}); err == nil {
		t.Fatal("Apply accepted an empty target")
	}

	planDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(planDir, filepath.FromSlash(PlanPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), Options{Target: planDir}); err == nil {
		t.Fatal("Apply accepted a plan path that is a directory")
	}

	malformed := t.TempDir()
	planPath := filepath.Join(malformed, filepath.FromSlash(PlanPath))
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), Options{Target: malformed}); err == nil {
		t.Fatal("Apply accepted a malformed plan")
	}

	missingPin := t.TempDir()
	writeCoveragePlan(t, missingPin, StatusDraft)
	if _, err := Apply(context.Background(), Options{Target: missingPin}); !errors.Is(err, ErrPinRequired) {
		t.Fatalf("missing-pin error = %v", err)
	}

	fetchFailure := errors.New("offline")
	classification := t.TempDir()
	writeCoveragePlan(t, classification, StatusDraft)
	if _, err := Apply(context.Background(), Options{Target: classification, Pin: coveragePin(), Fetcher: failingFetcher{fetchFailure}}); err == nil {
		t.Fatal("Apply hid the release classification failure")
	}

	if _, err := addedBaselines(t.TempDir(), []Entry{{Path: "missing", Class: ClassAdd}}, ""); err == nil {
		t.Fatal("addedBaselines accepted an absent added path")
	}
}

func TestUndoBoundaryFailures(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Undo(canceled, Options{Target: t.TempDir()}); err == nil {
		t.Fatal("Undo ignored a canceled context")
	}
	if _, err := Undo(context.Background(), Options{Target: ""}); err == nil {
		t.Fatal("Undo accepted an empty target")
	}

	planDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(planDir, filepath.FromSlash(PlanPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Undo(context.Background(), Options{Target: planDir}); err == nil {
		t.Fatal("Undo accepted a plan path that is a directory")
	}

	malformed := t.TempDir()
	planPath := filepath.Join(malformed, filepath.FromSlash(PlanPath))
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Undo(context.Background(), Options{Target: malformed}); err == nil {
		t.Fatal("Undo accepted a malformed plan")
	}

	invalidConfig := t.TempDir()
	writeCoveragePlan(t, invalidConfig, StatusApplied)
	configPath := filepath.Join(invalidConfig, ".pyxis", "config.yaml")
	if err := os.WriteFile(configPath, []byte("unknown: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Undo(context.Background(), Options{Target: invalidConfig, Pin: coveragePin()}); !errors.Is(err, ErrConfigSurfaceInvalid) {
		t.Fatalf("invalid-config error = %v", err)
	}

	missingPin := t.TempDir()
	writeCoveragePlan(t, missingPin, StatusApplied)
	if _, err := Undo(context.Background(), Options{Target: missingPin}); !errors.Is(err, ErrPinRequired) {
		t.Fatalf("missing-pin error = %v", err)
	}

	classification := t.TempDir()
	writeCoveragePlan(t, classification, StatusApplied)
	if _, err := Undo(context.Background(), Options{Target: classification, Pin: coveragePin(), Fetcher: failingFetcher{errors.New("offline")}}); err == nil {
		t.Fatal("Undo hid the release classification failure")
	}

	nonempty := filepath.Join(t.TempDir(), "managed")
	if err := os.Mkdir(nonempty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonempty, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var report UndoReport
	if err := report.remove(filepath.Dir(nonempty), filepath.Base(nonempty)); err == nil {
		t.Fatal("undo removal accepted a non-empty directory")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "changed"), []byte("operator edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := report.removeIfPristine(root, "changed", []byte("baseline")); err != nil {
		t.Fatal(err)
	}
	if len(report.Preserved) != 1 || report.Preserved[0] != "changed" {
		t.Fatalf("preserved paths = %v", report.Preserved)
	}
}
