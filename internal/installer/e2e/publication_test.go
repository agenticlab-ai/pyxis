package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// localPublicationTransport exercises real Git publication while the GitHub
// API transcript remains hermetic. The report retains the configured URL.
func localPublicationTransport(t *testing.T) func(context.Context, string, ...string) (string, error) {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "--initial-branch=main", bare).CombinedOutput(); err != nil {
		t.Fatalf("bare: %v: %s", err, out)
	}
	execPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, root string, args ...string) (string, error) {
		argv := append([]string(nil), args...)
		argv[len(argv)-2] = bare
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, argv...)...)
		cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+string(os.PathListSeparator)+strings.TrimSpace(string(execPath)))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git publication: %w: %s", err, out)
		}
		return string(out), nil
	}
}

func TestInstalledInitPublishesAndOrdinaryPushEnforcesHistory(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo, env := scratchRepo(t, r)
	tools := t.TempDir()
	installed := filepath.Join(repo, ".pyxis/bin", "pyxis-"+r.version+"-"+r.plat)
	if err := os.Symlink(installed, filepath.Join(tools, "pyxis")); err != nil {
		t.Fatal(err)
	}
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cat, filepath.Join(tools, "cat")); err != nil {
		t.Fatal(err)
	}
	for i, pair := range env {
		if path, ok := strings.CutPrefix(pair, "PATH="); ok {
			env[i] = "PATH=" + tools + string(os.PathListSeparator) + path
		}
	}
	git := func(args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := git("rev-parse", "@{upstream}")
	if err != nil || head != upstream {
		t.Fatalf("installation did not publish and track head: %s %s %v", head, upstream, err)
	}
	if refs, err := git("ls-remote", "--refs", "origin", "refs/heads/main"); err != nil || !strings.HasPrefix(refs, head+"\t") {
		t.Fatalf("configured destination=%q: %v", refs, err)
	}
	vision := filepath.Join(repo, "docs", "product-vision.md")
	body, err := os.ReadFile(vision)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, vision, append(body, []byte("\nUnauthorized post-install governance addition.\n")...), 0o644)
	if out, err := git("add", "--", "docs/product-vision.md"); err != nil {
		t.Fatalf("stage: %s %v", out, err)
	}
	if out, err := git("-c", "core.hooksPath=", "commit", "-m", "unauthorized governance addition"); err != nil {
		t.Fatalf("commit: %s %v", out, err)
	}
	if out, err := git("push", "origin", "HEAD:refs/heads/main"); err == nil || !strings.Contains(out, "Governing-Change") {
		t.Fatalf("ordinary push must enforce installed history: %s %v", out, err)
	}
	if refs, err := git("ls-remote", "--refs", "origin", "refs/heads/main"); err != nil || !strings.HasPrefix(refs, head+"\t") {
		t.Fatalf("rejected push changed destination: %s %v", refs, err)
	}
}
