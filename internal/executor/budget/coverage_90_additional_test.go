package budget

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCoverage90BudgetFailureAndValidationBoundaries(t *testing.T) {
	t.Run("invalid state root", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "relative-state")
		if _, err := NewForIdentity("host", "identity", nil, nil); err == nil {
			t.Fatal("budget accepted a relative state root")
		}
	})
	if err := realSleep(t.Context(), time.Nanosecond); err != nil {
		t.Fatalf("timer sleep = %v", err)
	}
	if (ledger{}).valid() {
		t.Fatal("ledger with missing windows was valid")
	}
	if (ledger{Minute: []time.Time{{}}, Hour: []time.Time{time.Now()}}).valid() {
		t.Fatal("ledger with a zero timestamp was valid")
	}

	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if _, replace := (&Budget{}).read(filepath.Join(t.TempDir(), "missing"), now); replace {
		t.Fatal("absent ledger requested conservative replacement")
	}
	directory := t.TempDir()
	if got, replace := (&Budget{}).read(directory, now); !replace || len(got.Minute) == 0 {
		t.Fatalf("unreadable ledger = %+v, replace=%v", got, replace)
	}

	closed, err := os.CreateTemp(t.TempDir(), "lock")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lockContext(t.Context(), closed); err == nil {
		t.Fatal("lock accepted a closed descriptor")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := (&Budget{}).attempt(canceled, closed, nil, "", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled attempt = %v", err)
	}

	want := errors.New("persist failed")
	b := &Budget{umbrellaPath: "umbrella", markerPath: "missing-marker", host: "host", writeRecord: func(string, any) error { return want }}
	if err := b.commit(ledger{}, ledger{}, "", now, 1); !errors.Is(err, want) {
		t.Fatalf("umbrella commit failure = %v", err)
	}
	if err := b.ensureBarrier(now, "identity"); !errors.Is(err, want) {
		t.Fatalf("barrier umbrella failure = %v", err)
	}
	calls := 0
	b.writeRecord = func(string, any) error {
		calls++
		if calls == 2 {
			return want
		}
		return nil
	}
	if err := b.ensureBarrier(now, "identity"); !errors.Is(err, want) {
		t.Fatalf("barrier identity failure = %v", err)
	}
}

type coverage90FileInfo struct {
	name string
	mode os.FileMode
	mod  time.Time
	sys  any
}

func (i coverage90FileInfo) Name() string       { return i.name }
func (i coverage90FileInfo) Size() int64        { return 0 }
func (i coverage90FileInfo) Mode() os.FileMode  { return i.mode }
func (i coverage90FileInfo) ModTime() time.Time { return i.mod }
func (i coverage90FileInfo) IsDir() bool        { return i.mode.IsDir() }
func (i coverage90FileInfo) Sys() any           { return i.sys }

type coverage90DirEntry struct {
	name string
	info os.FileInfo
	err  error
}

func (e coverage90DirEntry) Name() string               { return e.name }
func (e coverage90DirEntry) IsDir() bool                { return false }
func (e coverage90DirEntry) Type() os.FileMode          { return 0 }
func (e coverage90DirEntry) Info() (os.FileInfo, error) { return e.info, e.err }

func TestCoverage90DurableFilesystemBoundaries(t *testing.T) {
	want := errors.New("filesystem failed")
	path := filepath.Join(t.TempDir(), "record")
	if err := reapTempsWith(path, time.Now(), func(string) ([]os.DirEntry, error) { return nil, want }, nil, nil); !errors.Is(err, want) {
		t.Fatalf("reaper read failure = %v", err)
	}
	badInfo := coverage90DirEntry{name: tempPrefix + "bad", err: want}
	if err := reapTempsWith(path, time.Now(), func(string) ([]os.DirEntry, error) { return []os.DirEntry{badInfo}, nil }, func(string) error { return nil }, func(string) error { return nil }); !errors.Is(err, want) {
		t.Fatalf("reaper info failure = %v", err)
	}
	old := coverage90DirEntry{name: tempPrefix + "old", info: coverage90FileInfo{name: tempPrefix + "old", mod: time.Now().Add(-2 * longestWindow)}}
	if err := reapTempsWith(path, time.Now(), func(string) ([]os.DirEntry, error) { return []os.DirEntry{old}, nil }, func(string) error { return nil }, func(string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("reaper directory-sync failure = %v", err)
	}

	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := ensureRecordDir(filepath.Join(t.TempDir(), "outside")); err == nil {
		t.Fatal("record directory escaped the state root")
	}
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureOwnedDir(filepath.Join(parent, "child"), true); err == nil {
		t.Fatal("owned-directory check ignored an ENOTDIR failure")
	}
	if err := validateOwnedDir(filepath.Join(t.TempDir(), "missing"), true); err == nil {
		t.Fatal("missing owned directory was valid")
	}
	if err := validateOwnerWith(coverage90FileInfo{}, func() int { return 1 }); err == nil {
		t.Fatal("file info without an owner was valid")
	}

	f, err := os.CreateTemp(t.TempDir(), "private")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validatePrivateFileWith(f, os.Geteuid); err == nil {
		t.Fatal("closed private file was valid")
	}
	dir, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if err := validatePrivateFileWith(dir, os.Geteuid); err == nil {
		t.Fatal("directory was accepted as a private file")
	}

	var value map[string]any
	if err := decodeClosedJSON([]byte(`{} {}`), &value); err == nil {
		t.Fatal("multiple JSON values were accepted")
	}
	if err := decodeClosedJSON([]byte(`{} x`), &value); err == nil {
		t.Fatal("malformed trailing JSON was accepted")
	}
}

func TestCoverage90InvalidateMarkerFailureBoundaries(t *testing.T) {
	t.Run("invalid state root", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "relative-state")
		if err := InvalidateMarker("host"); err == nil {
			t.Fatal("marker invalidation accepted a relative state root")
		}
	})
	t.Run("unsafe state home", func(t *testing.T) {
		stateHome := t.TempDir()
		if err := os.Chmod(stateHome, 0o777); err != nil {
			t.Fatal(err)
		}
		t.Setenv("XDG_STATE_HOME", stateHome)
		if err := InvalidateMarker("host"); err == nil {
			t.Fatal("marker invalidation accepted an unsafe state home")
		}
	})
}
