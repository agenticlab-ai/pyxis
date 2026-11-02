package installer

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGitLinesSuccessAndFailure(t *testing.T) {
	if _, err := gitLines(context.Background(), t.TempDir(), "status", "--porcelain"); err == nil {
		t.Fatal("gitLines accepted a non-repository")
	}

	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(repo+"/untracked", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := gitLines(context.Background(), repo, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 {
		t.Fatalf("gitLines = %v, want one non-empty line", lines)
	}
}

func TestInstallerCoreBoundaryHelpers(t *testing.T) {
	if err := noopStage(t.Context(), &initRun{}); err != nil {
		t.Fatal(err)
	}
	run := &initRun{opts: Options{Target: t.TempDir()}}
	if got := run.payloadTemplatesDir(); !strings.HasSuffix(got, ".pyxis/templates") {
		t.Fatalf("default templates dir = %q", got)
	}
	if got := tag(" 1.2.3 "); got != "v1.2.3" {
		t.Fatalf("tag = %q", got)
	}
	if isHex64(strings.Repeat("g", 64)) {
		t.Fatal("non-hex digest accepted")
	}
	if got := resolveWiringSlug(t.Context(), t.TempDir(), "origin", "not/a/coordinate/extra"); got != "" {
		t.Fatalf("invalid coordinate resolved as %q", got)
	}
}

func TestInitHonorsCancellationBeforeAdmissionWithDefaultSinks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Init(ctx, Options{
		Target: t.TempDir(),
		Pin:    Pin{Version: "1.2.3", ManifestSHA256: strings.Repeat("a", 64)},
	})
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("cancelled init error = %v", err)
	}
}
