package draftshell

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestLocalGitProcessIsFileOnlyAndIgnoresRepositoryOverrides(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("init: %s %v", out, err)
	}
	t.Setenv("GIT_DIR", "/absent/repository")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.https.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")
	if out, err := RunLocalGit(t.Context(), root, nil, "git", "rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(string(out)) != "true" {
		t.Fatalf("override=%s %v", out, err)
	}
	for _, host := range []string{"gh", "forgejo"} {
		if _, err := RunLocalGit(t.Context(), root, nil, host, "version"); err == nil {
			t.Fatal("hosted fallback permitted")
		}
	}
	if _, err := RunLocalGit(t.Context(), root, nil, "git", "ls-remote", "https://example.invalid/repo"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("network transport=%v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := RunLocalGit(ctx, root, nil, "git", "status"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}
