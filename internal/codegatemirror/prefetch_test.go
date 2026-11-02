package codegatemirror

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrefetchMainlineFetchesAndWarnsWhenBehind(t *testing.T) {
	remote := gitFixture(t)
	writeF(t, remote, "keep.txt", "base\n")
	gitInDir(t, remote, "-c", "core.hooksPath=/dev/null", "add", "keep.txt")
	gitInDir(t, remote, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "base")
	local := t.TempDir()
	cmd := exec.Command("git", "clone", "-q", remote, local)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, out)
	}
	gitInDir(t, remote, "-c", "core.hooksPath=/dev/null", "commit", "--allow-empty", "-qm", "ahead")
	var buf bytes.Buffer
	PrefetchMainline(t.Context(), local, &buf)
	if !strings.Contains(buf.String(), "has commits not in HEAD") {
		t.Fatalf("behind warning missing: %s", buf.String())
	}
	if _, err := os.Stat(filepath.Join(local, ".git")); err != nil {
		t.Fatal(err)
	}
}

func TestPrefetchMainlineSkipsReposWithoutUpstream(t *testing.T) {
	dir := gitFixture(t)
	var buf bytes.Buffer
	PrefetchMainline(t.Context(), dir, &buf)
	if !strings.Contains(buf.String(), "skip mainline fetch: no remotes") {
		t.Fatalf("missing skip: %s", buf.String())
	}
}

func TestPrefetchMainlineSkipsEmptyConfiguredRemote(t *testing.T) {
	dir := gitFixture(t)
	gitInDir(t, dir, "remote", "add", "dummy", filepath.Join(t.TempDir(), "missing.git"))
	writeF(t, dir, ".pyxis/config.yaml", "schema_version: pyxis-config/v2\nsystem:\n  version: \"0.1.0\"\n  mode: source\nremote: \"\"\nbranch: \"\"\n")
	var buf bytes.Buffer
	PrefetchMainline(t.Context(), dir, &buf)
	if !strings.Contains(buf.String(), "skip mainline fetch: empty configured remote or branch") {
		t.Fatalf("missing empty-config skip: %s", buf.String())
	}
}

func TestPrefetchMainlineContinuesWhenFetchFails(t *testing.T) {
	dir := gitFixture(t)
	gitInDir(t, dir, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	var buf bytes.Buffer
	PrefetchMainline(t.Context(), dir, &buf)
	if !strings.Contains(buf.String(), "mainline fetch failed (continuing)") {
		t.Fatalf("missing fetch-failure warning: %s", buf.String())
	}
}

func gitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
