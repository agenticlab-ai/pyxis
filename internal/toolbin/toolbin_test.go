package toolbin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func TestEnsureBuildsFromSourceWhenPathIsEmpty(t *testing.T) {
	root := writeSourceTool(t, "hello", "package main\nfunc main() {}\n")
	t.Setenv("PATH", goBinDir(t))
	got, err := Ensure(t.Context(), root, "hello")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".pyxis", "bin", "hello")
	if got != want {
		t.Fatalf("Ensure = %q, want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("built binary missing: %v", err)
	}
}

func TestEnsureUsesPathWhenSourceTreeIsAbsent(t *testing.T) {
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "carina")
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	root := t.TempDir()
	got, err := Ensure(t.Context(), root, "carina")
	if err != nil {
		t.Fatal(err)
	}
	if got != bin {
		t.Fatalf("Ensure = %q, want PATH binary %q", got, bin)
	}
}

func TestEnsureRefusesMissingToolAndEmptyInputs(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Ensure(t.Context(), t.TempDir(), "carina"); err == nil {
		t.Fatal("missing tool succeeded")
	} else if !strings.Contains(err.Error(), "carina not on PATH") {
		t.Fatalf("error = %v", err)
	}
	if _, err := Ensure(t.Context(), "", "carina"); err == nil {
		t.Fatal("empty root succeeded")
	}
	if _, err := Ensure(t.Context(), t.TempDir(), ""); err == nil {
		t.Fatal("empty tool succeeded")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Ensure(ctx, t.TempDir(), "carina"); err == nil {
		t.Fatal("canceled context succeeded")
	}
}

func TestEnsureUsesAResolvedReleaseBinary(t *testing.T) {
	restore := resolveTool
	t.Cleanup(func() { resolveTool = restore })
	resolveTool = func(context.Context, resolve.Config, string, string) (string, error) {
		return "/cached/carina", nil
	}
	got, err := Ensure(t.Context(), t.TempDir(), "carina")
	if err != nil || got != "/cached/carina" {
		t.Fatalf("Ensure = %q, %v", got, err)
	}
}

func TestEnsureWrapsANonAbsentResolveFailure(t *testing.T) {
	restore := resolveTool
	t.Cleanup(func() { resolveTool = restore })
	resolveTool = func(context.Context, resolve.Config, string, string) (string, error) {
		return "", errors.New("offline pin")
	}
	t.Setenv("PATH", t.TempDir())
	_, err := Ensure(t.Context(), t.TempDir(), "carina")
	if err == nil || !strings.Contains(err.Error(), "resolve carina: offline pin") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildFromSourceReportsMissingGoAndCanceledContext(t *testing.T) {
	restore := lookPath
	t.Cleanup(func() { lookPath = restore })
	lookPath = func(string) (string, error) { return "", errors.New("missing go") }
	_, err := buildFromSource(t.Context(), resolve.SourceMode{Tool: "carina", RepoRoot: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "needs the Go toolchain") {
		t.Fatalf("error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := buildFromSource(ctx, resolve.SourceMode{Tool: "carina", RepoRoot: t.TempDir()}); err == nil {
		t.Fatal("canceled build succeeded")
	}
	fileRoot := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(fileRoot, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	lookPath = exec.LookPath
	_, err = buildFromSource(t.Context(), resolve.SourceMode{Tool: "carina", RepoRoot: fileRoot})
	if err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureReportsAFailedSourceBuild(t *testing.T) {
	root := writeSourceTool(t, "broken", "package main\nthis is not go\n")
	t.Setenv("PATH", goBinDir(t))
	_, err := Ensure(t.Context(), root, "broken")
	if err == nil || !strings.Contains(err.Error(), "build ./cmd/broken") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureBuildsFromSourceWhenConfigIsDefective(t *testing.T) {
	root := writeSourceTool(t, "hello", "package main\nfunc main() {}\n")
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("not-a-pyxis-key: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", goBinDir(t))
	got, err := Ensure(t.Context(), root, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(root, ".pyxis", "bin", "hello") {
		t.Fatalf("Ensure = %q", got)
	}
}

func writeSourceTool(t *testing.T, name, source string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "cmd", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/toolbin\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain required")
	}
	return root
}

func goBinDir(t *testing.T) string {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain required")
	}
	return filepath.Dir(goBin)
}
