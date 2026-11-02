// Package shellinit installs machine-level zsh startup without a source checkout.
package shellinit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	begin = "# >>> pyxis interactive worktrees >>>"
	end   = "# <<< pyxis interactive worktrees <<<"
)

// Options names the user's shell locations; empty overrides use Home.
type Options struct {
	Home     string
	ZDotDir  string
	DataHome string
	Assets   fs.FS
}

// Report identifies the installed source and the rc file that loads it.
type Report struct {
	RCPath      string
	StartupPath string
	Changed     bool
}

type asset struct {
	name string
	data []byte
}

// Install prepares a complete, content-addressed asset directory before
// atomically updating the managed rc block. A failed upgrade leaves the old
// block and its assets usable. Reinstalling identical bytes does no writes.
func Install(ctx context.Context, opts Options) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	if opts.Home == "" || opts.Assets == nil {
		return Report{}, errors.New("shell setup requires a home directory and startup assets")
	}
	if opts.ZDotDir == "" {
		opts.ZDotDir = opts.Home
	}
	if opts.DataHome == "" {
		opts.DataHome = filepath.Join(opts.Home, ".local", "share")
	}
	rcPath, err := filepath.Abs(filepath.Join(opts.ZDotDir, ".zshrc"))
	if err != nil {
		return Report{}, fmt.Errorf("resolve zshrc: %w", err)
	}
	dataPath, err := filepath.Abs(filepath.Join(opts.DataHome, "pyxis", "startup"))
	if err != nil {
		return Report{}, fmt.Errorf("resolve startup directory: %w", err)
	}
	files, digest, err := readAssets(opts.Assets)
	if err != nil {
		return Report{}, err
	}
	assetDir := filepath.Join(dataPath, digest)
	rep := Report{RCPath: rcPath, StartupPath: filepath.Join(assetDir, "startup.zsh")}
	target, old, mode, err := readRC(rcPath)
	if err != nil {
		return rep, err
	}
	updated, err := updateBlock(old, rep.StartupPath)
	if err != nil {
		return rep, fmt.Errorf("update %s: %w", rcPath, err)
	}
	if err := installAssets(ctx, assetDir, files); err != nil {
		return rep, fmt.Errorf("install startup assets: %w", err)
	}
	if bytes.Equal(old, updated) {
		return rep, nil
	}
	if err := writeRC(ctx, target, old, updated, mode); err != nil {
		return rep, fmt.Errorf("write %s: %w", rcPath, err)
	}
	rep.Changed = true
	return rep, nil
}

func readAssets(source fs.FS) ([]asset, string, error) {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, "", fmt.Errorf("read startup assets: %w", err)
	}
	var files []asset
	var content []byte
	loader := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zsh") {
			continue
		}
		data, err := fs.ReadFile(source, entry.Name())
		if err != nil {
			return nil, "", fmt.Errorf("read startup asset %s: %w", entry.Name(), err)
		}
		files = append(files, asset{entry.Name(), data})
		content = append(content, entry.Name()...)
		content = append(content, 0)
		content = append(content, data...)
		content = append(content, 0)
		loader = loader || entry.Name() == "startup.zsh"
	}
	if !loader {
		return nil, "", errors.New("startup assets are missing startup.zsh")
	}
	return files, fmt.Sprintf("%x", sha256.Sum256(content)), nil
}

func installAssets(ctx context.Context, dir string, files []asset) error {
	if _, err := os.Lstat(dir); err == nil {
		return verifyAssets(dir, files)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), ".install-")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmp) // Best-effort cleanup of unpublished temporary assets.
	}()
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(tmp, file.name), file.data, 0o600); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another installer may have published the identical directory first.
		if check := verifyAssets(dir, files); check != nil {
			return fmt.Errorf("publish startup assets: %w", err)
		}
	}
	return nil
}

func verifyAssets(dir string, files []asset) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("startup asset path %s is not a directory", dir)
	}
	for _, file := range files {
		path := filepath.Join(dir, file.name)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("startup asset %s is not a regular file", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, file.data) {
			return fmt.Errorf("startup asset %s was modified; move that asset directory aside and retry", path)
		}
	}
	return nil
}

// readRC follows a dotfile-manager symlink without replacing the link itself.
func readRC(path string) (string, []byte, fs.FileMode, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return path, nil, 0o600, nil
	}
	if err != nil {
		return "", nil, 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return "", nil, 0, fmt.Errorf("resolve zshrc symlink: %w", err)
		}
		info, err = os.Stat(path)
		if err != nil {
			return "", nil, 0, err
		}
	}
	if !info.Mode().IsRegular() {
		return "", nil, 0, errors.New("zshrc is not a regular file")
	}
	data, err := os.ReadFile(path)
	return path, data, info.Mode().Perm(), err
}

func updateBlock(data []byte, startup string) ([]byte, error) {
	quoted := "'" + strings.ReplaceAll(startup, "'", "'\"'\"'") + "'"
	block := begin + "\nif [ -r " + quoted + " ]; then\n  source " + quoted + "\nfi\n" + end + "\n"
	start, stop, offset := -1, -1, 0
	for _, line := range strings.SplitAfter(string(data), "\n") {
		switch strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r") {
		case begin:
			if start != -1 || stop != -1 {
				return nil, errors.New("duplicate pyxis startup block; keep one complete block and retry")
			}
			start = offset
		case end:
			if start == -1 || stop != -1 {
				return nil, errors.New("unmatched pyxis startup marker; restore a complete block and retry")
			}
			stop = offset + len(line)
		}
		offset += len(line)
	}
	if start == -1 {
		prefix := string(data)
		if len(prefix) > 0 && !strings.HasSuffix(prefix, "\n") {
			prefix += "\n"
		}
		return []byte(prefix + block), nil
	}
	if stop == -1 {
		return nil, errors.New("unfinished pyxis startup block; restore its end marker and retry")
	}
	return []byte(string(data[:start]) + block + string(data[stop:])), nil
}

func writeRC(ctx context.Context, path string, old, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pyxis-zshrc-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() {
		_ = os.Remove(name) // Best-effort cleanup when the atomic update fails.
	}()
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.WriteFile(name, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if !bytes.Equal(current, old) {
		return errors.New("zshrc changed during setup; retry to preserve the latest edits")
	}
	return os.Rename(name, path)
}
