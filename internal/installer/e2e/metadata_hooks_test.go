package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/lease"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/githooks"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
)

// Install the real payload and pinned pyxis/docgate binaries, then compose the
// installed hooks exactly as the executor does. Claims and evidence contain
// parentless metadata commits; ordinary pushes must still run the real gate.
// The remote is a local bare repository: no service or model is substituted.
func TestInstalledComposedHooksSeparateSupervisorMetadataFromBranchPushes(t *testing.T) {
	// This installed repository has its own origin/main. The source repo's
	// outer gate base is not a commit in the independent fixture's history.
	t.Setenv("PYXIS_CI_BASE", "")
	// Preserve the real Git helper directory before narrowing PATH. In a
	// pre-push environment the selected Git can be a relocatable binary whose
	// installation prefix is lost when the fixture invokes it by symlink.
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitPath, err = filepath.EvalSymlinks(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	helperPath, err := exec.CommandContext(t.Context(), gitPath, "--exec-path").Output()
	gitExecPath := strings.TrimSpace(string(helperPath))
	if err != nil || gitExecPath == "" {
		t.Fatalf("discover Git helper path: %q, %v", helperPath, err)
	}
	t.Setenv("GIT_EXEC_PATH", gitExecPath)
	release := buildFixtureRelease(t)
	repo, env := scratchRepo(t, release)
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	if err := os.Symlink(cat, filepath.Join(tools, "cat")); err != nil {
		t.Fatal(err)
	}
	// Match a machine PATH entry using the actual checksum-verified binary
	// prewarmed by init; the hook itself resolves docgate through its pin.
	installed := filepath.Join(repo, ".pyxis/bin", "pyxis-"+release.version+"-"+release.plat)
	if err := os.Symlink(installed, filepath.Join(tools, "pyxis")); err != nil {
		t.Fatal(err)
	}
	for _, pair := range env {
		key, value, _ := strings.Cut(pair, "=")
		if key == "PATH" {
			// The evidence adapter intentionally strips GIT_EXEC_PATH; retain
			// only this discovered toolchain directory for its real transport.
			value = strings.Join([]string{tools, value, gitExecPath}, string(os.PathListSeparator))
		}
		t.Setenv(key, value)
	}
	runGit := func(args ...string) (string, error) {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	mustGit := func(args ...string) string {
		out, err := runGit(args...)
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return out
	}
	origin := filepath.Join(t.TempDir(), "origin.git")
	// Model an existing remote baseline without asking a first push to compare
	// against a remote-tracking branch that cannot exist yet.
	mustGit("clone", "--bare", "--no-hardlinks", repo, origin)
	mustGit("remote", "set-url", "origin", origin)
	mustGit("fetch", "origin")
	parentHead := mustGit("rev-parse", "HEAD")
	if err := githooks.Install(t.Context(), repo); err != nil {
		t.Fatal(err)
	}
	activeHooks := mustGit("config", "core.hooksPath")
	if active, err := githooks.Active(t.Context(), repo, ".githooks"); err != nil || !active {
		t.Fatalf("installed composed hooks inactive: %v %v", active, err)
	}
	claimStore, err := lease.New(lease.Config{WorkDir: repo, Remote: "origin", Holder: "installed-hook-test"})
	if err != nil {
		t.Fatal(err)
	}
	task := port.Task{ID: "T-metadata", Number: 1}
	claim, err := claimStore.Acquire(t.Context(), task)
	if err != nil {
		t.Fatalf("claim CAS ran the installed branch gate: %v", err)
	}
	claimSHA := mustGit("--git-dir", origin, "rev-parse", "refs/claim/1")
	if err := claim.Release(t.Context()); err != nil {
		t.Fatalf("claim release through composed hooks: %v", err)
	}
	if refs := mustGit("--git-dir", origin, "for-each-ref", "--format=%(refname)", "refs/claim/"); refs != "" {
		t.Fatalf("claim leaked: %s", refs)
	}
	const evidenceRef = "refs/pyxis/installed-hook-test/review"
	store, err := gitrefstore.New(repo, "origin", evidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	const evidence = "{\"schema_version\":\"hook-evidence/v1\"}\n"
	if _, err := store.Update(t.Context(), "review/state.jsonl", "test: append review evidence", func([]byte) ([]byte, bool, error) {
		return []byte(evidence), true, nil
	}); err != nil {
		t.Fatalf("review evidence CAS ran the installed branch gate: %v", err)
	}
	if got, err := store.Read(t.Context(), "review/state.jsonl"); err != nil || !bytes.Equal(got, []byte(evidence)) {
		t.Fatalf("review evidence readback = %q: %v", got, err)
	}
	// The same parentless object is rejected when an ordinary Git push sends it
	// to a branch. The dispatcher has no metadata namespace exemption, either.
	for _, ref := range []string{"refs/heads/metadata-is-not-code", "refs/pyxis/manual-metadata-push"} {
		out, err := runGit("push", "origin", claimSHA+":"+ref)
		if err == nil || !strings.Contains(out, "merge-base") {
			t.Fatalf("ordinary orphan push did not run the real installed gate: %v\n%s", err, out)
		}
		if refs := mustGit("--git-dir", origin, "for-each-ref", "--format=%(refname)", ref); refs != "" {
			t.Fatalf("rejected ordinary push changed origin: %s", refs)
		}
	}
	mustGit("push", "origin", "HEAD:refs/heads/valid-branch")
	if got := mustGit("--git-dir", origin, "rev-parse", "refs/heads/valid-branch"); got != parentHead {
		t.Fatalf("valid branch push = %s, want %s", got, parentHead)
	}
	if out, err := runGit("commit", "--allow-empty", "-m", "forbidden parent mutation"); err == nil || !strings.Contains(out, "refusing to mutate the parent checkout") {
		t.Fatalf("parent guard lost: %v\n%s", err, out)
	}
	if got := mustGit("config", "core.hooksPath"); got != activeHooks {
		t.Fatalf("metadata operation changed hook configuration: %s", got)
	}
	if got := mustGit("rev-parse", "HEAD"); got != parentHead {
		t.Fatalf("metadata operation changed parent HEAD: %s", got)
	}
}
