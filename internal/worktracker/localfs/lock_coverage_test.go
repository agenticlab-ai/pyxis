package localfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithItemLockNamesBlockedLockDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, lockDirName), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := New(root, fixedClock())
	if err := store.withItemLock(1, func() error { return nil }); err == nil {
		t.Fatal("blocked lock directory succeeded")
	}
}
