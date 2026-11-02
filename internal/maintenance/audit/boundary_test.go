package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func completeEvent() Event {
	return Event{SchemaVersion: SchemaVersion, Timestamp: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Level: LevelInfo, Event: EventLifecycle, Phase: PhaseAgent,
		TraceID: strings.Repeat("a", 32), InvocationID: strings.Repeat("b", 32), HostID: strings.Repeat("c", 32), RepositoryID: strings.Repeat("d", 64)}
}

func TestKnownContextAndClosedEventValidation(t *testing.T) {
	root := ignoredRepository(t)
	w, err := Open(t.Context(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.now = func() time.Time { return completeEvent().Timestamp }
	event := completeEvent()
	event.JobID, event.JobDate, event.Timezone = "daily", "2026-10-15", "America/Los_Angeles"
	event.Route = &Route{Harness: "codex", Profile: "profile", Model: "native", Effort: "high", Policy: "policy", Qualification: "qualified"}
	event.Launch = event.Route
	event.Claim = &Claim{Owner: "owner", Ref: "refs/maintenance/daily", RecordOID: strings.Repeat("e", 40)}
	event.TargetVerification = &TargetVerification{Status: TargetNotVerified}
	for _, level := range []Level{LevelInfo, LevelWarn, LevelError} {
		event.Level = level
		if _, err := w.Append(t.Context(), event, Buffered); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(journalPath)))
	if err != nil || !bytes.Contains(data, []byte(`"job_id":"daily"`)) || !bytes.Contains(data, []byte(`"qualification":"qualified"`)) {
		t.Fatalf("known context missing: %s, %v", data, err)
	}
	for _, tc := range []struct {
		name string
		edit func(*Event)
	}{
		{"schema", func(e *Event) { e.SchemaVersion = "future" }},
		{"time", func(e *Event) { e.Timestamp = time.Time{} }},
		{"zone", func(e *Event) { e.Timestamp = e.Timestamp.In(time.FixedZone("other", 3600)) }},
		{"identity", func(e *Event) { e.TraceID = "invalid" }},
		{"level", func(e *Event) { e.Level = "unknown" }},
		{"event", func(e *Event) { e.Event = "unknown" }},
		{"phase", func(e *Event) { e.Phase = "unknown" }},
		{"outcome", func(e *Event) { e.Outcome = "unknown" }},
		{"target", func(e *Event) { e.TargetVerification = &TargetVerification{Status: "unknown"} }},
		{"cause", func(e *Event) { e.CauseCode = "unsafe code" }},
		{"unencodable time", func(e *Event) { e.Timestamp = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := completeEvent()
			tc.edit(&event)
			if _, err := encode(event); err == nil {
				t.Fatal("invalid event encoded")
			}
		})
	}
}

func TestEncodedByteBoundsAndUTF8(t *testing.T) {
	event := completeEvent()
	control := strings.Repeat("\x00", 4096)
	routeControl := strings.Repeat("\x00", 512)
	event.Route = &Route{Harness: routeControl, Profile: routeControl, Model: routeControl, Effort: routeControl, Policy: routeControl, Qualification: routeControl}
	event.Cause = control
	data, err := encode(event)
	if err != nil || len(data) > MaxRecordBytes || !json.Valid(bytes.TrimSpace(data)) || !bytes.Contains(data, []byte(`"cause_truncated":true`)) {
		t.Fatalf("escaped cause was not bounded: %d, %v", len(data), err)
	}
	event.Launch = event.Route
	if _, err := encode(event); err == nil {
		t.Fatal("oversized identity context admitted")
	}
	if got := truncateUTF8("éé", 3); got != "é" {
		t.Fatalf("UTF-8 truncation = %q", got)
	}
}

func TestAdmissionRootAndStateRefusals(t *testing.T) {
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(cancelled, ignoredRepository(t), quiet()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled admission = %v", err)
	}
	for _, tc := range []struct {
		name string
		path string
		data string
		mode os.FileMode
	}{
		{"lock permissions", ".pyxis/state/maintenance/audit.lock", "", 0o400},
		{"identity permissions", ".pyxis/state/maintenance/host-id", strings.Repeat("a", 32), 0o400},
		{"empty identity", ".pyxis/state/maintenance/host-id", "", 0o600},
		{"long identity", ".pyxis/state/maintenance/host-id", strings.Repeat("a", 129), 0o600},
		{"invalid identity", ".pyxis/state/maintenance/host-id", strings.Repeat("z", 32), 0o600},
		{"journal parent file", ".pyxis/maintenance", "file", 0o600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := ignoredRepository(t)
			path := filepath.Join(root, filepath.FromSlash(tc.path))
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.data), tc.mode); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(t.Context(), root, quiet()); err == nil {
				t.Fatal("unsafe state admitted")
			}
		})
	}
	root := ignoredRepository(t)
	subdir := filepath.Join(root, "nested")
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "missing"), subdir, filepath.Join(root, ".gitignore"), t.TempDir()} {
		if _, err := Open(t.Context(), path, quiet()); err == nil {
			t.Fatalf("invalid root admitted: %s", path)
		}
	}
	first, err := Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Open(t.Context(), root, quiet())
	if err != nil || first.hostID != second.hostID {
		t.Fatalf("host identity changed: %v", err)
	}
}

func TestAdmissionIOFailures(t *testing.T) {
	injected := errors.New("injected audit IO failure")
	for _, target := range []string{"audit.lock", "logs.jsonl", "host-id"} {
		t.Run("open "+target, func(t *testing.T) {
			hooks := defaultHooks()
			original := hooks.open
			hooks.open = func(r *os.Root, name string, flag int, mode os.FileMode) (*os.File, error) {
				if filepath.Base(name) == target {
					return nil, injected
				}
				return original(r, name, flag, mode)
			}
			if _, err := open(t.Context(), ignoredRepository(t), quiet(), hooks); !errors.Is(err, injected) {
				t.Fatalf("open refusal lost cause: %v", err)
			}
		})
	}
	for _, target := range []string{"logs.jsonl", "host-id", "directory", "host write"} {
		t.Run(target, func(t *testing.T) {
			hooks := defaultHooks()
			originalSync, originalWrite := hooks.sync, hooks.write
			hooks.sync = func(f *os.File) error {
				if filepath.Base(f.Name()) == target {
					return injected
				}
				return originalSync(f)
			}
			if target == "directory" {
				hooks.syncRoot = func(*os.Root) error { return injected }
			}
			hooks.write = func(f *os.File, data []byte) (int, error) {
				if target == "host write" && filepath.Base(f.Name()) == "host-id" {
					return 0, injected
				}
				return originalWrite(f, data)
			}
			if _, err := open(t.Context(), ignoredRepository(t), quiet(), hooks); !errors.Is(err, injected) {
				t.Fatalf("durability refusal lost cause: %v", err)
			}
		})
	}
}

func TestAppendCancellationAndDurabilityFailure(t *testing.T) {
	root := ignoredRepository(t)
	w, err := Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	event := lifecycle(identity(t), "fixture")
	if _, err := w.Append(t.Context(), event, Durability(99)); err == nil {
		t.Fatal("invalid durability admitted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := w.Append(ctx, event, Consequential); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled append = %v", err)
	}
	injected := errors.New("injected sync failure")
	original := w.hooks.sync
	w.hooks.sync = func(*os.File) error { return injected }
	if _, err := w.Append(t.Context(), event, Consequential); !errors.Is(err, injected) {
		t.Fatalf("admission sync = %v", err)
	}
	calls := 0
	w.hooks.sync = func(f *os.File) error {
		calls++
		if calls == 2 {
			return injected
		}
		return original(f)
	}
	_, err = w.Append(t.Context(), event, Consequential)
	var uncertain *UncertainError
	if !errors.As(err, &uncertain) || !errors.Is(err, injected) || uncertain.Written == 0 || !strings.Contains(err.Error(), "uncertain") {
		t.Fatalf("terminal sync lost uncertainty: %v", err)
	}
	w.hooks.sync = original
	w.hooks.write = func(*os.File, []byte) (int, error) { return 0, nil }
	if _, err := w.Append(t.Context(), event, Buffered); err == nil || !errors.As(err, &uncertain) || uncertain.Written != 0 {
		t.Fatalf("zero write = %v", err)
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(journalPath))); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(t.Context(), event, Buffered); err == nil {
		t.Fatal("removed journal recreated during append")
	}
}

func TestAppendCancellationAtAcquiredLockBoundaryReleasesLock(t *testing.T) {
	root := ignoredRepository(t)
	writer, err := Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(journalPath))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	originalOpen := writer.hooks.open
	writer.hooks.open = func(root *os.Root, name string, flag int, mode os.FileMode) (*os.File, error) {
		file, err := originalOpen(root, name, flag, mode)
		if err == nil && filepath.Base(name) == "audit.lock" {
			cancel()
		}
		return file, err
	}
	if _, err := writer.Append(ctx, lifecycle(identity(t), "must not be written"), Consequential); !errors.Is(err, context.Canceled) {
		t.Fatalf("lock-boundary cancellation = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("cancelled append changed journal: %v %q", err, after)
	}
	writer.hooks.open = originalOpen
	if _, err := writer.Append(t.Context(), lifecycle(identity(t), "live append"), Consequential); err != nil {
		t.Fatalf("append after cancellation did not acquire released lock: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || bytes.Contains(data, []byte("must not be written")) || !bytes.Contains(data, []byte(`"summary":"live append"`)) {
		t.Fatalf("post-cancellation journal = %v %s", err, data)
	}
}

func TestContendedLockCancellationAndTailReadFailure(t *testing.T) {
	root := ignoredRepository(t)
	w, err := Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(root, ".pyxis/state/maintenance/audit.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }() // Closing releases this test's advisory lock.
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	original := w.hooks.open
	w.hooks.open = func(r *os.Root, name string, flag int, mode os.FileMode) (*os.File, error) {
		f, err := original(r, name, flag, mode)
		if filepath.Base(name) == "audit.lock" && err == nil {
			cancel()
		}
		return f, err
	}
	if err := w.withLock(ctx, func(*os.Root, *os.File) error { t.Fatal("contended callback ran"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("contended cancellation = %v", err)
	}
	file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(journalPath)), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, "tail"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tailState(file); err == nil {
		t.Fatal("write-only tail read succeeded")
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tailState(file); err == nil {
		t.Fatal("closed tail file admitted")
	}
}
