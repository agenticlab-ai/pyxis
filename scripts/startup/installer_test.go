package startup_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Build the actual shipped entry point and install its verified bytes through
// install.sh. Shell launch recorders observe the harness boundary; Git, the
// installer, asset extraction, and zsh's normal rc loading are all real.
func TestMachineInstallerEnablesAllHarnessesInFreshZsh(t *testing.T) {
	installer, source, checksum := buildInstallerRelease(t)
	for _, cli := range append([]string{"codex"}, harnesses...) {
		t.Run(cli, func(t *testing.T) {
			f := setup(t, cli)
			put(t, filepath.Join(f.repo, ".pyxis", "config.local.yaml"), localConfigFixture)
			home := filepath.Join(f.base, "operator home")
			dotdir := filepath.Join(home, "zsh 'config' $literal")
			datahome := filepath.Join(home, "data 'quoted' $literal")
			if err := os.MkdirAll(dotdir, 0o700); err != nil {
				t.Fatal(err)
			}
			rc := filepath.Join(dotdir, ".zshrc")
			original := "# keep my settings\nexport PYXIS_EXISTING_SETTING=preserved\n"
			put(t, rc, original)
			f.env = append(f.env, "HOME="+home, "ZDOTDIR="+dotdir, "XDG_DATA_HOME="+datahome,
				"XDG_CONFIG_HOME="+filepath.Join(home, "config"), "PYXIS_ARTIFACT_SOURCE=file://"+source,
				"PYXIS_SHA256="+checksum, "PYXIS_INSTALL_DIR="+filepath.Join(home, "bin"), "PYXIS_VERSION=", "PYXIS_CHANNEL=")
			f.loadRC = true
			before := run(t, f.repo, f.env, "git", "show-ref")
			out := run(t, f.base, f.env, "sh", installer)
			if !strings.Contains(out, "interactive worktrees enabled") {
				t.Fatalf("no setup: %s", out)
			}
			first, err := os.ReadFile(rc)
			if err != nil || !bytes.HasPrefix(first, []byte(original)) || strings.Count(string(first), "# >>> pyxis interactive worktrees >>>") != 1 {
				t.Fatalf("rc contents: %q %v", first, err)
			}
			run(t, f.base, f.env, "sh", installer)
			second, err := os.ReadFile(rc)
			if err != nil || !bytes.Equal(first, second) {
				t.Fatalf("reinstall changed rc: %q %v", second, err)
			}
			for _, args := range [][]string{append(freshArgs(cli), "--help"), headlessArgs(cli)} {
				inv, out, err := f.launch(f.repo, true, true, args...)
				if err != nil || inv.Cwd != f.repo || !reflect.DeepEqual(inv.Args, args) {
					t.Fatalf("pass-through %v: %+v %v\n%s", args, inv, err, out)
				}
			}
			if got := run(t, f.repo, f.env, "git", "show-ref"); got != before {
				t.Fatal("install or pass-through changed refs")
			}
			inv, out, err := f.launch(f.repo, true, true, freshArgs(cli)...)
			if err != nil || inv.Cwd == f.repo || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
				t.Fatalf("installed interactive launch: %+v %v\n%s", inv, err, out)
			}
			if got := run(t, inv.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
				t.Fatalf("worktree HEAD %s, want latest main %s", got, f.latest)
			}
			assertLocalConfig(t, filepath.Join(inv.Cwd, ".pyxis", "config.local.yaml"), localConfigFixture, 0o600)
			resumed, out, err := f.launch(f.repo, true, true, resumeArgs(cli)...)
			wantCwd := strings.Contains(resumed.Cwd, ".pyxis-worktrees") && resumed.Cwd != inv.Cwd
			if family(cli) == "cursor" || cli == "opencode" {
				wantCwd = resumed.Cwd == f.repo // These CLIs bind sessions to saved workspaces.
			}
			if err != nil || !wantCwd || !reflect.DeepEqual(resumed.Args, resumedArgs(cli, resumed.Cwd)) {
				t.Fatalf("installed resume: %+v %v\n%s", resumed, err, out)
			}
			assertLocalConfig(t, filepath.Join(resumed.Cwd, ".pyxis", "config.local.yaml"), localConfigFixture, 0o600)
			if got := run(t, f.repo, f.env, "git", "branch", "--show-current"); got != "wip/keep" {
				t.Fatalf("original branch changed to %s", got)
			}
			if data, err := os.ReadFile(filepath.Join(f.repo, "src", "file.txt")); err != nil || string(data) != "user changes\n" {
				t.Fatal("original checkout changes lost")
			}
			if cli == "cursor" {
				for _, args := range [][]string{nil, {"."}, {"--new-window", "src"}} {
					editor, out, err := f.launch(f.repo, true, true, args...)
					want := []string{editor.Cwd}
					if len(args) == 2 {
						want = append([]string{"--new-window"}, want...)
					}
					if err != nil || editor.Command != "cursor" || !strings.Contains(editor.Cwd, ".pyxis-worktrees") || !reflect.DeepEqual(editor.Args, want) {
						t.Fatalf("installed IDE launch: %+v %v\n%s", editor, err, out)
					}
					if got := run(t, editor.Cwd, f.env, "git", "rev-parse", "HEAD"); got != f.latest {
						t.Fatalf("IDE worktree HEAD %s, want latest main %s", got, f.latest)
					}
					refs := run(t, f.repo, f.env, "git", "show-ref")
					reused, out, err := f.launch(editor.Cwd, true, true)
					if err != nil || !reflect.DeepEqual(reused.Args, []string{editor.Cwd}) || refs != run(t, f.repo, f.env, "git", "show-ref") {
						t.Fatalf("installed IDE worktree reuse: %+v %v\n%s", reused, err, out)
					}
				}
			}
		})
	}
	for _, failure := range []string{"checksum", "shell setup"} {
		t.Run(failure, func(t *testing.T) {
			home := t.TempDir()
			rc := filepath.Join(home, ".zshrc")
			content := "# keep my settings\n"
			wantSHA := checksum
			if failure == "checksum" {
				wantSHA = strings.Repeat("0", 64)
			} else {
				content += "# >>> pyxis interactive worktrees >>>\n"
			}
			put(t, rc, content)
			cmd := exec.CommandContext(t.Context(), "sh", installer)
			cmd.Dir = home
			cmd.Env = append(os.Environ(), "HOME="+home, "ZDOTDIR="+home,
				"XDG_DATA_HOME="+filepath.Join(home, "data"), "XDG_CONFIG_HOME="+filepath.Join(home, "config"),
				"PYXIS_INSTALL_DIR="+filepath.Join(home, "bin"), "PYXIS_ARTIFACT_SOURCE=file://"+source,
				"PYXIS_SHA256="+wantSHA, "PYXIS_VERSION=", "PYXIS_CHANNEL=")
			out, err := cmd.CombinedOutput()
			diagnostic := "checksum mismatch"
			if failure == "shell setup" {
				diagnostic = "zsh setup failed"
			}
			if err == nil || !strings.Contains(string(out), diagnostic) {
				t.Fatalf("installer refusal: %v\n%s", err, out)
			}
			data, err := os.ReadFile(rc)
			if err != nil || string(data) != content {
				t.Fatalf("failed install changed rc: %q %v", data, err)
			}
			_, binaryErr := os.Stat(filepath.Join(home, "bin", "pyxis"))
			if failure == "checksum" && !os.IsNotExist(binaryErr) {
				t.Fatal("unverified binary installed")
			}
			if failure == "shell setup" && (binaryErr != nil || !strings.Contains(string(out), "shell install")) {
				t.Fatal("setup failure did not identify the installed CLI and recovery command")
			}
			if _, err := os.Stat(filepath.Join(home, "data")); !os.IsNotExist(err) {
				t.Fatalf("failed install wrote startup assets: %v", err)
			}
		})
	}
}

func headlessArgs(cli string) []string {
	switch cli {
	case "codex":
		return []string{"exec", "test prompt"}
	case "opencode":
		return []string{"run", "test prompt"}
	case "cursor":
		return []string{"agent", "-p", "test prompt"}
	default:
		return []string{"-p", "test prompt"}
	}
}

func buildInstallerRelease(t *testing.T) (installer, source, checksum string) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("machine installer supports macOS and Linux")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	installer = filepath.Join(root, "install.sh")
	data, err := os.ReadFile(installer)
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^PINNED_VERSION="([^"]+)"$`).FindSubmatch(data)
	if len(match) != 2 {
		t.Fatal("installer pin is missing")
	}
	source = t.TempDir()
	binary := filepath.Join(source, fmt.Sprintf("pyxis-%s-%s-%s", match[1], runtime.GOOS, runtime.GOARCH))
	data, err = os.ReadFile(pyxisBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, data, 0o700); err != nil {
		t.Fatal(err)
	}
	return installer, source, fmt.Sprintf("%x", sha256.Sum256(data))
}
