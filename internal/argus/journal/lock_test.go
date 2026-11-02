package journal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

type pausedReplacement struct{ diskJournalFiles }

func (p pausedReplacement) rename(old, next string) error {
	if _, err := fmt.Fprintln(os.Stdout, "replacement ready"); err != nil {
		return err
	}
	if _, err := io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
		return err
	}
	return p.diskJournalFiles.rename(old, next)
}

func TestCorrectionProcessHelper(t *testing.T) {
	mode := os.Getenv("PYXIS_JOURNAL_CORRECTION_HELPER")
	if mode == "" {
		return
	}
	meta := metadata
	meta.Principal = "corrector-" + mode
	var files journalFiles = diskJournalFiles{}
	if mode == "holder" {
		files = pausedReplacement{}
	}
	if _, err := correctFile(context.Background(), os.Getenv("PYXIS_JOURNAL_CORRECTION_ROOT"), config.Defaults(), "widget", requests, meta, false, files); err != nil {
		t.Fatal(err)
	}
}

// The holder pauses after the final byte comparison, precisely where the old
// implementation let another successful process replace its pending append.
func TestConcurrentCorrectionProcessesCannotLoseSuccessfulEntries(t *testing.T) {
	root, file := writeFixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := func(mode string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestCorrectionProcessHelper$")
		cmd.Env = append(os.Environ(), "PYXIS_JOURNAL_CORRECTION_HELPER="+mode, "PYXIS_JOURNAL_CORRECTION_ROOT="+root)
		return cmd
	}
	holder := command("holder")
	stdin, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	holder.Stderr = &stderr
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = holder.Process.Kill() // Stop a paused helper on any test failure.
			_ = holder.Wait()
		}
	}()
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(ready) != "replacement ready" {
		t.Fatalf("holder did not reach the replacement boundary: %q %v %s", ready, err, &stderr)
	}
	output, err := command("contender").CombinedOutput()
	if err == nil || !bytes.Contains(output, []byte("another journal correction holds the lock")) {
		t.Fatalf("a competing process succeeded inside the read/replace transaction: %v %s", err, output)
	}
	before, err := os.ReadFile(file)
	if err != nil || string(before) != damagedLedger {
		t.Fatalf("refused contention changed the original: %v", err)
	}
	if _, err := stdin.Write([]byte("continue")); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, stdout); err != nil {
		t.Fatal(err)
	}
	err = holder.Wait()
	finished = true
	if err != nil {
		t.Fatalf("holder failed: %v %s", err, &stderr)
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.HasPrefix(after, []byte(damagedLedger)) || bytes.Count(after, []byte("**Principal**: corrector-holder")) != 2 || bytes.Contains(after, []byte("corrector-contender")) {
		t.Fatalf("a successful append was lost or history changed: %v", err)
	}
	output, err = command("contender").CombinedOutput()
	if err == nil || bytes.Contains(output, []byte("another journal correction holds the lock")) {
		t.Fatalf("retry did not release ownership and read the corrected ledger: %v %s", err, output)
	}
}

func TestCorrectionPreviewDoesNotAcquireOrCreateWriterLock(t *testing.T) {
	root, _ := writeFixture(t)
	if _, err := CorrectFile(context.Background(), root, config.Defaults(), "widget", requests, metadata, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".pyxis")); !os.IsNotExist(err) {
		t.Fatalf("preview created writer state: %v", err)
	}
	release, err := acquireJournalLock(root, "docs/problems/widget/delivery.md")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := CorrectFile(context.Background(), root, config.Defaults(), "widget", requests, metadata, true); err != nil {
		t.Fatalf("preview was blocked by a held writer: %v", err)
	}
	if candidate, err := CorrectFile(context.Background(), root, config.Defaults(), "widget", requests, metadata, false); err == nil || candidate != nil || !strings.Contains(err.Error(), "another journal correction holds the lock") {
		t.Fatalf("a second descriptor acquired held ownership: %v", err)
	}
}

func TestCorrectionLockRefusesFilesystemFailures(t *testing.T) {
	if release, err := acquireJournalLock(filepath.Join(t.TempDir(), "absent"), "ledger"); err == nil || release != nil {
		t.Fatal("missing root admitted")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".pyxis"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if release, err := acquireJournalLock(root, "ledger"); err == nil || release != nil {
		t.Fatal("blocked lock directory admitted")
	}
	root = t.TempDir()
	name := filepath.Join(root, ".pyxis", "state", "journal-correction", fmt.Sprintf("%x.lock", sha256.Sum256([]byte("ledger"))))
	if err := os.MkdirAll(name, 0o700); err != nil {
		t.Fatal(err)
	}
	if release, err := acquireJournalLock(root, "ledger"); err == nil || release != nil {
		t.Fatal("non-file sidecar admitted")
	}
	file, err := os.CreateTemp(t.TempDir(), "closed-lock")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lockJournalFile(file); err == nil {
		t.Fatal("unavailable kernel lock silently admitted")
	}
}

func TestRemovingExecutionLogsCannotBreakHeldCorrectionLock(t *testing.T) {
	root, file := writeFixture(t)
	release, err := acquireJournalLock(root, "docs/problems/widget/delivery.md")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	logs := filepath.Join(root, ".pyxis", "logs")
	if err := os.MkdirAll(logs, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "diagnostic.txt"), []byte("regenerable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(logs); err != nil {
		t.Fatal(err)
	}
	if candidate, err := CorrectFile(context.Background(), root, config.Defaults(), "widget", requests, metadata, false); err == nil || candidate != nil || !strings.Contains(err.Error(), "another journal correction holds the lock") {
		t.Fatalf("log cleanup destroyed persistent ownership: %v", err)
	}
	actual, err := os.ReadFile(file)
	if err != nil || string(actual) != damagedLedger {
		t.Fatalf("a competing write changed history after log cleanup: %v", err)
	}
}
