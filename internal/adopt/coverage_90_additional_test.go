package adopt

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverage90DispatchApplyAndBaselineBoundaries(t *testing.T) {
	root := t.TempDir()
	loop := filepath.Join(root, ".git")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(t.Context(), Options{Target: root, Pin: coveragePin()}); err == nil {
		t.Fatal("Inspect ignored an unreadable git marker")
	}

	invalidConfig := t.TempDir()
	writeCoveragePlan(t, invalidConfig, StatusDraft)
	configPath := filepath.Join(invalidConfig, filepath.FromSlash(configRelPath))
	if err := os.WriteFile(configPath, []byte("unknown: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(t.Context(), Options{Target: invalidConfig, Pin: coveragePin()}); !errors.Is(err, ErrConfigSurfaceInvalid) {
		t.Fatalf("invalid config error = %v", err)
	}

	baselineRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(baselineRoot, "added"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := addedBaselines(baselineRoot, []Entry{{Path: configRelPath, Class: ClassAdd}, {Path: "added", Class: ClassAdd}}, strings.Repeat("a", 64))
	if err != nil || len(added) != 2 || added[1].SHA256 == "" {
		t.Fatalf("added baselines = %+v, %v", added, err)
	}
}

func TestCoverage90AtomicWriterInjectedFailures(t *testing.T) {
	want := errors.New("atomic write failed")
	priorCreate, priorChmod := createAdoptTemp, chmodAdoptTemp
	t.Cleanup(func() { createAdoptTemp, chmodAdoptTemp = priorCreate, priorChmod })

	createAdoptTemp = func(string, string) (*os.File, error) { return nil, want }
	if err := writeFileAtomic(filepath.Join(t.TempDir(), "plan.json"), []byte("body")); !errors.Is(err, want) {
		t.Fatalf("create-temp failure = %v", err)
	}

	createAdoptTemp = func(dir, pattern string) (*os.File, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err == nil {
			err = f.Close()
		}
		return f, err
	}
	if err := writeFileAtomic(filepath.Join(t.TempDir(), "plan.json"), []byte("body")); err == nil {
		t.Fatal("write to a closed temp file succeeded")
	}

	createAdoptTemp = os.CreateTemp
	chmodAdoptTemp = func(string, os.FileMode) error { return want }
	if err := writeFileAtomic(filepath.Join(t.TempDir(), "plan.json"), []byte("body")); !errors.Is(err, want) {
		t.Fatalf("chmod failure = %v", err)
	}
}

func TestCoverage90HookChainFailureBoundaries(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if err := gitConfigUnset(t.Context(), repo, "pyxis.coverage.absent"); err != nil {
		t.Fatalf("idempotent unset = %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if err := UndoHookChain(t.Context(), missing, Hooks{PriorHooksPath: ".husky"}); err == nil {
		t.Fatal("hook restore ignored an invalid repository")
	}

	gitDir := filepath.Join(repo, ".git")
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, 0o755) })
	if _, err := ApplyHookChain(t.Context(), repo, Hooks{}); err == nil {
		t.Fatal("hook chaining ignored a read-only git directory")
	}
}
