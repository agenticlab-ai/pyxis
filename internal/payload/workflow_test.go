package payload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForgejoWorkflowRenderingSeparatesTrustedExpressions(t *testing.T) {
	tmpl := []byte("branch: '{{branch}}'\ntoken: ${{ secrets.TOKEN }}\n")
	out, err := RenderForgejoWorkflow(tmpl, map[string]string{"branch": "main"})
	if err != nil || string(out) != "branch: 'main'\ntoken: ${{ secrets.TOKEN }}\n" {
		t.Fatalf("render %q %v", out, err)
	}
	for _, value := range []string{"'\nrun: unsafe", "{{other}}", "${{ secrets.OTHER }}", "PYXIS_ACTION_EXPRESSION_0_END"} {
		if _, err := RenderForgejoWorkflow(tmpl, map[string]string{"branch": value}); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	if _, err := RenderForgejoWorkflow(tmpl, nil); err == nil {
		t.Fatal("missing value accepted")
	}
}

func TestWorkflowHostSelectionIsExplicit(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	source, target, err := WorkflowForRoot(root)
	if err != nil || source != InstalledWorkflow || target != InstalledWorkflowTarget {
		t.Fatalf("legacy: %s %s %v", source, target, err)
	}
	base := `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    chosen:
      provider: PROVIDER
      base_url: BASE
  bindings:
    code_host:
      connection: chosen
      repository: o/r
`
	for _, provider := range []string{"forgejo", "github"} {
		url := "https://github.com"
		want := InstalledWorkflowTarget
		if provider == "forgejo" {
			url = "http://127.0.0.1:3000"
			want = ForgejoWorkflowTarget
		}
		cfg := strings.ReplaceAll(strings.ReplaceAll(base, "PROVIDER", provider), "BASE", url)
		if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(cfg), 0600); err != nil {
			t.Fatal(err)
		}
		_, target, err := WorkflowForRoot(root)
		if err != nil || target != want {
			t.Fatalf("%s: %s %v", provider, target, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("integrations:\n  unknown: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WorkflowForRoot(root); err == nil {
		t.Fatal("accepted unknown integration key")
	}
}
