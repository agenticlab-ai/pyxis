package ciparity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const localGitProfile = `integrations:
  schema_version: pyxis-integrations/v1
  bindings:
    code_host:
      provider: local-git
      root: ../origin.git
  ci:
    mode: local
`

func TestLocalGitCIParityWithoutHostedWorkflows(t *testing.T) {
	for _, local := range []bool{false, true} {
		root := fixture(t)
		if local {
			writeFile(t, root, ".pyxis/config.local.yaml", localGitProfile)
		} else {
			writeFile(t, root, ".pyxis/config.yaml", testConfig+localGitProfile)
		}
		if err := os.RemoveAll(filepath.Join(root, ".github/workflows")); err != nil {
			t.Fatal(err)
		}
		if err := Check(t.Context(), root); err != nil {
			t.Fatalf("local CI without hosted workflows (overlay=%t): %v", local, err)
		}
		// Retained hosted files cannot select another provider or satisfy a
		// separate inventory when the selected code host uses only local CI.
		writeFile(t, root, ".github/workflows/checks.yml", "invalid retained workflow")
		writeFile(t, root, ".forgejo/workflows/checks.yml", "invalid retained workflow")
		if err := Check(t.Context(), root); err != nil {
			t.Fatalf("local CI inspected unselected hosted files: %v", err)
		}
		writeFile(t, root, ".pyxis/hooks/pre-push", testHook+"false\n")
		if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "delegate exactly") {
			t.Fatalf("local CI accepted a divergent pre-push hook: %v", err)
		}
	}
	root := fixture(t)
	writeFile(t, root, ".pyxis/config.local.yaml", localGitProfile+"code-gate:\n  memory:\n    command: false\n")
	if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "overrides diverge") {
		t.Fatalf("local CI accepted a changed shared predicate: %v", err)
	}
}

func TestLocalGitCIParityRefusesHostedMode(t *testing.T) {
	for _, mode := range []string{"", "actions", "both"} {
		t.Run(mode, func(t *testing.T) {
			root := fixture(t)
			profile := strings.Replace(localGitProfile, "  ci:\n    mode: local\n", "", 1)
			if mode != "" {
				profile += "  ci:\n    mode: " + mode + "\n"
			}
			writeFile(t, root, ".pyxis/config.local.yaml", profile)
			if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "integrations.ci.mode: local") {
				t.Fatalf("local code host with CI mode %q: %v", mode, err)
			}
		})
	}
}
