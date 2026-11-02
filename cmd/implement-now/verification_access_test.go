package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
)

// A nested declarative runner starts with PATH/HOME only. It must retain the
// owned scratch needed by Go and cgo while the inherited OS policy denies Git
// writes from the compiler and test binary.
func TestCodeGatePreservesNestedVerbEnvironment(t *testing.T) {
	t.Setenv("PYXIS_NESTED_VERB_TEST", "1")
	repo := codeGateWorkdir(t, fmt.Sprintf("code-gate:\n  nested-toolchain:\n    command: %s -test.run=^TestCodeGateNestedVerbHelper$\n    env: PYXIS_NESTED_VERB_TEST\n    timeout-s: 120\n", os.Args[0]))
	files := map[string]string{
		"go.mod":   "module fixture\n\ngo 1.26\n",
		"value.go": "package fixture\n\nimport \"C\"\n\nfunc value() int { return int(C.int(1)) }\n",
		"value_test.go": `package fixture
import ("os"; "os/exec"; "path/filepath"; "testing")
func TestNestedToolchain(t *testing.T) {
  if value() != 1 { t.Fatal("cgo toolchain failed") }
  if err := os.WriteFile(filepath.Join(os.TempDir(), "nested-scratch"), []byte("ok"), 0600); err != nil { t.Fatal(err) }
  if out, err := exec.Command("git", "update-ref", "refs/heads/injected", "HEAD").CombinedOutput(); err == nil { t.Fatalf("Git write succeeded: %s", out) }
}
`,
	}
	for path, data := range files {
		if err := os.WriteFile(filepath.Join(repo, path), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runGitCommand(t, repo, "init", "-q", "-b", "main")
	runGitCommand(t, repo, "config", "user.name", "test")
	runGitCommand(t, repo, "config", "user.email", "test@example.invalid")
	runGitCommand(t, repo, "add", ".pyxis/config.yaml", "go.mod", "value.go", "value_test.go")
	runGitCommand(t, repo, "commit", "-qm", "toolchain checkpoint")
	head := runGitCommand(t, repo, "rev-parse", "HEAD")
	if result := codeGateRunner(t.Context(), repo, nil); result.Err != nil || !result.Passed {
		t.Fatalf("nested toolchain: %+v", result)
	}
	if got := runGitCommand(t, repo, "rev-parse", "HEAD"); got != head {
		t.Fatal("nested toolchain changed checkpoint")
	}
}

func TestCodeGateNestedVerbHelper(t *testing.T) {
	if os.Getenv("PYXIS_NESTED_VERB_TEST") != "1" {
		return
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cfg := codegatemirror.Config{Checks: []codegatemirror.Check{{Name: "nested-go", Command: "go test -count=1 ./...", Class: codegatemirror.ClassRequired, TimeoutS: 90}}}
	if report := cfg.Run(context.Background(), dir); !report.Passed {
		t.Fatal(report.FailureSummary())
	}
}

// Exercise the real production verifier recursively. Both levels must keep
// Git metadata protected; no environment switch skips either boundary.
func TestCodeGateConfinesNestedVerification(t *testing.T) {
	repo := codeGateWorkdir(t, fmt.Sprintf("code-gate:\n  nested-verification:\n    command: %s -test.v -test.run=^TestCodeGateConfinesAgentAuthoredTests$\n    timeout-s: 120\n", os.Args[0]))
	runGitCommand(t, repo, "init", "-q", "-b", "main")
	runGitCommand(t, repo, "config", "user.name", "test")
	runGitCommand(t, repo, "config", "user.email", "test@example.invalid")
	runGitCommand(t, repo, "add", ".pyxis/config.yaml")
	runGitCommand(t, repo, "commit", "-qm", "outer checkpoint")
	head := runGitCommand(t, repo, "rev-parse", "HEAD")
	result := codeGateRunner(t.Context(), repo, nil)
	if result.Err != nil || !result.Passed {
		t.Fatalf("nested verifier: %+v", result)
	}
	if got := runGitCommand(t, repo, "rev-parse", "HEAD"); got != head {
		t.Fatal("nested verification changed outer checkpoint")
	}
}

// Run agent-authored Go test code through the production local-gate composition,
// including its compiler and test-binary descendants. A protected harness alone
// would not stop these post-checkpoint writes.
func TestCodeGateConfinesAgentAuthoredTests(t *testing.T) {
	for _, linked := range []bool{false, true} {
		name := "ordinary"
		if linked {
			name = "linked"
		}
		t.Run(name, func(t *testing.T) {
			repo := codeGateWorkdir(t, "code-gate:\n  adversarial-go-test:\n    command: go test -count=1 ./...\n")
			runGitCommand(t, repo, "init", "-q", "-b", "main")
			runGitCommand(t, repo, "config", "user.name", "test")
			runGitCommand(t, repo, "config", "user.email", "test@example.invalid")
			files := map[string]string{
				"go.mod": "module fixture\n\ngo 1.26\n",
				"boundary_test.go": `package fixture
import ("os"; "os/exec"; "path/filepath"; "testing")
func TestGitWritesDenied(t *testing.T) {
  for _, args := range [][]string{
    {"-c", "user.name=attacker", "-c", "user.email=attacker@example.invalid", "commit", "--allow-empty", "-m", "unverified"},
    {"update-ref", "refs/heads/injected", "HEAD"},
    {"config", "boundary.injected", "true"},
  } {
    if out, err := exec.Command("git", args...).CombinedOutput(); err == nil { t.Fatalf("Git mutation succeeded: %v: %s", args, out) }
  }
  gitdir, err := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
  if err != nil { t.Fatal(err) }
  p := string(gitdir[:len(gitdir)-1])
  if err := os.WriteFile(filepath.Join(p, "HEAD"), []byte("ref: refs/heads/injected\n"), 0600); err == nil { t.Fatal("raw metadata mutation succeeded") }
  if err := os.WriteFile(filepath.Join(os.TempDir(), "compiler-scratch"), []byte("ok"), 0600); err != nil { t.Fatal(err) }
}
`,
			}
			for path, data := range files {
				if err := os.WriteFile(filepath.Join(repo, path), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			runGitCommand(t, repo, "add", "go.mod", "boundary_test.go", ".pyxis/config.yaml")
			runGitCommand(t, repo, "commit", "-qm", "supervisor checkpoint")
			work := repo
			if linked {
				work = filepath.Join(t.TempDir(), "task")
				runGitCommand(t, repo, "worktree", "add", "-q", "-b", "task", work)
			}
			head := runGitCommand(t, work, "rev-parse", "HEAD")
			result := codeGateRunner(t.Context(), work, nil)
			if !result.Passed || result.Err != nil {
				t.Fatalf("confined local gate: %+v", result)
			}
			if got := runGitCommand(t, work, "rev-parse", "HEAD"); got != head {
				t.Fatalf("verification moved checkpoint: %s -> %s", head, got)
			}
			if got := strings.TrimSpace(runGitCommand(t, work, "status", "--porcelain")); got != "" {
				t.Fatalf("verification dirtied tree: %s", got)
			}
			if got := strings.TrimSpace(runGitCommand(t, repo, "for-each-ref", "refs/heads/injected")); got != "" {
				t.Fatalf("verification created ref: %s", got)
			}
		})
	}
}
