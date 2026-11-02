package converge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func TestDoctorTaskHealthUsesExplicitBindingBeforeLegacyKnob(t *testing.T) {
	root := t.TempDir()
	repo := config.Defaults()
	repo.Knobs.Tracker = "carina"
	repo.WorkItems = "missing-legacy-store"
	for _, provider := range []string{integrations.Forgejo, integrations.GitHub} {
		profile := integrations.Config{Connections: map[string]integrations.Connection{"tasks": {Provider: provider, BaseURL: "https://github.com"}}, Bindings: integrations.Bindings{Tasks: integrations.Binding{Connection: "tasks", Repository: "o/tasks"}}}
		if provider == integrations.Forgejo {
			profile.Connections["tasks"] = integrations.Connection{Provider: provider, BaseURL: "http://127.0.0.1:3000"}
		}
		c := checks{}
		c.tracker(resolve.Config{Root: root, Repo: repo, Integrations: &profile})
		if len(c.rows) != 1 || c.rows[0].Name != provider || c.rows[0].Severity != SeverityAdvisory || !strings.Contains(c.rows[0].Detail, "o/tasks") {
			t.Fatalf("native task health: %+v", c.rows)
		}
	}
	carina := integrations.Config{Bindings: integrations.Bindings{Tasks: integrations.Binding{Provider: integrations.Carina, Root: t.TempDir()}}}
	c := checks{}
	c.tracker(resolve.Config{Root: root, Repo: repo, Integrations: &carina})
	if len(c.rows) != 0 {
		t.Fatalf("explicit absolute Carina store ignored: %+v", c.rows)
	}
	carina.Bindings.Tasks = integrations.Binding{Connection: "unknown", Repository: "o/tasks"}
	c = checks{}
	c.tracker(resolve.Config{Root: root, Repo: repo, Integrations: &carina})
	if len(c.rows) != 1 || c.rows[0].Severity != SeverityFinding {
		t.Fatalf("invalid task binding not reported: %+v", c.rows)
	}
}

func TestDoctorInvalidIntegrationProfileIsSinglePrerequisiteFinding(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("integrations:\n  unknown: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := resolve.Config{Root: root, Repo: config.Defaults()}
	c := checks{}
	c.tracker(cfg)
	if len(c.rows) != 0 {
		t.Fatal("tracker duplicated invalid configuration finding")
	}
	c.prerequisites(t.Context(), cfg)
	count := 0
	for _, row := range c.rows {
		if row.Name == "integrations" && row.Severity == SeverityFinding {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("invalid configuration diagnostic: %+v", c.rows)
	}
	c = checks{}
	c.ciModeWorkflowMismatch(cfg)
	if len(c.rows) != 0 {
		t.Fatal("invalid config invented workflow mismatch")
	}
}

func TestForgejoGeneratedGateParticipatesInDriftClassification(t *testing.T) {
	root := t.TempDir()
	repo := config.Defaults()
	m := relmanifest.Manifest{SystemVersion: "v1.2.3", Binaries: map[string]map[string]string{"pyxis": {"linux-arm64": strings.Repeat("a", 64), "linux-amd64": strings.Repeat("b", 64)}}, Payload: relmanifest.Payload{Files: []relmanifest.PayloadFile{{Path: payload.ForgejoWorkflowTarget, Class: relmanifest.Generated}}}}
	tmpl, err := os.ReadFile(filepath.Join("../..", payload.ForgejoWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	generated := map[string][]byte{}
	refusals := desiredGenerated(root, repo, m, workflowArchive(t, tmpl), generated)
	if len(refusals) != 0 || len(generated[payload.ForgejoWorkflowTarget]) == 0 {
		t.Fatalf("native generator: %v", refusals)
	}
	generated = map[string][]byte{}
	refusals = desiredGenerated(root, repo, m, workflowArchive(t, nil), generated)
	if refusals[payload.ForgejoWorkflowTarget] == "" || len(generated) != 0 {
		t.Fatalf("missing verified native template: %v", refusals)
	}
}

func TestDoctorUnreadableArtifactCacheNamesTheLocalFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("bin", filepath.Join(root, ".pyxis/bin")); err != nil {
		t.Fatal(err)
	}
	repo := config.Defaults()
	repo.System.Mode = "release"
	repo.System.Version = "v1.2.3"
	repo.System.ManifestSHA256 = strings.Repeat("a", 64)
	c := checks{}
	c.cache(resolve.Config{Root: root, Repo: repo})
	if len(c.rows) != 1 || c.rows[0].Class != CheckCache || c.rows[0].Severity != SeverityFinding {
		t.Fatalf("unreadable cache was not named: %+v", c.rows)
	}
	if _, err := os.Lstat(filepath.Join(root, ".pyxis/bin")); err != nil {
		t.Fatal("doctor mutated unreadable cache")
	}
}
