package runtime

import (
	"context"

	"errors"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"os"
	"path/filepath"
	"testing"
)

func TestTaskStatePreservesConversationAndSeparatesHarnesses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", home)
	t.Setenv("XDG_DATA_HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".credentials.json"), []byte("test-credential"), 0o600); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	seen := map[string]bool{}
	for _, backend := range []string{"", "codex", "claude", "cursor", "opencode", "custom-harness"} {
		state, fresh, err := PrepareTaskState(t.Context(), work, entity.Harness(backend))
		if err != nil || !fresh || seen[state] {
			t.Fatalf("initial %s state=%q fresh=%v err=%v", backend, state, fresh, err)
		}
		seen[state] = true
		conversation := filepath.Join(state, "conversation")
		if err := os.WriteFile(conversation, []byte("resume this task"), 0o600); err != nil {
			t.Fatal(err)
		}
		again, fresh, err := PrepareTaskState(t.Context(), work, entity.Harness(backend))
		if err != nil || fresh || again != state {
			t.Fatalf("resume %s: %q fresh=%v err=%v", backend, again, fresh, err)
		}
		if got, err := os.ReadFile(conversation); err != nil || string(got) != "resume this task" {
			t.Fatalf("conversation lost: %q %v", got, err)
		}
	}
}

func TestTaskStateRejectsRedirectedAndIncompleteStorage(t *testing.T) {
	for _, kind := range []string{"root-file", "root-link", "state-link", "incomplete", "marker-link"} {
		t.Run(kind, func(t *testing.T) {
			work, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			root := TaskStateRoot(work)
			other := t.TempDir()
			switch kind {
			case "root-file":
				err = os.WriteFile(root, []byte("occupied"), 0o600)
			case "root-link":
				err = os.Symlink(other, root)
			default:
				err = os.Mkdir(root, 0o700)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "state-link" {
					err = os.Symlink(other, filepath.Join(root, "codex"))
				} else {
					err = os.Mkdir(filepath.Join(root, "codex"), 0o700)
				}
				if err == nil && kind == "marker-link" {
					err = os.Symlink(other, filepath.Join(root, "codex.ready"))
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := PrepareTaskState(t.Context(), work, "codex"); err == nil {
				t.Fatal("unsafe runtime state accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := PrepareTaskState(ctx, t.TempDir(), "codex"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := PrepareTaskState(t.Context(), t.TempDir(), "../other"); err == nil {
		t.Fatal("backend path traversal accepted")
	}
	if _, _, err := PrepareTaskState(t.Context(), "relative", "codex"); err == nil {
		t.Fatal("relative root accepted")
	}
}

func TestTaskStateFailedInitializationCanRetryWithoutLosingOtherSessions(t *testing.T) {
	for _, kind := range []string{"invalid credential", "blocked ready marker"} {
		t.Run(kind, func(t *testing.T) {
			work, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			other, _, err := PrepareTaskState(t.Context(), work, "")
			if err != nil {
				t.Fatal(err)
			}
			conversation := filepath.Join(other, "conversation")
			if err := os.WriteFile(conversation, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			obstacle := filepath.Join(home, "auth.json")
			if kind == "blocked ready marker" {
				obstacle = filepath.Join(TaskStateRoot(work), "codex.ready")
			}
			if err := os.Mkdir(obstacle, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, _, err := PrepareTaskState(t.Context(), work, "codex"); err == nil {
				t.Fatal("incomplete initialization accepted")
			}
			if _, err := os.Lstat(filepath.Join(TaskStateRoot(work), "codex")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed state was not removed: %v", err)
			}
			if got, err := os.ReadFile(conversation); err != nil || string(got) != "keep" {
				t.Fatalf("another harness session was lost: %q %v", got, err)
			}
			if err := os.Remove(obstacle); err != nil {
				t.Fatal(err)
			}
			if _, fresh, err := PrepareTaskState(t.Context(), work, "codex"); err != nil || !fresh {
				t.Fatalf("could not retry initialization: fresh=%v err=%v", fresh, err)
			}
		})
	}
}

func TestTaskStateDirectoryErrorsAreNotIgnored(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "xdg")
	if err := os.WriteFile(blocked, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareStateDirectories(root); err == nil {
		t.Fatal("inaccessible native configuration directory accepted")
	}
	if err := privateStateDirectory(filepath.Join(blocked, "child")); err == nil {
		t.Fatal("inaccessible state root accepted")
	}
}
