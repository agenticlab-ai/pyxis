package ciparity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitCommand(t *testing.T, root string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	return cmd
}

func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := gitCommand(t, root, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestHookExecutionContract(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, want string
		mutate     func(*testing.T, string)
	}{
		{"executable tracked target through symlink", "", func(*testing.T, string) {}},
		{"non-executable checkout", "executable regular file", func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, ".githooks/pre-push"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"non-executable tracked mode", "tracked with executable mode 100755", func(t *testing.T, root string) {
			gitRun(t, root, "config", "core.fileMode", "false")
			gitRun(t, root, "update-index", "--chmod=-x", ".pyxis/hooks/pre-push")
		}},
		{"untracked hook", "tracked with executable mode 100755", func(t *testing.T, root string) {
			gitRun(t, root, "rm", "--cached", "--", ".pyxis/hooks/pre-push")
		}},
		{"non-repository", "read tracked local pre-push mode", func(t *testing.T, root string) {
			if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
				t.Fatal(err)
			}
		}},
		{"directory hook", "executable regular file", func(t *testing.T, root string) {
			hook := filepath.Join(root, ".githooks/pre-push")
			if err := os.Remove(hook); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(hook, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"outside repository", "resolve inside the repository", func(t *testing.T, root string) {
			hook := filepath.Join(root, ".githooks/pre-push")
			if err := os.Remove(hook); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			writeFile(t, outside, "pre-push", testHook)
			if err := os.Symlink(filepath.Join(outside, "pre-push"), hook); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := fixture(t)
			tt.mutate(t, root)
			err := Check(t.Context(), root)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
	if err := checkHook(t.Context(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing repository passed")
	}
}

func TestHookRequiresExactInterpreter(t *testing.T) {
	t.Parallel()
	for _, interpreter := range []string{"#!/bin/true", "#!/bin/bash", "#!/bin/sh -n", "# comment", "", " #!/bin/sh", "#!/bin/sh\r"} {
		t.Run(interpreter, func(t *testing.T) {
			t.Parallel()
			root := fixture(t)
			writeFile(t, root, ".githooks/pre-push", strings.Replace(testHook, "#!/bin/sh", interpreter, 1))
			if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "exact interpreter") {
				t.Fatalf("interpreter bypass escaped: %v", err)
			}
		})
	}
}

func TestValidatedHookBlocksRealPush(t *testing.T) {
	for _, failingGate := range []string{"docgate", "code-gate"} {
		t.Run(failingGate, func(t *testing.T) {
			root := fixture(t)
			if err := Check(t.Context(), root); err != nil {
				t.Fatal(err)
			}
			// Git must execute the validated shim and propagate each delegate's
			// failure. The shell-only integration fixtures stand in for the gates.
			for _, gate := range []string{"docgate", "code-gate"} {
				status := "0"
				if gate == failingGate {
					status = "23"
				}
				rel := ".pyxis/scripts/" + gate + "-resolve.sh"
				writeFile(t, root, rel, "#!/bin/sh\necho "+gate+"-invoked >&2\nexit "+status+"\n")
				if err := os.Chmod(filepath.Join(root, rel), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			gitRun(t, root, "commit", "-qm", "fixture")
			gitRun(t, root, "config", "core.hooksPath", ".githooks")
			remote := t.TempDir()
			gitRun(t, remote, "init", "--bare", "-q")
			out, err := gitCommand(t, root, "push", remote, "HEAD:refs/heads/test").CombinedOutput()
			if err == nil || !strings.Contains(string(out), failingGate+"-invoked") {
				t.Fatalf("push did not run and fail at %s: %v\n%s", failingGate, err, out)
			}
			if refs := gitRun(t, remote, "for-each-ref", "refs/heads"); refs != "" {
				t.Fatalf("rejected push updated remote refs: %s", refs)
			}
			if failingGate == "docgate" && strings.Contains(string(out), "code-gate-invoked") {
				t.Fatal("code gate ran after the document gate failed")
			}
		})
	}
}
