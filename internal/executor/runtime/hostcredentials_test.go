package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestVerificationCannotOptIntoConfiguredHostCredentials(t *testing.T) {
	parent := WithHostCredentialNames(t.Context(), "CUSTOM_FORGEJO", "")
	child := WithHostCredentialNames(parent, "CUSTOM_GITHUB")
	if isHostCredential(parent, "CUSTOM_GITHUB") || !isHostCredential(child, "CUSTOM_FORGEJO") || !isHostCredential(child, "CUSTOM_GITHUB") || isHostCredential(t.Context(), "CUSTOM_FORGEJO") {
		t.Fatal("credential scope was mutated or lost")
	}
	ctx, cleanup, err := VerificationContext(child)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	dir := t.TempDir()
	// Stop before platform confinement or process launch; filtering must happen
	// first, even when the child has malicious verification env pass-throughs.
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("must-not-start")
	cmd.Dir, cmd.Env = dir, []string{"CUSTOM_FORGEJO=forgejo-secret", "CUSTOM_GITHUB=github-secret", "GH_ENTERPRISE_TOKEN=enterprise-secret", "GITHUB_ENTERPRISE_TOKEN=enterprise-secret", "DATABASE_TEST_URL=fixture-db", "PATH=" + os.Getenv("PATH")}
	if err := ConfineVerificationCommand(ctx, cmd); err == nil || cmd.Process != nil {
		t.Fatalf("malformed repository launched: %v", err)
	}
	env := strings.Join(cmd.Env, "\n")
	if strings.Contains(env, "secret") || strings.Contains(env, "CUSTOM_") || !strings.Contains(env, "DATABASE_TEST_URL=fixture-db") {
		t.Fatalf("verification credential isolation failed: %s", env)
	}
	for _, name := range []string{"CUSTOM_FORGEJO", "CUSTOM_GITHUB", "PYXIS_FORGEJO_TOKEN"} {
		t.Setenv(name, "host-secret")
	}
	for _, purpose := range []port.LaunchPurpose{port.LaunchPurposeImplementation, port.LaunchPurposeReview} {
		env := strings.Join(LaunchEnvironment(port.LaunchSpec{Purpose: purpose, Workdir: dir, RuntimeStateDir: dir + "-runtime"}), "\n")
		if strings.Contains(env, "host-secret") {
			t.Fatalf("harness inherited host credentials: %s", env)
		}
	}
}
