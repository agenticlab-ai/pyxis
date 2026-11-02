package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellInstallGrammarIsReadOnlyUntilInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"shell"}, 2}, {[]string{"shell", "unknown"}, 2},
		{[]string{"shell", "install", "--unknown"}, 2}, {[]string{"shell", "install", "extra"}, 2},
		{[]string{"shell", "--help"}, 0}, {[]string{"shell", "-h"}, 0},
		{[]string{"shell", "install", "--help"}, 0}, {[]string{"shell", "install", "-h"}, 0},
	} {
		var out bytes.Buffer
		if code := run(tc.args, &out, &out); code != tc.code || !strings.Contains(out.String(), "pyxis shell install") {
			t.Fatalf("%v: code=%d output=%s", tc.args, code, &out)
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("grammar wrote to home: %v %v", entries, err)
	}
}

type closedShellOutput struct{}

func (closedShellOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestShellInstallReportsOutputFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	for _, args := range [][]string{{"shell", "install", "--help"}, {"shell", "install"}} {
		if code := run(args, closedShellOutput{}, io.Discard); code != 1 {
			t.Fatalf("closed output accepted: %v code=%d", args, code)
		}
	}
}

func TestShellInstallUsesEnvironmentAndReportsFailures(t *testing.T) {
	home := t.TempDir()
	dotdir := filepath.Join(home, "zsh")
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", dotdir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	var out bytes.Buffer
	if code := run([]string{"shell", "install"}, &out, &out); code != 0 {
		t.Fatalf("install: %d %s", code, &out)
	}
	rc := filepath.Join(dotdir, ".zshrc")
	data, err := os.ReadFile(rc)
	if err != nil || !strings.Contains(string(data), filepath.Join(home, "data")) || !strings.Contains(out.String(), rc) {
		t.Fatalf("wrong setup: %q %v %s", data, err, &out)
	}
	if err := os.WriteFile(rc, []byte("# >>> pyxis interactive worktrees >>>"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"shell", "install"}, &out, &out); code != 1 || !strings.Contains(out.String(), "unfinished") {
		t.Fatalf("setup failure: %d %s", code, &out)
	}
	t.Setenv("HOME", "")
	out.Reset()
	if code := run([]string{"shell", "install"}, &out, &out); code != 1 {
		t.Fatalf("missing HOME: %d %s", code, &out)
	}
}

func TestShellMainlinePrintsConfiguredRemoteAndBranch(t *testing.T) {
	write := func(config string) string {
		root := t.TempDir()
		if config != "" {
			if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	const v2 = "schema_version: pyxis-config/v2\n"
	for _, tc := range []struct {
		name, config, want string
		code               int
	}{
		{"defaults without configuration", "", "origin\nmain\n", 0},
		{"configured pair", v2 + "branch: release/trunk\nremote: upstream\n", "upstream\nrelease/trunk\n", 0},
		{"invalid configuration", "unknown: value\n", "configuration", 1},
		{"option as remote", v2 + "remote: --upload-pack=false\n", "plain git names", 1},
		{"location as remote", v2 + "remote: https://example.invalid/repo.git\n", "plain git names", 1},
		{"option as branch", v2 + "branch: -main\n", "plain git names", 1},
		{"parent path in branch", v2 + "branch: release/../main\n", "plain git names", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"shell", "mainline", write(tc.config)}, &stdout, &stderr)
			got := stdout.String()
			if tc.code != 0 {
				got = stderr.String()
			}
			if code != tc.code || tc.code == 0 && got != tc.want || !strings.Contains(got, tc.want) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
			}
			if tc.code != 0 && stdout.Len() != 0 {
				t.Fatalf("refusal printed a mainline: %q", &stdout)
			}
		})
	}
	for _, args := range [][]string{{"shell", "mainline"}, {"shell", "mainline", ""}, {"shell", "mainline", "--repo"}, {"shell", "mainline", "a", "b"}} {
		var out bytes.Buffer
		if code := run(args, &out, &out); code != 2 || !strings.Contains(out.String(), "pyxis shell mainline <repository>") {
			t.Fatalf("%v: code=%d output=%s", args, code, &out)
		}
	}
	if code := run([]string{"shell", "mainline", write("")}, closedShellOutput{}, io.Discard); code != 1 {
		t.Fatalf("closed output accepted: code=%d", code)
	}
}
