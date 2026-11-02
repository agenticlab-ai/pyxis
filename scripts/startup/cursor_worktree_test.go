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

func TestCursorEditorOpensWorktree(t *testing.T) {
	for _, variant := range []string{"bare", "dot", "subdirectory", "outside", "window options", "profile paths", "editor", "separator"} {
		t.Run(variant, func(t *testing.T) {
			f := setup(t, "cursor")
			put(t, filepath.Join(f.repo, ".pyxis", "config.local.yaml"), localConfigFixture)
			before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
			cwd := f.repo
			var args, want []string
			subdir := false
			switch variant {
			case "dot":
				args = []string{"."}
			case "subdirectory":
				cwd, subdir = filepath.Join(f.repo, "src"), true
			case "outside":
				cwd, args, subdir = f.base, []string{filepath.Join(f.repo, "src")}, true
			case "window options":
				args = []string{"--new-window", "src", "--wait", "--disable-extensions"}
				want, subdir = []string{"--new-window", "--wait", "--disable-extensions"}, true
			case "profile paths":
				args = []string{"--profile", "debug profile", "--user-data-dir=../user data", "--extensions-dir", "../extensions", "."}
				want = []string{"--profile", "debug profile", "--user-data-dir", filepath.Join(f.base, "user data"), "--extensions-dir", filepath.Join(f.base, "extensions")}
			case "editor":
				args, want = []string{"editor", "."}, []string{"editor"}
			case "separator":
				args, want = []string{"--", "src"}, []string{"--"}
				subdir = true
			}
			inv, out, err := f.launch(cwd, true, true, args...)
			want = append(want, inv.Cwd)
			prefix := filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(f.repo), "cursor-")
			if err != nil || !strings.HasPrefix(inv.Cwd, prefix) || inv.Command != "cursor" || !reflect.DeepEqual(inv.Args, want) || (subdir && filepath.Base(inv.Cwd) != "src") {
				t.Fatalf("editor invocation: %+v want args=%q error=%v\n%s", inv, want, err, out)
			}
			if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
				t.Fatalf("editor HEAD=%s, want %s", got, f.latest)
			}
			root := run(t, inv.Cwd, f.env, "git", "rev-parse", "--show-toplevel")
			assertLocalConfig(t, filepath.Join(root, ".pyxis", "config.local.yaml"), localConfigFixture, 0o600)
			assertOriginal(t, f, before)
			refs := run(t, f.repo, f.env, "git", "show-ref")
			for _, from := range []string{inv.Cwd, f.base} {
				var reuseArgs []string
				if from == f.base {
					reuseArgs = []string{inv.Cwd}
				}
				reused, out, err := f.launch(from, true, true, reuseArgs...)
				if err != nil || reused.Command != "cursor" || !reflect.DeepEqual(reused.Args, []string{inv.Cwd}) || refs != run(t, f.repo, f.env, "git", "show-ref") {
					t.Fatalf("existing editor worktree: %+v %v\n%s", reused, err, out)
				}
			}
		})
	}
}

func TestCursorEditorPassThrough(t *testing.T) {
	f := setup(t, "cursor")
	// A broken remote proves these commands never attempt the worktree fetch.
	run(t, f.repo, f.env, "git", "remote", "set-url", "origin", filepath.Join(f.base, "missing.git"))
	if err := os.Mkdir(filepath.Join(f.repo, "tunnel"), 0o700); err != nil {
		t.Fatal(err)
	}
	refs := run(t, f.repo, f.env, "git", "show-ref")
	for _, args := range [][]string{
		{"--help"}, {"-v"}, {"--version"}, {"--status"}, {"tunnel"},
		{"--list-extensions"}, {"--install-extension", "sample.extension"},
		{"--add", "src"}, {"--remove", "src"}, {"--diff", "src/file.txt", "untracked.txt"},
		{"--goto", "src/file.txt:2:1"}, {"src/file.txt"}, {"missing-folder"}, {".", "src"},
		{"-"}, {"--chat"}, {"--unknown-option", "."}, {"--profile"}, {"--new-window=true"},
	} {
		inv, out, err := f.launch(f.repo, true, true, args...)
		if err != nil || inv.Cwd != f.repo || inv.Command != "cursor" || !reflect.DeepEqual(inv.Args, args) {
			t.Fatalf("editor pass-through %q: %+v %v\n%s", args, inv, err, out)
		}
	}
	for _, flags := range [][2]bool{{false, false}, {false, true}, {true, false}} {
		inv, out, err := f.launch(f.repo, flags[0], flags[1], ".")
		if err != nil || inv.Cwd != f.repo || !reflect.DeepEqual(inv.Args, []string{"."}) {
			t.Fatalf("editor terminal/interactive=%v: %+v %v\n%s", flags, inv, err, out)
		}
	}
	ordinary := filepath.Join(f.base, "ordinary")
	run(t, f.base, f.env, "git", "init", "--initial-branch=main", ordinary)
	for _, cwd := range []string{f.base, ordinary} {
		inv, out, err := f.launch(cwd, true, true, ".")
		if err != nil || inv.Cwd != cwd || !reflect.DeepEqual(inv.Args, []string{"."}) {
			t.Fatalf("editor outside adopted repo: %+v %v\n%s", inv, err, out)
		}
	}
	if refs != run(t, f.repo, f.env, "git", "show-ref") {
		t.Fatal("editor pass-through changed refs")
	}
	if _, err := os.Stat(filepath.Join(f.base, ".pyxis-worktrees")); !os.IsNotExist(err) {
		t.Fatalf("editor pass-through created a worktree: %v", err)
	}
}

func TestCursorEditorFetchFailureAndExitStatus(t *testing.T) {
	f := setup(t, "cursor")
	f.env = append(f.env, "PYXIS_PROBE_EXIT=7")
	inv, out, err := f.launch(f.repo, true, true)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
		t.Fatalf("editor exit not preserved: %+v %v\n%s", inv, err, out)
	}
	run(t, f.repo, f.env, "git", "remote", "set-url", "origin", filepath.Join(f.base, "missing.git"))
	refs := run(t, f.repo, f.env, "git", "show-ref")
	inv, out, err = f.launch(f.repo, true, true)
	if err == nil || inv.Command != "" || refs != run(t, f.repo, f.env, "git", "show-ref") {
		t.Fatalf("fetch failure launched editor: %+v %v\n%s", inv, err, out)
	}
}
