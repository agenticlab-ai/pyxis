package journal

import (
	"os"
	"path/filepath"
)

// The consumer owns this small filesystem port so atomicity can be verified
// with deterministic I/O failures, including failures hidden by a root UID.
type journalFiles interface {
	readFile(string) ([]byte, error)
	lstat(string) (os.FileInfo, error)
	evalSymlinks(string) (string, error)
	abs(string) (string, error)
	createTemp(string, string) (journalTempFile, error)
	rename(string, string) error
	remove(string) error
	lock(string, string) (func(), error)
}

type journalTempFile interface {
	Name() string
	Chmod(os.FileMode) error
	Write([]byte) (int, error)
	Close() error
}

type diskJournalFiles struct{}

func (diskJournalFiles) readFile(name string) ([]byte, error)   { return os.ReadFile(name) }
func (diskJournalFiles) lstat(name string) (os.FileInfo, error) { return os.Lstat(name) }
func (diskJournalFiles) evalSymlinks(name string) (string, error) {
	return filepath.EvalSymlinks(name)
}
func (diskJournalFiles) abs(name string) (string, error) { return filepath.Abs(name) }
func (diskJournalFiles) createTemp(dir, pattern string) (journalTempFile, error) {
	return os.CreateTemp(dir, pattern)
}
func (diskJournalFiles) rename(oldName, newName string) error { return os.Rename(oldName, newName) }
func (diskJournalFiles) remove(name string) error             { return os.Remove(name) }
func (diskJournalFiles) lock(root, ledger string) (func(), error) {
	return acquireJournalLock(root, ledger)
}
