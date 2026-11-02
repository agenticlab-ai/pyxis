package startup_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type invocation struct {
	Cwd     string   `json:"cwd"`
	Args    []string `json:"args"`
	Command string   `json:"command"`
}

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "pyxis-session-hook" {
		data, err := io.ReadAll(os.Stdin)
		if err == nil {
			err = os.WriteFile(os.Getenv("PYXIS_INVOCATION"), data, 0600)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(`{"systemMessage":"PYXIS_RESUME_READY"}`)
		os.Exit(0)
	}
	name := filepath.Base(os.Args[0])
	if name == "cp" {
		fmt.Fprintln(os.Stderr, "simulated copy failure")
		os.Exit(1)
	}
	if name == "codex" || name == "claude" || name == "agent" || name == "cursor-agent" || name == "cursor" || name == "opencode" {
		if name == "codex" && reflect.DeepEqual(os.Args[1:], []string{"completion", "zsh"}) {
			data, err := os.ReadFile(os.Getenv("PYXIS_COMPLETION"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Print(string(data))
			os.Exit(0)
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		data, err := json.Marshal(invocation{Cwd: cwd, Args: os.Args[1:], Command: name})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(os.Getenv("PYXIS_INVOCATION"), data, 0600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if name != "codex" && len(os.Args) > 1 && os.Args[len(os.Args)-1] == "--help" {
			data, err := os.ReadFile(os.Getenv("PYXIS_HELP"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Print(string(data))
		}
		code := 0
		if raw := os.Getenv("PYXIS_PROBE_EXIT"); raw != "" {
			var err error
			code, err = strconv.Atoi(raw)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		os.Exit(code)
	}
	code := m.Run()
	if builtPyxis.dir != "" {
		_ = os.RemoveAll(builtPyxis.dir) // Best-effort cleanup of the test build.
	}
	os.Exit(code)
}

var builtPyxis struct {
	once      sync.Once
	dir, path string
	err       error
}

// pyxisBinary builds the shipped entry point once per test process. Fixtures
// put it on PATH so the launcher reads each repository's remote and branch
// through the real verb, never through a pyxis the host happens to have.
func pyxisBinary(t *testing.T) string {
	t.Helper()
	builtPyxis.once.Do(func() {
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err == nil {
			builtPyxis.dir, err = os.MkdirTemp("", "pyxis-startup-test-")
		}
		if err != nil {
			builtPyxis.err = err
			return
		}
		builtPyxis.path = filepath.Join(builtPyxis.dir, "pyxis")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", builtPyxis.path, "./cmd/pyxis")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			builtPyxis.err = fmt.Errorf("build pyxis: %w\n%s", err, out)
		}
	})
	if builtPyxis.err != nil {
		t.Fatal(builtPyxis.err)
	}
	return builtPyxis.path
}

type fixture struct {
	loadRC           bool
	t                *testing.T
	repo, seed, base string
	bin              string
	snippet, record  string
	shellCwd, latest string
	env              []string
	zsh, script      string
	cli              string
	terminalInput    io.Reader
	terminalOutput   io.Writer
}

func run(t *testing.T, cwd string, env []string, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = cwd, env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %q: %v\n%s", name, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T, harness ...string) *fixture {
	t.Helper()
	cli, shellFile, helpFile := "codex", "codex-worktree.zsh", ""
	if len(harness) > 0 {
		cli = harness[0]
		family := cli
		if cli == "agent" || cli == "cursor-agent" {
			family = "cursor"
		}
		shellFile, helpFile = family+"-worktree.zsh", "testdata/"+family+"-help.txt"
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("terminal integration tests require macOS or Linux")
	}
	for _, tool := range []string{"git", "zsh", "script"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("terminal integration tests require %s: %v", tool, err)
		}
	}
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Fatal(err)
	}
	script, err := exec.LookPath("script")
	if err != nil {
		t.Fatal(err)
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(base, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, filepath.Join(bin, cli)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(pyxisBinary(t), filepath.Join(bin, "pyxis")); err != nil {
		t.Fatal(err)
	}
	snippet, err := filepath.Abs(shellFile)
	if err != nil {
		t.Fatal(err)
	}
	completion, err := filepath.Abs("testdata/codex-commands.zsh.txt")
	if err != nil {
		t.Fatal(err)
	}
	helpFile, err = filepath.Abs(helpFile)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, base: base, bin: bin, repo: filepath.Join(base, "sample application"), seed: filepath.Join(base, "seed"), snippet: snippet,
		record: filepath.Join(base, "invocation.json"), shellCwd: filepath.Join(base, "shell-cwd"), zsh: zsh, script: script, cli: cli}
	f.env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GORACE=atexit_sleep_ms=0",
		"PYXIS_COMPLETION="+completion, "PYXIS_HELP="+helpFile, "PYXIS_INVOCATION="+f.record, "PYXIS_SHELL_CWD="+f.shellCwd)
	origin := filepath.Join(base, "origin.git")
	run(t, base, f.env, "git", "init", "--bare", "--initial-branch=main", origin)
	run(t, base, f.env, "git", "init", "--initial-branch=main", f.seed)
	run(t, f.seed, f.env, "git", "config", "user.name", "Shell test")
	run(t, f.seed, f.env, "git", "config", "user.email", "shell-test@example.invalid")
	if err := os.Mkdir(filepath.Join(f.seed, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(f.seed, "src", "file.txt"), "initial\n")
	if err := os.Mkdir(filepath.Join(f.seed, ".pyxis"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(f.seed, ".pyxis", "README"), "This repository opts into the startup wrapper.\n")
	put(t, filepath.Join(f.seed, ".gitignore"), ".pyxis/config.local.yaml\n.vscode/\n")
	run(t, f.seed, f.env, "git", "add", "src/file.txt", ".pyxis/README", ".gitignore")
	run(t, f.seed, f.env, "git", "commit", "-m", "Initial main")
	run(t, f.seed, f.env, "git", "remote", "add", "origin", origin)
	run(t, f.seed, f.env, "git", "push", "origin", "main")
	run(t, base, f.env, "git", "clone", origin, f.repo)
	run(t, f.repo, f.env, "git", "checkout", "-b", "wip/keep")
	put(t, filepath.Join(f.repo, "src", "file.txt"), "user changes\n")
	put(t, filepath.Join(f.repo, "untracked.txt"), "keep me\n")
	put(t, filepath.Join(f.seed, "src", "file.txt"), "latest main\n")
	run(t, f.seed, f.env, "git", "add", "src/file.txt")
	run(t, f.seed, f.env, "git", "commit", "-m", "Advance remote main")
	run(t, f.seed, f.env, "git", "push", "origin", "main")
	f.latest = run(t, f.seed, f.env, "git", "rev-parse", "HEAD")
	return f
}

func (f *fixture) launch(cwd string, terminal, interactive bool, args ...string) (invocation, string, error) {
	f.t.Helper()
	if err := os.Remove(f.record); err != nil && !os.IsNotExist(err) {
		f.t.Fatal(err)
	}
	flags := "-dfc"
	if interactive {
		flags = "-dfic"
	}
	prepare := `source "$1"; shift; `
	if f.loadRC {
		flags = strings.ReplaceAll(flags, "f", "")
		prepare = `shift; ` // Let zsh itself load the installed .zshrc.
	}
	command := []string{f.zsh, flags, prepare + `cli="$1"; shift; "$cli" "$@"; code=$?; builtin pwd > "$PYXIS_SHELL_CWD"; exit $code`, "zsh", f.snippet, f.cli}
	command = append(command, args...)
	name := command[0]
	argv := command[1:]
	if terminal {
		name = f.script
		if runtime.GOOS == "darwin" {
			argv = append([]string{"-q", "/dev/null"}, command...)
		} else {
			quoted := make([]string, len(command))
			for i, arg := range command {
				quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
			}
			argv = []string{"-q", "-e", "-c", "exec " + strings.Join(quoted, " "), "/dev/null"}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, argv...)
	cmd.Dir, cmd.Env = cwd, f.env
	cmd.WaitDelay = time.Second
	var output bytes.Buffer
	cmd.Stdin, cmd.Stdout = f.terminalInput, &output
	if f.terminalOutput != nil {
		cmd.Stdout = io.MultiWriter(&output, f.terminalOutput)
	}
	cmd.Stderr = cmd.Stdout
	err := cmd.Run()
	var inv invocation
	if data, readErr := os.ReadFile(f.record); readErr == nil {
		if decodeErr := json.Unmarshal(data, &inv); decodeErr != nil {
			f.t.Fatal(decodeErr)
		}
	} else if !os.IsNotExist(readErr) {
		f.t.Fatal(readErr)
	}
	return inv, output.String(), err
}

func TestExcludedInvocationsDoNotFetchOrCreateWorktrees(t *testing.T) {
	f := setup(t)
	before := run(t, f.repo, f.env, "git", "show-ref")
	data, err := os.ReadFile("testdata/codex-commands.zsh.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, block, ok := strings.Cut(string(data), "\n_codex_commands() {")
	if !ok {
		t.Fatal("completion has no top-level commands")
	}
	block, _, _ = strings.Cut(block, "\n}")
	var commands []string
	for line := range strings.SplitSeq(block, "\n") {
		if entry, ok := strings.CutPrefix(line, "'"); ok {
			command, _, _ := strings.Cut(entry, ":")
			commands = append(commands, command)
		}
	}
	for _, subcommand := range commands {
		t.Run(subcommand, func(t *testing.T) {
			args := []string{"--model", "review", subcommand}
			inv, out, err := f.launch(f.repo, true, true, args...)
			if err != nil || inv.Cwd != f.repo || !reflect.DeepEqual(inv.Args, args) {
				t.Fatalf("invocation=%+v error=%v\n%s", inv, err, out)
			}
		})
	}
	for _, args := range [][]string{
		{"-h"}, {"--help"}, {"-V"}, {"--version"}, {"--model", "-h"}, {"--model=exec", "--help"},
		{"-c", "features.x=true", "exec", "--json"}, {"--remote", "ws://localhost:1234"}, {"--unknown-option"}, {"-m"},
		{"exec", "who you are?"}, {"exec", "--skip-git-repo-check", "who you are?"},
		{"resume"}, {"resume", "--last"}, {"resume", "--all"}, {"resume", ""},
		{"resume", "session", "--worktree"}, {"exec", "resume", "session"},
	} {
		inv, out, err := f.launch(f.repo, true, true, args...)
		if err != nil || inv.Cwd != f.repo || !reflect.DeepEqual(inv.Args, args) {
			t.Fatalf("args=%q invocation=%+v error=%v\n%s", args, inv, err, out)
		}
	}
	for _, flags := range [][2]bool{{false, false}, {false, true}, {true, false}} {
		inv, out, err := f.launch(f.repo, flags[0], flags[1], "hello")
		if err != nil || inv.Cwd != f.repo {
			t.Fatalf("tty/interactive=%v invocation=%+v error=%v\n%s", flags, inv, err, out)
		}
	}
	inv, out, err := f.launch(f.base, true, true, "hello")
	if err != nil || inv.Cwd != f.base {
		t.Fatalf("outside repo invocation=%+v error=%v\n%s", inv, err, out)
	}
	ordinary := filepath.Join(f.base, "ordinary-repository")
	run(t, f.base, f.env, "git", "init", "--initial-branch=main", ordinary)
	inv, out, err = f.launch(f.repo, true, true, "--cd", ordinary, "hello")
	if err != nil || inv.Cwd != f.repo || !reflect.DeepEqual(inv.Args, []string{"--cd", ordinary, "hello"}) {
		t.Fatalf("explicit other cwd invocation=%+v error=%v\n%s", inv, err, out)
	}
	if after := run(t, f.repo, f.env, "git", "show-ref"); before != after {
		t.Fatal("excluded invocation changed refs")
	}
	if got := run(t, f.repo, f.env, "git", "worktree", "list", "--porcelain"); strings.Count(got, "worktree ") != 1 {
		t.Fatalf("excluded invocation created a worktree: %s", got)
	}
}

func TestExplicitCodexResumeUsesWorktree(t *testing.T) {
	f := setup(t)
	before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
	const session = "01990eab-1111-7111-8111-111111111111"
	for _, args := range [][]string{
		{"resume", session},
		{"--model", "review", "resume", session, "exec"},
		{"resume", "exec"}, // Session names and prompts are not CLI subcommands.
		{"resume", "--", session, "continue the work"},
		{"-Csrc", "resume", session, "--image=../image.png"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			inv, out, err := f.launch(f.repo, true, true, args...)
			prefix := filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(f.repo), "codex-")
			if err != nil || !strings.HasPrefix(inv.Cwd, prefix) {
				t.Fatalf("resume=%+v error=%v\n%s", inv, err, out)
			}
			want := append([]string{"--cd", inv.Cwd}, args...)
			if args[0] == "-Csrc" {
				want = []string{"--cd", inv.Cwd, "resume", session, "--image", filepath.Join(f.base, "image.png")}
				if filepath.Base(inv.Cwd) != "src" {
					t.Fatalf("resume lost the requested subdirectory: %s", inv.Cwd)
				}
			}
			if !reflect.DeepEqual(inv.Args, want) {
				t.Fatalf("resume args=%q, want %q", inv.Args, want)
			}
			if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
				t.Fatalf("worktree HEAD=%s, want %s", got, f.latest)
			}
			refs := run(t, f.repo, f.env, "git", "show-ref")
			second, out, err := f.launch(inv.Cwd, true, true, "resume", session)
			if err != nil || second.Cwd != inv.Cwd || !reflect.DeepEqual(second.Args, []string{"--cd", inv.Cwd, "resume", session}) {
				t.Fatalf("linked-worktree resume=%+v error=%v\n%s", second, err, out)
			}
			if refs != run(t, f.repo, f.env, "git", "show-ref") {
				t.Fatal("linked-worktree resume changed refs")
			}
			assertOriginal(t, f, before)
		})
	}
}

func TestInteractiveFetchesMainAndPreservesCheckout(t *testing.T) {
	f := setup(t)
	before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
	for _, args := range [][]string{nil, {"fix the build"}, {"--", "exec"}, {"--", "--help"}, {"--model", "exec", "hello"}, {"-mreview", "hello"}} {
		inv, out, err := f.launch(f.repo, true, true, args...)
		if err != nil || !strings.HasPrefix(inv.Cwd, filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(f.repo), "codex-")) {
			t.Fatalf("args=%q invocation=%+v error=%v\n%s", args, inv, err, out)
		}
		if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
			t.Fatalf("worktree has %s; latest is %s", got, f.latest)
		}
		branch := run(t, inv.Cwd, f.env, "git", "branch", "--show-current")
		if !strings.HasPrefix(branch, "chore/codex-") {
			t.Fatalf("unexpected branch %s", branch)
		}
		if got := run(t, inv.Cwd, f.env, "git", "for-each-ref", "--format=%(upstream)", "refs/heads/"+branch); got != "" {
			t.Fatalf("branch tracks an upstream: %s", got)
		}
		data, err := os.ReadFile(f.shellCwd)
		if err != nil || strings.TrimSpace(string(data)) != inv.Cwd {
			t.Fatalf("shell did not stay in worktree: %s, %v", data, err)
		}
		second, out, err := f.launch(inv.Cwd, true, true, "continue")
		if err != nil || second.Cwd != inv.Cwd {
			t.Fatalf("existing worktree changed: %+v %v\n%s", second, err, out)
		}
	}
	if got := run(t, f.repo, f.env, "git", "branch", "--show-current"); got != "wip/keep" {
		t.Fatalf("primary checkout switched branch to %s", got)
	}
	if got := run(t, f.repo, f.env, "git", "rev-parse", "refs/heads/main"); got != f.latest {
		t.Fatalf("local main=%s, want %s", got, f.latest)
	}
	if after := run(t, f.repo, f.env, "git", "status", "--porcelain=v1"); before != after {
		t.Fatal("primary checkout's changes were altered")
	}
	data, err := os.ReadFile(filepath.Join(f.repo, "src", "file.txt"))
	if err != nil || string(data) != "user changes\n" {
		t.Fatalf("dirty file changed: %s %v", data, err)
	}
}

func TestInteractiveFastForwardsCheckedOutMain(t *testing.T) {
	f := setup(t)
	run(t, f.repo, f.env, "git", "checkout", "-f", "main")
	if got := run(t, f.repo, f.env, "git", "rev-parse", "HEAD"); got == f.latest {
		t.Fatal("precondition: local main should be behind origin")
	}
	inv, out, err := f.launch(f.repo, true, true)
	if err != nil || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
		t.Fatalf("launch: %+v %v\n%s", inv, err, out)
	}
	if got := run(t, f.repo, f.env, "git", "branch", "--show-current"); got != "main" {
		t.Fatalf("original branch changed: %s", got)
	}
	if got := run(t, f.repo, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
		t.Fatalf("checked-out main=%s, want %s", got, f.latest)
	}
	data, err := os.ReadFile(filepath.Join(f.repo, "src", "file.txt"))
	if err != nil || string(data) != "latest main\n" {
		t.Fatalf("working tree not fast-forwarded: %s %v", data, err)
	}
}

func TestInteractiveLeavesDirtyMainUnchanged(t *testing.T) {
	f := setup(t)
	run(t, f.repo, f.env, "git", "checkout", "-f", "main")
	behind := run(t, f.repo, f.env, "git", "rev-parse", "HEAD")
	put(t, filepath.Join(f.repo, "src", "file.txt"), "dirty on main\n")
	inv, out, err := f.launch(f.repo, true, true)
	if err != nil || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
		t.Fatalf("launch: %+v %v\n%s", inv, err, out)
	}
	if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
		t.Fatalf("worktree HEAD=%s, want %s", got, f.latest)
	}
	if got := run(t, f.repo, f.env, "git", "branch", "--show-current"); got != "main" {
		t.Fatalf("original branch changed: %s", got)
	}
	if got := run(t, f.repo, f.env, "git", "rev-parse", "HEAD"); got != behind {
		t.Fatalf("dirty main moved: %s, want %s", got, behind)
	}
	data, err := os.ReadFile(filepath.Join(f.repo, "src", "file.txt"))
	if err != nil || string(data) != "dirty on main\n" {
		t.Fatalf("dirty file changed: %s %v", data, err)
	}
}

func TestDiscoversMultipleRepositoriesWithoutConfiguration(t *testing.T) {
	f := setup(t)
	peer := filepath.Join(f.base, "another-repository")
	run(t, f.base, f.env, "git", "clone", filepath.Join(f.base, "origin.git"), peer)
	for _, repo := range []string{f.repo, peer} {
		t.Run(filepath.Base(repo), func(t *testing.T) {
			inv, out, err := f.launch(filepath.Join(repo, "src"), true, true, "hello")
			prefix := filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(repo), "codex-")
			if err != nil || !strings.HasPrefix(inv.Cwd, prefix) || filepath.Base(inv.Cwd) != "src" {
				t.Fatalf("discovery: %+v %v\n%s", inv, err, out)
			}
			if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
				t.Fatalf("discovered worktree has %s; latest is %s", got, f.latest)
			}
		})
	}
	inv, out, err := f.launch(f.base, true, true, "--cd", filepath.Join(peer, "src"), "hello")
	if err != nil || !strings.HasPrefix(inv.Cwd, filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(peer), "codex-")) || filepath.Base(inv.Cwd) != "src" || !reflect.DeepEqual(inv.Args, []string{"hello"}) {
		t.Fatalf("explicit adopted directory: %+v %v\n%s", inv, err, out)
	}
}

func TestRequiresDirectoryMarkerInNearestGitRoot(t *testing.T) {
	f := setup(t)
	ordinary := filepath.Join(f.base, "ordinary")
	nested := filepath.Join(f.repo, "nested")
	for _, repo := range []string{ordinary, nested} {
		run(t, f.base, f.env, "git", "init", "--initial-branch=main", repo)
	}
	// A marker below the root, a marker in an outer repository, and a marker
	// outside Git must not enable an otherwise unadopted location.
	child := filepath.Join(ordinary, "child")
	if err := os.MkdirAll(filepath.Join(child, ".pyxis"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.base, ".pyxis"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(ordinary, ".pyxis"), "a file is not a directory\n")
	before := run(t, f.repo, f.env, "git", "show-ref")
	for _, cwd := range []string{ordinary, nested, child, f.base} {
		inv, out, err := f.launch(cwd, true, true, "hello")
		if err != nil || inv.Cwd != cwd || !reflect.DeepEqual(inv.Args, []string{"hello"}) {
			t.Fatalf("unadopted location %s: %+v %v\n%s", cwd, inv, err, out)
		}
	}
	if after := run(t, f.repo, f.env, "git", "show-ref"); before != after {
		t.Fatal("unadopted location changed the outer repository's refs")
	}
	if _, err := os.Stat(filepath.Join(f.base, ".pyxis-worktrees")); !os.IsNotExist(err) {
		t.Fatalf("unadopted location created worktrees: %v", err)
	}
}

func TestPathsAndLaunchExitStatus(t *testing.T) {
	f := setup(t)
	args := []string{"--cd=src", "--image", "first.png,second image.png", "--add-dir=../shared", "hello world"}
	inv, out, err := f.launch(f.repo, true, true, args...)
	if err != nil || filepath.Base(inv.Cwd) != "src" {
		t.Fatalf("invocation=%+v error=%v\n%s", inv, err, out)
	}
	want := []string{"--image", filepath.Join(f.repo, "first.png") + "," + filepath.Join(f.repo, "second image.png"), "--add-dir", filepath.Join(f.base, "shared"), "hello world"}
	if !reflect.DeepEqual(inv.Args, want) {
		t.Fatalf("paths: got %q, want %q", inv.Args, want)
	}
	f.env = append(f.env, "PYXIS_PROBE_EXIT=7")
	inv, out, err = f.launch(f.repo, true, true)
	if err == nil || inv.Cwd == "" {
		t.Fatalf("expected launched Codex failure: %+v %v\n%s", inv, err, out)
	}
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 7 {
		t.Fatalf("exit status not propagated: %v", err)
	}
}

func TestFetchFailureDoesNotLaunchOrCreateWorktree(t *testing.T) {
	f := setup(t)
	run(t, f.repo, f.env, "git", "remote", "set-url", "origin", filepath.Join(f.base, "missing.git"))
	before := run(t, f.repo, f.env, "git", "show-ref")
	inv, out, err := f.launch(f.repo, true, true)
	if err == nil || inv.Cwd != "" {
		t.Fatalf("expected no launch: %+v %v\n%s", inv, err, out)
	}
	if after := run(t, f.repo, f.env, "git", "show-ref"); before != after {
		t.Fatal("fetch failure changed refs")
	}
	if _, err := os.Stat(filepath.Join(f.base, ".pyxis-worktrees")); !os.IsNotExist(err) {
		t.Fatalf("fetch failure created a worktree directory: %v", err)
	}
}
