package gitstore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestArtifactReaderReadsNamedObjectNotWorktree(t *testing.T) {
	repo := t.TempDir()
	runArtifactGit(t, repo, "init", "-q")
	runArtifactGit(t, repo, "config", "user.name", "Pyxis Test")
	runArtifactGit(t, repo, "config", "user.email", "pyxis@example.invalid")
	path := filepath.Join(repo, "proof.json")
	if err := os.WriteFile(path, []byte("committed-proof"), 0o600); err != nil {
		t.Fatal(err)
	}
	runArtifactGit(t, repo, "add", "proof.json")
	runArtifactGit(t, repo, "commit", "-q", "-m", "proof")
	object := strings.TrimSpace(runArtifactGit(t, repo, "rev-parse", "HEAD"))
	if err := os.WriteFile(path, []byte("uncommitted-replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, err := NewArtifactReader(repo)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reader.ReadActivationArtifact(context.Background(), object, "proof.json")
	if err != nil || string(got) != "committed-proof" {
		t.Fatalf("ReadActivationArtifact = %q, %v", got, err)
	}
	if _, err := reader.ReadActivationArtifact(context.Background(), object, "../proof.json"); err == nil {
		t.Fatal("path escape was accepted")
	}
}

func TestArtifactReaderRejectsEmptyRepositoryAndMalformedObject(t *testing.T) {
	if _, err := NewArtifactReader(""); err == nil {
		t.Fatal("empty repository was accepted")
	}
	if lowerHexObject("short") {
		t.Fatal("short object id was accepted")
	}
}

func TestReadRemoteRefReturnsExactCurrentBranchObject(t *testing.T) {
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "repo")
	runArtifactGit(t, root, "init", "--bare", "-q", remote)
	runArtifactGit(t, root, "init", "-q", repo)
	runArtifactGit(t, repo, "config", "user.name", "Pyxis Test")
	runArtifactGit(t, repo, "config", "user.email", "pyxis@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "proof.json"), []byte("proof"), 0o600); err != nil {
		t.Fatal(err)
	}
	runArtifactGit(t, repo, "add", "proof.json")
	runArtifactGit(t, repo, "commit", "-q", "-m", "proof")
	runArtifactGit(t, repo, "branch", "-M", "trunk")
	runArtifactGit(t, repo, "remote", "add", "upstream", remote)
	runArtifactGit(t, repo, "push", "-q", "upstream", "trunk")
	want := strings.TrimSpace(runArtifactGit(t, repo, "rev-parse", "HEAD"))
	got, err := ReadRemoteRef(t.Context(), repo, "upstream", "refs/heads/trunk")
	if err != nil || got != want {
		t.Fatalf("ReadRemoteRef = %q, %v; want %q", got, err, want)
	}
	if _, err := ReadRemoteRef(t.Context(), repo, "--upload-pack=bad", "refs/heads/trunk"); err == nil {
		t.Fatal("option-shaped remote was accepted")
	}
}

func TestSafeGitEnvironmentStripsEveryRepositoryAndConfigSteeringVariable(t *testing.T) {
	prior := environ
	t.Cleanup(func() { environ = prior })
	environ = func() []string {
		return []string{
			"PATH=/usr/bin", "HOME=/tmp/home", "GIT_DIR=/other", "GIT_WORK_TREE=/other",
			"GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=core.sshCommand", "GIT_CONFIG_VALUE_0=bad",
			"GIT_CONFIG_KEY_17=remote.origin.url", "GIT_CONFIG_VALUE_17=bad", "GIT_CONFIG_PARAMETERS='x=y'",
			"GIT_EXEC_PATH=/other", "GIT_TEMPLATE_DIR=/other",
		}
	}
	got := safeGitEnvironment()
	want := []string{"PATH=/usr/bin", "HOME=/tmp/home"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("safe git environment = %v, want %v", got, want)
	}
}

func runArtifactGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}
