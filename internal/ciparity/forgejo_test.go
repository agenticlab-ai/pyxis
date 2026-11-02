package ciparity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/release"
)

const forgejoProfile = `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    forge:
      provider: forgejo
      base_url: http://127.0.0.1:3000
  bindings:
    code_host:
      connection: forge
      repository: owner/repository
`

func forgejoWorkflow() string {
	return strings.NewReplacer("github.", "forgejo.", "uses: actions/checkout@v4", "uses: https://data.forgejo.org/actions/checkout@v4").Replace(testWorkflow)
}

func forgejoFixture(t *testing.T, local bool) string {
	t.Helper()
	root := fixture(t)
	if local {
		writeFile(t, root, ".pyxis/config.local.yaml", forgejoProfile)
	} else {
		writeFile(t, root, ".pyxis/config.yaml", testConfig+forgejoProfile)
	}
	writeFile(t, root, ".forgejo/workflows/checks.yml", forgejoWorkflow())
	return root
}

func TestForgejoWorkflowSelection(t *testing.T) {
	for _, local := range []bool{false, true} {
		root := forgejoFixture(t, local)
		// The unselected provider's files cannot satisfy or contaminate the
		// selected host's audit, including when the binding is machine-local.
		writeFile(t, root, ".github/workflows/checks.yml", "invalid GitHub workflow")
		if err := Check(t.Context(), root); err != nil {
			t.Fatalf("selected Forgejo workflow (local=%t): %v", local, err)
		}
		if err := os.RemoveAll(filepath.Join(root, ".forgejo/workflows")); err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, ".github/workflows/checks.yml", testWorkflow)
		if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), ".forgejo/workflows") {
			t.Fatalf("missing Forgejo surface fell back to valid GitHub files: %v", err)
		}
	}
	root := fixture(t)
	writeFile(t, root, ".forgejo/workflows/checks.yml", "invalid unselected workflow")
	if err := Check(t.Context(), root); err != nil {
		t.Fatalf("legacy GitHub default changed: %v", err)
	}
	writeFile(t, root, ".pyxis/config.yaml", testConfig+strings.Replace(forgejoProfile, "provider: forgejo\n      base_url: http://127.0.0.1:3000", "provider: github", 1))
	if err := Check(t.Context(), root); err != nil {
		t.Fatalf("explicit GitHub changed: %v", err)
	}
	writeFile(t, root, ".pyxis/config.local.yaml", "integrations:\n  unknown: true\n")
	if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("invalid integration binding passed: %v", err)
	}
}

func TestForgejoWorkflowMutationsFailParity(t *testing.T) {
	for _, tc := range []struct{ name, from, to, want string }{
		{"foreign base context", "forgejo.event.pull_request.base.sha || forgejo.event.before", "github.event.pull_request.base.sha || github.event.before", "trusted PYXIS_CI_BASE"},
		{"head replaces trusted base", "forgejo.event.pull_request.base.sha", "forgejo.event.pull_request.head.sha", "trusted PYXIS_CI_BASE"},
		{"foreign event context", "forgejo.event_name", "github.event_name", "trusted PYXIS_CI_BASE"},
		{"step replaces trusted base", "      - name: all checks", "      - name: all checks\n        env: {PYXIS_CI_BASE: HEAD}", "trusted PYXIS_CI_BASE"},
		{"missing document gate", "      - name: documents\n        run: " + docRun + "\n", "", "no unconditional"},
		{"only a selected check", gateRun, gateRun + " --check memory", "no unconditional"},
		{"extra remote check", "      - name: all checks", "      - run: go test ./remote-only\n      - name: all checks", "unregistered blocking step"},
		{"disabled confinement", "PYXIS_REQUIRE_PROCESS_CONFINEMENT: \"1\"", "PYXIS_REQUIRE_PROCESS_CONFINEMENT: \"0\"", "requires PYXIS_REQUIRE_PROCESS_CONFINEMENT"},
		{"overridden hosted identity", "    env:", "    env:\n      FORGEJO_ACTIONS: 'false'", "requires FORGEJO_ACTIONS"},
		{"GitHub-compatible identity disabled", "    env:", "    env:\n      GITHUB_ACTIONS: 'false'", "requires GITHUB_ACTIONS"},
		{"implicit checkout origin", "https://data.forgejo.org/actions/checkout@v4", "actions/checkout@v4", "unregistered blocking step"},
		{"untrusted checkout origin", "https://data.forgejo.org/actions/checkout@v4", "https://other.invalid/actions/checkout@v4", "unregistered blocking step"},
		{"checkout wrong revision", "with: {fetch-depth: 0}", "with: {fetch-depth: 0, ref: main}", "unregistered blocking step"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := forgejoFixture(t, false)
			body := forgejoWorkflow()
			if !strings.Contains(body, tc.from) {
				t.Fatal("mutation did not match fixture")
			}
			writeFile(t, root, ".forgejo/workflows/checks.yml", strings.Replace(body, tc.from, tc.to, 1))
			if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("mutation error = %v, want %s", err, tc.want)
			}
		})
	}
	root := forgejoFixture(t, false)
	writeFile(t, root, ".forgejo/workflows/new-remote-check.yaml", "on: schedule\njobs: {extra: {steps: [{run: false}]}}\n")
	if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "new-remote-check.yaml") {
		t.Fatalf("new Forgejo workflow escaped inventory: %v", err)
	}
}

func TestForgejoSetupRemainsClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    step
		want bool
	}{
		{"historical fetch", step{Run: replayFetch, If: strings.ReplaceAll(replayCondition, "github.", "forgejo.")}, true},
		{"foreign replay condition", step{Run: replayFetch, If: replayCondition}, false},
		{"unconditional replay", step{Run: replayFetch}, false},
		{"go setup", step{Uses: "https://github.com/actions/setup-go@v5", With: map[string]string{"go-version-file": "go.mod", "cache": "false"}}, true},
		{"foreign Go action", step{Uses: "https://evil.invalid/actions/setup-go@v5", With: map[string]string{"go-version-file": "go.mod"}}, false},
		{"changed Go input", step{Uses: "https://github.com/actions/setup-go@v5", With: map[string]string{"go-version-file": "go.mod", "go-version": "old"}}, false},
		{"python", step{Uses: "https://github.com/actions/setup-python@v5", With: map[string]string{"python-version": "3.x"}}, true},
		{"node", step{Uses: "https://github.com/actions/setup-node@v4", With: map[string]string{"node-version": "20"}}, true},
		{"cache restore", step{Uses: "https://github.com/actions/cache/restore@v4", With: map[string]string{"path": "cache", "key": "key"}}, true},
		{"cache save", step{Uses: "https://github.com/actions/cache/save@v4", With: map[string]string{"path": "cache", "key": "key"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := setupStep(tc.s, "forgejo"); got != tc.want {
				t.Fatalf("setupStep = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestForgejoSourceGateTemplates(t *testing.T) {
	shared, err := os.ReadFile("../../.pyxis/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := codegatemirror.Parse(shared)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"docs-gate"} {
		data, err := os.ReadFile(filepath.Join("../../.pyxis/dist/forgejo-source-workflows", name+".yml"))
		if err != nil {
			t.Fatal(err)
		}
		body := strings.NewReplacer("{{branch}}", "main", "{{runner_label}}", "pyxis-linux").Replace(string(data))
		full, err := checkWorkflow([]byte(body), cfg, "forgejo")
		if err != nil || full != (name == "docs-gate") {
			t.Fatalf("real Forgejo %s template: full=%t, error=%v", name, full, err)
		}
	}
}

func TestMaterializedForgejoSourceWorkflowInventory(t *testing.T) {
	root := fixture(t)
	shared, err := os.ReadFile("../../.pyxis/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	profile := forgejoProfile + `    releases:
      source:
        connection: forge
        repository: owner/repository
  ci:
    mode: both
    runner_label: pyxis-linux
`
	writeFile(t, root, ".pyxis/config.yaml", string(shared)+"\n"+profile)
	const templates = ".pyxis/dist/forgejo-source-workflows"
	entries, err := os.ReadDir(filepath.Join("../..", templates))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("../..", templates, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, filepath.Join(templates, entry.Name()), string(data))
	}
	paths, err := release.MaterializeSourceWorkflows(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 8 {
		t.Fatalf("materialized %d source workflows, want all eight", len(paths))
	}
	if err := Check(t.Context(), root); err != nil {
		t.Fatalf("production-rendered Forgejo source inventory violates shared gate parity: %v", err)
	}
	// The inventory's complete gate is essential; all supplemental source
	// workflows together cannot silently replace the required aggregate.
	if err := os.Remove(filepath.Join(root, ".forgejo/workflows/docs-gate.yml")); err != nil {
		t.Fatal(err)
	}
	if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "no unconditional") {
		t.Fatalf("source inventory passed without the complete gate: %v", err)
	}
}
