package runlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenKeepsFeaturesSeparate(t *testing.T) {
	for _, tc := range []struct{ first, second string }{
		{"foo-bar", "bar"},
		{"foo.bar", "foo-bar"},
		{"foo_bar", "foo-bar"},
		{"Foo-bar", "foo-bar"},
		{"foo-Bar", "foo-bar"},
		{"foo/bar", "foo%2Fbar"},
		{"", "unscoped"},
	} {
		t.Run(tc.first+" and "+tc.second, func(t *testing.T) {
			repo := t.TempDir()
			now := time.Date(2026, 9, 9, 1, 2, 0, 0, time.UTC)
			var paths []string
			for _, feature := range []string{tc.first, tc.second} {
				f, path, err := Open(repo, "logs", feature, now)
				if err != nil {
					t.Fatal(err)
				}
				if err := WriteLine(f, now, "feature %q", feature); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, path)
			}
			first, err := os.Stat(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			second, err := os.Stat(paths[1])
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(first, second) {
				t.Fatalf("features %q and %q share a log: %q / %q", tc.first, tc.second, paths[0], paths[1])
			}
			for _, path := range paths {
				data, err := os.ReadFile(path)
				if err != nil || strings.Count(string(data), "\n") != 1 {
					t.Fatalf("log %q = %q, %v; want only its own feature's line", path, data, err)
				}
			}
		})
	}
}

func TestOpenMatchesOnlyDatedFeatureFiles(t *testing.T) {
	repo := t.TempDir()
	directory := filepath.Join(repo, "logs[1]")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"00-not-a-date-bar.log", "01-01-26-foo-bar.log", "02-30-26-bar.log", "9-9-26-bar.log"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("unrelated\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(directory, "01-01-26-bar.log"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 9, 1, 2, 0, 0, time.UTC)
	for _, date := range []time.Time{now, now.AddDate(0, 0, 1)} {
		f, path, err := Open(repo, directory, "bar", date)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(directory, "09-09-26-bar.log"); path != want {
			t.Fatalf("log path = %q, want %q", path, want)
		}
	}
}
