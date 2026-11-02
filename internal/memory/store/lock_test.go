package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestLockSerialisesWriters(t *testing.T) {
	p := filepath.Join(t.TempDir(), "memory-write.lock")
	first, err := Lock(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	old := time.Unix(0, 0)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(p, 0); err == nil || !strings.Contains(err.Error(), "holds the lock") {
		t.Fatalf("paused old live owner lost its lock: %v", err)
	}
	first()
	second, err := Lock(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer second()
	first() // An old owner's repeated release must never release its successor.
	if _, err := Lock(p, 0); err == nil {
		t.Fatal("old release unlocked successor")
	}
	second()
	third, err := Lock(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	third()
	if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("sidecar must retain one inode: %v", err)
	}
}

func TestLockWaitingAndFailures(t *testing.T) {
	p := filepath.Join(t.TempDir(), "memory-write.lock")
	held, err := Lock(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	synctest.Test(t, func(t *testing.T) {
		if _, err := Lock(p, time.Second); err == nil {
			t.Fatal("a held lock passed")
		}
	})
	if _, err := Lock(t.TempDir(), 0); err == nil {
		t.Fatal("old directory lock must refuse upgrade")
	}
	if _, err := Lock(filepath.Join(t.TempDir(), "absent", "lock"), 0); err == nil {
		t.Fatal("missing parent must fail")
	}
	f, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := tryMemoryLock(f); err == nil {
		t.Fatal("closed descriptor was locked")
	}
	failure := errors.New("kernel refused locking")
	var attempted *os.File
	if release, err := lockWith(filepath.Join(t.TempDir(), "lock"), 0, func(f *os.File) (bool, error) {
		attempted = f
		return false, failure
	}); release != nil || !errors.Is(err, failure) {
		t.Fatalf("lost kernel refusal: %v", err)
	}
	if _, err := attempted.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("failed lock leaked descriptor: %v", err)
	}
}

func TestLockWaitsForRelease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "memory-write.lock")
		release, err := Lock(path, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		other, err := Lock(filepath.Join(t.TempDir(), "other.lock"), time.Second)
		if err != nil {
			t.Fatalf("another path is its own lock: %v", err)
		}
		other()
		done := make(chan struct{})
		go func() {
			defer close(done)
			again, err := Lock(path, 10*time.Second)
			if err != nil {
				t.Errorf("waiting writer: %v", err)
				return
			}
			again()
		}()
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("contender returned before release")
		default:
		}
		release()
		<-done
	})
}

func TestLockUnwritableDirectoryFailsWithoutWaiting(t *testing.T) {
	sealed := t.TempDir()
	if err := os.Chmod(sealed, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(sealed, 0o700); err != nil {
			t.Error(err)
		}
	})
	if release, err := Lock(filepath.Join(sealed, "x.lock"), time.Second); err == nil {
		release()
		t.Fatal("an unwritable lock dir must fail")
	} else if strings.Contains(err.Error(), "holds the lock") {
		t.Fatalf("permission failure classified as contention: %v", err)
	}
}
