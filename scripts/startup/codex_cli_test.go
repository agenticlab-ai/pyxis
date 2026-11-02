package startup_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Exercise the real TUI with a saved session from the original checkout. The
// temporary Codex home has no credentials, and its provider points to a closed
// loopback port. Only terminal control replies, /status, and /quit are sent.
func TestRealCodexResumeUsesWorktree(t *testing.T) {
	if os.Getenv("PYXIS_TEST_REAL_CODEX") != "1" {
		t.Skip("set PYXIS_TEST_REAL_CODEX=1 to check the installed Codex CLI")
	}
	f := setup(t)
	if err := os.Remove(filepath.Join(f.base, "bin", "codex")); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(f.base, "codex-home")
	sessions := filepath.Join(home, "sessions", "2026", "09", "01")
	if err := os.MkdirAll(sessions, 0700); err != nil {
		t.Fatal(err)
	}
	const id = "01990eab-1111-7111-8111-111111111111"
	const stamp = "2026-09-01T00:00:00Z"
	type sessionMeta struct {
		ID            string `json:"id"`
		Timestamp     string `json:"timestamp"`
		Cwd           string `json:"cwd"`
		Originator    string `json:"originator"`
		Version       string `json:"cli_version"`
		Source        string `json:"source"`
		ModelProvider string `json:"model_provider"`
	}
	meta, err := json.Marshal(struct {
		Timestamp string      `json:"timestamp"`
		Type      string      `json:"type"`
		Payload   sessionMeta `json:"payload"`
	}{Timestamp: stamp, Type: "session_meta", Payload: sessionMeta{id, stamp, f.repo, "codex-cli", "0.154.0", "cli", "startup-test"}})
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(sessions, "rollout-2026-09-01T00-00-00-"+id+".jsonl"), string(meta)+"\n")
	put(t, filepath.Join(home, "config.toml"), fmt.Sprintf(`model = "gpt-5"
model_provider = "startup-test"
check_for_update_on_startup = false
[projects.%q]
trust_level = "trusted"
[tui]
resume_cwd = "session"
animations = false
[model_providers.startup-test]
name = "Offline startup test"
base_url = "http://127.0.0.1:1/v1"
wire_api = "responses"
requires_openai_auth = false
`, f.repo))
	f.env = append(f.env, "CODEX_HOME="+home, "TERM=xterm-256color", "OPENAI_API_KEY=", "CODEX_API_KEY=")
	wrapper := f.snippet
	f.snippet = filepath.Join(f.base, "real-codex.zsh")
	put(t, f.snippet, "stty cols 400 rows 60\nsource '"+strings.ReplaceAll(wrapper, "'", "'\"'\"'")+"'\n")
	input, replies, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := input.Close(); err != nil {
			t.Error(err)
		}
		if err := replies.Close(); err != nil {
			t.Error(err)
		}
	})
	terminal := &codexTerminal{input: replies}
	f.terminalInput, f.terminalOutput = input, terminal
	before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
	_, out, err := f.launch(f.repo, true, true, "resume", id, "--no-alt-screen")
	plain := terminalANSI.ReplaceAllString(out, "")
	if err != nil {
		t.Fatalf("real Codex resume: %v\n%s", err, plain)
	}
	data, err := os.ReadFile(f.shellCwd)
	if err != nil {
		t.Fatal(err)
	}
	worktree := strings.TrimSpace(string(data))
	directory := regexp.MustCompile(`Directory:\s*` + regexp.QuoteMeta(worktree))
	if !strings.Contains(worktree, ".pyxis-worktrees") || !directory.MatchString(plain) || !strings.Contains(plain, "codex resume "+id) {
		t.Fatalf("Codex did not resume the saved session in %s:\n%s", worktree, plain)
	}
	assertOriginal(t, f, before)
}

var terminalANSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

type codexTerminal struct {
	input         *os.File
	data          bytes.Buffer
	cursorReplies int
	status        bool
	statusEntered bool
	quit          bool
	quitEntered   bool
}

func (c *codexTerminal) Write(data []byte) (int, error) {
	_, err := c.data.Write(data)
	if err != nil {
		return 0, err
	}
	queries := bytes.Count(c.data.Bytes(), []byte("\x1b[6n"))
	for c.cursorReplies < queries {
		if _, err := fmt.Fprint(c.input, "\x1b[1;1R"); err != nil {
			return 0, err
		}
		c.cursorReplies++
	}
	plain := terminalANSI.ReplaceAllString(c.data.String(), "")
	if !c.status && strings.Contains(plain, "gpt-5 default") {
		c.status = true
		if _, err := fmt.Fprint(c.input, "/status"); err != nil {
			return 0, err
		}
	}
	// Submit only after the composer renders the text, so Codex's paste
	// detection does not treat the command and Enter as a single pasted burst.
	if c.status && !c.statusEntered && strings.Contains(plain, "/status") {
		c.statusEntered = true
		if _, err := fmt.Fprint(c.input, "\r"); err != nil {
			return 0, err
		}
	}
	if !c.quit && strings.Contains(plain, "Directory:") {
		c.quit = true
		if _, err := fmt.Fprint(c.input, "/quit"); err != nil {
			return 0, err
		}
	}
	if c.quit && !c.quitEntered && strings.Contains(plain, "/quit") {
		c.quitEntered = true
		if _, err := fmt.Fprint(c.input, "\r"); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

// Help and version flags exercise the installed entry points without submitting
// prompts. Every wrapper must preserve output, exit status, cwd, and Git refs.
func TestInstalledHarnessHelpAndVersion(t *testing.T) {
	if os.Getenv("PYXIS_TEST_REAL_HARNESSES") != "1" {
		t.Skip("set PYXIS_TEST_REAL_HARNESSES=1 to check installed harnesses")
	}
	for _, cli := range append([]string{"codex"}, harnesses...) {
		t.Run(cli, func(t *testing.T) {
			if _, err := exec.LookPath(cli); err != nil {
				t.Skipf("%s is not installed", cli)
			}
			f := setup(t, cli)
			probe := filepath.Join(f.base, "bin", cli)
			if err := os.Remove(probe); err != nil {
				t.Fatal(err)
			}
			// With the recorder removed, PATH selects the original installation.
			// Do not symlink Cursor's shell shim: it searches PATH for the IDE.
			before := run(t, f.repo, f.env, "git", "show-ref")
			wrapper := f.snippet
			baseline := filepath.Join(f.base, "baseline.zsh")
			put(t, baseline, "# No wrapper.\n")
			for _, flag := range []string{"--help", "--version"} {
				args := append(freshArgs(cli), flag)
				f.snippet = baseline
				_, directOut, directErr := f.launch(f.repo, true, true, args...)
				f.snippet = wrapper
				_, wrappedOut, wrappedErr := f.launch(f.repo, true, true, args...)
				if directErr != nil || wrappedErr != nil || directOut != wrappedOut {
					t.Fatalf("%s: direct=%v wrapped=%v\ndirect: %s\nwrapped: %s", flag, directErr, wrappedErr, directOut, wrappedOut)
				}
			}
			data, err := os.ReadFile(f.shellCwd)
			if err != nil || strings.TrimSpace(string(data)) != f.repo {
				t.Fatalf("shell cwd changed: %q %v", data, err)
			}
			if before != run(t, f.repo, f.env, "git", "show-ref") {
				t.Fatal("help/version changed Git refs")
			}
			if _, err := os.Stat(filepath.Join(f.base, ".pyxis-worktrees")); !os.IsNotExist(err) {
				t.Fatalf("help/version created worktrees: %v", err)
			}
		})
	}
}

// This opt-in test runs the installed Codex binary. Both invocations must fail
// at its local Git check before they can submit a model prompt.
func TestRealCodexExecOutsideGitPreservesRejection(t *testing.T) {
	if os.Getenv("PYXIS_TEST_REAL_CODEX") != "1" {
		t.Skip("set PYXIS_TEST_REAL_CODEX=1 to check the installed Codex CLI")
	}
	realCodex, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	realCodex, err = filepath.Abs(realCodex)
	if err != nil {
		t.Fatal(err)
	}
	f := setup(t)
	// Replace only the fixture's recorder so its normal PTY launch path reaches
	// the actual binary, with the same directory and arguments in both cases.
	probe := filepath.Join(f.base, "bin", "codex")
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realCodex, probe); err != nil {
		t.Fatal(err)
	}
	before := run(t, f.repo, f.env, "git", "show-ref")
	wrapper := f.snippet
	baseline := filepath.Join(f.base, "baseline.zsh")
	put(t, baseline, "# No wrapper: resolve the installed Codex binary through PATH.\n")
	args := []string{"exec", "who you are?"}

	f.snippet = baseline
	_, directOut, directErr := f.launch(f.base, true, true, args...)
	const diagnostic = "Not inside a trusted directory and --skip-git-repo-check was not specified."
	if !strings.Contains(directOut, diagnostic) {
		t.Fatalf("installed Codex did not stop at the expected local Git check: %v\n%s", directErr, directOut)
	}
	var directExit *exec.ExitError
	if !errors.As(directErr, &directExit) || directExit.ExitCode() <= 0 {
		t.Fatalf("expected Codex to reject the untrusted directory: %v", directErr)
	}

	f.snippet = wrapper
	_, wrappedOut, wrappedErr := f.launch(f.base, true, true, args...)
	var wrappedExit *exec.ExitError
	if !errors.As(wrappedErr, &wrappedExit) || wrappedExit.ExitCode() != directExit.ExitCode() {
		t.Fatalf("wrapper changed Codex's exit status: direct=%v wrapped=%v\n%s", directErr, wrappedErr, wrappedOut)
	}
	if wrappedOut != directOut {
		t.Fatalf("wrapper changed Codex's output:\ndirect: %q\nwrapped: %q", directOut, wrappedOut)
	}
	data, err := os.ReadFile(f.shellCwd)
	if err != nil || strings.TrimSpace(string(data)) != f.base {
		t.Fatalf("wrapper changed the shell directory: %q %v", data, err)
	}
	if after := run(t, f.repo, f.env, "git", "show-ref"); before != after {
		t.Fatal("headless invocation changed Git refs")
	}
	if _, err := os.Stat(filepath.Join(f.base, ".pyxis-worktrees")); !os.IsNotExist(err) {
		t.Fatalf("headless invocation created worktrees: %v", err)
	}
}
