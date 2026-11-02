package audit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func repository(t *testing.T, ignore string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(ignore), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "--", ".gitignore")
	git(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "fixture")
	return root
}

func ignoredRepository(t *testing.T) string {
	return repository(t, "/.pyxis/maintenance/logs.jsonl\n/.pyxis/state/\n")
}

func identity(t *testing.T) Identity {
	t.Helper()
	value, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func lifecycle(id Identity, summary string) Event {
	return Event{Level: LevelInfo, Event: EventLifecycle, Phase: PhaseAdmission, TraceID: id.TraceID, InvocationID: id.InvocationID, Outcome: OutcomeRefused, CauseCode: "fixture_refusal", Summary: summary}
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRecordIsClosedBoundedRedactedAndLeavesUnknownIdentityAbsent(t *testing.T) {
	root := ignoredRepository(t)
	w, err := Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	canary := "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	credentialURL := "https://operator-secret@example.com/org/repo.git"
	event := lifecycle(identity(t), strings.Repeat("\x00", 5000)+" token="+canary+" "+credentialURL)
	event.Cause = strings.Repeat("\x01", 5000) + " PASSWORD=" + canary
	event.Delivery = &Delivery{PRURL: credentialURL}
	for index := range 100 {
		event.References = append(event.References, Reference{Kind: "report", Location: fmt.Sprintf("%s/%d/%s", credentialURL, index, strings.Repeat("x", 1000))})
	}
	if _, err := w.Append(t.Context(), event, Consequential); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(journalPath)))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(canary)) || bytes.Contains(data, []byte("operator-secret")) || len(data) > MaxRecordBytes {
		t.Fatalf("credential or oversized record escaped: %d bytes: %s", len(data), data)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSuffix(data, []byte{'\n'}), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema_version"] != SchemaVersion || got["summary_truncated"] != true || got["references_truncated"] != true {
		t.Fatalf("bounded schema markers absent: %#v", got)
	}
	if _, present := got["job_id"]; present {
		t.Fatal("unknown job identity was invented")
	}
	if _, present := got["job_date"]; present {
		t.Fatal("unknown job date was invented")
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(journalPath)))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("journal mode = %v, %v", info.Mode(), err)
	}
}

func TestAdmissionRejectsTrackedNonIgnoredSymlinkedUnwritableAndUnsyncableTargets(t *testing.T) {
	t.Run("tracked", func(t *testing.T) {
		root := ignoredRepository(t)
		path := filepath.Join(root, filepath.FromSlash(journalPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("tracked\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "-f", "--", journalPath)
		git(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "tracked journal")
		if _, err := Open(t.Context(), root, quiet()); err == nil {
			t.Fatal("tracked journal admitted")
		}
	})
	t.Run("non ignored", func(t *testing.T) {
		root := repository(t, "/.pyxis/state/\n")
		if _, err := Open(t.Context(), root, quiet()); err == nil || !strings.Contains(err.Error(), "/.pyxis/maintenance/logs.jsonl") || !strings.Contains(err.Error(), ".gitignore") {
			t.Fatalf("missing-ignore remedy absent: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(journalPath))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("refusal created journal: %v", err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		root := ignoredRepository(t)
		path := filepath.Join(root, filepath.FromSlash(journalPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, path); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(t.Context(), root, quiet()); err == nil {
			t.Fatal("symlinked journal admitted")
		}
		if got, _ := os.ReadFile(outside); string(got) != "outside" {
			t.Fatal("symlink target changed")
		}
	})
	t.Run("unwritable", func(t *testing.T) {
		root := ignoredRepository(t)
		path := filepath.Join(root, filepath.FromSlash(journalPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("history\n"), 0o400); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(t.Context(), root, quiet()); err == nil {
			t.Fatal("unwritable journal admitted")
		}
		if got, _ := os.ReadFile(path); string(got) != "history\n" {
			t.Fatal("unwritable history changed")
		}
	})
	t.Run("unsyncable", func(t *testing.T) {
		root := ignoredRepository(t)
		path := filepath.Join(root, filepath.FromSlash(journalPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("history\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		hooks := defaultHooks()
		hooks.sync = func(file *os.File) error {
			if filepath.Base(file.Name()) == "logs.jsonl" {
				return errors.New("injected sync refusal")
			}
			return file.Sync()
		}
		if _, err := open(t.Context(), root, quiet(), hooks); err == nil || !strings.Contains(err.Error(), "sync refusal") {
			t.Fatalf("unsyncable journal = %v", err)
		}
		if got, _ := os.ReadFile(path); string(got) != "history\n" {
			t.Fatal("unsyncable history changed")
		}
	})
}

func TestIncompleteTailAndPartialWritePreserveHistoryAndReportUncertainty(t *testing.T) {
	root := ignoredRepository(t)
	w, err := Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(journalPath))
	history := []byte(`{"historical":"unfinished"`)
	if err := os.WriteFile(path, history, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := w.Append(t.Context(), lifecycle(identity(t), "after historical tail"), Consequential)
	if err != nil || !result.HistoricalTailIncomplete {
		t.Fatalf("tail result = %+v, %v", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(data, append(append([]byte(nil), history...), '\n')) {
		t.Fatalf("historical tail changed: %v: %q", err, data)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
	if len(lines) != 3 {
		t.Fatalf("tail repair records = %d: %q", len(lines), data)
	}
	for _, line := range lines[1:] {
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("appended record not independently decodable: %v: %q", err, line)
		}
	}

	root = ignoredRepository(t)
	w, err = Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(root, filepath.FromSlash(journalPath))
	injected := errors.New("injected partial write")
	originalWrite := w.hooks.write
	first := true
	w.hooks.write = func(file *os.File, data []byte) (int, error) {
		if first {
			first = false
			n, writeErr := originalWrite(file, data[:len(data)/2])
			if writeErr != nil {
				return n, writeErr
			}
			return n, injected
		}
		return originalWrite(file, data)
	}
	if _, err := w.Append(t.Context(), lifecycle(identity(t), "partial"), Consequential); err == nil {
		t.Fatal("partial append reported success")
	} else {
		var uncertain *UncertainError
		if !errors.As(err, &uncertain) || uncertain.Written == 0 {
			t.Fatalf("partial append uncertainty = %v", err)
		}
	}
	partial, err := os.ReadFile(path)
	if err != nil || len(partial) == 0 || partial[len(partial)-1] == '\n' {
		t.Fatalf("partial fixture did not leave an incomplete tail: %v %q", err, partial)
	}
	w.hooks.write = originalWrite
	result, err = w.Append(t.Context(), lifecycle(identity(t), "after partial"), Consequential)
	if err != nil || !result.HistoricalTailIncomplete {
		t.Fatalf("partial recovery = %+v, %v", result, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(after, append(append([]byte(nil), partial...), '\n')) || !bytes.Contains(after, []byte(`"event":"journal_tail_incomplete"`)) {
		t.Fatalf("partial history was not retained and diagnosed: %v %q", err, after)
	}
}

func TestAppendProcessHelper(t *testing.T) {
	root := os.Getenv("PYXIS_MAINTENANCE_AUDIT_HELPER")
	if root == "" {
		return
	}
	w, err := Open(context.Background(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	id := identity(t)
	if _, err := w.Append(context.Background(), lifecycle(id, os.Getenv("PYXIS_MAINTENANCE_AUDIT_RECORD")), Consequential); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentProcessesAppendCompleteRecords(t *testing.T) {
	root := ignoredRepository(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const writers = 12
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wait sync.WaitGroup
	errorsFound := make(chan error, writers)
	for index := range writers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestAppendProcessHelper$")
			command.Env = append(os.Environ(), "PYXIS_MAINTENANCE_AUDIT_HELPER="+root, fmt.Sprintf("PYXIS_MAINTENANCE_AUDIT_RECORD=writer-%02d", index))
			if output, err := command.CombinedOutput(); err != nil {
				errorsFound <- fmt.Errorf("writer %d: %w: %s", index, err, output)
			}
		}(index)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	file, err := os.Open(filepath.Join(root, filepath.FromSlash(journalPath)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("interleaved record: %v: %q", err, scanner.Bytes())
		}
		seen[event.Summary] = true
	}
	if err := scanner.Err(); err != nil || len(seen) != writers {
		t.Fatalf("complete records = %d/%d, scan=%v", len(seen), writers, err)
	}
}

func defaultHooks() fileHooks {
	return fileHooks{
		open: func(root *os.Root, name string, flag int, mode os.FileMode) (*os.File, error) {
			return root.OpenFile(name, flag, mode)
		},
		write:    func(file *os.File, data []byte) (int, error) { return file.Write(data) },
		sync:     func(file *os.File) error { return file.Sync() },
		syncRoot: syncDirectory,
	}
}
