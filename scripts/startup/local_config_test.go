package startup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const localConfigFixture = "# machine-local settings\nexecutor:\n  routing:\n    default: claude\n"
const vscodeSettingsFixture = "{\n  \"editor.tabSize\": 2\n}\n"
const vscodeExtensionsFixture = "{\n  \"recommendations\": [\"golang.go\"]\n}\n"

func TestHarnessWorktreesPreserveOptionalLocalConfig(t *testing.T) {
	for _, cli := range append([]string{"codex"}, harnesses...) {
		for _, state := range []string{"absent", "present", "empty"} {
			t.Run(cli+"/"+state, func(t *testing.T) {
				f := setup(t, cli)
				source := filepath.Join(f.repo, ".pyxis", "config.local.yaml")
				content := localConfigFixture
				if state == "empty" {
					content = ""
				}
				if state != "absent" {
					put(t, source, content)
					if err := os.Chmod(source, 0o640); err != nil {
						t.Fatal(err)
					}
				}
				before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
				inv, out, err := f.launch(filepath.Join(f.repo, "src"), true, true, freshArgs(cli)...)
				if err != nil || inv.Command != cli || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
					t.Fatalf("launch: %+v %v\n%s", inv, err, out)
				}
				root := run(t, inv.Cwd, f.env, "git", "rev-parse", "--show-toplevel")
				destination := filepath.Join(root, ".pyxis", "config.local.yaml")
				if _, err := os.Lstat(filepath.Join(root, "untracked.txt")); !os.IsNotExist(err) {
					t.Fatalf("unrelated untracked file was copied: %v", err)
				}
				if state == "absent" {
					if _, err := os.Lstat(destination); !os.IsNotExist(err) {
						t.Fatalf("absent local config was created: %v", err)
					}
				} else {
					assertLocalConfig(t, destination, content, 0o640)
					if got := run(t, root, f.env, "git", "status", "--porcelain=v1"); got != "" {
						t.Fatalf("local config dirtied new worktree: %s", got)
					}
					put(t, destination, "# worktree-specific settings\n")
					assertLocalConfig(t, source, content, 0o640)
					put(t, source, "# changed original settings\n")
					second, out, err := f.launch(inv.Cwd, true, true, freshArgs(cli)...)
					if err != nil || second.Cwd != inv.Cwd {
						t.Fatalf("reused worktree: %+v %v\n%s", second, err, out)
					}
					assertLocalConfig(t, destination, "# worktree-specific settings\n", 0o640)
				}
				assertOriginal(t, f, before)
			})
		}
	}
}

func TestHarnessWorktreesPreserveOptionalVSCode(t *testing.T) {
	for _, cli := range append([]string{"codex"}, harnesses...) {
		for _, state := range []string{"absent", "present", "empty"} {
			t.Run(cli+"/"+state, func(t *testing.T) {
				f := setup(t, cli)
				source := filepath.Join(f.repo, ".vscode")
				if state != "absent" {
					if err := os.Mkdir(source, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(source, 0o700); err != nil {
						t.Fatal(err)
					}
					if state == "present" {
						putVSCode(t, source, 0o640)
					}
				}
				before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
				inv, out, err := f.launch(filepath.Join(f.repo, "src"), true, true, freshArgs(cli)...)
				if err != nil || inv.Command != cli || !strings.Contains(inv.Cwd, ".pyxis-worktrees") {
					t.Fatalf("launch: %+v %v\n%s", inv, err, out)
				}
				root := run(t, inv.Cwd, f.env, "git", "rev-parse", "--show-toplevel")
				destination := filepath.Join(root, ".vscode")
				if _, err := os.Lstat(filepath.Join(root, "untracked.txt")); !os.IsNotExist(err) {
					t.Fatalf("unrelated untracked file was copied: %v", err)
				}
				if state == "absent" {
					if _, err := os.Lstat(destination); !os.IsNotExist(err) {
						t.Fatalf("absent vscode directory was created: %v", err)
					}
				} else {
					if state == "empty" {
						assertEmptyVSCode(t, destination, 0o700)
					} else {
						assertVSCode(t, destination, 0o640)
					}
					if got := run(t, root, f.env, "git", "status", "--porcelain=v1"); got != "" {
						t.Fatalf("vscode dirtied new worktree: %s", got)
					}
					if state == "present" {
						put(t, filepath.Join(destination, "settings.json"), "{\n  \"worktree\": true\n}\n")
						assertVSCode(t, source, 0o640)
						put(t, filepath.Join(source, "settings.json"), "{\n  \"original\": true\n}\n")
						second, out, err := f.launch(inv.Cwd, true, true, freshArgs(cli)...)
						if err != nil || second.Cwd != inv.Cwd {
							t.Fatalf("reused worktree: %+v %v\n%s", second, err, out)
						}
						assertLocalConfig(t, filepath.Join(destination, "settings.json"), "{\n  \"worktree\": true\n}\n", 0o640)
					}
				}
				assertOriginal(t, f, before)
			})
		}
	}
}

func TestWorktreesPreserveLocalConfigAndVSCodeTogether(t *testing.T) {
	f := setup(t)
	put(t, filepath.Join(f.repo, ".pyxis", "config.local.yaml"), localConfigFixture)
	if err := os.Mkdir(filepath.Join(f.repo, ".vscode"), 0o700); err != nil {
		t.Fatal(err)
	}
	putVSCode(t, filepath.Join(f.repo, ".vscode"), 0o600)
	inv, out, err := f.launch(f.repo, true, true)
	if err != nil || inv.Command != "codex" || inv.Cwd == f.repo {
		t.Fatalf("launch: %+v %v\n%s", inv, err, out)
	}
	assertLocalConfig(t, filepath.Join(inv.Cwd, ".pyxis", "config.local.yaml"), localConfigFixture, 0o600)
	assertVSCode(t, filepath.Join(inv.Cwd, ".vscode"), 0o600)
}

func TestVSCodeSourceSymlinkCopiesContents(t *testing.T) {
	f := setup(t)
	shared := filepath.Join(f.base, "shared-vscode")
	if err := os.Mkdir(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	putVSCode(t, shared, 0o600)
	if err := os.Symlink(shared, filepath.Join(f.repo, ".vscode")); err != nil {
		t.Fatal(err)
	}
	inv, out, err := f.launch(f.repo, true, true)
	if err != nil || inv.Command != "codex" || inv.Cwd == f.repo {
		t.Fatalf("launch: %+v %v\n%s", inv, err, out)
	}
	destination := filepath.Join(inv.Cwd, ".vscode")
	info, err := os.Lstat(destination)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copied vscode should be a real directory: mode=%v error=%v", info.Mode(), err)
	}
	assertVSCode(t, destination, 0o600)
	put(t, filepath.Join(destination, "settings.json"), "{\n  \"worktree\": true\n}\n")
	assertVSCode(t, shared, 0o600)
}

func TestVSCodeCopyDoesNotFollowInnerSymlinks(t *testing.T) {
	f := setup(t)
	source := filepath.Join(f.repo, ".vscode")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	putVSCode(t, source, 0o600)
	outside := filepath.Join(f.base, "outside-secret.txt")
	put(t, outside, "keep outside secret\n")
	if err := os.Symlink(outside, filepath.Join(source, "secret.link")); err != nil {
		t.Fatal(err)
	}
	inv, out, err := f.launch(f.repo, true, true)
	if err != nil || inv.Command != "codex" || inv.Cwd == f.repo {
		t.Fatalf("launch: %+v %v\n%s", inv, err, out)
	}
	copied := filepath.Join(inv.Cwd, ".vscode", "secret.link")
	info, err := os.Lstat(copied)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("inner symlink was followed: mode=%v error=%v", info.Mode(), err)
	}
	target, err := os.Readlink(copied)
	if err != nil || target != outside {
		t.Fatalf("inner symlink target=%q error=%v", target, err)
	}
	assertLocalConfig(t, outside, "keep outside secret\n", 0o600)
}

func TestVSCodeCopyFailureStopsHarness(t *testing.T) {
	f := setup(t)
	if err := os.Mkdir(filepath.Join(f.repo, ".vscode"), 0o700); err != nil {
		t.Fatal(err)
	}
	putVSCode(t, filepath.Join(f.repo, ".vscode"), 0o600)
	preserved := filepath.Join(f.base, "existing-worktree")
	run(t, f.repo, f.env, "git", "worktree", "add", "-b", "chore/keep", preserved, "HEAD")
	put(t, filepath.Join(preserved, "untracked.txt"), "keep existing worktree\n")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, filepath.Join(f.base, "bin", "cp")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		assertVSCodeLaunchRejected(t, f)
	}
	if data, err := os.ReadFile(filepath.Join(preserved, "untracked.txt")); err != nil || string(data) != "keep existing worktree\n" {
		t.Fatalf("existing worktree changed: %q %v", data, err)
	}
}

func TestVSCodeRejectsRepositoryControlledDestinations(t *testing.T) {
	for _, tc := range []struct {
		name, kind, target string
		external           bool
	}{
		{"vscode symlink outside", "symlink", "outside", true},
		{"vscode dangling symlink", "symlink", "missing", true},
		{"vscode symlink tracked directory", "symlink", "src", false},
		{"vscode tracked file", "file", "", false},
		{"vscode tracked directory", "directory", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			if err := os.Mkdir(filepath.Join(f.repo, ".vscode"), 0o700); err != nil {
				t.Fatal(err)
			}
			putVSCode(t, filepath.Join(f.repo, ".vscode"), 0o600)
			outside := filepath.Join(f.base, "outside", "settings.json")
			if err := os.Mkdir(filepath.Dir(outside), 0o700); err != nil {
				t.Fatal(err)
			}
			put(t, outside, "preserve outside file\n")
			path := filepath.Join(f.seed, ".vscode")
			switch tc.kind {
			case "symlink":
				target := tc.target
				if tc.external {
					target = filepath.Join(f.base, target)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(path, "README"), "tracked directory\n")
			case "file":
				put(t, path, "tracked file\n")
			}
			run(t, f.seed, f.env, "git", "add", "-f", "--", ".vscode")
			run(t, f.seed, f.env, "git", "commit", "-m", "Add repository-controlled vscode destination")
			run(t, f.seed, f.env, "git", "push", "origin", "main")
			assertVSCodeLaunchRejected(t, f)
			assertLocalConfig(t, outside, "preserve outside file\n", 0o600)
			if _, err := os.Lstat(filepath.Join(f.base, "missing")); !os.IsNotExist(err) {
				t.Fatalf("created dangling symlink target: %v", err)
			}
		})
	}
}

func TestLocalConfigCreatesMissingWorktreeDirectory(t *testing.T) {
	f := setup(t)
	put(t, filepath.Join(f.repo, ".pyxis", "config.local.yaml"), localConfigFixture)
	run(t, f.seed, f.env, "git", "rm", ".pyxis/README")
	run(t, f.seed, f.env, "git", "commit", "-m", "Keep Pyxis directory local")
	run(t, f.seed, f.env, "git", "push", "origin", "main")
	inv, out, err := f.launch(f.repo, true, true)
	if err != nil || inv.Command != "codex" || inv.Cwd == f.repo {
		t.Fatalf("launch without tracked .pyxis: %+v %v\n%s", inv, err, out)
	}
	assertLocalConfig(t, filepath.Join(inv.Cwd, ".pyxis", "config.local.yaml"), localConfigFixture, 0o600)
}

func TestLocalConfigCopyFailureStopsHarness(t *testing.T) {
	f := setup(t)
	put(t, filepath.Join(f.repo, ".pyxis", "config.local.yaml"), localConfigFixture)
	preserved := filepath.Join(f.base, "existing-worktree")
	run(t, f.repo, f.env, "git", "worktree", "add", "-b", "chore/keep", preserved, "HEAD")
	put(t, filepath.Join(preserved, "untracked.txt"), "keep existing worktree\n")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, filepath.Join(f.base, "bin", "cp")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		assertLocalConfigLaunchRejected(t, f)
	}
	if data, err := os.ReadFile(filepath.Join(preserved, "untracked.txt")); err != nil || string(data) != "keep existing worktree\n" {
		t.Fatalf("existing worktree changed: %q %v", data, err)
	}
}

func TestLocalConfigRejectsRepositoryControlledDestinations(t *testing.T) {
	for _, tc := range []struct {
		name, path, kind, target string
		external                 bool
	}{
		{"config symlink outside", ".pyxis/config.local.yaml", "symlink", "outside/config.local.yaml", true},
		{"config dangling symlink", ".pyxis/config.local.yaml", "symlink", "missing.yaml", true},
		{"config symlink tracked file", ".pyxis/config.local.yaml", "symlink", "../src/file.txt", false},
		{"pyxis symlink outside", ".pyxis", "symlink", "outside", true},
		{"pyxis dangling symlink", ".pyxis", "symlink", "missing", true},
		{"pyxis symlink tracked directory", ".pyxis", "symlink", "src", false},
		{"config tracked file", ".pyxis/config.local.yaml", "file", "", false},
		{"config tracked directory", ".pyxis/config.local.yaml", "directory", "", false},
		{"pyxis tracked file", ".pyxis", "file", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			put(t, filepath.Join(f.repo, ".pyxis", "config.local.yaml"), localConfigFixture)
			outside := filepath.Join(f.base, "outside", "config.local.yaml")
			if err := os.Mkdir(filepath.Dir(outside), 0o700); err != nil {
				t.Fatal(err)
			}
			put(t, outside, "preserve outside file\n")
			if tc.path == ".pyxis" {
				run(t, f.seed, f.env, "git", "rm", ".pyxis/README")
			}
			path := filepath.Join(f.seed, tc.path)
			switch tc.kind {
			case "symlink":
				target := tc.target
				if tc.external {
					target = filepath.Join(f.base, target)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(path, "README"), "tracked directory\n")
			case "file":
				put(t, path, "tracked file\n")
			}
			run(t, f.seed, f.env, "git", "add", "-f", "--", tc.path)
			run(t, f.seed, f.env, "git", "commit", "-m", "Add repository-controlled config destination")
			run(t, f.seed, f.env, "git", "push", "origin", "main")
			assertLocalConfigLaunchRejected(t, f)
			assertLocalConfig(t, outside, "preserve outside file\n", 0o600)
			for _, missing := range []string{"missing", "missing.yaml"} {
				if _, err := os.Lstat(filepath.Join(f.base, missing)); !os.IsNotExist(err) {
					t.Fatalf("created dangling symlink target %s: %v", missing, err)
				}
			}
		})
	}
}

func assertLocalConfigLaunchRejected(t *testing.T, f *fixture) {
	t.Helper()
	assertCopyLaunchRejected(t, f, "cannot copy .pyxis/config.local.yaml")
	assertLocalConfig(t, filepath.Join(f.repo, ".pyxis", "config.local.yaml"), localConfigFixture, 0o600)
}

func assertVSCodeLaunchRejected(t *testing.T, f *fixture) {
	t.Helper()
	assertCopyLaunchRejected(t, f, "cannot copy .vscode")
	assertVSCode(t, filepath.Join(f.repo, ".vscode"), 0o600)
}

func assertCopyLaunchRejected(t *testing.T, f *fixture, fragment string) {
	t.Helper()
	before := run(t, f.repo, f.env, "git", "status", "--porcelain=v1")
	worktrees := run(t, f.repo, f.env, "git", "worktree", "list", "--porcelain")
	branches := refsHeadsExceptMain(run(t, f.repo, f.env, "git", "for-each-ref", "refs/heads"))
	inv, out, err := f.launch(f.repo, true, true)
	if err == nil || inv.Command != "" || !strings.Contains(out, fragment) {
		t.Fatalf("failed copy launched harness: %+v %v\n%s", inv, err, out)
	}
	assertOriginal(t, f, before)
	if got := run(t, f.repo, f.env, "git", "worktree", "list", "--porcelain"); got != worktrees {
		t.Fatalf("failed launch changed registered worktrees: before=%q after=%q", worktrees, got)
	}
	if got := refsHeadsExceptMain(run(t, f.repo, f.env, "git", "for-each-ref", "refs/heads")); got != branches {
		t.Fatalf("failed launch changed branches: before=%q after=%q", branches, got)
	}
	originMain := run(t, f.repo, f.env, "git", "rev-parse", "refs/remotes/origin/main")
	if got := run(t, f.repo, f.env, "git", "rev-parse", "refs/heads/main"); got != originMain {
		t.Fatalf("failed launch left main at %s, want origin/main %s", got, originMain)
	}
	parent := filepath.Join(f.base, ".pyxis-worktrees", filepath.Base(f.repo))
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatalf("failed launch left worktree directories: %v %v", entries, err)
	}
}

func refsHeadsExceptMain(out string) string {
	var kept []string
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" || strings.HasSuffix(line, "\trefs/heads/main") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func assertLocalConfig(t *testing.T, path, content string, permissions os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != content {
		t.Fatalf("local config %s: contents=%q error=%v", path, data, err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != permissions {
		t.Fatalf("local config %s: mode=%v, want regular file with permissions %v", path, info.Mode(), permissions)
	}
}

func putVSCode(t *testing.T, dir string, permissions os.FileMode) {
	t.Helper()
	put(t, filepath.Join(dir, "settings.json"), vscodeSettingsFixture)
	put(t, filepath.Join(dir, "extensions.json"), vscodeExtensionsFixture)
	for _, name := range []string{"settings.json", "extensions.json"} {
		if err := os.Chmod(filepath.Join(dir, name), permissions); err != nil {
			t.Fatal(err)
		}
	}
}

func assertVSCode(t *testing.T, dir string, permissions os.FileMode) {
	t.Helper()
	assertVSCodeDir(t, dir)
	assertLocalConfig(t, filepath.Join(dir, "settings.json"), vscodeSettingsFixture, permissions)
	assertLocalConfig(t, filepath.Join(dir, "extensions.json"), vscodeExtensionsFixture, permissions)
}

func assertEmptyVSCode(t *testing.T, dir string, permissions os.FileMode) {
	t.Helper()
	info := assertVSCodeDir(t, dir)
	if info.Mode().Perm() != permissions {
		t.Fatalf("vscode %s: mode=%v, want directory with permissions %v", dir, info.Mode(), permissions)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("vscode %s: entries=%v error=%v", dir, entries, err)
	}
}

func assertVSCodeDir(t *testing.T, dir string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("vscode %s: mode=%v, want a real directory", dir, info.Mode())
	}
	return info
}
