package runlogs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateRejectsCanceledAndInvalidDestinations(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Create(ctx, root, CLI, "run-*.jsonl"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled create: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, Root)); !os.IsNotExist(err) {
		t.Fatalf("canceled create wrote logs: %v", err)
	}
	if _, err := Create(t.Context(), root, CLI, "invalid/name"); err == nil {
		t.Fatal("accepted a filename containing a directory separator")
	}
	blocked := filepath.Join(root, Root, "blocked")
	if err := os.WriteFile(blocked, []byte("existing content"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(t.Context(), root, Root+"/blocked", "run-*"); err == nil {
		t.Fatal("accepted a regular file as the log directory")
	}
	if _, err := Resolve(t.Context(), root, Root+"/blocked/child", ""); err == nil {
		t.Fatal("accepted a log path beneath a regular file")
	}
}

func TestEventLoggerWarnsAndKeepsTerminalWhenStorageFails(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile(filepath.Join(root, ".pyxis"), []byte("existing content"), 0600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	logger, closeLog := EventLogger(t.Context(), &stderr, "review")
	logger.Info("review_completed")
	closeLog()
	for _, want := range []string{"cannot retain execution diagnostics", `"msg":"review_completed"`} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("missing %q in %s", want, stderr.String())
		}
	}
}

func TestResolveKeepsLogsInsideRepository(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name, override string
		bad            bool
	}{
		{"default", "", false},
		{"relative", Executor + "/custom.jsonl", false},
		{"absolute", filepath.Join(root, Executor, "custom.jsonl"), false},
		{"tracked tree", "docs/events.jsonl", true},
		{"traversal", Root + "/../../events.jsonl", true},
		{"another root", filepath.Join(t.TempDir(), "events.jsonl"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(t.Context(), root, Executor+"/events.jsonl", tc.override)
			if (err != nil) != tc.bad {
				t.Fatalf("Resolve=%q, %v", got, err)
			}
			if !tc.bad && !strings.HasPrefix(got, filepath.Join(root, Root)+string(filepath.Separator)) {
				t.Fatal(got)
			}
		})
	}
	if err := os.MkdirAll(filepath.Join(root, Root), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, Executor)); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(t.Context(), root, Executor+"/events.jsonl", ""); err == nil {
		t.Fatal("symlink escaped log tree")
	}
}

func TestEventLoggerRetainsStderrStream(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	var stderr bytes.Buffer
	logger, closeLog := EventLogger(t.Context(), &stderr, "review")
	logger.Info("review_completed", "task", "T1")
	closeLog()
	files, err := filepath.Glob(filepath.Join(root, Reviews, "review-*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	got, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, stderr.Bytes()) {
		t.Fatalf("file and stderr diverged: %s / %s", got, stderr.Bytes())
	}
	if info, err := os.Stat(files[0]); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("log permissions: %v %v", info, err)
	}
}

func TestRepositoryResolvesPersistentParentOfWorktree(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "seed")
	work := filepath.Join(t.TempDir(), "worker")
	git("worktree", "add", "--detach", work, "HEAD")
	got := Repository(t.Context(), work)
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(got)
	if err != nil || resolved != want {
		t.Fatalf("root=%s want=%s err=%v", got, want, err)
	}
	git("worktree", "remove", work)
	if got := Repository(t.Context(), ""); got != "" {
		t.Fatal(got)
	}
	plain := t.TempDir()
	if got := Repository(t.Context(), plain); got != plain {
		t.Fatal(got)
	}
}
