package journal

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

var errInjectedIO = errors.New("injected filesystem failure")

type faultyFiles struct {
	diskJournalFiles
	stage                         string
	reads, resolutions, absolutes int
}

func (f *faultyFiles) lock(root, ledger string) (func(), error) {
	if f.stage == "write lock" {
		return nil, errInjectedIO
	}
	return f.diskJournalFiles.lock(root, ledger)
}

func (f *faultyFiles) lstat(name string) (os.FileInfo, error) {
	if f.stage == "stat" {
		return nil, errInjectedIO
	}
	return f.diskJournalFiles.lstat(name)
}

func (f *faultyFiles) evalSymlinks(name string) (string, error) {
	f.resolutions++
	if (f.stage == "root resolution" && f.resolutions == 1) || (f.stage == "file resolution" && f.resolutions == 2) {
		return "", errInjectedIO
	}
	if f.stage == "parent symlink" && f.resolutions == 2 {
		return name + ".elsewhere", nil
	}
	return f.diskJournalFiles.evalSymlinks(name)
}

func (f *faultyFiles) abs(name string) (string, error) {
	f.absolutes++
	if (f.stage == "absolute path" && f.absolutes == 1) || (f.stage == "resolved absolute" && f.absolutes == 2) {
		return "", errInjectedIO
	}
	return f.diskJournalFiles.abs(name)
}

func (f *faultyFiles) readFile(name string) ([]byte, error) {
	f.reads++
	if (f.stage == "read" && f.reads == 1) || (f.stage == "recheck read" && f.reads == 2) {
		return nil, errInjectedIO
	}
	data, err := f.diskJournalFiles.readFile(name)
	if f.stage == "concurrent change" && f.reads == 2 {
		return append(data, '\n'), err
	}
	return data, err
}

func (f *faultyFiles) createTemp(dir, pattern string) (journalTempFile, error) {
	if f.stage == "prepare replacement" {
		return nil, errInjectedIO
	}
	tmp, err := f.diskJournalFiles.createTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return &faultyTemp{journalTempFile: tmp, stage: f.stage}, nil
}

func (f *faultyFiles) rename(old, next string) error {
	if f.stage == "atomic rename" {
		return errInjectedIO
	}
	return f.diskJournalFiles.rename(old, next)
}

func (f *faultyFiles) remove(name string) error {
	err := f.diskJournalFiles.remove(name)
	if f.stage == "cleanup failure" && err == nil {
		return errInjectedIO
	}
	return err
}

type faultyTemp struct {
	journalTempFile
	stage string
}

func (f *faultyTemp) Chmod(mode os.FileMode) error {
	if strings.HasPrefix(f.stage, "mode") || f.stage == "cleanup failure" {
		return errInjectedIO
	}
	return f.journalTempFile.Chmod(mode)
}

func (f *faultyTemp) Write(data []byte) (int, error) {
	if strings.HasPrefix(f.stage, "write") {
		return 0, errInjectedIO
	}
	return f.journalTempFile.Write(data)
}

func (f *faultyTemp) Close() error {
	err := f.journalTempFile.Close()
	if strings.Contains(f.stage, "close") {
		return errInjectedIO
	}
	return err
}

func TestFilesystemFailuresNeverPartiallyAppendHistory(t *testing.T) {
	for _, stage := range []string{"stat", "root resolution", "file resolution", "parent symlink", "absolute path", "resolved absolute", "write lock", "read", "prepare replacement", "mode", "mode and close", "write", "write and close", "close", "recheck read", "concurrent change", "atomic rename", "cleanup failure"} {
		t.Run(stage, func(t *testing.T) {
			root, file := writeFixture(t)
			data, err := correctFile(context.Background(), root, config.Defaults(), "widget", requests, metadata, false, &faultyFiles{stage: stage})
			if err == nil || data != nil {
				t.Fatal("failed replacement was reported as complete")
			}
			actual, readErr := os.ReadFile(file)
			if readErr != nil || string(actual) != damagedLedger {
				t.Fatalf("I/O failure changed historical bytes: %v", readErr)
			}
		})
	}
}

type lateCanceledContext struct {
	context.Context
	calls int
}

func (c *lateCanceledContext) Err() error {
	c.calls++
	if c.calls > 1 {
		return context.Canceled
	}
	return nil
}

func TestCanceledReplacementKeepsHistoricalFile(t *testing.T) {
	root, file := writeFixture(t)
	ctx := &lateCanceledContext{Context: context.Background()}
	if _, err := CorrectFile(ctx, root, config.Defaults(), "widget", requests, metadata, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation ignored: %v", err)
	}
	actual, err := os.ReadFile(file)
	if err != nil || string(actual) != damagedLedger {
		t.Fatalf("cancellation changed history: %v", err)
	}
}
