package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestConfiguredReleaseFailureKeepsSelectedProvider(t *testing.T) {
	root := t.TempDir()
	installer, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "install.sh"), installer, 0755); err != nil {
		t.Fatal(err)
	}
	github := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/pinned", TokenEnv: "SCOPED_TOKEN"}
	runner := &coverageGHRunner{results: []coverageGHResult{{stderr: "permission denied", code: 1}}}
	if _, err := Pins(t.Context(), PinsOptions{Version: "v1.2.3", RepoRoot: root, Target: &github, Runner: runner}); err == nil {
		t.Fatal("denied configured GitHub pins succeeded")
	}
	if runner.next != 1 {
		t.Fatal("configured GitHub pins did not use scoped operation")
	}
	forgejo := github
	forgejo.Provider = integrations.Forgejo
	forgejo.BaseURL = "invalid"
	if _, err := Pins(t.Context(), PinsOptions{Version: "v1.2.3", RepoRoot: root, Target: &forgejo}); err == nil {
		t.Fatal("invalid Forgejo source fell through to Github")
	}
	if _, err := Publish(t.Context(), PublishOptions{Tag: "bad", Target: &github, Runner: runner, OutDir: root}); err == nil {
		t.Fatal("invalid configured GitHub release succeeded")
	}
	current, err := os.ReadFile(filepath.Join(root, "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(installer) {
		t.Fatal("failed pins changed the trust anchor")
	}
}

func TestRemotePinsUnavailableStagingIsNamed(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if _, err := pinsFromRelease(t.Context(), &coverageGHRunner{}, "o/r", "v1.2.3"); err == nil || !strings.Contains(err.Error(), "pins") {
		t.Fatalf("unavailable private staging: %v", err)
	}
	if err := verifyRemoteAsset(t.Context(), PublishOptions{Runner: &coverageGHRunner{}}, "asset"); err == nil || !strings.Contains(err.Error(), "publish") {
		t.Fatalf("unavailable verification staging: %v", err)
	}
}
