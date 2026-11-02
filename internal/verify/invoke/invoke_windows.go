//go:build windows

package invoke

import (
	"errors"
	"os"
)

// tryLockFile fails CLOSED on Windows: per-workspace locking is not implemented
// here, so AcquireWorkspaceLock refuses rather than silently granting a lock it
// cannot enforce (fail-open would let concurrent runs corrupt the workspace).
func tryLockFile(_ *os.File) error {
	return errors.New("workspace locking is not implemented on windows")
}
