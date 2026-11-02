package events

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestPendingEventsTravelToFreshCloneWithoutChangingDraftFiles(t *testing.T) {
	root := t.TempDir()
	remote, first, second := filepath.Join(root, "remote.git"), filepath.Join(root, "first"), filepath.Join(root, "second")
	git(t, root, "init", "--bare", "-q", remote)
	if err := os.Mkdir(first, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, first, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(first, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, first, "add", "README.md")
	git(t, first, "commit", "-qm", "base")
	git(t, first, "remote", "add", "origin", remote)
	git(t, first, "push", "-q", "origin", "main")
	git(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	git(t, root, "clone", "-q", remote, second)
	prior := git(t, first, "status", "--porcelain")
	one, err := OpenPending(first, "origin", "widget", 1)
	if err != nil {
		t.Fatal(err)
	}
	event := fixtureEvent(t, DraftStepFailed)
	if err := one.Add(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	if after := git(t, first, "status", "--porcelain"); after != prior {
		t.Fatalf("pending event changed working files: before=%q after=%q", prior, after)
	}
	two, err := OpenPending(second, "origin", "widget", 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := two.Read(t.Context())
	if err != nil || len(got) != 1 || got[0].ID != event.ID {
		t.Fatalf("fresh clone pending events = %+v, %v", got, err)
	}
	log, changed, err := Append([]byte(heading), got...)
	if err != nil || !changed {
		t.Fatalf("drain changed=%v err=%v", changed, err)
	}
	if secondLog, changed, err := Append(log, got...); err != nil || changed || string(secondLog) != string(log) {
		t.Fatalf("repeated drain changed=%v err=%v", changed, err)
	}
	if err := two.Add(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	if got, err := one.Read(t.Context()); err != nil || len(got) != 1 {
		t.Fatalf("pending dedupe across clones = %+v, %v", got, err)
	}
}

func TestPendingRefValidatesScopeAndPayload(t *testing.T) {
	for _, tc := range []struct {
		feature string
		version int
	}{{"bad/name", 1}, {"widget", 0}} {
		if _, err := OpenPending(t.TempDir(), "origin", tc.feature, tc.version); err == nil {
			t.Fatalf("invalid pending scope accepted: %+v", tc)
		}
	}
	if err := (*Pending)(nil).Add(context.Background(), fixtureEvent(t, DraftStepFailed)); err == nil {
		t.Fatal("nil pending writer accepted")
	}
	if _, err := (*Pending)(nil).Read(context.Background()); err == nil {
		t.Fatal("nil pending reader accepted")
	}
}
