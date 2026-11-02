package store

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Lock serializes writers with a persistent kernel-owned sidecar. A live
// holder never loses ownership because it is old or paused. A dead holder's
// descriptor is closed by the kernel. Never unlink the sidecar: every writer
// must lock the same inode. Old directory locks fail closed during upgrades.
func Lock(path string, wait time.Duration) (release func(), err error) {
	return lockWith(path, wait, tryMemoryLock)
}

func lockWith(path string, wait time.Duration, attempt func(*os.File) (bool, error)) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("lock %s (remove an old directory lock only after stopping its writer): %w", path, err)
	}
	deadline := time.Now().Add(wait)
	for {
		locked, err := attempt(f)
		if err != nil {
			_ = f.Close() // Preserve the lock error; no ownership was obtained.
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		if locked {
			var once sync.Once
			return func() { once.Do(func() { _ = f.Close() /* Closing releases this owner's kernel lock. */ }) }, nil
		}
		if !time.Now().Before(deadline) {
			_ = f.Close() // Contention is the actionable failure.
			return nil, fmt.Errorf("another memory write holds the lock (%s); try again in a moment", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
