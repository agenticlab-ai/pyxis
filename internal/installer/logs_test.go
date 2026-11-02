package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise Git itself for both source and installed ignore rules. A fresh clone
// must retain shared decisions and task history without inheriting local logs.
func TestExecutionLogsIgnoredAndSharedRecordsClone(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	// TestMain puts Git behind a temporary symlink. Resolve the real binary
	// so relocatable Git installations can find their clone subprocesses.
	gitPath, err = filepath.EvalSymlinks(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for name, rules := range map[string]string{"source": string(source), "installed": operandGitignore} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
			git := func(dir string, args ...string) string {
				t.Helper()
				out, err := exec.CommandContext(t.Context(), gitPath, append([]string{"-C", dir}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v %s", args, err, out)
				}
				return string(out)
			}
			write := func(rel, body string) {
				t.Helper()
				p := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			git(root, "init", "-q")
			shared := []string{".workitems/items/1.md", "docs/problems/example/delivery.md", "docs/feedback/example/001.md", "logs/historical-decision.md"}
			for _, p := range shared {
				write(p, "shared decision\n")
			}
			// Historical evidence predates the ignore rule. Tracking must
			// survive even when new operational notes at that path are ignored.
			git(root, append([]string{"add", "--"}, shared...)...)
			write(".gitignore", rules)
			for _, area := range []string{"cli", "executor", "watch", "phasemeter", "codegate", "agents/codex/run", "reviews", "contract-gate", "sessions", "tests"} {
				p := ".pyxis/logs/" + area + "/event.jsonl"
				write(p, "local output\n")
				if got := git(root, "check-ignore", "--", p); strings.TrimSpace(got) != p {
					t.Fatalf("not ignored: %s", p)
				}
			}
			journal := ".pyxis/maintenance/logs.jsonl"
			write(journal, "local lifecycle\n")
			if got := git(root, "check-ignore", "--", journal); strings.TrimSpace(got) != journal {
				t.Fatalf("maintenance journal not ignored: %s", got)
			}
			git(root, "add", "--", ".gitignore")
			git(root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "shared state")
			if got := git(root, "status", "--porcelain"); got != "" {
				t.Fatalf("logs dirtied worktree: %s", got)
			}
			if got := git(root, "ls-files", "--", ".pyxis/logs"); got != "" {
				t.Fatalf("logs are tracked: %s", got)
			}
			clone := filepath.Join(t.TempDir(), "other-user")
			git(root, "clone", "-q", root, clone)
			for _, p := range shared {
				b, err := os.ReadFile(filepath.Join(clone, p))
				if err != nil || string(b) != "shared decision\n" {
					t.Fatalf("clone lost shared record %s: %v", p, err)
				}
			}
			if _, err := os.Stat(filepath.Join(clone, ".pyxis/logs")); !os.IsNotExist(err) {
				t.Fatalf("clone contains local logs: %v", err)
			}
		})
	}
}

func TestConvergedCoreIgnoreOwnsMaintenanceJournal(t *testing.T) {
	root := t.TempDir()
	git := exec.CommandContext(t.Context(), "git", "-C", root, "init", "-q")
	if output, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
	rules, err := os.ReadFile(filepath.Join("..", "..", ".pyxis", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, ".pyxis")
	if err := os.MkdirAll(filepath.Join(directory, "maintenance"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".gitignore"), rules, 0o600); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(directory, "maintenance", "logs.jsonl")
	if err := os.WriteFile(journal, []byte("local lifecycle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "git", "-C", root, "check-ignore", "--", ".pyxis/maintenance/logs.jsonl")
	if output, err := command.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != ".pyxis/maintenance/logs.jsonl" {
		t.Fatalf("converged ignore did not own maintenance journal: %v %s", err, output)
	}
}
