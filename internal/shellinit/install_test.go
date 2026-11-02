package shellinit

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{Home: t.TempDir(), Assets: fstest.MapFS{
		"startup.zsh":        {Data: []byte("# loader\n")},
		"worktree.zsh":       {Data: []byte("# shared\n")},
		"README.md":          {Data: []byte("not executable\n")},
		"testdata/probe.zsh": {Data: []byte("not an installed asset\n")},
	}}
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		t.Fatal(err)
	}
	// Model an existing user file with an exact mode, independent of umask.
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestInstallPreservesConfigurationAndUpdatesOnlyItsBlock(t *testing.T) {
	opts := testOptions(t)
	opts.ZDotDir = filepath.Join(opts.Home, "dot files")
	opts.DataHome = filepath.Join(opts.Home, "data 'quoted' $literal")
	rc := filepath.Join(opts.ZDotDir, ".zshrc")
	prefix := []byte("# user settings\r\nalias claude='claude --effort high'\r\n")
	write(t, rc, prefix)
	rep, err := Install(t.Context(), opts)
	if err != nil || !rep.Changed || rep.RCPath != rc {
		t.Fatalf("install: %+v %v", rep, err)
	}
	first := read(t, rc)
	if !bytes.HasPrefix(first, prefix) || strings.Count(string(first), begin) != 1 {
		t.Fatalf("existing config changed or duplicate block: %q", first)
	}
	if !strings.Contains(string(first), "'\"'\"'") || !strings.Contains(rep.StartupPath, opts.DataHome) {
		t.Fatal("custom data path was not preserved and shell quoted")
	}
	info, err := os.Stat(rc)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("rc mode changed: %v %v", info, err)
	}
	if got := read(t, rep.StartupPath); string(got) != "# loader\n" {
		t.Fatalf("wrong installed bytes: %q", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(rep.StartupPath), "README.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("non-shell assets were installed")
	}
	again, err := Install(t.Context(), opts)
	if err != nil || again.Changed || again.StartupPath != rep.StartupPath || !bytes.Equal(first, read(t, rc)) {
		t.Fatalf("reinstall was not idempotent: %+v %v", again, err)
	}
	after, err := os.Stat(rc)
	if err != nil || !after.ModTime().Equal(info.ModTime()) {
		t.Fatal("unchanged rc was rewritten")
	}
	suffix := []byte("# later user settings\nexport KEEP=yes\n")
	write(t, rc, append(first, suffix...))
	opts.Assets.(fstest.MapFS)["startup.zsh"].Data = []byte("# upgraded loader\n")
	upgraded, err := Install(t.Context(), opts)
	if err != nil || !upgraded.Changed || upgraded.StartupPath == rep.StartupPath {
		t.Fatalf("upgrade: %+v %v", upgraded, err)
	}
	updated := read(t, rc)
	if !bytes.HasPrefix(updated, prefix) || !bytes.HasSuffix(updated, suffix) || bytes.Contains(updated, []byte(rep.StartupPath)) || strings.Count(string(updated), begin) != 1 {
		t.Fatal("upgrade modified user content or retained the old block")
	}
	if string(read(t, rep.StartupPath)) != "# loader\n" {
		t.Fatal("upgrade overwrote the prior complete asset set")
	}
}

func TestInstallFollowsRcSymlinkAndCreatesMissingRc(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "symlink"}[existing], func(t *testing.T) {
			opts := testOptions(t)
			rc := filepath.Join(opts.Home, ".zshrc")
			if existing {
				target := filepath.Join(opts.Home, "dotfiles", "zshrc")
				write(t, target, []byte("# no final newline"))
				if err := os.Symlink(target, rc); err != nil {
					t.Fatal(err)
				}
			}
			rep, err := Install(t.Context(), opts)
			if err != nil || !rep.Changed {
				t.Fatalf("install: %+v %v", rep, err)
			}
			info, err := os.Lstat(rc)
			if err != nil || existing != (info.Mode()&os.ModeSymlink != 0) {
				t.Fatalf("rc symlink changed: %v %v", info, err)
			}
			if existing && !strings.HasPrefix(string(read(t, rc)), "# no final newline\n"+begin) {
				t.Fatal("existing final line was not preserved")
			}
			if !strings.HasPrefix(rep.StartupPath, filepath.Join(opts.Home, ".local", "share")) {
				t.Fatal("default data home was not used")
			}
		})
	}
}

func TestInstallRefusesBeforeChangingMalformedRc(t *testing.T) {
	for _, content := range []string{begin, end, begin + "\n" + begin + "\n" + end, begin + "\n" + end + "\n" + end, begin + "\n" + end + "\n" + begin} {
		t.Run(content, func(t *testing.T) {
			opts := testOptions(t)
			rc := filepath.Join(opts.Home, ".zshrc")
			write(t, rc, []byte(content))
			if _, err := Install(t.Context(), opts); err == nil {
				t.Fatal("malformed block accepted")
			}
			if string(read(t, rc)) != content {
				t.Fatal("malformed rc was modified")
			}
			if _, err := os.Stat(filepath.Join(opts.Home, ".local")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatal("assets written before rc validation")
			}
		})
	}
}

func TestInstallRefusesInvalidLocationsAndAssets(t *testing.T) {
	cases := map[string]func(*testing.T, *Options){
		"no home":   func(_ *testing.T, o *Options) { o.Home = "" },
		"no assets": func(_ *testing.T, o *Options) { o.Assets = nil },
		"no loader": func(_ *testing.T, o *Options) { o.Assets = fstest.MapFS{} },
		"rc is directory": func(t *testing.T, o *Options) {
			if err := os.Mkdir(filepath.Join(o.Home, ".zshrc"), 0o700); err != nil {
				t.Fatal(err)
			}
		},
		"dangling rc symlink": func(t *testing.T, o *Options) {
			if err := os.Symlink("missing", filepath.Join(o.Home, ".zshrc")); err != nil {
				t.Fatal(err)
			}
		},
		"data parent is file": func(t *testing.T, o *Options) {
			o.DataHome = filepath.Join(o.Home, "data")
			write(t, o.DataHome, []byte("keep"))
		},
		"rc parent is file": func(t *testing.T, o *Options) {
			o.ZDotDir = filepath.Join(o.Home, "config")
			write(t, o.ZDotDir, []byte("keep"))
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			opts := testOptions(t)
			prepare(t, &opts)
			if _, err := Install(t.Context(), opts); err == nil {
				t.Fatal("invalid installation accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Install(ctx, testOptions(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled setup: %v", err)
	}
}

func TestInstallPreservesRcWhenExistingAssetsAreDamaged(t *testing.T) {
	for _, damage := range []string{"edited", "missing", "symlink", "directory", "asset root"} {
		t.Run(damage, func(t *testing.T) {
			opts := testOptions(t)
			rep, err := Install(t.Context(), opts)
			if err != nil {
				t.Fatal(err)
			}
			before := read(t, rep.RCPath)
			if err := os.Remove(rep.StartupPath); err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "edited":
				write(t, rep.StartupPath, []byte("operator edit"))
			case "symlink":
				if err := os.Symlink("worktree.zsh", rep.StartupPath); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(rep.StartupPath, 0o700); err != nil {
					t.Fatal(err)
				}
			case "asset root":
				dir := filepath.Dir(rep.StartupPath)
				if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
				write(t, dir, []byte("keep"))
			}
			if _, err := Install(t.Context(), opts); err == nil {
				t.Fatal("damaged assets accepted")
			}
			if !bytes.Equal(before, read(t, rep.RCPath)) {
				t.Fatal("rc changed after asset failure")
			}
		})
	}
}

func TestRcUpdateDetectsInterveningEditAndCancellation(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	write(t, rc, []byte("new edit"))
	if err := writeRC(t.Context(), rc, []byte("old edit"), []byte("replacement"), 0o600); err == nil {
		t.Fatal("concurrent edit overwritten")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := writeRC(ctx, rc, []byte("new edit"), []byte("replacement"), 0o600); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled write: %v", err)
	}
	if string(read(t, rc)) != "new edit" {
		t.Fatal("existing rc changed")
	}
}

func TestFilesystemFailuresLeaveNoPartialRc(t *testing.T) {
	base := t.TempDir()
	if _, _, err := readAssets(os.DirFS(filepath.Join(base, "missing"))); err == nil {
		t.Fatal("missing packaged assets accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assetDir := filepath.Join(base, "assets", "version")
	if err := installAssets(ctx, assetDir, []asset{{"startup.zsh", []byte("source")}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled asset copy: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Dir(assetDir)); err != nil || len(entries) != 0 {
		t.Fatalf("partial assets left behind: %v %v", entries, err)
	}
	file := filepath.Join(base, "file")
	write(t, file, []byte("keep"))
	if err := writeRC(t.Context(), filepath.Join(file, ".zshrc"), nil, []byte("new"), 0o600); err == nil {
		t.Fatal("non-directory rc parent accepted")
	}
	dir := filepath.Join(base, "directory")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeRC(t.Context(), dir, nil, []byte("new"), 0o600); err == nil {
		t.Fatal("directory replaced by rc")
	}
}

func TestReadOnlyRcDirectoryPreservesConfiguration(t *testing.T) {
	opts := testOptions(t)
	opts.ZDotDir = filepath.Join(opts.Home, "readonly")
	rc := filepath.Join(opts.ZDotDir, ".zshrc")
	write(t, rc, []byte("# keep\n"))
	if err := os.Chmod(opts.ZDotDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(opts.ZDotDir, 0o700); err != nil {
			t.Error(err)
		}
	})
	probe, err := os.CreateTemp(opts.ZDotDir, "permission-probe-")
	if err == nil {
		if err := probe.Close(); err != nil {
			t.Fatal(err)
		}
		t.Skip("this account can write through directory permissions")
	}
	if _, err := Install(t.Context(), opts); err == nil {
		t.Fatal("read-only rc directory accepted")
	}
	if string(read(t, rc)) != "# keep\n" {
		t.Fatal("existing configuration changed")
	}
}
