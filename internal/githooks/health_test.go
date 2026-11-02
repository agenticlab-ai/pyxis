package githooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRepository(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	if _, err := git(t.Context(), root, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestActiveValidatesInstallation(t *testing.T) {
	root := testRepository(t)
	if active, err := Active(t.Context(), root, ".githooks"); err != nil || active {
		t.Fatalf("uninstalled = %v, %v", active, err)
	}
	for _, defect := range []string{"none", "source", "missing", "content", "mode"} {
		t.Run(defect, func(t *testing.T) {
			if err := Install(t.Context(), root); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, ".git", guardDirName, "pre-push")
			switch defect {
			case "source":
				if _, err := git(t.Context(), root, "config", hookSourceKey, "wrong"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := git(context.Background(), root, "config", hookSourceKey, ".githooks"); err != nil {
						t.Error(err)
					}
				})
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "content":
				if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "mode":
				if err := os.Chmod(path, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			active, err := Active(t.Context(), root, ".githooks")
			if defect == "none" {
				if !active || err != nil {
					t.Fatalf("valid installation = %v, %v", active, err)
				}
			} else if active || err == nil || !strings.Contains(err.Error(), "hooks-setup") {
				t.Fatalf("broken installation = %v, %v", active, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Active(ctx, root, ".githooks"); err == nil {
		t.Fatal("cancelled health probe succeeded")
	}
}

func TestActiveRejectsForeignDispatcherAndNonRepository(t *testing.T) {
	root := testRepository(t)
	foreign := filepath.Join(t.TempDir(), guardDirName)
	if _, err := git(t.Context(), root, "config", "core.hooksPath", foreign); err != nil {
		t.Fatal(err)
	}
	if active, err := Active(t.Context(), root, ".githooks"); err != nil || active {
		t.Fatalf("foreign dispatcher = %v, %v", active, err)
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[core]\n hooksPath = "+foreign+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	if active, err := Active(t.Context(), t.TempDir(), ".githooks"); err == nil || active {
		t.Fatalf("non-repository dispatcher = %v, %v", active, err)
	}
}
