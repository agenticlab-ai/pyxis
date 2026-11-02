package main

import (
	"context"
	"crypto/ed25519"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/credentialstore"
)

type activationMemoryCredentials map[string][]byte

func (s activationMemoryCredentials) Put(_ context.Context, account string, secret []byte) error {
	s[account] = append([]byte(nil), secret...)
	return nil
}

func (s activationMemoryCredentials) Get(_ context.Context, account string) ([]byte, error) {
	return append([]byte(nil), s[account]...), nil
}

func TestActiveAttemptOwnerUsesRolelessTerminalScope(t *testing.T) {
	secrets := activationMemoryCredentials{}
	account, err := credentialstore.Account("owner/repo", "producer:attempt-owner")
	if err != nil {
		t.Fatal(err)
	}
	key, err := credentialstore.Generate(t.Context(), secrets, account, "producer:attempt-owner")
	if err != nil {
		t.Fatal(err)
	}
	policy := routingevidence.ProducerPolicy{Repository: "owner/repo", EvidenceRef: "refs/pyxis/routing-evidence", ActivationPoint: "policy/1",
		Keys: []routingevidence.KeyGeneration{{Algorithm: "ed25519", ProducerClass: routingevidence.ProducerAttemptOwner,
			ImplementationGeneration: "attempt-owner/v1", KeyID: key.KeyID, PublicKey: key.PublicKey, PolicyActivationPoints: []string{"policy/1"}}}}
	capability, err := loadActiveAttemptOwner(t.Context(), secrets, policy)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(capability.PrivateKey)
	if len(capability.PrivateKey) != ed25519.PrivateKeySize || len(capability.Roles) != 1 || capability.Roles[0] != "" {
		t.Fatalf("attempt-owner capability = roles:%q key-bytes:%d", capability.Roles, len(capability.PrivateKey))
	}
}

func TestReadActivationMainlineRequiresCleanExactRemoteHead(t *testing.T) {
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "repo")
	runRoutingGit(t, root, "init", "--bare", "-q", remote)
	runRoutingGit(t, root, "init", "-q", repo)
	runRoutingGit(t, repo, "config", "user.name", "Pyxis Test")
	runRoutingGit(t, repo, "config", "user.email", "pyxis@example.invalid")
	runRoutingGit(t, repo, "remote", "add", "upstream", remote)
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis", "routing"), 0o700); err != nil {
		t.Fatal(err)
	}
	configWire := []byte("executor:\n  review:\n    budget_minutes: 15\n")
	policyWire := []byte("canonical-policy")
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), configWire, 0o600); err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(repo, ".pyxis", "routing", "producer-policy.json")
	if err := os.WriteFile(policy, policyWire, 0o600); err != nil {
		t.Fatal(err)
	}
	runRoutingGit(t, repo, "add", ".pyxis/config.yaml", ".pyxis/routing/producer-policy.json")
	runRoutingGit(t, repo, "commit", "-q", "-m", "activation inputs")
	runRoutingGit(t, repo, "branch", "-M", "trunk")
	runRoutingGit(t, repo, "push", "-q", "upstream", "trunk")

	gotConfig, gotPolicy, err := readActivationMainline(context.Background(), repo, "upstream", "trunk", ".pyxis/routing/producer-policy.json")
	if err != nil || string(gotConfig) != string(configWire) || string(gotPolicy) != string(policyWire) {
		t.Fatalf("read mainline = config %q policy %q err %v", gotConfig, gotPolicy, err)
	}
	if err := os.WriteFile(policy, []byte("worktree-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readActivationMainline(context.Background(), repo, "upstream", "trunk", ".pyxis/routing/producer-policy.json"); err == nil || !strings.Contains(err.Error(), "differs from committed") {
		t.Fatalf("dirty activation input = %v", err)
	}
	runRoutingGit(t, repo, "checkout", "-q", "--", ".pyxis/routing/producer-policy.json")
	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("feature"), 0o600); err != nil {
		t.Fatal(err)
	}
	runRoutingGit(t, repo, "add", "feature.txt")
	runRoutingGit(t, repo, "commit", "-q", "-m", "unmerged feature")
	if _, _, err := readActivationMainline(context.Background(), repo, "upstream", "trunk", ".pyxis/routing/producer-policy.json"); err == nil || !strings.Contains(err.Error(), "not the exact current remote mainline") {
		t.Fatalf("unmerged activation head = %v", err)
	}
}

func TestActivationGitEnvironmentStripsConfigInjection(t *testing.T) {
	got := activationGitEnvironment([]string{"PATH=/usr/bin", "GIT_DIR=/other", "GIT_CONFIG_COUNT=4", "GIT_CONFIG_KEY_3=remote.upstream.url", "GIT_CONFIG_VALUE_3=bad", "HOME=/tmp/home"})
	if strings.Join(got, "\n") != "PATH=/usr/bin\nHOME=/tmp/home" {
		t.Fatalf("activation git environment = %v", got)
	}
}

func runRoutingGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}
