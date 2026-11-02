package startup_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var harnesses = []string{"claude", "agent", "cursor-agent", "cursor", "opencode"}

func resumeArgs(cli string) []string {
	if cli == "codex" {
		return []string{"resume", "01990eab-1111-7111-8111-111111111111"}
	}
	if cli == "opencode" {
		return []string{"--session", "ses_startup_fixture"}
	}
	return append(freshArgs(cli), "--resume", "01990eab-1111-7111-8111-111111111111")
}

func resumedArgs(cli, cwd string) []string {
	args := resumeArgs(cli)
	if cli == "codex" {
		return append([]string{"--cd", cwd}, args...)
	}
	return args
}

func TestExplicitClaudeResumesUseWorktrees(t *testing.T) {
	for _, variant := range []string{"long", "inline", "short", "attached", "name", "transcript"} {
		t.Run(variant, func(t *testing.T) {
			f := setup(t, "claude")
			before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
			args := resumeArgs("claude")
			want := resumeArgs("claude")
			switch variant {
			case "inline":
				args = []string{"--resume=" + args[1]}
			case "short":
				args[0], want[0] = "-r", "-r"
			case "attached":
				args, want[0] = []string{"-r" + args[1]}, "-r"
			case "name":
				args[1], want[1] = "auth", "auth" // A resume name is not a utility command.
			case "transcript":
				put(t, filepath.Join(f.repo, "conversation.jsonl"), "fixture\n")
				before = run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
				args[1], want[1] = "../conversation.jsonl", filepath.Join(f.repo, "conversation.jsonl")
			}
			inv, out, err := f.launch(filepath.Join(f.repo, "src"), true, true, args...)
			prefix := filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(f.repo), "claude-")
			if err != nil || !strings.HasPrefix(inv.Cwd, prefix) || filepath.Base(inv.Cwd) != "src" {
				t.Fatalf("resume %q: %+v %v\n%s", args, inv, err, out)
			}
			if !reflect.DeepEqual(inv.Args, want) {
				t.Fatalf("resume args=%q, want %q", inv.Args, want)
			}
			if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
				t.Fatalf("resume HEAD=%s, want %s", got, f.latest)
			}
			refs := run(t, f.repo, f.env, "git", "show-ref")
			second, out, err := f.launch(inv.Cwd, true, true, want...)
			if err != nil || second.Cwd != inv.Cwd || !reflect.DeepEqual(second.Args, want) || refs != run(t, f.repo, f.env, "git", "show-ref") {
				t.Fatalf("linked resume: %+v %v\n%s", second, err, out)
			}
			assertOriginal(t, f, before)
			for _, flags := range [][2]bool{{false, false}, {false, true}, {true, false}} {
				inv, out, err := f.launch(f.repo, flags[0], flags[1], args...)
				if err != nil || inv.Cwd != f.repo || !reflect.DeepEqual(inv.Args, args) {
					t.Fatalf("noninteractive resume: %+v %v\n%s", inv, err, out)
				}
			}
			run(t, f.repo, f.env, "git", "remote", "set-url", "origin", filepath.Join(f.base, "missing.git"))
			inv, out, err = f.launch(f.repo, true, true, args...)
			if err == nil || inv.Command != "" {
				t.Fatalf("fetch failure launched resume: %+v %v\n%s", inv, err, out)
			}
		})
	}
}

func freshArgs(cli string) []string {
	if cli == "cursor" {
		return []string{"agent"}
	}
	return nil
}

func family(cli string) string {
	if cli == "agent" || cli == "cursor-agent" {
		return "cursor"
	}
	return cli
}

func TestHarnessesShareWorktreeLifecycle(t *testing.T) {
	for _, cli := range harnesses {
		t.Run(cli, func(t *testing.T) {
			f := setup(t, cli)
			before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
			cwd := filepath.Join(f.repo, "src")
			args := freshArgs(cli)
			inv, out, err := f.launch(cwd, true, true, args...)
			prefix := filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(f.repo), family(cli)+"-")
			if err != nil || !strings.HasPrefix(inv.Cwd, prefix) || filepath.Base(inv.Cwd) != "src" || inv.Command != cli || !reflect.DeepEqual(inv.Args, argsOrEmpty(args)) {
				t.Fatalf("fresh invocation=%+v error=%v\n%s", inv, err, out)
			}
			if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
				t.Fatalf("new worktree has %s, want latest main %s", got, f.latest)
			}
			if got := run(t, inv.Cwd, f.env, "git", "branch", "--show-current"); !strings.HasPrefix(got, "chore/"+family(cli)+"-") {
				t.Fatalf("unexpected branch %q", got)
			}
			assertOriginal(t, f, before)
			refs := run(t, f.repo, f.env, "git", "show-ref")
			second, out, err := f.launch(inv.Cwd, true, true, args...)
			if err != nil || second.Cwd != inv.Cwd || refs != run(t, f.repo, f.env, "git", "show-ref") {
				t.Fatalf("existing worktree changed: %+v %v\n%s", second, err, out)
			}
			data, err := os.ReadFile(f.shellCwd)
			if err != nil || strings.TrimSpace(string(data)) != inv.Cwd {
				t.Fatalf("shell did not stay in the worktree: %q %v", data, err)
			}
		})
	}
}

func argsOrEmpty(args []string) []string {
	if args == nil {
		return []string{}
	}
	return args
}

func assertOriginal(t *testing.T, f *fixture, before string) {
	t.Helper()
	if got := run(t, f.repo, f.env, "git", "status", "--porcelain=v1"); got != before {
		t.Fatalf("original checkout changed: before=%q after=%q", before, got)
	}
	if got := run(t, f.repo, f.env, "git", "branch", "--show-current"); got != "wip/keep" {
		t.Fatalf("original branch changed: %s", got)
	}
}

func TestHarnessPassThrough(t *testing.T) {
	for _, cli := range harnesses {
		t.Run(cli, func(t *testing.T) {
			f := setup(t, cli)
			run(t, f.repo, f.env, "git", "remote", "set-url", "origin", filepath.Join(f.base, "missing.git"))
			before := run(t, f.repo, f.env, "git", "show-ref")
			cases := [][]string{{"-h"}, {"--help"}, {"--version"}, {"-v"}, {"--unknown-option"}, {"--model"}}
			switch cli {
			case "claude":
				cases = append(cases, []string{"-p", "hello"}, []string{"-phello"}, []string{"--model", "auth", "--print", "hello"}, []string{"--bg"}, []string{"--cloud", "hello"}, []string{"--continue"}, []string{"-r"}, []string{"--resume="}, []string{"--fork-session"}, []string{"--from-pr=1170"}, []string{"-w", "own-tree"}, []string{"--tmux"}, []string{"--output-format=json"})
				for _, cmd := range []string{"auth", "doctor", "mcp", "plugin", "plugins", "update", "upgrade", "daemon", "remote-control", "self-hosted-runner", "future-headless", "future-alias"} {
					cases = append(cases, []string{cmd})
				}
			case "agent", "cursor-agent":
				cases = append(cases, resumeArgs(cli), []string{"--resume=session"}, []string{"--resume=-2"})
				cases = append(cases, []string{"-p", "hello"}, []string{"-phello"}, []string{"-fp", "hello"}, []string{"--model", "status", "--print", "hello"}, []string{"--continue"}, []string{"--resume"}, []string{"--resume="}, []string{"--list-models"}, []string{"--worktree"}, []string{"--output-format=json"})
				for _, cmd := range []string{"login", "logout", "mcp", "worker", "status", "whoami", "models", "about", "create-chat", "rule", "ls", "resume", "future-headless", "future-alias"} {
					cases = append(cases, []string{cmd})
				}
			case "cursor":
				cases = append(cases, resumeArgs(cli), []string{"agent", "--resume=session"})
				cases = append(cases, []string{"agent", "--print", "hello"}, []string{"agent", "--continue"}, []string{"agent", "--resume"}, []string{"agent", "--resume", ""}, []string{"agent", "--list-models"}, []string{"agent", "--worktree"}, []string{"agent", "--help"})
			case "opencode":
				cases = append(cases, resumeArgs(cli), []string{"--session=ses_fixture"}, []string{"-s", "ses_fixture"}, []string{"-sses_fixture"})
				cases = append(cases, []string{"run", "hello"}, []string{"--continue"}, []string{"-s"}, []string{"--fork"}, []string{"--session="})
				for _, cmd := range []string{"run", "acp", "attach", "serve", "web", "mcp", "models", "providers", "auth", "agent", "pr", "session", "plugin", "plug", "db", "future-headless", "future-alias", "another-alias"} {
					// A directory named like a utility must not be treated as a project.
					if err := os.Mkdir(filepath.Join(f.repo, cmd), 0700); err != nil {
						t.Fatal(err)
					}
					cases = append(cases, []string{cmd})
				}
			}
			for _, args := range cases {
				inv, out, err := f.launch(f.repo, true, true, args...)
				if err != nil || inv.Cwd != f.repo || inv.Command != cli || !reflect.DeepEqual(inv.Args, argsOrEmpty(args)) {
					t.Fatalf("args=%q invocation=%+v error=%v\n%s", args, inv, err, out)
				}
			}
			for _, flags := range [][2]bool{{false, false}, {false, true}, {true, false}} {
				inv, out, err := f.launch(f.repo, flags[0], flags[1], freshArgs(cli)...)
				if err != nil || inv.Cwd != f.repo {
					t.Fatalf("terminal/interactive=%v invocation=%+v error=%v\n%s", flags, inv, err, out)
				}
			}
			ordinary := filepath.Join(f.base, "ordinary")
			run(t, f.base, f.env, "git", "init", "--initial-branch=main", ordinary)
			for _, cwd := range []string{f.base, ordinary} {
				inv, out, err := f.launch(cwd, true, true, freshArgs(cli)...)
				if err != nil || inv.Cwd != cwd {
					t.Fatalf("outside adopted repo: %+v %v\n%s", inv, err, out)
				}
			}
			if got := run(t, f.repo, f.env, "git", "show-ref"); got != before {
				t.Fatal("pass-through changed Git refs")
			}
			if _, err := os.Stat(filepath.Join(f.base, ".pyxis-worktrees")); !os.IsNotExist(err) {
				t.Fatalf("pass-through created a worktree: %v", err)
			}
		})
	}
}

func TestHarnessArgumentsAndPaths(t *testing.T) {
	for _, cli := range harnesses {
		t.Run(cli, func(t *testing.T) {
			f := setup(t, cli)
			var args, want []string
			subdir := false
			switch cli {
			case "claude":
				args = []string{"--model", "auth", "--add-dir", "../one", "../two", "--mcp-config", "config.json", ` {"mcpServers":{}}`, "--settings={\"x\":true}", "--plugin-dir", "plugin", "--append-system-prompt-file", "prompt.md", "--", "hello world"}
				want = []string{"--model", "auth", "--add-dir", filepath.Join(f.base, "one"), filepath.Join(f.base, "two"), "--mcp-config", filepath.Join(f.repo, "config.json"), ` {"mcpServers":{}}`, "--settings", `{"x":true}`, "--plugin-dir", filepath.Join(f.repo, "plugin"), "--append-system-prompt-file", filepath.Join(f.repo, "prompt.md"), "--", "hello world"}
			case "agent", "cursor-agent", "cursor":
				args = append(freshArgs(cli), "--workspace=src", "--model", "status", "--add-dir", "../shared", "--plugin-dir", "plugin", "fix", "status")
				want = append(freshArgs(cli), "--model", "status", "--add-dir", filepath.Join(f.base, "shared"), "--plugin-dir", filepath.Join(f.repo, "plugin"), "fix", "status")
				subdir = true
			case "opencode":
				args = []string{"--model=provider/model", "--prompt", "run", "--mini", "--", "src"}
				want = []string{"--model", "provider/model", "--prompt", "run", "--mini", "--"}
				subdir = true
			}
			inv, out, err := f.launch(f.repo, true, true, args...)
			if err != nil || inv.Cwd == f.repo || !reflect.DeepEqual(inv.Args, want) || (subdir && filepath.Base(inv.Cwd) != "src") {
				t.Fatalf("got %+v, want args=%q, error=%v\n%s", inv, want, err, out)
			}
		})
	}
}

func TestHarnessFetchFailureAndExitStatus(t *testing.T) {
	for _, cli := range harnesses {
		t.Run(cli, func(t *testing.T) {
			f := setup(t, cli)
			f.env = append(f.env, "PYXIS_PROBE_EXIT=7")
			inv, out, err := f.launch(f.repo, true, true, freshArgs(cli)...)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 7 || inv.Cwd == "" || inv.Cwd == f.repo {
				t.Fatalf("launch exit not preserved: %+v %v\n%s", inv, err, out)
			}
			run(t, f.repo, f.env, "git", "remote", "set-url", "origin", filepath.Join(f.base, "missing.git"))
			before := run(t, f.repo, f.env, "git", "show-ref")
			inv, out, err = f.launch(f.repo, true, true, freshArgs(cli)...)
			if err == nil || inv.Command != "" || before != run(t, f.repo, f.env, "git", "show-ref") {
				t.Fatalf("fetch failure launched harness: %+v %v\n%s", inv, err, out)
			}
		})
	}
}

func TestExplicitProjectsFromOutsideTheRepository(t *testing.T) {
	for _, cli := range []string{"cursor-agent", "opencode"} {
		t.Run(cli, func(t *testing.T) {
			f := setup(t, cli)
			project := filepath.Join(f.repo, "src")
			args := []string{project}
			if cli == "cursor-agent" {
				args = []string{"--workspace", project}
			}
			inv, out, err := f.launch(f.base, true, true, args...)
			prefix := filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(f.repo), family(cli)+"-")
			if err != nil || !strings.HasPrefix(inv.Cwd, prefix) || filepath.Base(inv.Cwd) != "src" || len(inv.Args) != 0 {
				t.Fatalf("explicit project: %+v %v\n%s", inv, err, out)
			}
		})
	}
}

func TestAggregateLoaderCanBeSourcedRepeatedlyFromAnotherDirectory(t *testing.T) {
	for _, cli := range append([]string{"codex"}, harnesses...) {
		t.Run(cli, func(t *testing.T) {
			f := setup(t, cli)
			dir := filepath.Join(f.base, "shell files")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			files, err := filepath.Glob("*.zsh")
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range files {
				data, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(dir, name), string(data))
			}
			f.snippet = filepath.Join(f.base, "rc.zsh")
			loader := "'" + strings.ReplaceAll(filepath.Join(dir, "startup.zsh"), "'", "'\"'\"'") + "'"
			put(t, f.snippet, "source "+loader+"\nsource "+loader+"\n")
			inv, out, err := f.launch(f.repo, true, true, freshArgs(cli)...)
			if err != nil || inv.Cwd == f.repo || inv.Command != cli {
				t.Fatalf("loader: %+v %v\n%s", inv, err, out)
			}
		})
	}
}

func TestExistingAliasesKeepTheirArgumentsAndUseTheWrapper(t *testing.T) {
	for _, cli := range append([]string{"codex"}, harnesses...) {
		t.Run(cli, func(t *testing.T) {
			f := setup(t, cli)
			wrapper := f.snippet
			want := []string{"--model", "alias-model"}
			if cli == "claude" {
				want = []string{"--effort", "high", "--permission-mode", "plan"}
			}
			if cli == "cursor" {
				want = []string{"agent"}
			}
			quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
			f.snippet = filepath.Join(f.base, "alias-rc.zsh")
			put(t, f.snippet, "alias "+cli+"="+quote(cli+" "+strings.Join(want, " "))+"\nsource "+quote(wrapper)+"\nfunction pyxis_alias_probe {\n  "+cli+" \"$@\"\n}\n")
			f.cli = "pyxis_alias_probe"
			inv, out, err := f.launch(f.repo, true, true)
			if err != nil || inv.Cwd == f.repo || inv.Command != cli || !reflect.DeepEqual(inv.Args, want) {
				t.Fatalf("alias lost or bypassed wrapper: %+v %v\n%s", inv, err, out)
			}
		})
	}
}
