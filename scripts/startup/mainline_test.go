package startup_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The repository's configuration names the mainline. The worktree starts from
// the configured remote branch, and the default pair is left alone.
func TestConfiguredMainlineSelectsWorktreeBase(t *testing.T) {
	f := setup(t)
	upstream := filepath.Join(f.base, "upstream.git")
	run(t, f.base, f.env, "git", "init", "--bare", "--initial-branch=trunk", upstream)
	run(t, f.seed, f.env, "git", "checkout", "-b", "trunk")
	put(t, filepath.Join(f.seed, "src", "file.txt"), "latest trunk\n")
	run(t, f.seed, f.env, "git", "commit", "-am", "Advance trunk")
	run(t, f.seed, f.env, "git", "push", upstream, "trunk")
	trunk := run(t, f.seed, f.env, "git", "rev-parse", "HEAD")
	run(t, f.repo, f.env, "git", "remote", "add", "upstream", upstream)
	put(t, filepath.Join(f.repo, ".pyxis", "config.yaml"), "schema_version: pyxis-config/v2\nbranch: trunk\nremote: upstream\n")
	main := run(t, f.repo, f.env, "git", "rev-parse", "refs/heads/main")
	inv, out, err := f.launch(f.repo, true, true)
	if err != nil || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
		t.Fatalf("configured launch: %+v %v\n%s", inv, err, out)
	}
	if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != trunk {
		t.Fatalf("worktree HEAD %s, want upstream trunk %s", got, trunk)
	}
	if got := run(t, f.repo, f.env, "git", "rev-parse", "refs/heads/trunk"); got != trunk {
		t.Fatalf("local trunk %s, want %s", got, trunk)
	}
	if got := run(t, f.repo, f.env, "git", "rev-parse", "refs/heads/main"); got != main {
		t.Fatalf("unconfigured main moved from %s to %s", main, got)
	}
}

// Configuration pyxis refuses stops the launch before any fetch or worktree.
// The refused value here would otherwise reach git as an option.
func TestRefusedConfigurationDoesNotLaunchOrCreateWorktree(t *testing.T) {
	f := setup(t)
	put(t, filepath.Join(f.repo, ".pyxis", "config.yaml"), "schema_version: pyxis-config/v2\nremote: --upload-pack=false\n")
	before := run(t, f.repo, f.env, "git", "show-ref")
	inv, out, err := f.launch(f.repo, true, true)
	if err == nil || inv.Cwd != "" || !strings.Contains(out, "no worktree created") {
		t.Fatalf("expected no launch: %+v %v\n%s", inv, err, out)
	}
	if after := run(t, f.repo, f.env, "git", "show-ref"); before != after {
		t.Fatal("refused configuration changed refs")
	}
	if _, err := os.Stat(filepath.Join(f.base, ".pyxis-worktrees")); !os.IsNotExist(err) {
		t.Fatalf("refused configuration created a worktree directory: %v", err)
	}
}

// A checkout sourced without an installed pyxis keeps the engine defaults.
func TestMissingPyxisUsesDefaultMainline(t *testing.T) {
	f := setup(t)
	if err := os.Remove(filepath.Join(f.bin, "pyxis")); err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{f.bin, filepath.Dir(git), "/usr/bin", "/bin", "/usr/sbin", "/sbin"}
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(dir, "pyxis")); err == nil {
			t.Skipf("pyxis is installed in system directory %s", dir)
		}
	}
	f.env = append(f.env, "PATH="+strings.Join(dirs, string(os.PathListSeparator)))
	inv, out, err := f.launch(f.repo, true, true)
	if err != nil || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
		t.Fatalf("default launch: %+v %v\n%s", inv, err, out)
	}
	if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
		t.Fatalf("worktree HEAD %s, want origin main %s", got, f.latest)
	}
}
