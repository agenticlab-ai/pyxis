package gitrefstore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchTransfersOnlyWhenTheLocalRefDiffers(t *testing.T) {
	remote, repo := cloneFixture(t)
	const ref = "refs/pyxis/test-evidence"
	store, err := New(repo, "origin", ref)
	if err != nil {
		t.Fatal(err)
	}
	write := func(s *Store, body string) {
		t.Helper()
		if _, err := s.Update(t.Context(), "log", "write "+body, func([]byte) ([]byte, bool, error) { return []byte(body), true, nil }); err != nil {
			t.Fatal(err)
		}
	}
	fetchHead := filepath.Join(repo, ".git", "FETCH_HEAD")
	fetch := func(want string, transferred bool) {
		t.Helper()
		if err := os.Remove(fetchHead); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		snapshot, err := store.Fetch(t.Context(), "")
		if err != nil || string(snapshot.Files["log"]) != want {
			t.Fatalf("Fetch = %q, %v; want %q", snapshot.Files["log"], err, want)
		}
		if local := strings.TrimSpace(runGit(t, repo, "rev-parse", ref)); local != snapshot.Head {
			t.Fatalf("local ref %s, fetched head %s", local, snapshot.Head)
		}
		if _, err := os.Stat(fetchHead); (err == nil) != transferred {
			t.Fatalf("fetch transferred = %v, want %v", err == nil, transferred)
		}
	}

	write(store, "one")
	fetch("one", false)

	runGit(t, repo, "update-ref", "-d", ref)
	fetch("one", true)

	other := filepath.Join(t.TempDir(), "other")
	runGit(t, filepath.Dir(other), "clone", "-q", remote, other)
	writer, err := New(other, "origin", ref)
	if err != nil {
		t.Fatal(err)
	}
	write(writer, "two")
	fetch("two", true)
	fetch("two", false)
}

func TestFetchBranchPinsCommitWithoutWritingFetchHead(t *testing.T) {
	_, repo := cloneFixture(t)
	branch := strings.TrimSpace(runGit(t, repo, "branch", "--show-current"))
	store, err := New(repo, "origin", "refs/pyxis/published-source")
	if err != nil {
		t.Fatal(err)
	}
	fetchHead := filepath.Join(repo, ".git", "FETCH_HEAD")
	if err := os.Remove(fetchHead); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	head, err := store.FetchBranch(t.Context(), branch)
	if err != nil || head != strings.TrimSpace(runGit(t, repo, "rev-parse", "origin/"+branch)) {
		t.Fatalf("FetchBranch = %q, %v", head, err)
	}
	if _, err := os.Stat(fetchHead); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("FetchBranch wrote FETCH_HEAD: %v", err)
	}
	if _, err := store.FetchBranch(t.Context(), "missing\nbranch"); err == nil {
		t.Fatal("invalid branch fetch succeeded")
	}
}
