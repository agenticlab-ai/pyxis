package budget

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type traceFile struct {
	name  string
	label string
	kind  string
	trace *[]string
}

func (f *traceFile) Name() string { return f.name }
func (f *traceFile) Chmod(os.FileMode) error {
	*f.trace = append(*f.trace, f.label+":chmod-"+f.kind)
	return nil
}
func (f *traceFile) Write(p []byte) (int, error) {
	*f.trace = append(*f.trace, f.label+":write-"+f.kind)
	return len(p), nil
}
func (f *traceFile) Sync() error {
	*f.trace = append(*f.trace, f.label+":fsync-"+f.kind)
	return nil
}
func (f *traceFile) Close() error {
	*f.trace = append(*f.trace, f.label+":close-"+f.kind)
	return nil
}

func tracedOps(label string, trace *[]string) durableOps {
	return durableOps{
		createTemp: func(dir, _ string) (durableFile, error) {
			*trace = append(*trace, label+":create-temp")
			return &traceFile{name: filepath.Join(dir, tempPrefix+"trace"), label: label, kind: "temp", trace: trace}, nil
		},
		rename: func(_, _ string) error {
			*trace = append(*trace, label+":rename")
			return nil
		},
		remove: func(path string) error {
			if !strings.HasPrefix(filepath.Base(path), tempPrefix) {
				*trace = append(*trace, label+":unlink")
			}
			return nil
		},
		openDir: func(path string) (durableFile, error) {
			*trace = append(*trace, label+":open-dir")
			return &traceFile{name: path, label: label, kind: "dir", trace: trace}, nil
		},
	}
}

type failingSyncFile struct {
	*os.File
	err error
}

func (f *failingSyncFile) Sync() error { return f.err }

// A-20: exclusion survives replacement.
//
// flock binds to an INODE, and writeDurable renames a new inode over the
// record. A process locking the record file itself would hold the OLD inode
// while a second process locking the record after the rename holds the new
// one — mutual exclusion would stop existing at exactly the moment a write
// commits. Sidecars are never renamed over, so every process locks the same
// inode. This test fails if the lock is ever taken on the record.
func TestExclusionSurvivesRecordReplacement(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()

	const writers, each = 8, 25
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := newBudget(t, c, "github.com", "octocat", nil)
			for j := 0; j < each; j++ {
				if err := b.Acquire(context.Background(), 1); err != nil {
					t.Errorf("acquire: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	umbrella, _ := HostPath("github.com")
	got := len(readLedger(t, umbrella).Hour)
	if got != writers*each {
		t.Errorf("umbrella recorded %d writes, want %d — a lost update means exclusion broke across a rename",
			got, writers*each)
	}
}

// The lock is a sidecar beside the record, never the record itself.
func TestLockIsASidecar(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.umbrellaPath + lockSuffix); err != nil {
		t.Errorf("no sidecar lock beside the umbrella: %v", err)
	}
	if _, err := os.Stat(b.identityPath + lockSuffix); err != nil {
		t.Errorf("no sidecar lock beside the identity ledger: %v", err)
	}
}

// A-20 / AP-12: scratch is reaped, not merely abandoned. A crash between
// create and rename strands a temp; a resource type without a reaper
// accumulates husks.
func TestStrandedTempsAreReaped(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cutOver(t, "github.com")
	c := newClock()
	b := newBudget(t, c, "github.com", "octocat", nil)
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Dir(b.umbrellaPath)
	old := filepath.Join(dir, tempPrefix+"stranded")
	fresh := filepath.Join(dir, tempPrefix+"inflight")
	for _, p := range []string{old, fresh} {
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Age both against the SAME clock the reaper reads. Anchoring one to the
	// wall clock and the other to the injected one makes the outcome depend on
	// what time of day the suite runs, which is how the first draft of this
	// test passed and failed by turns.
	stale := c.now().Add(-2 * longestWindow)
	if err := os.Chtimes(old, stale, stale); err != nil {
		t.Fatal(err)
	}
	fresher := c.now()
	if err := os.Chtimes(fresh, fresher, fresher); err != nil {
		t.Fatal(err)
	}

	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("a stranded temp older than the longest window survived the reaper")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("the reaper removed a young temp — it may belong to a writer mid-commit")
	}
}

func TestReaperFailureStopsAdmission(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "_host.json")
	stale := filepath.Join(dir, tempPrefix+"stranded")
	if err := os.WriteFile(stale, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * longestWindow)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	want := errors.New("injected remove failure")
	err := reapTempsWith(record, time.Now(), os.ReadDir, func(string) error { return want }, fsyncDir)
	if !errors.Is(err, want) {
		t.Fatalf("reapTempsWith error = %v, want injected removal failure", err)
	}
}

func TestBudgetRejectsNonPrivateHostDirectory(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	b, err := NewForIdentity("github.com", "octocat", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(b.umbrellaPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := b.Acquire(context.Background(), 1); err == nil {
		t.Fatal("budget admitted through a host directory writable by other principals")
	}
}

func TestBudgetRejectsSymlinkedBudgetDirectory(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)
	root := filepath.Join(xdg, "pyxis")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "content-budget")); err != nil {
		t.Fatal(err)
	}
	b, err := NewForIdentity("github.com", "octocat", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Acquire(context.Background(), 1); err == nil {
		t.Fatal("budget admitted through a symlinked control directory")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("validation followed the symlink before rejecting it: %v", entries)
	}
}

// writeDurable replaces atomically: a reader never sees a partial record.
func TestWriteDurableLeavesNoPartialRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "record.json")
	if err := writeDurable(path, ledger{Minute: []time.Time{}, Hour: []time.Time{}}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), tempPrefix) {
			t.Errorf("a temp survived a successful write: %s", e.Name())
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the record was not published: %v", err)
	}
}

func TestWriteDurableSyscallOrder(t *testing.T) {
	var trace []string
	path := filepath.Join(t.TempDir(), "record.json")
	if err := writeDurableWith(tracedOps("record", &trace), path, ledger{}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"record:create-temp", "record:chmod-temp", "record:write-temp",
		"record:fsync-temp", "record:close-temp", "record:rename",
		"record:open-dir", "record:fsync-dir", "record:close-dir",
	}
	if fmt.Sprint(trace) != fmt.Sprint(want) {
		t.Fatalf("durable replacement order:\n got %v\nwant %v", trace, want)
	}
}

func TestOrderedPairDoesNotStartIdentityBeforeUmbrellaDirectorySync(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	b, err := NewForIdentity("github.com", "octocat", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var trace []string
	b.writeRecord = func(path string, v any) error {
		label := "identity"
		if path == b.umbrellaPath {
			label = "umbrella"
		}
		return writeDurableWith(tracedOps(label, &trace), path, v)
	}
	if err := b.commit(ledger{}, ledger{}, b.identityPath, time.Now(), 1); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(trace, " ")
	if !strings.Contains(joined, "umbrella:fsync-dir umbrella:close-dir identity:create-temp") {
		t.Fatalf("identity began before umbrella durability completed: %v", trace)
	}
}

func TestInitializationDoesNotStartMarkerBeforeConservativeLedgersAreDurable(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	b, err := NewForIdentity("github.com", "octocat", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var trace []string
	b.writeRecord = func(path string, v any) error {
		label := "identity"
		switch path {
		case b.umbrellaPath:
			label = "umbrella"
		case b.markerPath:
			label = "marker"
		}
		return writeDurableWith(tracedOps(label, &trace), path, v)
	}
	if err := b.ensureBarrier(time.Now(), b.identityPath); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(trace, " ")
	if !strings.Contains(joined, "umbrella:fsync-dir umbrella:close-dir identity:create-temp") ||
		!strings.Contains(joined, "identity:fsync-dir identity:close-dir marker:create-temp") {
		t.Fatalf("marker began before conservative ledgers were durable: %v", trace)
	}
}

func TestIdentityPersistenceFailureRetainsUmbrellaAndSendsNothing(t *testing.T) {
	for _, failure := range []string{"fsync", "rename"} {
		t.Run(failure, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			cutOver(t, "github.com")
			b, err := NewForIdentity("github.com", "octocat", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			injected := errors.New("injected identity " + failure + " failure")
			b.writeRecord = func(path string, v any) error {
				if path != b.identityPath {
					return writeDurable(path, v)
				}
				ops := osDurableOps
				switch failure {
				case "fsync":
					ops.createTemp = func(dir, pattern string) (durableFile, error) {
						f, err := os.CreateTemp(dir, pattern)
						if err != nil {
							return nil, err
						}
						return &failingSyncFile{File: f, err: injected}, nil
					}
				case "rename":
					ops.rename = func(_, _ string) error { return injected }
				}
				return writeDurableWith(ops, path, v)
			}

			mediumCalls := 0
			send := func() error {
				if err := b.Acquire(context.Background(), 1); err != nil {
					return err
				}
				mediumCalls++
				return nil
			}
			if err := send(); !errors.Is(err, injected) {
				t.Fatalf("send error = %v, want %v", err, injected)
			}
			if mediumCalls != 0 {
				t.Fatalf("made %d medium call(s) after identity persistence failed", mediumCalls)
			}
			if got := len(readLedger(t, b.umbrellaPath).Minute); got != 1 {
				t.Fatalf("umbrella retained %d entries, want the failed write's charge", got)
			}
			if _, err := os.Stat(b.identityPath); !os.IsNotExist(err) {
				t.Fatalf("identity ledger exists after injected %s failure: %v", failure, err)
			}
		})
	}
}

// removeDurable is idempotent — invalidating an already-absent marker is a
// completed invalidation, not an error, so a retried downgrade converges.
func TestRemoveDurableIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marker.json")
	if err := removeDurable(path); err != nil {
		t.Errorf("removing an absent record: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeDurable(path); err != nil {
		t.Errorf("removing a present record: %v", err)
	}
	if err := removeDurable(path); err != nil {
		t.Errorf("second removal: %v", err)
	}
}

func TestRemoveDurableSyscallOrder(t *testing.T) {
	var trace []string
	path := filepath.Join(t.TempDir(), "marker.json")
	if err := removeDurableWith(tracedOps("marker", &trace), path); err != nil {
		t.Fatal(err)
	}
	want := []string{"marker:unlink", "marker:open-dir", "marker:fsync-dir", "marker:close-dir"}
	if fmt.Sprint(trace) != fmt.Sprint(want) {
		t.Fatalf("durable deletion order:\n got %v\nwant %v", trace, want)
	}
}
