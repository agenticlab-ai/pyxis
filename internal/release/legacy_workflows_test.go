package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestBuildLegacyProducesCompleteVerifiedMatrix(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd/docgate"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.25\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd/docgate/main.go"), []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "dist")
	if err := BuildLegacy(t.Context(), "docgate", "docgate/v1.2.3", root, out); err != nil {
		t.Fatal(err)
	}
	if err := verifyPublish(PublishOptions{LegacyTool: "docgate", Tag: "docgate/v1.2.3", OutDir: out}); err != nil {
		t.Fatal(err)
	}
	if err := BuildLegacy(t.Context(), "docgate", "docgate/v1.2.3", root, out); err == nil {
		t.Fatal("rebuilt into nonempty output")
	}
	for _, input := range []struct{ tool, tag, out string }{{"bad", "bad/v1.2.3", out}, {"docgate", "docgate/v1.2.3", ""}, {"docgate", "docgate/v1.2.3", filepath.Join(root, "go.mod", "dist")}} {
		if err := BuildLegacy(t.Context(), input.tool, input.tag, root, input.out); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	if err := BuildLegacy(t.Context(), "docgate", "docgate/v1.2.3", t.TempDir(), filepath.Join(root, "missing-source")); err == nil {
		t.Fatal("published a legacy matrix without compiler source")
	}
	t.Setenv("PATH", "")
	if err := BuildLegacy(t.Context(), "docgate", "docgate/v1.2.3", root, filepath.Join(root, "no-go")); err == nil {
		t.Fatal("built without Go")
	}
}

func TestLegacyVerificationAndInstallerRetarget(t *testing.T) {
	dir := t.TempDir()
	for _, platform := range Platforms {
		if err := os.WriteFile(filepath.Join(dir, "docgate-v1.2.3-"+platform), []byte(platform), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writeSums(dir); err != nil {
		t.Fatal(err)
	}
	if err := verifyLegacy(dir, "docgate", "docgate/v1.2.3"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][2]string{{"other", "other/v1.2.3"}, {"codegate", "docgate/v1.2.3"}, {"docgate", "docgate/bad"}} {
		if _, err := legacyVersion(args[0], args[1]); err == nil {
			t.Fatalf("accepted %+v", args)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "docgate-v1.2.3-"+Platforms[0]), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyLegacy(dir, "docgate", "docgate/v1.2.3"); err == nil {
		t.Fatal("accepted checksum mismatch")
	}
	src, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	target := integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: "http://127.0.0.1:3000", Repository: "o/dist", TokenEnv: "DIST_TOKEN"}
	next, err := installerForTarget(string(src), MirrorOptions{SourceRepo: "agenticlab-ai/pyxis", DistRepo: "o/dist", DistTarget: &target})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`REPO="o/dist"`, `RELEASE_PROVIDER="${PYXIS_RELEASE_PROVIDER:-forgejo}"`, `RELEASE_TOKEN_ENV="${PYXIS_RELEASE_TOKEN_ENV:-DIST_TOKEN}"`} {
		if !strings.Contains(next, want) {
			t.Errorf("missing %q", want)
		}
	}
	if _, err := installerForTarget("old installer", MirrorOptions{DistTarget: &target}); err == nil {
		t.Fatal("accepted installer without provider support")
	}
}

func TestSourceWorkflowMaterializationRequiresExplicitSelection(t *testing.T) {
	root := t.TempDir()
	if _, err := MaterializeSourceWorkflows(root); err == nil {
		t.Fatal("materialized without configured source")
	}
	templateDir := filepath.Join(root, ".pyxis/dist/forgejo-source-workflows")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatal(err)
	}
	srcDir := "../../.pyxis/dist/forgejo-source-workflows"
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(templateDir, e.Name()), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := `schema_version: pyxis-config/v2
integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000
  bindings:
    code_host:
      connection: local
      repository: o/source
    releases:
      source:
        connection: local
        repository: o/source
  ci:
    mode: both
    runner_label: source-linux
`
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := MaterializeSourceWorkflows(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 8 {
		t.Fatalf("paths %v", paths)
	}
	for _, path := range paths {
		if filepath.Base(path) == "memory-reconciliation.yml" {
			t.Fatalf("retired standalone memory workflow was materialized: %s", path)
		}
	}
	if _, err := MaterializeSourceWorkflows(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[0], []byte("operator workflow"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeSourceWorkflows(root); err == nil {
		t.Fatal("overwrote operator workflow")
	}
	bytes, err := os.ReadFile(paths[0])
	if err != nil || string(bytes) != "operator workflow" {
		t.Fatal("collision not preserved")
	}
}
