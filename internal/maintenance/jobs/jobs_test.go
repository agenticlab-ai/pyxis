package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"go.yaml.in/yaml/v3"
)

var fixtureSource = SourceBinding{Remote: "origin", Branch: "main"}

func fixtureJob(id string) Job {
	return Job{ID: id, Enabled: true, Schedule: Schedule{Cron: "0 2 * * *", Timezone: "America/Los_Angeles"}, Timeout: "90m", Runner: Runner{Harness: entity.Codex, Routing: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextLarge, Assurance: executiondemand.AssuranceOrdinary}}, Delivery: &Delivery{Mode: "draft-pr", Branch: "maintenance/" + id}, Prompt: "Perform repository maintenance."}
}

func fixtureStore(t *testing.T, jobs ...Job) *Store {
	t.Helper()
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	for index, job := range jobs {
		var source *SourceBinding
		if index == 0 {
			source = &fixtureSource
		}
		if err := store.Create(t.Context(), source, job); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func fileBytes(t *testing.T, store *Store) []byte {
	t.Helper()
	data, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCRUDPreservesIdentityUnrelatedJobsAndRepositoryIsolation(t *testing.T) {
	first := fixtureStore(t, fixtureJob("first"), fixtureJob("second"))
	second := fixtureStore(t, fixtureJob("other"))
	originalSecond, err := first.Read(t.Context(), "second")
	if err != nil {
		t.Fatal(err)
	}
	beforeRead := fileBytes(t, first)
	got, err := first.Read(t.Context(), "first")
	if err != nil || got.ID != "first" {
		t.Fatalf("read = %#v, %v", got, err)
	}
	list, err := first.List(t.Context())
	if err != nil || len(list) != 2 || !reflect.DeepEqual(beforeRead, fileBytes(t, first)) {
		t.Fatalf("list/read changed bytes or values: %#v, %v", list, err)
	}
	updated := fixtureJob("first")
	updated.Prompt = "Updated prompt."
	if err := first.Update(t.Context(), "first", updated); err != nil {
		t.Fatal(err)
	}
	if preserved, _ := first.Read(t.Context(), "second"); !reflect.DeepEqual(preserved, originalSecond) {
		t.Fatalf("unrelated job changed: %#v", preserved)
	}
	if err := first.Delete(t.Context(), "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Read(t.Context(), "first"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted read error = %v", err)
	}
	if jobs, err := second.List(t.Context()); err != nil || len(jobs) != 1 || jobs[0].ID != "other" {
		t.Fatalf("second repository changed: %#v, %v", jobs, err)
	}
}

func TestLoadReturnsValidatedDetachedDocumentWithoutWrites(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	before := fileBytes(t, store)
	document, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	document.Jobs[0].ID = "changed"
	document.Jobs[0].Delivery.Branch = "changed"
	again, err := store.Load(t.Context())
	if err != nil || again.Jobs[0].ID != "first" || again.Jobs[0].Delivery.Branch == "changed" {
		t.Fatalf("load alias or error: %#v, %v", again, err)
	}
	if !reflect.DeepEqual(before, fileBytes(t, store)) {
		t.Fatal("load changed policy bytes")
	}
	if !ValidID("daily-job") || ValidID("INVALID") {
		t.Fatal("exported ID validation diverged")
	}
}

func TestLoadPreservesValidatedEmptyDocument(t *testing.T) {
	store := fixtureStore(t, fixtureJob("last"))
	if err := store.Delete(t.Context(), "last"); err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, store)
	document, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("loaded empty policy lost its validated shape: %v", err)
	}
	if _, err := Encode(document); err != nil {
		t.Fatalf("loaded empty policy cannot be serialized: %v", err)
	}
	if !reflect.DeepEqual(before, fileBytes(t, store)) {
		t.Fatal("loading empty policy changed file bytes")
	}
}

func TestCreateRequiresBindingAndNamedMutationErrors(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(t.Context(), nil, fixtureJob("first")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing binding error = %v", err)
	}
	if _, err := os.Stat(store.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing binding created target: %v", err)
	}
	if err := store.Create(t.Context(), &fixtureSource, fixtureJob("first")); err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, store)
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"exists", store.Create(t.Context(), nil, fixtureJob("first")), ErrAlreadyExists},
		{"missing update", store.Update(t.Context(), "missing", fixtureJob("missing")), ErrNotFound},
		{"missing delete", store.Delete(t.Context(), "missing"), ErrNotFound},
		{"identity conflict", store.Update(t.Context(), "first", fixtureJob("changed")), ErrConflict},
		{"binding conflict", store.Create(t.Context(), &SourceBinding{Remote: "upstream", Branch: "main"}, fixtureJob("new")), ErrConflict},
		{"invalid replacement", store.Update(t.Context(), "first", func() Job { job := fixtureJob("first"); job.Timeout = "0s"; return job }()), ErrInvalid},
	}
	for _, test := range tests {
		if !errors.Is(test.err, test.want) {
			t.Errorf("%s error = %v, want %v", test.name, test.err, test.want)
		}
	}
	if !reflect.DeepEqual(before, fileBytes(t, store)) {
		t.Fatal("failed mutations changed the target")
	}
	missing, _ := New(t.TempDir())
	if _, err := missing.List(t.Context()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing list error = %v", err)
	}
}

func TestDecodeRejectsClosedDocumentViolations(t *testing.T) {
	valid, err := Encode(Document{SchemaVersion: SchemaVersion, Source: fixtureSource, Jobs: []Job{fixtureJob("first")}})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"malformed":          "schema_version: [",
		"unknown":            string(valid) + "unknown: true\n",
		"duplicate key":      "schema_version: pyxis-maintenance-jobs/v1\nschema_version: pyxis-maintenance-jobs/v1\nsource: {remote: origin, branch: main}\njobs: []\n",
		"alias":              "schema_version: pyxis-maintenance-jobs/v1\nsource: &source {remote: origin, branch: main}\njobs: []\ncopy: *source\n",
		"unsupported schema": "schema_version: pyxis-maintenance-jobs/v2\nsource: {remote: origin, branch: main}\njobs: []\n",
		"extra document":     string(valid) + "---\n{}\n",
		"duplicate id":       string(rawEncode(t, Document{SchemaVersion: SchemaVersion, Source: fixtureSource, Jobs: []Job{fixtureJob("first"), fixtureJob("first")}})),
	}
	invalidID := fixtureJob("INVALID")
	cases["invalid id"] = string(rawEncode(t, Document{SchemaVersion: SchemaVersion, Source: fixtureSource, Jobs: []Job{invalidID}}))
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(data)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestDocumentValidationRejectsInvalidTypedValues(t *testing.T) {
	tests := map[string]func(*Document){
		"invalid source":  func(document *Document) { document.Source.Branch = "topic/.hidden" },
		"invalid cron":    func(document *Document) { document.Jobs[0].Schedule.Cron = "*/5 * * * *" },
		"invalid routing": func(document *Document) { document.Jobs[0].Runner.Routing.Capability = "unknown" },
		"duplicate delivery": func(document *Document) {
			second := fixtureJob("second")
			second.Delivery.Branch = document.Jobs[0].Delivery.Branch
			document.Jobs = append(document.Jobs, second)
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			document := Document{SchemaVersion: SchemaVersion, Source: fixtureSource, Jobs: []Job{fixtureJob("first")}}
			change(&document)
			if err := document.Validate(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestMalformedStoredDocumentBlocksMutationUnchanged(t *testing.T) {
	store, _ := New(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := []byte("schema_version: [")
	if err := os.WriteFile(store.path, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(t.Context(), "first"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
	if got := fileBytes(t, store); !reflect.DeepEqual(got, bad) {
		t.Fatalf("malformed target changed: %q", got)
	}
}

func rawEncode(t *testing.T, document Document) []byte {
	t.Helper()
	data, err := yaml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSymlinkTargetsRefuseWithoutChangingReferent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis/maintenance"), 0o755); err != nil {
		t.Fatal(err)
	}
	referent := filepath.Join(t.TempDir(), "jobs.yaml")
	before := []byte("unchanged")
	if err := os.WriteFile(referent, before, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(referent, filepath.Join(root, ".pyxis/maintenance/jobs.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); !errors.Is(err, ErrInvalid) {
		t.Fatalf("New error = %v", err)
	}
	after, _ := os.ReadFile(referent)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("symlink referent changed")
	}
}

func TestConcurrentCreatesDoNotLoseUpdates(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const count = 24
	var wait sync.WaitGroup
	errorsFound := make(chan error, count)
	start := make(chan struct{})
	for index := range count {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start // Race the initial lock-file creation as well as later updates.
			id := "job-" + string(rune('a'+index))
			errorsFound <- store.Create(context.Background(), &fixtureSource, fixtureJob(id))
		}(index)
	}
	close(start)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := store.List(t.Context())
	if err != nil || len(jobs) != count {
		t.Fatalf("jobs = %d, %v", len(jobs), err)
	}
}

func TestFailuresCancellationAndUnconfirmedCommit(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	prior := fileBytes(t, store)
	failure := errors.New("injected")
	store.beforeWrite = func() error { return failure }
	if err := store.Update(t.Context(), "first", changedJob()); !errors.Is(err, failure) || !reflect.DeepEqual(prior, fileBytes(t, store)) {
		t.Fatalf("write failure = %v or changed target", err)
	}
	store.beforeWrite = nil
	store.rename = func(*os.Root, string, string) error { return failure }
	if err := store.Update(t.Context(), "first", changedJob()); !errors.Is(err, failure) || !reflect.DeepEqual(prior, fileBytes(t, store)) {
		t.Fatalf("replace failure = %v or changed target", err)
	}
	store.rename = (*os.Root).Rename
	ctx, cancel := context.WithCancel(context.Background())
	store.beforeWrite = func() error { cancel(); return nil }
	if err := store.Update(ctx, "first", changedJob()); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(prior, fileBytes(t, store)) {
		t.Fatalf("cancellation = %v or changed target", err)
	}
	store.beforeWrite = nil
	store.syncDir = func(*os.Root) error { return failure }
	err := store.Update(t.Context(), "first", changedJob())
	var uncertain *UnconfirmedError
	if !errors.Is(err, ErrUnconfirmed) || !errors.As(err, &uncertain) {
		t.Fatalf("post-commit error = %v", err)
	}
	committed := fileBytes(t, store)
	if !reflect.DeepEqual(committed, uncertain.Bytes) {
		t.Fatal("uncertainty did not carry complete committed bytes")
	}
	if _, err := Decode(uncertain.Bytes); err != nil {
		t.Fatalf("uncertain bytes are not parseable: %v", err)
	}
	store.syncDir = syncDirectory
	if err := store.Update(t.Context(), "first", fixtureJob("first")); err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
}

func changedJob() Job {
	job := fixtureJob("first")
	job.Prompt = "Changed"
	return job
}

func TestCanceledLockWaitReleasesOwnedLocks(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- store.withLock(context.Background(), func() error { close(locked); <-release; return nil })
	}()
	<-locked
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := store.Update(ctx, "first", changedJob()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock cancellation = %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := store.Update(t.Context(), "first", changedJob()); err != nil {
		t.Fatalf("subsequent lock = %v", err)
	}
}

func TestCanonicalRootAliasesShareLockIdentity(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "repo")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	first, _ := New(root)
	second, err := New(alias)
	if err != nil || first.Root() != second.Root() || first.lockPath != second.lockPath {
		t.Fatalf("canonical stores differ: %#v %#v %v", first, second, err)
	}
}

func TestExternalLockCancellation(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	file, err := os.OpenFile(store.lockPath, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := store.Delete(ctx, "first"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
}
