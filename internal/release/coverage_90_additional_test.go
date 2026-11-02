package release

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type failAfterWriter struct {
	remaining int
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.remaining == 0 {
		return 0, errors.New("write refused")
	}
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, errors.New("write refused")
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestCoverage90TarAndArtifactFailures(t *testing.T) {
	tw := tar.NewWriter(&failAfterWriter{remaining: 512})
	if err := writeMember(tw, tarEntry{name: "literal", data: []byte("body")}); err == nil || !strings.Contains(err.Error(), "write literal") {
		t.Fatalf("literal write error = %v", err)
	}

	dir := t.TempDir()
	if err := writeDeterministicTarGz(filepath.Join(t.TempDir(), "archive.tgz"), []tarEntry{{name: "directory", src: dir}}); err == nil || !strings.Contains(err.Error(), "stream directory") {
		t.Fatalf("directory member error = %v", err)
	}
	if _, err := hashFile(dir); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("directory hash error = %v", err)
	}

	flat := t.TempDir()
	if err := os.Mkdir(filepath.Join(flat, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := listArtifacts(flat); err == nil || !strings.Contains(err.Error(), "subdirectory") {
		t.Fatalf("nested artifact error = %v", err)
	}
	if err := os.Remove(filepath.Join(flat, "nested")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(flat, "link")); err == nil {
		if _, err := listArtifacts(flat); err == nil || !strings.Contains(err.Error(), "non-regular") {
			t.Fatalf("symlink artifact error = %v", err)
		}
	}
}

func TestCoverage90ReleaseOptionAndRemoteFailures(t *testing.T) {
	if _, err := Pack(t.Context(), PackOptions{Version: "bad"}); err == nil {
		t.Fatal("invalid pack version succeeded")
	}
	if err := Verify(t.TempDir(), "bad"); err == nil {
		t.Fatal("invalid verify version succeeded")
	}
	t.Run("pack defaults root", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if _, err := Pack(t.Context(), PackOptions{Version: "v1.2.3", OutDir: t.TempDir(), BinariesDir: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "class policy") {
			t.Fatalf("default-root pack error = %v", err)
		}
	})
	malformed := t.TempDir()
	if err := os.MkdirAll(filepath.Join(malformed, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(malformed, ".pyxis", "class-policy.yaml"), []byte("not: [closed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(t.Context(), PackOptions{Version: "v1.2.3", RepoRoot: malformed, OutDir: t.TempDir(), BinariesDir: t.TempDir()}); err == nil {
		t.Fatal("malformed class policy succeeded")
	}
	if err := checkOutDirForVersion(filepath.Join(t.TempDir(), "missing"), "v1.2.3"); err == nil {
		t.Fatal("missing output directory passed version check")
	}
	unreadableBinaries := t.TempDir()
	unreadableBinary := filepath.Join(unreadableBinaries, "tool-v1.2.3-darwin-amd64")
	if err := os.WriteFile(unreadableBinary, []byte("binary"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, _, err := collectBinaries(unreadableBinaries, "v1.2.3", []string{"tool"}); err == nil {
		t.Fatal("unreadable binary was hashed")
	}
	unreadableArtifacts := t.TempDir()
	if err := os.WriteFile(filepath.Join(unreadableArtifacts, "artifact-v1.2.3"), []byte("body"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSums(unreadableArtifacts); err == nil {
		t.Fatal("unreadable artifact was summed")
	}

	if _, err := Pins(t.Context(), PinsOptions{Version: "bad"}); err == nil {
		t.Fatal("invalid pins version succeeded")
	}
	t.Run("pins defaults root and rejects nil runner", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, installSHName), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		if _, err := Pins(t.Context(), PinsOptions{Version: "v1.2.3"}); err == nil || !strings.Contains(err.Error(), "runner") {
			t.Fatalf("nil pins runner error = %v", err)
		}
	})
	if err := writeFileAtomic(filepath.Join(t.TempDir(), "missing", "install.sh"), []byte("body")); err == nil {
		t.Fatal("atomic write into a missing directory succeeded")
	}
	noDownload := &coverageGHRunner{results: []coverageGHResult{{}, {}, {}, {}, {}}}
	if _, err := pinsFromRelease(t.Context(), noDownload, "owner/repo", "v1.2.3"); err == nil || !strings.Contains(err.Error(), "hash the downloaded") {
		t.Fatalf("missing downloaded pin error = %v", err)
	}

	if _, err := Publish(t.Context(), PublishOptions{}); err == nil || !strings.Contains(err.Error(), "runner") {
		t.Fatalf("nil publish runner error = %v", err)
	}
	transport := errors.New("transport")
	if _, _, err := remoteAssets(t.Context(), PublishOptions{Tag: "v1.2.3", Runner: &coverageGHRunner{results: []coverageGHResult{{err: transport}}}}); !errors.Is(err, transport) {
		t.Fatalf("remote asset transport error = %v", err)
	}
	if _, _, err := remoteAssets(t.Context(), PublishOptions{Tag: "v1.2.3", Runner: &coverageGHRunner{results: []coverageGHResult{{stdout: "{"}}}}); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("remote asset parse error = %v", err)
	}
}

func TestCoverage90MirrorAcquisitionFailures(t *testing.T) {
	target := &integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "owner/dist"}
	t.Run("default root without pins", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, installSHName), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		_, err := Mirror(t.Context(), MirrorOptions{Tag: "v1.2.3", SourceRepo: "owner/source", Runner: &coverageGHRunner{}, DistTarget: target})
		if err == nil || !strings.Contains(err.Error(), "no PINNED_VERSION") {
			t.Fatalf("missing pins error = %v", err)
		}
	})
	root := t.TempDir()
	installer := "REPO=\"owner/source\"\nRELEASE_PROVIDER=\"${PYXIS_RELEASE_PROVIDER:-github}\"\nRELEASE_BASE_URL=\"${PYXIS_RELEASE_BASE_URL:-https://github.com}\"\nRELEASE_TOKEN_ENV=\"${PYXIS_RELEASE_TOKEN_ENV:-}\"\nPINNED_VERSION=\"v1.2.3\"\n"
	if err := os.WriteFile(filepath.Join(root, installSHName), []byte(installer), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Mirror(t.Context(), MirrorOptions{Tag: "v1.2.3", RepoRoot: root, SourceRepo: "owner/source", DistTarget: target, Runner: &coverageGHRunner{results: []coverageGHResult{{code: 1, stderr: "download refused"}}}}); err == nil || !strings.Contains(err.Error(), "download refused") {
		t.Fatalf("download error = %v", err)
	}
}

// The staged copy seals into the archive before the manifest exists, so the real
// template stages with empty pins: valid, token-free, and refused by the shim if
// run as is. Converge renders the working copy for each consumer.
func TestStagedWorkflowFromShippedTemplateCarriesNoPins(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(overlaySrcDir), "docs-gate.yml"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "docs-gate.yml"), tmpl, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := renderStagedWorkflow(root, "docs-gate.yml", "out.yml"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "out.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "{{") {
		t.Errorf("staged workflow retains a token:\n%s", got)
	}
	for _, want := range []string{"branches: [main]", "git fetch origin main", "PYXIS_VERSION: ''", "PYXIS_SHA256_LINUX_AMD64: ''"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("staged workflow missing %q:\n%s", want, got)
		}
	}
}

func TestCoverage90StageAndPreconditionFailures(t *testing.T) {
	root := t.TempDir()
	if err := renderStagedWorkflow(root, "missing.yml", "out.yml"); err == nil {
		t.Fatal("missing workflow template rendered")
	}
	if err := os.WriteFile(filepath.Join(root, "bad.yml"), []byte("{{unknown}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := renderStagedWorkflow(root, "bad.yml", "out.yml"); err == nil || !strings.Contains(err.Error(), "render") {
		t.Fatalf("unknown template token error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "good.yml"), []byte("branch: {{branch}}\nremote: {{remote}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := renderStagedWorkflow(root, "good.yml", "blocked/out.yml"); err == nil {
		t.Fatal("workflow rendered through a file parent")
	}

	if err := CheckWorkflowToolchainFree(t.TempDir()); err != nil {
		t.Fatalf("missing workflow directory: %v", err)
	}
	workflowRoot := t.TempDir()
	workflowDir := filepath.Join(workflowRoot, filepath.FromSlash(overlayDstDir))
	if err := os.MkdirAll(filepath.Join(workflowDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CheckWorkflowToolchainFree(workflowRoot); err != nil {
		t.Fatalf("non-regular workflow entry: %v", err)
	}

	if _, err := readSums(writeTempFile(t, "empty-sums", nil)); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty sums error = %v", err)
	}
}

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

var _ io.Writer = (*failAfterWriter)(nil)
