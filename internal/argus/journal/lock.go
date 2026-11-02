package journal

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path"
)

// Keep the sidecar inode stable across ledger replacement and process exits.
// A competing correction refuses before reading; a retry reads the new ledger.
// The kernel releases ownership on close or a crash. Never unlink the sidecar.
func acquireJournalLock(repository, ledger string) (func(), error) {
	root, err := os.OpenRoot(repository)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }() // Read-only root handle, no data to flush.
	const dir = ".pyxis/state/journal-correction"
	if err := root.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	name := path.Join(dir, fmt.Sprintf("%x.lock", sha256.Sum256([]byte(ledger))))
	file, err := root.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockJournalFile(file); err != nil {
		_ = file.Close() // Preserve the lock refusal; no ownership was obtained.
		return nil, err
	}
	return func() { _ = file.Close() /* Closing releases kernel ownership. */ }, nil
}
