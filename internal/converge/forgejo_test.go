package converge

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

func workflowArchive(t *testing.T, template []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "payload.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if template != nil {
		if err := tw.WriteHeader(&tar.Header{Name: payload.ForgejoWorkflow, Mode: 0644, Size: int64(len(template))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(template); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestForgejoWorkflowArchiveAndConfigurationFailures(t *testing.T) {
	root := t.TempDir()
	repo := config.Defaults()
	m := relmanifest.Manifest{SystemVersion: "v1.2.3", Binaries: map[string]map[string]string{"pyxis": {"linux-arm64": strings.Repeat("a", 64), "linux-amd64": strings.Repeat("b", 64)}}}
	tmpl, err := os.ReadFile(filepath.Join("../..", payload.ForgejoWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	if _, reason := desiredForgejoWorkflow(root, repo, m, ""); reason == "" {
		t.Fatal("accepted missing template")
	}
	if _, reason := desiredForgejoWorkflow(root, repo, m, filepath.Join(root, "missing")); reason == "" {
		t.Fatal("accepted absent archive")
	}
	if _, reason := desiredForgejoWorkflow(root, repo, m, workflowArchive(t, nil)); reason == "" {
		t.Fatal("accepted archive without template")
	}
	archive := workflowArchive(t, tmpl)
	if _, reason := desiredForgejoWorkflow(root, repo, m, archive); reason != "" {
		t.Fatal(reason)
	}
	if _, reason := desiredForgejoWorkflow(root, repo, m, workflowArchive(t, []byte("{{missing}}"))); reason == "" {
		t.Fatal("accepted unresolved template")
	}
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	profile := `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    artifacts:
      provider: forgejo
      base_url: http://127.0.0.1:3000
      token_env: TOKEN
  bindings:
    artifacts:
      connection: artifacts
      repository: o/artifacts
`
	for _, token := range []string{"PYXIS_ARTIFACT_TOKEN", "PYXIS_VERSION"} {
		if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(strings.ReplaceAll(profile, "TOKEN", token)), 0600); err != nil {
			t.Fatal(err)
		}
		out, reason := desiredForgejoWorkflow(root, repo, m, archive)
		if token == "PYXIS_VERSION" {
			if reason == "" {
				t.Fatal("accepted reserved token env")
			}
		} else {
			if reason != "" {
				t.Fatal(reason)
			}
			if strings.Count(string(out), "PYXIS_ARTIFACT_TOKEN:") != 1 {
				t.Fatal("duplicate token key")
			}
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("integrations:\n  invalid: value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, reason := desiredForgejoWorkflow(root, repo, m, archive); reason == "" {
		t.Fatal("invalid profile ignored")
	}
	if _, err := ProjectWorkflowManifest(root, m); err == nil {
		t.Fatal("invalid profile projected")
	}
}

func TestForgejoWorkflowUsesManifestPinsAndHostBinding(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := `schema_version: 2
integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000
  bindings:
    code_host:
      connection: local
      repository: o/code
    artifacts:
      connection: local
      repository: o/binaries
  ci:
    mode: both
    runner_label: pyxis-linux
`
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	tmpl, err := os.ReadFile(filepath.Join("../..", payload.ForgejoWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, payload.ForgejoWorkflow)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, tmpl, 0644); err != nil {
		t.Fatal(err)
	}
	m := relmanifest.Manifest{SystemVersion: "v1.2.3", Binaries: map[string]map[string]string{"pyxis": {"linux-arm64": strings.Repeat("a", 64), "linux-amd64": strings.Repeat("b", 64)}}}
	m.Payload.Files = []relmanifest.PayloadFile{{Path: payload.InstalledWorkflowTarget, Class: relmanifest.Generated}}
	projected, err := ProjectWorkflowManifest(root, m)
	if err != nil {
		t.Fatal(err)
	}
	if projected.Payload.Files[0].Path != payload.ForgejoWorkflowTarget || m.Payload.Files[0].Path != payload.InstalledWorkflowTarget {
		t.Fatal("projection modified trust-anchor manifest or missed Forgejo target")
	}
	repo := config.Defaults()
	out, reason := desiredForgejoWorkflow(root, repo, m, "")
	if reason != "" {
		t.Fatal(reason)
	}
	for _, want := range []string{"v1.2.3", strings.Repeat("a", 64), strings.Repeat("b", 64), "http://127.0.0.1:3000/o/binaries/releases/download/v1.2.3", "runs-on: pyxis-linux", "${{ secrets.PYXIS_ARTIFACT_TOKEN }}"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(string(out), "go run") || strings.Contains(string(out), "gh ") {
		t.Fatal("installed Forgejo workflow needs source toolchain or GitHub CLI")
	}
	if !hostedWorkflowIncluded(workflowMode(root, repo)) {
		t.Fatal("integration CI mode ignored")
	}
	repo.Branch = "main; touch /tmp/escaped"
	if _, reason := desiredForgejoWorkflow(root, repo, m, ""); reason == "" {
		t.Fatal("accepted shell injection")
	}
	delete(m.Binaries["pyxis"], "linux-arm64")
	if _, reason := desiredForgejoWorkflow(root, config.Defaults(), m, ""); reason == "" {
		t.Fatal("accepted unpinned runner platform")
	}
}
