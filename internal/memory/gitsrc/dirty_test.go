package gitsrc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawDirtyChecksBytesModesLinksAndIndex(t *testing.T) {
	for _, kind := range []string{"bytes", "mode", "deleted", "link over file", "staged", "link", "changed link", "file over link", "gitlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := newRepo(t)
			r, err := Open(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(dir, "README.md")
			if strings.Contains(kind, "link") && kind != "link over file" && kind != "gitlink" {
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("absent", p); err != nil {
					t.Fatal(err)
				}
				git(t, dir, "add", "README.md")
				git(t, dir, "commit", "-qm", "symbolic link")
			}
			switch kind {
			case "bytes":
				write(t, dir, "README.md", "changed")
			case "mode":
				if err := os.Chmod(p, 0o755); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			case "link over file":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("absent", p); err != nil {
					t.Fatal(err)
				}
			case "staged":
				write(t, dir, "README.md", "changed")
				git(t, dir, "add", "README.md")
			case "changed link":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("other", p); err != nil {
					t.Fatal(err)
				}
			case "file over link":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				write(t, dir, "README.md", "regular")
			case "gitlink":
				git(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+git(t, dir, "rev-parse", "HEAD")+",module")
				git(t, dir, "commit", "-qm", "gitlink")
			}
			dirty, err := r.Dirty(t.Context(), ".")
			if kind == "gitlink" {
				if err == nil {
					t.Fatal("unverifiable gitlink passed")
				}
				return
			}
			if err != nil || dirty != (kind != "link") {
				t.Fatalf("raw cleanliness: %v %v", dirty, err)
			}
		})
	}
}

func TestRawDirtyReadFailuresAreNotClean(t *testing.T) {
	dir := newRepo(t)
	for _, command := range []string{"diff-index", "ls-files", "ls-tree", "cat-file"} {
		t.Run(command, func(t *testing.T) {
			r, err := Open(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			real := r.run
			failure := errors.New("read denied")
			r.run = func(ctx context.Context, in []byte, args ...string) ([]byte, error) {
				if args[0] == command {
					return nil, failure
				}
				return real(ctx, in, args...)
			}
			if _, err := r.Dirty(t.Context(), "memory"); !errors.Is(err, failure) {
				t.Fatalf("lost failure: %v", err)
			}
		})
	}
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	real := r.run
	r.run = func(ctx context.Context, in []byte, args ...string) ([]byte, error) {
		if args[0] == "cat-file" {
			return []byte(strings.ReplaceAll(string(in), "\n", " missing\n")), nil
		}
		return real(ctx, in, args...)
	}
	if _, err := r.Dirty(t.Context(), "README.md"); err == nil {
		t.Fatal("missing blob treated as clean")
	}
	r.run = real
	r.Root = filepath.Join(dir, "absent")
	if _, err := r.Dirty(t.Context(), "."); err == nil {
		t.Fatal("absent working root treated as clean")
	}
}

func TestRawDirtyPathUnion(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "internal/a/file.go", "package a\n")
	write(t, dir, "internal/b/file.go", "package b\n")
	git(t, dir, "add", "internal")
	git(t, dir, "commit", "-qm", "two scopes")
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "internal/b/file.go", "package b\n// dirty\n")
	for _, tc := range []struct {
		paths []string
		dirty bool
	}{
		{nil, false},
		{[]string{"README.md", "internal/a"}, false},
		{[]string{"README.md", "internal/b"}, true},
	} {
		dirty, err := r.DirtyPaths(t.Context(), tc.paths)
		if err != nil || dirty != tc.dirty {
			t.Fatalf("%v: %v %v", tc.paths, dirty, err)
		}
	}
}
