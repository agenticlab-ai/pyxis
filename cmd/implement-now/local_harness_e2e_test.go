package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Exercise the complete production sweep with real Git and helper-process
// replacements for the external APIs. Both the tracked policy and invoking
// harness select Codex; the ignored machine policy must instead implement with
// OpenCode and independently review with Cursor, including its real PTY shim.
func TestEndToEnd_MachineLocalImplementationAndReviewHarnesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper-process E2E currently targets POSIX Git environments")
	}
	for _, postGreen := range []bool{false, true} {
		name := "before-publish"
		if postGreen {
			name = "after-green"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{Review: true, ReviewPostGreenOnly: postGreen})
			t.Setenv("ANTHROPIC_API_KEY", "e2e-model-auth-only")
			t.Setenv("CURSOR_API_KEY", "e2e-model-auth-only")
			if err := os.WriteFile(filepath.Join(fixture.repo, ".gitignore"), []byte(".pyxis/config.local.yaml\n.pyxis/logs/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(fixture.repo, ".pyxis", "config.yaml")
			tracked, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			tracked = append(tracked, []byte("  routing:\n    default: codex\n")...)
			tracked = append(tracked, []byte("\n"+opencodeCatalogYAML("anthropic/claude-haiku-4-5", "anthropic/claude-sonnet-5", "anthropic/claude-opus-4-8"))...)
			if err := os.WriteFile(configPath, tracked, 0o644); err != nil {
				t.Fatal(err)
			}
			fixture.gitAt(fixture.repo, "add", ".gitignore", ".pyxis/config.yaml")
			fixture.gitAt(fixture.repo, "commit", "-m", "chore: ignore machine routing")
			fixture.gitAt(fixture.repo, "push", "origin", "main")
			const local = "executor:\n  routing:\n    default: opencode\n  review:\n    backend: cursor\nharness-config:\n  opencode:\n    implementation:\n      implementation-strong:\n        model: opencode-go/kimi-k3\n"
			if err := os.WriteFile(filepath.Join(fixture.repo, ".pyxis", "config.local.yaml"), []byte(local), 0o600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if exit := run(fixture.args(), &stdout, &stderr); exit != 0 {
				t.Fatalf("run exit = %d\nstdout:\n%s\nstderr:\n%s", exit, stdout.String(), stderr.String())
			}
			wantReview := "pre-publish-reviewed: 1"
			if postGreen {
				wantReview = "review-completed: 1"
			}
			for _, want := range []string{"succeeded: 1", "failed:    0", wantReview} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("summary missing %q:\n%s", want, stdout.String())
				}
			}
			calls := readE2ECalls(t, fixture.stateDir)
			implementations := callsFor(calls, "opencode", "run")
			reviews := callsFor(calls, "cursor-agent", "-p")
			if len(implementations) != 1 || len(reviews) != 1 || len(callsFor(calls, "codex", "exec")) != 0 {
				t.Fatalf("launches: opencode=%d cursor=%d codex=%d", len(implementations), len(reviews), len(callsFor(calls, "codex", "exec")))
			}
			if !containsArg(implementations[0].Args, "opencode-go/kimi-k3") {
				t.Fatalf("implementation ignored the machine-local model: %q", implementations[0].Args)
			}
			if samePath(implementations[0].Dir, reviews[0].Dir) || samePath(reviews[0].Dir, fixture.repo) {
				t.Fatal("review did not run independently of the implementation worktree and original checkout")
			}
			if !containsArg(reviews[0].Args, "--force") || !containsArg(reviews[0].Args, "disabled") || !containsArg(reviews[0].Args, "--trust") || containsArg(reviews[0].Args, "--session") {
				t.Fatalf("review did not use fresh tools under the shared guard: %q", reviews[0].Args)
			}
			head := fixture.gitBare("rev-parse", e2eHead)
			if !strings.Contains(reviews[0].Stdin, head) {
				t.Fatalf("review did not cover the published revision %s", head)
			}
			if got := fixture.gitBare("show", e2eHead+":e2e-output.txt"); got != "implemented by the stub harness" {
				t.Fatalf("published implementation = %q", got)
			}
			if got := fixture.gitAt(fixture.repo, "status", "--short"); got != "" {
				t.Fatalf("machine-local settings dirtied the checkout: %s", got)
			}
			if got := fixture.gitBare("ls-tree", "-r", "--name-only", e2eHead); strings.Contains(got, "config.local.yaml") {
				t.Fatal("machine-local settings were published")
			}
		})
	}
}
